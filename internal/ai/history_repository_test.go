package ai

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"atlasnote/internal/database"
)

func TestRepositorySavesAndDeletesAIHistoryAndArtifactWithoutCascadingNoteSources(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(t.Context(), `
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES ('note-1', 'Note 1', 'note-1.md', 0, 0, 0, 2, '2026-07-28T00:00:00Z', '2026-07-28T00:00:00Z')
`); err != nil {
		t.Fatalf("insert note fixture: %v", err)
	}

	repository := NewRepository(db)
	history, err := repository.saveHistory(t.Context(), SaveAIHistoryInput{
		ID:         "history-1",
		Kind:       AssistantKindQA,
		Title:      "Saved question",
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Messages: []AIConversationMessage{
			{Role: "user", Content: "Question"},
			{Role: "assistant", Content: "Answer"},
		},
		Sources: []AIHistorySource{{NoteID: "note-1", InputRevision: 2}},
	})
	if err != nil {
		t.Fatalf("save history: %v", err)
	}
	if history.Status != AIRecordStatusSaved || len(history.Messages) != 2 || len(history.Sources) != 1 {
		t.Fatalf("saved history = %#v", history)
	}

	artifact, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
		ID:         "artifact-1",
		Kind:       ArtifactKindDocument,
		Title:      "Saved document",
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Content:    "Final document",
		Sources:    []AIHistorySource{{NoteID: "note-1", InputRevision: 2}},
	})
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if artifact.Status != AIRecordStatusSaved || artifact.Content != "Final document" || len(artifact.Sources) != 1 {
		t.Fatalf("saved artifact = %#v", artifact)
	}
	summary, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
		ID:         "summary-1",
		Kind:       ArtifactKindSummary,
		Title:      "Note 1の要約",
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Content:    "## 概要\nSummary text",
		Sources:    []AIHistorySource{{NoteID: "note-1", InputRevision: 2}},
	})
	if err != nil {
		t.Fatalf("save summary artifact: %v", err)
	}
	if summary.Kind != ArtifactKindSummary || summary.Content == "" {
		t.Fatalf("saved summary artifact = %#v", summary)
	}

	if _, err := db.ExecContext(t.Context(), "UPDATE notes SET revision = 3 WHERE id = 'note-1'"); err != nil {
		t.Fatalf("update note revision: %v", err)
	}
	staleHistory, err := repository.getHistory(t.Context(), "history-1")
	if err != nil {
		t.Fatalf("get stale history: %v", err)
	}
	if staleHistory.Status != AIRecordStatusStale {
		t.Fatalf("stale history status = %q, want %q", staleHistory.Status, AIRecordStatusStale)
	}

	if _, err := db.ExecContext(t.Context(), "DELETE FROM notes WHERE id = 'note-1'"); err != nil {
		t.Fatalf("delete source note: %v", err)
	}
	orphanedArtifact, err := repository.getArtifact(t.Context(), "artifact-1")
	if err != nil {
		t.Fatalf("get orphaned artifact: %v", err)
	}
	if orphanedArtifact.Status != AIRecordStatusOrphaned {
		t.Fatalf("orphaned artifact status = %q, want %q", orphanedArtifact.Status, AIRecordStatusOrphaned)
	}

	if err := repository.deleteHistory(t.Context(), "history-1"); err != nil {
		t.Fatalf("delete history: %v", err)
	}
	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_history_messages WHERE history_id = 'history-1'").Scan(&count); err != nil {
		t.Fatalf("count deleted history messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("history messages after delete = %d, want 0", count)
	}
	if err := repository.deleteArtifactsByKinds(t.Context(), []ArtifactKind{ArtifactKindDocument}); err != nil {
		t.Fatalf("delete writing artifacts: %v", err)
	}
	if _, err := repository.getArtifact(t.Context(), "artifact-1"); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("get deleted writing artifact error = %v, want ErrArtifactNotFound", err)
	}
	if preservedSummary, err := repository.getArtifact(t.Context(), "summary-1"); err != nil || preservedSummary.Kind != ArtifactKindSummary {
		t.Fatalf("summary after writing delete = %#v, %v", preservedSummary, err)
	}
}

func TestRepositoryListsAllAIHistories(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repository := NewRepository(db)
	for index := 0; index < 101; index++ {
		if _, err := repository.saveHistory(t.Context(), SaveAIHistoryInput{
			ID:         fmt.Sprintf("history-%03d", index),
			Kind:       AssistantKindQA,
			Title:      fmt.Sprintf("History %03d", index),
			ProviderID: ProviderOpenRouter,
			ModelID:    "openai/test",
			Messages: []AIConversationMessage{
				{Role: "user", Content: "Question"},
				{Role: "assistant", Content: "Answer"},
			},
		}); err != nil {
			t.Fatalf("save history %d: %v", index, err)
		}
	}

	histories, err := repository.listHistories(t.Context())
	if err != nil {
		t.Fatalf("list histories: %v", err)
	}
	if len(histories) != 101 {
		t.Fatalf("listed histories = %d, want 101", len(histories))
	}
}

func TestLegacyAIRecordReadsPreserveHistoryArtifactAndSummaryCounts(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(db)
	if _, err := repository.saveHistory(t.Context(), SaveAIHistoryInput{
		ID: "history-1", Kind: AssistantKindQA, Title: "Legacy chat",
		ProviderID: ProviderOpenRouter, ModelID: "openai/test",
		Messages: []AIConversationMessage{{Role: "user", Content: "Question"}, {Role: "assistant", Content: "Answer"}},
	}); err != nil {
		t.Fatalf("save history: %v", err)
	}
	for _, fixture := range []struct {
		id   string
		kind ArtifactKind
	}{
		{"artifact-1", ArtifactKindDocument}, {"summary-1", ArtifactKindSummary},
	} {
		if _, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
			ID: fixture.id, Kind: fixture.kind, Title: fixture.id,
			ProviderID: ProviderOpenRouter, ModelID: "openai/test", Content: "Legacy content",
		}); err != nil {
			t.Fatalf("save artifact %s: %v", fixture.id, err)
		}
	}
	count := func(table string) int {
		t.Helper()
		var value int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&value); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return value
	}
	beforeHistories, beforeArtifacts := count("ai_histories"), count("ai_artifacts")
	if items, err := repository.listHistories(t.Context()); err != nil || len(items) != 1 {
		t.Fatalf("list histories = %d, %v", len(items), err)
	}
	if item, err := repository.getHistory(t.Context(), "history-1"); err != nil || len(item.Messages) != 2 {
		t.Fatalf("get history = %#v, %v", item, err)
	}
	if items, err := repository.listArtifacts(t.Context()); err != nil || len(items) != 2 {
		t.Fatalf("list artifacts = %d, %v", len(items), err)
	}
	if item, err := repository.getArtifact(t.Context(), "summary-1"); err != nil || item.Kind != ArtifactKindSummary || item.Content != "Legacy content" {
		t.Fatalf("get summary = %#v, %v", item, err)
	}
	if after := count("ai_histories"); after != beforeHistories {
		t.Fatalf("history count changed: %d -> %d", beforeHistories, after)
	}
	if after := count("ai_artifacts"); after != beforeArtifacts {
		t.Fatalf("artifact count changed: %d -> %d", beforeArtifacts, after)
	}
}

func TestListArtifactsPageBoundariesAndSummaryReachability(t *testing.T) {
	for _, count := range []int{0, 100, 101} {
		t.Run(fmt.Sprintf("writing-%d", count), func(t *testing.T) {
			db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repository := NewRepository(db)
			for index := 0; index < count; index++ {
				if _, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
					ID: fmt.Sprintf("artifact-%03d", index), Kind: ArtifactKindDocument,
					Title: "Legacy artifact", ProviderID: ProviderOpenRouter, ModelID: "openai/test", Content: "content",
				}); err != nil {
					t.Fatal(err)
				}
			}
			first, hasNext, err := repository.listArtifactsPage(t.Context(), "writing", 0)
			if err != nil {
				t.Fatal(err)
			}
			wantFirst := count
			if wantFirst > aiRecordListLimit {
				wantFirst = aiRecordListLimit
			}
			if len(first) != wantFirst || hasNext != (count > aiRecordListLimit) {
				t.Fatalf("first page: items=%d hasNext=%t, want %d/%t", len(first), hasNext, wantFirst, count > aiRecordListLimit)
			}
			if count == 101 {
				second, next, err := repository.listArtifactsPage(t.Context(), "writing", len(first))
				if err != nil || len(second) != 1 || next {
					t.Fatalf("second page: items=%d hasNext=%t err=%v", len(second), next, err)
				}
				seen := map[string]bool{}
				for _, item := range append(first, second...) {
					if seen[item.ID] {
						t.Fatalf("duplicate item %s", item.ID)
					}
					seen[item.ID] = true
				}
				if len(seen) != count {
					t.Fatalf("missing items: got %d, want %d", len(seen), count)
				}
			}
		})
	}

	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)
	for index := 0; index < 150; index++ {
		kind := ArtifactKindDocument
		if index < 10 {
			kind = ArtifactKindSummary
		}
		if _, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
			ID: fmt.Sprintf("record-%03d", index), Kind: kind,
			Title: "Legacy record", ProviderID: ProviderOpenRouter, ModelID: "openai/test", Content: "content",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE ai_artifacts SET updated_at = CASE WHEN kind = 'summary' THEN '2020-01-01T00:00:00Z' ELSE '2026-01-01T00:00:00Z' END`); err != nil {
		t.Fatal(err)
	}
	// The older summaries are absent from the original combined first 100.
	legacy, err := repository.listArtifacts(t.Context())
	if err != nil || len(legacy) != 100 {
		t.Fatalf("legacy list: items=%d err=%v", len(legacy), err)
	}
	for _, item := range legacy {
		if item.Kind == ArtifactKindSummary {
			t.Fatal("fixture summary appeared in the first 100")
		}
	}
	seen := map[string]bool{}
	for _, kind := range []string{"writing", "summary"} {
		offset := 0
		var previous *AIArtifact
		for {
			page, hasNext, err := repository.listArtifactsPage(t.Context(), kind, offset)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page {
				if previous != nil && (previous.UpdatedAt.Before(item.UpdatedAt) || (previous.UpdatedAt.Equal(item.UpdatedAt) && previous.ID > item.ID)) {
					t.Fatalf("unstable artifact order: %s before %s", previous.ID, item.ID)
				}
				current := item
				previous = &current
				if seen[item.ID] {
					t.Fatalf("duplicate item %s", item.ID)
				}
				seen[item.ID] = true
				if kind == "summary" && item.Kind != ArtifactKindSummary {
					t.Fatalf("wrong summary kind %s", item.Kind)
				}
				if kind == "writing" && item.Kind == ArtifactKindSummary {
					t.Fatal("summary in writing page")
				}
			}
			offset += len(page)
			if !hasNext {
				break
			}
		}
	}
	if len(seen) != 150 {
		t.Fatalf("missing items: got %d, want 150", len(seen))
	}
	for index := 150; index < 241; index++ {
		if _, err := repository.saveArtifact(t.Context(), SaveAIArtifactInput{
			ID: fmt.Sprintf("record-%03d", index), Kind: ArtifactKindSummary,
			Title: "Later summary", ProviderID: ProviderOpenRouter, ModelID: "openai/test", Content: "content",
		}); err != nil {
			t.Fatal(err)
		}
	}
	firstSummary, hasNext, err := repository.listArtifactsPage(t.Context(), "summary", 0)
	if err != nil || len(firstSummary) != 100 || !hasNext {
		t.Fatalf("first summary page: items=%d hasNext=%t err=%v", len(firstSummary), hasNext, err)
	}
	lastSummary, hasNext, err := repository.listArtifactsPage(t.Context(), "summary", len(firstSummary))
	if err != nil || len(lastSummary) != 1 || hasNext {
		t.Fatalf("second summary page: items=%d hasNext=%t err=%v", len(lastSummary), hasNext, err)
	}
	seenSummaries := map[string]bool{}
	for _, item := range append(firstSummary, lastSummary...) {
		if seenSummaries[item.ID] {
			t.Fatalf("duplicate summary %s", item.ID)
		}
		seenSummaries[item.ID] = true
	}
	if len(seenSummaries) != 101 {
		t.Fatalf("missing summaries: got %d, want 101", len(seenSummaries))
	}
}
