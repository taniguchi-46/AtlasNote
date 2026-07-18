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

func TestSyncExclusiveGateBlocksLocalMutations(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	service := note.NewService(note.NewRepository(db), store)

	syncCtx, unlockSync := service.BeginSyncExclusive(ctx)
	unlocked := false
	defer func() {
		if !unlocked {
			unlockSync()
		}
	}()
	if _, err := service.Create(syncCtx, note.CreateInput{Title: "Remote", Content: "applied by sync"}); err != nil {
		t.Fatalf("sync-owned mutation: %v", err)
	}

	started := make(chan struct{})
	mutationDone := make(chan error, 1)
	go func() {
		close(started)
		_, createErr := service.Create(ctx, note.CreateInput{Title: "Local", Content: "wait for sync"})
		mutationDone <- createErr
	}()
	<-started
	select {
	case mutationErr := <-mutationDone:
		t.Fatalf("local mutation completed while sync gate was held: %v", mutationErr)
	case <-time.After(100 * time.Millisecond):
	}

	unlockSync()
	unlocked = true
	select {
	case mutationErr := <-mutationDone:
		if mutationErr != nil {
			t.Fatalf("local mutation after sync gate release: %v", mutationErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("local mutation did not resume after sync gate release")
	}
}

func TestApplySyncTagRejectsMismatchedNormalizedName(t *testing.T) {
	service := note.NewService(nil, nil)
	err := service.ApplySyncTag(context.Background(), note.SyncTagPayload{
		ID: strings.Repeat("a", 32), Name: "Tag", NormalizedName: "not-the-normalized-name",
	})
	if !errors.Is(err, note.ErrValidation) {
		t.Fatalf("mismatched normalized tag error = %v", err)
	}
}

func TestApplySyncUpdatesPreserveRemoteTimestamps(t *testing.T) {
	ctx, repository, _, service, _ := newRecoveryTestService(t)
	createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 6, time.UTC)
	updatedAt := time.Date(2025, time.February, 3, 4, 5, 6, 7, time.UTC)

	createdNote, err := service.Create(ctx, note.CreateInput{Title: "Local", Content: "local"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := service.ApplySyncNote(ctx, note.SyncNotePayload{
		ID: createdNote.ID, Title: "Remote", Content: "remote",
		CreatedAt: createdAt.Format(time.RFC3339Nano), UpdatedAt: updatedAt.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("apply synced note: %v", err)
	}
	syncedNote, err := service.Get(ctx, createdNote.ID)
	if err != nil {
		t.Fatalf("get synced note: %v", err)
	}
	if !syncedNote.CreatedAt.Equal(createdAt) || !syncedNote.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("synced note timestamps = %s / %s", syncedNote.CreatedAt, syncedNote.UpdatedAt)
	}

	createdNotebook, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Local notebook"})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	if err := service.ApplySyncNotebook(ctx, note.SyncNotebookPayload{
		ID: createdNotebook.ID, Name: "Remote notebook", Icon: createdNotebook.Icon,
		CreatedAt: createdAt.Format(time.RFC3339Nano), UpdatedAt: updatedAt.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("apply synced notebook: %v", err)
	}
	syncedNotebook, err := repository.GetNotebook(ctx, createdNotebook.ID)
	if err != nil {
		t.Fatalf("get synced notebook: %v", err)
	}
	if !syncedNotebook.CreatedAt.Equal(createdAt) || !syncedNotebook.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("synced notebook timestamps = %s / %s", syncedNotebook.CreatedAt, syncedNotebook.UpdatedAt)
	}

	createdTag, err := service.CreateTag(ctx, note.TagCreateInput{Name: "Local tag"})
	if err != nil || createdTag.Tag == nil {
		t.Fatalf("create tag: result=%#v err=%v", createdTag, err)
	}
	if err := service.ApplySyncTag(ctx, note.SyncTagPayload{
		ID: createdTag.Tag.ID, Name: "Remote tag", NormalizedName: "remote tag",
		CreatedAt: createdAt.Format(time.RFC3339Nano), UpdatedAt: updatedAt.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("apply synced tag: %v", err)
	}
	tags, err := service.ListTags(ctx)
	if err != nil {
		t.Fatalf("list synced tags: %v", err)
	}
	for _, tag := range tags {
		if tag.ID == createdTag.Tag.ID {
			if !tag.CreatedAt.Equal(createdAt) || !tag.UpdatedAt.Equal(updatedAt) {
				t.Fatalf("synced tag timestamps = %s / %s", tag.CreatedAt, tag.UpdatedAt)
			}
			return
		}
	}
	t.Fatal("synced tag was not found")
}

func TestApplySyncTagReturnsNameConflictInsteadOfMarkingItApplied(t *testing.T) {
	ctx, _, _, service, _ := newRecoveryTestService(t)
	first, err := service.CreateTag(ctx, note.TagCreateInput{Name: "Alpha"})
	if err != nil || first.Tag == nil {
		t.Fatalf("create first tag: result=%#v err=%v", first, err)
	}
	second, err := service.CreateTag(ctx, note.TagCreateInput{Name: "Beta"})
	if err != nil || second.Tag == nil {
		t.Fatalf("create second tag: result=%#v err=%v", second, err)
	}
	err = service.ApplySyncTag(ctx, note.SyncTagPayload{
		ID: second.Tag.ID, Name: "Alpha", NormalizedName: "alpha",
		CreatedAt: second.Tag.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err == nil {
		t.Fatal("synced tag name conflict was silently accepted")
	}
	tags, listErr := service.ListTags(ctx)
	if listErr != nil {
		t.Fatalf("list tags: %v", listErr)
	}
	for _, tag := range tags {
		if tag.ID == second.Tag.ID && tag.Name != "Beta" {
			t.Fatalf("conflicting synced tag was applied: %#v", tag)
		}
	}
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
	if created.Revision != 1 {
		t.Fatalf("created revision = %d, want 1", created.Revision)
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
	if got.Revision != 1 {
		t.Fatalf("got revision = %d, want 1", got.Revision)
	}

	summaries, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summary count = %d", len(summaries))
	}
	if summaries[0].Revision != 1 {
		t.Fatalf("summary revision = %d, want 1", summaries[0].Revision)
	}

	updated, err := service.Update(ctx, created.ID, note.UpdateInput{
		Title:            ptr("Updated note"),
		Content:          ptr("Updated content"),
		ExpectedRevision: ptr(created.Revision),
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
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}

	if err := service.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: updated.Revision}); err != nil {
		t.Fatalf("delete note: %v", err)
	}

	if _, err := service.Get(ctx, created.ID); err != note.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestServiceRecoverReusesStableIndexAndReadsChangedMarkdown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	countingStore := &countingRecoveryStore{MarkdownStore: store}
	service := note.NewService(note.NewRepository(db), countingStore)
	created, err := service.Create(ctx, note.CreateInput{Title: "Stable", Content: "before"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	countingStore.reads = 0
	if _, err := service.Recover(ctx); err != nil {
		t.Fatalf("recover stable note: %v", err)
	}
	if countingStore.reads != 0 {
		t.Fatalf("stable recovery reads = %d, want 0", countingStore.reads)
	}

	if err := store.Write(ctx, created.ID, "after external edit"); err != nil {
		t.Fatalf("write external markdown: %v", err)
	}
	if _, err := service.Recover(ctx); err != nil {
		t.Fatalf("recover changed note: %v", err)
	}
	if countingStore.reads == 0 {
		t.Fatal("changed recovery did not read markdown")
	}
}

func TestServiceRejectsMissingAndStaleRevisionBeforeStorageMutation(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Original", Content: "original content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	if _, err := service.Update(ctx, created.ID, note.UpdateInput{Content: ptr("missing revision")}); !errors.Is(err, note.ErrValidation) {
		t.Fatalf("missing revision error = %v, want ErrValidation", err)
	}

	staleRevision := created.Revision + 1
	_, err = service.Update(ctx, created.ID, note.UpdateInput{
		Content:          ptr("stale content"),
		ExpectedRevision: ptr(staleRevision),
	})
	if !errors.Is(err, note.ErrRevisionConflict) {
		t.Fatalf("stale update error = %v, want ErrRevisionConflict", err)
	}
	content, err := store.Read(ctx, created.ID)
	if err != nil {
		t.Fatalf("read content after stale update: %v", err)
	}
	if content != "original content" {
		t.Fatalf("content after stale update = %q", content)
	}
	operations, err := repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list storage operations after stale update: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("storage operations after stale update = %#v", operations)
	}

	err = service.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: staleRevision})
	if !errors.Is(err, note.ErrRevisionConflict) {
		t.Fatalf("stale delete error = %v, want ErrRevisionConflict", err)
	}
	exists, err := store.Exists(ctx, created.ID)
	if err != nil {
		t.Fatalf("check content after stale delete: %v", err)
	}
	if !exists {
		t.Fatal("stale delete removed markdown")
	}
	if _, err := repository.Get(ctx, created.ID); err != nil {
		t.Fatalf("stale delete removed note record: %v", err)
	}
	operations, err = repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list storage operations after stale delete: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("storage operations after stale delete = %#v", operations)
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
		ClearNotebook:    ptr(true),
		ExpectedRevision: ptr(created.Revision),
	})
	if err != nil {
		t.Fatalf("clear notebook: %v", err)
	}
	if updated.NotebookID != nil {
		t.Fatalf("expected notebook id to be cleared, got %v", *updated.NotebookID)
	}
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}
}

func TestServiceUpdateNotebookRejectsCyclesAndAllowsMovingToAnotherTree(t *testing.T) {
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
	grandchild, err := service.CreateNotebook(ctx, note.NotebookCreateInput{
		Name:     "Grandchild",
		ParentID: ptr(child.ID),
	})
	if err != nil {
		t.Fatalf("create grandchild notebook: %v", err)
	}
	otherRoot, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Other root"})
	if err != nil {
		t.Fatalf("create other root notebook: %v", err)
	}

	for name, candidateParentID := range map[string]string{
		"self":       parent.ID,
		"child":      child.ID,
		"grandchild": grandchild.ID,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.UpdateNotebook(ctx, parent.ID, note.NotebookUpdateInput{
				ParentID: ptr(candidateParentID),
			})
			if !errors.Is(err, note.ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}

			unchanged, getErr := service.ListNotebooks(ctx)
			if getErr != nil {
				t.Fatalf("list notebooks after rejected move: %v", getErr)
			}
			for _, notebook := range unchanged {
				if notebook.ID == parent.ID && notebook.ParentID != nil {
					t.Fatalf("parent changed after rejected move: %v", *notebook.ParentID)
				}
			}
		})
	}

	moved, err := service.UpdateNotebook(ctx, parent.ID, note.NotebookUpdateInput{
		ParentID: ptr(otherRoot.ID),
	})
	if err != nil {
		t.Fatalf("move notebook to another tree: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != otherRoot.ID {
		t.Fatalf("moved parent id = %v", moved.ParentID)
	}

	moved, err = service.UpdateNotebook(ctx, parent.ID, note.NotebookUpdateInput{
		ClearParent: ptr(true),
	})
	if err != nil {
		t.Fatalf("move notebook to root: %v", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("expected moved notebook parent to be cleared, got %v", *moved.ParentID)
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
	grandchild, err := service.CreateNotebook(ctx, note.NotebookCreateInput{
		Name:     "Grandchild",
		ParentID: ptr(child.ID),
	})
	if err != nil {
		t.Fatalf("create grandchild notebook: %v", err)
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
	grandchildNote, err := service.Create(ctx, note.CreateInput{
		NotebookID: ptr(grandchild.ID),
		Title:      "Grandchild note",
		Content:    "Grandchild content",
	})
	if err != nil {
		t.Fatalf("create grandchild note: %v", err)
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
	for _, id := range []string{parentNote.ID, childNote.ID, grandchildNote.ID} {
		summary, ok := summaryByID[id]
		if !ok {
			t.Fatalf("expected note %s to remain in trash", id)
		}
		if !summary.IsTrashed {
			t.Fatalf("expected note %s to be trashed", id)
		}
		if summary.Revision != 2 {
			t.Fatalf("trashed note %s revision = %d, want 2", id, summary.Revision)
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
	if parentSummary.Revision != 2 {
		t.Fatalf("detached parent note revision = %d, want 2", parentSummary.Revision)
	}

	childSummary := summaryByID[childNote.ID]
	if childSummary.IsTrashed {
		t.Fatal("expected child note to remain active")
	}
	if childSummary.NotebookID == nil || *childSummary.NotebookID != child.ID {
		t.Fatalf("expected child note to keep child notebook id, got %v", childSummary.NotebookID)
	}
	if childSummary.Revision != 1 {
		t.Fatalf("unchanged child note revision = %d, want 1", childSummary.Revision)
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

	if _, err := service.Recover(ctx); err != nil {
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

	_, err = service.Recover(ctx)
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

	if _, err := service.Recover(ctx); err != nil {
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

func TestServiceUpdateCASRestoresPreviousRevisionWhenMarkdownCommitFails(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, notesDir := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Original", Content: "original content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	commitErr := errors.New("rename denied")
	failingService := note.NewService(repository, &commitTempFailingStore{
		MarkdownStore: store,
		err:           commitErr,
	})

	_, err = failingService.Update(ctx, created.ID, note.UpdateInput{
		Title:            ptr("Must roll back"),
		Content:          ptr("must roll back"),
		ExpectedRevision: ptr(created.Revision),
	})
	if err == nil || !errors.Is(err, commitErr) {
		t.Fatalf("update error = %v, want commit error", err)
	}

	record, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get rolled back note record: %v", err)
	}
	if record.Title != "Original" || record.Revision != created.Revision {
		t.Fatalf("rolled back record = %#v", record)
	}
	content, err := store.Read(ctx, created.ID)
	if err != nil {
		t.Fatalf("read rolled back markdown: %v", err)
	}
	if content != "original content" {
		t.Fatalf("rolled back markdown = %q", content)
	}
	operations, err := repository.ListStorageOperations(ctx)
	if err != nil {
		t.Fatalf("list operations after rollback: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("operations after rollback = %#v", operations)
	}
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		t.Fatalf("read notes directory after rollback: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary markdown remains after rollback: %s", entry.Name())
		}
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

	err = service.Delete(ctx, created.ID, note.DeleteInput{ExpectedRevision: created.Revision})
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
	if _, err := restartedService.Recover(ctx); err != nil {
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

func TestServiceRecoverReportsMissingMarkdownAndKeepsHealthyNotesAvailable(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	missing, err := service.Create(ctx, note.CreateInput{Title: "Missing", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	healthy, err := service.Create(ctx, note.CreateInput{Title: "Healthy", Content: "healthy content"})
	if err != nil {
		t.Fatalf("create healthy note: %v", err)
	}
	healthySecond, err := service.Create(ctx, note.CreateInput{Title: "Healthy 2", Content: "second healthy content"})
	if err != nil {
		t.Fatalf("create second healthy note: %v", err)
	}
	if err := store.Delete(ctx, missing.ID); err != nil {
		t.Fatalf("delete markdown fixture: %v", err)
	}

	report, err := service.Recover(ctx)
	if err != nil {
		t.Fatalf("recover with missing markdown: %v", err)
	}
	if len(report.MissingNotes) != 1 || report.MissingNotes[0].ID != missing.ID {
		t.Fatalf("missing notes = %#v", report.MissingNotes)
	}
	if _, err := repository.Get(ctx, missing.ID); err != nil {
		t.Fatalf("note record should be preserved: %v", err)
	}
	got, err := service.Get(ctx, healthy.ID)
	if err != nil || got.Content != "healthy content" {
		t.Fatalf("get healthy note after recovery = %#v, %v", got, err)
	}
	got, err = service.Get(ctx, healthySecond.ID)
	if err != nil || got.Content != "second healthy content" {
		t.Fatalf("get second healthy note after recovery = %#v, %v", got, err)
	}
}

func TestServiceRecoverClearsMissingDiagnosticAfterRestore(t *testing.T) {
	t.Parallel()

	ctx, _, store, service, _ := newRecoveryTestService(t)
	created, err := service.Create(ctx, note.CreateInput{Title: "Restore", Content: "original"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete markdown fixture: %v", err)
	}
	report, err := service.Recover(ctx)
	if err != nil || len(report.MissingNotes) != 1 {
		t.Fatalf("initial recovery report = %#v, %v", report, err)
	}

	if err := store.Write(ctx, created.ID, "restored content"); err != nil {
		t.Fatalf("restore markdown: %v", err)
	}
	report, err = service.Recover(ctx)
	if err != nil {
		t.Fatalf("recover restored markdown: %v", err)
	}
	if len(report.MissingNotes) != 0 {
		t.Fatalf("missing notes after restore = %#v", report.MissingNotes)
	}
	got, err := service.Get(ctx, created.ID)
	if err != nil || got.Content != "restored content" {
		t.Fatalf("restored note = %#v, %v", got, err)
	}
}

func TestServiceDeleteMissingRequiresContentToRemainMissing(t *testing.T) {
	t.Parallel()

	ctx, repository, store, service, _ := newRecoveryTestService(t)
	missing, err := service.Create(ctx, note.CreateInput{Title: "Missing", Content: "content"})
	if err != nil {
		t.Fatalf("create missing note: %v", err)
	}
	healthy, err := service.Create(ctx, note.CreateInput{Title: "Healthy", Content: "healthy"})
	if err != nil {
		t.Fatalf("create healthy note: %v", err)
	}
	if err := store.Delete(ctx, missing.ID); err != nil {
		t.Fatalf("delete markdown fixture: %v", err)
	}
	if err := service.DeleteMissing(ctx, missing.ID); err != nil {
		t.Fatalf("delete missing note: %v", err)
	}
	if _, err := repository.Get(ctx, missing.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("missing note record remains: %v", err)
	}
	if _, err := service.Get(ctx, healthy.ID); err != nil {
		t.Fatalf("healthy note unavailable: %v", err)
	}

	if err := service.DeleteMissing(ctx, healthy.ID); !errors.Is(err, note.ErrContentAvailable) {
		t.Fatalf("expected available-content rejection, got %v", err)
	}
	if _, err := repository.Get(ctx, healthy.ID); err != nil {
		t.Fatalf("healthy note record was deleted: %v", err)
	}
}

func TestServiceRecoverQuarantinesOrphanMarkdown(t *testing.T) {
	t.Parallel()

	ctx, _, store, service, notesDir := newRecoveryTestService(t)
	if err := store.Write(ctx, "orphan", "orphan content"); err != nil {
		t.Fatalf("write orphan markdown: %v", err)
	}

	if _, err := service.Recover(ctx); err != nil {
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

func TestServiceConcurrentContentUpdatesAllowOneRevisionWriter(t *testing.T) {
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
				Title:            ptr(values[0]),
				Content:          ptr(values[1]),
				ExpectedRevision: ptr(created.Revision),
			})
			results <- updateResult{title: updated.Title, content: updated.Content, err: err}
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for result := range results {
		if result.err == nil {
			successes++
			if (result.title == "First" && result.content != "first content") ||
				(result.title == "Second" && result.content != "second content") {
				t.Fatalf("mismatched update result: title %q, content %q", result.title, result.content)
			}
			continue
		}
		if errors.Is(result.err, note.ErrRevisionConflict) {
			conflicts++
			continue
		}
		t.Fatalf("concurrent update: %v", result.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results: successes=%d conflicts=%d", successes, conflicts)
	}

	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get concurrently updated note: %v", err)
	}
	if (got.Title == "First" && got.Content != "first content") ||
		(got.Title == "Second" && got.Content != "second content") {
		t.Fatalf("mismatched persisted note: title %q, content %q", got.Title, got.Content)
	}
	if got.Revision != 2 {
		t.Fatalf("persisted revision = %d, want 2", got.Revision)
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

type commitTempFailingStore struct {
	*storage.MarkdownStore
	err error
}

type countingRecoveryStore struct {
	*storage.MarkdownStore
	reads int
}

func (s *countingRecoveryStore) Read(ctx context.Context, id string) (string, error) {
	s.reads++
	return s.MarkdownStore.Read(ctx, id)
}

func (s *commitTempFailingStore) CommitTemp(context.Context, string, string) error {
	return s.err
}

func (s *commitDeleteFailingStore) CommitDelete(context.Context, string, string) error {
	return s.err
}
