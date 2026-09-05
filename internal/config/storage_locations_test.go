package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTestMigrationFile(t *testing.T, root string, relative string, contents string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create test migration directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test migration file: %v", err)
	}
}

func TestStorageLocationsRoundTripIsAtomicAndVersioned(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "nested", "storage-locations.json")
	dataRoot := filepath.Join(t.TempDir(), "data")
	backupRoot := filepath.Join(t.TempDir(), "backup")
	locations := StorageLocations{Version: storageLocationsVersion, DataRoot: dataRoot, BackupRoot: backupRoot}
	if err := SaveStorageLocationsTo(configFile, locations); err != nil {
		t.Fatalf("save locations: %v", err)
	}
	loaded, err := LoadStorageLocationsFrom(configFile)
	if err != nil {
		t.Fatalf("load locations: %v", err)
	}
	if loaded != locations {
		t.Fatalf("loaded locations = %#v, want %#v", loaded, locations)
	}
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("config file was not committed: %v", err)
	}
}

func TestProbeDataRootRejectsUnrelatedNonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}
	_, err := ProbeDataRoot(root)
	if !errors.Is(err, ErrRootInvalid) {
		t.Fatalf("probe error = %v, want ErrRootInvalid", err)
	}
	if got := RootErrorCodeOf(err); got != RootErrorUnrelatedContent {
		t.Fatalf("probe error code = %q, want %q", got, RootErrorUnrelatedContent)
	}
}

func TestProbeDataRootAcceptsExistingAtlasRootAndEmptyTarget(t *testing.T) {
	existing := t.TempDir()
	if err := os.WriteFile(filepath.Join(existing, "atlasnote.db"), []byte("sqlite"), 0o600); err != nil {
		t.Fatalf("write database marker: %v", err)
	}
	probe, err := ProbeDataRoot(existing)
	if err != nil || probe.Kind != RootExisting || !probe.HasAtlasData {
		t.Fatalf("existing probe = %#v, %v", probe, err)
	}
	empty := filepath.Join(t.TempDir(), "new-root")
	probe, err = ProbeDataRoot(empty)
	if err != nil || probe.Kind != RootEmpty || probe.Exists {
		t.Fatalf("empty probe = %#v, %v", probe, err)
	}
}

func TestProbeDataRootIgnoresBootstrapFileWhenCheckingForEmptyRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, storageLocationsFile), []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatalf("write bootstrap file: %v", err)
	}
	probe, err := ProbeDataRoot(root)
	if err != nil || probe.Kind != RootEmpty || !probe.Exists {
		t.Fatalf("bootstrap-only probe = %#v, %v", probe, err)
	}
}

func TestLoadStorageLocationsForRecoverySkipsRootProbe(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), storageLocationsFile)
	root := filepath.Dir(configFile)
	locations := StorageLocations{Version: storageLocationsVersion, DataRoot: filepath.Join(root, "missing-data"), BackupRoot: filepath.Join(root, "missing-backup")}
	if err := SaveStorageLocationsTo(configFile, locations); err != nil {
		t.Fatalf("save locations: %v", err)
	}
	t.Setenv(storageLocationsPathEnv, configFile)
	loaded, err := LoadStorageLocationsForRecovery()
	if err != nil || loaded != locations {
		t.Fatalf("recovery locations = %#v, %v", loaded, err)
	}
}

func TestStorageLocationsPathIsIndependentFromDefaultDataRoot(t *testing.T) {
	defaultRoot := filepath.Join(t.TempDir(), "documents", "AtlasNote")
	t.Setenv(defaultDataRootEnv, defaultRoot)
	t.Setenv(storageLocationsPathEnv, "")
	path, err := StorageLocationsPath()
	if err != nil {
		t.Fatalf("storage locations path: %v", err)
	}
	if filepath.Clean(filepath.Dir(path)) == filepath.Clean(defaultRoot) {
		t.Fatalf("storage locations path = %q, unexpectedly inside data root %q", path, defaultRoot)
	}
}

func TestResolveStorageLocationsHonorsEnvironmentOverride(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "env-data")
	configFile := filepath.Join(t.TempDir(), "storage-locations.json")
	t.Setenv(dataDirEnv, dataRoot)
	t.Setenv(storageLocationsPathEnv, configFile)
	resolution, err := ResolveStorageLocations()
	if err != nil {
		t.Fatalf("resolve locations: %v", err)
	}
	if !resolution.Environment || resolution.SetupRequired || resolution.Source != LocationSourceEnvironment {
		t.Fatalf("resolution = %#v", resolution)
	}
	if resolution.Locations.DataRoot != filepath.Clean(dataRoot) || resolution.Locations.BackupRoot != filepath.Clean(dataRoot) {
		t.Fatalf("resolved paths = %#v", resolution.Locations)
	}
}

func TestApplyPendingStorageLocationMigrationCopiesDataWithoutRemovingSource(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, "notes"), 0o700); err != nil {
		t.Fatalf("create source notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "notes", "note.md"), []byte("body"), 0o600); err != nil {
		t.Fatalf("write source note: %v", err)
	}
	target := filepath.Join(t.TempDir(), "target")
	backupSource := t.TempDir()
	if err := os.MkdirAll(filepath.Join(backupSource, ".atlasnote-backups"), 0o700); err != nil {
		t.Fatalf("create source backup: %v", err)
	}
	backupTarget := filepath.Join(t.TempDir(), "backup-target")
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "migration-test",
		SourceDataRoot: source, TargetDataRoot: target,
		SourceBackupRoot: backupSource, TargetBackupRoot: backupTarget,
	}); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if err != nil || !completed {
		t.Fatalf("apply migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(target, "notes", "note.md")); err != nil {
		t.Fatalf("copied note missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "notes", "note.md")); err != nil {
		t.Fatalf("source note removed: %v", err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending migration remains: %v", err)
	}
}

func TestApplyPendingStorageLocationMigrationMovesInternalArchiveWithDataRoot(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, "notes"), 0o700); err != nil {
		t.Fatalf("create source notes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, ".atlasnote-backups", "space"), 0o700); err != nil {
		t.Fatalf("create source archive: %v", err)
	}
	// The folder picker always returns an existing directory. Keep this target
	// empty so the test reproduces selecting a newly created desktop folder.
	target := t.TempDir()
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "internal-archive-test",
		SourceDataRoot: source, TargetDataRoot: target,
		SourceBackupRoot: source, TargetBackupRoot: target,
	}); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if err != nil || !completed {
		t.Fatalf("apply migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(target, "atlasnote.db")); err != nil {
		t.Fatalf("copied database missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".atlasnote-backups", "space")); err != nil {
		t.Fatalf("internal archive missing after migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "atlasnote.db")); err != nil {
		t.Fatalf("source database was removed: %v", err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending migration remains: %v", err)
	}
}

func TestMigrationToSharedRootUsesSeparateBackupSourceWithoutMergingInternalArchive(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	writeTestMigrationFile(t, sourceData, filepath.Join(".atlasnote-backups", "stale", "manifest.json"), "stale data archive")
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "current", "manifest.json"), "current backup archive")

	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "shared-target-separate-backup",
		SourceDataRoot: sourceData, TargetDataRoot: target,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: target,
	}); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if err != nil || !completed {
		t.Fatalf("apply migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(target, ".atlasnote-backups", "current", "manifest.json")); err != nil {
		t.Fatalf("current backup was not placed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".atlasnote-backups", "stale", "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale internal archive was merged: %v", err)
	}
}

func TestApplyPendingStorageLocationMigrationKeepsNonEmptyTarget(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "non-empty-target-test",
		SourceDataRoot: source, TargetDataRoot: target,
		SourceBackupRoot: source, TargetBackupRoot: target,
	}); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.DataPlan != PendingStorageMigrationPlanCopyRequired || pending.BackupPlan != PendingStorageMigrationPlanCopyRequired || pending.Phase != PendingStorageMigrationPhasePrepared {
		t.Fatalf("persisted migration decision = %#v, %v", pending, err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || !errors.Is(err, ErrRootInvalid) {
		t.Fatalf("apply migration = %v, %v, want ErrRootInvalid", completed, err)
	}
	if _, err := os.Stat(filepath.Join(target, "keep.txt")); err != nil {
		t.Fatalf("target file was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "atlasnote.db")); err != nil {
		t.Fatalf("source database was removed: %v", err)
	}
	if _, err := LoadPendingStorageLocationMigration(); err != nil {
		t.Fatalf("pending migration was removed: %v", err)
	}
}

func TestV2MigrationUsesPersistedOpenExistingPlansWithoutMerging(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("source"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetData, "atlasnote.db"), []byte("target"), 0o600); err != nil {
		t.Fatalf("write target database: %v", err)
	}
	writeTestMigrationFile(t, sourceData, filepath.Join("notes", "source.md"), "source note")
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "source", "manifest.json"), "source backup")
	writeTestMigrationFile(t, targetBackup, filepath.Join(".atlasnote-backups", "space", "target", "manifest.json"), "target backup")

	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "open-existing-test",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}); err != nil {
		t.Fatalf("save open-existing migration: %v", err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.DataPlan != PendingStorageMigrationPlanOpenExisting || pending.BackupPlan != PendingStorageMigrationPlanOpenExisting {
		t.Fatalf("persisted open-existing plan = %#v, %v", pending, err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("apply open-existing migration = %v, %v", completed, err)
	}
	if got, err := os.ReadFile(filepath.Join(targetData, "atlasnote.db")); err != nil || string(got) != "target" {
		t.Fatalf("existing target was changed: %q, %v", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(targetData, "notes", "source.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source data was merged into existing target: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(targetBackup, ".atlasnote-backups", "space", "target", "manifest.json")); err != nil || string(got) != "target backup" {
		t.Fatalf("existing backup target was changed: %q, %v", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups", "space", "source", "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source backup was merged into existing target: %v", err)
	}
}

func TestStorageLocationsRejectNestedDistinctRoots(t *testing.T) {
	dataRoot := t.TempDir()
	backupRoot := filepath.Join(dataRoot, "archive")
	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		t.Fatalf("create nested archive: %v", err)
	}
	if err := SaveStorageLocationsTo(filepath.Join(t.TempDir(), "storage-locations.json"), StorageLocations{
		Version: storageLocationsVersion, DataRoot: dataRoot, BackupRoot: backupRoot,
	}); !errors.Is(err, ErrLocationsInvalid) {
		t.Fatalf("nested roots error = %v, want ErrLocationsInvalid", err)
	}
}

func TestPendingStorageLocationMigrationRejectsUnsafeID(t *testing.T) {
	err := validatePendingMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "../escape",
		SourceDataRoot: t.TempDir(), TargetDataRoot: t.TempDir(),
		SourceBackupRoot: t.TempDir(), TargetBackupRoot: t.TempDir(),
	})
	if !errors.Is(err, ErrLocationsInvalid) {
		t.Fatalf("unsafe migration ID error = %v, want ErrLocationsInvalid", err)
	}
}

func TestV2MigrationResumesBackupAfterDataPlacementProgressFailure(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	writeTestMigrationFile(t, sourceData, filepath.Join("notes", "note.md"), "本文")
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generations", "one", "manifest.json"), "manifest-one")
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generations", "two", "payload.db"), "payload-two")

	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "resume-after-data",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}); err != nil {
		t.Fatalf("save v2 migration: %v", err)
	}
	failed := true
	pendingStorageMigrationTestHook = func(event string) error {
		if failed && event == string(PendingStorageMigrationPhaseDataPlaced) {
			failed = false
			return errors.New("test data progress failure")
		}
		return nil
	}
	t.Cleanup(func() { pendingStorageMigrationTestHook = nil })
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		t.Fatalf("first migration attempt = %v, %v, want progress failure", completed, err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.Version != pendingStorageMigrationVersion || pending.Phase != PendingStorageMigrationPhasePrepared {
		t.Fatalf("pending after progress failure = %#v, %v", pending, err)
	}
	if _, err := os.Stat(filepath.Join(targetData, "notes", "note.md")); err != nil {
		t.Fatalf("data was not placed before injected failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup unexpectedly placed before retry: %v", err)
	}

	pendingStorageMigrationTestHook = nil
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed migration = %v, %v", completed, err)
	}
	for relative, want := range map[string]string{
		filepath.Join(".atlasnote-backups", "space", "generations", "one", "manifest.json"): "manifest-one",
		filepath.Join(".atlasnote-backups", "space", "generations", "two", "payload.db"):    "payload-two",
	} {
		got, readErr := os.ReadFile(filepath.Join(targetBackup, relative))
		if readErr != nil || string(got) != want {
			t.Fatalf("copied backup %s = %q, %v", relative, string(got), readErr)
		}
	}
	if _, err := os.Stat(filepath.Join(sourceData, "notes", "note.md")); err != nil {
		t.Fatalf("source data changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceBackup, ".atlasnote-backups", "space", "generations", "two", "payload.db")); err != nil {
		t.Fatalf("source backup changed: %v", err)
	}
	locations, err := LoadStorageLocations()
	if err != nil || locations.DataRoot != filepath.Clean(targetData) || locations.BackupRoot != filepath.Clean(targetBackup) {
		t.Fatalf("committed locations = %#v, %v", locations, err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed migration marker remains: %v", err)
	}
}

func TestV2MigrationResumesAfterBackupPlacementProgressFailure(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generations", "one", "manifest.json"), "manifest")
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "resume-after-backup",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}); err != nil {
		t.Fatalf("save v2 migration: %v", err)
	}
	failed := true
	pendingStorageMigrationTestHook = func(event string) error {
		if failed && event == string(PendingStorageMigrationPhaseBackupPlaced) {
			failed = false
			return errors.New("test backup progress failure")
		}
		return nil
	}
	t.Cleanup(func() { pendingStorageMigrationTestHook = nil })
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		t.Fatalf("first migration attempt = %v, %v, want progress failure", completed, err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.Phase != PendingStorageMigrationPhaseDataPlaced {
		t.Fatalf("pending after backup progress failure = %#v, %v", pending, err)
	}
	if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups", "space", "generations", "one", "manifest.json")); err != nil {
		t.Fatalf("backup was not placed before injected failure: %v", err)
	}
	pendingStorageMigrationTestHook = nil
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed backup migration = %v, %v", completed, err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed migration marker remains: %v", err)
	}
}

func TestV2MigrationKeepsPendingUntilBootstrapCommitSucceeds(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generations", "one", "manifest.json"), "manifest")
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "resume-after-config",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}); err != nil {
		t.Fatalf("save v2 migration: %v", err)
	}
	failed := true
	pendingStorageMigrationTestHook = func(event string) error {
		if failed && event == "config-save" {
			failed = false
			return errors.New("test bootstrap save failure")
		}
		return nil
	}
	t.Cleanup(func() { pendingStorageMigrationTestHook = nil })
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		t.Fatalf("first migration attempt = %v, %v, want bootstrap failure", completed, err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.Phase != PendingStorageMigrationPhaseBackupPlaced {
		t.Fatalf("pending after bootstrap failure = %#v, %v", pending, err)
	}
	if _, err := LoadStorageLocations(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bootstrap unexpectedly committed: %v", err)
	}
	pendingStorageMigrationTestHook = nil
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed bootstrap migration = %v, %v", completed, err)
	}
}

func TestV2MigrationKeepsCommittedStateWhenPendingRemovalFails(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	if err := SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "resume-after-marker-removal",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}); err != nil {
		t.Fatalf("save v2 migration: %v", err)
	}
	failed := true
	pendingStorageMigrationTestHook = func(event string) error {
		if failed && event == "pending-remove" {
			failed = false
			return errors.New("test pending marker removal failure")
		}
		return nil
	}
	t.Cleanup(func() { pendingStorageMigrationTestHook = nil })
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		t.Fatalf("first migration attempt = %v, %v", completed, err)
	}
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil || pending.Phase != PendingStorageMigrationPhaseConfigCommitted {
		t.Fatalf("pending after marker removal failure = %#v, %v", pending, err)
	}
	locations, err := LoadStorageLocations()
	if err != nil || locations.DataRoot != filepath.Clean(targetData) {
		t.Fatalf("committed locations after marker removal failure = %#v, %v", locations, err)
	}

	pendingStorageMigrationTestHook = nil
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed marker cleanup = %v, %v", completed, err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending marker remains after cleanup retry: %v", err)
	}
}

func TestPendingMigrationReplacementDoesNotRemoveFixedPathDirectory(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	markerPath, err := PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatalf("pending marker path: %v", err)
	}
	if err := os.MkdirAll(markerPath, 0o700); err != nil {
		t.Fatalf("create marker directory: %v", err)
	}
	keepPath := filepath.Join(markerPath, "keep.txt")
	if err := os.WriteFile(keepPath, []byte("preserve"), 0o600); err != nil {
		t.Fatalf("write marker directory content: %v", err)
	}
	err = SavePendingStorageLocationMigration(PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "fixed-path-replacement",
		Action:         pendingStorageMigrationSwitch,
		TargetDataRoot: t.TempDir(), TargetBackupRoot: t.TempDir(),
	})
	if !errors.Is(err, ErrLocationsInvalid) {
		t.Fatalf("fixed path replacement error = %v, want ErrLocationsInvalid", err)
	}
	if contents, readErr := os.ReadFile(keepPath); readErr != nil || string(contents) != "preserve" {
		t.Fatalf("fixed path directory changed: %q, %v", string(contents), readErr)
	}
}

func TestLegacyMigrationWithExistingTargetDoesNotGuessCompletion(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	t.Setenv(storageLocationsPathEnv, configFile)
	sourceData := t.TempDir()
	targetData := t.TempDir()
	sourceBackup := t.TempDir()
	targetBackup := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("source"), 0o600); err != nil {
		t.Fatalf("write source database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetData, "atlasnote.db"), []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing target database: %v", err)
	}
	legacy := PendingStorageLocationMigration{
		Version: pendingStorageMigrationLegacyVersion, ID: "legacy-ambiguous",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy marker: %v", err)
	}
	writeTestMigrationFile(t, filepath.Dir(configFile), pendingStorageMigrationFile, string(encoded))
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || !errors.Is(err, ErrRootInvalid) {
		t.Fatalf("legacy ambiguous migration = %v, %v", completed, err)
	}
	got, err := os.ReadFile(filepath.Join(targetData, "atlasnote.db"))
	if err != nil || string(got) != "existing" {
		t.Fatalf("existing target changed: %q, %v", string(got), err)
	}
	pending, err := LoadPendingStorageLocationMigrationForRecovery()
	if err != nil || pending.Version != pendingStorageMigrationLegacyVersion {
		t.Fatalf("legacy marker after safe refusal = %#v, %v", pending, err)
	}
}
