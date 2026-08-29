package backup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/database"
)

const (
	backupDirectoryName  = ".atlasnote-backups"
	generationsName      = "generations"
	stagingName          = "staging"
	rollbackName         = "rollback"
	manifestName         = "manifest.json"
	pendingName          = "pending.json"
	maximumManifestSize  = 16 << 20
	maximumManifestFiles = 100_000
)

const restoreWorkspaceDirectory = ".atlasnote-restore"

var backupIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// SyncExclusiveGate and ContentSnapshotGate keep backup coordination narrow
// so this package does not depend on note implementation details.
type SyncExclusiveGate interface {
	BeginSyncExclusive(context.Context) (context.Context, func())
}

type ContentSnapshotGate interface {
	BeginStorageSnapshot(context.Context) func()
}

type restoreAuthorization struct {
	BackupID     string
	ManifestHash string
	ExpiresAt    time.Time
}

type Service struct {
	db            *sql.DB
	notes         SyncExclusiveGate
	contentLocks  ContentSnapshotGate
	paths         Paths
	mu            sync.Mutex
	restoreTokens map[string]restoreAuthorization
	closed        bool
}

func NewService(db *sql.DB, notes SyncExclusiveGate, contentLocks ContentSnapshotGate, paths Paths) *Service {
	return &Service{
		db:            db,
		notes:         notes,
		contentLocks:  contentLocks,
		paths:         paths,
		restoreTokens: make(map[string]restoreAuthorization),
	}
}

// Shutdown waits for any in-flight backup or restore staging operation and
// prevents new operations from starting before the database is closed.
func (s *Service) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func RootFor(managementRoot string, spaceID string) (string, error) {
	if !backupIDPattern.MatchString(spaceID) || strings.TrimSpace(managementRoot) == "" {
		return "", ErrValidation
	}
	root, err := filepath.Abs(filepath.Clean(managementRoot))
	if err != nil {
		return "", ErrValidation
	}
	backupRoot := filepath.Join(root, backupDirectoryName, spaceID)
	if !safePathWithin(root, backupRoot) {
		return "", ErrValidation
	}
	return backupRoot, nil
}

func PendingRestoreExists(backupRoot string) (bool, error) {
	backupRoot, err := filepath.Abs(filepath.Clean(backupRoot))
	if err != nil {
		return false, ErrValidation
	}
	managementRoot := filepath.Dir(filepath.Dir(backupRoot))
	if !safePathWithin(managementRoot, backupRoot) {
		return false, ErrRestoreApply
	}
	markerPath := filepath.Join(backupRoot, pendingName)
	info, err := os.Lstat(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect backup restore marker: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, ErrRestoreApply
	}
	return true, nil
}

func (s *Service) CreateAutomaticBackup(ctx context.Context) (AutomaticResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return AutomaticResult{}, err
	}
	if err := s.ensureNoRecoveryConflictLocked(); err != nil {
		return AutomaticResult{}, err
	}
	last, _, err := s.latestAutomaticLocked(ctx)
	if err != nil {
		return AutomaticResult{}, err
	}
	if !last.IsZero() && time.Since(last) < backupInterval {
		return AutomaticResult{Skipped: true}, nil
	}
	summary, err := s.createGenerationLocked(ctx, KindAutomatic)
	if err != nil {
		return AutomaticResult{}, err
	}
	return AutomaticResult{Created: true, Backup: &summary}, nil
}

func (s *Service) List(ctx context.Context) (ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return ListResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ListResult{}, err
	}
	entries, err := s.generationEntriesLocked()
	if err != nil {
		return ListResult{}, err
	}
	result := ListResult{Backups: make([]BackupSummary, 0, len(entries))}
	for _, entry := range entries {
		summary := BackupSummary{ID: entry.name, Restorable: false}
		manifest, _, loadErr := s.loadManifestLocked(entry.name)
		if loadErr != nil {
			summary.ErrorMessage = "バックアップの検証に失敗しました。"
			result.Backups = append(result.Backups, summary)
			continue
		}
		summary = summaryFromManifest(manifest)
		if err := s.validateGenerationLocked(ctx, entry.name, manifest); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ListResult{}, err
			}
			summary.Restorable = false
			summary.ErrorMessage = "バックアップの検証に失敗しました。"
		}
		result.Backups = append(result.Backups, summary)
	}
	sort.SliceStable(result.Backups, func(i, j int) bool {
		return result.Backups[i].CreatedAt > result.Backups[j].CreatedAt
	})
	return result, nil
}

func (s *Service) Status(ctx context.Context) (StatusResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return StatusResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return StatusResult{}, err
	}
	entries, err := s.generationEntriesLocked()
	if err != nil {
		return StatusResult{}, err
	}
	last, _, err := s.latestAutomaticLocked(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	pending, err := s.pendingRestoreExistsLocked()
	if err != nil {
		return StatusResult{}, err
	}
	due := last.IsZero() || time.Since(last) >= backupInterval
	return StatusResult{
		AutomaticEnabled: true,
		AutomaticDue:     due && !pending,
		LastAutomaticAt:  formatTime(last),
		BackupCount:      len(entries),
		PendingRestore:   pending,
	}, nil
}

func (s *Service) PreviewRestore(ctx context.Context, backupID string) (RestorePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return RestorePreview{}, err
	}
	if err := validateBackupID(backupID); err != nil {
		return RestorePreview{}, err
	}
	if err := s.ensureNoRecoveryConflictLocked(); err != nil {
		return RestorePreview{}, err
	}
	manifest, raw, err := s.loadManifestLocked(backupID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RestorePreview{}, ErrNotFound
		}
		return RestorePreview{}, wrapTampered(err)
	}
	if err := s.validateGenerationLocked(ctx, backupID, manifest); err != nil {
		return RestorePreview{}, wrapTampered(err)
	}
	token, err := randomID()
	if err != nil {
		return RestorePreview{}, err
	}
	for key, authorization := range s.restoreTokens {
		if time.Now().UTC().After(authorization.ExpiresAt) {
			delete(s.restoreTokens, key)
		}
	}
	manifestHash := hashBytes(raw)
	s.restoreTokens[token] = restoreAuthorization{
		BackupID: backupID, ManifestHash: manifestHash, ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	return RestorePreview{
		Token: token, BackupID: backupID, CreatedAt: manifest.CreatedAt,
		SizeBytes: manifest.TotalBytes, FileCount: manifest.FileCount,
		Message: "選択したバックアップで現在の保存空間を置き換えます。現在のデータは復元前に安全用バックアップとして保存されます。",
	}, nil
}

func (s *Service) readyLocked() error {
	if s == nil || s.closed || s.db == nil || s.notes == nil || s.contentLocks == nil {
		return ErrUnavailable
	}
	if err := validatePaths(s.paths); err != nil {
		return err
	}
	return nil
}

func (s *Service) backupRootLocked() string {
	root := s.paths.ArchiveRoot
	if strings.TrimSpace(root) == "" {
		root = s.paths.ManagementRoot
	}
	backupRoot, err := RootFor(root, s.paths.SpaceID)
	if err != nil {
		return filepath.Join(filepath.Clean(root), backupDirectoryName, s.paths.SpaceID)
	}
	return backupRoot
}

func (s *Service) generationsRootLocked() string {
	return filepath.Join(s.backupRootLocked(), generationsName)
}

func (s *Service) stagingRootLocked() string {
	return filepath.Join(s.backupRootLocked(), stagingName)
}

func (s *Service) restoreStagingRootLocked() string {
	if !s.usesLocalRestoreWorkspace() || strings.TrimSpace(s.paths.RestoreWorkspaceRoot) == "" {
		return s.stagingRootLocked()
	}
	return filepath.Join(filepath.Clean(s.paths.RestoreWorkspaceRoot), restoreWorkspaceDirectory, s.paths.SpaceID, stagingName)
}

func (s *Service) restoreRollbackRootLocked() string {
	if !s.usesLocalRestoreWorkspace() || strings.TrimSpace(s.paths.RestoreWorkspaceRoot) == "" {
		return filepath.Join(s.backupRootLocked(), rollbackName)
	}
	return filepath.Join(filepath.Clean(s.paths.RestoreWorkspaceRoot), restoreWorkspaceDirectory, s.paths.SpaceID, rollbackName)
}

func (s *Service) usesLocalRestoreWorkspace() bool {
	if strings.TrimSpace(s.paths.RestoreWorkspaceRoot) == "" {
		return false
	}
	archiveRoot := s.paths.ArchiveRoot
	if strings.TrimSpace(archiveRoot) == "" {
		archiveRoot = s.paths.ManagementRoot
	}
	return filepath.Clean(archiveRoot) != filepath.Clean(s.paths.ManagementRoot)
}

func (s *Service) pendingMarkerPathLocked() string {
	if s.usesLocalRestoreWorkspace() {
		workspaceRoot := s.paths.RestoreWorkspaceRoot
		if strings.TrimSpace(workspaceRoot) == "" {
			workspaceRoot = s.paths.ManagementRoot
		}
		return filepath.Join(filepath.Clean(workspaceRoot), restoreWorkspaceDirectory, s.paths.SpaceID, pendingName)
	}
	return filepath.Join(s.backupRootLocked(), pendingName)
}

func (s *Service) pendingRestoreExistsLocked() (bool, error) {
	pending, err := pendingRestoreExistsAt(s.pendingMarkerPathLocked())
	if err != nil || pending || !s.usesLocalRestoreWorkspace() {
		return pending, err
	}
	// Accept a legacy marker left in the archive root so an upgrade can finish
	// a restore that was staged by an earlier build.
	return PendingRestoreExists(s.backupRootLocked())
}

func (s *Service) ensureNoRecoveryConflictLocked() error {
	backupPending, err := s.pendingRestoreExistsLocked()
	if err != nil {
		return err
	}
	if backupPending {
		return ErrRestorePending
	}
	syncMarker := filepath.Join(filepath.Clean(s.paths.DataDir), ".sync-recovery", pendingName)
	if info, err := os.Lstat(syncMarker); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ErrRecoveryConflict
		}
		return ErrRecoveryConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect sync recovery marker: %w", err)
	}
	return nil
}

type generationEntry struct {
	name string
}

func (s *Service) generationEntriesLocked() ([]generationEntry, error) {
	root := s.generationsRootLocked()
	if !safePathWithin(s.backupRootLocked(), root) {
		return nil, ErrValidation
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []generationEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list backup generations: %w", err)
	}
	result := make([]generationEntry, 0, len(entries))
	for _, entry := range entries {
		if !backupIDPattern.MatchString(entry.Name()) || entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		result = append(result, generationEntry{name: entry.Name()})
	}
	return result, nil
}

func (s *Service) loadManifestLocked(backupID string) (Manifest, []byte, error) {
	if err := validateBackupID(backupID); err != nil {
		return Manifest{}, nil, err
	}
	if !safePathWithin(s.backupRootLocked(), s.generationsRootLocked()) {
		return Manifest{}, nil, ErrTampered
	}
	generationDir := filepath.Join(s.generationsRootLocked(), backupID)
	manifestPath := filepath.Join(generationDir, manifestName)
	if !safePathWithin(s.generationsRootLocked(), manifestPath) {
		return Manifest{}, nil, ErrValidation
	}
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
	if err := validateManifest(manifest, backupID, s.paths.SpaceID); err != nil {
		return Manifest{}, nil, err
	}
	return manifest, raw, nil
}

func (s *Service) validateGenerationLocked(ctx context.Context, backupID string, manifest Manifest) error {
	if !safePathWithin(s.backupRootLocked(), s.generationsRootLocked()) {
		return ErrTampered
	}
	generationDir := filepath.Join(s.generationsRootLocked(), backupID)
	if !safePathWithin(s.generationsRootLocked(), generationDir) {
		return ErrTampered
	}
	if err := validateManifestTree(ctx, generationDir, manifest, true); err != nil {
		return err
	}
	databaseInfo, err := database.ValidateSnapshot(ctx, filepath.Join(generationDir, "atlasnote.db"))
	if err != nil {
		return err
	}
	if databaseInfo.SchemaVersion != manifest.SchemaVersion {
		return ErrTampered
	}
	return nil
}

func (s *Service) createGenerationLocked(ctx context.Context, kind string) (summary BackupSummary, returnErr error) {
	if kind != KindAutomatic {
		return BackupSummary{}, ErrValidation
	}
	if err := database.ValidateOpen(ctx, s.db); err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	operationID, err := randomID()
	if err != nil {
		return BackupSummary{}, err
	}
	stageDir := filepath.Join(s.stagingRootLocked(), operationID)
	generationDir := filepath.Join(s.generationsRootLocked(), operationID)
	if !safePathWithin(s.backupRootLocked(), stageDir) || !safePathWithin(s.backupRootLocked(), generationDir) {
		return BackupSummary{}, ErrValidation
	}
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return BackupSummary{}, fmt.Errorf("create backup staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stageDir)
		}
	}()

	lockedContext, unlockNotes := s.notes.BeginSyncExclusive(ctx)
	defer unlockNotes()
	unlockContent := s.contentLocks.BeginStorageSnapshot(lockedContext)
	defer unlockContent()
	if err := database.OnlineBackup(lockedContext, s.db, filepath.Join(stageDir, "atlasnote.db")); err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	if err := copyTree(lockedContext, s.paths.NotesDir, filepath.Join(stageDir, "notes")); err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	info, err := database.ValidateSnapshot(lockedContext, filepath.Join(stageDir, "atlasnote.db"))
	if err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	if err := removeDatabaseSidecars(filepath.Join(stageDir, "atlasnote.db")); err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	manifest, err := buildManifest(lockedContext, stageDir, operationID, KindAutomatic, s.paths.SpaceID, info.SchemaVersion)
	if err != nil {
		return BackupSummary{}, wrapTampered(err)
	}
	if err := writeManifest(filepath.Join(stageDir, manifestName), manifest); err != nil {
		return BackupSummary{}, err
	}
	if _, err := os.Lstat(generationDir); err == nil {
		return BackupSummary{}, ErrValidation
	} else if !errors.Is(err, os.ErrNotExist) {
		return BackupSummary{}, err
	}
	if err := os.MkdirAll(filepath.Dir(generationDir), 0o700); err != nil {
		return BackupSummary{}, fmt.Errorf("create backup generation directory: %w", err)
	}
	if err := os.Rename(stageDir, generationDir); err != nil {
		return BackupSummary{}, fmt.Errorf("commit backup generation: %w", err)
	}
	committed = true
	pruneGenerations(s.generationsRootLocked(), KindAutomatic, maximumAutomaticBackups)
	return summaryFromManifest(manifest), nil
}

func (s *Service) latestAutomaticLocked(ctx context.Context) (time.Time, int, error) {
	entries, err := s.generationEntriesLocked()
	if err != nil {
		return time.Time{}, 0, err
	}
	var latest time.Time
	count := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return time.Time{}, 0, err
		}
		manifest, _, err := s.loadManifestLocked(entry.name)
		if err != nil || manifest.Kind != KindAutomatic {
			continue
		}
		if err := s.validateGenerationLocked(ctx, entry.name, manifest); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return time.Time{}, 0, err
			}
			continue
		}
		count++
		created, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
		if err != nil {
			continue
		}
		if created.After(time.Now().UTC()) {
			continue
		}
		if created.After(latest) {
			latest = created
		}
	}
	return latest, count, nil
}

func summaryFromManifest(manifest Manifest) BackupSummary {
	return BackupSummary{
		ID: manifest.ID, Kind: manifest.Kind, CreatedAt: manifest.CreatedAt,
		SizeBytes: manifest.TotalBytes, FileCount: manifest.FileCount, Restorable: true,
	}
}

func validatePaths(paths Paths) error {
	if !backupIDPattern.MatchString(paths.SpaceID) || strings.TrimSpace(paths.ManagementRoot) == "" || strings.TrimSpace(paths.DataDir) == "" {
		return ErrValidation
	}
	managementRoot, err := filepath.Abs(filepath.Clean(paths.ManagementRoot))
	if err != nil {
		return ErrValidation
	}
	dataDir, err := filepath.Abs(filepath.Clean(paths.DataDir))
	if err != nil || !safePathWithinOrEqual(managementRoot, dataDir) {
		return ErrValidation
	}
	archiveRoot := managementRoot
	if strings.TrimSpace(paths.ArchiveRoot) != "" {
		archiveRoot, err = filepath.Abs(filepath.Clean(paths.ArchiveRoot))
		if err != nil {
			return ErrValidation
		}
	}
	if strings.TrimSpace(paths.RestoreWorkspaceRoot) != "" {
		workspaceRoot, workspaceErr := filepath.Abs(filepath.Clean(paths.RestoreWorkspaceRoot))
		if workspaceErr != nil || !safePathWithinOrEqual(managementRoot, workspaceRoot) {
			return ErrValidation
		}
	}
	backupRoot := filepath.Join(archiveRoot, backupDirectoryName, paths.SpaceID)
	if !safePathWithin(archiveRoot, backupRoot) {
		return ErrValidation
	}
	databasePath, err := filepath.Abs(filepath.Clean(paths.DatabasePath))
	if err != nil {
		return ErrValidation
	}
	notesDir, err := filepath.Abs(filepath.Clean(paths.NotesDir))
	if err != nil {
		return ErrValidation
	}
	if databasePath != filepath.Join(dataDir, "atlasnote.db") || notesDir != filepath.Join(dataDir, "notes") {
		return ErrValidation
	}
	return nil
}

func validateBackupID(id string) error {
	if !backupIDPattern.MatchString(id) {
		return ErrValidation
	}
	return nil
}

func validateManifest(manifest Manifest, expectedID string, expectedSpaceID string) error {
	if manifest.Version != manifestVersion || manifest.ID != expectedID || manifest.Kind != KindAutomatic && manifest.Kind != KindRestoreSafety || manifest.SpaceID != expectedSpaceID {
		return ErrTampered
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil || manifest.SchemaVersion < 0 || manifest.FileCount != len(manifest.Files) || manifest.FileCount > maximumManifestFiles || manifest.TotalBytes < 0 {
		return ErrTampered
	}
	if len(manifest.Files) == 0 {
		return ErrTampered
	}
	previous := ""
	var total int64
	hasDatabase := false
	for _, entry := range manifest.Files {
		if _, err := manifestFilePath(".", entry.Path); err != nil || entry.Size < 0 || len(entry.SHA256) != sha256.Size*2 {
			return ErrTampered
		}
		if entry.Path <= previous {
			return ErrTampered
		}
		previous = entry.Path
		if entry.Path == "atlasnote.db" {
			if hasDatabase {
				return ErrTampered
			}
			hasDatabase = true
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return ErrTampered
		}
		if total > (1<<63-1)-entry.Size {
			return ErrTampered
		}
		total += entry.Size
	}
	if total != manifest.TotalBytes {
		return ErrTampered
	}
	if !hasDatabase {
		return ErrTampered
	}
	return nil
}

// validateManifestTree verifies that a backup contains exactly the files named
// by its manifest. This matters because the notes directory is the canonical
// source of note content and an unlisted file must not silently disappear on
// restore.
func validateManifestTree(ctx context.Context, root string, manifest Manifest, verifyDatabaseHash bool) error {
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return ErrTampered
	}
	topLevel, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	allowedTopLevel := map[string]struct{}{
		"atlasnote.db":  {},
		"manifest.json": {},
		"notes":         {},
	}
	for _, entry := range topLevel {
		if _, ok := allowedTopLevel[entry.Name()]; !ok || entry.Type()&os.ModeSymlink != 0 {
			return ErrTampered
		}
	}
	manifestPath := filepath.Join(root, manifestName)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil || manifestInfo.Mode()&os.ModeSymlink != 0 || !manifestInfo.Mode().IsRegular() || manifestInfo.Size() > maximumManifestSize {
		return ErrTampered
	}
	notesRoot := filepath.Join(root, "notes")
	notesInfo, err := os.Lstat(notesRoot)
	if err != nil || notesInfo.Mode()&os.ModeSymlink != 0 || !notesInfo.IsDir() {
		return ErrTampered
	}

	expectedNotes := make(map[string]FileEntry)
	var databaseEntry FileEntry
	hasDatabase := false
	for _, entry := range manifest.Files {
		filePath, err := manifestFilePath(root, entry.Path)
		if err != nil {
			return err
		}
		if entry.Path == "atlasnote.db" {
			if hasDatabase {
				return ErrTampered
			}
			hasDatabase = true
			databaseEntry = entry
			info, err := os.Lstat(filePath)
			if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return ErrTampered
			}
			if verifyDatabaseHash {
				if info.Size() != entry.Size {
					return ErrTampered
				}
				digest, size, err := hashFile(ctx, filePath)
				if err != nil {
					return err
				}
				if digest != entry.SHA256 || size != entry.Size {
					return ErrTampered
				}
			}
			continue
		}
		expectedNotes[entry.Path] = entry
	}
	if !hasDatabase {
		return ErrTampered
	}

	visitedNotes := make(map[string]struct{}, len(expectedNotes))
	var notesTotal int64
	if err := filepath.WalkDir(notesRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrTampered
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return ErrTampered
		}
		relative, err := filepath.Rel(notesRoot, filePath)
		if err != nil {
			return err
		}
		manifestRelative := filepath.ToSlash(filepath.Join("notes", relative))
		expected, ok := expectedNotes[manifestRelative]
		if !ok {
			return ErrTampered
		}
		info, err := os.Lstat(filePath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != expected.Size {
			return ErrTampered
		}
		digest, size, err := hashFile(ctx, filePath)
		if err != nil {
			return err
		}
		if digest != expected.SHA256 || size != expected.Size {
			return ErrTampered
		}
		visitedNotes[manifestRelative] = struct{}{}
		if notesTotal > (1<<63-1)-size {
			return ErrTampered
		}
		notesTotal += size
		return nil
	}); err != nil {
		return err
	}
	if len(visitedNotes) != len(expectedNotes) {
		return ErrTampered
	}
	if verifyDatabaseHash {
		if notesTotal > (1<<63-1)-databaseEntry.Size || notesTotal+databaseEntry.Size != manifest.TotalBytes {
			return ErrTampered
		}
	}
	return nil
}

func manifestFilePath(root string, relative string) (string, error) {
	if relative == "" || strings.Contains(relative, "\\") || strings.HasPrefix(relative, "/") || strings.Contains(relative, ":") {
		return "", ErrTampered
	}
	clean := path.Clean(relative)
	if clean != relative || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", ErrTampered
	}
	if clean != "atlasnote.db" && !strings.HasPrefix(clean, "notes/") {
		return "", ErrTampered
	}
	candidate := filepath.Join(root, filepath.FromSlash(clean))
	if !safePathWithin(root, candidate) && filepath.Clean(root) != filepath.Clean(candidate) {
		return "", ErrTampered
	}
	return candidate, nil
}

func buildManifest(ctx context.Context, stageDir string, id string, kind string, spaceID string, schemaVersion int) (Manifest, error) {
	entries := make([]FileEntry, 0)
	for _, relative := range []string{"atlasnote.db"} {
		entry, err := fileEntry(ctx, stageDir, relative)
		if err != nil {
			return Manifest{}, err
		}
		entries = append(entries, entry)
	}
	notesRoot := filepath.Join(stageDir, "notes")
	if err := filepath.WalkDir(notesRoot, func(filePath string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if dirEntry.Type()&os.ModeSymlink != 0 {
			return ErrTampered
		}
		relativeToNotes, err := filepath.Rel(notesRoot, filePath)
		if err != nil {
			return err
		}
		entry, err := fileEntry(ctx, stageDir, filepath.ToSlash(filepath.Join("notes", relativeToNotes)))
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	}); err != nil {
		return Manifest{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var total int64
	for _, entry := range entries {
		total += entry.Size
	}
	return Manifest{
		Version: manifestVersion, ID: id, Kind: kind, SpaceID: spaceID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), SchemaVersion: schemaVersion,
		TotalBytes: total, FileCount: len(entries), Files: entries,
	}, nil
}

func fileEntry(ctx context.Context, root string, relative string) (FileEntry, error) {
	filePath, err := manifestFilePath(root, relative)
	if err != nil {
		return FileEntry{}, err
	}
	digest, size, err := hashFile(ctx, filePath)
	if err != nil {
		return FileEntry{}, err
	}
	return FileEntry{Path: relative, Size: size, SHA256: digest}, nil
}

func copyTree(ctx context.Context, sourceRoot string, destinationRoot string) error {
	info, err := os.Lstat(sourceRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrTampered
	}
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrTampered
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		destinationPath := filepath.Join(destinationRoot, relative)
		if !safePathWithin(destinationRoot, destinationPath) {
			return ErrTampered
		}
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o700)
		}
		if !entry.Type().IsRegular() {
			return ErrTampered
		}
		return copyFile(ctx, sourcePath, destinationPath)
	})
}

func copyFile(ctx context.Context, sourcePath string, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ErrTampered
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = destination.Close()
			_ = os.Remove(destinationPath)
		}
	}()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, err := destination.Write(buffer[:read]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

func hashFile(ctx context.Context, filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			size += int64(read)
			if _, err := hash.Write(buffer[:read]); err != nil {
				return "", 0, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", 0, readErr
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func writeManifest(filePath string, manifest Manifest) error {
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return writeAtomicFile(filePath, encoded)
}

func writeAtomicFile(filePath string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return err
	}
	temporaryID, err := randomID()
	if err != nil {
		return err
	}
	temporaryPath := filepath.Join(filepath.Dir(filePath), "."+filepath.Base(filePath)+"."+temporaryID+".tmp")
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	written, err := file.Write(contents)
	if err != nil {
		return err
	}
	if written != len(contents) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, filePath); err != nil {
		return err
	}
	committed = true
	return nil
}

func pruneGenerations(root string, kind string, maximum int) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	type generation struct {
		path    string
		created time.Time
	}
	generations := make([]generation, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !backupIDPattern.MatchString(entry.Name()) {
			continue
		}
		manifestPath := filepath.Join(root, entry.Name(), manifestName)
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest Manifest
		if json.Unmarshal(raw, &manifest) != nil || manifest.Kind != kind {
			continue
		}
		created, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
		if err != nil {
			continue
		}
		generations = append(generations, generation{path: filepath.Join(root, entry.Name()), created: created})
	}
	sort.Slice(generations, func(i, j int) bool { return generations[i].created.After(generations[j].created) })
	if len(generations) <= maximum {
		return
	}
	for _, item := range generations[maximum:] {
		if safePathWithin(root, item.path) {
			_ = os.RemoveAll(item.path)
		}
	}
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func hashBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func removeDatabaseSidecars(databasePath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(databasePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove sqlite snapshot sidecar: %w", err)
		}
	}
	return nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func wrapTampered(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrTampered, err)
}

func safePathWithin(root string, candidate string) bool {
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	if info, err := os.Lstat(cleanRoot); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false
	}
	cleanCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(cleanRoot, cleanCandidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	current := cleanRoot
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || index < len(parts)-1 && !info.IsDir() {
			return false
		}
	}
	return true
}

func safePathWithinOrEqual(root string, candidate string) bool {
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	cleanCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	if cleanRoot == cleanCandidate {
		info, err := os.Lstat(cleanRoot)
		return errors.Is(err, os.ErrNotExist) || err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
	}
	return safePathWithin(cleanRoot, cleanCandidate)
}
