package ai

import (
	"errors"
	"testing"
)

func TestServiceRejectsInvalidAIRecordsWithoutPersistence(t *testing.T) {
	service, db := newV3Service(t, &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"})

	if _, err := service.SaveHistory(t.Context(), SaveAIHistoryInput{
		Kind:       AssistantKindQA,
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Messages: []AIConversationMessage{
			{Role: "assistant", Content: "invalid first role"},
			{Role: "assistant", Content: "invalid second role"},
		},
	}); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("invalid history error = %v, want ErrInputInvalid", err)
	}
	if _, err := service.SaveArtifact(t.Context(), SaveAIArtifactInput{
		Kind:       ArtifactKindDocument,
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
	}); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("invalid artifact error = %v, want ErrInputInvalid", err)
	}

	for _, table := range []struct {
		name  string
		query string
	}{
		{name: "ai_histories", query: "SELECT COUNT(*) FROM ai_histories"},
		{name: "ai_artifacts", query: "SELECT COUNT(*) FROM ai_artifacts"},
	} {
		var count int
		if err := db.QueryRowContext(t.Context(), table.query).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table.name, err)
		}
		if count != 0 {
			t.Fatalf("invalid input persisted %d rows in %s", count, table.name)
		}
	}
}
