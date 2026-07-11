package note_test

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
)

func TestRepositoryUpdateCASIncrementsRevisionAndRejectsStaleUpdate(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	record := createRepositoryTestNote(t, repository, "cas-update")
	record.Title = "Updated"
	record.UpdatedAt = record.UpdatedAt.Add(time.Minute)

	nextRevision, err := repository.UpdateCAS(t.Context(), record, 1)
	if err != nil {
		t.Fatalf("update note with CAS: %v", err)
	}
	if nextRevision != 2 {
		t.Fatalf("next revision = %d, want 2", nextRevision)
	}

	updated, err := repository.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatalf("get updated note: %v", err)
	}
	if updated.Title != "Updated" || updated.Revision != 2 {
		t.Fatalf("updated record = %#v", updated)
	}

	record.Title = "Stale overwrite"
	_, err = repository.UpdateCAS(t.Context(), record, 1)
	assertRevisionConflict(t, err, record.ID, 1, 2)

	afterConflict, err := repository.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatalf("get note after conflict: %v", err)
	}
	if afterConflict.Title != "Updated" || afterConflict.Revision != 2 {
		t.Fatalf("record changed after conflict = %#v", afterConflict)
	}
}

func TestRepositoryConcurrentUpdateCASAllowsOnlyOneWriter(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	record := createRepositoryTestNote(t, repository, "concurrent-cas")
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup

	for _, title := range []string{"First", "Second"} {
		title := title
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			candidate := record
			candidate.Title = title
			candidate.UpdatedAt = candidate.UpdatedAt.Add(time.Minute)
			_, err := repository.UpdateCAS(t.Context(), candidate, 1)
			results <- err
		}()
	}

	close(start)
	workers.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, note.ErrRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results: successes=%d conflicts=%d", successes, conflicts)
	}

	updated, err := repository.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatalf("get concurrently updated note: %v", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision after concurrent update = %d, want 2", updated.Revision)
	}
}

func TestRepositoryUpdateWithStorageOperationCASDoesNotJournalConflict(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	record := createRepositoryTestNote(t, repository, "journal-conflict")
	record.Title = "Stale"
	operation := note.StorageOperation{
		ID:          "operation-conflict",
		NoteID:      record.ID,
		Type:        note.StorageOperationUpsert,
		ContentHash: "hash",
		CreatedAt:   time.Now().UTC(),
	}

	_, err := repository.UpdateWithStorageOperationCAS(t.Context(), record, operation, 0)
	assertRevisionConflict(t, err, record.ID, 0, 1)

	operations, err := repository.ListStorageOperations(t.Context())
	if err != nil {
		t.Fatalf("list storage operations: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("storage operations after conflict = %#v", operations)
	}
}

func TestRepositoryUpdateWithStorageOperationCASRollsBackRevisionWhenJournalInsertFails(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	record := createRepositoryTestNote(t, repository, "journal-rollback")
	if err := repository.BeginStorageOperation(t.Context(), note.StorageOperation{
		ID:        "duplicate-operation",
		NoteID:    "another-note",
		Type:      note.StorageOperationDelete,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("prepare duplicate storage operation: %v", err)
	}

	record.Title = "Must roll back"
	record.UpdatedAt = record.UpdatedAt.Add(time.Minute)
	_, err := repository.UpdateWithStorageOperationCAS(t.Context(), record, note.StorageOperation{
		ID:          "duplicate-operation",
		NoteID:      record.ID,
		Type:        note.StorageOperationUpsert,
		ContentHash: "hash",
		CreatedAt:   time.Now().UTC(),
	}, 1)
	if err == nil {
		t.Fatal("CAS update succeeded with duplicate storage operation")
	}

	afterFailure, getErr := repository.Get(t.Context(), record.ID)
	if getErr != nil {
		t.Fatalf("get note after rolled back CAS update: %v", getErr)
	}
	if afterFailure.Title != "Original" || afterFailure.Revision != 1 {
		t.Fatalf("record changed after rolled back CAS update = %#v", afterFailure)
	}
}

func TestRepositoryDeleteCASRejectsStaleRevision(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	record := createRepositoryTestNote(t, repository, "cas-delete")
	record.Title = "Revision two"
	record.UpdatedAt = record.UpdatedAt.Add(time.Minute)
	if _, err := repository.UpdateCAS(t.Context(), record, 1); err != nil {
		t.Fatalf("prepare revision two: %v", err)
	}

	err := repository.DeleteCAS(t.Context(), record.ID, 1)
	assertRevisionConflict(t, err, record.ID, 1, 2)
	if _, err := repository.Get(t.Context(), record.ID); err != nil {
		t.Fatalf("stale delete removed note: %v", err)
	}

	if err := repository.DeleteCAS(t.Context(), record.ID, 2); err != nil {
		t.Fatalf("delete note with current revision: %v", err)
	}
	if err := repository.DeleteCAS(t.Context(), record.ID, 2); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("delete missing note error = %v, want ErrNotFound", err)
	}
}

func newRepositoryTest(t *testing.T) *note.Repository {
	t.Helper()

	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return note.NewRepository(db)
}

func createRepositoryTestNote(t *testing.T, repository *note.Repository, id string) note.Record {
	t.Helper()

	now := time.Now().UTC()
	record := note.Record{
		ID:          id,
		Title:       "Original",
		ContentPath: id + ".md",
		Revision:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repository.Create(t.Context(), record); err != nil {
		t.Fatalf("create repository test note: %v", err)
	}

	return record
}

func assertRevisionConflict(
	t *testing.T,
	err error,
	noteID string,
	expectedRevision int64,
	actualRevision int64,
) {
	t.Helper()

	if !errors.Is(err, note.ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}
	var conflict *note.RevisionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error type = %T, want *note.RevisionConflict", err)
	}
	if conflict.Code != note.ErrorCodeRevisionConflict ||
		conflict.NoteID != noteID ||
		conflict.ExpectedRevision != expectedRevision ||
		conflict.ActualRevision != actualRevision {
		t.Fatalf("conflict = %#v", conflict)
	}
}
