package readapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/note"
)

func TestChangeStoreExpiryClientAndSpaceBinding(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	service := New(nil, nil, "space-a")
	service.changes.now = func() time.Time { return now }
	id := strings.Repeat("a", 32)
	service.changes.items[id] = &pendingChange{clientID: "client-a", review: ChangeReview{OperationID: id, StorageSpaceID: "space-a", State: StatusPending, CreatedAt: now, ExpiresAt: now.Add(changeTTL)}}
	request := Request{RequestID: "get", Params: json.RawMessage(`{"operationId":"` + id + `"}`)}
	owner := Principal{ClientID: "client-a", StorageSpaceID: "space-a"}
	if response := service.getOperation(owner, request); response.Status != StatusOK {
		t.Fatalf("owner get: %+v", response)
	}
	other := owner
	other.ClientID = "client-b"
	if response := service.getOperation(other, request); response.Error == nil || response.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("other client get: %+v", response)
	}
	other = owner
	other.StorageSpaceID = "space-b"
	if response := service.getOperation(other, request); response.Error == nil || response.Error.Code != "RESOURCE_UNAVAILABLE" {
		t.Fatalf("other space get: %+v", response)
	}
	if reviews := service.ListChangeReviews("space-b"); reviews != nil {
		t.Fatalf("other space GUI review: %+v", reviews)
	}
	now = now.Add(changeTTL + time.Second)
	response := service.getOperation(owner, request)
	data, ok := response.Data.(map[string]any)
	if response.Status != StatusOK || !ok || data["state"] != "expired" {
		t.Fatalf("expired operation: %+v", response)
	}
	if result := service.RejectChange("space-a", id); result.State != "unavailable" {
		t.Fatalf("expired operation was mutable: %+v", result)
	}
}

func TestApprovalPermitIsSingleUseAndBoundToState(t *testing.T) {
	now := time.Now().UTC()
	change := &pendingChange{review: ChangeReview{OperationID: strings.Repeat("b", 32), StorageSpaceID: "space-a", TargetIDs: []string{strings.Repeat("c", 32)}, ExpectedRevision: 3}}
	state := [32]byte{1}
	permit, token, err := issueApprovalPermit(change, state, now)
	if err != nil {
		t.Fatal(err)
	}
	otherState := [32]byte{2}
	if permit.consume(change, token, otherState, now) {
		t.Fatal("permit accepted another target state")
	}
	forged := token
	forged[0] ^= 1
	if permit.consume(change, forged, state, now) {
		t.Fatal("permit accepted a forged token")
	}
	if !permit.consume(change, token, state, now) {
		t.Fatal("permit rejected original state")
	}
	if permit.consume(change, token, state, now) {
		t.Fatal("permit was reused")
	}
	other, otherToken, err := issueApprovalPermit(change, state, now)
	if err != nil {
		t.Fatal(err)
	}
	if other.consume(change, otherToken, state, now.Add(time.Minute)) {
		t.Fatal("expired permit was accepted")
	}
}

type failingChangeReader struct {
	noteReader
	current note.Note
}

func (r failingChangeReader) BeginExternalRead(ctx context.Context) (context.Context, func()) {
	return ctx, func() {}
}
func (r failingChangeReader) Get(_ context.Context, id string) (note.Note, error) {
	if id != r.current.ID {
		return note.Note{}, note.ErrNotFound
	}
	return r.current, nil
}
func (failingChangeReader) ListNoteTags(context.Context, string) (note.NoteTagsResult, error) {
	return note.NoteTagsResult{Tags: []note.Tag{}}, nil
}

type openChangeLocks struct{}

func (openChangeLocks) NoteLockStatus(context.Context, string) (bool, bool, string, error) {
	return false, false, "", nil
}
func (openChangeLocks) NotebookLockStatus(context.Context, string) (bool, bool, string, error) {
	return false, false, "", nil
}

type failingChangeWriter struct{}

func (failingChangeWriter) Create(context.Context, note.CreateInput) (note.Note, error) {
	return note.Note{}, errors.New("write failed")
}
func (failingChangeWriter) Update(context.Context, string, note.UpdateInput) (note.Note, error) {
	return note.Note{}, errors.New("write failed")
}
func (failingChangeWriter) SetNoteTagsWithExpectedRevision(context.Context, string, note.SetNoteTagsWithExpectedRevisionInput) (note.NoteTagsResult, error) {
	return note.NoteTagsResult{}, errors.New("write failed")
}

func TestBackendSaveFailureKeepsPendingChangeAndReview(t *testing.T) {
	id := strings.Repeat("a", 32)
	noteID := strings.Repeat("b", 32)
	now := time.Now().UTC()
	reader := failingChangeReader{current: note.Note{ID: noteID, Title: "before", Content: "body", Revision: 1}}
	service := New(reader, openChangeLocks{}, "space-a")
	service.writes = failingChangeWriter{}
	after := "after"
	service.changes.items[id] = &pendingChange{
		clientID: "client-a", principal: Principal{ClientID: "client-a", StorageSpaceID: "space-a"}, noteID: noteID, title: &after,
		review: ChangeReview{OperationID: id, Kind: OperationNotesRequestUpdate, StorageSpaceID: "space-a", TargetIDs: []string{noteID}, ExpectedRevision: 1, State: StatusPending, CreatedAt: now, ExpiresAt: now.Add(changeTTL), Items: []ChangeReviewItem{{NoteID: noteID, Before: "before", After: "after"}}},
	}
	result := service.ApproveChange(context.Background(), "space-a", id)
	if result.State != StatusPending {
		t.Fatalf("failed save discarded change: %+v", result)
	}
	stored := service.changes.items[id]
	if stored.review.State != StatusPending || stored.review.Items[0].After != "after" {
		t.Fatalf("failed save lost proposal: %+v", stored.review)
	}
	if reader.current.Title != "before" {
		t.Fatal("failed save changed note")
	}
}
