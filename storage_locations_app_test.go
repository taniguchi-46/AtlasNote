package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"atlasnote/internal/config"
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
