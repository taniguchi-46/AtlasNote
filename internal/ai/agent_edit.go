package ai

import (
	"context"
	"strings"
	"unicode/utf8"
)

const (
	agentProposalMessageBytes = 8 * 1024
	agentProposalReasonBytes  = 1024
	agentProposalHunkBytes    = 16 * 1024
)

func (s *Service) runAgentEditProposal(
	ctx context.Context,
	input AssistantInput,
	providerID ProviderID,
	modelID string,
	kind AssistantKind,
	mode ChatMode,
	messages []AIConversationMessage,
	contextNotes []ContextNote,
) (AssistantResult, error) {
	if len(input.ExpectedSources) == 0 {
		return AssistantResult{}, ErrInputInvalid
	}
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
	if first < 0 || strings.Contains(snapshot.Content[first+1:], *response.Before) {
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
