package appcleanup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	appConfigDirectory         = "AtlasNote"
	storageMigrationMarker     = "storage-location-migration.json"
	syncRecoveryDirectory      = ".sync-recovery"
	restoreWorkspaceDirectory  = ".atlasnote-restore"
	backupDirectory            = ".atlasnote-backups"
	pendingRecoveryMarker      = "pending.json"
	dataMigrationStageSuffix   = ".atlasnote-migration-"
	backupMigrationStagePrefix = ".atlasnote-backups.atlasnote-migration-"
)

// ensureNoPendingOperations is deliberately conservative. The uninstall
// helper must not touch even the unrelated display settings or credentials
// while a normal startup still has a migration/recovery operation to resume.
func ensureNoPendingOperations(identity UserIdentity, dataRoots []string, archiveRoots []string) error {
	if err := inspectPendingMarker(filepath.Join(identity.AppDataDir, appConfigDirectory, storageMigrationMarker)); err != nil {
		return err
	}
	for _, root := range dataRoots {
		if err := inspectDataMigrationStages(root); err != nil {
			return err
		}
		if err := inspectNonEmptyRecoveryDirectory(filepath.Join(root, syncRecoveryDirectory)); err != nil {
			return err
		}
		if err := inspectRestoreWorkspaces(filepath.Join(root, restoreWorkspaceDirectory)); err != nil {
			return err
		}
	}
	for _, root := range archiveRoots {
		if err := inspectBackupMigrationStages(root); err != nil {
			return err
		}
		if err := inspectBackupPendingMarkers(root); err != nil {
			return err
		}
	}
	return nil
}

func inspectPendingMarker(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isUnsafeFileInfo(path, info) || !info.Mode().IsRegular() {
		return ErrUnsafeCleanupPath
	}
	return ErrPendingRecovery
}

func inspectExistingDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if isUnsafeFileInfo(path, info) || !info.IsDir() {
		return false, ErrUnsafeCleanupPath
	}
	return true, nil
}

func inspectNonEmptyRecoveryDirectory(path string) error {
	exists, err := inspectExistingDirectory(path)
	if err != nil || !exists {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		info, err := os.Lstat(childPath)
		if err != nil {
			return err
		}
		if isUnsafeFileInfo(childPath, info) {
			return ErrUnsafeCleanupPath
		}
	}
	if len(entries) > 0 {
		return ErrPendingRecovery
	}
	return nil
}

func inspectRestoreWorkspaces(path string) error {
	exists, err := inspectExistingDirectory(path)
	if err != nil || !exists {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !spaceIDPattern.MatchString(entry.Name()) {
			return ErrUnsafeCleanupPath
		}
		spacePath := filepath.Join(path, entry.Name())
		spaceInfo, err := os.Lstat(spacePath)
		if err != nil {
			return err
		}
		if isUnsafeFileInfo(spacePath, spaceInfo) || !spaceInfo.IsDir() {
			return ErrUnsafeCleanupPath
		}
		spaceEntries, err := os.ReadDir(spacePath)
		if err != nil {
			return err
		}
		if err := inspectPendingMarker(filepath.Join(spacePath, pendingRecoveryMarker)); err != nil {
			return err
		}
		for _, spaceEntry := range spaceEntries {
			spaceEntryPath := filepath.Join(spacePath, spaceEntry.Name())
			spaceEntryInfo, err := os.Lstat(spaceEntryPath)
			if err != nil {
				return err
			}
			if isUnsafeFileInfo(spaceEntryPath, spaceEntryInfo) {
				return ErrUnsafeCleanupPath
			}
		}
		if len(spaceEntries) > 0 {
			return ErrPendingRecovery
		}
	}
	return nil
}

func inspectBackupPendingMarkers(root string) error {
	exists, err := inspectExistingDirectory(root)
	if err != nil || !exists {
		return err
	}
	backupRoot := filepath.Join(root, backupDirectory)
	exists, err = inspectExistingDirectory(backupRoot)
	if err != nil || !exists {
		return err
	}
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !spaceIDPattern.MatchString(entry.Name()) {
			return ErrUnsafeCleanupPath
		}
		spacePath := filepath.Join(backupRoot, entry.Name())
		spaceInfo, err := os.Lstat(spacePath)
		if err != nil {
			return err
		}
		if isUnsafeFileInfo(spacePath, spaceInfo) || !spaceInfo.IsDir() {
			return ErrUnsafeCleanupPath
		}
		if err := inspectPendingMarker(filepath.Join(spacePath, pendingRecoveryMarker)); err != nil {
			return err
		}
	}
	return nil
}

func inspectDataMigrationStages(root string) error {
	parent := filepath.Dir(root)
	exists, err := inspectExistingDirectory(parent)
	if err != nil || !exists {
		return err
	}
	baseName := filepath.Base(root)
	prefix := baseName + dataMigrationStageSuffix
	entries, err := os.ReadDir(parent)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || len(entry.Name()) == len(prefix) {
			continue
		}
		stagePath := filepath.Join(parent, entry.Name())
		stageInfo, err := os.Lstat(stagePath)
		if err != nil {
			return err
		}
		if isUnsafeFileInfo(stagePath, stageInfo) || !stageInfo.IsDir() {
			return ErrUnsafeCleanupPath
		}
		return ErrPendingRecovery
	}
	return nil
}

func inspectBackupMigrationStages(root string) error {
	exists, err := inspectExistingDirectory(root)
	if err != nil || !exists {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), backupMigrationStagePrefix) || len(entry.Name()) == len(backupMigrationStagePrefix) {
			continue
		}
		stagePath := filepath.Join(root, entry.Name())
		stageInfo, err := os.Lstat(stagePath)
		if err != nil {
			return err
		}
		if isUnsafeFileInfo(stagePath, stageInfo) || !stageInfo.IsDir() {
			return ErrUnsafeCleanupPath
		}
		return ErrPendingRecovery
	}
	return nil
}
