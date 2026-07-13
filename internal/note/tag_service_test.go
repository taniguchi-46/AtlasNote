package note_test

import (
	"context"
	"strings"
	"testing"

	"atlasnote/internal/note"
)

func TestServiceTagNormalizesNamesAndRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	createdResult, err := service.CreateTag(ctx, note.TagCreateInput{Name: "\u00a0Project\u2003Plan\u00a0"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if createdResult.Error != nil || createdResult.Tag == nil {
		t.Fatalf("create result = %#v", createdResult)
	}
	if createdResult.Tag.Name != "Project Plan" {
		t.Fatalf("normalized tag name = %q", createdResult.Tag.Name)
	}

	duplicateResult, err := service.CreateTag(ctx, note.TagCreateInput{Name: "project plan"})
	if err != nil {
		t.Fatalf("create duplicate tag: %v", err)
	}
	assertTagErrorCode(t, duplicateResult.Error, note.TagErrorNameConflict)

	composedResult, err := service.CreateTag(ctx, note.TagCreateInput{Name: "Café"})
	if err != nil {
		t.Fatalf("create composed tag: %v", err)
	}
	if composedResult.Error != nil || composedResult.Tag == nil {
		t.Fatalf("composed create result = %#v", composedResult)
	}
	decomposedResult, err := service.CreateTag(ctx, note.TagCreateInput{Name: "cafe\u0301"})
	if err != nil {
		t.Fatalf("create decomposed duplicate tag: %v", err)
	}
	assertTagErrorCode(t, decomposedResult.Error, note.TagErrorNameConflict)

	updatedResult, err := service.UpdateTag(ctx, createdResult.Tag.ID, note.TagUpdateInput{Name: "PROJECT PLAN"})
	if err != nil {
		t.Fatalf("rename tag using same normalized name: %v", err)
	}
	if updatedResult.Error != nil || updatedResult.Tag == nil || updatedResult.Tag.Name != "PROJECT PLAN" {
		t.Fatalf("case-only rename result = %#v", updatedResult)
	}
}

func TestServiceTagValidatesNameBoundsAndControls(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	for _, testCase := range []struct {
		name  string
		input string
		code  string
	}{
		{name: "empty", input: "\u00a0\u2003", code: note.TagErrorNameEmpty},
		{name: "control", input: "bad\tname", code: note.TagErrorNameInvalid},
		{name: "nul", input: "bad\x00name", code: note.TagErrorNameInvalid},
		{name: "too long", input: strings.Repeat("a", note.MaxTagNameLength+1), code: note.TagErrorNameTooLong},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := service.CreateTag(ctx, note.TagCreateInput{Name: testCase.input})
			if err != nil {
				t.Fatalf("create invalid tag: %v", err)
			}
			assertTagErrorCode(t, result.Error, testCase.code)
		})
	}

	result, err := service.CreateTag(ctx, note.TagCreateInput{Name: strings.Repeat("a", note.MaxTagNameLength)})
	if err != nil {
		t.Fatalf("create maximum length tag: %v", err)
	}
	if result.Error != nil || result.Tag == nil {
		t.Fatalf("maximum length result = %#v", result)
	}
}

func TestServiceSetNoteTagsIsAtomicAndDoesNotChangeNoteRevision(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	createdNote, err := service.Create(ctx, note.CreateInput{Title: "Tagged", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	alpha := createTagForTest(t, ctx, service, "Alpha")
	beta := createTagForTest(t, ctx, service, "Beta")

	setResult, err := service.SetNoteTags(ctx, createdNote.ID, note.SetNoteTagsInput{
		TagIDs: []string{beta.ID, alpha.ID, beta.ID},
	})
	if err != nil {
		t.Fatalf("set note tags: %v", err)
	}
	if setResult.Error != nil || len(setResult.Tags) != 2 ||
		setResult.Tags[0].ID != alpha.ID || setResult.Tags[1].ID != beta.ID {
		t.Fatalf("set result = %#v", setResult)
	}

	afterSet, err := service.Get(ctx, createdNote.ID)
	if err != nil {
		t.Fatalf("get note after tag set: %v", err)
	}
	if afterSet.Revision != createdNote.Revision || !afterSet.UpdatedAt.Equal(createdNote.UpdatedAt) {
		t.Fatalf("tag set changed note metadata: before=%#v after=%#v", createdNote, afterSet)
	}

	missingResult, err := service.SetNoteTags(ctx, createdNote.ID, note.SetNoteTagsInput{
		TagIDs: []string{alpha.ID, "missing-tag"},
	})
	if err != nil {
		t.Fatalf("set note tags with missing tag: %v", err)
	}
	assertTagErrorCode(t, missingResult.Error, note.TagErrorNotFound)

	unchangedResult, err := service.ListNoteTags(ctx, createdNote.ID)
	if err != nil {
		t.Fatalf("list note tags after rejected replacement: %v", err)
	}
	if unchangedResult.Error != nil || len(unchangedResult.Tags) != 2 {
		t.Fatalf("tags changed after rejected replacement = %#v", unchangedResult)
	}

	trashed, err := service.Update(ctx, createdNote.ID, note.UpdateInput{
		IsTrashed:        ptr(true),
		ExpectedRevision: ptr(afterSet.Revision),
	})
	if err != nil {
		t.Fatalf("trash tagged note: %v", err)
	}
	if !trashed.IsTrashed {
		t.Fatal("tagged note was not moved to trash")
	}

	restored, err := service.Update(ctx, createdNote.ID, note.UpdateInput{
		IsTrashed:        ptr(false),
		ExpectedRevision: ptr(trashed.Revision),
	})
	if err != nil {
		t.Fatalf("restore tagged note: %v", err)
	}
	if restored.IsTrashed {
		t.Fatal("tagged note was not restored")
	}

	afterRestore, err := service.ListNoteTags(ctx, createdNote.ID)
	if err != nil {
		t.Fatalf("list tags after trash restore: %v", err)
	}
	if afterRestore.Error != nil || len(afterRestore.Tags) != 2 {
		t.Fatalf("tags changed by trash restore = %#v", afterRestore)
	}

	clearedResult, err := service.SetNoteTags(ctx, createdNote.ID, note.SetNoteTagsInput{})
	if err != nil {
		t.Fatalf("clear note tags: %v", err)
	}
	if clearedResult.Error != nil || len(clearedResult.Tags) != 0 {
		t.Fatalf("clear result = %#v", clearedResult)
	}
}

func TestServiceDeleteTagDetachesItWithoutDeletingNote(t *testing.T) {
	t.Parallel()

	ctx, _, _, service, _ := newRecoveryTestService(t)
	createdNote, err := service.Create(ctx, note.CreateInput{Title: "Tagged", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	tag := createTagForTest(t, ctx, service, "Delete me")
	if _, err := service.SetNoteTags(ctx, createdNote.ID, note.SetNoteTagsInput{TagIDs: []string{tag.ID}}); err != nil {
		t.Fatalf("attach tag: %v", err)
	}

	deletedResult, err := service.DeleteTag(ctx, tag.ID)
	if err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	if !deletedResult.Deleted || deletedResult.Error != nil {
		t.Fatalf("delete result = %#v", deletedResult)
	}

	tagsResult, err := service.ListNoteTags(ctx, createdNote.ID)
	if err != nil {
		t.Fatalf("list note tags after tag delete: %v", err)
	}
	if tagsResult.Error != nil || len(tagsResult.Tags) != 0 {
		t.Fatalf("tags after delete = %#v", tagsResult)
	}
	if _, err := service.Get(ctx, createdNote.ID); err != nil {
		t.Fatalf("tag delete removed note: %v", err)
	}
}

func createTagForTest(t *testing.T, ctx context.Context, service *note.Service, name string) note.Tag {
	t.Helper()

	result, err := service.CreateTag(ctx, note.TagCreateInput{Name: name})
	if err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	if result.Error != nil || result.Tag == nil {
		t.Fatalf("create tag %q result = %#v", name, result)
	}
	return *result.Tag
}

func assertTagErrorCode(t *testing.T, tagError *note.TagError, want string) {
	t.Helper()

	if tagError == nil || tagError.Code != want {
		t.Fatalf("tag error = %#v, want code %q", tagError, want)
	}
}
