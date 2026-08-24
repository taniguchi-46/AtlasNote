package ai

import (
	"context"
	"strings"
	"sync"
)

const (
	aiAssistantOutputTokens = 2048
	aiWritingOutputTokens   = 4096
)

type assistantRequest struct {
	id      string
	cancel  context.CancelFunc
	cleanup func()

	mu           sync.Mutex
	userCanceled bool
	terminal     bool
}

func (r *assistantRequest) markCanceled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.userCanceled || r.terminal {
		return false
	}
	r.userCanceled = true
	return true
}

func (r *assistantRequest) finish() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.terminal = true
	return r.userCanceled
}

func (s *Service) RunAssistant(ctx context.Context, input AssistantInput) (result AssistantResult, err error) {
	requestCtx, request, err := s.startAssistantRequest(ctx, input.RequestID)
	if err != nil {
		return AssistantResult{}, err
	}
	if request == nil {
		return s.runAssistant(ctx, input)
	}
	defer func() {
		userCanceled := request.finish()
		s.assistantMu.Lock()
		if s.activeAssistant == request {
			s.activeAssistant = nil
		}
		s.assistantMu.Unlock()
		request.cleanup()
		if userCanceled {
			result = AssistantResult{}
			err = ErrCancelled
		}
	}()
	return s.runAssistant(requestCtx, input)
}

func (s *Service) startAssistantRequest(ctx context.Context, requestID string) (context.Context, *assistantRequest, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ctx, nil, nil
	}
	operationCtx, cleanup := s.operationContext(ctx)
	request := &assistantRequest{id: requestID, cancel: cleanup, cleanup: cleanup}

	s.assistantMu.Lock()
	if s.activeAssistant != nil {
		s.assistantMu.Unlock()
		cleanup()
		return ctx, nil, ErrBusy
	}
	s.activeAssistant = request
	s.assistantMu.Unlock()
	return operationCtx, request, nil
}

func (s *Service) CancelAssistant(requestID string) bool {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false
	}
	s.assistantMu.Lock()
	request := s.activeAssistant
	s.assistantMu.Unlock()
	if request == nil || request.id != requestID || !request.markCanceled() {
		return false
	}
	request.cancel()
	return true
}

func (s *Service) runAssistant(ctx context.Context, input AssistantInput) (AssistantResult, error) {
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
	question, err := normalizeAssistantQuestion(input.Question)
	if err != nil {
		return AssistantResult{}, err
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
	if conversationMessageBytes(messages) > textMessageLimitBytes {
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
		return s.runAgentEditProposal(ctx, input, providerID, modelID, kind, mode, messages, contextNotes)
	}

	providerMessages := make([]TextMessage, 0, len(messages)+1)
	if contextMessage := buildContextMessage(contextNotes); contextMessage != "" {
		providerMessages = append(providerMessages, TextMessage{Role: "user", Content: contextMessage})
	}
	for _, message := range messages {
		providerMessages = append(providerMessages, TextMessage{Role: message.Role, Content: message.Content})
	}
	result, err := s.generateText(ctx, providerID, modelID, TextGenerationInput{
		ModelID:           modelID,
		SystemInstruction: buildAssistantInstruction(kind, mode, input.WebSearch),
		Messages:          providerMessages,
		MaxOutputTokens:   aiAssistantOutputTokens,
		WebSearch:         input.WebSearch,
	})
	if err != nil {
		return AssistantResult{}, err
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
	instruction, err := normalizeWritingInstruction(input.Instruction)
	if err != nil {
		return WritingResult{}, err
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

	userContent := buildWritingUserMessage(kind, instruction, contextNotes)
	result, err := s.generateText(ctx, providerID, modelID, TextGenerationInput{
		ModelID:           modelID,
		SystemInstruction: buildWritingInstruction(kind),
		Messages:          []TextMessage{{Role: "user", Content: userContent}},
		MaxOutputTokens:   aiWritingOutputTokens,
	})
	if err != nil {
		return WritingResult{}, err
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
