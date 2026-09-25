package note_test

import (
	"strings"
	"testing"
	"time"

	"atlasnote/internal/note"
)

func TestRepositorySearchIndexesMarkdownAndPaginates(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	first := createSearchTestNote(t, repository, "search-first", "Go notes", time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC), false)
	second := createSearchTestNote(t, repository, "search-second", "SQLite", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), false)
	trashed := createSearchTestNote(t, repository, "search-trashed", "Archived", time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC), true)

	for _, document := range []note.SearchDocument{
		{NoteID: first.ID, Title: first.Title, Body: "全文検索の設計と保存キュー", Revision: first.Revision, ContentHash: "hash-first"},
		{NoteID: second.ID, Title: second.Title, Body: "全文検索とFTS5", Revision: second.Revision, ContentHash: "hash-second"},
		{NoteID: trashed.ID, Title: trashed.Title, Body: "全文検索のアーカイブ", Revision: trashed.Revision, ContentHash: "hash-trashed"},
	} {
		if err := repository.UpsertSearchIndex(t.Context(), document); err != nil {
			t.Fatalf("upsert search document %q: %v", document.NoteID, err)
		}
	}

	pageOne, err := repository.Search(t.Context(), note.SearchInput{Query: "全文検索", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("search first page: %v", err)
	}
	if pageOne.Error != nil || pageOne.Total != 2 || len(pageOne.Items) != 1 || !pageOne.HasNext {
		t.Fatalf("first search page = %#v", pageOne)
	}
	if pageOne.Items[0].MatchScope != "body" || !strings.Contains(pageOne.Items[0].Snippet, "<mark>") {
		t.Fatalf("first search item = %#v", pageOne.Items[0])
	}

	pageTwo, err := repository.Search(t.Context(), note.SearchInput{Query: "全文検索", Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}
	if pageTwo.Error != nil || pageTwo.Total != 2 || len(pageTwo.Items) != 1 || pageTwo.HasNext {
		t.Fatalf("second search page = %#v", pageTwo)
	}
	if pageOne.Items[0].Note.ID == pageTwo.Items[0].Note.ID {
		t.Fatalf("pagination returned duplicate note %q", pageOne.Items[0].Note.ID)
	}

	withTrash, err := repository.Search(t.Context(), note.SearchInput{
		Query:          "全文検索",
		IncludeTrashed: true,
		PageSize:       10,
	})
	if err != nil {
		t.Fatalf("search including trashed notes: %v", err)
	}
	if withTrash.Error != nil || withTrash.Total != 3 || len(withTrash.Items) != 3 {
		t.Fatalf("search including trashed notes = %#v", withTrash)
	}

	titleOnly, err := repository.Search(t.Context(), note.SearchInput{Query: "Go", Scope: note.SearchScopeTitle})
	if err != nil {
		t.Fatalf("title search: %v", err)
	}
	if titleOnly.Error != nil || titleOnly.Total != 1 || len(titleOnly.Items) != 1 {
		t.Fatalf("title search = %#v", titleOnly)
	}
	if titleOnly.Items[0].Note.ID != first.ID || titleOnly.Items[0].MatchScope != note.SearchScopeTitle || titleOnly.Items[0].Snippet != "" {
		t.Fatalf("title search item = %#v", titleOnly.Items[0])
	}

	sorted, err := repository.Search(t.Context(), note.SearchInput{
		Query:         "全文検索",
		PageSize:      10,
		SortBy:        note.NoteSortByCreatedAt,
		SortDirection: note.NoteSortDirectionAsc,
	})
	if err != nil {
		t.Fatalf("sorted search: %v", err)
	}
	if sorted.Error != nil || len(sorted.Items) != 2 || sorted.Items[0].Note.ID != second.ID || sorted.Items[1].Note.ID != first.ID {
		t.Fatalf("sorted search result = %#v", sorted)
	}
}

func TestRepositorySearchEvidenceUsesAllTerms(t *testing.T) {
	repository := newRepositoryTest(t)
	for _, fixture := range []struct{ id, title, body string }{
		{"split", "alpha", "前置き beta 後続"},
		{"title", "alpha beta", "別の本文"},
		{"body", "別題", "alpha beta 本文"},
		{"short", "短", "長い前置き beta 後続"},
	} {
		record := createSearchTestNote(t, repository, fixture.id, fixture.title, time.Now().UTC(), false)
		if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{NoteID: record.ID, Title: fixture.title, Body: fixture.body, Revision: record.Revision}); err != nil {
			t.Fatal(err)
		}
	}
	for _, testCase := range []struct{ query, id, scope, snippet string }{
		{"alpha beta", "split", "both", "前置き <mark>beta</mark> 後続"},
		{"alpha beta", "title", "title", ""},
		{"alpha beta", "body", "body", "<mark>alpha</mark> <mark>beta</mark> 本文"},
		{"短 beta", "short", "both", "長い前置き beta 後続"},
	} {
		result, err := repository.Search(t.Context(), note.SearchInput{Query: testCase.query})
		if err != nil || result.Error != nil {
			t.Fatalf("query %q: %#v, %v", testCase.query, result, err)
		}
		found := false
		for _, item := range result.Items {
			if item.Note.ID != testCase.id {
				continue
			}
			found = true
			if item.MatchScope != testCase.scope || item.Snippet != testCase.snippet {
				t.Fatalf("query %q item = %#v", testCase.query, item)
			}
		}
		if !found {
			t.Fatalf("query %q missing %s: %#v", testCase.query, testCase.id, result)
		}
	}
}

func TestRepositorySearchIndexStateStoresContentMTime(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	createSearchTestNote(t, repository, "search-mtime", "mtime", time.Now().UTC(), false)
	contentMTime := time.Now().UTC().Truncate(time.Nanosecond)
	if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{
		NoteID:       "search-mtime",
		Title:        "mtime",
		Body:         "benchmark",
		Revision:     1,
		ContentHash:  "hash",
		ContentMTime: contentMTime,
	}); err != nil {
		t.Fatalf("upsert search index: %v", err)
	}

	state, found, err := repository.GetSearchIndexState(t.Context(), "search-mtime")
	if err != nil {
		t.Fatalf("get search index state: %v", err)
	}
	if !found || state.ContentMTimeUnix != contentMTime.UnixNano() {
		t.Fatalf("search state mtime = %#v, found=%t", state, found)
	}
}

func TestRepositorySearchValidationAndShortQueryFallback(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	noteRecord := createSearchTestNote(t, repository, "search-validation", "A_B%", time.Now().UTC(), false)
	if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{
		NoteID:   noteRecord.ID,
		Title:    noteRecord.Title,
		Body:     "短い検索語と100%の確認",
		Revision: noteRecord.Revision,
	}); err != nil {
		t.Fatalf("upsert validation search document: %v", err)
	}

	cases := []struct {
		name  string
		input note.SearchInput
		code  string
	}{
		{name: "query too long", input: note.SearchInput{Query: strings.Repeat("あ", note.MaxSearchQueryLength+1)}, code: note.SearchErrorQueryTooLong},
		{name: "control character", input: note.SearchInput{Query: "ok\x00"}, code: note.SearchErrorQueryInvalid},
		{name: "scope", input: note.SearchInput{Query: "abc", Scope: "body"}, code: note.SearchErrorScopeInvalid},
		{name: "page", input: note.SearchInput{Query: "abc", Page: -1}, code: note.SearchErrorPageInvalid},
		{name: "page size", input: note.SearchInput{Query: "abc", PageSize: note.MaxSearchPageSize + 1}, code: note.SearchErrorPageSizeInvalid},
		{name: "sort field", input: note.SearchInput{Query: "abc", SortBy: "updated_at", SortDirection: note.NoteSortDirectionDesc}, code: note.SearchErrorSortByInvalid},
		{name: "sort direction", input: note.SearchInput{Query: "abc", SortBy: note.NoteSortByTitle, SortDirection: "desc; DROP TABLE notes"}, code: note.SearchErrorSortDirectionInvalid},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := repository.Search(t.Context(), testCase.input)
			if err != nil {
				t.Fatalf("search returned Go error: %v", err)
			}
			if result.Error == nil || result.Error.Code != testCase.code {
				t.Fatalf("search error = %#v, want code %s", result.Error, testCase.code)
			}
		})
	}

	shortQuery, err := repository.Search(t.Context(), note.SearchInput{Query: "検索"})
	if err != nil {
		t.Fatalf("short query fallback search: %v", err)
	}
	if shortQuery.Error != nil || shortQuery.Total != 1 || len(shortQuery.Items) != 1 {
		t.Fatalf("short query fallback result = %#v", shortQuery)
	}
	if shortQuery.Items[0].MatchScope != "body" || !strings.Contains(shortQuery.Items[0].Snippet, "検索") {
		t.Fatalf("short query evidence = %#v", shortQuery.Items[0])
	}
	oneCharacter, err := repository.Search(t.Context(), note.SearchInput{Query: "検"})
	if err != nil || oneCharacter.Total != 1 || oneCharacter.Items[0].MatchScope != "body" || !strings.Contains(oneCharacter.Items[0].Snippet, "検") {
		t.Fatalf("one-character search = %#v, %v", oneCharacter, err)
	}

	escapedLike, err := repository.Search(t.Context(), note.SearchInput{Query: "%"})
	if err != nil {
		t.Fatalf("escaped LIKE search: %v", err)
	}
	if escapedLike.Error != nil || escapedLike.Total != 1 {
		t.Fatalf("escaped LIKE result = %#v", escapedLike)
	}
}

func TestRepositorySearchIndexLifecycle(t *testing.T) {
	t.Parallel()

	repository := newRepositoryTest(t)
	first := createSearchTestNote(t, repository, "search-lifecycle-first", "First", time.Now().UTC(), false)
	second := createSearchTestNote(t, repository, "search-lifecycle-second", "Second", time.Now().UTC().Add(time.Second), false)

	if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{
		NoteID: first.ID, Title: first.Title, Body: "old body", Revision: first.Revision,
	}); err != nil {
		t.Fatalf("upsert old search document: %v", err)
	}
	if err := repository.UpsertSearchIndex(t.Context(), note.SearchDocument{
		NoteID: first.ID, Title: first.Title, Body: "new body", Revision: first.Revision,
	}); err != nil {
		t.Fatalf("upsert replacement search document: %v", err)
	}

	oldResult, err := repository.Search(t.Context(), note.SearchInput{Query: "old"})
	if err != nil {
		t.Fatalf("search old document: %v", err)
	}
	newResult, err := repository.Search(t.Context(), note.SearchInput{Query: "new"})
	if err != nil {
		t.Fatalf("search replacement document: %v", err)
	}
	if oldResult.Total != 0 || newResult.Total != 1 {
		t.Fatalf("replacement results: old=%#v new=%#v", oldResult, newResult)
	}

	if err := repository.DeleteSearchIndex(t.Context(), first.ID); err != nil {
		t.Fatalf("delete search document: %v", err)
	}
	deletedResult, err := repository.Search(t.Context(), note.SearchInput{Query: "new"})
	if err != nil {
		t.Fatalf("search deleted document: %v", err)
	}
	if deletedResult.Total != 0 {
		t.Fatalf("deleted document search result = %#v", deletedResult)
	}

	if err := repository.ReplaceSearchIndex(t.Context(), []note.SearchDocument{{
		NoteID: second.ID, Title: second.Title, Body: "rebuilt body", Revision: second.Revision,
	}}); err != nil {
		t.Fatalf("rebuild search index: %v", err)
	}
	rebuiltResult, err := repository.Search(t.Context(), note.SearchInput{Query: "rebuilt"})
	if err != nil {
		t.Fatalf("search rebuilt document: %v", err)
	}
	if rebuiltResult.Total != 1 || rebuiltResult.Items[0].Note.ID != second.ID {
		t.Fatalf("rebuilt search result = %#v", rebuiltResult)
	}
}

func createSearchTestNote(t *testing.T, repository *note.Repository, id, title string, updatedAt time.Time, trashed bool) note.Record {
	t.Helper()

	record := note.Record{
		ID:          id,
		Title:       title,
		ContentPath: id + ".md",
		IsTrashed:   trashed,
		Revision:    1,
		CreatedAt:   updatedAt.Add(-time.Hour),
		UpdatedAt:   updatedAt,
	}
	if err := repository.Create(t.Context(), record); err != nil {
		t.Fatalf("create search test note: %v", err)
	}
	return record
}
