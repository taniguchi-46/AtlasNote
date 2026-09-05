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
	pendingStorageMigrationVersion = 1
	pendingStorageMigrationFile    = "storage-location-migration.json"
	pendingStorageMigrationMigrate = "migrate"
	pendingStorageMigrationSwitch  = "switch"
	pendingStorageMigrationCancel  = "cancel"
)

const (
	PendingStorageMigrationActionMigrate = pendingStorageMigrationMigrate
	PendingStorageMigrationActionSwitch  = pendingStorageMigrationSwitch
	PendingStorageMigrationActionCancel  = pendingStorageMigrationCancel
)

type PendingStorageLocationMigration struct {
	Version          int    `json:"version"`
	ID               string `json:"id"`
	Action           string `json:"action,omitempty"`
	SourceDataRoot   string `json:"sourceDataRoot"`
	TargetDataRoot   string `json:"targetDataRoot"`
	SourceBackupRoot string `json:"sourceBackupRoot"`
	TargetBackupRoot string `json:"targetBackupRoot"`
}

func PendingStorageLocationMigrationPath() (string, error) {
	path, err := StorageLocationsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), pendingStorageMigrationFile), nil
}

func SavePendingStorageLocationMigration(migration PendingStorageLocationMigration) error {
	normalizePendingMigration(&migration)
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
	return loadPendingStorageLocationMigrationFrom(path)
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
	if err := validatePendingMigration(migration); err != nil {
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
	switch migration.Action {
	case pendingStorageMigrationCancel:
		if err := SaveStorageLocations(StorageLocations{
			Version: storageLocationsVersion, DataRoot: migration.SourceDataRoot, BackupRoot: migration.SourceBackupRoot,
		}); err != nil {
			return false, err
		}
		return true, removePendingStorageLocationMigration()
	case pendingStorageMigrationSwitch:
		if err := SaveStorageLocations(StorageLocations{
			Version: storageLocationsVersion, DataRoot: migration.TargetDataRoot, BackupRoot: migration.TargetBackupRoot,
		}); err != nil {
			return false, err
		}
		return true, removePendingStorageLocationMigration()
	case pendingStorageMigrationMigrate:
		// Continue with the version 1 copy-on-restart migration below.
	default:
		return false, ErrLocationsInvalid
	}
	if err := ValidateStorageLocations(StorageLocations{
		Version: storageLocationsVersion, DataRoot: migration.TargetDataRoot, BackupRoot: migration.TargetBackupRoot,
	}); err != nil {
		return false, err
	}
	backupTargetWasExisting := existingMigrationTarget(migration.TargetBackupRoot)
	if err := applyRootMigration(ctx, migration.SourceDataRoot, migration.TargetDataRoot, false, migration.ID, filepath.Clean(migration.TargetDataRoot) != filepath.Clean(migration.TargetBackupRoot)); err != nil {
		return false, err
	}
	if !backupTargetWasExisting {
		if err := applyRootMigration(ctx, migration.SourceBackupRoot, migration.TargetBackupRoot, true, migration.ID, false); err != nil {
			return false, err
		}
	}
	if err := SaveStorageLocations(StorageLocations{
		Version: storageLocationsVersion, DataRoot: migration.TargetDataRoot, BackupRoot: migration.TargetBackupRoot,
	}); err != nil {
		return false, err
	}
	return true, removePendingStorageLocationMigration()
}

func removePendingStorageLocationMigration() error {
	path, err := PendingStorageLocationMigrationPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
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
	if migration.Version != pendingStorageMigrationVersion || !validMigrationID(migration.ID) {
		return ErrLocationsInvalid
	}
	if migration.Action != pendingStorageMigrationMigrate && migration.Action != pendingStorageMigrationSwitch && migration.Action != pendingStorageMigrationCancel {
		return ErrLocationsInvalid
	}
	for _, path := range []string{migration.SourceDataRoot, migration.TargetDataRoot, migration.SourceBackupRoot, migration.TargetBackupRoot} {
		normalized, err := normalizeAbsolutePath(path)
		if err != nil || normalized != filepath.Clean(path) {
			return ErrLocationsInvalid
		}
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

func existingMigrationTarget(path string) bool {
	if probe, err := ProbeBackupRoot(path); err == nil && probe.Kind == RootExisting {
		return true
	}
	if probe, err := ProbeDataRoot(path); err == nil && probe.Kind == RootExisting {
		return true
	}
	return false
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
		if probe, probeErr := ProbeBackupRoot(target); probeErr == nil && probe.Kind == RootExisting {
			return nil
		}
		if _, probeErr := ProbeDataRoot(target); probeErr != nil && !errors.Is(probeErr, os.ErrNotExist) {
			return probeErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return copyBackupArchive(ctx, source, target, operationID)
	}
	probe, err := probeForMigration(target, false)
	if err != nil {
		return err
	}
	if probe.Kind == RootExisting {
		// A valid existing root is an explicit “open existing” choice. It is
		// already complete, so never merge or overwrite it.
		return nil
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
	if err := prepareMigrationStage(stage); err != nil {
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
		_ = os.RemoveAll(stage)
		return err
	}
	if err := commitCopiedDataRoot(stage, target); err != nil {
		_ = os.RemoveAll(stage)
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
	if err := prepareMigrationStage(stage); err != nil {
		return err
	}
	if err := copyDirectory(ctx, sourceArchive, stage, nil); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetArchive), 0o700); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	if err := commitCopiedDirectory(stage, targetArchive); err != nil {
		_ = os.RemoveAll(stage)
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

func prepareMigrationStage(path string) error {
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
	return os.RemoveAll(path)
}

func isWithinOrEqual(parent string, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
