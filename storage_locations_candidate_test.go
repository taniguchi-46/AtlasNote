package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/config"
	"atlasnote/internal/diagnostics"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestStorageLocationSelectionRejectsInvalidFirstAndPreservesValidCandidate(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	defaultRoot := filepath.Join(t.TempDir(), "default")
	invalidRoot := filepath.Join(t.TempDir(), "invalid")
	validRoot := filepath.Join(t.TempDir(), "valid")
	if err := os.MkdirAll(invalidRoot, 0o700); err != nil {
		t.Fatalf("create invalid root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidRoot, "unrelated.txt"), []byte("not Atlas Note"), 0o600); err != nil {
		t.Fatalf("write invalid root: %v", err)
	}
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	t.Cleanup(func() { app.shutdown(t.Context()) })
	paths := []string{invalidRoot, validRoot, invalidRoot}
	index := 0
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		path := paths[index]
		index++
		return path, nil
	}

	first := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if first.Error == nil || first.Status == nil {
		t.Fatalf("invalid first selection = %#v", first)
	}
	if first.Status.PendingDataRoot != "" || first.Status.PendingSelection {
		t.Fatalf("invalid first selection mutated pending state: %#v", first.Status)
	}

	valid := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if valid.Error != nil || valid.Status == nil || valid.Status.PendingDataRoot != filepath.Clean(validRoot) || valid.Status.PendingBackupRoot != filepath.Clean(validRoot) {
		t.Fatalf("valid selection = %#v", valid)
	}
	invalid := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if invalid.Error == nil || invalid.Status == nil {
		t.Fatalf("invalid replacement selection = %#v", invalid)
	}
	if invalid.Status.PendingDataRoot != filepath.Clean(validRoot) || invalid.Status.PendingBackupRoot != filepath.Clean(validRoot) || !invalid.Status.PendingSelection {
		t.Fatalf("valid candidate was lost after rejection: %#v", invalid.Status)
	}
}

func TestApplyStorageLocationsRevalidatesAndClearsOnlyUnconfirmedCandidate(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	defaultRoot := filepath.Join(t.TempDir(), "default")
	selectedRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	t.Cleanup(func() { app.shutdown(t.Context()) })
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) { return selectedRoot, nil }
	selected := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if selected.Error != nil {
		t.Fatalf("select target: %#v", selected)
	}
	if err := os.Remove(selectedRoot); err != nil {
		t.Fatalf("remove selected target: %v", err)
	}
	if err := os.WriteFile(selectedRoot, []byte("target became unavailable"), 0o600); err != nil {
		t.Fatalf("replace selected target: %v", err)
	}

	applied := app.ApplyStorageLocations()
	if applied.Error == nil || applied.Status == nil || applied.RestartRequired {
		t.Fatalf("unavailable target apply = %#v", applied)
	}
	if applied.Status.PendingSelection || applied.Status.PendingDataRoot != "" || applied.Status.PendingBackupRoot != "" {
		t.Fatalf("unconfirmed candidate was not cleared: %#v", applied.Status)
	}
	if _, err := config.LoadStorageLocationsFrom(configFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed apply changed bootstrap config: %v", err)
	}
}

func TestStorageLocationDiagnosticKeepsSafeDetailsAndDeduplicatesStatusFailures(t *testing.T) {
	app := &App{
		diagnostics:  diagnostics.NewStore("", diagnostics.Metadata{AppVersion: "0.1.0", VCSRevision: "test-revision"}),
		startupPhase: StartupPhaseStorageRecovery,
	}
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("file"), 0o600); err != nil {
		t.Fatalf("write root fixture: %v", err)
	}
	_, cause := config.ProbeDataRoot(root)
	first := app.storageLocationErrorFor(cause, "storage-location.select", string(StartupPhaseStorageRecovery), "data", "status-test")
	second := app.storageLocationErrorFor(cause, "storage-location.select", string(StartupPhaseStorageRecovery), "data", "status-test")
	if first == nil || second == nil || first.DiagnosticID == "" || first.DiagnosticID != second.DiagnosticID {
		t.Fatalf("diagnostic IDs = %#v, %#v", first, second)
	}
	if first.Code != "STORAGE_LOCATION_NOT_DIRECTORY" || first.Reason == "" || first.Action == "" {
		t.Fatalf("diagnostic error = %#v", first)
	}
	result := app.GetStorageLocationDiagnostics()
	if len(result.Events) != 1 || result.Events[0].DiagnosticID != first.DiagnosticID || result.Report == "" {
		t.Fatalf("diagnostics result = %#v", result)
	}
	if strings.Contains(result.Report, root) || strings.Contains(result.Report, "file") {
		t.Fatalf("diagnostics report leaked path: %s", result.Report)
	}
}

func TestRecoverySelectsBothUnavailableRootsInEitherOrder(t *testing.T) {
	for _, first := range []string{"data", "backup"} {
		t.Run(first, func(t *testing.T) {
			t.Setenv("ATLAS_NOTE_DATA_DIR", "")
			configFile := filepath.Join(t.TempDir(), "locations.json")
			t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
			data, backup := t.TempDir(), t.TempDir()
			if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{Version: 1, DataRoot: data, BackupRoot: backup}); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(configFile)
			for _, root := range []string{data, backup} {
				if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			app := NewApp()
			app.diagnostics = diagnostics.NewStore(t.TempDir(), diagnostics.Metadata{})
			app.startup(t.Context())
			t.Cleanup(func() { app.shutdown(t.Context()) })
			if app.startupPhase != StartupPhaseStorageRecovery {
				t.Fatal(app.startupPhase)
			}
			targets := map[string]string{"data": t.TempDir(), "backup": t.TempDir()}
			selectedPath := targets[first]
			app.openDirectory = func(context.Context, runtime.OpenDialogOptions) (string, error) { return selectedPath, nil }
			if result := app.SelectStorageLocation(first); result.Error != nil {
				t.Fatalf("first: %#v", result.Error)
			}
			if result := app.ApplyStorageLocations(); result.Error == nil || result.RestartRequired || result.Status.PendingSelection {
				t.Fatalf("partial apply: %#v", result)
			}
			after, _ := os.ReadFile(configFile)
			if string(after) != string(before) {
				t.Fatal("config changed")
			}
			for _, root := range []string{data, backup} {
				b, e := os.ReadFile(filepath.Join(root, "keep.txt"))
				if e != nil || string(b) != "keep" {
					t.Fatal("source changed")
				}
			}
			second := "backup"
			if first == "backup" {
				second = "data"
			}
			for _, kind := range []string{first, second} {
				selectedPath = targets[kind]
				if r := app.SelectStorageLocation(kind); r.Error != nil {
					t.Fatalf("select %s: %#v", kind, r.Error)
				}
			}
			if r := app.ApplyStorageLocations(); r.Error != nil || !r.RestartRequired {
				t.Fatalf("apply pair: %#v", r)
			}
			app.shutdown(t.Context())
			restarted := NewApp()
			restarted.startup(t.Context())
			t.Cleanup(func() { restarted.shutdown(t.Context()) })
			if status := restarted.GetStartupStatus(); !status.Ready {
				t.Fatalf("reconfigured startup: %#v", status)
			}
			locations, err := config.LoadStorageLocationsFrom(configFile)
			if err != nil || locations.DataRoot != targets["data"] || locations.BackupRoot != targets["backup"] {
				t.Fatalf("reconfigured roots: %#v, %v", locations, err)
			}
		})
	}
}

func TestCandidatePairRejectsContainmentAndAllowsSameRoot(t *testing.T) {
	root := t.TempDir()
	for _, pair := range [][2]string{{root, filepath.Join(root, "child")}, {filepath.Join(root, "child"), root}} {
		if err := config.ValidateStorageLocationPaths(config.StorageLocations{Version: 1, DataRoot: pair[0], BackupRoot: pair[1]}); config.RootErrorCodeOf(err) != config.RootErrorOverlappingRoots {
			t.Fatalf("containment: %v", err)
		}
	}
	if err := config.ValidateStorageLocationPaths(config.StorageLocations{Version: 1, DataRoot: root, BackupRoot: root}); err != nil {
		t.Fatal(err)
	}
}

func TestSelectionAndFailedApplyKeepPersistentMigration(t *testing.T) {
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", filepath.Join(t.TempDir(), "locations.json"))
	data, backup := t.TempDir(), t.TempDir()
	pending := config.PendingStorageLocationMigration{Version: 1, ID: "preserve-selection", SourceDataRoot: data, SourceBackupRoot: backup, TargetDataRoot: t.TempDir(), TargetBackupRoot: t.TempDir()}
	if err := config.SavePendingStorageLocationMigration(pending); err != nil {
		t.Fatal(err)
	}
	marker, err := config.PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.startupPhase = StartupPhaseStorageRecovery
	app.managementRoot = data
	app.archiveRoot = backup
	selected := t.TempDir()
	app.openDirectory = func(context.Context, runtime.OpenDialogOptions) (string, error) { return selected, nil }
	if r := app.SelectStorageLocation("data"); r.Error != nil {
		t.Fatal(r.Error)
	}
	// A contained candidate is rejected while the previous valid pair remains.
	selected = filepath.Join(selected, "nested")
	if r := app.SelectStorageLocation("backup"); r.Error == nil || r.Status.PendingDataRoot != filepath.Dir(selected) {
		t.Fatalf("invalid selection: %#v", r)
	}
	if err := os.WriteFile(filepath.Join(pending.TargetBackupRoot, "unrelated.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := app.ApplyStorageLocations(); r.Error == nil || r.Status.PendingSelection || !r.Status.PendingMigration {
		t.Fatalf("invalid apply: %#v", r)
	}
	after, err := os.ReadFile(marker)
	if err != nil || string(before) != string(after) {
		t.Fatal("persistent migration changed")
	}
}

func TestBackupAutomaticallyFollowsRepeatedDataSelection(t *testing.T) {
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", filepath.Join(t.TempDir(), "locations.json"))
	root := t.TempDir()
	app := NewApp()
	app.managementRoot = root
	app.archiveRoot = root
	selected := t.TempDir()
	app.openDirectory = func(context.Context, runtime.OpenDialogOptions) (string, error) { return selected, nil }
	for i := 0; i < 2; i++ {
		selected = t.TempDir()
		r := app.SelectStorageLocation("data")
		if r.Error != nil || r.Status.PendingBackupRoot != selected {
			t.Fatalf("follow: %#v", r)
		}
	}
	if r := app.SelectStorageLocation("backup"); r.Error != nil {
		t.Fatal(r.Error)
	}
	explicit := selected
	selected = t.TempDir()
	if r := app.SelectStorageLocation("data"); r.Error != nil || r.Status.PendingBackupRoot != explicit {
		t.Fatalf("explicit backup changed: %#v", r)
	}
}
