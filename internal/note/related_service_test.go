package note_test

import (
	"fmt"
	"strings"
	"testing"

	"atlasnote/internal/note"
)

func TestRelatedNotesAppliesNotebookScopeBeforeCandidateLimits(t *testing.T) {
	ctx, _, _, service, _ := newRecoveryTestService(t)
	parent, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "対象"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "子", ParentID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	links := make([]string, 0, 88)
	outside := make([]string, 0, 85)
	for i := 0; i < 85; i++ {
		item, err := service.Create(ctx, note.CreateInput{Title: "scope-signal", Content: "scope-signal 本文"})
		if err != nil {
			t.Fatal(err)
		}
		outside = append(outside, item.ID)
		links = append(links, fmt.Sprintf("[out](atlasnote://note/%s)", item.ID))
	}
	direct, err := service.Create(ctx, note.CreateInput{NotebookID: &parent.ID, Title: "scope-signal", Content: "scope-signal 直下"})
	if err != nil {
		t.Fatal(err)
	}
	nested, err := service.Create(ctx, note.CreateInput{NotebookID: &child.ID, Title: "scope-signal", Content: "scope-signal 子孫"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{direct.ID, nested.ID} {
		links = append(links, fmt.Sprintf("[in](atlasnote://note/%s)", id))
	}
	source, err := service.Create(ctx, note.CreateInput{Title: "scope-signal", Content: strings.Join(links, "\n")})
	if err != nil {
		t.Fatal(err)
	}
	firstTag, err := service.CreateTag(ctx, note.TagCreateInput{Name: "scope-a"})
	if err != nil || firstTag.Tag == nil {
		t.Fatal(err)
	}
	secondTag, err := service.CreateTag(ctx, note.TagCreateInput{Name: "scope-b"})
	if err != nil || secondTag.Tag == nil {
		t.Fatal(err)
	}
	tagIDs := []string{firstTag.Tag.ID, secondTag.Tag.ID}
	for _, id := range append(append(outside, direct.ID, nested.ID), source.ID) {
		result, err := service.SetNoteTags(ctx, id, note.SetNoteTagsInput{TagIDs: tagIDs})
		if err != nil || result.Error != nil {
			t.Fatalf("tag %s: %#v, %v", id, result, err)
		}
	}
	for _, testCase := range []struct {
		descendants bool
		want        []string
	}{
		{false, []string{direct.ID}},
		{true, []string{direct.ID, nested.ID}},
	} {
		result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID, NotebookID: &parent.ID, Descendants: testCase.descendants})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Items) != len(testCase.want) {
			t.Fatalf("scope result = %#v", result)
		}
		for _, item := range result.Items {
			if item.NoteID != direct.ID && (!testCase.descendants || item.NoteID != nested.ID) {
				t.Fatalf("out-of-scope candidate: %#v", item)
			}
		}
	}
}

func TestRelatedNotesRanksLinksAndTextAndTracksRevision(t *testing.T) {
	t.Parallel()
	ctx, _, _, service, _ := newRecoveryTestService(t)
	linked, err := service.Create(ctx, note.CreateInput{Title: "別の題名", Content: "関連資料"})
	if err != nil {
		t.Fatal(err)
	}
	textMatch, err := service.Create(ctx, note.CreateInput{Title: "日本語の設計", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Create(ctx, note.CreateInput{
		Title: "日本語の設計", Content: "[link](atlasnote://note/" + linked.ID + ")",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].NoteID != linked.ID || result.Items[1].NoteID != textMatch.ID {
		t.Fatalf("related notes = %#v", result)
	}
	if result.Items[0].Reasons[0] != "このノートからリンク" || result.Items[1].Reasons[0] != "タイトルに共通する語句" {
		t.Fatalf("reasons = %#v", result)
	}
	backlink := "[source](atlasnote://note/" + source.ID + ")"
	linked, err = service.Update(ctx, linked.ID, note.UpdateInput{Content: &backlink, ExpectedRevision: &linked.Revision})
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil || len(result.Items) != 2 || len(result.Items[0].Reasons) < 2 || result.Items[0].Reasons[1] != "このノートへのリンク" {
		t.Fatalf("circular link evidence = %#v, %v", result, err)
	}
	newTitle := "日本語の設計 更新"
	updated, err := service.Update(ctx, textMatch.ID, note.UpdateInput{Title: &newTitle, ExpectedRevision: &textMatch.Revision})
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].NoteID != linked.ID {
		t.Fatalf("limited result = %#v", result)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Items[1].Revision != updated.Revision {
		t.Fatalf("stale revision = %#v", result.Items[1])
	}
}

func TestRelatedNotesRejectsBrokenIndex(t *testing.T) {
	t.Parallel()
	ctx, repo, _, service, _ := newRecoveryTestService(t)
	source, err := service.Create(ctx, note.CreateInput{Title: "日本語", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSearchIndex(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err == nil || !strings.Contains(err.Error(), "inconsistent") || len(result.Items) != 0 {
		t.Fatalf("broken index result = %#v, err=%v", result, err)
	}
}

func TestRelatedNotesNotebookScopeAndTrash(t *testing.T) {
	t.Parallel()
	ctx, _, _, service, _ := newRecoveryTestService(t)
	parent, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "親"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateNotebook(ctx, note.NotebookCreateInput{Name: "子", ParentID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := service.Create(ctx, note.CreateInput{NotebookID: &parent.ID, Title: "日本語の設計", Content: "直接"})
	if err != nil {
		t.Fatal(err)
	}
	nested, err := service.Create(ctx, note.CreateInput{NotebookID: &child.ID, Title: "日本語の設計", Content: "子孫"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Create(ctx, note.CreateInput{Title: "日本語の設計", Content: "元"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID, NotebookID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].NoteID != direct.ID {
		t.Fatalf("direct scope = %#v", result)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID, NotebookID: &parent.ID, Descendants: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("descendant scope = %#v", result)
	}
	trashed := true
	if _, err := service.Update(ctx, nested.ID, note.UpdateInput{IsTrashed: &trashed, ExpectedRevision: &nested.Revision}); err != nil {
		t.Fatal(err)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID, NotebookID: &parent.ID, Descendants: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].NoteID != direct.ID {
		t.Fatalf("trash exclusion = %#v", result)
	}
}

func TestRelatedNotesUsesCurrentTagsWithoutRevisionChange(t *testing.T) {
	t.Parallel()
	ctx, _, _, service, _ := newRecoveryTestService(t)
	source, err := service.Create(ctx, note.CreateInput{Title: "設計", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := service.Create(ctx, note.CreateInput{Title: "別件", Content: "他の本文"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateTag(ctx, note.TagCreateInput{Name: "project"})
	if err != nil || first.Tag == nil {
		t.Fatalf("tag: %#v, %v", first, err)
	}
	second, err := service.CreateTag(ctx, note.TagCreateInput{Name: "research"})
	if err != nil || second.Tag == nil {
		t.Fatalf("tag: %#v, %v", second, err)
	}
	ids := []string{first.Tag.ID, second.Tag.ID}
	if result, err := service.SetNoteTags(ctx, source.ID, note.SetNoteTagsInput{TagIDs: ids}); err != nil || result.Error != nil {
		t.Fatalf("source tags: %#v, %v", result, err)
	}
	if result, err := service.SetNoteTags(ctx, candidate.ID, note.SetNoteTagsInput{TagIDs: ids[:1]}); err != nil || result.Error != nil {
		t.Fatalf("candidate tag: %#v, %v", result, err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil || len(result.Items) != 0 {
		t.Fatalf("single generic tag = %#v, %v", result, err)
	}
	if result, err := service.SetNoteTags(ctx, candidate.ID, note.SetNoteTagsInput{TagIDs: ids}); err != nil || result.Error != nil {
		t.Fatalf("candidate tags: %#v, %v", result, err)
	}
	result, err = service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil || len(result.Items) != 1 || result.Items[0].Revision != candidate.Revision {
		t.Fatalf("current tag result = %#v, %v", result, err)
	}
}

func TestRelatedNotesBreaksEqualScoresByID(t *testing.T) {
	t.Parallel()
	ctx, _, _, service, _ := newRecoveryTestService(t)
	first, err := service.Create(ctx, note.CreateInput{Title: "日本語検索", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, note.CreateInput{Title: "日本語検索", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Create(ctx, note.CreateInput{Title: "日本語検索", Content: "元"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("equal score result = %#v", result)
	}
	wantFirst := first.ID
	if second.ID < wantFirst {
		wantFirst = second.ID
	}
	if result.Items[0].NoteID != wantFirst {
		t.Fatalf("unstable tie order = %#v", result)
	}
}

func TestRelatedNotesOneCharacterTitle(t *testing.T) {
	t.Parallel()
	ctx, _, _, service, _ := newRecoveryTestService(t)
	candidate, err := service.Create(ctx, note.CreateInput{Title: "別題", Content: "短い文字"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Create(ctx, note.CreateInput{Title: "短", Content: "元"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: source.ID})
	if err != nil || len(result.Items) != 1 || result.Items[0].NoteID != candidate.ID {
		t.Fatalf("one-character candidate = %#v, %v", result, err)
	}
}
