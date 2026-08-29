package sync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
)

func TestRecordSyncChangesPersistsOutboxAndBaseHeadAtomically(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint:          "https://dav.example.test",
		RemoteRoot:        "/atlasnote",
		Username:          "alice",
		VaultID:           strings.Repeat("a", 32),
		HeadManifestHash:  strings.Repeat("b", 64),
		HeadETag:          `"head-1"`,
		Status:            StatusIdle,
		AllowInsecureHTTP: true,
		CredentialRef:     "credential-ref",
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	savedConnection, err := repository.GetConnection(ctx)
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	if savedConnection == nil || !savedConnection.AllowInsecureHTTP {
		t.Fatalf("allow insecure HTTP setting was not persisted: %#v", savedConnection)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	change, err := note.NewNoteSyncChange("change-1", note.Record{
		ID:        strings.Repeat("1", 32),
		Title:     "Title",
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}, "# Title")
	if err != nil {
		t.Fatalf("build sync change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{change}); err != nil {
		t.Fatalf("enqueue change: %v", err)
	}

	items, err := repository.ListOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("outbox length = %d, want 1", len(items))
	}
	if items[0].BaseManifestHash != strings.Repeat("b", 64) || items[0].BaseHeadETag != `"head-1"` {
		t.Fatalf("outbox base = (%q, %q)", items[0].BaseManifestHash, items[0].BaseHeadETag)
	}
	if strings.Contains(items[0].ObjectJSON, "password") {
		t.Fatal("outbox object must not contain credential fields")
	}

	state, err := repository.GetItemState(ctx, note.SyncEntityKey(note.SyncEntityNote, strings.Repeat("1", 32)))
	if err != nil {
		t.Fatalf("get item state: %v", err)
	}
	if state == nil || state.LocalObjectHash == "" || state.BaseObjectHash != "" {
		t.Fatalf("unexpected item state: %#v", state)
	}

	first := items[0]
	if err := repository.MarkItemSynced(ctx, first.EntityKey, first.EntityType, first.ObjectHash, first.ObjectJSON); err != nil {
		t.Fatalf("mark first object synced: %v", err)
	}
	second, err := note.NewNoteSyncChange("change-2", note.Record{
		ID:        strings.Repeat("1", 32),
		Title:     "Title 2",
		Revision:  2,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Second),
	}, "# Title 2")
	if err != nil {
		t.Fatalf("build second sync change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{second}); err != nil {
		t.Fatalf("enqueue second change: %v", err)
	}
	items, err = repository.ListOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list coalesced outbox: %v", err)
	}
	if len(items) != 1 || items[0].ObjectHash == first.ObjectHash {
		t.Fatalf("outbox was not coalesced: %#v", items)
	}
	baseSnapshot, err := repository.GetSnapshot(ctx, first.EntityKey, first.ObjectHash)
	if err != nil {
		t.Fatalf("get base snapshot: %v", err)
	}
	if baseSnapshot != first.ObjectJSON {
		t.Fatalf("base snapshot = %q, want %q", baseSnapshot, first.ObjectJSON)
	}
}

func TestRequeueConflictLocalKeepsLatestEditAndRebasesRemote(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	if err := repository.SaveConnection(ctx, Connection{
		Endpoint: "https://dav.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("a", 32), HeadManifestHash: strings.Repeat("b", 64),
		HeadETag: `"head-current"`, Status: StatusConflict, CredentialRef: "ref",
	}); err != nil {
		t.Fatalf("save connection: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	buildChange := func(changeSetID string, title string, body string, revision int64) note.SyncChange {
		t.Helper()
		change, changeErr := note.NewNoteSyncChange(changeSetID, note.Record{
			ID: strings.Repeat("1", 32), Title: title, Revision: revision, CreatedAt: now, UpdatedAt: now.Add(time.Duration(revision) * time.Second),
		}, body)
		if changeErr != nil {
			t.Fatalf("build %s change: %v", title, changeErr)
		}
		return change
	}

	baseChange := buildChange("base", "Base", "base", 1)
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{baseChange}); err != nil {
		t.Fatalf("enqueue base: %v", err)
	}
	baseItems, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(baseItems) != 1 {
		t.Fatalf("read base outbox: items=%#v err=%v", baseItems, err)
	}
	base := baseItems[0]
	if err := repository.MarkItemSynced(ctx, base.EntityKey, base.EntityType, base.ObjectHash, base.ObjectJSON); err != nil {
		t.Fatalf("mark base synced: %v", err)
	}

	conflictedChange := buildChange("conflicted", "Conflicted", "conflicted", 2)
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{conflictedChange}); err != nil {
		t.Fatalf("enqueue conflicted local version: %v", err)
	}
	conflictedItems, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(conflictedItems) != 1 {
		t.Fatalf("read conflicted outbox: items=%#v err=%v", conflictedItems, err)
	}
	conflicted := conflictedItems[0]

	remoteChange := buildChange("remote", "Remote", "remote", 2)
	remoteJSON, err := objectDocument(remoteChange.EntityType, remoteChange.EntityKey, remoteChange.ObjectJSON, false)
	if err != nil {
		t.Fatalf("build remote object: %v", err)
	}
	remoteHash := hashBytes(remoteJSON)

	latestChange := buildChange("latest", "Latest", "latest", 3)
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{latestChange}); err != nil {
		t.Fatalf("enqueue edit made after conflict: %v", err)
	}
	latestItems, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(latestItems) != 1 {
		t.Fatalf("read latest outbox: items=%#v err=%v", latestItems, err)
	}
	latest := latestItems[0]

	if err := repository.RequeueConflictLocal(ctx, Conflict{
		ID: "conflict-1", EntityKey: conflicted.EntityKey, EntityType: conflicted.EntityType,
		LocalObjectHash: conflicted.ObjectHash, LocalSnapshot: conflicted.ObjectJSON,
		RemoteObjectHash: remoteHash, RemoteSnapshot: string(remoteJSON),
	}); err != nil {
		t.Fatalf("requeue local conflict: %v", err)
	}
	requeued, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(requeued) != 1 {
		t.Fatalf("read requeued outbox: items=%#v err=%v", requeued, err)
	}
	if requeued[0].ObjectHash != latest.ObjectHash || requeued[0].ObjectJSON != latest.ObjectJSON ||
		requeued[0].AttemptCount != 0 || requeued[0].FailedClass != "" {
		t.Fatalf("latest local edit was not preserved: got=%#v latest=%#v", requeued[0], latest)
	}
	state, err := repository.GetItemState(ctx, conflicted.EntityKey)
	if err != nil {
		t.Fatalf("read rebased item state: %v", err)
	}
	if state == nil || state.LocalObjectHash != latest.ObjectHash || state.BaseObjectHash != remoteHash || state.RemoteObjectHash != remoteHash {
		t.Fatalf("conflict state was not rebased: %#v", state)
	}
	remoteBase, err := repository.GetSnapshot(ctx, conflicted.EntityKey, remoteHash)
	if err != nil || remoteBase != string(remoteJSON) {
		t.Fatalf("remote conflict base snapshot = %q err=%v", remoteBase, err)
	}
}

func TestConfigureConnectionRollsBackTargetResetWhenSnapshotInsertFails(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	repository := NewRepository(db)
	oldConnection := Connection{
		Endpoint: "https://old.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("a", 32), Status: StatusIdle, CredentialRef: "old-ref",
	}
	if err := repository.SaveConnection(ctx, oldConnection); err != nil {
		t.Fatalf("save old connection: %v", err)
	}
	now := time.Now().UTC()
	oldChange, err := note.NewNoteSyncChange("old-change", note.Record{
		ID: strings.Repeat("2", 32), Title: "Old", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, "old")
	if err != nil {
		t.Fatalf("build old change: %v", err)
	}
	if err := repository.EnqueueChanges(ctx, []note.SyncChange{oldChange}); err != nil {
		t.Fatalf("enqueue old change: %v", err)
	}
	oldItems, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(oldItems) != 1 {
		t.Fatalf("read old outbox: items=%#v err=%v", oldItems, err)
	}
	if _, err := db.ExecContext(ctx, `
CREATE TRIGGER fail_sync_outbox_insert
BEFORE INSERT ON sync_outbox
BEGIN
	SELECT RAISE(ABORT, 'injected sync outbox failure');
END;
`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	newChange, err := note.NewNoteSyncChange("new-change", note.Record{
		ID: strings.Repeat("3", 32), Title: "New", Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, "new")
	if err != nil {
		t.Fatalf("build new change: %v", err)
	}
	err = repository.ConfigureConnection(ctx, Connection{
		Endpoint: "https://new.example.test", RemoteRoot: "/atlasnote", Username: "alice",
		VaultID: strings.Repeat("b", 32), Status: StatusIdle, CredentialRef: "new-ref",
	}, true, []note.SyncChange{newChange})
	if err == nil {
		t.Fatal("configuration unexpectedly succeeded through injected outbox failure")
	}
	saved, err := repository.GetConnection(ctx)
	if err != nil {
		t.Fatalf("read connection after rollback: %v", err)
	}
	if saved == nil || saved.Endpoint != oldConnection.Endpoint || saved.VaultID != oldConnection.VaultID {
		t.Fatalf("connection was partially replaced: %#v", saved)
	}
	items, err := repository.ListOutbox(ctx, 10)
	if err != nil || len(items) != 1 || items[0].ObjectHash != oldItems[0].ObjectHash {
		t.Fatalf("old tracking was not restored: items=%#v err=%v", items, err)
	}
}

func TestValidateEndpointRequiresExplicitHTTPOptIn(t *testing.T) {
	for _, endpoint := range []string{
		"https://alice:secret@dav.example.test",
		"https://dav.example.test?token=secret",
	} {
		if _, err := ValidateEndpoint(endpoint, false); err == nil {
			t.Fatalf("ValidateEndpoint(%q) accepted unsafe endpoint", endpoint)
		}
	}
	if _, err := ValidateEndpoint("http://dav.example.test", false); err == nil {
		t.Fatal("ValidateEndpoint accepted HTTP without explicit opt-in")
	}
	if endpoint, err := ValidateEndpoint("http://dav.example.test/base/", true); err != nil || endpoint != "http://dav.example.test/base" {
		t.Fatalf("HTTP endpoint with opt-in = %q, err = %v", endpoint, err)
	}
	if endpoint, err := ValidateEndpoint("https://dav.example.test/base/", false); err != nil || endpoint != "https://dav.example.test/base" {
		t.Fatalf("normalize endpoint = %q, err = %v", endpoint, err)
	}
}
