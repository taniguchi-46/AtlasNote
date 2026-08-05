package ai

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	aiMaxContextSources       = 10
	aiMaxExplicitNoteIDs      = 10
	aiMaxQuestionBytes        = 8 * 1024
	aiMaxInstructionBytes     = 12 * 1024
	aiMaxSavedArtifactBytes   = 128 * 1024
	aiContextBytes            = 48 * 1024
	aiContextNoteBytes        = 16 * 1024
	aiMaxSearchQueryBytes     = 2 * 1024
	aiMaxHistoryMessages      = textMaxMessages
	aiAssistantOutputTokens   = 2048
	aiWritingOutputTokens     = 4096
	agentProposalMessageBytes = 8 * 1024
	agentProposalReasonBytes  = 1024
	agentProposalHunkBytes    = 16 * 1024
)

func (s *Service) PrepareContext(ctx context.Context, input AIContextInput) ([]AIContextSource, error) {
	normalized, err := normalizeContextInput(input)
	if err != nil {
		return nil, err
	}
	contextNotes, err := s.collectContext(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return contextSources(contextNotes), nil
}

func (s *Service) RunAssistant(ctx context.Context, input AssistantInput) (AssistantResult, error) {
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return AssistantResult{}, err
	}
	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return AssistantResult{}, err
	}
	kind, err := normalizeAssistantKind(input.Kind)
	if err != nil {
		return AssistantResult{}, err
	}
	mode, err := normalizeChatMode(input.Mode)
	if err != nil {
		return AssistantResult{}, err
	}
	if input.WebSearch && providerID != ProviderOpenRouter {
		return AssistantResult{}, ErrModelCapabilityUnavailable
	}
	question := strings.TrimSpace(input.Question)
	if question == "" || !utf8.ValidString(question) || len([]byte(question)) > aiMaxQuestionBytes {
		return AssistantResult{}, ErrInputInvalid
	}
	messages, err := normalizeConversationMessages(input.Messages)
	if err != nil {
		return AssistantResult{}, err
	}
	if len(messages) == 0 || messages[len(messages)-1].Role != "user" || messages[len(messages)-1].Content != question {
		messages = append(messages, AIConversationMessage{Role: "user", Content: question})
	}
	if len(messages) > aiMaxHistoryMessages {
		return AssistantResult{}, ErrInputTooLarge
	}

	contextNotes, err := s.collectContext(ctx, AIContextInput{
		NoteIDs:          input.NoteIDs,
		SearchQuery:      input.SearchQuery,
		IncludeBacklinks: input.IncludeBacklinks,
	})
	if err != nil {
		return AssistantResult{}, err
	}
	if kind == AssistantKindQA && len(contextNotes) == 0 {
		return AssistantResult{}, ErrInputInvalid
	}
	if err := validateExpectedSources(input.ExpectedSources, contextNotes); err != nil {
		return AssistantResult{}, err
	}
	if mode == ChatModeAgent && !input.WebSearch {
		target, err := normalizeAgentEditTarget(input.AgentTarget)
		if err != nil {
			return AssistantResult{}, err
		}
		normalizedContext, err := normalizeContextInput(AIContextInput{NoteIDs: input.NoteIDs})
		if err != nil {
			return AssistantResult{}, err
		}
		if len(normalizedContext.NoteIDs) == 0 || normalizedContext.NoteIDs[0] != target.NoteID {
			return AssistantResult{}, ErrInputInvalid
		}
		snapshot, err := s.getAgentEditSnapshot(ctx, target, contextNotes)
		if err != nil {
			return AssistantResult{}, err
		}
		return s.runAgentEditProposal(ctx, providerID, modelID, kind, mode, messages, contextNotes, target, snapshot)
	}

	if !s.tryStartGeneration() {
		return AssistantResult{}, ErrBusy
	}
	defer s.finishGeneration()

	apiKey, err := s.credentialForSummary(ctx, providerID, modelID)
	if err != nil {
		return AssistantResult{}, err
	}
	adapter, ok := s.adapter.(TextGenerationProviderAdapter)
	if !ok {
		return AssistantResult{}, ErrProviderUnavailable
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()

	providerMessages := make([]TextMessage, 0, len(messages)+1)
	if contextMessage := buildContextMessage(contextNotes); contextMessage != "" {
		providerMessages = append(providerMessages, TextMessage{Role: "user", Content: contextMessage})
	}
	for _, message := range messages {
		providerMessages = append(providerMessages, TextMessage{Role: message.Role, Content: message.Content})
	}
	result, err := adapter.GenerateText(operationCtx, providerID, apiKey, TextGenerationInput{
		ModelID:           modelID,
		SystemInstruction: buildAssistantInstruction(kind, mode, input.WebSearch),
		Messages:          providerMessages,
		MaxOutputTokens:   aiAssistantOutputTokens,
		WebSearch:         input.WebSearch,
	})
	if err != nil {
		return AssistantResult{}, toSafeError(err)
	}
	if input.WebSearch && result.WebSearchRequests != 1 {
		return AssistantResult{}, ErrInvalidResponse
	}
	answer := strings.TrimSpace(result.Text)
	if answer == "" {
		return AssistantResult{}, ErrInvalidResponse
	}
	messages = append(messages, AIConversationMessage{Role: "assistant", Content: answer})
	return AssistantResult{
		ProviderID:        providerID,
		ModelID:           modelID,
		Kind:              kind,
		Mode:              mode,
		Messages:          messages,
		Sources:           contextSources(contextNotes),
		Citations:         result.Citations,
		WebSearchRequests: result.WebSearchRequests,
	}, nil
}

func (s *Service) runAgentEditProposal(
	ctx context.Context,
	providerID ProviderID,
	modelID string,
	kind AssistantKind,
	mode ChatMode,
	messages []AIConversationMessage,
	contextNotes []ContextNote,
	target AgentEditTarget,
	snapshot ContextNote,
) (AssistantResult, error) {
	adapter, ok := s.adapter.(StructuredStreamingProviderAdapter)
	if !ok {
		return AssistantResult{}, ErrProviderUnavailable
	}
	if err := s.validateStructuredModel(providerID, modelID); err != nil {
		return AssistantResult{}, err
	}
	prompt, schema, err := buildAgentEditPrompt(messages, contextNotes, target)
	if err != nil {
		return AssistantResult{}, err
	}
	if !s.tryStartGeneration() {
		return AssistantResult{}, ErrBusy
	}
	defer s.finishGeneration()

	apiKey, err := s.credentialForSummary(ctx, providerID, modelID)
	if err != nil {
		return AssistantResult{}, err
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	raw, err := adapter.GenerateStructured(operationCtx, providerID, apiKey, StructuredGenerationInput{
		Name:            "atlas_note_agent_edit",
		ModelID:         modelID,
		Prompt:          prompt,
		Schema:          schema,
		MaxOutputTokens: aiAssistantOutputTokens,
	}, nil)
	if err != nil {
		return AssistantResult{}, toSafeError(err)
	}
	message, proposal, err := normalizeAgentEditResponse(raw, snapshot)
	if err != nil {
		return AssistantResult{}, err
	}
	messages = append(messages, AIConversationMessage{Role: "assistant", Content: message})
	return AssistantResult{
		ProviderID: providerID,
		ModelID:    modelID,
		Kind:       kind,
		Mode:       mode,
		Messages:   messages,
		Sources:    contextSources(contextNotes),
		Proposal:   proposal,
	}, nil
}

func (s *Service) getAgentEditSnapshot(ctx context.Context, target AgentEditTarget, contextNotes []ContextNote) (ContextNote, error) {
	for _, item := range contextNotes {
		if item.NoteID == target.NoteID && item.Revision != target.BaseRevision {
			return ContextNote{}, ErrContextChanged
		}
	}
	provider := s.getContextProvider()
	if provider == nil {
		return ContextNote{}, ErrProviderUnavailable
	}
	snapshot, err := provider.Get(ctx, target.NoteID)
	if err != nil || snapshot.IsTrashed {
		return ContextNote{}, ErrContextChanged
	}
	if snapshot.NoteID != target.NoteID || snapshot.Revision != target.BaseRevision || !utf8.ValidString(snapshot.Content) {
		return ContextNote{}, ErrContextChanged
	}
	return snapshot, nil
}

type agentEditResponse struct {
	Message     *string `json:"message"`
	HasProposal *bool   `json:"hasProposal"`
	Reason      *string `json:"reason"`
	Before      *string `json:"before"`
	After       *string `json:"after"`
}

func normalizeAgentEditTarget(input *AgentEditTarget) (AgentEditTarget, error) {
	if input == nil {
		return AgentEditTarget{}, ErrInputInvalid
	}
	noteID := strings.TrimSpace(input.NoteID)
	if noteID == "" || input.BaseRevision < 1 {
		return AgentEditTarget{}, ErrInputInvalid
	}
	return AgentEditTarget{NoteID: noteID, BaseRevision: input.BaseRevision}, nil
}

func normalizeAgentEditResponse(raw string, snapshot ContextNote) (string, *AgentEditProposal, error) {
	var response agentEditResponse
	if err := decodeStrictJSON([]byte(raw), &response); err != nil {
		return "", nil, ErrInvalidResponse
	}
	if response.Message == nil || response.HasProposal == nil || response.Reason == nil || response.Before == nil || response.After == nil {
		return "", nil, ErrInvalidResponse
	}
	message := strings.TrimSpace(*response.Message)
	if message == "" || !utf8.ValidString(message) || len([]byte(message)) > agentProposalMessageBytes {
		return "", nil, ErrInvalidResponse
	}
	if !*response.HasProposal {
		if *response.Reason != "" || *response.Before != "" || *response.After != "" {
			return "", nil, ErrInvalidResponse
		}
		return message, nil, nil
	}
	if strings.TrimSpace(*response.Reason) == "" || *response.Before == "" || !utf8.ValidString(*response.Reason) || !utf8.ValidString(*response.Before) || !utf8.ValidString(*response.After) {
		return "", nil, ErrInvalidResponse
	}
	if len([]byte(*response.Reason)) > agentProposalReasonBytes || len([]byte(*response.Before)) > agentProposalHunkBytes || len([]byte(*response.After)) > agentProposalHunkBytes || *response.Before == *response.After {
		return "", nil, ErrInvalidResponse
	}
	first := strings.Index(snapshot.Content, *response.Before)
	if first < 0 || strings.Index(snapshot.Content[first+1:], *response.Before) >= 0 {
		return "", nil, ErrInvalidResponse
	}
	return message, &AgentEditProposal{
		TargetNoteID:   snapshot.NoteID,
		TargetTitle:    snapshot.Title,
		BaseRevision:   snapshot.Revision,
		Reason:         strings.TrimSpace(*response.Reason),
		Before:         *response.Before,
		After:          *response.After,
		AffectedFields: []string{"content"},
	}, nil
}

func (s *Service) RunWriting(ctx context.Context, input WritingInput) (WritingResult, error) {
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return WritingResult{}, err
	}
	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return WritingResult{}, err
	}
	kind, err := normalizeWritingKind(input.Kind)
	if err != nil {
		return WritingResult{}, err
	}
	instruction := strings.TrimSpace(input.Instruction)
	if instruction == "" || !utf8.ValidString(instruction) || len([]byte(instruction)) > aiMaxInstructionBytes {
		return WritingResult{}, ErrInputInvalid
	}
	contextNotes, err := s.collectContext(ctx, AIContextInput{
		NoteIDs:          input.NoteIDs,
		SearchQuery:      input.SearchQuery,
		IncludeBacklinks: input.IncludeBacklinks,
	})
	if err != nil {
		return WritingResult{}, err
	}
	if len(contextNotes) == 0 && kind != WritingKindPrompt && kind != WritingKindPromptImprovement {
		return WritingResult{}, ErrInputInvalid
	}
	if err := validateExpectedSources(input.ExpectedSources, contextNotes); err != nil {
		return WritingResult{}, err
	}

	if !s.tryStartGeneration() {
		return WritingResult{}, ErrBusy
	}
	defer s.finishGeneration()

	apiKey, err := s.credentialForSummary(ctx, providerID, modelID)
	if err != nil {
		return WritingResult{}, err
	}
	adapter, ok := s.adapter.(TextGenerationProviderAdapter)
	if !ok {
		return WritingResult{}, ErrProviderUnavailable
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	userContent := buildWritingUserMessage(kind, instruction, contextNotes)
	result, err := adapter.GenerateText(operationCtx, providerID, apiKey, TextGenerationInput{
		ModelID:           modelID,
		SystemInstruction: buildWritingInstruction(kind),
		Messages:          []TextMessage{{Role: "user", Content: userContent}},
		MaxOutputTokens:   aiWritingOutputTokens,
	})
	if err != nil {
		return WritingResult{}, toSafeError(err)
	}
	content := strings.TrimSpace(result.Text)
	if content == "" {
		return WritingResult{}, ErrInvalidResponse
	}
	return WritingResult{
		ProviderID: providerID,
		ModelID:    modelID,
		Kind:       kind,
		Content:    content,
		Sources:    contextSources(contextNotes),
	}, nil
}

func (s *Service) SaveHistory(ctx context.Context, input SaveAIHistoryInput) (AIHistory, error) {
	normalized, err := normalizeSaveHistoryInput(input)
	if err != nil {
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
	normalized, err := normalizeSaveArtifactInput(input)
	if err != nil {
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

func (s *Service) collectContext(ctx context.Context, input AIContextInput) ([]ContextNote, error) {
	provider := s.getContextProvider()
	if provider == nil {
		if len(input.NoteIDs) == 0 && strings.TrimSpace(input.SearchQuery) == "" {
			return []ContextNote{}, nil
		}
		return nil, ErrProviderUnavailable
	}
	normalized, err := normalizeContextInput(input)
	if err != nil {
		return nil, err
	}

	items := make([]ContextNote, 0, aiMaxContextSources)
	seen := make(map[string]struct{})
	add := func(item ContextNote) bool {
		if len(items) >= aiMaxContextSources || item.NoteID == "" || item.IsTrashed {
			return false
		}
		if _, ok := seen[item.NoteID]; ok {
			return false
		}
		remaining := aiContextBytes
		for _, existing := range items {
			remaining -= len([]byte(existing.Content))
		}
		if remaining <= 0 {
			return false
		}
		item.Content = limitUTF8Bytes(item.Content, minInt(remaining, aiContextNoteBytes))
		if strings.TrimSpace(item.Content) == "" {
			return false
		}
		seen[item.NoteID] = struct{}{}
		items = append(items, item)
		return true
	}

	for _, noteID := range normalized.NoteIDs {
		item, err := provider.Get(ctx, noteID)
		if err != nil {
			return nil, ErrInputInvalid
		}
		if item.IsTrashed {
			return nil, ErrInputInvalid
		}
		if !add(item) {
			return nil, ErrInputTooLarge
		}
	}
	if normalized.SearchQuery != "" {
		searched, err := provider.Search(ctx, normalized.SearchQuery, aiMaxContextSources)
		if err != nil {
			return nil, ErrProviderUnavailable
		}
		for _, item := range searched {
			if !add(item) {
				break
			}
		}
	}
	if normalized.IncludeBacklinks && len(normalized.NoteIDs) > 0 {
		backlinks, err := provider.ListBacklinks(ctx, normalized.NoteIDs[0], aiMaxContextSources)
		if err != nil {
			return nil, ErrProviderUnavailable
		}
		for _, item := range backlinks {
			if !add(item) {
				break
			}
		}
	}
	return items, nil
}

func (s *Service) getContextProvider() NoteContextProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contextProvider
}

func normalizeContextInput(input AIContextInput) (AIContextInput, error) {
	seen := make(map[string]struct{})
	noteIDs := make([]string, 0, len(input.NoteIDs))
	for _, value := range input.NoteIDs {
		noteID := strings.TrimSpace(value)
		if noteID == "" {
			continue
		}
		if _, ok := seen[noteID]; ok {
			continue
		}
		seen[noteID] = struct{}{}
		noteIDs = append(noteIDs, noteID)
	}
	if len(noteIDs) > aiMaxExplicitNoteIDs {
		return AIContextInput{}, ErrInputTooLarge
	}
	searchQuery := strings.TrimSpace(input.SearchQuery)
	if len([]byte(searchQuery)) > aiMaxSearchQueryBytes {
		return AIContextInput{}, ErrInputTooLarge
	}
	return AIContextInput{NoteIDs: noteIDs, SearchQuery: searchQuery, IncludeBacklinks: input.IncludeBacklinks}, nil
}

func normalizeConversationMessages(messages []AIConversationMessage) ([]AIConversationMessage, error) {
	if len(messages) > aiMaxHistoryMessages {
		return nil, ErrInputTooLarge
	}
	result := make([]AIConversationMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if (role != "user" && role != "assistant") || content == "" || !utf8.ValidString(content) {
			return nil, ErrInputInvalid
		}
		result = append(result, AIConversationMessage{Role: role, Content: content})
	}
	return result, nil
}

func normalizeAssistantKind(kind AssistantKind) (AssistantKind, error) {
	switch kind {
	case AssistantKindQA, AssistantKindBrainstorm:
		return kind, nil
	default:
		return "", ErrInputInvalid
	}
}

func normalizeChatMode(mode ChatMode) (ChatMode, error) {
	switch mode {
	case "", ChatModeAsk:
		return ChatModeAsk, nil
	case ChatModeAgent:
		return ChatModeAgent, nil
	default:
		return "", ErrInputInvalid
	}
}

func normalizeWritingKind(kind WritingKind) (WritingKind, error) {
	switch kind {
	case WritingKindPrompt, WritingKindPromptImprovement, WritingKindREADME, WritingKindDocument, WritingKindBlog, WritingKindRequirements:
		return kind, nil
	default:
		return "", ErrInputInvalid
	}
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

func normalizeSources(sources []AIHistorySource) ([]AIHistorySource, error) {
	if len(sources) > aiMaxContextSources {
		return nil, ErrInputTooLarge
	}
	seen := make(map[string]struct{}, len(sources))
	result := make([]AIHistorySource, 0, len(sources))
	for _, source := range sources {
		noteID := strings.TrimSpace(source.NoteID)
		if noteID == "" || source.InputRevision < 1 {
			return nil, ErrInputInvalid
		}
		if _, ok := seen[noteID]; ok {
			return nil, ErrInputInvalid
		}
		seen[noteID] = struct{}{}
		result = append(result, AIHistorySource{NoteID: noteID, InputRevision: source.InputRevision})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].NoteID < result[j].NoteID })
	return result, nil
}

// validateExpectedSources ensures that the snapshots shown in the UI are the
// same snapshots sent to the provider. Empty expected sources preserve the
// existing optional-context behavior for callers that did not preview first.
func validateExpectedSources(expected []AIHistorySource, actual []ContextNote) error {
	if len(expected) == 0 {
		return nil
	}
	normalizedExpected, err := normalizeSources(expected)
	if err != nil {
		return err
	}
	actualSources := make([]AIHistorySource, 0, len(actual))
	for _, item := range actual {
		actualSources = append(actualSources, AIHistorySource{
			NoteID:        item.NoteID,
			InputRevision: item.Revision,
		})
	}
	normalizedActual, err := normalizeSources(actualSources)
	if err != nil {
		return err
	}
	if len(normalizedExpected) != len(normalizedActual) {
		return ErrContextChanged
	}
	for index := range normalizedExpected {
		if normalizedExpected[index] != normalizedActual[index] {
			return ErrContextChanged
		}
	}
	return nil
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
	if err != nil || len(messages) < 2 || len(messages)%2 != 0 {
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

func contextSources(items []ContextNote) []AIContextSource {
	result := make([]AIContextSource, 0, len(items))
	for _, item := range items {
		result = append(result, AIContextSource{
			NoteID:      item.NoteID,
			Title:       item.Title,
			Revision:    item.Revision,
			Snippet:     item.Snippet,
			ContentByte: len([]byte(item.Content)),
		})
	}
	return result
}

func limitUTF8Bytes(value string, limit int) string {
	if limit <= 0 || len([]byte(value)) <= limit {
		return value
	}
	bytes := []byte(value[:0])
	for _, runeValue := range value {
		candidate := string(runeValue)
		if len(bytes)+len([]byte(candidate)) > limit {
			break
		}
		bytes = append(bytes, []byte(candidate)...)
	}
	return string(bytes)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
