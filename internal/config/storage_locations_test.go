package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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
