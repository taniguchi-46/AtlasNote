package note

import (
	"encoding/json"
	"testing"
)

func TestSearchModelsUseStableJSONContract(t *testing.T) {
	notebookID := "notebook-1"
	input := SearchInput{
		Query:          "全文検索",
		Scope:          SearchScopeAll,
		NotebookID:     &notebookID,
		IncludeTrashed: false,
		Page:           2,
		PageSize:       30,
	}
	result := SearchResult{
		Items: []SearchItem{{
			Note:       Summary{ID: "note-1", Title: "検索対象"},
			Snippet:    "本文の抜粋",
			MatchScope: "body",
		}},
		Page:     2,
		PageSize: 30,
		Total:    31,
		HasNext:  true,
		Error: &SearchError{
			Code:      SearchErrorIndexNotReady,
			Message:   "検索索引を準備しています",
			Retryable: true,
		},
	}

	for name, value := range map[string]any{"input": input, "result": result} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		if len(decoded) == 0 {
			t.Fatalf("%s JSON is empty", name)
		}
	}
}

func TestSearchValidationConstantsMatchContract(t *testing.T) {
	if DefaultSearchPage != 1 || DefaultSearchPageSize != 30 {
		t.Fatalf("unexpected search defaults: page=%d pageSize=%d", DefaultSearchPage, DefaultSearchPageSize)
	}
	if MaxSearchPageSize != 100 || MaxSearchQueryLength != 200 {
		t.Fatalf("unexpected search limits: pageSize=%d queryLength=%d", MaxSearchPageSize, MaxSearchQueryLength)
	}
	for _, code := range []string{
		SearchErrorQueryTooLong,
		SearchErrorQueryInvalid,
		SearchErrorScopeInvalid,
		SearchErrorPageInvalid,
		SearchErrorPageSizeInvalid,
		SearchErrorIndexNotReady,
		SearchErrorIndexInconsistent,
		SearchErrorIndexFailed,
	} {
		if code == "" {
			t.Fatal("search error code must not be empty")
		}
	}
}
