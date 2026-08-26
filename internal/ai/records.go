package ai

import (
	"context"
	"strings"
	"unicode/utf8"
)

const aiMaxSavedArtifactBytes = 128 * 1024

func (s *Service) SaveHistory(ctx context.Context, input SaveAIHistoryInput) (AIHistory, error) {
	releaseContent := s.beginAIRecordAccess(ctx)
	defer releaseContent()
	normalized, err := normalizeSaveHistoryInput(input)
	if err != nil {
		return AIHistory{}, err
	}
	if err := s.assertAIAllowedSources(ctx, normalized.Sources); err != nil {
		return AIHistory{}, err
	}
	if normalized.ID == "" {
		normalized.ID, err = newCredentialReference()
		if err != nil {
			return AIHistory{}, ErrProviderUnavailable
		}
	}
	return s.repository.saveHistory(ctx, normalized)
}

func (s *Service) ListHistories(ctx context.Context) ([]AIHistory, error) {
	return s.repository.listHistories(ctx)
}

func (s *Service) GetHistory(ctx context.Context, id string) (AIHistory, error) {
	if strings.TrimSpace(id) == "" {
		return AIHistory{}, ErrHistoryNotFound
	}
	return s.repository.getHistory(ctx, strings.TrimSpace(id))
}

func (s *Service) DeleteHistory(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrHistoryNotFound
	}
	return s.repository.deleteHistory(ctx, strings.TrimSpace(id))
}

func (s *Service) DeleteAllHistories(ctx context.Context) error {
	return s.repository.deleteAllHistories(ctx)
}

func (s *Service) SaveArtifact(ctx context.Context, input SaveAIArtifactInput) (AIArtifact, error) {
	releaseContent := s.beginAIRecordAccess(ctx)
	defer releaseContent()
	normalized, err := normalizeSaveArtifactInput(input)
	if err != nil {
		return AIArtifact{}, err
	}
	if err := s.assertAIAllowedSources(ctx, normalized.Sources); err != nil {
		return AIArtifact{}, err
	}
	if normalized.ID == "" {
		normalized.ID, err = newCredentialReference()
		if err != nil {
			return AIArtifact{}, ErrProviderUnavailable
		}
	}
	return s.repository.saveArtifact(ctx, normalized)
}

func (s *Service) ListArtifacts(ctx context.Context) ([]AIArtifact, error) {
	return s.repository.listArtifacts(ctx)
}

func (s *Service) GetArtifact(ctx context.Context, id string) (AIArtifact, error) {
	if strings.TrimSpace(id) == "" {
		return AIArtifact{}, ErrArtifactNotFound
	}
	return s.repository.getArtifact(ctx, strings.TrimSpace(id))
}

func (s *Service) DeleteArtifact(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrArtifactNotFound
	}
	return s.repository.deleteArtifact(ctx, strings.TrimSpace(id))
}

func (s *Service) DeleteAllArtifacts(ctx context.Context) error {
	return s.repository.deleteAllArtifacts(ctx)
}

func (s *Service) DeleteAllWritingArtifacts(ctx context.Context) error {
	return s.repository.deleteArtifactsByKinds(ctx, []ArtifactKind{
		ArtifactKindPrompt,
		ArtifactKindPromptImprovement,
		ArtifactKindREADME,
		ArtifactKindDocument,
		ArtifactKindBlog,
		ArtifactKindRequirements,
	})
}

func normalizeArtifactKind(kind ArtifactKind) (ArtifactKind, error) {
	switch kind {
	case ArtifactKindSummary,
		ArtifactKindPrompt,
		ArtifactKindPromptImprovement,
		ArtifactKindREADME,
		ArtifactKindDocument,
		ArtifactKindBlog,
		ArtifactKindRequirements:
		return kind, nil
	default:
		return "", ErrInputInvalid
	}
}

func normalizeSaveHistoryInput(input SaveAIHistoryInput) (SaveAIHistoryInput, error) {
	kind, err := normalizeAssistantKind(input.Kind)
	if err != nil {
		return SaveAIHistoryInput{}, err
	}
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return SaveAIHistoryInput{}, err
	}
	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return SaveAIHistoryInput{}, err
	}
	messages, err := normalizeConversationMessages(input.Messages)
	if err != nil {
		return SaveAIHistoryInput{}, err
	}
	if len(messages) < 2 || len(messages)%2 != 0 {
		return SaveAIHistoryInput{}, ErrInputInvalid
	}
	for index, message := range messages {
		wantRole := "user"
		if index%2 == 1 {
			wantRole = "assistant"
		}
		if message.Role != wantRole {
			return SaveAIHistoryInput{}, ErrInputInvalid
		}
	}
	sources, err := normalizeSources(input.Sources)
	if err != nil {
		return SaveAIHistoryInput{}, err
	}
	return SaveAIHistoryInput{
		ID:         strings.TrimSpace(input.ID),
		Kind:       kind,
		Title:      normalizeRecordTitle(input.Title),
		ProviderID: providerID,
		ModelID:    modelID,
		Messages:   messages,
		Sources:    sources,
	}, nil
}

func normalizeSaveArtifactInput(input SaveAIArtifactInput) (SaveAIArtifactInput, error) {
	kind, err := normalizeArtifactKind(input.Kind)
	if err != nil {
		return SaveAIArtifactInput{}, err
	}
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return SaveAIArtifactInput{}, err
	}
	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return SaveAIArtifactInput{}, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" || !utf8.ValidString(content) {
		return SaveAIArtifactInput{}, ErrInputInvalid
	}
	if len([]byte(content)) > aiMaxSavedArtifactBytes {
		return SaveAIArtifactInput{}, ErrInputTooLarge
	}
	sources, err := normalizeSources(input.Sources)
	if err != nil {
		return SaveAIArtifactInput{}, err
	}
	return SaveAIArtifactInput{
		ID:         strings.TrimSpace(input.ID),
		Kind:       kind,
		Title:      normalizeRecordTitle(input.Title),
		ProviderID: providerID,
		ModelID:    modelID,
		Content:    content,
		Sources:    sources,
	}, nil
}
