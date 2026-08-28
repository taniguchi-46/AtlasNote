package contentlock_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
	syncservice "atlasnote/internal/sync"
)

func TestNoteLockEncryptsContentAndRequiresSessionUnlock(t *testing.T) {
	ctx := context.Background()
	manager, notes, store := newLockFixture(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "機密ノート", Content: "confidential body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	lock, aiRecords, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("enable note lock: %v", err)
	}
	if aiRecords != 0 {
		t.Fatalf("AI record count = %d, want 0", aiRecords)
	}
	if !lock.Unlocked {
		t.Fatal("newly enabled lock must be session-unlocked")
	}
	status, err := manager.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("get enabled note lock status: %v", err)
	}
	if !status.Protected || status.Locked || !status.ExplicitLock || status.Source != contentlock.TargetNote {
		t.Fatalf("enabled note lock status = %#v", status)
	}

	raw, err := store.ReadRaw(ctx, created.ID)
	if err != nil {
		t.Fatalf("read raw content: %v", err)
	}
	if strings.Contains(string(raw), created.Content) || !strings.HasPrefix(string(raw), "ATLASNOTE-LOCK-1\n") {
		t.Fatalf("stored note is not protected: %q", raw)
	}
	loaded, err := notes.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get unlocked protected note: %v", err)
	}
	if loaded.Content != created.Content || !loaded.Protected || loaded.Locked {
		t.Fatalf("unexpected unlocked note: %#v", loaded)
	}

	if _, err := manager.LockNow(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID}); err != nil {
		t.Fatalf("lock now: %v", err)
	}
	status, err = manager.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil || !status.Protected || !status.Locked || !status.ExplicitLock {
		t.Fatalf("locked note status = %#v, %v", status, err)
	}
	if _, err := notes.Get(ctx, created.ID); !errors.Is(err, contentlock.ErrLocked) {
		t.Fatalf("get locked note error = %v, want ErrLocked", err)
	}
	if _, err := manager.Unlock(ctx, contentlock.UnlockInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("unlock note: %v", err)
	}
	loaded, err = notes.Get(ctx, created.ID)
	if err != nil || loaded.Content != created.Content {
		t.Fatalf("get after unlock = %#v, %v", loaded, err)
	}

	if err := manager.Disable(ctx, contentlock.DisableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("disable note lock: %v", err)
	}
	raw, err = store.ReadRaw(ctx, created.ID)
	if err != nil {
		t.Fatalf("read unprotected raw content: %v", err)
	}
	if string(raw) != created.Content {
		t.Fatalf("raw content after unlock = %q, want %q", raw, created.Content)
	}
	status, err = manager.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil || status.Protected || status.Locked || status.ExplicitLock {
		t.Fatalf("disabled note lock status = %#v, %v", status, err)
	}
}

func TestNoteLockStatusPersistsAcrossManagerSessions(t *testing.T) {
	ctx := context.Background()
	manager, notes, store, db := newLockFixtureWithDatabase(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "再起動確認", Content: "protected after restart"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable note lock: %v", err)
	}

	manager.Close()
	reopened := contentlock.NewManager(db, store)
	t.Cleanup(reopened.Close)
	notes.SetContentLockGuard(reopened)

	status, err := reopened.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("get restarted note lock status: %v", err)
	}
	if !status.Protected || !status.Locked || !status.ExplicitLock || status.Source != contentlock.TargetNote {
		t.Fatalf("restarted note lock status = %#v", status)
	}
	if _, err := notes.Get(ctx, created.ID); !errors.Is(err, contentlock.ErrLocked) {
		t.Fatalf("get note after manager restart error = %v, want ErrLocked", err)
	}
	if _, err := reopened.Unlock(ctx, contentlock.UnlockInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("unlock restarted note: %v", err)
	}
	if err := reopened.Disable(ctx, contentlock.DisableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("disable restarted note lock: %v", err)
	}
	status, err = reopened.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil || status.Protected || status.Locked || status.ExplicitLock {
		t.Fatalf("disabled restarted note lock status = %#v, %v", status, err)
	}
}

func TestMigratedOrphanedTrashedNoteSupportsLockStatusAndDeletion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "atlasnote.db")
	store, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	legacyDB, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open database for legacy fixture: %v", err)
	}
	legacyNotes := note.NewService(note.NewRepository(legacyDB), store)
	notebook, err := legacyNotes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "削除済み"})
	if err != nil {
		_ = legacyDB.Close()
		t.Fatalf("create notebook: %v", err)
	}
	created, err := legacyNotes.Create(ctx, note.CreateInput{
		NotebookID: &notebook.ID,
		Title:      "ゴミ箱ノート",
		Content:    "本文",
	})
	if err != nil {
		_ = legacyDB.Close()
		t.Fatalf("create note: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close database before legacy mutation: %v", err)
	}

	// Reproduce the legacy deletion result: the notebook row disappears while
	// the note retains its old reference.
	legacyFileDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy file database: %v", err)
	}
	if _, err := legacyFileDB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		_ = legacyFileDB.Close()
		t.Fatalf("disable legacy foreign keys: %v", err)
	}
	if _, err := legacyFileDB.ExecContext(ctx, "DELETE FROM notebooks WHERE id = ?", notebook.ID); err != nil {
		_ = legacyFileDB.Close()
		t.Fatalf("delete legacy notebook: %v", err)
	}
	var legacyNotebookID string
	if err := legacyFileDB.QueryRowContext(ctx, "SELECT notebook_id FROM notes WHERE id = ?", created.ID).Scan(&legacyNotebookID); err != nil {
		_ = legacyFileDB.Close()
		t.Fatalf("read legacy orphan reference: %v", err)
	}
	if legacyNotebookID != notebook.ID {
		_ = legacyFileDB.Close()
		t.Fatalf("legacy notebook id = %q, want %q", legacyNotebookID, notebook.ID)
	}
	if _, err := legacyFileDB.ExecContext(ctx, "PRAGMA user_version = 15"); err != nil {
		_ = legacyFileDB.Close()
		t.Fatalf("set legacy schema version: %v", err)
	}
	if err := legacyFileDB.Close(); err != nil {
		t.Fatalf("close legacy file database: %v", err)
	}

	migratedDB, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("migrate orphaned note database: %v", err)
	}
	t.Cleanup(func() { _ = migratedDB.Close() })
	manager := contentlock.NewManager(migratedDB, store)
	t.Cleanup(manager.Close)
	service := note.NewService(note.NewRepository(migratedDB), store)
	service.SetContentLockGuard(manager)

	status, err := manager.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("get migrated orphan note lock status: %v", err)
	}
	if status.Protected || status.Locked || status.ExplicitLock || status.Source != "" {
		t.Fatalf("migrated orphan note lock status = %#v", status)
	}
	if err := service.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: created.Revision}); err != nil {
		t.Fatalf("delete migrated orphan note: %v", err)
	}
}

func TestAIContentAccessBlocksLockConversion(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "AI送信中", Content: "body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	release := manager.BeginAIContentAccess(ctx)
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	completed := make(chan error, 1)
	go func() {
		_, _, enableErr := manager.Enable(ctx, contentlock.EnableInput{
			TargetType: contentlock.TargetNote,
			TargetID:   created.ID,
			Passphrase: "correct horse battery staple",
		})
		completed <- enableErr
	}()

	select {
	case enableErr := <-completed:
		t.Fatalf("lock conversion completed while AI access was active: %v", enableErr)
	case <-time.After(100 * time.Millisecond):
	}

	release()
	released = true
	select {
	case enableErr := <-completed:
		if enableErr != nil {
			t.Fatalf("enable after AI access release: %v", enableErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock conversion did not resume after AI access release")
	}
}

func TestExportContentAccessBlocksLockNow(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "エクスポート中", Content: "body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable note lock: %v", err)
	}

	release := manager.BeginExportContentAccess(ctx)
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	completed := make(chan error, 1)
	go func() {
		_, lockErr := manager.LockNow(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
		completed <- lockErr
	}()

	select {
	case lockErr := <-completed:
		t.Fatalf("lock now completed while export access was active: %v", lockErr)
	case <-time.After(100 * time.Millisecond):
	}

	release()
	released = true
	select {
	case lockErr := <-completed:
		if lockErr != nil {
			t.Fatalf("lock now after export access release: %v", lockErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock now did not resume after export access release")
	}
}

func TestNotebookLockCoversChildNotebookAndDoesNotPermitAI(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	parent, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "親"})
	if err != nil {
		t.Fatalf("create parent notebook: %v", err)
	}
	child, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "子", ParentID: &parent.ID})
	if err != nil {
		t.Fatalf("create child notebook: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{NotebookID: &child.ID, Title: "配下", Content: "body"})
	if err != nil {
		t.Fatalf("create child note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNotebook,
		TargetID:   parent.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable notebook lock: %v", err)
	}
	notebookStatus, err := manager.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetNotebook, ID: parent.ID})
	if err != nil {
		t.Fatalf("get enabled notebook lock status: %v", err)
	}
	if !notebookStatus.Protected || notebookStatus.Locked || !notebookStatus.ExplicitLock || notebookStatus.Source != contentlock.TargetNotebook {
		t.Fatalf("enabled notebook lock status = %#v", notebookStatus)
	}
	status, err := manager.NoteStatus(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note status: %v", err)
	}
	if !status.Protected || status.ExplicitLock || status.Source != contentlock.TargetNotebook {
		t.Fatalf("unexpected child lock status: %#v", status)
	}
	if err := manager.AssertAIAllowed(ctx, created.ID); !errors.Is(err, contentlock.ErrLocked) {
		t.Fatalf("AI access error = %v, want ErrLocked", err)
	}
}

func TestListRequiredLocksAndBatchLockRespectInheritedScope(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	parent, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "親"})
	if err != nil {
		t.Fatalf("create parent notebook: %v", err)
	}
	child, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "子", ParentID: &parent.ID})
	if err != nil {
		t.Fatalf("create child notebook: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{NotebookID: &child.ID, Title: "配下", Content: "body"})
	if err != nil {
		t.Fatalf("create child note: %v", err)
	}
	for _, input := range []contentlock.EnableInput{
		{TargetType: contentlock.TargetNotebook, TargetID: parent.ID, Passphrase: "parent passphrase"},
		{TargetType: contentlock.TargetNote, TargetID: created.ID, Passphrase: "note passphrase"},
	} {
		if _, _, err := manager.Enable(ctx, input); err != nil {
			t.Fatalf("enable lock for %s: %v", input.TargetType, err)
		}
	}

	required, err := manager.ListRequiredLocks(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("list required unlocked locks: %v", err)
	}
	if len(required) != 0 {
		t.Fatalf("unlocked note requirements = %#v, want none", required)
	}

	locked, err := manager.LockTargetsNow(ctx, []contentlock.Target{
		{Type: contentlock.TargetNotebook, ID: parent.ID},
		{Type: contentlock.TargetNote, ID: created.ID},
	})
	if err != nil {
		t.Fatalf("batch lock: %v", err)
	}
	if len(locked) != 2 {
		t.Fatalf("batch locked = %#v, want two locks", locked)
	}
	required, err = manager.ListRequiredLocks(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("list required locked locks: %v", err)
	}
	if len(required) != 2 {
		t.Fatalf("locked note requirements = %#v, want parent and note locks", required)
	}
	if required[0].Unlocked || required[1].Unlocked {
		t.Fatalf("required locks must not report an available session key: %#v", required)
	}

	if _, err := manager.Unlock(ctx, contentlock.UnlockInput{
		TargetType: contentlock.TargetNotebook,
		TargetID:   parent.ID,
		Passphrase: "parent passphrase",
	}); err != nil {
		t.Fatalf("unlock parent: %v", err)
	}
	required, err = manager.ListRequiredLocks(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID})
	if err != nil {
		t.Fatalf("list partially unlocked requirements: %v", err)
	}
	if len(required) != 1 || required[0].TargetType != contentlock.TargetNote {
		t.Fatalf("partially unlocked requirements = %#v, want note lock only", required)
	}

	// A stale timer target is safe: it must not turn a successful future
	// unlock into an API error when the lock has already been removed.
	if locked, err := manager.LockTargetsNow(ctx, []contentlock.Target{{Type: contentlock.TargetNotebook, ID: child.ID}}); err != nil || len(locked) != 0 {
		t.Fatalf("stale batch target = %#v, %v", locked, err)
	}
}

func TestProtectedContentRejectsTampering(t *testing.T) {
	ctx := context.Background()
	manager, notes, store := newLockFixture(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "改ざん", Content: "body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable note lock: %v", err)
	}
	raw, err := store.ReadRaw(ctx, created.ID)
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	raw[len(raw)-1] ^= 1
	if err := store.WriteTempRaw(ctx, created.ID, "tampered", raw); err != nil {
		t.Fatalf("stage tampered raw content: %v", err)
	}
	if err := store.CommitTemp(ctx, created.ID, "tampered"); err != nil {
		t.Fatalf("commit tampered raw content: %v", err)
	}
	if _, err := notes.Get(ctx, created.ID); !errors.Is(err, contentlock.ErrIntegrity) {
		t.Fatalf("tampered get error = %v, want ErrIntegrity", err)
	}
}

func TestProtectedNoteDoesNotRetainPlaintextSyncPayload(t *testing.T) {
	ctx := context.Background()
	manager, notes, _, db := newLockFixtureWithSync(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "同期対象", Content: "plain body before lock"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable note lock: %v", err)
	}
	exported, err := notes.ExportSyncChanges(ctx)
	if err != nil {
		t.Fatalf("export sync snapshot with protected note: %v", err)
	}
	for _, change := range exported {
		if change.EntityKey == note.SyncEntityKey(note.SyncEntityNote, created.ID) || change.EntityKey == note.SyncEntityKey(note.SyncEntityNoteTags, created.ID) {
			t.Fatalf("protected note leaked into sync snapshot: %#v", change)
		}
	}
	updatedContent := "plain body after lock"
	expectedRevision := created.Revision
	if _, err := notes.Update(ctx, created.ID, note.UpdateInput{
		Content:          &updatedContent,
		ExpectedRevision: &expectedRevision,
	}); err != nil {
		t.Fatalf("update protected note: %v", err)
	}
	var outboxCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_outbox WHERE entity_key = ?", note.SyncEntityKey(note.SyncEntityNote, created.ID)).Scan(&outboxCount); err != nil {
		t.Fatalf("count sync outbox payloads: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("protected note retained %d sync payloads", outboxCount)
	}
}

func TestFailedMoveFromProtectedScopeDoesNotRetainPlaintextSyncPayload(t *testing.T) {
	ctx := context.Background()
	manager, notes, store, db := newLockFixtureWithSync(t)
	notebook, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "保護された移動元"})
	if err != nil {
		t.Fatalf("create protected notebook: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{NotebookID: &notebook.ID, Title: "移動中", Content: "protected body"})
	if err != nil {
		t.Fatalf("create protected note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNotebook,
		TargetID:   notebook.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable notebook lock: %v", err)
	}

	repository := note.NewRepository(db)
	repository.SetSyncChangeRecorder(syncservice.NewRepository(db))
	failingNotes := note.NewService(repository, &commitTempFailingStore{
		MarkdownStore: store,
		err:           errors.New("rename denied"),
	})
	failingNotes.SetContentLockGuard(manager)
	clearNotebook := true
	if _, err := failingNotes.Update(ctx, created.ID, note.UpdateInput{
		ClearNotebook:    &clearNotebook,
		ExpectedRevision: &created.Revision,
	}); err == nil {
		t.Fatal("move with failed Markdown commit unexpectedly succeeded")
	}

	var outboxCount int
	entityKey := note.SyncEntityKey(note.SyncEntityNote, created.ID)
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_outbox WHERE entity_key = ?", entityKey).Scan(&outboxCount); err != nil {
		t.Fatalf("count failed-move sync outbox payloads: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("failed protected move retained %d plaintext sync payloads", outboxCount)
	}
	var stateCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_item_states WHERE entity_key = ?", entityKey).Scan(&stateCount); err != nil {
		t.Fatalf("count failed-move sync states: %v", err)
	}
	if stateCount != 0 {
		t.Fatalf("failed protected move retained %d plaintext sync states", stateCount)
	}
	restored, err := notes.Get(ctx, created.ID)
	if err != nil || restored.NotebookID == nil || *restored.NotebookID != notebook.ID || restored.Content != created.Content {
		t.Fatalf("failed protected move did not restore the original note: %#v, %v", restored, err)
	}
}

func TestExplicitLocksBlockDeletionAndHierarchyMoves(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	parent, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "親"})
	if err != nil {
		t.Fatalf("create parent notebook: %v", err)
	}
	other, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "移動先"})
	if err != nil {
		t.Fatalf("create destination notebook: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{NotebookID: &parent.ID, Title: "保護", Content: "body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote,
		TargetID:   created.ID,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable note lock: %v", err)
	}
	expectedRevision := created.Revision
	if err := notes.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: expectedRevision}); !errors.Is(err, contentlock.ErrValidation) {
		t.Fatalf("delete explicitly locked note error = %v, want content lock validation error", err)
	}
	if _, err := notes.UpdateNotebook(ctx, parent.ID, note.NotebookUpdateInput{ParentID: &other.ID}); !errors.Is(err, note.ErrValidation) {
		t.Fatalf("move hierarchy while locks exist error = %v, want ErrValidation", err)
	}
}

func TestProtectedNotebookTrashDoesNotCreatePlaintextNoteSyncChange(t *testing.T) {
	ctx := context.Background()
	manager, notes, _, db := newLockFixtureWithSync(t)
	notebook, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "保護対象"})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	created, err := notes.Create(ctx, note.CreateInput{NotebookID: &notebook.ID, Title: "同期しない本文", Content: "secret body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetSpace,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable space lock: %v", err)
	}
	if err := notes.DeleteNotebook(ctx, notebook.ID, note.NotebookDeleteInput{Mode: note.NotebookDeleteModeTrashNotes}); err != nil {
		t.Fatalf("trash protected notebook: %v", err)
	}
	var noteOutboxCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_outbox WHERE entity_key = ?", note.SyncEntityKey(note.SyncEntityNote, created.ID)).Scan(&noteOutboxCount); err != nil {
		t.Fatalf("count protected note sync changes: %v", err)
	}
	if noteOutboxCount != 0 {
		t.Fatalf("protected notebook trash retained %d plaintext note sync changes", noteOutboxCount)
	}
}

func TestKeepNotesNotebookDeletionIsRejectedWhenAnyLockExists(t *testing.T) {
	ctx := context.Background()
	manager, notes, _ := newLockFixture(t)
	notebook, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "移動元"})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	if _, err := notes.Create(ctx, note.CreateInput{NotebookID: &notebook.ID, Title: "移動対象", Content: "body"}); err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := manager.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetSpace,
		Passphrase: "correct horse battery staple",
	}); err != nil {
		t.Fatalf("enable space lock: %v", err)
	}
	if err := notes.DeleteNotebook(ctx, notebook.ID, note.NotebookDeleteInput{Mode: note.NotebookDeleteModeKeepNotes}); !errors.Is(err, note.ErrValidation) {
		t.Fatalf("keep notes deletion error = %v, want note.ErrValidation", err)
	}
}

func newLockFixture(t *testing.T) (*contentlock.Manager, *note.Service, *storage.MarkdownStore) {
	t.Helper()
	manager, service, store, _ := newLockFixtureWithDatabase(t)
	return manager, service, store
}

func newLockFixtureWithDatabase(t *testing.T) (*contentlock.Manager, *note.Service, *storage.MarkdownStore, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	manager := contentlock.NewManager(db, store)
	t.Cleanup(manager.Close)
	service := note.NewService(note.NewRepository(db), store)
	service.SetContentLockGuard(manager)
	return manager, service, store, db
}

func newLockFixtureWithSync(t *testing.T) (*contentlock.Manager, *note.Service, *storage.MarkdownStore, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	manager := contentlock.NewManager(db, store)
	t.Cleanup(manager.Close)
	repository := note.NewRepository(db)
	repository.SetSyncChangeRecorder(syncservice.NewRepository(db))
	service := note.NewService(repository, store)
	service.SetContentLockGuard(manager)
	return manager, service, store, db
}

type commitTempFailingStore struct {
	*storage.MarkdownStore
	err error
}

func (s *commitTempFailingStore) CommitTemp(context.Context, string, string) error {
	return s.err
}
