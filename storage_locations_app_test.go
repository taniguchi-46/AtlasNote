package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/config"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestNewAppRequiresSetupBeforeCreatingDefaultData(t *testing.T) {
	defaultRoot := filepath.Join(t.TempDir(), "default")
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	status := app.GetStartupStatus()
	if status.Ready || !status.SetupRequired || status.Phase != StartupPhaseSetupRequired {
		t.Fatalf("startup status = %#v", status)
	}
	if _, err := os.Stat(defaultRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("setup probe created default data root: %v", err)
	}
	if app.db != nil || app.spaceRegistry != nil {
		t.Fatal("setup-required app opened data services")
	}
}

func TestStorageLocationSelectionUsesNativeDirectoryDialogAndPersistsSetup(t *testing.T) {
	defaultRoot := filepath.Join(t.TempDir(), "default")
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	dataRoot := filepath.Join(t.TempDir(), "data")
	backupRoot := filepath.Join(t.TempDir(), "backup")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	t.Cleanup(func() { app.shutdown(t.Context()) })
	paths := []string{dataRoot, backupRoot}
	index := 0
	app.openDirectory = func(_ context.Context, options runtime.OpenDialogOptions) (string, error) {
		if options.Title == "" {
			t.Fatal("directory dialog title is empty")
		}
		selected := paths[index]
		index++
		return selected, nil
	}
	dataResult := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if dataResult.Error != nil || dataResult.Path != filepath.Clean(dataRoot) {
		t.Fatalf("data selection = %#v", dataResult)
	}
	backupResult := app.SelectStorageLocation(string(StorageLocationBackupRoot))
	if backupResult.Error != nil || backupResult.Path != filepath.Clean(backupRoot) {
		t.Fatalf("backup selection = %#v", backupResult)
	}
	applied := app.ApplyStorageLocations()
	if applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply setup locations = %#v", applied)
	}
	locations, err := config.LoadStorageLocationsFrom(configFile)
	if err != nil {
		t.Fatalf("load persisted locations: %v", err)
	}
	if locations.DataRoot != filepath.Clean(dataRoot) || locations.BackupRoot != filepath.Clean(backupRoot) {
		t.Fatalf("persisted locations = %#v", locations)
	}
}

func TestDefaultStorageLocationSetupRestartsAndCreatesNotebook(t *testing.T) {
	defaultRoot := filepath.Join(t.TempDir(), "default")
	configFile := filepath.Join(defaultRoot, "storage-locations.json")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	app.startup(t.Context())
	if status := app.GetStartupStatus(); status.Phase != StartupPhaseSetupRequired || status.Ready {
		t.Fatalf("initial setup status = %#v", status)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply default locations = %#v", applied)
	}
	app.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	t.Cleanup(func() { restarted.shutdown(t.Context()) })
	status := restarted.GetStartupStatus()
	if !status.Ready || status.Phase != StartupPhaseReady {
		t.Fatalf("restarted status = %#v", status)
	}
	created, err := restarted.CreateNotebook(note.NotebookCreateInput{Name: "初回ノートブック"})
	if err != nil {
		t.Fatalf("create notebook after default setup: %v", err)
	}
	if created.Name != "初回ノートブック" {
		t.Fatalf("created notebook = %#v", created)
	}
}

func TestInvalidSavedStorageLocationEntersRecoveryWithoutChangingOldRoot(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	oldRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldRoot, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write old root marker: %v", err)
	}
	newRoot := filepath.Join(t.TempDir(), "new-root")
	locations := config.StorageLocations{Version: 1, DataRoot: oldRoot, BackupRoot: oldRoot}
	encoded, err := json.Marshal(locations)
	if err != nil {
		t.Fatalf("marshal invalid locations: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configFile), 0o700); err != nil {
		t.Fatalf("create bootstrap directory: %v", err)
	}
	if err := os.WriteFile(configFile, encoded, 0o600); err != nil {
		t.Fatalf("write invalid locations: %v", err)
	}
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	status := app.GetStartupStatus()
	if status.Ready || status.Phase != StartupPhaseStorageRecovery || status.StorageLocationError == nil {
		t.Fatalf("recovery status = %#v", status)
	}
	if status.StorageLocationError.Code != "STORAGE_LOCATION_UNRELATED_CONTENT" {
		t.Fatalf("recovery error = %#v", status.StorageLocationError)
	}
	if _, err := app.CreateNotebook(note.NotebookCreateInput{Name: "作成不可"}); err == nil {
		t.Fatal("notebook creation succeeded before storage recovery")
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return newRoot, nil
	}
	selected := app.SelectStorageLocation(string(StorageLocationDataRoot))
	if selected.Error != nil || selected.Path != filepath.Clean(newRoot) {
		t.Fatalf("select recovery root = %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply recovery root = %#v", applied)
	}
	if _, err := os.Stat(filepath.Join(oldRoot, "keep.txt")); err != nil {
		t.Fatalf("old root changed: %v", err)
	}
	app.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	t.Cleanup(func() { restarted.shutdown(t.Context()) })
	if status := restarted.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(newRoot) {
		t.Fatalf("recovered startup status = %#v", status)
	}
}

func TestStorageLocationMigrationCopiesCurrentRootOnNextStartup(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	dataRoot := filepath.Join(t.TempDir(), "data")
	// Native directory selection returns an existing directory. Use an empty
	// one to cover the restart-time migration path from the reported failure.
	targetRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: dataRoot}); err != nil {
		t.Fatalf("save initial locations: %v", err)
	}
	app := NewApp()
	app.startup(t.Context())
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatalf("initial app is not ready: %#v", status)
	}
	created, err := app.CreateNote(note.CreateInput{Title: "移行", Content: "移行本文"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) { return targetRoot, nil }
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select migration target: %#v", selected)
	}
	applied := app.ApplyStorageLocations()
	if applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply migration: %#v", applied)
	}
	pending, err := config.LoadPendingStorageLocationMigration()
	if err != nil || pending.Version != config.PendingStorageMigrationVersion || pending.DataPlan != config.PendingStorageMigrationPlanCopyRequired || pending.BackupPlan != config.PendingStorageMigrationPlanUnchanged || pending.Phase != config.PendingStorageMigrationPhasePrepared {
		t.Fatalf("persisted migration plan = %#v, %v", pending, err)
	}
	app.shutdown(t.Context())

	moved := NewApp()
	moved.startup(t.Context())
	t.Cleanup(func() { moved.shutdown(t.Context()) })
	status := moved.GetStartupStatus()
	if !status.Ready || status.DataDir != filepath.Clean(targetRoot) {
		t.Fatalf("migrated startup status = %#v", status)
	}
	got, err := moved.GetNote(created.ID)
	if err != nil || got.Content != "移行本文" {
		t.Fatalf("migrated note = %#v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "notes", created.ID+".md")); err != nil {
		t.Fatalf("source data was removed: %v", err)
	}
}

func TestFailedStorageLocationMigrationCanReturnToOriginalRoot(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	sourceRoot := filepath.Join(t.TempDir(), "source")
	targetRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", sourceRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	setup := NewApp()
	setup.startup(t.Context())
	if applied := setup.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply initial setup: %#v", applied)
	}
	setup.shutdown(t.Context())

	app := NewApp()
	app.startup(t.Context())
	created, err := app.CreateNote(note.CreateInput{Title: "元データ", Content: "保持する本文"})
	if err != nil {
		t.Fatalf("create source note: %v", err)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return targetRoot, nil
	}
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select migration target: %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("save migration: %#v", applied)
	}
	app.shutdown(t.Context())

	if err := os.WriteFile(filepath.Join(targetRoot, "unrelated.txt"), []byte("changed after selection"), 0o600); err != nil {
		t.Fatalf("invalidate target after selection: %v", err)
	}
	failed := NewApp()
	failed.startup(t.Context())
	status := failed.GetStartupStatus()
	t.Cleanup(func() { failed.shutdown(t.Context()) })
	if status.Ready || status.Phase != StartupPhaseStorageRecovery || status.StorageLocationError == nil {
		t.Fatalf("failed migration status = %#v", status)
	}
	if !status.StorageLocations.PendingMigration || status.StorageLocations.PendingDataRoot != filepath.Clean(targetRoot) {
		t.Fatalf("pending migration status = %#v", status.StorageLocations)
	}

	if result := failed.CancelPendingStorageLocationMigration(); result.Error != nil || !result.RestartRequired {
		t.Fatalf("cancel pending migration: %#v", result)
	} else if result.Status == nil || result.Status.PendingDataRoot != "" || result.Status.PendingBackupRoot != "" {
		t.Fatalf("cancel status exposed stale target: %#v", result.Status)
	}
	failed.shutdown(t.Context())

	restored := NewApp()
	restored.startup(t.Context())
	t.Cleanup(func() { restored.shutdown(t.Context()) })
	if status := restored.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(sourceRoot) {
		t.Fatalf("restored startup status = %#v", status)
	}
	got, err := restored.GetNote(created.ID)
	if err != nil || got.Content != "保持する本文" {
		t.Fatalf("restored note = %#v, %v", got, err)
	}
	if _, err := config.LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled migration remains: %v", err)
	}
}

func TestFailedStorageLocationMigrationCanStartInAnotherEmptyRootWithoutCopying(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	sourceRoot := filepath.Join(t.TempDir(), "source")
	failedTarget := t.TempDir()
	newRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", sourceRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	setup := NewApp()
	setup.startup(t.Context())
	if applied := setup.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply initial setup: %#v", applied)
	}
	setup.shutdown(t.Context())

	app := NewApp()
	app.startup(t.Context())
	created, err := app.CreateNote(note.CreateInput{Title: "元データ", Content: "自動移行しない本文"})
	if err != nil {
		t.Fatalf("create source note: %v", err)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return failedTarget, nil
	}
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select failed target: %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("save migration: %#v", applied)
	}
	app.shutdown(t.Context())
	if err := os.WriteFile(filepath.Join(failedTarget, "unrelated.txt"), []byte("invalid"), 0o600); err != nil {
		t.Fatalf("invalidate target: %v", err)
	}

	failed := NewApp()
	failed.startup(t.Context())
	t.Cleanup(func() { failed.shutdown(t.Context()) })
	if status := failed.GetStartupStatus(); status.Phase != StartupPhaseStorageRecovery {
		t.Fatalf("recovery status = %#v", status)
	}
	failed.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return newRoot, nil
	}
	if selected := failed.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select new root: %#v", selected)
	}
	if applied := failed.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("switch to new root: %#v", applied)
	}
	failed.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	t.Cleanup(func() { restarted.shutdown(t.Context()) })
	if status := restarted.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(newRoot) {
		t.Fatalf("new root startup status = %#v", status)
	}
	if _, err := restarted.GetNote(created.ID); err == nil {
		t.Fatal("source note was copied into a newly selected empty root")
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "notes", created.ID+".md")); err != nil {
		t.Fatalf("source note disappeared: %v", err)
	}
	if _, err := config.LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaced migration remains: %v", err)
	}
}

func TestFailedStorageLocationMigrationCanRetryAfterTargetRecovers(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	sourceRoot := filepath.Join(t.TempDir(), "source")
	targetRoot := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", sourceRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)

	setup := NewApp()
	setup.startup(t.Context())
	if applied := setup.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("apply initial setup: %#v", applied)
	}
	setup.shutdown(t.Context())

	app := NewApp()
	app.startup(t.Context())
	created, err := app.CreateNote(note.CreateInput{Title: "再試行", Content: "再試行本文"})
	if err != nil {
		t.Fatalf("create source note: %v", err)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return targetRoot, nil
	}
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select target: %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("save migration: %#v", applied)
	}
	app.shutdown(t.Context())
	marker := filepath.Join(targetRoot, "unrelated.txt")
	if err := os.WriteFile(marker, []byte("temporary failure"), 0o600); err != nil {
		t.Fatalf("invalidate target: %v", err)
	}
	failed := NewApp()
	failed.startup(t.Context())
	if status := failed.GetStartupStatus(); status.Phase != StartupPhaseStorageRecovery {
		t.Fatalf("recovery status = %#v", status)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatalf("repair target: %v", err)
	}
	if result := failed.RetryPendingStorageLocationMigration(); result.Error != nil || !result.RestartRequired {
		t.Fatalf("retry pending migration: %#v", result)
	}
	failed.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	t.Cleanup(func() { restarted.shutdown(t.Context()) })
	if status := restarted.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(targetRoot) {
		t.Fatalf("retried startup status = %#v", status)
	}
	got, err := restarted.GetNote(created.ID)
	if err != nil || got.Content != "再試行本文" {
		t.Fatalf("retried note = %#v, %v", got, err)
	}
}

func TestInvalidLegacyMigrationKeepsRecoveryStatusAndAllowsExplicitSwitch(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	sourceData := filepath.Join(t.TempDir(), "source-data")
	sourceBackup := filepath.Join(t.TempDir(), "source-backup")
	newData := filepath.Join(t.TempDir(), "new-data")
	defaultRoot := filepath.Join(t.TempDir(), "default")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", defaultRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{Version: 1, DataRoot: sourceData, BackupRoot: sourceBackup}); err != nil {
		t.Fatalf("save bootstrap locations: %v", err)
	}
	legacy := config.PendingStorageLocationMigration{
		Version: 1, ID: "legacy-invalid-overlap",
		SourceDataRoot: sourceData, TargetDataRoot: filepath.Join(sourceData, "nested-target"),
		SourceBackupRoot: sourceBackup, TargetBackupRoot: sourceBackup,
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy migration: %v", err)
	}
	markerPath, err := config.PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatalf("pending marker path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		t.Fatalf("create marker directory: %v", err)
	}
	if err := os.WriteFile(markerPath, encoded, 0o600); err != nil {
		t.Fatalf("write legacy migration: %v", err)
	}

	app := NewApp()
	app.startup(t.Context())
	defer app.shutdown(t.Context())
	status := app.GetStartupStatus()
	if status.Ready || status.Phase != StartupPhaseStorageRecovery || status.StorageLocationError == nil {
		t.Fatalf("legacy recovery status = %#v", status)
	}
	if status.StorageLocations == nil || !status.StorageLocations.PendingMigration {
		t.Fatalf("legacy pending status = %#v", status.StorageLocations)
	}
	statusResult := app.GetStorageLocationStatus()
	if statusResult.Status == nil || statusResult.Error == nil {
		t.Fatalf("status result lost status or error: %#v", statusResult)
	}
	if statusResult.Status.PendingDataRoot != "" || statusResult.Status.PendingBackupRoot != "" {
		t.Fatalf("unsafe legacy targets were exposed as pending choices: %#v", statusResult.Status)
	}

	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return newData, nil
	}
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select explicit recovery root: %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("save explicit recovery switch: %#v", applied)
	}
	if got, err := os.ReadFile(markerPath); err != nil || !strings.Contains(string(got), `"action": "switch"`) {
		t.Fatalf("replacement switch intent = %q, %v", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(sourceData, "nested-target")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid legacy target was touched: %v", err)
	}
	app.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	defer restarted.shutdown(t.Context())
	if status := restarted.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(newData) {
		t.Fatalf("explicit switch startup status = %#v", status)
	}
	if _, err := config.LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("switch marker remains after restart: %v", err)
	}
	locations, err := config.LoadStorageLocations()
	if err != nil || locations.DataRoot != filepath.Clean(newData) {
		t.Fatalf("switched bootstrap locations = %#v, %v", locations, err)
	}
}

func TestCorruptMigrationMarkerCanBeReplacedWithoutUsingItsContents(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	sourceData := filepath.Join(t.TempDir(), "source-data")
	sourceBackup := filepath.Join(t.TempDir(), "source-backup")
	newData := filepath.Join(t.TempDir(), "new-data")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{Version: 1, DataRoot: sourceData, BackupRoot: sourceBackup}); err != nil {
		t.Fatalf("save bootstrap locations: %v", err)
	}
	markerPath, err := config.PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatalf("pending marker path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		t.Fatalf("create marker directory: %v", err)
	}
	if err := os.WriteFile(markerPath, []byte(`{"version":2,"action":"migrate","sourceDataRoot":"C:\\untrusted`), 0o600); err != nil {
		t.Fatalf("write corrupt migration marker: %v", err)
	}

	app := NewApp()
	app.startup(t.Context())
	defer app.shutdown(t.Context())
	if status := app.GetStartupStatus(); status.Phase != StartupPhaseStorageRecovery || status.StorageLocations == nil || !status.StorageLocations.PendingMigration {
		t.Fatalf("corrupt marker recovery status = %#v", status)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) {
		return newData, nil
	}
	if selected := app.SelectStorageLocation(string(StorageLocationDataRoot)); selected.Error != nil {
		t.Fatalf("select root after corrupt marker: %#v", selected)
	}
	if applied := app.ApplyStorageLocations(); applied.Error != nil || !applied.RestartRequired {
		t.Fatalf("replace corrupt marker: %#v", applied)
	}
	app.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	defer restarted.shutdown(t.Context())
	if status := restarted.GetStartupStatus(); !status.Ready || status.DataDir != filepath.Clean(newData) {
		t.Fatalf("corrupt marker switch startup status = %#v", status)
	}
}

func TestEnvironmentOverrideRemainsAuthoritativeWithInvalidPendingMarker(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	envRoot := t.TempDir()
	db, err := database.Open(t.Context(), filepath.Join(envRoot, "atlasnote.db"))
	if err != nil {
		t.Fatalf("create environment database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close environment database: %v", err)
	}
	t.Setenv("ATLAS_NOTE_DATA_DIR", envRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	markerPath, err := config.PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatalf("pending marker path: %v", err)
	}
	legacy := config.PendingStorageLocationMigration{
		Version: 1, ID: "environment-invalid-marker",
		SourceDataRoot: envRoot, TargetDataRoot: filepath.Join(envRoot, "nested-target"),
		SourceBackupRoot: envRoot, TargetBackupRoot: envRoot,
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal pending migration: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		t.Fatalf("create marker directory: %v", err)
	}
	if err := os.WriteFile(markerPath, encoded, 0o600); err != nil {
		t.Fatalf("write pending migration: %v", err)
	}

	app := NewApp()
	app.startup(t.Context())
	defer app.shutdown(t.Context())
	status := app.GetStartupStatus()
	if !status.Ready || status.Phase != StartupPhaseReady || status.StorageLocations == nil || !status.StorageLocations.EnvironmentOverride {
		t.Fatalf("environment override startup status = %#v", status)
	}
	statusResult := app.GetStorageLocationStatus()
	if statusResult.Status == nil || statusResult.Error == nil {
		t.Fatalf("invalid pending marker status = %#v", statusResult)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("environment override removed pending marker: %v", err)
	}
}

func TestAppUsesConfiguredSeparateBackupRoot(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	dataRoot := filepath.Join(t.TempDir(), "data")
	backupRoot := filepath.Join(t.TempDir(), "archive")
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot}); err != nil {
		t.Fatalf("save locations: %v", err)
	}
	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatalf("app is not ready: %#v", status)
	}
	created, err := app.CreateNote(note.CreateInput{Title: "外部バックアップ", Content: "本文"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	result := app.CreateAutomaticBackup()
	if result.Error != nil || result.Backup == nil {
		t.Fatalf("create backup: %#v", result)
	}
	archiveManifest := filepath.Join(backupRoot, ".atlasnote-backups", app.activeSpace.ID, "generations", result.Backup.ID, "manifest.json")
	if _, err := os.Stat(archiveManifest); err != nil {
		t.Fatalf("configured archive missing for %s: %v", created.ID, err)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, ".atlasnote-backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup leaked into data root: %v", err)
	}
	app.openDirectory = func(_ context.Context, _ runtime.OpenDialogOptions) (string, error) { return dataRoot, nil }
	if selected := app.SelectStorageLocation(string(StorageLocationBackupRoot)); selected.Error != nil {
		t.Fatalf("selecting the data root as the default archive failed: %#v", selected)
	}
}
