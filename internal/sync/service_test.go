package sync

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

type fakeRemote struct {
	files     map[string][]byte
	etags     map[string]string
	seq       int
	beforePut func(string, string)
}

// Some compatible WebDAV servers omit ETag on PUT but provide a strong ETag
// when the resource is fetched. The sync protocol verifies that GET response.
type putWithoutETagRemote struct {
	*fakeRemote
}

func (f *putWithoutETagRemote) Put(ctx context.Context, remotePath string, body []byte, ifMatch string, ifNoneMatch string) (RemoteResponse, error) {
	response, err := f.fakeRemote.Put(ctx, remotePath, body, ifMatch, ifNoneMatch)
	response.ETag = ""
	return response, err
}

// fallbackETagRemote simulates a server that exposes an ETag via WebDAV
// properties but omits it from GET response headers.
type fallbackETagRemote struct {
	*fakeRemote
	strongReadCount int
}

func (f *fallbackETagRemote) Get(ctx context.Context, remotePath string) (RemoteResponse, error) {
	response, err := f.fakeRemote.Get(ctx, remotePath)
	response.ETag = ""
	return response, err
}

func (f *fallbackETagRemote) GetWithStrongETag(ctx context.Context, remotePath string) (RemoteResponse, error) {
	f.strongReadCount++
	return f.fakeRemote.Get(ctx, remotePath)
}

type statusRemote struct {
	statusCode int
}

func (r statusRemote) response() (RemoteResponse, error) {
	return RemoteResponse{StatusCode: r.statusCode}, &HTTPStatusError{StatusCode: r.statusCode}
}

func (r statusRemote) Get(context.Context, string) (RemoteResponse, error) {
	return r.response()
}

func (r statusRemote) Put(context.Context, string, []byte, string, string) (RemoteResponse, error) {
	return r.response()
}

func (r statusRemote) Mkcol(context.Context, string) error {
	_, err := r.response()
	return err
}

func (r statusRemote) Propfind(context.Context, string, string) (RemoteResponse, error) {
	return r.response()
}

type countingCredentialStore struct {
	values      map[string]string
	setCalls    int
	deleteCalls int
}

type commitTempErrorStore struct {
	*storage.MarkdownStore
	err error
}

func (s *commitTempErrorStore) CommitTemp(context.Context, string, string) error {
	return s.err
}

func newCountingCredentialStore() *countingCredentialStore {
	return &countingCredentialStore{values: make(map[string]string)}
}

func (s *countingCredentialStore) Get(ref string) (string, error) {
	value, ok := s.values[ref]
	if !ok {
		return "", ErrCredentialNotFound
	}
	return value, nil
}

func (s *countingCredentialStore) Set(ref string, secret string) error {
	s.setCalls++
	s.values[ref] = secret
	return nil
}

func (s *countingCredentialStore) Delete(ref string) error {
	s.deleteCalls++
	delete(s.values, ref)
	return nil
}

func newFakeRemote() *fakeRemote {
	return &fakeRemote{files: make(map[string][]byte), etags: make(map[string]string)}
}

func (f *fakeRemote) Get(_ context.Context, remotePath string) (RemoteResponse, error) {
	body, ok := f.files[remotePath]
	if !ok {
		return RemoteResponse{StatusCode: http.StatusNotFound}, &HTTPStatusError{StatusCode: http.StatusNotFound}
	}
	return RemoteResponse{StatusCode: http.StatusOK, Body: append([]byte(nil), body...), ETag: f.etags[remotePath]}, nil
}

func (f *fakeRemote) Put(_ context.Context, remotePath string, body []byte, ifMatch string, ifNoneMatch string) (RemoteResponse, error) {
	if f.beforePut != nil {
		f.beforePut(remotePath, ifMatch)
	}
	_, exists := f.files[remotePath]
	if ifNoneMatch == "*" && exists {
		return RemoteResponse{StatusCode: http.StatusPreconditionFailed, ETag: f.etags[remotePath]}, &HTTPStatusError{StatusCode: http.StatusPreconditionFailed, ETag: f.etags[remotePath]}
	}
	if ifMatch != "" && f.etags[remotePath] != ifMatch {
		return RemoteResponse{StatusCode: http.StatusPreconditionFailed, ETag: f.etags[remotePath]}, &HTTPStatusError{StatusCode: http.StatusPreconditionFailed, ETag: f.etags[remotePath]}
	}
	f.seq++
	f.files[remotePath] = append([]byte(nil), body...)
	f.etags[remotePath] = `"etag-` + strings.TrimSpace(string(rune('0'+f.seq))) + `"`
	return RemoteResponse{StatusCode: http.StatusCreated, ETag: f.etags[remotePath]}, nil
}

func (f *fakeRemote) Mkcol(_ context.Context, _ string) error { return nil }

func (f *fakeRemote) Propfind(_ context.Context, _ string, _ string) (RemoteResponse, error) {
	return RemoteResponse{StatusCode: http.StatusMultiStatus}, nil
}

func TestRolledBackMarkdownMutationsDoNotLeaveUncommittedSyncPayloads(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	store, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	commitErr := errors.New("rename denied")
	failingNotes := note.NewService(noteRepository, &commitTempErrorStore{MarkdownStore: store, err: commitErr})

	if _, err := failingNotes.Create(ctx, note.CreateInput{Title: "Rejected", Content: "not canonical"}); !errors.Is(err, commitErr) {
		t.Fatalf("create error = %v, want commit error", err)
	}
	if count, err := repository.CountOutbox(ctx); err != nil || count != 0 {
		t.Fatalf("rolled back create outbox count = %d, err=%v", count, err)
	}
	var stateCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_item_states").Scan(&stateCount); err != nil || stateCount != 0 {
		t.Fatalf("rolled back create state count = %d, err=%v", stateCount, err)
	}

	notes := note.NewService(noteRepository, store)
	created, err := notes.Create(ctx, note.CreateInput{Title: "Original", Content: "canonical"})
	if err != nil {
		t.Fatalf("create canonical note: %v", err)
	}
	if _, err := failingNotes.Update(ctx, created.ID, note.UpdateInput{
		Title: stringTestPointer("Rejected update"), Content: stringTestPointer("not committed"),
		ExpectedRevision: int64TestPointer(created.Revision),
	}); !errors.Is(err, commitErr) {
		t.Fatalf("update error = %v, want commit error", err)
	}
	items, err := repository.ListOutbox(ctx, 100)
	if err != nil {
		t.Fatalf("list rollback outbox: %v", err)
	}
	var restored *OutboxItem
	for index := range items {
		if items[index].EntityKey == note.SyncEntityKey(note.SyncEntityNote, created.ID) {
			restored = &items[index]
			break
		}
	}
	if restored == nil {
		t.Fatal("rolled back update did not restore the previous sync payload")
	}
	object, err := decodeObject([]byte(restored.ObjectJSON))
	if err != nil {
		t.Fatalf("decode restored sync object: %v", err)
	}
	var payload note.SyncNotePayload
	if err := json.Unmarshal(object.Payload, &payload); err != nil {
		t.Fatalf("decode restored note payload: %v", err)
	}
	if payload.Title != "Original" || payload.Content != "canonical" {
		t.Fatalf("restored payload = %#v", payload)
	}
}

func TestDeleteTagQueuesRemainingNoteTagsInsteadOfRelationTombstone(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	store, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	notes := note.NewService(noteRepository, store)
	createdNote, err := notes.Create(ctx, note.CreateInput{Title: "Tagged", Content: "body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	first, err := notes.CreateTag(ctx, note.TagCreateInput{Name: "First"})
	if err != nil || first.Tag == nil {
		t.Fatalf("create first tag: result=%#v err=%v", first, err)
	}
	second, err := notes.CreateTag(ctx, note.TagCreateInput{Name: "Second"})
	if err != nil || second.Tag == nil {
		t.Fatalf("create second tag: result=%#v err=%v", second, err)
	}
	if result, err := notes.SetNoteTags(ctx, createdNote.ID, note.SetNoteTagsInput{TagIDs: []string{first.Tag.ID, second.Tag.ID}}); err != nil || result.Error != nil {
		t.Fatalf("set note tags: result=%#v err=%v", result, err)
	}
	if result, err := notes.DeleteTag(ctx, first.Tag.ID); err != nil || result.Error != nil || !result.Deleted {
		t.Fatalf("delete first tag: result=%#v err=%v", result, err)
	}

	items, err := repository.ListOutbox(ctx, 100)
	if err != nil {
		t.Fatalf("list tag delete outbox: %v", err)
	}
	relationKey := note.SyncEntityKey(note.SyncEntityNoteTags, createdNote.ID)
	for _, item := range items {
		if item.EntityKey != relationKey {
			continue
		}
		object, decodeErr := decodeObject([]byte(item.ObjectJSON))
		if decodeErr != nil {
			t.Fatalf("decode note-tags object: %v", decodeErr)
		}
		if object.Deleted {
			t.Fatal("tag deletion queued a note-tags tombstone")
		}
		var payload note.SyncNoteTagsPayload
		if err := json.Unmarshal(object.Payload, &payload); err != nil {
			t.Fatalf("decode note-tags payload: %v", err)
		}
		if len(payload.TagIDs) != 1 || payload.TagIDs[0] != second.Tag.ID {
			t.Fatalf("remaining synced tags = %#v, want only %s", payload.TagIDs, second.Tag.ID)
		}
		return
	}
	t.Fatal("note-tags outbox item was not found")
}

func TestLoadRemoteStateRejectsManifestGenerationMismatch(t *testing.T) {
	ctx := context.Background()
	remote := newFakeRemote()
	vaultID := strings.Repeat("a", 32)
	if err := initializeRemote(ctx, remote, vaultID); err != nil {
		t.Fatalf("initialize remote: %v", err)
	}
	manifest := manifestFor(vaultID, 3, map[string]ManifestEntry{})
	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	manifestHash := hashBytes(manifestBytes)
	if _, err := remote.Put(ctx, manifestPath(manifestHash), manifestBytes, "", "*"); err != nil {
		t.Fatalf("put manifest: %v", err)
	}
	headBytes, err := canonicalJSON(HeadDocument{
		FormatVersion: FormatVersion, VaultID: vaultID, Generation: 4, ManifestHash: manifestHash,
	})
	if err != nil {
		t.Fatalf("encode head: %v", err)
	}
	if _, err := remote.Put(ctx, headPath, headBytes, remote.etags[headPath], ""); err != nil {
		t.Fatalf("put head: %v", err)
	}
	service := NewService(nil, nil, nil)
	if _, err := service.loadRemoteState(ctx, remote, Connection{VaultID: vaultID}, false); !errors.Is(err, ErrInvalidRemoteFormat) {
		t.Fatalf("generation mismatch error = %v, want ErrInvalidRemoteFormat", err)
	}
}

func stringTestPointer(value string) *string { return &value }
func int64TestPointer(value int64) *int64    { return &value }

func TestSyncNowInitializesRemoteAndUploadsImmutableObject(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	repository := NewRepository(db)
	vaultID := strings.Repeat("c", 32)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint:      "https://dav.example.test",
		RemoteRoot:    "/atlasnote",
		Username:      "alice",
		VaultID:       vaultID,
		Status:        StatusIdle,
		FailSafe:      true,
		CredentialRef: "ref-1",
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	change, err := note.NewNoteSyncChange("change-1", note.Record{
		ID: strings.Repeat("1", 32), Title: "Title", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, "body")
	if err != nil {
		t.Fatalf("build change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{change}); err != nil {
		t.Fatalf("enqueue change: %v", err)
	}

	credentials := NewCredentialManager(NewSessionCredentialStore())
	if _, err := credentials.Save("ref-1", "secret", false); err != nil {
		t.Fatalf("save credential: %v", err)
	}
	remoteStore := newFakeRemote()
	remote := &putWithoutETagRemote{fakeRemote: remoteStore}
	service := NewService(repository, nil, credentials)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })

	result, err := service.SyncNow(ctx, SyncNowInput{InitializeRemote: true})
	if err != nil {
		t.Fatalf("sync now: %v", err)
	}
	if result.Status != StatusSynced || result.Uploaded != 1 || result.Remaining != 0 {
		t.Fatalf("sync result = %#v", result)
	}
	if _, ok := remoteStore.files[formatPath]; !ok {
		t.Fatal("format.json was not initialized")
	}
	if _, ok := remoteStore.files[headPath]; !ok {
		t.Fatal("head.json was not initialized")
	}
	if len(remoteStore.files) < 5 {
		t.Fatalf("remote immutable layout has %d files, want format/head/manifest/object", len(remoteStore.files))
	}
}

func TestSyncNowMapsWebDAVUnauthorizedToAuthRequired(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("a", 32), Status: StatusIdle, CredentialRef: "ref",
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	credentials := NewCredentialManager(NewSessionCredentialStore())
	if _, err := credentials.Save("ref", "wrong", false); err != nil {
		t.Fatalf("save credential: %v", err)
	}
	service := NewService(repository, nil, credentials)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) {
		return statusRemote{statusCode: http.StatusUnauthorized}, nil
	})
	result, err := service.SyncNow(ctx, SyncNowInput{})
	if err == nil || result.Status != StatusAuthRequired {
		t.Fatalf("unauthorized sync result=%#v err=%v", result, err)
	}
	saved, getErr := repository.GetConnection(ctx)
	if getErr != nil || saved == nil || saved.Status != StatusAuthRequired {
		t.Fatalf("saved unauthorized status=%#v err=%v", saved, getErr)
	}
}

func TestConfigurationCheckIsReadOnlyAndAcceptsUninitializedRemote(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	secureStore := newCountingCredentialStore()
	remote := newFakeRemote()
	service := NewService(NewRepository(db), nil, NewCredentialManager(secureStore))
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })

	result, err := service.TestConfiguration(ctx, ConnectionInput{
		WebDAVURL: "https://dav.example.test/atlasnote", Username: "alice", Password: "secret",
	})
	if err != nil {
		t.Fatalf("test configuration: %v", err)
	}
	if !result.Success || result.RemoteInitialized {
		t.Fatalf("configuration result = %#v", result)
	}
	connection, err := NewRepository(db).GetConnection(ctx)
	if err != nil {
		t.Fatalf("read connection after check: %v", err)
	}
	if connection != nil || secureStore.setCalls != 0 || len(remote.files) != 0 {
		t.Fatalf("configuration check wrote state: connection=%#v sets=%d remoteFiles=%d", connection, secureStore.setCalls, len(remote.files))
	}
}

func TestConfigurationCheckReportsAuthenticationPermissionAndTimeout(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		statusCode int
	}{
		{name: "authentication", statusCode: http.StatusUnauthorized},
		{name: "permission", statusCode: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.statusCode)
			}))
			defer server.Close()
			db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
			if err != nil {
				t.Fatalf("open database: %v", err)
			}
			defer db.Close()
			service := NewService(NewRepository(db), nil, NewCredentialManager(NewSessionCredentialStore()))
			_, err = service.TestConfiguration(t.Context(), ConnectionInput{
				WebDAVURL: server.URL, Username: "alice", Password: "secret", AllowInsecureHTTP: true,
			})
			var statusErr *HTTPStatusError
			if !errors.As(err, &statusErr) || statusErr.StatusCode != testCase.statusCode {
				t.Fatalf("configuration error = %v", err)
			}
		})
	}

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			_, _ = io.Copy(io.Discard, request.Body)
			<-request.Context().Done()
		}))
		defer server.Close()
		db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		defer db.Close()
		service := NewService(NewRepository(db), nil, NewCredentialManager(NewSessionCredentialStore()))
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		_, err = service.TestConfiguration(ctx, ConnectionInput{
			WebDAVURL: server.URL, Username: "alice", Password: "secret", AllowInsecureHTTP: true,
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout error = %v", err)
		}
	})
}

func TestConfigureDoesNotReuseCredentialForChangedTargetOrWriteOnFailedSetup(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://old.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("a", 32), Status: StatusIdle, CredentialRef: "old-ref", FailSafe: true,
	}); err != nil {
		t.Fatalf("save existing connection: %v", err)
	}
	secureStore := newCountingCredentialStore()
	secureStore.values["old-ref"] = "old-secret"
	service := NewService(repository, nil, NewCredentialManager(secureStore))
	clientCalls := 0
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) {
		clientCalls++
		return newFakeRemote(), nil
	})

	_, err = service.Configure(ctx, ConnectionInput{
		WebDAVURL: "https://new.example.test/atlasnote", Username: "alice", SetupMode: "initialize",
	})
	if err == nil || clientCalls != 0 {
		t.Fatalf("changed target reused a credential: err=%v clientCalls=%d", err, clientCalls)
	}

	initializedRemote := newFakeRemote()
	if err := initializeRemote(ctx, initializedRemote, strings.Repeat("b", 32)); err != nil {
		t.Fatalf("initialize fake remote: %v", err)
	}
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return initializedRemote, nil })
	_, err = service.Configure(ctx, ConnectionInput{
		WebDAVURL: "https://new.example.test/atlasnote", Username: "alice", Password: "new-secret", SetupMode: "initialize",
	})
	if !errors.Is(err, ErrRemoteAlreadyInitialized) {
		t.Fatalf("failed setup error = %v", err)
	}
	if secureStore.setCalls != 0 {
		t.Fatalf("failed setup wrote %d credentials", secureStore.setCalls)
	}
	saved, err := repository.GetConnection(ctx)
	if err != nil {
		t.Fatalf("read connection: %v", err)
	}
	if saved == nil || saved.Endpoint != "https://old.example.test" {
		t.Fatalf("failed setup changed connection: %#v", saved)
	}
}

func TestConfigureInitializeCanResumeWithManualSyncAndReportsPending(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	store, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	noteRepository := note.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(repository)
	notes := note.NewService(noteRepository, store)
	if _, err := notes.Create(ctx, note.CreateInput{Title: "Local", Content: "local body"}); err != nil {
		t.Fatalf("create local note: %v", err)
	}
	secureStore := newCountingCredentialStore()
	remote := newFakeRemote()
	service := NewService(repository, notes, NewCredentialManager(secureStore))
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })

	configured, err := service.Configure(ctx, ConnectionInput{
		WebDAVURL: "https://dav.example.test/atlasnote", Username: "alice", Password: "secret", SetupMode: "initialize",
	})
	if err != nil {
		t.Fatalf("configure initialize: %v", err)
	}
	if configured.Status != StatusPending || configured.OutboxCount == 0 {
		t.Fatalf("configured status = %#v", configured)
	}
	result, err := service.SyncNow(ctx, SyncNowInput{})
	if err != nil {
		t.Fatalf("resume initialization with manual sync: %v", err)
	}
	if result.Status != StatusSynced || result.Uploaded == 0 {
		t.Fatalf("manual initialization result = %#v", result)
	}
	if _, ok := remote.files[formatPath]; !ok {
		t.Fatal("manual sync did not initialize the selected empty remote")
	}
}

func TestReconnectToSameVaultPreservesTracking(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	vaultID := strings.Repeat("a", 32)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://old.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: vaultID, HeadManifestHash: strings.Repeat("b", 64), HeadETag: `"old-head"`,
		Status: StatusIdle, CredentialRef: "old-ref",
	}); err != nil {
		t.Fatalf("save old connection: %v", err)
	}
	change, err := note.NewNoteSyncChange("local-change", note.Record{
		ID: strings.Repeat("1", 32), Title: "Local", Revision: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, "local")
	if err != nil {
		t.Fatalf("build local change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{change}); err != nil {
		t.Fatalf("enqueue local change: %v", err)
	}
	before, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(before) != 1 {
		t.Fatalf("read tracking before reconnect: items=%#v err=%v", before, err)
	}
	secureStore := newCountingCredentialStore()
	secureStore.values["old-ref"] = "old-secret"
	remote := newFakeRemote()
	if err := initializeRemote(ctx, remote, vaultID); err != nil {
		t.Fatalf("initialize same remote vault: %v", err)
	}
	service := NewService(repository, nil, NewCredentialManager(secureStore))
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })

	configured, err := service.Configure(ctx, ConnectionInput{
		WebDAVURL: "https://new.example.test/atlasnote", Username: "alice", Password: "new-secret", SetupMode: "reconnect",
	})
	if err != nil {
		t.Fatalf("reconnect same vault: %v", err)
	}
	if configured.Status != StatusPending {
		t.Fatalf("reconnect status = %#v", configured)
	}
	after, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(after) != 1 || after[0].ObjectHash != before[0].ObjectHash {
		t.Fatalf("reconnect reset tracking: before=%#v after=%#v err=%v", before, after, err)
	}
	saved, err := repository.GetConnection(ctx)
	if err != nil || saved == nil || saved.VaultID != vaultID || saved.Endpoint != "https://new.example.test/atlasnote" {
		t.Fatalf("reconnected connection = %#v err=%v", saved, err)
	}
}

func TestGetStatusDoesNotKeepResolvedConflictOrIdleWithOutbox(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("a", 32), Status: StatusConflict, CredentialRef: "ref",
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	service := NewService(repository, nil, NewCredentialManager(NewSessionCredentialStore()))
	status, err := service.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get resolved conflict status: %v", err)
	}
	if status.Status != StatusSynced || status.Connection == nil || status.Connection.Status != StatusSynced {
		t.Fatalf("resolved conflict status = %#v", status)
	}
	change, err := note.NewNoteSyncChange("pending", note.Record{
		ID: strings.Repeat("1", 32), Title: "Pending", Revision: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, "pending")
	if err != nil {
		t.Fatalf("build pending change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{change}); err != nil {
		t.Fatalf("enqueue pending change: %v", err)
	}
	status, err = service.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get pending status: %v", err)
	}
	if status.Status != StatusPending || status.Connection == nil || status.Connection.Status != StatusPending {
		t.Fatalf("pending status = %#v", status)
	}
}

func TestEmptyRemoteFailSafeBlocksLocalDataButAllowsEmptyLocal(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		withNote  bool
		wantBlock bool
	}{
		{name: "local data", withNote: true, wantBlock: true},
		{name: "empty local", withNote: false, wantBlock: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
			if err != nil {
				t.Fatalf("open database: %v", err)
			}
			defer db.Close()
			store, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
			if err != nil {
				t.Fatalf("create markdown store: %v", err)
			}
			notes := note.NewService(note.NewRepository(db), store)
			if testCase.withNote {
				if _, err := notes.Create(ctx, note.CreateInput{Title: "Keep me", Content: "local body"}); err != nil {
					t.Fatalf("create local note: %v", err)
				}
			}

			vaultID := strings.Repeat("c", 32)
			remote := newFakeRemote()
			if err := initializeRemote(ctx, remote, vaultID); err != nil {
				t.Fatalf("initialize fake remote: %v", err)
			}
			repository := NewRepository(db)
			if err := repository.SaveConnection(ctx, Connection{
				Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
				VaultID: vaultID, Status: StatusIdle, CredentialRef: "ref", HasLastSync: true,
				LastSyncAt: time.Now().UTC(), HeadManifestHash: strings.Repeat("d", 64), FailSafe: true,
			}); err != nil {
				t.Fatalf("save connection: %v", err)
			}
			credentials := NewCredentialManager(NewSessionCredentialStore())
			if _, err := credentials.Save("ref", "secret", false); err != nil {
				t.Fatalf("save credential: %v", err)
			}
			service := NewService(repository, notes, credentials)
			service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })

			result, err := service.SyncNow(ctx, SyncNowInput{})
			if testCase.wantBlock {
				if !errors.Is(err, ErrFailSafeTriggered) || result.Status != StatusFailed {
					t.Fatalf("fail-safe result=%#v err=%v", result, err)
				}
				changes, exportErr := notes.ExportSyncChanges(ctx)
				if exportErr != nil || len(changes) == 0 {
					t.Fatalf("local data changed after fail-safe: changes=%d err=%v", len(changes), exportErr)
				}
			} else if err != nil || result.Status != StatusSynced {
				t.Fatalf("empty local sync result=%#v err=%v", result, err)
			}
		})
	}
}

func TestFailSafeDoesNotTreatTombstoneManifestAsEmpty(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	store, err := storage.NewMarkdownStore(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	notes := note.NewService(note.NewRepository(db), store)
	if _, err := notes.Create(ctx, note.CreateInput{Title: "Keep me", Content: "local body"}); err != nil {
		t.Fatalf("create local note: %v", err)
	}

	vaultID := strings.Repeat("e", 32)
	remote := newFakeRemote()
	if err := initializeRemote(ctx, remote, vaultID); err != nil {
		t.Fatalf("initialize fake remote: %v", err)
	}
	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: vaultID, Status: StatusIdle, CredentialRef: "ref", HasLastSync: true,
		LastSyncAt: time.Now().UTC(), FailSafe: true,
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{note.NewNoteTombstoneChange("change", strings.Repeat("f", 32))}); err != nil {
		t.Fatalf("enqueue tombstone: %v", err)
	}
	credentials := NewCredentialManager(NewSessionCredentialStore())
	_, _ = credentials.Save("ref", "secret", false)
	service := NewService(repository, notes, credentials)
	service.SetClientFactory(func(Connection, string) (RemoteClient, error) { return remote, nil })
	result, err := service.SyncNow(ctx, SyncNowInput{})
	if err != nil || result.Status != StatusSynced {
		t.Fatalf("tombstone sync result=%#v err=%v", result, err)
	}
}

func TestRequireStrongETagRejectsWeakAndUnquotedValues(t *testing.T) {
	for _, value := range []string{"", "W/\"weak\"", "etag"} {
		if !errors.Is(requireStrongETag(value), ErrMissingStrongETag) {
			t.Fatalf("etag %q was accepted", value)
		}
	}
	if err := requireStrongETag(`"strong"`); err != nil {
		t.Fatalf("strong etag rejected: %v", err)
	}
}

func TestLoadRemoteStateUsesStrongETagReaderWhenGetHeaderIsMissing(t *testing.T) {
	ctx := context.Background()
	vaultID := strings.Repeat("a", 32)
	remote := &fallbackETagRemote{fakeRemote: newFakeRemote()}
	if err := initializeRemote(ctx, remote, vaultID); err != nil {
		t.Fatalf("initialize remote: %v", err)
	}

	state, err := (&Service{}).loadRemoteState(ctx, remote, Connection{VaultID: vaultID}, false)
	if err != nil {
		t.Fatalf("load remote state with ETag reader: %v", err)
	}
	if err := requireStrongETag(state.headETag); err != nil {
		t.Fatalf("head ETag = %q, err=%v", state.headETag, err)
	}
	if remote.strongReadCount != 1 {
		t.Fatalf("strong ETag reader calls = %d, want 1", remote.strongReadCount)
	}
}

func TestRemoteDocumentsRejectInvalidEntityContracts(t *testing.T) {
	format := FormatDocument{FormatVersion: FormatVersion, VaultID: strings.Repeat("a", 32)}
	for _, entry := range []ManifestEntry{
		{EntityKey: "note:not-hex", EntityType: note.SyncEntityNote, ObjectHash: strings.Repeat("b", 64)},
		{EntityKey: note.SyncEntityKey(note.SyncEntityTag, strings.Repeat("1", 32)), EntityType: note.SyncEntityNote, ObjectHash: strings.Repeat("b", 64)},
		{EntityKey: "unknown:" + strings.Repeat("1", 32), EntityType: "unknown", ObjectHash: strings.Repeat("b", 64)},
	} {
		manifest := ManifestDocument{FormatVersion: FormatVersion, VaultID: format.VaultID, Generation: 1, Entries: []ManifestEntry{entry}}
		encoded, err := canonicalJSON(manifest)
		if err != nil {
			t.Fatalf("encode invalid manifest: %v", err)
		}
		if !errors.Is(validateManifest(manifest, format, hashBytes(encoded)), ErrInvalidRemoteFormat) {
			t.Fatalf("invalid manifest entry was accepted: %#v", entry)
		}
	}

	invalidTombstone, err := canonicalJSON(ObjectDocument{
		FormatVersion: FormatVersion,
		EntityKey:     note.SyncEntityKey(note.SyncEntityNote, strings.Repeat("1", 32)),
		EntityType:    note.SyncEntityNote,
		Deleted:       true,
		Payload:       []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("encode invalid tombstone: %v", err)
	}
	if _, err := decodeObject(invalidTombstone); !errors.Is(err, ErrInvalidRemoteFormat) {
		t.Fatalf("tombstone payload was accepted: %v", err)
	}
}
