package ai

import "testing"

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
		{NoteID: "note-1", Title: "Current", Revision: 4, ContentByte: len("current body")},
		{NoteID: "note-2", Title: "Search result", Revision: 2, ContentByte: len("search body")},
		{NoteID: "note-3", Title: "Backlink", Revision: 1, ContentByte: len("backlink body")},
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
