//go:build windows

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsMigrationDataStageSharingFailureResumesAfterHandleRelease(t *testing.T) {
	configureWindowsMigrationTest(t)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := filepath.Join(t.TempDir(), "data")
	targetBackup := t.TempDir()
	writeWindowsMigrationData(t, sourceData)

	migration := PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "windows-data-stage-sharing",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
		DataPlan: PendingStorageMigrationPlanCopyRequired, BackupPlan: PendingStorageMigrationPlanUnchanged,
		Phase: PendingStorageMigrationPhasePrepared,
	}
	stage := migrationStagePathForData(migration)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatalf("create data stage: %v", err)
	}
	if err := writeMigrationStageMarker(stage, migrationStageMarkerForData(migration)); err != nil {
		t.Fatalf("write data stage marker: %v", err)
	}
	partial := filepath.Join(stage, "notes", "partial.md")
	writeTestMigrationFile(t, stage, filepath.Join("notes", "partial.md"), "partial")
	_, release := openWindowsSharingHandle(t, partial, windows.FILE_SHARE_READ)

	if err := SavePendingStorageLocationMigration(migration); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		release()
		t.Fatalf("sharing-blocked data stage apply = %v, %v, want failure", completed, err)
	}
	assertWindowsMigrationFailureState(t, migration, stage, partial)

	release()
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed data migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(targetData, "notes", "note.md")); err != nil {
		t.Fatalf("copied data missing after retry: %v", err)
	}
	assertWindowsMigrationFinished(t, stage, targetData)
}

func TestWindowsMigrationSeparateBackupStageSharingFailureResumesAfterHandleRelease(t *testing.T) {
	configureWindowsMigrationTest(t)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetBackup := filepath.Join(t.TempDir(), "backup")
	writeWindowsMigrationData(t, sourceData)
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generation", "payload.db"), "payload")

	migration := PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "windows-separate-backup-sharing",
		SourceDataRoot: sourceData, TargetDataRoot: sourceData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
		DataPlan: PendingStorageMigrationPlanUnchanged, BackupPlan: PendingStorageMigrationPlanCopyRequired,
		Phase: PendingStorageMigrationPhaseDataPlaced,
	}
	stage := migrationStagePathForBackup(migration)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatalf("create backup stage: %v", err)
	}
	if err := writeMigrationStageMarker(stage, migrationStageMarkerForBackup(migration)); err != nil {
		t.Fatalf("write backup stage marker: %v", err)
	}
	partial := filepath.Join(stage, "space", "generations", "partial", "payload.db")
	writeTestMigrationFile(t, stage, filepath.Join("space", "generations", "partial", "payload.db"), "partial")
	_, release := openWindowsSharingHandle(t, partial, windows.FILE_SHARE_READ)

	if err := SavePendingStorageLocationMigration(migration); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		release()
		t.Fatalf("sharing-blocked separate backup apply = %v, %v, want failure", completed, err)
	}
	assertWindowsMigrationFailureState(t, migration, stage, partial)

	release()
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed separate backup migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups", "space", "generation", "payload.db")); err != nil {
		t.Fatalf("copied backup missing after retry: %v", err)
	}
	assertWindowsMigrationFinished(t, stage, targetBackup)
}

func TestWindowsMigrationSharedTargetBackupStageSharingFailureResumesAfterHandleRelease(t *testing.T) {
	configureWindowsMigrationTest(t)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetRoot := filepath.Join(t.TempDir(), "target")
	writeWindowsMigrationData(t, sourceData)
	writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generation", "payload.db"), "payload")
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		t.Fatalf("create shared target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetRoot, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write shared target database: %v", err)
	}

	migration := PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "windows-shared-backup-sharing",
		SourceDataRoot: sourceData, TargetDataRoot: targetRoot,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetRoot,
		DataPlan: PendingStorageMigrationPlanCopyRequired, BackupPlan: PendingStorageMigrationPlanCopyRequired,
		Phase: PendingStorageMigrationPhaseDataPlaced,
	}
	if err := writeMigrationStageMarker(targetRoot, migrationStageMarkerForData(migration)); err != nil {
		t.Fatalf("write placed data marker: %v", err)
	}
	stage := migrationStagePathForBackup(migration)
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatalf("create shared backup stage: %v", err)
	}
	if err := writeMigrationStageMarker(stage, migrationStageMarkerForBackup(migration)); err != nil {
		t.Fatalf("write shared backup stage marker: %v", err)
	}
	partial := filepath.Join(stage, "space", "generations", "partial", "payload.db")
	writeTestMigrationFile(t, stage, filepath.Join("space", "generations", "partial", "payload.db"), "partial")
	_, release := openWindowsSharingHandle(t, partial, windows.FILE_SHARE_READ)

	if err := SavePendingStorageLocationMigration(migration); err != nil {
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		release()
		t.Fatalf("sharing-blocked shared backup apply = %v, %v, want failure", completed, err)
	}
	assertWindowsMigrationFailureState(t, migration, stage, partial)
	if _, err := os.Stat(filepath.Join(targetRoot, migrationStageMarkerFile)); err != nil {
		t.Fatalf("placed data marker was not retained: %v", err)
	}

	release()
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed shared backup migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, ".atlasnote-backups", "space", "generation", "payload.db")); err != nil {
		t.Fatalf("copied shared backup missing after retry: %v", err)
	}
	assertWindowsMigrationFinished(t, stage, targetRoot)
}

func TestWindowsMigrationSourceReadSharingFailureRetainsStageAndResumes(t *testing.T) {
	configureWindowsMigrationTest(t)
	sourceData := t.TempDir()
	sourceBackup := t.TempDir()
	targetData := filepath.Join(t.TempDir(), "data")
	targetBackup := t.TempDir()
	writeWindowsMigrationData(t, sourceData)

	migration := PendingStorageLocationMigration{
		Version: pendingStorageMigrationVersion, ID: "windows-source-read-sharing",
		SourceDataRoot: sourceData, TargetDataRoot: targetData,
		SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
		DataPlan: PendingStorageMigrationPlanCopyRequired, BackupPlan: PendingStorageMigrationPlanUnchanged,
		Phase: PendingStorageMigrationPhasePrepared,
	}
	lockedSource := filepath.Join(sourceData, "atlasnote.db")
	_, release := openWindowsSharingHandle(t, lockedSource, 0)

	if err := SavePendingStorageLocationMigration(migration); err != nil {
		release()
		t.Fatalf("save pending migration: %v", err)
	}
	completed, err := ApplyPendingStorageLocationMigration(context.Background())
	if completed || err == nil {
		release()
		t.Fatalf("source-read-blocked apply = %v, %v, want failure", completed, err)
	}
	stage := migrationStagePathForData(migration)
	assertWindowsMigrationFailureState(t, migration, stage, filepath.Join(stage, migrationStageMarkerFile))

	release()
	completed, err = ApplyPendingStorageLocationMigration(context.Background())
	if !completed || err != nil {
		t.Fatalf("resumed source-read migration = %v, %v", completed, err)
	}
	if _, err := os.Stat(filepath.Join(targetData, "notes", "note.md")); err != nil {
		t.Fatalf("copied data missing after source-read retry: %v", err)
	}
	assertWindowsMigrationFinished(t, stage, targetData)
}

func TestWindowsMigrationRejectsStageMarkerAndSiblingWithoutMutation(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, migration PendingStorageLocationMigration, stage string)
	}{
		{
			name: "mismatched-marker",
			setup: func(t *testing.T, migration PendingStorageLocationMigration, stage string) {
				t.Helper()
				if err := os.MkdirAll(stage, 0o700); err != nil {
					t.Fatalf("create marker stage: %v", err)
				}
				marker := migrationStageMarkerForBackup(migration)
				marker.OperationID = "other-operation"
				if err := writeMigrationStageMarker(stage, marker); err != nil {
					t.Fatalf("write mismatched marker: %v", err)
				}
				writeTestMigrationFile(t, stage, "partial.txt", "preserve")
			},
		},
		{
			name: "look-alike-sibling",
			setup: func(t *testing.T, migration PendingStorageLocationMigration, stage string) {
				t.Helper()
				sibling := stage + "-other"
				if err := os.MkdirAll(sibling, 0o700); err != nil {
					t.Fatalf("create sibling stage: %v", err)
				}
				writeTestMigrationFile(t, sibling, "partial.txt", "preserve")
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			configureWindowsMigrationTest(t)
			sourceData := t.TempDir()
			sourceBackup := t.TempDir()
			targetBackup := filepath.Join(t.TempDir(), "backup")
			writeWindowsMigrationData(t, sourceData)
			writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generation", "payload.db"), "payload")
			migration := PendingStorageLocationMigration{
				Version: pendingStorageMigrationVersion, ID: "windows-reject-stage-" + testCase.name,
				SourceDataRoot: sourceData, TargetDataRoot: sourceData,
				SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
				DataPlan: PendingStorageMigrationPlanUnchanged, BackupPlan: PendingStorageMigrationPlanCopyRequired,
				Phase: PendingStorageMigrationPhaseDataPlaced,
			}
			stage := migrationStagePathForBackup(migration)
			testCase.setup(t, migration, stage)
			if err := SavePendingStorageLocationMigration(migration); err != nil {
				t.Fatalf("save pending migration: %v", err)
			}

			completed, err := ApplyPendingStorageLocationMigration(context.Background())
			if completed || err == nil {
				t.Fatalf("unsafe stage apply = %v, %v, want rejection", completed, err)
			}
			assertWindowsPendingMigration(t, migration)
			if testCase.name == "look-alike-sibling" {
				if _, err := os.Lstat(stage + "-other"); err != nil {
					t.Fatalf("look-alike sibling changed after rejection: %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("backup target changed after rejection: %v", err)
			}
		})
	}
}

func TestWindowsMigrationRejectsStageLinksWithoutMutation(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, migration PendingStorageLocationMigration, stage string)
	}{
		{
			name: "stage-link",
			setup: func(t *testing.T, migration PendingStorageLocationMigration, stage string) {
				t.Helper()
				if err := os.Symlink(migration.SourceBackupRoot, stage); err != nil {
					t.Skipf("symbolic links are unavailable: %v", err)
				}
			},
		},
		{
			name: "marker-link",
			setup: func(t *testing.T, migration PendingStorageLocationMigration, stage string) {
				t.Helper()
				if err := os.MkdirAll(stage, 0o700); err != nil {
					t.Fatalf("create linked marker stage: %v", err)
				}
				markerPath := filepath.Join(t.TempDir(), migrationStageMarkerFile)
				if err := writeMigrationStageMarker(filepath.Dir(markerPath), migrationStageMarkerForBackup(migration)); err != nil {
					t.Fatalf("write external marker: %v", err)
				}
				if err := os.Symlink(markerPath, filepath.Join(stage, migrationStageMarkerFile)); err != nil {
					t.Skipf("symbolic links are unavailable: %v", err)
				}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			configureWindowsMigrationTest(t)
			sourceData := t.TempDir()
			sourceBackup := t.TempDir()
			targetBackup := filepath.Join(t.TempDir(), "backup")
			writeWindowsMigrationData(t, sourceData)
			writeTestMigrationFile(t, sourceBackup, filepath.Join(".atlasnote-backups", "space", "generation", "payload.db"), "payload")
			migration := PendingStorageLocationMigration{
				Version: pendingStorageMigrationVersion, ID: "windows-reject-link-" + testCase.name,
				SourceDataRoot: sourceData, TargetDataRoot: sourceData,
				SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
				DataPlan: PendingStorageMigrationPlanUnchanged, BackupPlan: PendingStorageMigrationPlanCopyRequired,
				Phase: PendingStorageMigrationPhaseDataPlaced,
			}
			stage := migrationStagePathForBackup(migration)
			testCase.setup(t, migration, stage)
			if err := SavePendingStorageLocationMigration(migration); err != nil {
				t.Fatalf("save pending migration: %v", err)
			}

			completed, err := ApplyPendingStorageLocationMigration(context.Background())
			if completed || err == nil {
				t.Fatalf("linked stage apply = %v, %v, want rejection", completed, err)
			}
			assertWindowsPendingMigration(t, migration)
			if _, err := os.Lstat(stage); err != nil {
				t.Fatalf("linked stage changed after rejection: %v", err)
			}
			if testCase.name == "marker-link" {
				if _, err := os.Lstat(filepath.Join(stage, migrationStageMarkerFile)); err != nil {
					t.Fatalf("linked marker changed after rejection: %v", err)
				}
			}
			if _, err := os.Stat(filepath.Join(targetBackup, ".atlasnote-backups")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("backup target changed after link rejection: %v", err)
			}
		})
	}
}

func configureWindowsMigrationTest(t *testing.T) {
	t.Helper()
	t.Setenv(storageLocationsPathEnv, filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json"))
}

func writeWindowsMigrationData(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "atlasnote.db"), []byte("database"), 0o600); err != nil {
		t.Fatalf("write migration database: %v", err)
	}
	writeTestMigrationFile(t, root, filepath.Join("notes", "note.md"), "note")
}

func openWindowsSharingHandle(t *testing.T, path string, shareMode uint32) (windows.Handle, func()) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("encode Windows path: %v", err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		shareMode,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("open Windows sharing handle for %q: %v", path, err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if err := windows.CloseHandle(handle); err != nil {
			t.Errorf("close Windows sharing handle: %v", err)
		}
	}
	t.Cleanup(release)
	return handle, release
}

func assertWindowsMigrationFailureState(t *testing.T, migration PendingStorageLocationMigration, stage string, preserved string) {
	t.Helper()
	assertWindowsPendingMigration(t, migration)
	if stage != "" {
		if _, err := os.Lstat(stage); err != nil {
			t.Fatalf("stage missing after failure: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(stage, migrationStageMarkerFile)); err != nil {
			t.Fatalf("owned stage marker missing after failure: %v", err)
		}
	}
	if preserved != "" {
		if _, err := os.Lstat(preserved); err != nil {
			t.Fatalf("preserved stage entry missing after failure: %v", err)
		}
	}
}

func assertWindowsPendingMigration(t *testing.T, migration PendingStorageLocationMigration) {
	t.Helper()
	pending, err := LoadPendingStorageLocationMigration()
	if err != nil {
		t.Fatalf("pending migration was lost after failure: %v", err)
	}
	if pending.ID != migration.ID || pending.Phase != migration.Phase {
		t.Fatalf("pending after failure = %#v, want ID %q phase %q", pending, migration.ID, migration.Phase)
	}
	pendingPath, err := PendingStorageLocationMigrationPath()
	if err != nil {
		t.Fatalf("pending migration path: %v", err)
	}
	if _, err := os.Lstat(pendingPath); err != nil {
		t.Fatalf("pending marker missing after failure: %v", err)
	}
}

func assertWindowsMigrationFinished(t *testing.T, stage string, targetRoot string) {
	t.Helper()
	if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stage remains after successful retry: %v", err)
	}
	if _, err := LoadPendingStorageLocationMigration(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending marker remains after successful retry: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(targetRoot, migrationStageMarkerFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("data marker remains after successful retry: %v", err)
	}
	if locations, err := LoadStorageLocations(); err != nil || locations.DataRoot == "" {
		t.Fatalf("committed storage locations = %#v, %v", locations, err)
	}
}
