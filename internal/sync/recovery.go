package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

const (
	RecoveryActionReupload   = "reupload-local"
	RecoveryActionRedownload = "redownload-remote"
	recoveryMarkerVersion    = 1
	recoveryTokenLifetime    = 5 * time.Minute
)

var (
	ErrRecoveryAuthorization = errors.New("sync recovery confirmation is invalid or expired")
	ErrRecoveryPending       = errors.New("a sync recovery is already waiting for restart")
	ErrRecoveryPath          = errors.New("sync recovery path is invalid")
	recoveryIDPattern        = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

type RecoveryPreview struct {
	Token       string `json:"token"`
	Action      string `json:"action"`
	LocalItems  int    `json:"localItems"`
	RemoteItems int    `json:"remoteItems"`
	Message     string `json:"message"`
}

type RecoveryExecutionInput struct {
	Token string `json:"token"`
}

type RecoveryResult struct {
	Action          string `json:"action"`
	RestartRequired bool   `json:"restartRequired"`
	BackupPath      string `json:"backupPath,omitempty"`
	Message         string `json:"message"`
}

type recoveryAuthorization struct {
	Action    string
	Binding   string
	ExpiresAt time.Time
}

type recoverySyncedItem struct {
	EntityKey  string
	EntityType string
	ObjectHash string
	ObjectJSON string
}

type recoveryContext struct {
	connection       Connection
	client           RemoteClient
	remote           remoteState
	localChanges     []note.SyncChange
	localFingerprint string
	binding          string
}

type recoveryMarker struct {
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Action    string `json:"action"`
	CreatedAt string `json:"createdAt"`
}

type RecoveryPaths struct {
	DataDir      string
	DatabasePath string
	NotesDir     string
}

func (s *Service) PrepareRecovery(ctx context.Context, action string) (RecoveryPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, unlockNotes := s.lockNotesForSync(ctx)
	defer unlockNotes()

	if action != RecoveryActionReupload && action != RecoveryActionRedownload {
		return RecoveryPreview{}, ErrRecoveryAuthorization
	}
	if action == RecoveryActionRedownload && strings.TrimSpace(s.recoveryDataDir) == "" {
		return RecoveryPreview{}, ErrRecoveryPath
	}
	recovery, err := s.loadRecoveryContext(ctx)
	if err != nil {
		return RecoveryPreview{}, err
	}
	if action == RecoveryActionRedownload {
		if err := s.checkEmptyRemoteFailSafe(ctx, recovery.connection, recovery.remote, true); err != nil {
			return RecoveryPreview{}, err
		}
	}
	token, err := randomHex(16)
	if err != nil {
		return RecoveryPreview{}, err
	}
	now := time.Now().UTC()
	for key, authorization := range s.recoveryTokens {
		if now.After(authorization.ExpiresAt) {
			delete(s.recoveryTokens, key)
		}
	}
	s.recoveryTokens[token] = recoveryAuthorization{
		Action: action, Binding: recovery.binding, ExpiresAt: now.Add(recoveryTokenLifetime),
	}
	message := "ローカルデータを正として同期先を条件付き更新します。同期先だけにある項目はtombstoneとして保持されます。"
	if action == RecoveryActionRedownload {
		message = "同期先を一時領域へ完全に検証し、再起動時に現在のローカル保管庫をバックアップしてから置換します。"
	}
	return RecoveryPreview{
		Token: token, Action: action, LocalItems: len(recovery.localChanges),
		RemoteItems: len(recovery.remote.manifest.Entries), Message: message,
	}, nil
}

func (s *Service) ExecuteRecovery(ctx context.Context, input RecoveryExecutionInput) (RecoveryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, unlockNotes := s.lockNotesForSync(ctx)
	defer unlockNotes()

	authorization, ok := s.recoveryTokens[input.Token]
	delete(s.recoveryTokens, input.Token)
	if !ok || time.Now().UTC().After(authorization.ExpiresAt) {
		return RecoveryResult{}, ErrRecoveryAuthorization
	}
	recovery, err := s.loadRecoveryContext(ctx)
	if err != nil {
		return RecoveryResult{}, err
	}
	if recovery.binding != authorization.Binding {
		return RecoveryResult{}, ErrRecoveryAuthorization
	}
	switch authorization.Action {
	case RecoveryActionReupload:
		return s.reuploadLocal(ctx, recovery)
	case RecoveryActionRedownload:
		if err := s.checkEmptyRemoteFailSafe(ctx, recovery.connection, recovery.remote, true); err != nil {
			return RecoveryResult{}, err
		}
		return s.stageRemoteRedownload(ctx, recovery)
	default:
		return RecoveryResult{}, ErrRecoveryAuthorization
	}
}

func (s *Service) loadRecoveryContext(ctx context.Context) (recoveryContext, error) {
	connection, err := s.repository.GetConnection(ctx)
	if err != nil {
		return recoveryContext{}, err
	}
	if connection == nil {
		return recoveryContext{}, ErrRemoteNotInitialized
	}
	if s.notes == nil {
		return recoveryContext{}, errors.New("note service is unavailable")
	}
	secret, err := s.credentials.Get(connection.CredentialRef)
	if err != nil {
		return recoveryContext{}, ErrCredentialsUnavailable
	}
	client, err := s.clientFactory(*connection, secret)
	if err != nil {
		return recoveryContext{}, err
	}
	remote, err := s.loadRemoteState(ctx, client, *connection, false)
	if err != nil {
		return recoveryContext{}, err
	}
	changes, err := s.notes.ExportSyncChanges(ctx)
	if err != nil {
		return recoveryContext{}, err
	}
	fingerprint, err := localChangesFingerprint(changes)
	if err != nil {
		return recoveryContext{}, err
	}
	binding := strings.Join([]string{
		JoinWebDAVURL(connection.Endpoint, connection.RemoteRoot), connection.Username,
		connection.VaultID, remote.head.ManifestHash, remote.headETag, fingerprint,
	}, "\n")
	return recoveryContext{
		connection: *connection, client: client, remote: remote,
		localChanges: changes, localFingerprint: fingerprint, binding: binding,
	}, nil
}

func localChangesFingerprint(changes []note.SyncChange) (string, error) {
	values := make([]string, 0, len(changes))
	for _, change := range changes {
		raw, err := objectDocument(change.EntityType, change.EntityKey, change.ObjectJSON, change.Deleted)
		if err != nil {
			return "", err
		}
		values = append(values, change.EntityKey+":"+hashBytes(raw))
	}
	sort.Strings(values)
	return hashBytes([]byte(strings.Join(values, "\n"))), nil
}

func (s *Service) reuploadLocal(ctx context.Context, recovery recoveryContext) (RecoveryResult, error) {
	entries := make(map[string]ManifestEntry, len(recovery.localChanges)+len(recovery.remote.entries))
	items := make([]recoverySyncedItem, 0, len(recovery.localChanges)+len(recovery.remote.entries))
	for _, change := range recovery.localChanges {
		raw, err := objectDocument(change.EntityType, change.EntityKey, change.ObjectJSON, change.Deleted)
		if err != nil {
			return RecoveryResult{}, err
		}
		hash := hashBytes(raw)
		if err := putImmutable(ctx, recovery.client, objectPath(hash), raw); err != nil {
			return RecoveryResult{}, err
		}
		entries[change.EntityKey] = ManifestEntry{EntityKey: change.EntityKey, EntityType: change.EntityType, ObjectHash: hash}
		items = append(items, recoverySyncedItem{EntityKey: change.EntityKey, EntityType: change.EntityType, ObjectHash: hash, ObjectJSON: string(raw)})
	}
	for key, entry := range recovery.remote.entries {
		if _, exists := entries[key]; exists {
			continue
		}
		raw, err := s.fetchRemoteObject(ctx, recovery.client, entry)
		if err != nil {
			return RecoveryResult{}, err
		}
		object, err := decodeObject(raw)
		if err != nil {
			return RecoveryResult{}, err
		}
		if !object.Deleted {
			raw, err = objectDocument(entry.EntityType, entry.EntityKey, nil, true)
			if err != nil {
				return RecoveryResult{}, err
			}
		}
		hash := hashBytes(raw)
		if err := putImmutable(ctx, recovery.client, objectPath(hash), raw); err != nil {
			return RecoveryResult{}, err
		}
		entries[key] = ManifestEntry{EntityKey: key, EntityType: entry.EntityType, ObjectHash: hash}
		items = append(items, recoverySyncedItem{EntityKey: key, EntityType: entry.EntityType, ObjectHash: hash, ObjectJSON: string(raw)})
	}

	manifest := manifestFor(recovery.remote.format.VaultID, recovery.remote.head.Generation+1, entries)
	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		return RecoveryResult{}, err
	}
	manifestHash := hashBytes(manifestBytes)
	if err := putImmutable(ctx, recovery.client, manifestPath(manifestHash), manifestBytes); err != nil {
		return RecoveryResult{}, err
	}
	currentHead, err := getResponseWithStrongETag(ctx, recovery.client, headPath)
	if err != nil {
		return RecoveryResult{}, classifyRemoteError(err)
	}
	if currentHead.ETag != recovery.remote.headETag || requireStrongETag(currentHead.ETag) != nil {
		return RecoveryResult{}, ErrHeadPrecondition
	}
	var currentHeadDocument HeadDocument
	if err := json.Unmarshal(currentHead.Body, &currentHeadDocument); err != nil ||
		validateHead(currentHeadDocument, recovery.remote.format) != nil || currentHeadDocument != recovery.remote.head {
		return RecoveryResult{}, ErrHeadPrecondition
	}
	newHeadBytes, err := canonicalJSON(HeadDocument{
		FormatVersion: FormatVersion, VaultID: recovery.remote.format.VaultID,
		Generation: manifest.Generation, ManifestHash: manifestHash,
	})
	if err != nil {
		return RecoveryResult{}, err
	}
	_, err = recovery.client.Put(ctx, headPath, newHeadBytes, recovery.remote.headETag, "")
	if err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusPreconditionFailed {
			return RecoveryResult{}, ErrHeadPrecondition
		}
		return RecoveryResult{}, classifyRemoteError(err)
	}
	verified, err := getResponseWithStrongETag(ctx, recovery.client, headPath)
	if err != nil {
		return RecoveryResult{}, classifyRemoteError(err)
	}
	var verifiedHead HeadDocument
	if err := json.Unmarshal(verified.Body, &verifiedHead); err != nil || validateHead(verifiedHead, recovery.remote.format) != nil ||
		verifiedHead.ManifestHash != manifestHash || verifiedHead.Generation != manifest.Generation ||
		requireStrongETag(verified.ETag) != nil {
		return RecoveryResult{}, ErrInvalidRemoteFormat
	}
	sort.Slice(items, func(i, j int) bool { return items[i].EntityKey < items[j].EntityKey })
	if err := s.repository.CommitRecoveryState(ctx, manifestHash, verified.ETag, items); err != nil {
		return RecoveryResult{}, err
	}
	return RecoveryResult{Action: RecoveryActionReupload, Message: "ローカルデータを同期先へ安全に再アップロードしました。"}, nil
}

func putImmutable(ctx context.Context, client RemoteClient, remotePath string, body []byte) error {
	if _, err := client.Put(ctx, remotePath, body, "", "*"); err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return classifyRemoteError(err)
		}
		existing, getErr := client.Get(ctx, remotePath)
		if getErr != nil || hashBytes(existing.Body) != hashBytes(body) {
			return ErrInvalidRemoteFormat
		}
	}
	return nil
}

func (s *Service) stageRemoteRedownload(ctx context.Context, recovery recoveryContext) (result RecoveryResult, returnErr error) {
	dataDir := filepath.Clean(s.recoveryDataDir)
	recoveryRoot := filepath.Join(dataDir, ".sync-recovery")
	markerPath := filepath.Join(recoveryRoot, "pending.json")
	if _, err := os.Stat(markerPath); err == nil {
		return RecoveryResult{}, ErrRecoveryPending
	} else if !errors.Is(err, os.ErrNotExist) {
		return RecoveryResult{}, err
	}
	id, err := randomHex(16)
	if err != nil {
		return RecoveryResult{}, err
	}
	stageDir := filepath.Join(recoveryRoot, "staging", id)
	if !pathWithin(dataDir, stageDir) {
		return RecoveryResult{}, ErrRecoveryPath
	}
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return RecoveryResult{}, fmt.Errorf("create sync recovery staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stageDir)
		}
	}()

	stageDatabasePath := filepath.Join(stageDir, "atlasnote.db")
	stageNotesDir := filepath.Join(stageDir, "notes")
	db, err := database.Open(ctx, stageDatabasePath)
	if err != nil {
		return RecoveryResult{}, err
	}
	dbClosed := false
	defer func() {
		if !dbClosed {
			_ = db.Close()
		}
	}()
	markdownStore, err := storage.NewMarkdownStore(stageNotesDir)
	if err != nil {
		return RecoveryResult{}, err
	}
	stageNotes := note.NewService(note.NewRepository(db), markdownStore)
	stageRepository := NewRepository(db)
	orderedEntries, prefetched, err := s.orderRemoteEntries(ctx, recovery.client, recovery.remote)
	if err != nil {
		return RecoveryResult{}, err
	}
	items := make([]recoverySyncedItem, 0, len(orderedEntries))
	for _, entry := range orderedEntries {
		raw := prefetched[entry.EntityKey]
		if len(raw) == 0 {
			raw, err = s.fetchRemoteObject(ctx, recovery.client, entry)
			if err != nil {
				return RecoveryResult{}, err
			}
		}
		if err := applyRemoteObjectTo(ctx, stageNotes, raw); err != nil {
			return RecoveryResult{}, err
		}
		items = append(items, recoverySyncedItem{EntityKey: entry.EntityKey, EntityType: entry.EntityType, ObjectHash: entry.ObjectHash, ObjectJSON: string(raw)})
	}
	stagedConnection := recovery.connection
	stagedConnection.HeadManifestHash = recovery.remote.head.ManifestHash
	stagedConnection.HeadETag = recovery.remote.headETag
	stagedConnection.LastSyncAt = time.Now().UTC()
	stagedConnection.HasLastSync = true
	stagedConnection.Status = StatusSynced
	if err := stageRepository.SaveConnection(ctx, stagedConnection); err != nil {
		return RecoveryResult{}, err
	}
	if err := stageRepository.CommitRecoveryState(ctx, recovery.remote.head.ManifestHash, recovery.remote.headETag, items); err != nil {
		return RecoveryResult{}, err
	}
	if _, err := stageNotes.Recover(ctx); err != nil {
		return RecoveryResult{}, err
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return RecoveryResult{}, errors.New("staged sync recovery database failed integrity validation")
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return RecoveryResult{}, fmt.Errorf("checkpoint staged sync recovery database: %w", err)
	}
	if err := db.Close(); err != nil {
		return RecoveryResult{}, err
	}
	dbClosed = true
	marker := recoveryMarker{Version: recoveryMarkerVersion, ID: id, Action: RecoveryActionRedownload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := writeRecoveryMarker(markerPath, marker); err != nil {
		return RecoveryResult{}, err
	}
	committed = true
	backupPath := filepath.Join(recoveryRoot, "backups", id)
	return RecoveryResult{
		Action: RecoveryActionRedownload, RestartRequired: true, BackupPath: backupPath,
		Message: "同期先の検証が完了しました。アプリを終了し、次回起動時にバックアップ付きで置換します。",
	}, nil
}

func writeRecoveryMarker(markerPath string, marker recoveryMarker) error {
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o700); err != nil {
		return fmt.Errorf("create sync recovery marker directory: %w", err)
	}
	encoded, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	temporaryPath := markerPath + ".tmp"
	if err := os.WriteFile(temporaryPath, encoded, 0o600); err != nil {
		return fmt.Errorf("write sync recovery marker: %w", err)
	}
	if err := os.Rename(temporaryPath, markerPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("commit sync recovery marker: %w", err)
	}
	return nil
}

func ApplyPendingRecovery(paths RecoveryPaths) (string, error) {
	return applyPendingRecovery(paths, os.Rename)
}

func applyPendingRecovery(paths RecoveryPaths, rename func(string, string) error) (string, error) {
	dataDir := filepath.Clean(paths.DataDir)
	recoveryRoot := filepath.Join(dataDir, ".sync-recovery")
	markerPath := filepath.Join(recoveryRoot, "pending.json")
	encoded, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read sync recovery marker: %w", err)
	}
	var marker recoveryMarker
	if err := json.Unmarshal(encoded, &marker); err != nil || marker.Version != recoveryMarkerVersion || marker.Action != RecoveryActionRedownload || !recoveryIDPattern.MatchString(marker.ID) {
		return "", ErrRecoveryPath
	}
	stageDir := filepath.Join(recoveryRoot, "staging", marker.ID)
	backupDir := filepath.Join(recoveryRoot, "backups", marker.ID)
	if !pathWithin(dataDir, stageDir) || !pathWithin(dataDir, backupDir) || !pathWithin(dataDir, paths.DatabasePath) || !pathWithin(dataDir, paths.NotesDir) {
		return "", ErrRecoveryPath
	}
	stageDatabase := filepath.Join(stageDir, "atlasnote.db")
	stageNotes := filepath.Join(stageDir, "notes")
	backupDatabase := filepath.Join(backupDir, "atlasnote.db")
	backupNotes := filepath.Join(backupDir, "notes")
	if !validRecoverySource(stageDatabase, paths.DatabasePath, backupDatabase) || !validRecoverySource(stageNotes, paths.NotesDir, backupNotes) {
		return "", ErrRecoveryPath
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("create sync recovery backup directory: %w", err)
	}
	rollback := func(cause error) (string, error) {
		rollbackRecoverySwap(paths, stageDatabase, stageNotes, backupDatabase, backupNotes, rename)
		return "", cause
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		activePath := paths.DatabasePath + suffix
		backupPath := backupDatabase + suffix
		if !pathExists(backupPath) && pathExists(activePath) {
			if err := rename(activePath, backupPath); err != nil {
				return rollback(fmt.Errorf("back up local sync database: %w", err))
			}
		}
	}
	if !pathExists(backupNotes) && pathExists(paths.NotesDir) {
		if err := rename(paths.NotesDir, backupNotes); err != nil {
			return rollback(fmt.Errorf("back up local notes: %w", err))
		}
	}
	if pathExists(stageDatabase) {
		if pathExists(paths.DatabasePath) {
			return rollback(ErrRecoveryPath)
		}
		if err := rename(stageDatabase, paths.DatabasePath); err != nil {
			return rollback(fmt.Errorf("install staged sync database: %w", err))
		}
	}
	if pathExists(stageNotes) {
		if pathExists(paths.NotesDir) {
			return rollback(ErrRecoveryPath)
		}
		if err := rename(stageNotes, paths.NotesDir); err != nil {
			return rollback(fmt.Errorf("install staged notes: %w", err))
		}
	}
	if !pathExists(paths.DatabasePath) || !pathExists(paths.NotesDir) {
		return rollback(ErrRecoveryPath)
	}
	if err := os.Remove(markerPath); err != nil {
		return "", fmt.Errorf("complete sync recovery marker: %w", err)
	}
	return backupDir, nil
}

func rollbackRecoverySwap(paths RecoveryPaths, stageDatabase string, stageNotes string, backupDatabase string, backupNotes string, rename func(string, string) error) {
	if !pathExists(stageNotes) && pathExists(paths.NotesDir) && pathExists(backupNotes) {
		_ = rename(paths.NotesDir, stageNotes)
	}
	if !pathExists(stageDatabase) && pathExists(paths.DatabasePath) && pathExists(backupDatabase) {
		_ = rename(paths.DatabasePath, stageDatabase)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		activePath := paths.DatabasePath + suffix
		backupPath := backupDatabase + suffix
		if !pathExists(activePath) && pathExists(backupPath) {
			_ = rename(backupPath, activePath)
		}
	}
	if !pathExists(paths.NotesDir) && pathExists(backupNotes) {
		_ = rename(backupNotes, paths.NotesDir)
	}
}

func validRecoverySource(stagePath string, activePath string, backupPath string) bool {
	if pathExists(stagePath) {
		return pathExists(activePath) || pathExists(backupPath)
	}
	return pathExists(activePath) && pathExists(backupPath)
}

func pathWithin(root string, candidate string) bool {
	cleanRoot := filepath.Clean(root)
	relative, err := filepath.Rel(cleanRoot, filepath.Clean(candidate))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	current := cleanRoot
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

func pathExists(value string) bool {
	_, err := os.Stat(value)
	return err == nil
}
