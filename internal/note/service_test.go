package note_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestServiceRecoverPendingUpdate(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Before", Content: "before content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	record, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note record: %v", err)
	}
	operation := note.StorageOperation{
		ID:          "pendingupdate",
		NoteID:      created.ID,
		Type:        note.StorageOperationUpsert,
		ContentHash: storage.HashContent("recovered content"),
		CreatedAt:   time.Now().UTC(),
	}
	if err := store.WriteTemp(ctx, created.ID, operation.ID, "recovered content"); err != nil {
		t.Fatalf("write pending markdown: %v", err)
	}
	record.Title = "Recovered"
	record.UpdatedAt = time.Now().UTC()
	if err := repository.UpdateWithStorageOperation(ctx, record, operation); err != nil {
		t.Fatalf("create pending update: %v", err)
	}

	if err := service.Recover(ctx); err != nil {
		t.Fatalf("recover pending update: %v", err)
	}
	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get recovered note: %v", err)
	}
	if got.Title != "Recovered" || got.Content != "recovered content" {
		t.Fatalf("recovered note = title %q, content %q", got.Title, got.Content)
	}
	operations, err := repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list pending operations: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("pending operation count = %d", len(operations))
	}
}

func TestServiceRecoverRejectsTamperedPendingMarkdown(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Before", Content: "before content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	record, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note record: %v", err)
	}
	operation := note.StorageOperation{
		ID:          "tamperedupdate",
		NoteID:      created.ID,
		Type:        note.StorageOperationUpsert,
		ContentHash: storage.HashContent("expected content"),
		CreatedAt:   time.Now().UTC(),
	}
	if err := store.WriteTemp(ctx, created.ID, operation.ID, "tampered content"); err != nil {
		t.Fatalf("write tampered markdown: %v", err)
	}
	record.Title = "Expected"
	if err := repository.UpdateWithStorageOperation(ctx, record, operation); err != nil {
		t.Fatalf("create pending update: %v", err)
	}

	err = service.Recover(ctx)
	if err == nil || !strings.Contains(err.Error(), "temp hash mismatch") {
		t.Fatalf("expected temp hash mismatch, got %v", err)
	}
	content, err := store.Read(ctx, created.ID)
	if err != nil {
		t.Fatalf("read preserved markdown: %v", err)
	}
	if content != "before content" {
		t.Fatalf("existing markdown was overwritten: %q", content)
	}
}

func TestServiceRecoverPendingDelete(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Delete", Content: "delete content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	operation := note.StorageOperation{
		ID:        "pendingdelete",
		NoteID:    created.ID,
		Type:      note.StorageOperationDelete,
		CreatedAt: time.Now().UTC(),
	}
	if err := repository.BeginStorageOperation(ctx, operation); err != nil {
		t.Fatalf("begin pending delete: %v", err)
	}
	if err := store.StageDelete(ctx, created.ID, operation.ID); err != nil {
		t.Fatalf("stage pending delete: %v", err)
	}
	if err := repository.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete pending note record: %v", err)
	}

	if err := service.Recover(ctx); err != nil {
		t.Fatalf("recover pending delete: %v", err)
	}
	if _, err := repository.Get(ctx, created.ID); err != note.ErrNotFound {
		t.Fatalf("expected deleted record, got %v", err)
	}
	staged, err := store.DeleteStagedExists(ctx, created.ID, operation.ID)
	if err != nil {
		t.Fatalf("check staged delete: %v", err)
	}
	if staged {
		t.Fatal("staged delete remains after recovery")
	}
}

func TestServiceDeleteReturnsCommitFailureAndRecoverRetries(t *testing.T) {
	t.Parallel()

	ctx, repository, store, _, _ := newRecoveryTestService(t)
	commitErr := errors.New("remove denied")
	failingStore := &commitDeleteFailingStore{
		MarkdownStore: store,
		err:           commitErr,
	}
	service := note.NewService(repository, failingStore)
	created, err := service.Create(ctx, note.CreateInput{Title: "Delete", Content: "delete content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	err = service.Delete(ctx, created.ID)
	if err == nil || !errors.Is(err, commitErr) {
		t.Fatalf("expected commit delete error, got %v", err)
	}
	if _, err := repository.Get(ctx, created.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("expected deleted record, got %v", err)
	}
	operations, err := repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list pending operations: %v", err)
	}
	if len(operations) != 1 || operations[0].Type != note.StorageOperationDelete {
		t.Fatalf("pending operations = %#v", operations)
	}
	staged, err := store.DeleteStagedExists(ctx, created.ID, operations[0].ID)
	if err != nil {
		t.Fatalf("check staged delete: %v", err)
	}
	if !staged {
		t.Fatal("expected staged markdown to remain after commit failure")
	}

	restartedService := note.NewService(repository, store)
	if err := restartedService.Recover(ctx); err != nil {
		t.Fatalf("recover pending delete: %v", err)
	}
	staged, err = store.DeleteStagedExists(ctx, created.ID, operations[0].ID)
	if err != nil {
		t.Fatalf("check recovered staged delete: %v", err)
	}
	if staged {
		t.Fatal("staged markdown remains after recovery")
	}
	operations, err = repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list recovered operations: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("pending operation count after recovery = %d", len(operations))
	}
}

func TestServiceRecoverRejectsMissingMarkdown(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Missing", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete markdown fixture: %v", err)
	}

	err = service.Recover(ctx)
	if err == nil || !strings.Contains(err.Error(), "markdown content is missing") {
		t.Fatalf("expected missing markdown error, got %v", err)
	}
	if _, err := repository.Get(ctx, created.ID); err != nil {
		t.Fatalf("note record should be preserved: %v", err)
	}
}

func TestServiceRecoverQuarantinesOrphanMarkdown(t *testing.T) {
	t.Parallel()

	ctx, _, store, service, notesDir := newRecoveryTestService(t)
	if err := store.Write(ctx, "orphan", "orphan content"); err != nil {
		t.Fatalf("write orphan markdown: %v", err)
	}

	if err := service.Recover(ctx); err != nil {
		t.Fatalf("recover orphan markdown: %v", err)
	}
	if _, err := os.Stat(filepath.Join(notesDir, "orphan.md")); !os.IsNotExist(err) {
		t.Fatalf("orphan markdown was not moved: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(notesDir, "recovery"))
	if err != nil {
		t.Fatalf("read recovery directory: %v", err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "orphan.md.") {
		t.Fatalf("recovery entries = %v", entries)
	}
}

func TestServiceSerializesConcurrentContentUpdates(t *testing.T) {
	t.Parallel()

	ctx, repository, _, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Initial", Content: "initial"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	type updateResult struct {
		title   string
		content string
		err     error
	}
	results := make(chan updateResult, 2)
	var wait sync.WaitGroup
	for _, values := range [][2]string{{"First", "first content"}, {"Second", "second content"}} {
		values := values
		wait.Add(1)
		go func() {
			defer wait.Done()
			updated, err := service.Update(ctx, created.ID, note.UpdateInput{
				Title:   ptr(values[0]),
				Content: ptr(values[1]),
			})
			results <- updateResult{title: updated.Title, content: updated.Content, err: err}
		}()
	}
	wait.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent update: %v", result.err)
		}
		if (result.title == "First" && result.content != "first content") ||
			(result.title == "Second" && result.content != "second content") {
			t.Fatalf("mismatched update result: title %q, content %q", result.title, result.content)
		}
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get concurrently updated note: %v", err)
	}
	if (got.Title == "First" && got.Content != "first content") ||
		(got.Title == "Second" && got.Content != "second content") {
		t.Fatalf("mismatched persisted note: title %q, content %q", got.Title, got.Content)
	}
	operations, err := repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list storage operations: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("pending operation count = %d", len(operations))
	}
}

func newRecoveryTestService(t *testing.T) (context.Context, *note.Repository, *storage.MarkdownStore, *note.Service, string) {
	t.Helper()

	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	notesDir := filepath.Join(tempDir, "notes")
	store, err := storage.NewMarkdownStore(notesDir)
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	repository := note.NewRepository(db)
	service := note.NewService(repository, store)

	return ctx, repository, store, service, notesDir
}

type commitDeleteFailingStore struct {
	*storage.MarkdownStore
	err error
}

func (s *commitDeleteFailingStore) CommitDelete(context.Context, string, string) error {
	return s.err
}
