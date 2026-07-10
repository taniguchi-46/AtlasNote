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

func TestServiceCreateAndUpdateNotebookIcon(t *testing.T) {
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

	created, err := service.CreateNotebook(ctx, note.NotebookCreateInput{
		Name: "Project",
		Icon: ptr("default:calendar"),
	})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	if created.Icon != "default:calendar" {
		t.Fatalf("created icon = %q", created.Icon)
	}

	updated, err := service.UpdateNotebook(ctx, created.ID, note.NotebookUpdateInput{
		Icon: ptr("default:pen"),
	})
	if err != nil {
		t.Fatalf("update notebook icon: %v", err)
	}
	if updated.Icon != "default:pen" {
		t.Fatalf("updated icon = %q", updated.Icon)
	}

	withoutIcon, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Default"})
	if err != nil {
		t.Fatalf("create notebook without icon: %v", err)
	}
	if withoutIcon.Icon != "default:note" {
		t.Fatalf("default icon = %q", withoutIcon.Icon)
	}
}

func TestServiceDeleteNotebookWithNotesTrashedDeletesChildNotebooks(t *testing.T) {
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

	parent, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Parent"})
	if err != nil {
		t.Fatalf("create parent notebook: %v", err)
	}
	child, err := service.CreateNotebook(ctx, note.NotebookCreateInput{
		Name:     "Child",
		ParentID: ptr(parent.ID),
	})
	if err != nil {
		t.Fatalf("create child notebook: %v", err)
	}

	parentNote, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(parent.ID),
		Title:      "Parent note",
		Content:    "Parent content",
	})
	if err != nil {
		t.Fatalf("create parent note: %v", err)
	}
	childNote, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(child.ID),
		Title:      "Child note",
		Content:    "Child content",
	})
	if err != nil {
		t.Fatalf("create child note: %v", err)
	}

	err = service.DeleteNotebook(ctx, parent.ID, note.NotebookDeleteInput{
		Mode: note.NotebookDeleteModeTrashNotes,
	})
	if err != nil {
		t.Fatalf("delete notebook with notes trashed: %v", err)
	}

	notebooks, err := service.ListNotebooks(ctx)
	if err != nil {
		t.Fatalf("list notebooks: %v", err)
	}
	if len(notebooks) != 0 {
		t.Fatalf("expected parent and child notebooks to be deleted, got %d", len(notebooks))
	}

	summaries, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	summaryByID := map[string]note.Summary{}
	for _, summary := range summaries {
		summaryByID[summary.ID] = summary
	}
	for _, id := range []string{parentNote.ID, childNote.ID} {
		summary, ok := summaryByID[id]
		if !ok {
			t.Fatalf("expected note %s to remain in trash", id)
		}
		if !summary.IsTrashed {
			t.Fatalf("expected note %s to be trashed", id)
		}
		if summary.NotebookID != nil {
			t.Fatalf("expected trashed note %s notebook id to be cleared, got %v", id, *summary.NotebookID)
		}
	}
}

func TestServiceDeleteNotebookKeepingNotesKeepsChildNotebooks(t *testing.T) {
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

	parent, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Parent"})
	if err != nil {
		t.Fatalf("create parent notebook: %v", err)
	}
	child, err := service.CreateNotebook(ctx, note.NotebookCreateInput{
		Name:     "Child",
		ParentID: ptr(parent.ID),
	})
	if err != nil {
		t.Fatalf("create child notebook: %v", err)
	}

	parentNote, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(parent.ID),
		Title:      "Parent note",
		Content:    "Parent content",
	})
	if err != nil {
		t.Fatalf("create parent note: %v", err)
	}
	childNote, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(child.ID),
		Title:      "Child note",
		Content:    "Child content",
	})
	if err != nil {
		t.Fatalf("create child note: %v", err)
	}

	err = service.DeleteNotebook(ctx, parent.ID, note.NotebookDeleteInput{
		Mode: note.NotebookDeleteModeKeepNotes,
	})
	if err != nil {
		t.Fatalf("delete notebook keeping notes: %v", err)
	}

	notebooks, err := service.ListNotebooks(ctx)
	if err != nil {
		t.Fatalf("list notebooks: %v", err)
	}
	if len(notebooks) != 1 {
		t.Fatalf("expected child notebook to remain, got %d notebooks", len(notebooks))
	}
	if notebooks[0].ID != child.ID {
		t.Fatalf("remaining notebook id = %q", notebooks[0].ID)
	}
	if notebooks[0].ParentID != nil {
		t.Fatalf("expected child notebook parent to be cleared, got %v", *notebooks[0].ParentID)
	}

	summaries, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	summaryByID := map[string]note.Summary{}
	for _, summary := range summaries {
		summaryByID[summary.ID] = summary
	}

	parentSummary := summaryByID[parentNote.ID]
	if parentSummary.IsTrashed {
		t.Fatal("expected parent note to remain active")
	}
	if parentSummary.NotebookID != nil {
		t.Fatalf("expected parent note notebook id to be cleared, got %v", *parentSummary.NotebookID)
	}

	childSummary := summaryByID[childNote.ID]
	if childSummary.IsTrashed {
		t.Fatal("expected child note to remain active")
	}
	if childSummary.NotebookID == nil || *childSummary.NotebookID != child.ID {
		t.Fatalf("expected child note to keep child notebook id, got %v", childSummary.NotebookID)
	}
}
