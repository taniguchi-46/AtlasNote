package ai

import (
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
		Kind:       WritingKindDocument,
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
	if err := repository.deleteArtifact(t.Context(), "artifact-1"); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}
}
