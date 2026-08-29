package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func TestReuploadLocalUsesConditionalHeadAndTombstonesRemoteOnlyItems(t *testing.T) {
	ctx := context.Background()
	db, notes, repository, service, remote := newRecoveryTestService(t, ctx)
	defer db.Close()
	local, err := notes.Create(ctx, note.CreateInput{Title: "Local", Content: "local body"})
	if err != nil {
		t.Fatalf("create local note: %v", err)
	}
	remoteOnlyID := strings.Repeat("d", 32)
	remoteOnlyKey := note.SyncEntityKey(note.SyncEntityNote, remoteOnlyID)
	remotePayload, err := json.Marshal(note.SyncNotePayload{
		ID: remoteOnlyID, Title: "Remote only", Content: "remote body",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal remote payload: %v", err)
	}
	remoteObject, err := objectDocument(note.SyncEntityNote, remoteOnlyKey, remotePayload, false)
	if err != nil {
		t.Fatalf("build remote object: %v", err)
	}
	setFakeRemoteObjects(t, remote, strings.Repeat("f", 32), map[string][]byte{remoteOnlyKey: remoteObject})

	preview, err := service.PrepareRecovery(ctx, RecoveryActionReupload)
	if err != nil {
		t.Fatalf("prepare reupload: %v", err)
	}
	result, err := service.ExecuteRecovery(ctx, RecoveryExecutionInput{Token: preview.Token})
	if err != nil {
		t.Fatalf("execute reupload: %v", err)
	}
	if result.RestartRequired || result.Action != RecoveryActionReupload {
		t.Fatalf("reupload result = %#v", result)
	}

	head := decodeFakeHead(t, remote)
	var manifest ManifestDocument
	if err := json.Unmarshal(remote.files[manifestPath(head.ManifestHash)], &manifest); err != nil {
		t.Fatalf("decode reupload manifest: %v", err)
	}
	entries := make(map[string]ManifestEntry)
	for _, entry := range manifest.Entries {
		entries[entry.EntityKey] = entry
	}
	if _, ok := entries[note.SyncEntityKey(note.SyncEntityNote, local.ID)]; !ok {
		t.Fatalf("local note missing from reupload manifest: %#v", entries)
	}
	remoteOnlyEntry, ok := entries[remoteOnlyKey]
	if !ok {
		t.Fatal("remote-only item was omitted instead of tombstoned")
	}
	remoteOnlyAfter, err := decodeObject(remote.files[objectPath(remoteOnlyEntry.ObjectHash)])
	if err != nil || !remoteOnlyAfter.Deleted {
		t.Fatalf("remote-only object after reupload = %#v err=%v", remoteOnlyAfter, err)
	}
	saved, err := repository.GetConnection(ctx)
	if err != nil || saved == nil || saved.HeadManifestHash != head.ManifestHash || saved.Status != StatusSynced {
		t.Fatalf("local recovery state was not committed: connection=%#v err=%v", saved, err)
	}
}

func TestReuploadLocalLeavesTrackingUntouchedOnHeadRace(t *testing.T) {
	ctx := context.Background()
	db, notes, repository, service, remote := newRecoveryTestService(t, ctx)
	defer db.Close()
	if _, err := notes.Create(ctx, note.CreateInput{Title: "Local", Content: "local body"}); err != nil {
		t.Fatalf("create local note: %v", err)
	}
	preview, err := service.PrepareRecovery(ctx, RecoveryActionReupload)
	if err != nil {
		t.Fatalf("prepare reupload: %v", err)
	}
	remote.beforePut = func(remotePath string, ifMatch string) {
		if remotePath == headPath && ifMatch != "" {
			remote.beforePut = nil
			remote.etags[headPath] = `"raced"`
		}
	}
	_, err = service.ExecuteRecovery(ctx, RecoveryExecutionInput{Token: preview.Token})
	if !errors.Is(err, ErrHeadPrecondition) {
		t.Fatalf("head race error = %v", err)
	}
	saved, getErr := repository.GetConnection(ctx)
	if getErr != nil || saved == nil || saved.HeadManifestHash != "" {
		t.Fatalf("local tracking changed after 412: connection=%#v err=%v", saved, getErr)
	}
}

func TestReuploadLocalPreservesMutationCreatedDuringRecovery(t *testing.T) {
	ctx := context.Background()
	db, notes, repository, service, remote := newRecoveryTestService(t, ctx)
	defer db.Close()
	created, err := notes.Create(ctx, note.CreateInput{Title: "Local", Content: "before recovery"})
	if err != nil {
		t.Fatalf("create local note: %v", err)
	}
	preview, err := service.PrepareRecovery(ctx, RecoveryActionReupload)
	if err != nil {
		t.Fatalf("prepare reupload: %v", err)
	}

	reachedHeadWrite := make(chan struct{})
	allowHeadWrite := make(chan struct{}, 1)
	defer func() {
		select {
		case allowHeadWrite <- struct{}{}:
		default:
		}
	}()
	remote.beforePut = func(remotePath string, ifMatch string) {
		if remotePath == headPath && ifMatch != "" {
			close(reachedHeadWrite)
			<-allowHeadWrite
		}
	}
	executionDone := make(chan error, 1)
	go func() {
		_, executeErr := service.ExecuteRecovery(ctx, RecoveryExecutionInput{Token: preview.Token})
		executionDone <- executeErr
	}()
	select {
	case <-reachedHeadWrite:
	case <-time.After(3 * time.Second):
		t.Fatal("reupload did not reach the conditional head write")
	}

	updatedContent := "created while recovery was committing"
	mutationDone := make(chan error, 1)
	go func() {
		_, updateErr := notes.Update(ctx, created.ID, note.UpdateInput{
			Content:          &updatedContent,
			ExpectedRevision: &created.Revision,
		})
		mutationDone <- updateErr
	}()
	select {
	case mutationErr := <-mutationDone:
		t.Fatalf("local mutation completed before recovery commit: %v", mutationErr)
	case <-time.After(100 * time.Millisecond):
	}

	allowHeadWrite <- struct{}{}
	select {
	case executeErr := <-executionDone:
		if executeErr != nil {
			t.Fatalf("execute reupload: %v", executeErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reupload did not complete")
	}
	select {
	case mutationErr := <-mutationDone:
		if mutationErr != nil {
			t.Fatalf("local mutation after recovery commit: %v", mutationErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("local mutation did not resume after recovery")
	}
	outboxCount, err := repository.CountOutbox(ctx)
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount == 0 {
		t.Fatal("local mutation was cleared from the outbox by recovery commit")
	}
}

func TestRedownloadStagesThenSwapsOnRestartWithBackup(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "atlasnote.db")
	notesDir := filepath.Join(dataDir, "notes")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	markdownStore, err := storage.NewMarkdownStore(notesDir)
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	noteRepository := note.NewRepository(db)
	repository := NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	notes := note.NewService(noteRepository, markdownStore)
	vaultID := strings.Repeat("a", 32)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: vaultID, Status: StatusIdle, CredentialRef: "ref", FailSafe: true,
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{Title: "Remote source", Content: "remote body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	credentials := NewCredentialManager(NewSessionCredentialStore())
	_, _ = credentials.Save("ref", "secret", false)
	remote := newFakeRemote()
	service := NewService(repository, notes, credentials)
	service.SetRecoveryDataDir(dataDir)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })
	if _, err := service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil {
		t.Fatalf("upload remote fixture: %v", err)
	}

	noteRepository.SetSyncChangeRecorder(nil)
	current, err := notes.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get local note: %v", err)
	}
	localBody := "local body that must be backed up"
	if _, err := notes.Update(ctx, created.ID, note.UpdateInput{Content: &localBody, ExpectedRevision: &current.Revision}); err != nil {
		t.Fatalf("change local note: %v", err)
	}
	preview, err := service.PrepareRecovery(ctx, RecoveryActionRedownload)
	if err != nil {
		t.Fatalf("prepare redownload: %v", err)
	}
	result, err := service.ExecuteRecovery(ctx, RecoveryExecutionInput{Token: preview.Token})
	if err != nil {
		t.Fatalf("stage redownload: %v", err)
	}
	if !result.RestartRequired {
		t.Fatalf("redownload result = %#v", result)
	}
	markerBytes, err := os.ReadFile(filepath.Join(dataDir, ".sync-recovery", "pending.json"))
	if err != nil {
		t.Fatalf("read recovery marker: %v", err)
	}
	markerText := string(markerBytes)
	for _, secret := range []string{"secret", "alice", "dav.example.test"} {
		if strings.Contains(markerText, secret) {
			t.Fatalf("recovery marker contains secret or target data: %s", markerText)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close active database: %v", err)
	}

	backupPath, err := ApplyPendingRecovery(RecoveryPaths{DataDir: dataDir, DatabasePath: databasePath, NotesDir: notesDir})
	if err != nil {
		t.Fatalf("apply pending recovery: %v", err)
	}
	if backupPath != result.BackupPath || !pathExists(filepath.Join(backupPath, "atlasnote.db")) || !pathExists(filepath.Join(backupPath, "notes")) {
		t.Fatalf("backup was not retained: returned=%q result=%q", backupPath, result.BackupPath)
	}
	newDB, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open recovered database: %v", err)
	}
	defer newDB.Close()
	newStore, err := storage.NewMarkdownStore(notesDir)
	if err != nil {
		t.Fatalf("open recovered notes: %v", err)
	}
	recovered, err := note.NewService(note.NewRepository(newDB), newStore).Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get recovered note: %v", err)
	}
	if recovered.Content != "remote body" {
		t.Fatalf("recovered content = %q", recovered.Content)
	}
}

func TestPendingRecoveryRollsBackWhenStageInstallFails(t *testing.T) {
	dataDir := t.TempDir()
	paths := RecoveryPaths{
		DataDir: dataDir, DatabasePath: filepath.Join(dataDir, "atlasnote.db"), NotesDir: filepath.Join(dataDir, "notes"),
	}
	id := strings.Repeat("b", 32)
	stageDir := filepath.Join(dataDir, ".sync-recovery", "staging", id)
	if err := os.MkdirAll(filepath.Join(stageDir, "notes"), 0o700); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	if err := os.MkdirAll(paths.NotesDir, 0o700); err != nil {
		t.Fatalf("create active notes: %v", err)
	}
	if err := os.WriteFile(paths.DatabasePath, []byte("old-db"), 0o600); err != nil {
		t.Fatalf("write active database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.NotesDir, "old.md"), []byte("old-note"), 0o600); err != nil {
		t.Fatalf("write active note: %v", err)
	}
	stageDatabase := filepath.Join(stageDir, "atlasnote.db")
	if err := os.WriteFile(stageDatabase, []byte("new-db"), 0o600); err != nil {
		t.Fatalf("write stage database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, "notes", "new.md"), []byte("new-note"), 0o600); err != nil {
		t.Fatalf("write stage note: %v", err)
	}
	markerPath := filepath.Join(dataDir, ".sync-recovery", "pending.json")
	if err := writeRecoveryMarker(markerPath, recoveryMarker{Version: recoveryMarkerVersion, ID: id, Action: RecoveryActionRedownload, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	wantFailure := errors.New("injected stage install failure")
	_, err := applyPendingRecovery(paths, func(source string, destination string) error {
		if source == stageDatabase && destination == paths.DatabasePath {
			return wantFailure
		}
		return os.Rename(source, destination)
	})
	if !errors.Is(err, wantFailure) {
		t.Fatalf("apply recovery error = %v", err)
	}
	activeDatabase, readErr := os.ReadFile(paths.DatabasePath)
	if readErr != nil || string(activeDatabase) != "old-db" || !pathExists(filepath.Join(paths.NotesDir, "old.md")) {
		t.Fatalf("rollback did not restore active vault: db=%q readErr=%v", activeDatabase, readErr)
	}
	if !pathExists(stageDatabase) || !pathExists(filepath.Join(stageDir, "notes", "new.md")) {
		t.Fatal("rollback did not restore staged vault for a safe retry")
	}
}

func newRecoveryTestService(t *testing.T, ctx context.Context) (*sql.DB, *note.Service, *Repository, *Service, *fakeRemote) {
	t.Helper()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	markdownStore, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		db.Close()
		t.Fatalf("create markdown store: %v", err)
	}
	repository := NewRepository(db)
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	notes := note.NewService(noteRepository, markdownStore)
	vaultID := strings.Repeat("f", 32)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: vaultID, Status: StatusIdle, CredentialRef: "ref", FailSafe: true,
	}); err != nil {
		db.Close()
		t.Fatalf("save connection: %v", err)
	}
	credentials := NewCredentialManager(NewSessionCredentialStore())
	_, _ = credentials.Save("ref", "secret", false)
	remote := newFakeRemote()
	if err := initializeRemote(ctx, remote, vaultID); err != nil {
		db.Close()
		t.Fatalf("initialize remote: %v", err)
	}
	service := NewService(repository, notes, credentials)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })
	return db, notes, repository, service, remote
}

func setFakeRemoteObjects(t *testing.T, remote *fakeRemote, vaultID string, objects map[string][]byte) {
	t.Helper()
	oldHead := decodeFakeHead(t, remote)
	entries := make(map[string]ManifestEntry)
	for key, raw := range objects {
		object, err := decodeObject(raw)
		if err != nil {
			t.Fatalf("decode fixture object: %v", err)
		}
		hash := hashBytes(raw)
		if _, err := remote.Put(context.Background(), objectPath(hash), raw, "", "*"); err != nil {
			t.Fatalf("upload fixture object: %v", err)
		}
		entries[key] = ManifestEntry{EntityKey: key, EntityType: object.EntityType, ObjectHash: hash}
	}
	manifestBytes, err := canonicalJSON(manifestFor(vaultID, oldHead.Generation+1, entries))
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	manifestHash := hashBytes(manifestBytes)
	if _, err := remote.Put(context.Background(), manifestPath(manifestHash), manifestBytes, "", "*"); err != nil {
		t.Fatalf("upload fixture manifest: %v", err)
	}
	headBytes, err := canonicalJSON(HeadDocument{FormatVersion: FormatVersion, VaultID: vaultID, Generation: oldHead.Generation + 1, ManifestHash: manifestHash})
	if err != nil {
		t.Fatalf("encode fixture head: %v", err)
	}
	if _, err := remote.Put(context.Background(), headPath, headBytes, remote.etags[headPath], ""); err != nil {
		t.Fatalf("upload fixture head: %v", err)
	}
}

func decodeFakeHead(t *testing.T, remote *fakeRemote) HeadDocument {
	t.Helper()
	var head HeadDocument
	if err := json.Unmarshal(remote.files[headPath], &head); err != nil {
		t.Fatalf("decode fake head: %v", err)
	}
	return head
}
