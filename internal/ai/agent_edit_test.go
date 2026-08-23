package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceReturnsValidatedAgentEditProposalWithoutPersistence(t *testing.T) {
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{},
		text:                "ordinary text must not be used",
		structured:          `{"message":"本文を簡潔にします。確認してから適用してください。","hasProposal":true,"reason":"重複を減らすため","before":"current body","after":"revised body"}`,
	}
	service, db := newV3Service(t, adapter)

	result, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAgent,
		Question:   "本文を短くして",
		NoteIDs:    []string{"note-1", "note-2"},
		ExpectedSources: []AIHistorySource{
			{NoteID: "note-1", InputRevision: 4},
			{NoteID: "note-2", InputRevision: 2},
		},
		AgentTarget: &AgentEditTarget{
			NoteID:       "note-1",
			BaseRevision: 4,
		},
	})
	if err != nil {
		t.Fatalf("run agent proposal: %v", err)
	}
	if result.Proposal == nil {
		t.Fatal("agent result omitted proposal")
	}
	if result.Proposal.TargetNoteID != "note-1" || result.Proposal.TargetTitle != "Current" || result.Proposal.BaseRevision != 4 || result.Proposal.Before != "current body" || result.Proposal.After != "revised body" {
		t.Fatalf("agent proposal = %#v", result.Proposal)
	}
	if len(result.Proposal.AffectedFields) != 1 || result.Proposal.AffectedFields[0] != "content" {
		t.Fatalf("agent proposal fields = %#v", result.Proposal.AffectedFields)
	}
	if adapter.structuredCalls != 1 || adapter.textCalls != 0 || adapter.lastStructuredInput.Name != "atlas_note_agent_edit" || adapter.lastStructuredInput.MaxOutputTokens != aiAssistantOutputTokens {
		t.Fatalf("agent structured input = calls:%d text:%d input:%#v", adapter.structuredCalls, adapter.textCalls, adapter.lastStructuredInput)
	}
	assertAIExecutionHasNoPersistentSideEffects(t, db)
}

func TestServiceAllowsAgentWhenCatalogCannotConfirmStructuredSupport(t *testing.T) {
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{
			listResult: ModelListResult{Models: []ModelInfo{{
				ID:              "openai/test",
				SupportsSummary: true,
				Available:       true,
				AgentCapability: AgentCapabilityUnknown,
			}}},
		},
		structured: `{"message":"ok","hasProposal":false,"reason":"","before":"","after":""}`,
	}
	service, _ := newV3Service(t, adapter)
	if _, err := service.ListModels(t.Context(), ListModelsInput{ProviderID: ProviderOpenRouter, UseStoredCredential: true}); err != nil {
		t.Fatalf("cache model catalog: %v", err)
	}

	result, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAgent,
		Question:   "確認して",
		NoteIDs:    []string{"note-1"},
		ExpectedSources: []AIHistorySource{
			{NoteID: "note-1", InputRevision: 4},
		},
		AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4},
	})
	if err != nil {
		t.Fatalf("agent with unknown catalog capability: %v", err)
	}
	if result.Proposal != nil || adapter.structuredCalls != 1 {
		t.Fatalf("agent result = %#v, structured calls = %d", result, adapter.structuredCalls)
	}
}

func TestServiceRejectsExplicitlyUnsupportedAgentModel(t *testing.T) {
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{
			listResult: ModelListResult{Models: []ModelInfo{{
				ID:              "openai/test",
				SupportsSummary: true,
				Available:       true,
				AgentCapability: AgentCapabilityUnsupported,
			}}},
		},
		structured: `{"message":"must not run","hasProposal":false,"reason":"","before":"","after":""}`,
	}
	service, _ := newV3Service(t, adapter)
	if _, err := service.ListModels(t.Context(), ListModelsInput{ProviderID: ProviderOpenRouter, UseStoredCredential: true}); err != nil {
		t.Fatalf("cache model catalog: %v", err)
	}
	_, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAgent,
		Question:   "変更して",
		NoteIDs:    []string{"note-1"},
		ExpectedSources: []AIHistorySource{
			{NoteID: "note-1", InputRevision: 4},
		},
		AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4},
	})
	if !errors.Is(err, ErrModelCapabilityUnavailable) {
		t.Fatalf("unsupported agent model error = %v", err)
	}
	if adapter.structuredCalls != 0 {
		t.Fatalf("unsupported agent made %d structured calls", adapter.structuredCalls)
	}
}

func TestServiceStopsAgentOnContextCancellationAndClassifiesTimeout(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		started := make(chan struct{}, 1)
		adapter := &testV3TextAdapter{
			testProviderAdapter: &testProviderAdapter{},
			structured:          `{"message":"must not complete","hasProposal":false,"reason":"","before":"","after":""}`,
			started:             started,
			release:             make(chan struct{}),
		}
		service, db := newV3Service(t, adapter)
		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			_, err := service.RunAssistant(ctx, validAgentAssistantInput())
			result <- err
		}()

		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("Agent request did not reach the provider adapter")
		}
		cancel()
		select {
		case err := <-result:
			if !errors.Is(err, ErrProviderUnavailable) {
				t.Fatalf("canceled Agent error = %v, want safe provider error", err)
			}
		case <-time.After(time.Second):
			t.Fatal("canceled Agent request did not finish")
		}
		if !service.tryStartGeneration() {
			t.Fatal("canceled Agent kept the app-wide generation lock")
		}
		service.finishGeneration()
		assertAIExecutionHasNoPersistentSideEffects(t, db)
	})

	t.Run("provider deadline", func(t *testing.T) {
		adapter := &testV3TextAdapter{
			testProviderAdapter: &testProviderAdapter{},
			structuredErr:       context.DeadlineExceeded,
		}
		service, db := newV3Service(t, adapter)

		_, err := service.RunAssistant(t.Context(), validAgentAssistantInput())
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("timed out Agent error = %v, want ErrTimeout", err)
		}
		if adapter.structuredCalls != 1 {
			t.Fatalf("timed out Agent structured calls = %d, want 1", adapter.structuredCalls)
		}
		assertAIExecutionHasNoPersistentSideEffects(t, db)
	})
}

func validAgentAssistantInput() AssistantInput {
	return AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAgent,
		Question:   "Change it",
		NoteIDs:    []string{"note-1"},
		ExpectedSources: []AIHistorySource{
			{NoteID: "note-1", InputRevision: 4},
		},
		AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4},
	}
}

func TestServiceRejectsInvalidOrStaleAgentEditProposal(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		input  AssistantInput
		output string
		want   error
		calls  int
	}{
		{
			name:  "missing target",
			input: AssistantInput{ProviderID: ProviderOpenRouter, ModelID: "openai/test", Kind: AssistantKindQA, Mode: ChatModeAgent, Question: "Change it", NoteIDs: []string{"note-1"}, ExpectedSources: []AIHistorySource{{NoteID: "note-1", InputRevision: 4}}},
			want:  ErrInputInvalid,
		},
		{
			name:  "target is not active note",
			input: AssistantInput{ProviderID: ProviderOpenRouter, ModelID: "openai/test", Kind: AssistantKindQA, Mode: ChatModeAgent, Question: "Change it", NoteIDs: []string{"note-1", "note-2"}, ExpectedSources: []AIHistorySource{{NoteID: "note-1", InputRevision: 4}, {NoteID: "note-2", InputRevision: 2}}, AgentTarget: &AgentEditTarget{NoteID: "note-2", BaseRevision: 2}},
			want:  ErrInputInvalid,
		},
		{
			name:  "missing prepared sources",
			input: AssistantInput{ProviderID: ProviderOpenRouter, ModelID: "openai/test", Kind: AssistantKindQA, Mode: ChatModeAgent, Question: "Change it", NoteIDs: []string{"note-1"}, AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4}},
			want:  ErrInputInvalid,
		},
		{
			name:   "hunk is not in target body",
			input:  AssistantInput{ProviderID: ProviderOpenRouter, ModelID: "openai/test", Kind: AssistantKindQA, Mode: ChatModeAgent, Question: "Change it", NoteIDs: []string{"note-1"}, ExpectedSources: []AIHistorySource{{NoteID: "note-1", InputRevision: 4}}, AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4}},
			output: `{"message":"提案です","hasProposal":true,"reason":"理由","before":"missing","after":"replacement"}`,
			want:   ErrInvalidResponse,
			calls:  1,
		},
		{
			name:   "missing required response field",
			input:  AssistantInput{ProviderID: ProviderOpenRouter, ModelID: "openai/test", Kind: AssistantKindQA, Mode: ChatModeAgent, Question: "Change it", NoteIDs: []string{"note-1"}, ExpectedSources: []AIHistorySource{{NoteID: "note-1", InputRevision: 4}}, AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4}},
			output: `{"message":"提案です","hasProposal":false}`,
			want:   ErrInvalidResponse,
			calls:  1,
		},
		{
			name: "oversized conversation before provider request",
			input: AssistantInput{
				ProviderID: ProviderOpenRouter,
				ModelID:    "openai/test",
				Kind:       AssistantKindQA,
				Mode:       ChatModeAgent,
				Question:   "Change it",
				Messages:   []AIConversationMessage{{Role: "user", Content: strings.Repeat("x", textMessageLimitBytes)}},
				NoteIDs:    []string{"note-1"},
				ExpectedSources: []AIHistorySource{
					{NoteID: "note-1", InputRevision: 4},
				},
				AgentTarget: &AgentEditTarget{NoteID: "note-1", BaseRevision: 4},
			},
			want: ErrInputTooLarge,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "unused", structured: testCase.output}
			service, _ := newV3Service(t, adapter)
			_, err := service.RunAssistant(t.Context(), testCase.input)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("agent proposal error = %v, want %v", err, testCase.want)
			}
			if adapter.structuredCalls != testCase.calls {
				t.Fatalf("structured provider calls = %d, want %d", adapter.structuredCalls, testCase.calls)
			}
		})
	}
}
