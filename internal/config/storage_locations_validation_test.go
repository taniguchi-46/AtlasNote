package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRootValidationErrorRetainsClassificationAndCause(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write invalid root: %v", err)
	}
	_, err := ProbeDataRoot(root)
	var validationErr *RootValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("validation error = %v, want RootValidationError", err)
	}
	if validationErr.Code != RootErrorNotDirectory || validationErr.Stage != RootValidationStageRootLookup || validationErr.Role != RootValidationRoleData {
		t.Fatalf("validation error = %#v", validationErr)
	}
	if !errors.Is(err, ErrRootInvalid) || !errors.Is(err, validationErr.Cause) {
		t.Fatalf("validation error does not preserve sentinels: %v", err)
	}

	overlapRoot := t.TempDir()
	overlapping := ValidateStorageLocations(StorageLocations{
		Version:    storageLocationsVersion,
		DataRoot:   filepath.Join(overlapRoot, "data"),
		BackupRoot: filepath.Join(overlapRoot, "data", "archive"),
	})
	var overlapErr *RootValidationError
	if !errors.As(overlapping, &overlapErr) || overlapErr.Code != RootErrorOverlappingRoots || overlapErr.Stage != RootValidationStageConfig {
		t.Fatalf("overlap validation error = %#v, %v", overlapErr, overlapping)
	}
	if !errors.Is(overlapping, ErrLocationsInvalid) {
		t.Fatalf("overlap validation error = %v, want ErrLocationsInvalid", overlapping)
	}
}

func TestRootErrorOSNumberOfExtractsWrappedErrno(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", syscall.Errno(13))
	if got := RootErrorOSNumberOf(err); got != 13 {
		t.Fatalf("OS error number = %d, want 13", got)
	}
}

func TestPlacedDataMarkerWithoutAtlasDataIsRejectedForEveryRetryPhase(t *testing.T) {
	phases := []PendingStorageMigrationPhase{
		PendingStorageMigrationPhasePrepared,
		PendingStorageMigrationPhaseDataPlaced,
		PendingStorageMigrationPhaseBackupPlaced,
		PendingStorageMigrationPhaseConfigCommitted,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
			t.Setenv(storageLocationsPathEnv, configFile)
			sourceData := t.TempDir()
			targetData := t.TempDir()
			sourceBackup := t.TempDir()
			targetBackup := t.TempDir()
			if err := os.WriteFile(filepath.Join(sourceData, "atlasnote.db"), []byte("source"), 0o600); err != nil {
				t.Fatalf("write source data: %v", err)
			}
			migration := PendingStorageLocationMigration{
				Version: pendingStorageMigrationVersion, ID: "missing-placed-data-" + string(phase),
				SourceDataRoot: sourceData, TargetDataRoot: targetData,
				SourceBackupRoot: sourceBackup, TargetBackupRoot: targetBackup,
				DataPlan: PendingStorageMigrationPlanCopyRequired, BackupPlan: PendingStorageMigrationPlanUnchanged,
				Phase: phase,
			}
			if err := writeMigrationStageMarker(targetData, migrationStageMarkerForData(migration)); err != nil {
				t.Fatalf("write placed data marker: %v", err)
			}
			if err := SavePendingStorageLocationMigration(migration); err != nil {
				t.Fatalf("save migration: %v", err)
			}
			completed, err := ApplyPendingStorageLocationMigration(context.Background())
			if completed || err == nil || !errors.Is(err, ErrRootInvalid) {
				t.Fatalf("marker-only retry = %v, %v", completed, err)
			}
			pending, loadErr := LoadPendingStorageLocationMigration()
			if loadErr != nil || pending.Phase != phase || pending.ID != migration.ID {
				t.Fatalf("pending migration changed after rejection = %#v, %v", pending, loadErr)
			}
			if _, statErr := os.Stat(filepath.Join(targetData, migrationStageMarkerFile)); statErr != nil {
				t.Fatalf("placed data marker was removed after rejection: %v", statErr)
			}
		})
	}
}
