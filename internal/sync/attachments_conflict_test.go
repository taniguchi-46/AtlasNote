package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	attachmentstore "atlasnote/internal/attachment"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

type syncAttachmentTestDevice struct {
	repository  *Repository
	notes       *note.Service
	attachments *attachmentstore.Store
	service     *Service
	notesDir    string
	db          *sql.DB
}

func newSyncAttachmentTestDevice(t *testing.T, remote *fakeRemote, vaultID string) *syncAttachmentTestDevice {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open device database: %v", err)
	}
	repository := NewRepository(db)
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	notesDir := filepath.Join(root, "notes")
	markdownStore, err := storage.NewMarkdownStore(notesDir)
	if err != nil {
		t.Fatalf("create device markdown store: %v", err)
	}
	attachments, err := attachmentstore.NewStore(notesDir, nil)
	if err != nil {
		t.Fatalf("create device attachment store: %v", err)
	}
	notes := note.NewService(noteRepository, markdownStore)
	notes.SetAttachmentStore(attachments)
	credentials := NewCredentialManager(NewSessionCredentialStore())
	if _, err := credentials.Save("ref", "secret", false); err != nil {
		t.Fatalf("save device credential: %v", err)
	}
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: vaultID, Status: StatusIdle, CredentialRef: "ref", FailSafe: true,
	}); err != nil {
		t.Fatalf("save device connection: %v", err)
	}
	service := NewService(repository, notes, credentials)
	service.SetAttachmentStore(attachments)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })
	device := &syncAttachmentTestDevice{
		repository: repository, notes: notes, attachments: attachments, service: service, notesDir: notesDir, db: db,
	}
	t.Cleanup(func() {
		if device.db != nil {
			if err := device.db.Close(); err != nil {
				t.Errorf("close device database: %v", err)
			}
		}
	})
	return device
}

func restartSyncAttachmentTestDevice(t *testing.T, device *syncAttachmentTestDevice, remote *fakeRemote) {
	t.Helper()
	ctx := context.Background()
	if device.db == nil {
		t.Fatal("test device database is already closed")
	}
	if err := device.db.Close(); err != nil {
		t.Fatalf("close device database for restart: %v", err)
	}
	root := filepath.Dir(device.notesDir)
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("reopen device database: %v", err)
	}
	repository := NewRepository(db)
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	markdownStore, err := storage.NewMarkdownStore(device.notesDir)
	if err != nil {
		_ = db.Close()
		t.Fatalf("reopen device markdown store: %v", err)
	}
	attachments, err := attachmentstore.NewStore(device.notesDir, nil)
	if err != nil {
		_ = db.Close()
		t.Fatalf("reopen device attachment store: %v", err)
	}
	notes := note.NewService(noteRepository, markdownStore)
	notes.SetAttachmentStore(attachments)
	credentials := NewCredentialManager(NewSessionCredentialStore())
	if _, err := credentials.Save("ref", "secret", false); err != nil {
		_ = db.Close()
		t.Fatalf("save restarted device credential: %v", err)
	}
	service := NewService(repository, notes, credentials)
	service.SetAttachmentStore(attachments)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })
	device.repository = repository
	device.notes = notes
	device.attachments = attachments
	device.service = service
	device.db = db
	if err := attachments.Recover(ctx); err != nil {
		t.Fatalf("recover restarted device attachments: %v", err)
	}
	if _, err := repository.GetConnection(ctx); err != nil {
		t.Fatalf("read restarted device connection: %v", err)
	}
}

func createSyncAttachmentTestNote(t *testing.T, device *syncAttachmentTestDevice) (note.Note, attachmentstore.Metadata, string) {
	t.Helper()
	ctx := context.Background()
	created, err := device.notes.Create(ctx, note.CreateInput{Title: "Shared note", Content: "body"})
	if err != nil {
		t.Fatalf("create sync note: %v", err)
	}
	imageBytes, err := syncTestImageBytes()
	if err != nil {
		t.Fatalf("decode sync test image: %v", err)
	}
	saved, err := device.attachments.Save(ctx, attachmentstore.SaveInput{
		NoteID: created.ID, Kind: "image", MIMEType: "image/png", Name: "shared.png", Data: imageBytes,
	})
	if err != nil {
		t.Fatalf("save sync test attachment: %v", err)
	}
	content := "body\n![shared](" + saved.Reference + ")"
	updated, err := device.notes.Update(ctx, created.ID, note.UpdateInput{
		Content: &content, ExpectedRevision: &created.Revision,
	})
	if err != nil {
		t.Fatalf("add sync attachment reference: %v", err)
	}
	return updated, saved.Metadata, saved.Reference
}

func syncTestImageBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
}

var errInjectedSyncCopyStage = errors.New("injected sync-copy staging failure")

type failAfterSyncCopyStore struct {
	*attachmentstore.Store
	remaining int
}

func (s *failAfterSyncCopyStore) StageSyncAttachment(ctx context.Context, noteID string, operationID string, metadata attachmentstore.Metadata, data []byte) error {
	if s.remaining == 0 {
		return errInjectedSyncCopyStage
	}
	s.remaining--
	return s.Store.StageSyncAttachment(ctx, noteID, operationID, metadata, data)
}

var errInjectedAfterSyncCopyCommit = errors.New("injected sync-copy commit failure")

var errInjectedBeforeSyncCopyCommit = errors.New("injected sync-copy commit interruption")

type failAfterSyncCopyCommitStore struct {
	*attachmentstore.Store
	failed bool
}

func (s *failAfterSyncCopyCommitStore) CommitSyncAttachments(ctx context.Context, noteID string, operationID string) error {
	if err := s.Store.CommitSyncAttachments(ctx, noteID, operationID); err != nil {
		return err
	}
	if !s.failed {
		s.failed = true
		return errInjectedAfterSyncCopyCommit
	}
	return nil
}

type failBeforeSyncCopyCommitStore struct {
	*attachmentstore.Store
}

func (s *failBeforeSyncCopyCommitStore) CommitSyncAttachments(context.Context, string, string) error {
	return errInjectedBeforeSyncCopyCommit
}

type removeSyncCopyManifestStore struct {
	*attachmentstore.Store
	manifestPath string
}

func (s *removeSyncCopyManifestStore) StageSyncAttachment(ctx context.Context, noteID string, operationID string, metadata attachmentstore.Metadata, data []byte) error {
	if err := s.Store.StageSyncAttachment(ctx, noteID, operationID, metadata, data); err != nil {
		return err
	}
	return os.Remove(s.manifestPath)
}

func prepareKeepBothAttachmentConflict(t *testing.T, vaultID string) (*syncAttachmentTestDevice, *fakeRemote, Conflict, attachmentstore.Metadata) {
	t.Helper()
	ctx := context.Background()
	remote := newFakeRemote()
	device := newSyncAttachmentTestDevice(t, remote, vaultID)
	original, sourceAttachment, sourceReference := createSyncAttachmentTestNote(t, device)
	if result, err := device.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial sync for recovery test = %#v, err=%v", result, err)
	}
	current, err := device.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get local note for recovery test: %v", err)
	}
	localContent := "local recovery version\n" + sourceReference
	if _, err := device.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &localContent, ExpectedRevision: &current.Revision,
	}); err != nil {
		t.Fatalf("create local recovery conflict version: %v", err)
	}

	remoteDevice := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("import remote recovery conflict device = %#v, err=%v", result, err)
	}
	remoteCurrent, err := remoteDevice.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get remote recovery conflict note: %v", err)
	}
	remoteContent := "remote recovery version\n" + sourceReference
	if _, err := remoteDevice.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &remoteContent, ExpectedRevision: &remoteCurrent.Revision,
	}); err != nil {
		t.Fatalf("create remote recovery conflict version: %v", err)
	}
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced {
		t.Fatalf("sync remote recovery conflict version = %#v, err=%v", result, err)
	}
	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("sync local recovery conflict version = %#v, err=%v", result, err)
	}
	conflicts, err := device.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("recovery test conflicts = %#v, err=%v", conflicts, err)
	}
	return device, remote, conflicts[0], sourceAttachment
}

func readFakeRemoteManifest(t *testing.T, remote *fakeRemote) ManifestDocument {
	t.Helper()
	var head HeadDocument
	if err := json.Unmarshal(remote.files[headPath], &head); err != nil {
		t.Fatalf("decode fake remote head: %v", err)
	}
	var manifest ManifestDocument
	if err := json.Unmarshal(remote.files[manifestPath(head.ManifestHash)], &manifest); err != nil {
		t.Fatalf("decode fake remote manifest: %v", err)
	}
	return manifest
}

func assertFakeRemoteAttachmentParents(t *testing.T, remote *fakeRemote, manifest ManifestDocument) {
	t.Helper()
	entries := make(map[string]ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entries[entry.EntityKey] = entry
	}
	for _, entry := range manifest.Entries {
		if entry.EntityType != note.SyncEntityAttachment {
			continue
		}
		noteID, _, ok := attachmentIDsFromKey(entry.EntityKey)
		if !ok {
			t.Fatalf("invalid attachment key in fake remote manifest: %#v", entry)
		}
		parent, ok := entries[note.SyncEntityKey(note.SyncEntityNote, noteID)]
		if !ok {
			t.Fatalf("attachment has no note parent: %#v", entry)
		}
		parentObject, err := decodeObject(remote.files[objectPath(parent.ObjectHash)])
		if err != nil {
			t.Fatalf("decode attachment parent: %v", err)
		}
		if parentObject.Deleted {
			t.Fatalf("attachment parent is a tombstone: %#v", entry)
		}
		attachmentObject, err := decodeObject(remote.files[objectPath(entry.ObjectHash)])
		if err != nil {
			t.Fatalf("decode attachment object: %v", err)
		}
		var payload note.SyncAttachmentPayload
		if err := json.Unmarshal(attachmentObject.Payload, &payload); err != nil {
			t.Fatalf("decode attachment payload: %v", err)
		}
		if payload.NoteID != noteID {
			t.Fatalf("attachment payload parent = %q, manifest parent = %q", payload.NoteID, noteID)
		}
	}
}

func TestSyncAttachmentParentDeleteConflictPreservesAttachmentData(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("1", 32)
	deviceA := newSyncAttachmentTestDevice(t, remote, vaultID)
	noteA, originalAttachment, originalReference := createSyncAttachmentTestNote(t, deviceA)
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device A sync = %#v, err=%v", result, err)
	}

	deviceB := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device B import = %#v, err=%v", result, err)
	}
	if _, _, err := deviceB.attachments.Read(ctx, noteA.ID, originalAttachment.ID); err != nil {
		t.Fatalf("read imported attachment on device B: %v", err)
	}

	if err := deviceA.notes.Delete(ctx, noteA.ID, note.DeleteInput{ExpectedRevision: noteA.Revision}); err != nil {
		t.Fatalf("delete note on device A: %v", err)
	}
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced {
		t.Fatalf("delete sync on device A = %#v, err=%v", result, err)
	}
	deletedManifest := readFakeRemoteManifest(t, remote)
	var deletedNote ManifestEntry
	for _, entry := range deletedManifest.Entries {
		if entry.EntityKey == note.SyncEntityKey(note.SyncEntityNote, noteA.ID) {
			deletedNote = entry
		}
		if entry.EntityType == note.SyncEntityAttachment {
			t.Fatalf("device A delete left an attachment in the remote manifest: %#v", entry)
		}
	}
	if deletedNote.EntityKey == "" {
		t.Fatalf("device A delete did not publish the note tombstone")
	}
	deletedObject, err := decodeObject(remote.files[objectPath(deletedNote.ObjectHash)])
	if err != nil || !deletedObject.Deleted {
		t.Fatalf("remote note after device A delete = %#v, err=%v", deletedObject, err)
	}

	newImage, err := syncTestImageBytes()
	if err != nil {
		t.Fatalf("decode new sync test image: %v", err)
	}
	newAttachment, err := deviceB.attachments.Save(ctx, attachmentstore.SaveInput{
		NoteID: noteA.ID, Kind: "image", MIMEType: "image/png", Name: "new.png", Data: newImage,
	})
	if err != nil {
		t.Fatalf("save new device B attachment: %v", err)
	}
	currentB, err := deviceB.notes.Get(ctx, noteA.ID)
	if err != nil {
		t.Fatalf("get stale device B note: %v", err)
	}
	contentB := currentB.Content + "\n![new](" + newAttachment.Reference + ")"
	if _, err := deviceB.notes.Update(ctx, noteA.ID, note.UpdateInput{
		Content: &contentB, ExpectedRevision: &currentB.Revision,
	}); err != nil {
		t.Fatalf("update stale device B note: %v", err)
	}
	result, err := deviceB.service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("stale device B sync = %#v, err=%v", result, err)
	}
	if _, oldBytes, err := deviceB.attachments.Read(ctx, noteA.ID, originalAttachment.ID); err != nil || len(oldBytes) == 0 {
		t.Fatalf("old attachment was lost during conflict: err=%v", err)
	}
	if _, newBytes, err := deviceB.attachments.Read(ctx, noteA.ID, newAttachment.Metadata.ID); err != nil || !bytes.Equal(newBytes, newImage) {
		t.Fatalf("new attachment was lost during conflict: err=%v", err)
	}
	conflicts, err := deviceB.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 || conflicts[0].EntityKey != note.SyncEntityKey(note.SyncEntityNote, noteA.ID) {
		t.Fatalf("device B conflicts = %#v, err=%v", conflicts, err)
	}
	pending, err := deviceB.repository.ListOutbox(ctx, 1000)
	if err != nil {
		t.Fatalf("list pending device B outbox: %v", err)
	}
	for _, attachment := range []attachmentstore.Metadata{originalAttachment, newAttachment.Metadata} {
		found := false
		for _, item := range pending {
			if item.EntityKey == note.SyncAttachmentEntityKey(noteA.ID, attachment.ID) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("attachment %s was not kept pending after parent conflict", attachment.ID)
		}
	}
	manifestAfterConflict := readFakeRemoteManifest(t, remote)
	for _, entry := range manifestAfterConflict.Entries {
		if entry.EntityType == note.SyncEntityAttachment {
			t.Fatalf("conflicting device published an attachment under the remote tombstone: %#v", entry)
		}
	}

	if err := deviceB.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflicts[0].ID, Choice: "local"}); err != nil {
		t.Fatalf("resolve note conflict with local version: %v", err)
	}
	result, err = deviceB.service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusSynced || result.Remaining != 0 {
		t.Fatalf("resolved device B sync = %#v, err=%v", result, err)
	}
	manifestAfterResolution := readFakeRemoteManifest(t, remote)
	assertFakeRemoteAttachmentParents(t, remote, manifestAfterResolution)

	deviceC := newSyncAttachmentTestDevice(t, remote, vaultID)
	result, err = deviceC.service.SyncNow(ctx, SyncNowInput{ImportRemote: true})
	if err != nil || result.Status != StatusSynced || result.Downloaded < 3 {
		t.Fatalf("new device import after conflict = %#v, err=%v", result, err)
	}
	importedNote, err := deviceC.notes.Get(ctx, noteA.ID)
	if err != nil || importedNote.Content != contentB {
		t.Fatalf("new device note after conflict = %#v, err=%v", importedNote, err)
	}
	if _, importedOldBytes, err := deviceC.attachments.Read(ctx, noteA.ID, originalAttachment.ID); err != nil || len(importedOldBytes) == 0 {
		t.Fatalf("new device could not redownload original attachment: %v", err)
	}
	if _, importedNewBytes, err := deviceC.attachments.Read(ctx, noteA.ID, newAttachment.Metadata.ID); err != nil || !bytes.Equal(importedNewBytes, newImage) {
		t.Fatalf("new device could not redownload new attachment: %v", err)
	}
	if !strings.Contains(importedNote.Content, originalReference) {
		t.Fatalf("resolved note lost its original attachment reference: %q", importedNote.Content)
	}
}

func TestSyncAttachmentParentDeleteWithoutLocalChangesFollowsRemote(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("4", 32)
	deviceA := newSyncAttachmentTestDevice(t, remote, vaultID)
	noteA, originalAttachment, _ := createSyncAttachmentTestNote(t, deviceA)
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device A sync = %#v, err=%v", result, err)
	}

	deviceB := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device B import = %#v, err=%v", result, err)
	}
	if err := deviceA.notes.Delete(ctx, noteA.ID, note.DeleteInput{ExpectedRevision: noteA.Revision}); err != nil {
		t.Fatalf("delete note on device A: %v", err)
	}
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced {
		t.Fatalf("sync note deletion on device A = %#v, err=%v", result, err)
	}

	deletedManifest := readFakeRemoteManifest(t, remote)
	var deletedNote ManifestEntry
	for _, entry := range deletedManifest.Entries {
		if entry.EntityKey == note.SyncEntityKey(note.SyncEntityNote, noteA.ID) {
			deletedNote = entry
		}
		if entry.EntityType == note.SyncEntityAttachment {
			t.Fatalf("remote note deletion left an attachment entry: %#v", entry)
		}
	}
	if deletedNote.EntityKey == "" {
		t.Fatalf("remote note deletion did not publish a tombstone")
	}

	result, err := deviceB.service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusSynced || result.Conflicts != 0 {
		t.Fatalf("unchanged attachment parent deletion sync = %#v, err=%v", result, err)
	}
	if _, err := deviceB.notes.Get(ctx, noteA.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("unchanged parent note after remote deletion = %v, want ErrNotFound", err)
	}
	if attachments, err := deviceB.attachments.List(ctx, noteA.ID); err != nil || len(attachments) != 0 {
		t.Fatalf("unchanged parent attachments after remote deletion = %#v, err=%v", attachments, err)
	}
	if _, _, err := deviceB.attachments.Read(ctx, noteA.ID, originalAttachment.ID); !errors.Is(err, attachmentstore.ErrAttachmentNotFound) {
		t.Fatalf("unchanged attachment after remote deletion = %v, want ErrAttachmentNotFound", err)
	}
	conflicts, err := deviceB.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("unchanged attachment parent deletion conflicts = %#v, err=%v", conflicts, err)
	}
}

func TestSyncAttachmentOnlyChangeDoesNotDeleteLocalParentData(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("3", 32)
	deviceA := newSyncAttachmentTestDevice(t, remote, vaultID)
	noteA, originalAttachment, _ := createSyncAttachmentTestNote(t, deviceA)
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device A sync = %#v, err=%v", result, err)
	}

	deviceB := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device B import = %#v, err=%v", result, err)
	}
	if err := deviceA.notes.Delete(ctx, noteA.ID, note.DeleteInput{ExpectedRevision: noteA.Revision}); err != nil {
		t.Fatalf("delete note on device A: %v", err)
	}
	if _, err := deviceA.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync note deletion on device A: %v", err)
	}

	imageBytes, err := syncTestImageBytes()
	if err != nil {
		t.Fatalf("decode attachment-only test image: %v", err)
	}
	newAttachment, err := deviceB.attachments.Save(ctx, attachmentstore.SaveInput{
		NoteID: noteA.ID, Kind: "image", MIMEType: "image/png", Name: "attachment-only.png", Data: imageBytes,
	})
	if err != nil {
		t.Fatalf("save attachment-only change: %v", err)
	}
	result, err := deviceB.service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("attachment-only conflict sync = %#v, err=%v", result, err)
	}
	if _, err := deviceB.notes.Get(ctx, noteA.ID); err != nil {
		t.Fatalf("parent note was deleted instead of conflicted: %v", err)
	}
	if _, oldBytes, err := deviceB.attachments.Read(ctx, noteA.ID, originalAttachment.ID); err != nil || len(oldBytes) == 0 {
		t.Fatalf("original attachment was lost in attachment-only conflict: %v", err)
	}
	if _, newBytes, err := deviceB.attachments.Read(ctx, noteA.ID, newAttachment.Metadata.ID); err != nil || !bytes.Equal(newBytes, imageBytes) {
		t.Fatalf("new attachment was lost in attachment-only conflict: %v", err)
	}
	conflicts, err := deviceB.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 || conflicts[0].EntityKey != note.SyncEntityKey(note.SyncEntityNote, noteA.ID) {
		t.Fatalf("attachment-only conflicts = %#v, err=%v", conflicts, err)
	}
	stateBeforeRetry, err := deviceB.repository.GetItemState(ctx, note.SyncEntityKey(note.SyncEntityNote, noteA.ID))
	if err != nil || stateBeforeRetry == nil {
		t.Fatalf("parent state before retry = %#v, err=%v", stateBeforeRetry, err)
	}
	if _, err := deviceB.repository.db.ExecContext(ctx, "UPDATE sync_outbox SET next_retry_at = ?", formatTimestamp(time.Now().UTC().Add(-time.Hour))); err != nil {
		t.Fatalf("expire conflict outbox retry timestamps: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		retryResult, retryErr := deviceB.service.SyncNow(ctx, SyncNowInput{})
		if retryErr != nil || retryResult.Status != StatusConflict || retryResult.Conflicts == 0 {
			t.Fatalf("repeated attachment-only conflict sync %d = %#v, err=%v", attempt+1, retryResult, retryErr)
		}
	}
	stateAfterRetry, err := deviceB.repository.GetItemState(ctx, note.SyncEntityKey(note.SyncEntityNote, noteA.ID))
	if err != nil || stateAfterRetry == nil || stateAfterRetry.BaseObjectHash != stateBeforeRetry.BaseObjectHash || stateAfterRetry.BaseObjectHash == conflicts[0].RemoteObjectHash {
		t.Fatalf("parent state was rebased while conflict remained open: before=%#v after=%#v conflict=%#v err=%v", stateBeforeRetry, stateAfterRetry, conflicts[0], err)
	}
	if _, err := deviceB.notes.Get(ctx, noteA.ID); err != nil {
		t.Fatalf("parent note was deleted after repeated conflict retry: %v", err)
	}
	if err := deviceB.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflicts[0].ID, Choice: "both"}); !errors.Is(err, ErrInvalidRemoteFormat) {
		t.Fatalf("keep both on deleted-parent conflict = %v, want invalid remote format", err)
	}
	if _, err := deviceB.notes.Get(ctx, noteA.ID); err != nil {
		t.Fatalf("failed keep-both choice changed the parent note: %v", err)
	}
	manifestAfterConflict := readFakeRemoteManifest(t, remote)
	for _, entry := range manifestAfterConflict.Entries {
		if entry.EntityType == note.SyncEntityAttachment {
			t.Fatalf("attachment-only change was published under a deleted parent: %#v", entry)
		}
	}
	if err := deviceB.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflicts[0].ID, Choice: "local"}); err != nil {
		t.Fatalf("resolve attachment-only parent conflict locally: %v", err)
	}
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced || result.Remaining != 0 {
		t.Fatalf("sync attachment-only local resolution = %#v, err=%v", result, err)
	}
	assertFakeRemoteAttachmentParents(t, remote, readFakeRemoteManifest(t, remote))
}

func TestSyncAttachmentParentDeleteConflictRemoteResolutionRemovesLocalData(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("6", 32)
	deviceA := newSyncAttachmentTestDevice(t, remote, vaultID)
	noteA, _, _ := createSyncAttachmentTestNote(t, deviceA)
	if result, err := deviceA.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device A sync = %#v, err=%v", result, err)
	}

	deviceB := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial device B import = %#v, err=%v", result, err)
	}
	if err := deviceA.notes.Delete(ctx, noteA.ID, note.DeleteInput{ExpectedRevision: noteA.Revision}); err != nil {
		t.Fatalf("delete note on device A: %v", err)
	}
	if _, err := deviceA.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync note deletion on device A: %v", err)
	}

	imageBytes, err := syncTestImageBytes()
	if err != nil {
		t.Fatalf("decode remote-resolution test image: %v", err)
	}
	newAttachment, err := deviceB.attachments.Save(ctx, attachmentstore.SaveInput{
		NoteID: noteA.ID, Kind: "image", MIMEType: "image/png", Name: "remote-resolution.png", Data: imageBytes,
	})
	if err != nil {
		t.Fatalf("save remote-resolution attachment: %v", err)
	}
	result, err := deviceB.service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("remote-resolution conflict sync = %#v, err=%v", result, err)
	}
	conflicts, err := deviceB.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("remote-resolution conflicts = %#v, err=%v", conflicts, err)
	}
	if err := deviceB.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflicts[0].ID, Choice: "remote"}); err != nil {
		t.Fatalf("resolve parent conflict with remote version: %v", err)
	}
	if _, err := deviceB.notes.Get(ctx, noteA.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("parent note after remote conflict resolution = %v, want ErrNotFound", err)
	}
	if attachments, err := deviceB.attachments.List(ctx, noteA.ID); err != nil || len(attachments) != 0 {
		t.Fatalf("attachments after remote conflict resolution = %#v, err=%v", attachments, err)
	}
	if _, _, err := deviceB.attachments.Read(ctx, noteA.ID, newAttachment.Metadata.ID); !errors.Is(err, attachmentstore.ErrAttachmentNotFound) {
		t.Fatalf("new attachment after remote conflict resolution = %v, want ErrAttachmentNotFound", err)
	}
	if resolved, err := deviceB.repository.ListConflicts(ctx); err != nil || len(resolved) != 0 {
		t.Fatalf("remote-resolution conflicts after choice = %#v, err=%v", resolved, err)
	}
	if result, err := deviceB.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced || result.Remaining != 0 {
		t.Fatalf("sync after remote conflict resolution = %#v, err=%v", result, err)
	}
	assertFakeRemoteAttachmentParents(t, remote, readFakeRemoteManifest(t, remote))
}

func TestKeepBothNoteVersionsCopiesAndRewritesAttachments(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("2", 32)
	device := newSyncAttachmentTestDevice(t, remote, vaultID)
	original, sourceAttachment, sourceReference := createSyncAttachmentTestNote(t, device)
	if result, err := device.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial sync = %#v, err=%v", result, err)
	}

	current, err := device.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get local note before conflict: %v", err)
	}
	localContent := "local version\n" + sourceReference
	if _, err := device.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &localContent, ExpectedRevision: &current.Revision,
	}); err != nil {
		t.Fatalf("create local conflict version: %v", err)
	}

	otherNoteID := strings.Repeat("9", 32)
	otherAttachmentID := strings.Repeat("8", 32)
	externalReference := attachmentstore.Reference(otherNoteID, otherAttachmentID)
	externalURL := "https://example.test/proxy?target=" + sourceReference
	remoteContent := "remote version\n" + sourceReference + "\n" + externalReference + "\n" + externalURL
	remoteDevice := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("import remote conflict device = %#v, err=%v", result, err)
	}
	remoteCurrent, err := remoteDevice.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get remote conflict note: %v", err)
	}
	if _, err := remoteDevice.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &remoteContent, ExpectedRevision: &remoteCurrent.Revision,
	}); err != nil {
		t.Fatalf("create remote conflict version: %v", err)
	}
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced {
		t.Fatalf("sync remote conflict version = %#v, err=%v", result, err)
	}
	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("sync local conflict version = %#v, err=%v", result, err)
	}
	conflicts, err := device.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 || conflicts[0].EntityKey != note.SyncEntityKey(note.SyncEntityNote, original.ID) {
		t.Fatalf("attachment conflict = %#v, err=%v", conflicts, err)
	}
	conflict := conflicts[0]
	if err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"}); err != nil {
		t.Fatalf("keep both conflict versions: %v", err)
	}

	copyNoteID := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]
	copyAttachmentID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + sourceAttachment.ID))[:32]
	expectedReference := attachmentstore.Reference(copyNoteID, copyAttachmentID)
	copiedNote, err := device.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("get copied note: %v", err)
	}
	if !strings.Contains(copiedNote.Content, "remote version\n"+expectedReference+"\n") {
		t.Fatalf("copied note reference rewrite = %q", copiedNote.Content)
	}
	if !strings.Contains(copiedNote.Content, externalReference) || !strings.Contains(copiedNote.Content, externalURL) {
		t.Fatalf("copied note changed unrelated references = %q", copiedNote.Content)
	}
	copiedAttachments, err := device.attachments.List(ctx, copyNoteID)
	if err != nil || len(copiedAttachments) != 1 || copiedAttachments[0].ID != copyAttachmentID {
		t.Fatalf("copied attachment manifest = %#v, err=%v", copiedAttachments, err)
	}
	_, sourceBytes, err := device.attachments.Read(ctx, original.ID, sourceAttachment.ID)
	if err != nil {
		t.Fatalf("read source attachment: %v", err)
	}
	_, copiedBytes, err := device.attachments.Read(ctx, copyNoteID, copyAttachmentID)
	if err != nil || !bytes.Equal(copiedBytes, sourceBytes) {
		t.Fatalf("copied attachment body mismatch: err=%v", err)
	}

	if err := device.service.keepBothNoteVersions(ctx, conflict); err != nil {
		t.Fatalf("retry keep both conflict versions: %v", err)
	}
	copiedAttachmentsAfterRetry, err := device.attachments.List(ctx, copyNoteID)
	if err != nil || len(copiedAttachmentsAfterRetry) != 1 {
		t.Fatalf("retry duplicated copied attachments = %#v, err=%v", copiedAttachmentsAfterRetry, err)
	}
	pending, err := device.repository.ListOutbox(ctx, 1000)
	if err != nil {
		t.Fatalf("list copied attachment outbox after retry: %v", err)
	}
	copyAttachmentOutboxCount := 0
	for _, item := range pending {
		if item.EntityKey == note.SyncAttachmentEntityKey(copyNoteID, copyAttachmentID) {
			copyAttachmentOutboxCount++
		}
	}
	if copyAttachmentOutboxCount != 1 {
		t.Fatalf("copied attachment outbox count after retry = %d, want 1", copyAttachmentOutboxCount)
	}

	archiveBytes, err := device.attachments.ExportZip(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("export copied note ZIP: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("read copied note ZIP: %v", err)
	}
	manifestFound, bodyFound := false, false
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open copied ZIP entry %q: %v", file.Name, err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read copied ZIP entry %q: read=%v close=%v", file.Name, readErr, closeErr)
		}
		if file.Name == "manifest.json" {
			manifestFound = bytes.Contains(data, []byte(copyAttachmentID))
		}
		if bytes.Equal(data, sourceBytes) {
			bodyFound = true
		}
	}
	if !manifestFound || !bodyFound {
		t.Fatalf("copied ZIP did not contain independent manifest/body: manifest=%v body=%v", manifestFound, bodyFound)
	}

	remainingSource, err := device.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get source note before delete: %v", err)
	}
	if err := device.notes.Delete(ctx, original.ID, note.DeleteInput{ExpectedRevision: remainingSource.Revision}); err != nil {
		t.Fatalf("delete source note after keep both: %v", err)
	}
	if _, err := device.notes.Get(ctx, copyNoteID); err != nil {
		t.Fatalf("copied note was removed with source note: %v", err)
	}
	if _, copiedBytesAfterDelete, err := device.attachments.Read(ctx, copyNoteID, copyAttachmentID); err != nil || !bytes.Equal(copiedBytesAfterDelete, sourceBytes) {
		t.Fatalf("copied attachment after source deletion: err=%v", err)
	}
	if archiveBytes, err := device.attachments.ExportZip(ctx, copyNoteID); err != nil || len(archiveBytes) == 0 {
		t.Fatalf("copied ZIP after source deletion: size=%d err=%v", len(archiveBytes), err)
	}
}

func TestKeepBothNoteVersionsStagesAttachmentsAcrossPartialFailure(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("5", 32)
	device := newSyncAttachmentTestDevice(t, remote, vaultID)
	original, firstAttachment, firstReference := createSyncAttachmentTestNote(t, device)
	imageBytes, err := syncTestImageBytes()
	if err != nil {
		t.Fatalf("decode second sync test image: %v", err)
	}
	secondAttachment, err := device.attachments.Save(ctx, attachmentstore.SaveInput{
		NoteID: original.ID, Kind: "image", MIMEType: "image/png", Name: "second.png", Data: imageBytes,
	})
	if err != nil {
		t.Fatalf("save second sync test attachment: %v", err)
	}
	withSecondReference := original.Content + "\n![second](" + secondAttachment.Reference + ")"
	original, err = device.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &withSecondReference, ExpectedRevision: &original.Revision,
	})
	if err != nil {
		t.Fatalf("add second sync attachment reference: %v", err)
	}
	if result, err := device.service.SyncNow(ctx, SyncNowInput{InitializeRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("initial sync = %#v, err=%v", result, err)
	}

	current, err := device.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get local note before partial keep-both: %v", err)
	}
	localContent := "local version\n" + firstReference + "\n" + secondAttachment.Reference
	if _, err := device.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &localContent, ExpectedRevision: &current.Revision,
	}); err != nil {
		t.Fatalf("create local partial keep-both conflict version: %v", err)
	}

	remoteDevice := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("import remote partial keep-both device = %#v, err=%v", result, err)
	}
	remoteCurrent, err := remoteDevice.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get remote partial keep-both note: %v", err)
	}
	remoteContent := "remote version\n" + firstReference + "\n" + secondAttachment.Reference
	if _, err := remoteDevice.notes.Update(ctx, original.ID, note.UpdateInput{
		Content: &remoteContent, ExpectedRevision: &remoteCurrent.Revision,
	}); err != nil {
		t.Fatalf("create remote partial keep-both conflict version: %v", err)
	}
	if result, err := remoteDevice.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusSynced {
		t.Fatalf("sync remote partial keep-both version = %#v, err=%v", result, err)
	}
	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil || result.Status != StatusConflict || result.Conflicts == 0 {
		t.Fatalf("sync local partial keep-both conflict = %#v, err=%v", result, err)
	}
	conflicts, err := device.repository.ListConflicts(ctx)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("partial keep-both conflicts = %#v, err=%v", conflicts, err)
	}
	conflict := conflicts[0]
	copyNoteID := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]

	device.service.SetAttachmentStore(&failAfterSyncCopyStore{Store: device.attachments, remaining: 1})
	err = device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"})
	if !errors.Is(err, errInjectedSyncCopyStage) {
		t.Fatalf("partial keep-both staging error = %v, want injected error", err)
	}
	if _, err := device.notes.Get(ctx, copyNoteID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("partially copied note = %v, want ErrNotFound", err)
	}
	if copied, err := device.attachments.List(ctx, copyNoteID); err != nil || len(copied) != 0 {
		t.Fatalf("partially copied active attachments = %#v, err=%v", copied, err)
	}
	active, err := device.attachments.ListAll(ctx)
	if err != nil || len(active) != 2 {
		t.Fatalf("active attachments exposed partial staging = %#v, err=%v", active, err)
	}
	if _, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync with unresolved partial keep-both conflict: %v", err)
	}

	restartedAttachments, err := attachmentstore.NewStore(device.notesDir, nil)
	if err != nil {
		t.Fatalf("reopen attachment store after partial keep-both: %v", err)
	}
	if err := restartedAttachments.Recover(ctx); err != nil {
		t.Fatalf("recover partial keep-both staging: %v", err)
	}
	device.attachments = restartedAttachments
	device.notes.SetAttachmentStore(restartedAttachments)
	device.service.SetAttachmentStore(restartedAttachments)
	if err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"}); err != nil {
		t.Fatalf("retry keep-both after partial staging: %v", err)
	}

	firstCopyID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + firstAttachment.ID))[:32]
	secondCopyID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + secondAttachment.Metadata.ID))[:32]
	copiedNote, err := device.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("get copied note after partial retry: %v", err)
	}
	if !strings.Contains(copiedNote.Content, attachmentstore.Reference(copyNoteID, firstCopyID)) ||
		!strings.Contains(copiedNote.Content, attachmentstore.Reference(copyNoteID, secondCopyID)) {
		t.Fatalf("copied references after partial retry = %q", copiedNote.Content)
	}
	copiedAttachments, err := device.attachments.List(ctx, copyNoteID)
	if err != nil || len(copiedAttachments) != 2 {
		t.Fatalf("copied attachments after partial retry = %#v, err=%v", copiedAttachments, err)
	}
	active, err = device.attachments.ListAll(ctx)
	if err != nil || len(active) != 4 {
		t.Fatalf("active attachments after partial retry = %#v, err=%v", active, err)
	}

	_, firstSourceBytes, err := device.attachments.Read(ctx, original.ID, firstAttachment.ID)
	if err != nil {
		t.Fatalf("read first source attachment after partial retry: %v", err)
	}
	_, secondSourceBytes, err := device.attachments.Read(ctx, original.ID, secondAttachment.Metadata.ID)
	if err != nil {
		t.Fatalf("read second source attachment after partial retry: %v", err)
	}
	sourceBeforeDelete, err := device.notes.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get source note revision before partial retry source deletion: %v", err)
	}
	if err := device.notes.Delete(ctx, original.ID, note.DeleteInput{ExpectedRevision: sourceBeforeDelete.Revision}); err != nil {
		t.Fatalf("delete source note after partial retry: %v", err)
	}
	if _, copiedBytes, err := device.attachments.Read(ctx, copyNoteID, firstCopyID); err != nil || !bytes.Equal(copiedBytes, firstSourceBytes) {
		t.Fatalf("first copied attachment after source deletion: err=%v", err)
	}
	if _, copiedBytes, err := device.attachments.Read(ctx, copyNoteID, secondCopyID); err != nil || !bytes.Equal(copiedBytes, secondSourceBytes) {
		t.Fatalf("second copied attachment after source deletion: err=%v", err)
	}
}

func TestKeepBothNoteVersionsRecoversCopyOutboxAfterRestart(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                    string
		setup                   func(t *testing.T, device *syncAttachmentTestDevice, conflict Conflict)
		wantCommitFailure       error
		wantOutboxFailure       bool
		wantActiveAttachmentNum int
	}{
		{
			name: "formal attachment commit completes first", wantCommitFailure: errInjectedAfterSyncCopyCommit,
			wantActiveAttachmentNum: 1,
			setup: func(_ *testing.T, device *syncAttachmentTestDevice, _ Conflict) {
				device.service.SetAttachmentStore(&failAfterSyncCopyCommitStore{Store: device.attachments})
			},
		},
		{
			name: "attachment commit is interrupted", wantCommitFailure: errInjectedBeforeSyncCopyCommit,
			wantActiveAttachmentNum: 0,
			setup: func(_ *testing.T, device *syncAttachmentTestDevice, _ Conflict) {
				device.service.SetAttachmentStore(&failBeforeSyncCopyCommitStore{Store: device.attachments})
			},
		},
		{
			name: "outbox enqueue fails", wantOutboxFailure: true, wantActiveAttachmentNum: 1,
			setup: func(t *testing.T, device *syncAttachmentTestDevice, conflict Conflict) {
				triggerSQL := "CREATE TRIGGER fail_keep_both_copy_outbox " +
					"BEFORE INSERT ON sync_outbox " +
					"WHEN NEW.change_set_id = '" + conflict.ID + "-remote' " +
					"BEGIN SELECT RAISE(ABORT, 'injected keep-both outbox failure'); END"
				if _, err := device.repository.db.ExecContext(ctx, triggerSQL); err != nil {
					t.Fatalf("create outbox failure trigger: %v", err)
				}
				t.Cleanup(func() {
					_, _ = device.repository.db.ExecContext(ctx, "DROP TRIGGER IF EXISTS fail_keep_both_copy_outbox")
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vaultID := strings.Repeat("6", 32)
			device, remote, conflict, sourceAttachment := prepareKeepBothAttachmentConflict(t, vaultID)
			copyNoteID := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]
			copyAttachmentID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + sourceAttachment.ID))[:32]
			test.setup(t, device, conflict)

			err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"})
			if test.wantCommitFailure != nil {
				if !errors.Is(err, test.wantCommitFailure) {
					t.Fatalf("formal commit failure = %v, want %v", err, test.wantCommitFailure)
				}
			} else if test.wantOutboxFailure && (err == nil || !strings.Contains(err.Error(), "injected keep-both outbox failure")) {
				t.Fatalf("outbox enqueue failure = %v, want injected failure", err)
			}

			if _, err := device.notes.Get(ctx, copyNoteID); err != nil {
				t.Fatalf("copied note after injected failure: %v", err)
			}
			if copied, err := device.attachments.List(ctx, copyNoteID); err != nil || len(copied) != test.wantActiveAttachmentNum || (test.wantActiveAttachmentNum == 1 && copied[0].ID != copyAttachmentID) {
				t.Fatalf("copied attachments after injected failure = %#v, err=%v", copied, err)
			}

			if test.wantOutboxFailure {
				if _, err := device.repository.db.ExecContext(ctx, "DROP TRIGGER IF EXISTS fail_keep_both_copy_outbox"); err != nil {
					t.Fatalf("drop outbox failure trigger: %v", err)
				}
			}
			restartSyncAttachmentTestDevice(t, device, remote)

			if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
				t.Fatalf("restart sync recovery = %#v, err=%v", result, err)
			}
			manifest := readFakeRemoteManifest(t, remote)
			entries := make(map[string]ManifestEntry, len(manifest.Entries))
			for _, entry := range manifest.Entries {
				entries[entry.EntityKey] = entry
			}
			copyNoteKey := note.SyncEntityKey(note.SyncEntityNote, copyNoteID)
			copyAttachmentKey := note.SyncAttachmentEntityKey(copyNoteID, copyAttachmentID)
			if _, ok := entries[copyNoteKey]; !ok {
				t.Fatalf("recovered copy note is missing from remote manifest: %#v", manifest.Entries)
			}
			if _, ok := entries[copyAttachmentKey]; !ok {
				t.Fatalf("recovered copy attachment is missing from remote manifest: %#v", manifest.Entries)
			}
			assertFakeRemoteAttachmentParents(t, remote, manifest)
			if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
				t.Fatalf("repeat restart sync recovery = %#v, err=%v", result, err)
			}
			repeatedManifest := readFakeRemoteManifest(t, remote)
			copyEntryCount := 0
			for _, entry := range repeatedManifest.Entries {
				if entry.EntityKey == copyNoteKey || entry.EntityKey == copyAttachmentKey {
					copyEntryCount++
				}
			}
			if copyEntryCount != 2 {
				t.Fatalf("repeat restart sync changed copied entity count = %d, manifest=%#v", copyEntryCount, repeatedManifest.Entries)
			}

			redownloaded := newSyncAttachmentTestDevice(t, remote, vaultID)
			if result, err := redownloaded.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
				t.Fatalf("redownload recovered copy = %#v, err=%v", result, err)
			}
			importedCopy, err := redownloaded.notes.Get(ctx, copyNoteID)
			if err != nil {
				t.Fatalf("read redownloaded copied note: %v", err)
			}
			if !strings.Contains(importedCopy.Content, attachmentstore.Reference(copyNoteID, copyAttachmentID)) {
				t.Fatalf("redownloaded copied note reference = %q", importedCopy.Content)
			}
			if copied, err := redownloaded.attachments.List(ctx, copyNoteID); err != nil || len(copied) != 1 || copied[0].ID != copyAttachmentID {
				t.Fatalf("redownloaded copied attachments = %#v, err=%v", copied, err)
			}
			_, sourceBytes, err := redownloaded.attachments.Read(ctx, sourceAttachment.NoteID, sourceAttachment.ID)
			if err != nil {
				t.Fatalf("read redownloaded source attachment: %v", err)
			}
			_, copiedBytes, err := redownloaded.attachments.Read(ctx, copyNoteID, copyAttachmentID)
			if err != nil || !bytes.Equal(sourceBytes, copiedBytes) {
				t.Fatalf("redownloaded source/copy attachment mismatch: err=%v", err)
			}
			if _, err := redownloaded.notes.Get(ctx, sourceAttachment.NoteID); err != nil {
				// The original note ID is derived from the conflict key and must
				// remain available after recovering the independent copy.
				t.Fatalf("redownloaded source note was lost: %v", err)
			}
		})
	}
}

func TestKeepBothNoteVersionsPreservesEditBeforeSyncAfterRestart(t *testing.T) {
	ctx := context.Background()
	vaultID := strings.Repeat("7", 32)
	device, remote, conflict, sourceAttachment := prepareKeepBothAttachmentConflict(t, vaultID)
	copyNoteID := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]
	copyAttachmentID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + sourceAttachment.ID))[:32]

	device.service.SetAttachmentStore(&failBeforeSyncCopyCommitStore{Store: device.attachments})
	if err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"}); !errors.Is(err, errInjectedBeforeSyncCopyCommit) {
		t.Fatalf("interrupted attachment commit = %v, want %v", err, errInjectedBeforeSyncCopyCommit)
	}
	restartSyncAttachmentTestDevice(t, device, remote)

	copyNote, err := device.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("get copied note after restart: %v", err)
	}
	editedContent := copyNote.Content + "\npost-restart edit"
	copyNote, err = device.notes.Update(ctx, copyNoteID, note.UpdateInput{
		Content: &editedContent, ExpectedRevision: &copyNote.Revision,
	})
	if err != nil {
		t.Fatalf("edit copied note before first post-restart sync: %v", err)
	}
	if copyNote.Content != editedContent {
		t.Fatalf("edited copied note content = %q, want %q", copyNote.Content, editedContent)
	}

	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync edited copied note after restart = %#v, err=%v", result, err)
	}
	manifest := readFakeRemoteManifest(t, remote)
	entries := make(map[string]ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entries[entry.EntityKey] = entry
	}
	copyNoteEntry, ok := entries[note.SyncEntityKey(note.SyncEntityNote, copyNoteID)]
	if !ok {
		t.Fatalf("edited copied note is missing from remote manifest: %#v", manifest.Entries)
	}
	if _, ok := entries[note.SyncAttachmentEntityKey(copyNoteID, copyAttachmentID)]; !ok {
		t.Fatalf("recovered copied attachment is missing from remote manifest: %#v", manifest.Entries)
	}
	copyObject, err := decodeObject(remote.files[objectPath(copyNoteEntry.ObjectHash)])
	if err != nil {
		t.Fatalf("decode edited copied note object: %v", err)
	}
	var copyPayload note.SyncNotePayload
	if err := json.Unmarshal(copyObject.Payload, &copyPayload); err != nil {
		t.Fatalf("decode edited copied note payload: %v", err)
	}
	if copyPayload.Content != editedContent {
		t.Fatalf("remote copied note content = %q, want %q", copyPayload.Content, editedContent)
	}

	restartSyncAttachmentTestDevice(t, device, remote)
	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("repeat sync after restart = %#v, err=%v", result, err)
	}

	sourceMetadata, sourceBytes, err := device.attachments.Read(ctx, sourceAttachment.NoteID, sourceAttachment.ID)
	if err != nil {
		t.Fatalf("read source attachment after recovery: %v", err)
	}
	if sourceMetadata.ID != sourceAttachment.ID {
		t.Fatalf("source attachment metadata changed after recovery: %#v", sourceMetadata)
	}
	redownloaded := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := redownloaded.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("redownload edited copied note = %#v, err=%v", result, err)
	}
	importedCopy, err := redownloaded.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("read redownloaded edited copied note: %v", err)
	}
	if importedCopy.Content != editedContent {
		t.Fatalf("redownloaded copied note content = %q, want %q", importedCopy.Content, editedContent)
	}
	if _, copiedBytes, err := redownloaded.attachments.Read(ctx, copyNoteID, copyAttachmentID); err != nil || !bytes.Equal(copiedBytes, sourceBytes) {
		t.Fatalf("redownloaded copied attachment after edited recovery: err=%v", err)
	}
	if _, err := redownloaded.notes.Get(ctx, sourceAttachment.NoteID); err != nil {
		t.Fatalf("redownloaded source note was lost: %v", err)
	}
}

func TestKeepBothNoteVersionsBlocksEditedCopyUntilIncompleteStageRecovers(t *testing.T) {
	ctx := context.Background()
	vaultID := strings.Repeat("8", 32)
	device, remote, conflict, sourceAttachment := prepareKeepBothAttachmentConflict(t, vaultID)
	copyNoteID := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]
	copyAttachmentID := hashBytes([]byte("conflict-remote-copy-attachment:" + conflict.ID + "-remote:" + sourceAttachment.ID))[:32]
	copyOperationID := conflict.ID + "-remote"
	stageManifestPath := filepath.Join(device.notesDir, "attachments", copyNoteID+"."+copyOperationID+".sync-copy", "manifest.json")
	device.service.SetAttachmentStore(&removeSyncCopyManifestStore{
		Store: device.attachments, manifestPath: stageManifestPath,
	})
	if err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"}); !errors.Is(err, attachmentstore.ErrManifestInvalid) {
		t.Fatalf("incomplete attachment stage = %v, want %v", err, attachmentstore.ErrManifestInvalid)
	}
	if _, err := device.notes.Get(ctx, copyNoteID); err != nil {
		t.Fatalf("copied note after incomplete attachment stage: %v", err)
	}
	if copied, err := device.attachments.List(ctx, copyNoteID); err != nil || len(copied) != 0 {
		t.Fatalf("active copied attachments after incomplete stage = %#v, err=%v", copied, err)
	}
	restartSyncAttachmentTestDevice(t, device, remote)

	copyNote, err := device.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("get copied note before incomplete-stage edit: %v", err)
	}
	editedContent := copyNote.Content + "\nedit while attachment recovery is pending"
	copyNote, err = device.notes.Update(ctx, copyNoteID, note.UpdateInput{
		Content: &editedContent, ExpectedRevision: &copyNote.Revision,
	})
	if err != nil {
		t.Fatalf("edit copied note while stage is incomplete: %v", err)
	}

	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync with incomplete attachment stage = %#v, err=%v", result, err)
	}
	manifest := readFakeRemoteManifest(t, remote)
	for _, entry := range manifest.Entries {
		if entry.EntityKey == note.SyncEntityKey(note.SyncEntityNote, copyNoteID) ||
			entry.EntityKey == note.SyncAttachmentEntityKey(copyNoteID, copyAttachmentID) {
			t.Fatalf("incomplete-stage copy was published: %#v", entry)
		}
	}
	pending, err := device.repository.ListOutbox(ctx, 1000)
	if err != nil {
		t.Fatalf("list outbox after incomplete-stage sync: %v", err)
	}
	copyOutboxKept := false
	for _, item := range pending {
		if item.EntityKey == note.SyncEntityKey(note.SyncEntityNote, copyNoteID) {
			copyOutboxKept = true
			break
		}
	}
	if !copyOutboxKept {
		t.Fatalf("edited copied note outbox was lost while attachment stage was incomplete")
	}
	if ready, err := device.attachments.RecoverSyncCopy(ctx, copyNoteID); err != nil || ready {
		t.Fatalf("incomplete attachment stage recovery probe = ready:%v err:%v", ready, err)
	}

	if err := device.service.ResolveConflict(ctx, ConflictResolutionInput{ConflictID: conflict.ID, Choice: "both"}); err != nil {
		t.Fatalf("complete incomplete-stage keep-both retry: %v", err)
	}
	copyNote, err = device.notes.Get(ctx, copyNoteID)
	if err != nil {
		t.Fatalf("get copied note after incomplete-stage retry: %v", err)
	}
	if copyNote.Content != editedContent {
		t.Fatalf("copied note was overwritten during stage recovery = %q, want %q", copyNote.Content, editedContent)
	}
	if copied, err := device.attachments.List(ctx, copyNoteID); err != nil || len(copied) != 1 || copied[0].ID != copyAttachmentID {
		t.Fatalf("recovered copied attachments = %#v, err=%v", copied, err)
	}
	if result, err := device.service.SyncNow(ctx, SyncNowInput{}); err != nil {
		t.Fatalf("sync after incomplete-stage recovery = %#v, err=%v", result, err)
	}

	manifest = readFakeRemoteManifest(t, remote)
	entries := make(map[string]ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entries[entry.EntityKey] = entry
	}
	copyNoteEntry, ok := entries[note.SyncEntityKey(note.SyncEntityNote, copyNoteID)]
	if !ok {
		t.Fatalf("recovered edited copied note is missing from remote manifest: %#v", manifest.Entries)
	}
	if _, ok := entries[note.SyncAttachmentEntityKey(copyNoteID, copyAttachmentID)]; !ok {
		t.Fatalf("recovered edited copied attachment is missing from remote manifest: %#v", manifest.Entries)
	}
	copyObject, err := decodeObject(remote.files[objectPath(copyNoteEntry.ObjectHash)])
	if err != nil {
		t.Fatalf("decode recovered edited copied note object: %v", err)
	}
	var copyPayload note.SyncNotePayload
	if err := json.Unmarshal(copyObject.Payload, &copyPayload); err != nil {
		t.Fatalf("decode recovered edited copied note payload: %v", err)
	}
	if copyPayload.Content != editedContent {
		t.Fatalf("recovered remote copied note content = %q, want %q", copyPayload.Content, editedContent)
	}

	_, sourceBytes, err := device.attachments.Read(ctx, sourceAttachment.NoteID, sourceAttachment.ID)
	if err != nil {
		t.Fatalf("read source bytes after incomplete-stage recovery: %v", err)
	}
	redownloaded := newSyncAttachmentTestDevice(t, remote, vaultID)
	if result, err := redownloaded.service.SyncNow(ctx, SyncNowInput{ImportRemote: true}); err != nil || result.Status != StatusSynced {
		t.Fatalf("redownload after incomplete-stage recovery = %#v, err=%v", result, err)
	}
	importedCopy, err := redownloaded.notes.Get(ctx, copyNoteID)
	if err != nil || importedCopy.Content != editedContent {
		t.Fatalf("redownloaded copied note after incomplete-stage recovery = %#v, err=%v", importedCopy, err)
	}
	if _, copiedBytes, err := redownloaded.attachments.Read(ctx, copyNoteID, copyAttachmentID); err != nil || !bytes.Equal(copiedBytes, sourceBytes) {
		t.Fatalf("redownloaded copied attachment after incomplete-stage recovery: err=%v", err)
	}
	if _, err := redownloaded.notes.Get(ctx, sourceAttachment.NoteID); err != nil {
		t.Fatalf("source note was lost after incomplete-stage recovery: %v", err)
	}
}
