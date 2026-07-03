package note_test

import (
	"context"
	"path/filepath"
	"testing"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func ptr[T any](v T) *T {
	return &v
}

func TestServiceCreateGetUpdateDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	service := note.NewService(note.NewRepository(db), store)

	created, err := service.Create(ctx, note.CreateInput{
		Title:   "First note",
		Content: "# Hello\n\nContent",
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created note id is empty")
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if got.Title != "First note" {
		t.Fatalf("got title %q", got.Title)
	}
	if got.Content != "# Hello\n\nContent" {
		t.Fatalf("got content %q", got.Content)
	}

	summaries, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summary count = %d", len(summaries))
	}

	updated, err := service.Update(ctx, created.ID, note.UpdateInput{
		Title:   ptr("Updated note"),
		Content: ptr("Updated content"),
	})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.Title != "Updated note" {
		t.Fatalf("updated title %q", updated.Title)
	}
	if updated.Content != "Updated content" {
		t.Fatalf("updated content %q", updated.Content)
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	if _, err := service.Get(ctx, created.ID); err != note.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestServiceUpdateCanClearNotebook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	service := note.NewService(note.NewRepository(db), store)

	notebook, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Project"})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}

	created, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(notebook.ID),
		Title:      "Notebook note",
		Content:    "Content",
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if created.NotebookID == nil || *created.NotebookID != notebook.ID {
		t.Fatalf("created notebook id = %v", created.NotebookID)
	}

	updated, err := service.Update(ctx, created.ID, note.UpdateInput{
		ClearNotebook: ptr(true),
	})
	if err != nil {
		t.Fatalf("clear notebook: %v", err)
	}
	if updated.NotebookID != nil {
		t.Fatalf("expected notebook id to be cleared, got %v", *updated.NotebookID)
	}
}
