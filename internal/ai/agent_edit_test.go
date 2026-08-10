package ai

import (
	"errors"
	"strings"
	"testing"
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
