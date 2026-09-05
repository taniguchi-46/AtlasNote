package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	pendingStorageMigrationLegacyVersion = 1
	pendingStorageMigrationVersion       = 2
	pendingStorageMigrationFile          = "storage-location-migration.json"
	pendingStorageMigrationMigrate       = "migrate"
	pendingStorageMigrationSwitch        = "switch"
	pendingStorageMigrationCancel        = "cancel"
	migrationStageMarkerFile             = ".atlasnote-migration-stage.json"
)

const (
	PendingStorageMigrationActionMigrate = pendingStorageMigrationMigrate
	PendingStorageMigrationActionSwitch  = pendingStorageMigrationSwitch
	PendingStorageMigrationActionCancel  = pendingStorageMigrationCancel
	PendingStorageMigrationVersion       = pendingStorageMigrationVersion
)

type PendingStorageMigrationPhase string

const (
	PendingStorageMigrationPhasePrepared        PendingStorageMigrationPhase = "prepared"
	PendingStorageMigrationPhaseDataPlaced      PendingStorageMigrationPhase = "data-placed"
	PendingStorageMigrationPhaseBackupPlaced    PendingStorageMigrationPhase = "backup-placed"
	PendingStorageMigrationPhaseConfigCommitted PendingStorageMigrationPhase = "config-committed"
)

type PendingStorageMigrationPlan string

const (
	PendingStorageMigrationPlanUnchanged    PendingStorageMigrationPlan = "unchanged"
	PendingStorageMigrationPlanOpenExisting PendingStorageMigrationPlan = "open-existing"
	PendingStorageMigrationPlanCopyRequired PendingStorageMigrationPlan = "copy-required"
)

type PendingStorageLocationMigration struct {
	Version          int                          `json:"version"`
	ID               string                       `json:"id"`
	Action           string                       `json:"action,omitempty"`
	SourceDataRoot   string                       `json:"sourceDataRoot"`
	TargetDataRoot   string                       `json:"targetDataRoot"`
	SourceBackupRoot string                       `json:"sourceBackupRoot"`
	TargetBackupRoot string                       `json:"targetBackupRoot"`
	DataPlan         PendingStorageMigrationPlan  `json:"dataPlan,omitempty"`
	BackupPlan       PendingStorageMigrationPlan  `json:"backupPlan,omitempty"`
	Phase            PendingStorageMigrationPhase `json:"phase,omitempty"`
}

type migrationStageMarker struct {
	Version     int    `json:"version"`
	OperationID string `json:"operationId"`
	Kind        string `json:"kind"`
	SourcePath  string `json:"sourcePath"`
	TargetPath  string `json:"targetPath"`
}

// pendingStorageMigrationTestHook is intentionally private. Tests use it to
// exercise the crash boundaries between a directory rename and its durable
// progress update without exposing a failure mechanism through Wails.
var pendingStorageMigrationTestHook func(string) error

func PendingStorageLocationMigrationPath() (string, error) {
	path, err := StorageLocationsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), pendingStorageMigrationFile), nil
}

func SavePendingStorageLocationMigration(migration PendingStorageLocationMigration) error {
	if migration.Version == pendingStorageMigrationLegacyVersion || migration.Version == 0 {
		prepared, err := preparePendingMigrationForWrite(migration)
		if err != nil {
			return err
		}
		migration = prepared
	} else {
		normalizePendingMigration(&migration)
		if migration.Version == pendingStorageMigrationVersion && migration.Action == pendingStorageMigrationMigrate && (migration.DataPlan == "" || migration.BackupPlan == "" || migration.Phase == "") {
			prepared, err := preparePendingMigrationForWrite(migration)
			if err == nil {
				migration = prepared
			} else if validatePendingMigrationPathsForPlanning(migration) == nil {
				// Callers that construct a v2 marker directly may be testing a
				// target that is already unsafe or non-empty. Keep the intent
				// durable as copy-required so Apply can reject it without ever
				// overwriting the target.
				migration.DataPlan = PendingStorageMigrationPlanCopyRequired
				migration.BackupPlan = PendingStorageMigrationPlanCopyRequired
				migration.Phase = PendingStorageMigrationPhasePrepared
			} else {
				return err
			}
		} else if migration.Version == pendingStorageMigrationVersion && migration.Action != pendingStorageMigrationMigrate && migration.Phase == "" {
			migration.Phase = PendingStorageMigrationPhasePrepared
		}
	}
	if err := validatePendingMigration(migration); err != nil {
		return err
	}
	path, err := PendingStorageLocationMigrationPath()
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(migration, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return writeAtomic(path, encoded)
}

func LoadPendingStorageLocationMigration() (PendingStorageLocationMigration, error) {
	path, err := PendingStorageLocationMigrationPath()
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	migration, err := loadPendingStorageLocationMigrationFrom(path)
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	if err := validatePendingMigration(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return migration, nil
}

// LoadPendingStorageLocationMigrationForRecovery reads only the safe shape of
// the marker. It intentionally does not validate whether a migrate action is
// still executable: recovery must remain available when an old marker no
// longer satisfies current path constraints.
func LoadPendingStorageLocationMigrationForRecovery() (PendingStorageLocationMigration, error) {
	path, err := PendingStorageLocationMigrationPath()
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return loadPendingStorageLocationMigrationFrom(path)
}

func ValidatePendingStorageLocationMigration(migration PendingStorageLocationMigration) error {
	return validatePendingMigration(migration)
}

// PreparePendingStorageLocationMigrationForRetry upgrades a legacy marker only
// when its target roots are still empty or missing. It deliberately does not
// reinterpret an existing target as an old completion state.
func PreparePendingStorageLocationMigrationForRetry(migration PendingStorageLocationMigration) (PendingStorageLocationMigration, error) {
	if migration.Version == pendingStorageMigrationLegacyVersion ||
		(migration.Action == pendingStorageMigrationMigrate &&
			(migration.DataPlan == "" || migration.BackupPlan == "" || migration.Phase == "")) {
		legacy := migration
		legacy.Version = pendingStorageMigrationLegacyVersion
		return prepareLegacyPendingMigrationForResume(legacy)
	}
	if err := validatePendingMigration(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return migration, nil
}

func loadPendingStorageLocationMigrationFrom(path string) (PendingStorageLocationMigration, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	if isUnsafeStoragePath(path, info) || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return PendingStorageLocationMigration{}, ErrLocationsInvalid
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var migration PendingStorageLocationMigration
	if err := decoder.Decode(&migration); err != nil {
		return PendingStorageLocationMigration{}, fmt.Errorf("%w: %v", ErrLocationsInvalid, err)
	}
	normalizePendingMigration(&migration)
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return PendingStorageLocationMigration{}, ErrLocationsInvalid
	}
	if err := validatePendingMigrationShape(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return migration, nil
}

func ApplyPendingStorageLocationMigration(ctx context.Context) (bool, error) {
	if strings.TrimSpace(os.Getenv(dataDirEnv)) != "" {
		// An environment-selected root is authoritative. Keep any UI-created
		// migration marker for the next run after the override is removed.
		return false, nil
	}
	migration, err := LoadPendingStorageLocationMigration()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if migration.Version == pendingStorageMigrationLegacyVersion {
		migration, err = prepareLegacyPendingMigrationForResume(migration)
		if err != nil {
			return false, err
		}
		if err := SavePendingStorageLocationMigration(migration); err != nil {
			return false, err
		}
	}
	switch migration.Action {
	case pendingStorageMigrationCancel:
		return applySimplePendingMigration(migration.SourceDataRoot, migration.SourceBackupRoot)
	case pendingStorageMigrationSwitch:
		return applySimplePendingMigration(migration.TargetDataRoot, migration.TargetBackupRoot)
	case pendingStorageMigrationMigrate:
		return applyMigratePendingStorageLocation(ctx, migration)
	default:
		return false, ErrLocationsInvalid
	}
}

func applySimplePendingMigration(dataRoot string, backupRoot string) (bool, error) {
	if err := runPendingStorageMigrationTestHook("config-save"); err != nil {
		return false, err
	}
	if err := SaveStorageLocations(StorageLocations{
		Version: storageLocationsVersion, DataRoot: dataRoot, BackupRoot: backupRoot,
	}); err != nil {
		return false, err
	}
	if err := removePendingStorageLocationMigration(); err != nil {
		return false, err
	}
	return true, nil
}

func removePendingStorageLocationMigration() error {
	path, err := PendingStorageLocationMigrationPath()
	if err != nil {
		return err
	}
	if err := runPendingStorageMigrationTestHook("pending-remove"); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeStoragePath(path, info) || !info.Mode().IsRegular() {
		return ErrLocationsInvalid
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

func normalizePendingMigration(migration *PendingStorageLocationMigration) {
	if migration.Action == "" {
		migration.Action = pendingStorageMigrationMigrate
	}
}

func validatePendingMigration(migration PendingStorageLocationMigration) error {
	normalizePendingMigration(&migration)
	if err := validatePendingMigrationShape(migration); err != nil {
		return err
	}
	switch migration.Action {
	case pendingStorageMigrationMigrate:
		return validatePendingMigrationMigrate(migration)
	case pendingStorageMigrationSwitch:
		return validatePendingMigrationSwitch(migration)
	case pendingStorageMigrationCancel:
		return validatePendingMigrationCancel(migration)
	default:
		return ErrLocationsInvalid
	}
}

func validatePendingMigrationShape(migration PendingStorageLocationMigration) error {
	if migration.Version != pendingStorageMigrationLegacyVersion && migration.Version != pendingStorageMigrationVersion {
		return ErrLocationsInvalid
	}
	if !validMigrationID(migration.ID) {
		return ErrLocationsInvalid
	}
	if migration.Action != pendingStorageMigrationMigrate && migration.Action != pendingStorageMigrationSwitch && migration.Action != pendingStorageMigrationCancel {
		return ErrLocationsInvalid
	}
	return nil
}

func validatePendingMigrationMigrate(migration PendingStorageLocationMigration) error {
	if err := validateMigrationPaths(migration.SourceDataRoot, migration.TargetDataRoot, migration.SourceBackupRoot, migration.TargetBackupRoot); err != nil {
		return err
	}
	if filepath.Clean(migration.SourceDataRoot) == filepath.Clean(migration.TargetDataRoot) && filepath.Clean(migration.SourceBackupRoot) == filepath.Clean(migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if rootsOverlap(migration.SourceDataRoot, migration.SourceBackupRoot) || rootsOverlap(migration.TargetDataRoot, migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.SourceDataRoot) != filepath.Clean(migration.TargetDataRoot) && pathsOverlapOrEqual(migration.SourceDataRoot, migration.TargetDataRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.SourceBackupRoot) != filepath.Clean(migration.TargetBackupRoot) && pathsOverlapOrEqual(migration.SourceBackupRoot, migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.TargetDataRoot) != filepath.Clean(migration.SourceDataRoot) && pathsOverlapOrEqual(migration.TargetDataRoot, migration.SourceBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.TargetBackupRoot) != filepath.Clean(migration.SourceBackupRoot) && pathsOverlapOrEqual(migration.TargetBackupRoot, migration.SourceDataRoot) {
		return ErrLocationsInvalid
	}
	if migration.Version == pendingStorageMigrationVersion {
		if !validMigrationPlan(migration.DataPlan) || !validMigrationPlan(migration.BackupPlan) || !validMigrationPhase(migration.Phase) {
			return ErrLocationsInvalid
		}
	}
	return nil
}

func validatePendingMigrationSwitch(migration PendingStorageLocationMigration) error {
	if err := validateMigrationPaths(migration.TargetDataRoot, migration.TargetBackupRoot); err != nil {
		return err
	}
	if filepath.Clean(migration.TargetDataRoot) != filepath.Clean(migration.TargetBackupRoot) && rootsOverlap(migration.TargetDataRoot, migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	return nil
}

func validatePendingMigrationCancel(migration PendingStorageLocationMigration) error {
	if err := validateMigrationPaths(migration.SourceDataRoot, migration.SourceBackupRoot); err != nil {
		return err
	}
	if filepath.Clean(migration.SourceDataRoot) != filepath.Clean(migration.SourceBackupRoot) && rootsOverlap(migration.SourceDataRoot, migration.SourceBackupRoot) {
		return ErrLocationsInvalid
	}
	return nil
}

func validateMigrationPaths(paths ...string) error {
	for _, path := range paths {
		normalized, err := normalizeAbsolutePath(path)
		if err != nil || normalized != filepath.Clean(path) {
			return ErrLocationsInvalid
		}
	}
	return nil
}

func validMigrationPlan(plan PendingStorageMigrationPlan) bool {
	return plan == PendingStorageMigrationPlanUnchanged || plan == PendingStorageMigrationPlanOpenExisting || plan == PendingStorageMigrationPlanCopyRequired
}

func validMigrationPhase(phase PendingStorageMigrationPhase) bool {
	return phase == PendingStorageMigrationPhasePrepared || phase == PendingStorageMigrationPhaseDataPlaced || phase == PendingStorageMigrationPhaseBackupPlaced || phase == PendingStorageMigrationPhaseConfigCommitted
}

func preparePendingMigrationForWrite(migration PendingStorageLocationMigration) (PendingStorageLocationMigration, error) {
	normalizePendingMigration(&migration)
	if migration.Version != pendingStorageMigrationLegacyVersion && migration.Version != 0 && migration.Version != pendingStorageMigrationVersion {
		return PendingStorageLocationMigration{}, ErrLocationsInvalid
	}
	if migration.Action == pendingStorageMigrationMigrate && (migration.Version != pendingStorageMigrationVersion || migration.DataPlan == "" || migration.BackupPlan == "" || migration.Phase == "") {
		dataPlan, err := planMigrationRoot(migration.TargetDataRoot, false)
		if err != nil {
			return PendingStorageLocationMigration{}, err
		}
		backupPlan, err := planMigrationRoot(migration.TargetBackupRoot, true)
		if err != nil {
			// A data root is also a valid default archive root. It may not have an
			// archive directory yet, so fall back to the data-root probe only for
			// the planning decision.
			if filepath.Clean(migration.TargetDataRoot) == filepath.Clean(migration.TargetBackupRoot) {
				if dataProbe, probeErr := ProbeDataRoot(migration.TargetBackupRoot); probeErr == nil && dataProbe.Kind == RootExisting {
					backupPlan = PendingStorageMigrationPlanOpenExisting
				} else {
					return PendingStorageLocationMigration{}, err
				}
			} else {
				return PendingStorageLocationMigration{}, err
			}
		}
		if filepath.Clean(migration.SourceDataRoot) == filepath.Clean(migration.SourceBackupRoot) && filepath.Clean(migration.TargetDataRoot) == filepath.Clean(migration.TargetBackupRoot) {
			backupPlan = PendingStorageMigrationPlanUnchanged
		}
		migration.Version = pendingStorageMigrationVersion
		migration.DataPlan = dataPlan
		migration.BackupPlan = backupPlan
		migration.Phase = PendingStorageMigrationPhasePrepared
	} else {
		migration.Version = pendingStorageMigrationVersion
		if migration.Action != pendingStorageMigrationMigrate && migration.Phase == "" {
			migration.Phase = PendingStorageMigrationPhasePrepared
		}
	}
	if err := validatePendingMigration(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return migration, nil
}

func prepareLegacyPendingMigrationForResume(migration PendingStorageLocationMigration) (PendingStorageLocationMigration, error) {
	if err := validatePendingMigration(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	if migration.Action != pendingStorageMigrationMigrate {
		migration.Version = pendingStorageMigrationVersion
		if migration.Phase == "" {
			migration.Phase = PendingStorageMigrationPhasePrepared
		}
		return migration, validatePendingMigration(migration)
	}
	dataPlan, err := planLegacyMigrationRoot(migration.TargetDataRoot, false)
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	backupPlan, err := planLegacyMigrationRoot(migration.TargetBackupRoot, true)
	if err != nil {
		return PendingStorageLocationMigration{}, err
	}
	if filepath.Clean(migration.SourceDataRoot) == filepath.Clean(migration.SourceBackupRoot) && filepath.Clean(migration.TargetDataRoot) == filepath.Clean(migration.TargetBackupRoot) {
		backupPlan = PendingStorageMigrationPlanUnchanged
	}
	migration.Version = pendingStorageMigrationVersion
	migration.DataPlan = dataPlan
	migration.BackupPlan = backupPlan
	migration.Phase = PendingStorageMigrationPhasePrepared
	if err := validatePendingMigration(migration); err != nil {
		return PendingStorageLocationMigration{}, err
	}
	return migration, nil
}

func planLegacyMigrationRoot(target string, backup bool) (PendingStorageMigrationPlan, error) {
	probe, err := probeForMigration(target, backup)
	if err != nil {
		return "", err
	}
	if probe.Kind == RootEmpty {
		return PendingStorageMigrationPlanCopyRequired, nil
	}
	// A v1 marker has no persisted evidence distinguishing a deliberately
	// selected existing root from a target that was already partially copied.
	// Never infer completion from its current contents.
	return "", ErrRootInvalid
}

func validatePendingMigrationPathsForPlanning(migration PendingStorageLocationMigration) error {
	if err := validateMigrationPaths(migration.SourceDataRoot, migration.TargetDataRoot, migration.SourceBackupRoot, migration.TargetBackupRoot); err != nil {
		return err
	}
	if filepath.Clean(migration.SourceDataRoot) == filepath.Clean(migration.TargetDataRoot) && filepath.Clean(migration.SourceBackupRoot) == filepath.Clean(migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if rootsOverlap(migration.SourceDataRoot, migration.SourceBackupRoot) || rootsOverlap(migration.TargetDataRoot, migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.SourceDataRoot) != filepath.Clean(migration.TargetDataRoot) && pathsOverlapOrEqual(migration.SourceDataRoot, migration.TargetDataRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.SourceBackupRoot) != filepath.Clean(migration.TargetBackupRoot) && pathsOverlapOrEqual(migration.SourceBackupRoot, migration.TargetBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.TargetDataRoot) != filepath.Clean(migration.SourceDataRoot) && pathsOverlapOrEqual(migration.TargetDataRoot, migration.SourceBackupRoot) {
		return ErrLocationsInvalid
	}
	if filepath.Clean(migration.TargetBackupRoot) != filepath.Clean(migration.SourceBackupRoot) && pathsOverlapOrEqual(migration.TargetBackupRoot, migration.SourceDataRoot) {
		return ErrLocationsInvalid
	}
	return nil
}

func planMigrationRoot(target string, backup bool) (PendingStorageMigrationPlan, error) {
	probe, err := probeForMigration(target, backup)
	if err != nil {
		return "", err
	}
	if probe.Kind == RootEmpty {
		return PendingStorageMigrationPlanCopyRequired, nil
	}
	if probe.Kind == RootExisting {
		return PendingStorageMigrationPlanOpenExisting, nil
	}
	return "", ErrRootInvalid
}

func rootsOverlap(first string, second string) bool {
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	return first != second && (isWithinOrEqual(first, second) || isWithinOrEqual(second, first))
}

func pathsOverlapOrEqual(first string, second string) bool {
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	return isWithinOrEqual(first, second) || isWithinOrEqual(second, first)
}

func applyMigratePendingStorageLocation(ctx context.Context, migration PendingStorageLocationMigration) (bool, error) {
	if migration.Phase == PendingStorageMigrationPhaseConfigCommitted {
		return finishCommittedMigration(migration)
	}
	if migration.Phase == PendingStorageMigrationPhaseDataPlaced || migration.Phase == PendingStorageMigrationPhaseBackupPlaced {
		if err := verifyDataMigrationPlacement(migration); err != nil {
			return false, err
		}
	}
	if migration.Phase == PendingStorageMigrationPhaseBackupPlaced {
		if err := verifyBackupMigrationPlacement(migration); err != nil {
			return false, err
		}
	}

	if migration.Phase == PendingStorageMigrationPhasePrepared {
		if err := applyDataMigrationPlan(ctx, migration); err != nil {
			return false, err
		}
		if err := saveMigrationProgress(&migration, PendingStorageMigrationPhaseDataPlaced); err != nil {
			return false, err
		}
	}

	if migration.Phase == PendingStorageMigrationPhaseDataPlaced {
		if err := applyBackupMigrationPlan(ctx, migration); err != nil {
			return false, err
		}
		if err := saveMigrationProgress(&migration, PendingStorageMigrationPhaseBackupPlaced); err != nil {
			return false, err
		}
	}

	if migration.Phase == PendingStorageMigrationPhaseBackupPlaced {
		if err := runPendingStorageMigrationTestHook("config-save"); err != nil {
			return false, err
		}
		if err := SaveStorageLocations(StorageLocations{
			Version: storageLocationsVersion, DataRoot: migration.TargetDataRoot, BackupRoot: migration.TargetBackupRoot,
		}); err != nil {
			return false, err
		}
		if err := saveMigrationProgress(&migration, PendingStorageMigrationPhaseConfigCommitted); err != nil {
			return false, err
		}
	}

	if migration.Phase != PendingStorageMigrationPhaseConfigCommitted {
		return false, ErrLocationsInvalid
	}
	return finishCommittedMigration(migration)
}

func verifyDataMigrationPlacement(migration PendingStorageLocationMigration) error {
	if migration.DataPlan != PendingStorageMigrationPlanCopyRequired {
		return nil
	}
	matched, err := migrationStageMarkerMatches(migration.TargetDataRoot, migrationStageMarkerForData(migration))
	if err != nil {
		return err
	}
	if !matched {
		return ErrRootInvalid
	}
	return nil
}

func verifyBackupMigrationPlacement(migration PendingStorageLocationMigration) error {
	if migration.BackupPlan != PendingStorageMigrationPlanCopyRequired {
		return nil
	}
	sourceArchive := filepath.Join(filepath.Clean(migration.SourceBackupRoot), ".atlasnote-backups")
	info, err := os.Lstat(sourceArchive)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeStoragePath(sourceArchive, info) || !info.IsDir() {
		return ErrRootInvalid
	}
	matched, err := migrationStageMarkerMatches(filepath.Join(filepath.Clean(migration.TargetBackupRoot), ".atlasnote-backups"), migrationStageMarkerForBackup(migration))
	if err != nil {
		return err
	}
	if !matched {
		return ErrRootInvalid
	}
	return nil
}

func applyDataMigrationPlan(ctx context.Context, migration PendingStorageLocationMigration) error {
	switch migration.DataPlan {
	case PendingStorageMigrationPlanUnchanged, PendingStorageMigrationPlanOpenExisting:
		return nil
	case PendingStorageMigrationPlanCopyRequired:
		marker := migrationStageMarkerForData(migration)
		placed, err := migrationStageMarkerMatches(migration.TargetDataRoot, marker)
		if err != nil {
			return err
		}
		if placed {
			return nil
		}
		if err := requireEmptyOrMissingMigrationTarget(migration.TargetDataRoot, false); err != nil {
			return err
		}
		skipBackupDirectory := filepath.Clean(migration.SourceDataRoot) != filepath.Clean(migration.SourceBackupRoot) ||
			filepath.Clean(migration.TargetDataRoot) != filepath.Clean(migration.TargetBackupRoot)
		return applyRootMigration(ctx, migration.SourceDataRoot, migration.TargetDataRoot, false, migration.ID, skipBackupDirectory)
	default:
		return ErrLocationsInvalid
	}
}

func applyBackupMigrationPlan(ctx context.Context, migration PendingStorageLocationMigration) error {
	switch migration.BackupPlan {
	case PendingStorageMigrationPlanUnchanged, PendingStorageMigrationPlanOpenExisting:
		return nil
	case PendingStorageMigrationPlanCopyRequired:
		marker := migrationStageMarkerForBackup(migration)
		placed, err := migrationStageMarkerMatches(marker.TargetPath, marker)
		if err != nil {
			return err
		}
		if placed {
			return nil
		}
		if filepath.Clean(migration.TargetBackupRoot) == filepath.Clean(migration.TargetDataRoot) && migration.DataPlan == PendingStorageMigrationPlanCopyRequired {
			// The data-root copy owns the rest of this directory. Only the
			// archive child must still be empty before it is placed.
			if err := requireEmptyOrMissingMigrationTarget(marker.TargetPath, true); err != nil {
				return err
			}
		} else if err := requireEmptyOrMissingMigrationTarget(migration.TargetBackupRoot, true); err != nil {
			return err
		}
		return applyRootMigration(ctx, migration.SourceBackupRoot, migration.TargetBackupRoot, true, migration.ID, false)
	default:
		return ErrLocationsInvalid
	}
}

func finishCommittedMigration(migration PendingStorageLocationMigration) (bool, error) {
	if migration.DataPlan == PendingStorageMigrationPlanCopyRequired {
		if err := removeOwnedMigrationMarker(migration.TargetDataRoot, migrationStageMarkerForData(migration)); err != nil {
			return false, err
		}
	}
	if migration.BackupPlan == PendingStorageMigrationPlanCopyRequired {
		if err := removeOwnedMigrationMarker(filepath.Join(migration.TargetBackupRoot, ".atlasnote-backups"), migrationStageMarkerForBackup(migration)); err != nil {
			return false, err
		}
	}
	if err := removePendingStorageLocationMigration(); err != nil {
		return false, err
	}
	return true, nil
}

func saveMigrationProgress(migration *PendingStorageLocationMigration, phase PendingStorageMigrationPhase) error {
	if err := runPendingStorageMigrationTestHook(string(phase)); err != nil {
		return err
	}
	migration.Phase = phase
	return SavePendingStorageLocationMigration(*migration)
}

func runPendingStorageMigrationTestHook(event string) error {
	if pendingStorageMigrationTestHook == nil {
		return nil
	}
	return pendingStorageMigrationTestHook(event)
}

func migrationStageMarkerForData(migration PendingStorageLocationMigration) migrationStageMarker {
	return migrationStageMarker{
		Version: 1, OperationID: migration.ID, Kind: "data",
		SourcePath: filepath.Clean(migration.SourceDataRoot), TargetPath: filepath.Clean(migration.TargetDataRoot),
	}
}

func migrationStageMarkerForBackup(migration PendingStorageLocationMigration) migrationStageMarker {
	return migrationStageMarker{
		Version: 1, OperationID: migration.ID, Kind: "backup",
		SourcePath: filepath.Join(filepath.Clean(migration.SourceBackupRoot), ".atlasnote-backups"),
		TargetPath: filepath.Join(filepath.Clean(migration.TargetBackupRoot), ".atlasnote-backups"),
	}
}

func migrationStageMarkerMatches(root string, expected migrationStageMarker) (bool, error) {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if isUnsafeStoragePath(root, info) || !info.IsDir() {
		return false, ErrRootInvalid
	}
	markerPath := filepath.Join(root, migrationStageMarkerFile)
	markerInfo, err := os.Lstat(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if isUnsafeStoragePath(markerPath, markerInfo) || !markerInfo.Mode().IsRegular() || markerInfo.Size() > 1<<20 {
		return false, ErrLocationsInvalid
	}
	encoded, err := os.ReadFile(markerPath)
	if err != nil {
		return false, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var actual migrationStageMarker
	if err := decoder.Decode(&actual); err != nil {
		return false, ErrLocationsInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return false, ErrLocationsInvalid
	}
	if actual.Version != expected.Version || actual.OperationID != expected.OperationID || actual.Kind != expected.Kind {
		return false, ErrRootInvalid
	}
	for _, path := range []string{actual.SourcePath, actual.TargetPath, expected.SourcePath, expected.TargetPath} {
		normalized, pathErr := normalizeAbsolutePath(path)
		if pathErr != nil || normalized != filepath.Clean(path) {
			return false, ErrLocationsInvalid
		}
	}
	if filepath.Clean(actual.SourcePath) != filepath.Clean(expected.SourcePath) || filepath.Clean(actual.TargetPath) != filepath.Clean(expected.TargetPath) {
		return false, ErrRootInvalid
	}
	return true, nil
}

func removeOwnedMigrationMarker(root string, expected migrationStageMarker) error {
	matched, err := migrationStageMarkerMatches(root, expected)
	if err != nil || !matched {
		return err
	}
	return os.Remove(filepath.Join(root, migrationStageMarkerFile))
}

func requireEmptyOrMissingMigrationTarget(path string, backup bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeStoragePath(path, info) || !info.IsDir() {
		return ErrRootInvalid
	}
	if backup {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return ErrRootInvalid
		}
		return nil
	}
	probe, err := ProbeDataRoot(path)
	if err != nil {
		return err
	}
	if probe.Kind != RootEmpty {
		return ErrRootInvalid
	}
	return nil
}

func validMigrationID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func applyRootMigration(ctx context.Context, source string, target string, backup bool, operationID string, skipBackupDirectory bool) error {
	source, err := normalizeAbsolutePath(source)
	if err != nil {
		return err
	}
	target, err = normalizeAbsolutePath(target)
	if err != nil {
		return err
	}
	if source == target {
		return nil
	}
	if isWithinOrEqual(source, target) || isWithinOrEqual(target, source) {
		return ErrLocationsInvalid
	}
	if backup {
		// The default archive is inside the data root. After the data copy has
		// completed, that target is no longer an empty backup directory, but it
		// is still a valid Atlas Note root into which the archive can be copied.
		if err := ctx.Err(); err != nil {
			return err
		}
		return copyBackupArchive(ctx, source, target, operationID)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return copyDataRoot(ctx, source, target, operationID, skipBackupDirectory)
}

func probeForMigration(root string, backup bool) (RootProbe, error) {
	if backup {
		return ProbeBackupRoot(root)
	}
	return ProbeDataRoot(root)
}

func copyDataRoot(ctx context.Context, source string, target string, operationID string, skipBackupDirectory bool) error {
	if probe, err := ProbeDataRoot(source); err != nil || probe.Kind != RootExisting {
		if err != nil {
			return err
		}
		return ErrRootInvalid
	}
	stage := target + ".atlasnote-migration-" + operationID
	if pathsOverlapOrEqual(source, stage) {
		return ErrLocationsInvalid
	}
	marker := migrationStageMarker{
		Version: 1, OperationID: operationID, Kind: "data",
		SourcePath: filepath.Clean(source), TargetPath: filepath.Clean(target),
	}
	if err := prepareMigrationStage(stage, marker); err != nil {
		return err
	}
	if err := writeMigrationStageMarker(stage, marker); err != nil {
		return err
	}
	if err := copyDirectory(ctx, source, stage, func(relative string, entry os.DirEntry) bool {
		if relative == "atlasnote.lock" || relative == "storage-spaces.lock" || relative == storageLocationsFile || relative == pendingStorageMigrationFile {
			return true
		}
		if skipBackupDirectory && relative == ".atlasnote-backups" {
			return true
		}
		return false
	}); err != nil {
		_ = removeOwnedMigrationStage(stage, marker)
		return err
	}
	if err := commitCopiedDataRoot(stage, target); err != nil {
		_ = removeOwnedMigrationStage(stage, marker)
		return err
	}
	probe, err := ProbeDataRoot(target)
	if err != nil || probe.Kind != RootExisting {
		return ErrRootInvalid
	}
	return nil
}

func copyBackupArchive(ctx context.Context, source string, target string, operationID string) error {
	sourceArchive := filepath.Join(source, ".atlasnote-backups")
	targetArchive := filepath.Join(target, ".atlasnote-backups")
	if info, err := os.Lstat(sourceArchive); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	} else if isUnsafeStoragePath(sourceArchive, info) || !info.IsDir() {
		return ErrRootInvalid
	}
	stage := filepath.Join(target, ".atlasnote-backups.atlasnote-migration-"+operationID)
	if pathsOverlapOrEqual(sourceArchive, stage) {
		return ErrLocationsInvalid
	}
	marker := migrationStageMarker{
		Version: 1, OperationID: operationID, Kind: "backup",
		SourcePath: filepath.Clean(sourceArchive), TargetPath: filepath.Clean(targetArchive),
	}
	if err := prepareMigrationStage(stage, marker); err != nil {
		return err
	}
	if err := writeMigrationStageMarker(stage, marker); err != nil {
		return err
	}
	if err := copyDirectory(ctx, sourceArchive, stage, nil); err != nil {
		_ = removeOwnedMigrationStage(stage, marker)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetArchive), 0o700); err != nil {
		_ = removeOwnedMigrationStage(stage, marker)
		return err
	}
	if err := commitCopiedDirectory(stage, targetArchive); err != nil {
		_ = removeOwnedMigrationStage(stage, marker)
		return err
	}
	return nil
}

func copyDirectory(ctx context.Context, source string, target string, skip func(string, os.DirEntry) bool) error {
	info, err := os.Lstat(source)
	if err != nil || isUnsafeStoragePath(source, info) || !info.IsDir() {
		if errors.Is(err, os.ErrNotExist) {
			return ErrRootInvalid
		}
		return err
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if isUnsafeStoragePath(sourcePath, entryInfo) {
			return ErrRootInvalid
		}
		if entry.Name() == migrationStageMarkerFile {
			return nil
		}
		if skip != nil && skip(filepath.ToSlash(relative), entry) {
			if entryInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		destination := filepath.Join(target, relative)
		if entryInfo.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if !entryInfo.Mode().IsRegular() {
			return ErrRootInvalid
		}
		return copyFileForMigration(ctx, sourcePath, destination)
	})
}

func copyFileForMigration(ctx context.Context, source string, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		_ = os.Remove(target)
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		_ = os.Remove(target)
		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func commitCopiedDirectory(stage string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := ensureAbsent(target); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

// commitCopiedDataRoot installs a staged data-root copy. The folder picker
// returns existing directories, so a selected empty directory is a valid
// migration target. Only an empty, non-symlink directory may be removed here;
// archive directories continue to use commitCopiedDirectory's strict
// no-overwrite behavior.
func commitCopiedDataRoot(stage string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := removeEmptyMigrationTarget(target); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

func removeEmptyMigrationTarget(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeStoragePath(path, info) || !info.IsDir() {
		return ErrRootInvalid
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return ErrRootInvalid
	}
	// os.Remove only removes an empty directory. If another process writes to
	// the target after the check above, the removal fails and preserves it.
	return os.Remove(path)
}

func ensureAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return ErrRootInvalid
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func prepareMigrationStage(path string, expected migrationStageMarker) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeStoragePath(path, info) || !info.IsDir() {
		return ErrRootInvalid
	}
	matched, err := migrationStageMarkerMatches(path, expected)
	if err != nil {
		return err
	}
	if !matched {
		// A stage without our operation record may belong to another operation
		// or to the user. Never remove it as a guessed recovery.
		return ErrRootInvalid
	}
	return os.RemoveAll(path)
}

func writeMigrationStageMarker(stage string, marker migrationStageMarker) error {
	encoded, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return writeAtomic(filepath.Join(stage, migrationStageMarkerFile), encoded)
}

func removeOwnedMigrationStage(stage string, expected migrationStageMarker) error {
	matched, err := migrationStageMarkerMatches(stage, expected)
	if err != nil || !matched {
		return err
	}
	return os.RemoveAll(stage)
}

func isWithinOrEqual(parent string, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
