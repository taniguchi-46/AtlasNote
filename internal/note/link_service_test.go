package note_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func TestServiceMaintainsBacklinksAcrossNoteLifecycle(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	target, err := service.Create(ctx, note.CreateInput{Title: "Target", Content: "target"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	source, err := service.Create(ctx, note.CreateInput{
		Title:   "Source",
		Content: "[Target](atlasnote://note/" + target.ID + ")",
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	result, err := service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil {
		t.Fatalf("list backlinks: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != source.ID {
		t.Fatalf("initial backlinks = %#v", result)
	}

	circularContent := fmt.Sprintf(
		"[Source](atlasnote://note/%s)\n[Self](atlasnote://note/%s)",
		source.ID,
		target.ID,
	)
	_, err = service.Update(ctx, target.ID, note.UpdateInput{
		Content:          &circularContent,
		ExpectedRevision: &target.Revision,
	})
	if err != nil {
		t.Fatalf("create circular links: %v", err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: source.ID})
	if err != nil || result.Total != 1 || result.Items[0].ID != target.ID {
		t.Fatalf("backlinks for circular source = %#v, err=%v", result, err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil || result.Total != 2 {
		t.Fatalf("backlinks with self link = %#v, err=%v", result, err)
	}

	updatedTarget, err := service.Get(ctx, target.ID)
	if err != nil {
		t.Fatalf("get target after circular link: %v", err)
	}
	plainContent := "target"
	_, err = service.Update(ctx, target.ID, note.UpdateInput{
		Content:          &plainContent,
		ExpectedRevision: &updatedTarget.Revision,
	})
	if err != nil {
		t.Fatalf("remove circular links: %v", err)
	}

	updatedTitle := "Renamed source"
	_, err = service.Update(ctx, source.ID, note.UpdateInput{
		Title:            &updatedTitle,
		ExpectedRevision: &source.Revision,
	})
	if err != nil {
		t.Fatalf("rename source: %v", err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil || result.Total != 1 || result.Items[0].Title != updatedTitle {
		t.Fatalf("backlinks after source rename = %#v, err=%v", result, err)
	}

	updatedContent := "No link"
	currentSource, err := service.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	_, err = service.Update(ctx, source.ID, note.UpdateInput{
		Content:          &updatedContent,
		ExpectedRevision: &currentSource.Revision,
	})
	if err != nil {
		t.Fatalf("remove source link: %v", err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil || result.Total != 0 {
		t.Fatalf("backlinks after content update = %#v, err=%v", result, err)
	}

	currentSource, err = service.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get source after unlink: %v", err)
	}
	linkedContent := "[Target](atlasnote://note/" + target.ID + ")"
	_, err = service.Update(ctx, source.ID, note.UpdateInput{
		Content:          &linkedContent,
		ExpectedRevision: &currentSource.Revision,
	})
	if err != nil {
		t.Fatalf("restore source link: %v", err)
	}

	currentSource, err = service.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get source before trash: %v", err)
	}
	trashed := true
	_, err = service.Update(ctx, source.ID, note.UpdateInput{
		IsTrashed:        &trashed,
		ExpectedRevision: &currentSource.Revision,
	})
	if err != nil {
		t.Fatalf("trash source: %v", err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil || result.Total != 0 {
		t.Fatalf("backlinks after trash = %#v, err=%v", result, err)
	}

	currentSource, err = service.Get(ctx, source.ID)
	if err != nil {
		t.Fatalf("get trashed source: %v", err)
	}
	restored := false
	_, err = service.Update(ctx, source.ID, note.UpdateInput{
		IsTrashed:        &restored,
		ExpectedRevision: &currentSource.Revision,
	})
	if err != nil {
		t.Fatalf("restore source: %v", err)
	}
	result, err = service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID})
	if err != nil || result.Total != 1 {
		t.Fatalf("backlinks after restore = %#v, err=%v", result, err)
	}

	currentTarget, err := service.Get(ctx, target.ID)
	if err != nil {
		t.Fatalf("get target before delete: %v", err)
	}
	if err := service.Delete(ctx, target.ID, note.DeleteInput{ExpectedRevision: currentTarget.Revision}); err != nil {
		t.Fatalf("delete target: %v", err)
	}
	if _, err := service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID}); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("deleted target backlinks error = %v, want ErrNotFound", err)
	}
}

func TestServiceNoteLinkIndexFailureDoesNotRollbackNote(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
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
	repository := note.NewRepository(db)
	service := note.NewService(repository, store)
	target, err := service.Create(ctx, note.CreateInput{Title: "Target", Content: "target"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE note_links"); err != nil {
		t.Fatalf("drop link index fixture: %v", err)
	}

	created, err := service.Create(ctx, note.CreateInput{
		Title:   "Source",
		Content: "[Target](atlasnote://note/" + target.ID + ")",
	})
	if err != nil {
		t.Fatalf("create source with failed link index: %v", err)
	}
	if _, err := repository.Get(ctx, created.ID); err != nil {
		t.Fatalf("note was rolled back after link index failure: %v", err)
	}
	content, err := store.Read(ctx, created.ID)
	if err != nil || content == "" {
		t.Fatalf("markdown after link index failure = %q, %v", content, err)
	}
	if _, err := service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: target.ID}); err == nil {
		t.Fatal("list backlinks succeeded after link index failure")
	}
}

func TestServiceKeepsDanglingNoteLinksInMarkdown(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	danglingID := "abcdefabcdefabcdefabcdefabcdefab"
	content := "[Missing](atlasnote://note/" + danglingID + ")"
	created, err := service.Create(ctx, note.CreateInput{Title: "Dangling", Content: content})
	if err != nil {
		t.Fatalf("create dangling-link note: %v", err)
	}
	loaded, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get dangling-link note: %v", err)
	}
	if loaded.Content != content {
		t.Fatalf("dangling link content = %q, want %q", loaded.Content, content)
	}
	if _, err := service.ListBacklinks(ctx, note.BacklinkListInput{NoteID: danglingID}); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("dangling target backlinks error = %v, want ErrNotFound", err)
	}
}
