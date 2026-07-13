package note_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func TestServiceRecoverReconcilesExternalMarkdownEditOnce(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "External", Content: "before"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := store.Write(ctx, created.ID, "after external edit"); err != nil {
		t.Fatalf("write external markdown: %v", err)
	}

	report, err := service.Recover(ctx)
	if err != nil || len(report.MissingNotes) != 0 {
		t.Fatalf("recover external edit = %#v, %v", report, err)
	}
	got, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get reconciled record: %v", err)
	}
	if got.Revision != created.Revision+1 {
		t.Fatalf("reconciled revision = %d, want %d", got.Revision, created.Revision+1)
	}

	if _, err := service.Recover(ctx); err != nil {
		t.Fatalf("repeat recovery: %v", err)
	}
	got, err = repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get record after repeat recovery: %v", err)
	}
	if got.Revision != created.Revision+1 {
		t.Fatalf("repeat recovery revision = %d, want %d", got.Revision, created.Revision+1)
	}
	result, err := service.Search(ctx, note.SearchInput{Query: "external"})
	if err != nil || result.Error != nil || result.Total != 1 {
		t.Fatalf("search reconciled content = %#v, %v", result, err)
	}
}

func TestServiceRecoverDoesNotTurnStaleSearchIndexIntoExternalConflict(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Stale index", Content: "before"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	record, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note record: %v", err)
	}
	record.UpdatedAt = time.Now().UTC()
	if _, err := repository.UpdateCAS(ctx, record, record.Revision); err != nil {
		t.Fatalf("advance record without index fixture: %v", err)
	}
	if err := store.Write(ctx, created.ID, "changed while index was stale"); err != nil {
		t.Fatalf("write external markdown: %v", err)
	}

	if _, err := service.Recover(ctx); err != nil {
		t.Fatalf("recover stale index: %v", err)
	}
	reconciled, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get recovered record: %v", err)
	}
	if reconciled.Revision != created.Revision+1 {
		t.Fatalf("recovered revision = %d, want %d", reconciled.Revision, created.Revision+1)
	}
}

func TestServiceRecoverTreatsRenamedMarkdownAsMissingAndOrphan(t *testing.T) {
	t.Parallel()

	ctx, _, store, service, notesDir := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Renamed", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	contentPath, err := store.ContentPath(created.ID)
	if err != nil {
		t.Fatalf("content path: %v", err)
	}
	if err := os.Rename(filepath.Join(notesDir, contentPath), filepath.Join(notesDir, "renamed-by-user.md")); err != nil {
		t.Fatalf("rename markdown fixture: %v", err)
	}

	report, err := service.Recover(ctx)
	if err != nil {
		t.Fatalf("recover renamed markdown: %v", err)
	}
	if len(report.MissingNotes) != 1 || report.MissingNotes[0].ID != created.ID {
		t.Fatalf("missing notes after rename = %#v", report.MissingNotes)
	}
	if _, err := os.Stat(filepath.Join(notesDir, "renamed-by-user.md")); !os.IsNotExist(err) {
		t.Fatalf("renamed orphan was not quarantined: %v", err)
	}
}

func TestServiceSearchDelegatesToRepository(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	db, err := database.Open(t.Context(), filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	repository := note.NewRepository(db)
	now := time.Now().UTC()
	record := note.Record{
		ID:          "service-search-note",
		Title:       "Searchable note",
		ContentPath: "service-search-note.md",
		Revision:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repository.Create(t.Context(), record); err != nil {
		t.Fatalf("create note record: %v", err)
	}
	if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{
		NoteID:   record.ID,
		Title:    record.Title,
		Body:     "service search body",
		Revision: record.Revision,
	}); err != nil {
		t.Fatalf("upsert search document: %v", err)
	}

	service := note.NewService(repository, store)
	result, err := service.Search(t.Context(), note.SearchInput{Query: "service"})
	if err != nil {
		t.Fatalf("search notes: %v", err)
	}
	if result.Error != nil || result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("search result = %#v", result)
	}
	if result.Items[0].Note.ID != record.ID {
		t.Fatalf("search result note ID = %q, want %q", result.Items[0].Note.ID, record.ID)
	}
}

func TestServiceSearchReturnsStructuredValidationError(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	service := note.NewService(repository, nil)

	result, err := service.Search(t.Context(), note.SearchInput{Query: "ok\x00"})
	if err != nil {
		t.Fatalf("search returned Go error: %v", err)
	}
	if result.Error == nil || result.Error.Code != note.SearchErrorQueryInvalid {
		t.Fatalf("search error = %#v", result.Error)
	}
}

func TestServiceWriteOperationsMaintainSearchIndex(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "First title", Content: "first body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	createdResult, err := service.Search(ctx, note.SearchInput{Query: "first"})
	if err != nil {
		t.Fatalf("search created note: %v", err)
	}
	if createdResult.Error != nil || createdResult.Total != 1 {
		t.Fatalf("created note search result = %#v", createdResult)
	}

	updatedTitle := "Second title"
	updatedContent := "second body"
	updated, err := service.Update(ctx, created.ID, note.UpdateInput{
		Title:            &updatedTitle,
		Content:          &updatedContent,
		ExpectedRevision: &created.Revision,
	})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}

	oldResult, err := service.Search(ctx, note.SearchInput{Query: "first"})
	if err != nil {
		t.Fatalf("search old content: %v", err)
	}
	newResult, err := service.Search(ctx, note.SearchInput{Query: "second"})
	if err != nil {
		t.Fatalf("search new content: %v", err)
	}
	if oldResult.Total != 0 || newResult.Total != 1 || newResult.Items[0].Note.ID != created.ID {
		t.Fatalf("updated search results: old=%#v new=%#v", oldResult, newResult)
	}

	if err := service.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: updated.Revision}); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	deletedResult, err := service.Search(ctx, note.SearchInput{Query: "second"})
	if err != nil {
		t.Fatalf("search deleted note: %v", err)
	}
	if deletedResult.Total != 0 {
		t.Fatalf("deleted note search result = %#v", deletedResult)
	}
}

func TestServiceRecoverRebuildsSearchIndex(t *testing.T) {
	t.Parallel()

	ctx, repository, _, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Rebuild title", Content: "rebuild body"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := repository.DeleteSearchIndex(ctx, created.ID); err != nil {
		t.Fatalf("remove search index fixture: %v", err)
	}

	missingResult, err := service.Search(ctx, note.SearchInput{Query: "rebuild"})
	if err != nil {
		t.Fatalf("search before rebuild: %v", err)
	}
	if missingResult.Total != 0 {
		t.Fatalf("search before rebuild = %#v", missingResult)
	}

	if _, err := service.Recover(ctx); err != nil {
		t.Fatalf("recover search index: %v", err)
	}
	rebuiltResult, err := service.Search(ctx, note.SearchInput{Query: "rebuild"})
	if err != nil {
		t.Fatalf("search after rebuild: %v", err)
	}
	if rebuiltResult.Error != nil || rebuiltResult.Total != 1 || rebuiltResult.Items[0].Note.ID != created.ID {
		t.Fatalf("search after rebuild = %#v", rebuiltResult)
	}
}

func TestServiceSearchIndexFailureDoesNotRollbackNote(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	db, err := database.Open(t.Context(), filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	repository := note.NewRepository(db)
	if _, err := db.ExecContext(t.Context(), "DROP TABLE note_search"); err != nil {
		t.Fatalf("drop search index fixture: %v", err)
	}
	service := note.NewService(repository, store)

	created, err := service.Create(t.Context(), note.CreateInput{Title: "Persisted title", Content: "persisted body"})
	if err != nil {
		t.Fatalf("create note with failed search index: %v", err)
	}
	if _, err := repository.Get(t.Context(), created.ID); err != nil {
		t.Fatalf("note was rolled back after search index failure: %v", err)
	}
	content, err := store.Read(t.Context(), created.ID)
	if err != nil || content != "persisted body" {
		t.Fatalf("markdown after search index failure = %q, %v", content, err)
	}

	result, err := service.Search(t.Context(), note.SearchInput{Query: "Persisted", Scope: note.SearchScopeTitle})
	if err != nil {
		t.Fatalf("search after index failure returned Go error: %v", err)
	}
	if result.Error == nil || result.Error.Code != note.SearchErrorIndexFailed {
		t.Fatalf("search error after index failure = %#v", result.Error)
	}
}
