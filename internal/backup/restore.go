package backup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

const (
	pendingVersion        = 1
	pendingPhaseStaged    = "staged"
	pendingPhaseBackedUp  = "current-backed-up"
	pendingPhaseInstalled = "installed"
)

type pendingMarker struct {
	Version      int    `json:"version"`
	ID           string `json:"id"`
	BackupID     string `json:"backupId"`
	SpaceID      string `json:"spaceId"`
	ManifestHash string `json:"manifestHash"`
	CreatedAt    string `json:"createdAt"`
	Phase        string `json:"phase"`
}

type RestorePaths struct {
	ManagementRoot string
	BackupRoot     string
	SpaceID        string
	DataDir        string
	DatabasePath   string
	NotesDir       string
}

type ApplyResult struct {
	RestoreSafetyBackupID string
}

func (s *Service) ExecuteRestore(ctx context.Context, input RestoreExecutionInput) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return RestoreResult{}, err
	}
	if err := s.ensureNoRecoveryConflictLocked(); err != nil {
		return RestoreResult{}, err
	}
	authorization, ok := s.restoreTokens[strings.TrimSpace(input.Token)]
	if ok {
		delete(s.restoreTokens, strings.TrimSpace(input.Token))
	}
	if !ok || time.Now().UTC().After(authorization.ExpiresAt) {
		return RestoreResult{}, ErrRestoreAuthorization
	}
	manifest, raw, err := s.loadManifestLocked(authorization.BackupID)
	if err != nil {
		return RestoreResult{}, wrapTampered(err)
	}
	if hashBytes(raw) != authorization.ManifestHash {
		return RestoreResult{}, ErrRestoreAuthorization
	}
	if err := s.validateGenerationLocked(ctx, authorization.BackupID, manifest); err != nil {
		return RestoreResult{}, wrapTampered(err)
	}
	operationID, err := randomID()
	if err != nil {
		return RestoreResult{}, err
	}
	stageDir := filepath.Join(s.stagingRootLocked(), operationID)
	if !safePathWithin(s.backupRootLocked(), stageDir) {
		return RestoreResult{}, ErrValidation
	}
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return RestoreResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stageDir)
		}
	}()
	if err := copyGenerationToStage(ctx, s.generationsRootLocked(), authorization.BackupID, stageDir, manifest, raw); err != nil {
		return RestoreResult{}, fmt.Errorf("%w: copy restore generation: %w", ErrTampered, err)
	}
	if err := prepareRestoreStage(ctx, stageDir, manifest); err != nil {
		return RestoreResult{}, fmt.Errorf("%w: prepare restore stage: %w", ErrTampered, err)
	}
	marker := pendingMarker{
		Version: pendingVersion, ID: operationID, BackupID: authorization.BackupID,
		SpaceID: s.paths.SpaceID, ManifestHash: authorization.ManifestHash,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Phase: pendingPhaseStaged,
	}
	markerPath := filepath.Join(s.backupRootLocked(), pendingName)
	if err := writePendingMarker(markerPath, marker); err != nil {
		return RestoreResult{}, err
	}
	committed = true
	return RestoreResult{
		BackupID: authorization.BackupID, RestartRequired: true,
		Message: "バックアップの検証が完了しました。アプリを再起動すると復元を適用します。",
	}, nil
}

func (s *Service) CancelRestore(ctx context.Context) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return RestoreResult{}, err
	}
	marker, markerPath, err := readPendingMarker(s.backupRootLocked(), s.paths.SpaceID)
	if errors.Is(err, os.ErrNotExist) {
		return RestoreResult{Canceled: true, Message: "復元待機状態はありません。"}, nil
	}
	if err != nil {
		return RestoreResult{}, err
	}
	if marker.Phase != pendingPhaseStaged {
		return RestoreResult{}, ErrRestorePending
	}
	if err := os.Remove(markerPath); err != nil {
		return RestoreResult{}, err
	}
	stageDir := filepath.Join(s.stagingRootLocked(), marker.ID)
	rollbackDir := filepath.Join(s.backupRootLocked(), rollbackName, marker.ID)
	if safePathWithin(s.backupRootLocked(), stageDir) {
		_ = os.RemoveAll(stageDir)
	}
	if safePathWithin(s.backupRootLocked(), rollbackDir) {
		_ = os.RemoveAll(rollbackDir)
	}
	if err := ctx.Err(); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{Canceled: true, Message: "復元を取り消しました。"}, nil
}

func ApplyPendingRestore(ctx context.Context, paths RestorePaths) (ApplyResult, error) {
	if err := validateRestorePaths(paths); err != nil {
		return ApplyResult{}, err
	}
	markerPath := filepath.Join(paths.BackupRoot, pendingName)
	marker, _, err := readPendingMarker(paths.BackupRoot, paths.SpaceID)
	if errors.Is(err, os.ErrNotExist) {
		return ApplyResult{}, nil
	}
	if err != nil {
		return ApplyResult{}, err
	}
	syncMarker := filepath.Join(paths.DataDir, ".sync-recovery", pendingName)
	if info, err := os.Lstat(syncMarker); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ApplyResult{}, ErrRecoveryConflict
		}
		return ApplyResult{}, ErrRecoveryConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ApplyResult{}, err
	}
	if err := validatePendingMarker(marker, paths); err != nil {
		return ApplyResult{}, err
	}
	generationsRoot := filepath.Join(paths.BackupRoot, generationsName)
	if !safePathWithin(paths.BackupRoot, generationsRoot) {
		return ApplyResult{}, ErrRestoreApply
	}
	manifest, raw, err := loadManifestAt(generationsRoot, marker.BackupID, paths.SpaceID)
	if err != nil {
		return ApplyResult{}, wrapTampered(err)
	}
	if hashBytes(raw) != marker.ManifestHash {
		return ApplyResult{}, ErrTampered
	}
	if err := validateGenerationFiles(ctx, filepath.Join(generationsRoot, marker.BackupID), manifest); err != nil {
		return ApplyResult{}, wrapTampered(err)
	}
	stageDir := filepath.Join(paths.BackupRoot, stagingName, marker.ID)
	rollbackDir := filepath.Join(paths.BackupRoot, rollbackName, marker.ID)
	if !safePathWithin(paths.BackupRoot, stageDir) || !safePathWithin(paths.BackupRoot, rollbackDir) {
		return ApplyResult{}, ErrRestoreApply
	}
	if marker.Phase == pendingPhaseStaged {
		if err := validateRestoreStage(ctx, stageDir, manifest); err != nil {
			return ApplyResult{}, wrapTampered(err)
		}
		if err := createRestoreSafetyBackup(ctx, paths, marker.ID); err != nil {
			return ApplyResult{}, wrapRestoreApply(err)
		}
		if err := moveCurrentToRollback(paths, rollbackDir); err != nil {
			rollbackRestore(paths, stageDir, rollbackDir)
			return ApplyResult{}, wrapRestoreApply(err)
		}
		marker.Phase = pendingPhaseBackedUp
		if err := writePendingMarker(markerPath, marker); err != nil {
			rollbackRestore(paths, stageDir, rollbackDir)
			return ApplyResult{}, wrapRestoreApply(err)
		}
	}
	if marker.Phase == pendingPhaseBackedUp {
		if err := installRestoreStage(paths, stageDir); err != nil {
			// Keep the marker and the partially moved components. The next
			// startup can resume each idempotent move from the backed-up phase.
			return ApplyResult{}, wrapRestoreApply(err)
		}
		marker.Phase = pendingPhaseInstalled
		if err := writePendingMarker(markerPath, marker); err != nil {
			// The install itself may already be complete when marker replacement
			// fails; leaving the backed-up phase is safe and retryable.
			return ApplyResult{}, wrapRestoreApply(err)
		}
	}
	if marker.Phase != pendingPhaseInstalled {
		return ApplyResult{}, ErrRestoreApply
	}
	if err := validateInstalledRestore(ctx, paths); err != nil {
		rollbackRestore(paths, stageDir, rollbackDir)
		return ApplyResult{}, wrapRestoreApply(err)
	}
	if err := os.Remove(markerPath); err != nil {
		return ApplyResult{}, wrapRestoreApply(err)
	}
	_ = os.RemoveAll(stageDir)
	_ = os.RemoveAll(rollbackDir)
	return ApplyResult{RestoreSafetyBackupID: marker.ID}, nil
}

func validateRestorePaths(paths RestorePaths) error {
	if !backupIDPattern.MatchString(paths.SpaceID) || strings.TrimSpace(paths.ManagementRoot) == "" || strings.TrimSpace(paths.BackupRoot) == "" || strings.TrimSpace(paths.DataDir) == "" {
		return ErrValidation
	}
	managementRoot, err := filepath.Abs(filepath.Clean(paths.ManagementRoot))
	if err != nil {
		return ErrValidation
	}
	expectedBackupRoot, err := RootFor(managementRoot, paths.SpaceID)
	if err != nil || filepath.Clean(expectedBackupRoot) != filepath.Clean(paths.BackupRoot) {
		return ErrValidation
	}
	dataDir, err := filepath.Abs(filepath.Clean(paths.DataDir))
	if err != nil || !safePathWithinOrEqual(managementRoot, dataDir) {
		return ErrValidation
	}
	if filepath.Clean(paths.DatabasePath) != filepath.Join(dataDir, "atlasnote.db") || filepath.Clean(paths.NotesDir) != filepath.Join(dataDir, "notes") {
		return ErrValidation
	}
	return nil
}

func readPendingMarker(backupRoot string, spaceID string) (pendingMarker, string, error) {
	markerPath := filepath.Join(filepath.Clean(backupRoot), pendingName)
	info, err := os.Lstat(markerPath)
	if err != nil {
		return pendingMarker{}, markerPath, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return pendingMarker{}, markerPath, ErrRestoreApply
	}
	encoded, err := os.ReadFile(markerPath)
	if err != nil {
		return pendingMarker{}, markerPath, err
	}
	var marker pendingMarker
	if err := json.Unmarshal(encoded, &marker); err != nil {
		return pendingMarker{}, markerPath, err
	}
	if err := validatePendingMarker(marker, RestorePaths{SpaceID: spaceID}); err != nil {
		return pendingMarker{}, markerPath, err
	}
	return marker, markerPath, nil
}

func validatePendingMarker(marker pendingMarker, paths RestorePaths) error {
	if marker.Version != pendingVersion || !backupIDPattern.MatchString(marker.ID) || !backupIDPattern.MatchString(marker.BackupID) || marker.SpaceID != paths.SpaceID || marker.Phase != pendingPhaseStaged && marker.Phase != pendingPhaseBackedUp && marker.Phase != pendingPhaseInstalled || len(marker.ManifestHash) != sha256.Size*2 {
		return ErrRestoreApply
	}
	if _, err := time.Parse(time.RFC3339Nano, marker.CreatedAt); err != nil {
		return ErrRestoreApply
	}
	for _, value := range []string{marker.ManifestHash} {
		for _, char := range value {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				return ErrRestoreApply
			}
		}
	}
	return nil
}

func writePendingMarker(markerPath string, marker pendingMarker) error {
	encoded, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	return writeAtomicFile(markerPath, encoded)
}

func copyGenerationToStage(ctx context.Context, generationsRoot string, backupID string, stageDir string, manifest Manifest, rawManifest []byte) error {
	sourceDir := filepath.Join(generationsRoot, backupID)
	if !safePathWithin(generationsRoot, sourceDir) || !safePathWithin(filepath.Dir(stageDir), stageDir) {
		return ErrTampered
	}
	if err := os.MkdirAll(filepath.Join(stageDir, "notes"), 0o700); err != nil {
		return err
	}
	for _, entry := range manifest.Files {
		sourcePath, err := manifestFilePath(sourceDir, entry.Path)
		if err != nil {
			return err
		}
		destinationPath, err := manifestFilePath(stageDir, entry.Path)
		if err != nil {
			return err
		}
		if err := copyFile(ctx, sourcePath, destinationPath); err != nil {
			return err
		}
	}
	manifestPath := filepath.Join(stageDir, manifestName)
	if !safePathWithin(stageDir, manifestPath) {
		return ErrTampered
	}
	if err := writeAtomicFile(manifestPath, rawManifest); err != nil {
		return err
	}
	return nil
}

func prepareRestoreStage(ctx context.Context, stageDir string, manifest Manifest) (returnErr error) {
	stageDatabasePath := filepath.Join(stageDir, "atlasnote.db")
	db, err := database.Open(ctx, stageDatabasePath)
	if err != nil {
		return err
	}
	dbClosed := false
	defer func() {
		if !dbClosed {
			if err := db.Close(); returnErr == nil && err != nil {
				returnErr = err
			}
		}
	}()
	store, err := storage.NewMarkdownStore(filepath.Join(stageDir, "notes"))
	if err != nil {
		return err
	}
	manager := contentlock.NewManager(db, store)
	defer manager.Close()
	if err := manager.Recover(ctx); err != nil {
		return fmt.Errorf("recover staged content locks: %w", err)
	}
	stageNotes := note.NewService(note.NewRepository(db), store)
	stageNotes.SetContentLockGuard(manager)
	recoveryReport, err := stageNotes.Recover(ctx)
	if err != nil {
		return fmt.Errorf("recover staged notes: %w", err)
	}
	if len(recoveryReport.MissingNotes) > 0 {
		return fmt.Errorf("staged notes are missing: %w", ErrTampered)
	}
	if err := database.ValidateOpen(ctx, db); err != nil {
		return fmt.Errorf("validate staged database: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint staged database: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close staged database: %w", err)
	}
	dbClosed = true
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(stageDatabasePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove staged database sidecar: %w", err)
		}
	}
	if err := validateRestoreStageFiles(ctx, stageDir, manifest); err != nil {
		return fmt.Errorf("validate staged restore files: %w", err)
	}
	return nil
}

func validateRestoreStage(ctx context.Context, stageDir string, manifest Manifest) error {
	if info, err := os.Lstat(stageDir); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrRestoreApply
	}
	return validateRestoreStageFiles(ctx, stageDir, manifest)
}

// database.Open may apply forward-compatible migrations to the staged copy,
// so the database byte hash from the original generation is not expected to
// remain identical. Markdown bytes are canonical and must retain their exact
// manifest hashes; the staged database is checked by SQLite integrity and
// schema validation instead.
func validateRestoreStageFiles(ctx context.Context, stageDir string, manifest Manifest) error {
	if err := validateManifestTree(ctx, stageDir, manifest, false); err != nil {
		return err
	}
	_, err := database.ValidateSnapshot(ctx, filepath.Join(stageDir, "atlasnote.db"))
	return err
}

func validateGenerationFiles(ctx context.Context, generationDir string, manifest Manifest) error {
	generationsRoot := filepath.Dir(generationDir)
	if !safePathWithin(filepath.Dir(generationsRoot), generationsRoot) {
		return ErrTampered
	}
	if err := validateManifestTree(ctx, generationDir, manifest, true); err != nil {
		return err
	}
	info, err := database.ValidateSnapshot(ctx, filepath.Join(generationDir, "atlasnote.db"))
	if err != nil || info.SchemaVersion != manifest.SchemaVersion {
		if err != nil {
			return err
		}
		return ErrTampered
	}
	return nil
}

func loadManifestAt(generationsRoot string, backupID string, spaceID string) (Manifest, []byte, error) {
	if err := validateBackupID(backupID); err != nil {
		return Manifest{}, nil, err
	}
	if !safePathWithin(filepath.Dir(generationsRoot), generationsRoot) {
		return Manifest{}, nil, ErrTampered
	}
	generationDir := filepath.Join(generationsRoot, backupID)
	manifestPath := filepath.Join(generationDir, manifestName)
	info, err := os.Lstat(manifestPath)
	if err != nil {
		return Manifest{}, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maximumManifestSize {
		return Manifest{}, nil, ErrTampered
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, nil, err
	}
	if err := validateManifest(manifest, backupID, spaceID); err != nil {
		return Manifest{}, nil, err
	}
	return manifest, raw, nil
}

func createRestoreSafetyBackup(ctx context.Context, paths RestorePaths, id string) error {
	generationsRoot := filepath.Join(paths.BackupRoot, generationsName)
	generationDir := filepath.Join(generationsRoot, id)
	if !safePathWithin(paths.BackupRoot, generationDir) {
		return ErrRestoreApply
	}
	if info, err := os.Lstat(generationDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return ErrRestoreApply
		}
		manifest, _, loadErr := loadManifestAt(generationsRoot, id, paths.SpaceID)
		if loadErr != nil || manifest.Kind != KindRestoreSafety {
			return ErrRestoreApply
		}
		return validateGenerationFiles(ctx, generationDir, manifest)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stageDir := filepath.Join(paths.BackupRoot, stagingName, "safety-"+id)
	if !safePathWithin(paths.BackupRoot, stageDir) {
		return ErrRestoreApply
	}
	if info, err := os.Lstat(stageDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return ErrRestoreApply
		}
		if err := os.RemoveAll(stageDir); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stageDir)
		}
	}()
	if err := copyColdDatabase(ctx, paths.DatabasePath, filepath.Join(stageDir, "atlasnote.db")); err != nil {
		return err
	}
	if err := copyTree(ctx, paths.NotesDir, filepath.Join(stageDir, "notes")); err != nil {
		return err
	}
	info, err := database.ValidateSnapshot(ctx, filepath.Join(stageDir, "atlasnote.db"))
	if err != nil {
		return err
	}
	if err := removeDatabaseSidecars(filepath.Join(stageDir, "atlasnote.db")); err != nil {
		return err
	}
	manifest, err := buildManifest(ctx, stageDir, id, KindRestoreSafety, paths.SpaceID, info.SchemaVersion)
	if err != nil {
		return err
	}
	if err := writeManifest(filepath.Join(stageDir, manifestName), manifest); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(generationDir), 0o700); err != nil {
		return err
	}
	if err := os.Rename(stageDir, generationDir); err != nil {
		return err
	}
	committed = true
	pruneGenerations(generationsRoot, KindRestoreSafety, maximumSafetyBackups)
	return nil
}

func copyColdDatabase(ctx context.Context, sourcePath string, destinationPath string) error {
	if info, err := os.Lstat(sourcePath); errors.Is(err, os.ErrNotExist) {
		db, openErr := database.Open(ctx, destinationPath)
		if openErr != nil {
			return openErr
		}
		return db.Close()
	} else if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ErrRestoreApply
	}
	sourceDB, err := database.Open(ctx, sourcePath)
	if err != nil {
		return err
	}
	backupErr := database.OnlineBackup(ctx, sourceDB, destinationPath)
	closeErr := sourceDB.Close()
	if backupErr != nil {
		return backupErr
	}
	return closeErr
}

func moveCurrentToRollback(paths RestorePaths, rollbackDir string) error {
	if !safePathWithin(paths.BackupRoot, rollbackDir) {
		return ErrRestoreApply
	}
	if err := os.MkdirAll(rollbackDir, 0o700); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := moveCurrentIfNeeded(paths.DatabasePath+suffix, filepath.Join(rollbackDir, "atlasnote.db"+suffix)); err != nil {
			return err
		}
	}
	return moveCurrentIfNeeded(paths.NotesDir, filepath.Join(rollbackDir, "notes"))
}

func moveCurrentIfNeeded(sourcePath string, destinationPath string) error {
	sourceExists := pathExists(sourcePath)
	destinationExists := pathExists(destinationPath)
	if sourceExists && destinationExists {
		return ErrRestoreApply
	}
	if !sourceExists {
		return nil
	}
	return moveIfPresent(sourcePath, destinationPath)
}

func installRestoreStage(paths RestorePaths, stageDir string) error {
	for _, item := range []struct {
		source, destination string
		optional            bool
	}{
		{source: filepath.Join(stageDir, "atlasnote.db-wal"), destination: paths.DatabasePath + "-wal", optional: true},
		{source: filepath.Join(stageDir, "atlasnote.db-shm"), destination: paths.DatabasePath + "-shm", optional: true},
		{source: filepath.Join(stageDir, "atlasnote.db"), destination: paths.DatabasePath},
		{source: filepath.Join(stageDir, "notes"), destination: paths.NotesDir},
	} {
		sourceExists := pathExists(item.source)
		destinationExists := pathExists(item.destination)
		if sourceExists && destinationExists {
			return ErrRestoreApply
		}
		if !sourceExists && !destinationExists {
			if item.optional {
				continue
			}
			return ErrRestoreApply
		}
		if sourceExists {
			if err := moveIfPresent(item.source, item.destination); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateInstalledRestore(ctx context.Context, paths RestorePaths) error {
	if info, err := os.Lstat(paths.NotesDir); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrRestoreApply
	}
	_, err := database.ValidateSnapshot(ctx, paths.DatabasePath)
	return err
}

func rollbackRestore(paths RestorePaths, stageDir string, rollbackDir string) {
	if !safePathWithin(paths.BackupRoot, rollbackDir) || !safePathWithin(paths.BackupRoot, stageDir) {
		return
	}
	for _, item := range []struct{ active, staged, old string }{
		{paths.DatabasePath + "-shm", filepath.Join(stageDir, "atlasnote.db-shm"), filepath.Join(rollbackDir, "atlasnote.db-shm")},
		{paths.DatabasePath + "-wal", filepath.Join(stageDir, "atlasnote.db-wal"), filepath.Join(rollbackDir, "atlasnote.db-wal")},
		{paths.DatabasePath, filepath.Join(stageDir, "atlasnote.db"), filepath.Join(rollbackDir, "atlasnote.db")},
		{paths.NotesDir, filepath.Join(stageDir, "notes"), filepath.Join(rollbackDir, "notes")},
	} {
		if pathExists(item.old) {
			if pathExists(item.active) && !pathExists(item.staged) {
				_ = os.Rename(item.active, item.staged)
			}
			if !pathExists(item.active) {
				_ = os.Rename(item.old, item.active)
			}
		}
	}
}

func moveIfPresent(sourcePath string, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrRestoreApply
	}
	if pathExists(destinationPath) {
		return ErrRestoreApply
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	return os.Rename(sourcePath, destinationPath)
}

func wrapRestoreApply(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrRestoreApply, err)
}

func pathExists(value string) bool {
	_, err := os.Lstat(value)
	return err == nil
}
