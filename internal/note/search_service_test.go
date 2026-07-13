package note_test

import (
	"path/filepath"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

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
