package ai

import (
	"strings"
	"testing"
)

func TestServicePreparesContextSourcesFromApprovedNotes(t *testing.T) {
	service, _ := newV3Service(t, &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"})

	sources, err := service.PrepareContext(t.Context(), AIContextInput{
		NoteIDs:          []string{"note-1"},
		SearchQuery:      "body",
		IncludeBacklinks: true,
	})
	if err != nil {
		t.Fatalf("prepare context: %v", err)
	}
	want := []AIContextSource{
		{NoteID: "note-1", Title: "Current", Revision: 4, CharacterCount: len("current body"), ContentByte: len("current body"), TotalContentByte: len("current body")},
		{NoteID: "note-2", Title: "Search result", Revision: 2, CharacterCount: len("search body"), ContentByte: len("search body"), TotalContentByte: len("search body")},
		{NoteID: "note-3", Title: "Backlink", Revision: 1, CharacterCount: len("backlink body"), ContentByte: len("backlink body"), TotalContentByte: len("backlink body")},
	}
	if len(sources) != len(want) {
		t.Fatalf("context source count = %d, want %d", len(sources), len(want))
	}
	for index := range want {
		if sources[index] != want[index] {
			t.Fatalf("context source %d = %#v, want %#v", index, sources[index], want[index])
		}
	}
}

func TestServicePreservesFullNoteMetricsWhenContextBodyIsTruncated(t *testing.T) {
	content := strings.Repeat("あ", aiContextNoteBytes/3) + "😀"
	service, _ := newV3Service(t, &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"})
	service.SetNoteContextProvider(testContextProvider{notes: map[string]ContextNote{
		"long-note": {NoteID: "long-note", Title: "長いノート", Content: content, Revision: 9},
	}})

	sources, err := service.PrepareContext(t.Context(), AIContextInput{NoteIDs: []string{"long-note"}})
	if err != nil {
		t.Fatalf("prepare long context: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %#v", sources)
	}
	source := sources[0]
	if source.CharacterCount != utf16CharacterCount(content) {
		t.Fatalf("character count = %d, want %d", source.CharacterCount, utf16CharacterCount(content))
	}
	if source.TotalContentByte != len([]byte(content)) {
		t.Fatalf("total content bytes = %d, want %d", source.TotalContentByte, len([]byte(content)))
	}
	if source.ContentByte > aiContextNoteBytes || !source.ContentTruncated {
		t.Fatalf("truncation metadata = %#v", source)
	}
}
