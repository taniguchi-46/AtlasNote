package ai

import (
	"errors"
	"strings"
	"testing"
)

func TestServiceRejectsMalformedAssistantAndWritingInputBeforeProviderRequest(t *testing.T) {
	adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"}
	service, _ := newV3Service(t, adapter)

	for _, testCase := range []struct {
		name string
		run  func() error
	}{
		{
			name: "empty assistant question",
			run: func() error {
				_, err := service.RunAssistant(t.Context(), AssistantInput{
					ProviderID: ProviderOpenRouter,
					ModelID:    "openai/test",
					Kind:       AssistantKindQA,
					NoteIDs:    []string{"note-1"},
				})
				return err
			},
		},
		{
			name: "oversized assistant question",
			run: func() error {
				_, err := service.RunAssistant(t.Context(), AssistantInput{
					ProviderID: ProviderOpenRouter,
					ModelID:    "openai/test",
					Kind:       AssistantKindQA,
					Question:   strings.Repeat("a", aiMaxQuestionBytes+1),
					NoteIDs:    []string{"note-1"},
				})
				return err
			},
		},
		{
			name: "invalid writing instruction",
			run: func() error {
				_, err := service.RunWriting(t.Context(), WritingInput{
					ProviderID:  ProviderOpenRouter,
					ModelID:     "openai/test",
					Kind:        WritingKindDocument,
					Instruction: string([]byte{0xff}),
					NoteIDs:     []string{"note-1"},
				})
				return err
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.run(); !errors.Is(err, ErrInputInvalid) {
				t.Fatalf("error = %v, want ErrInputInvalid", err)
			}
			if adapter.textCalls != 0 {
				t.Fatalf("provider calls = %d, want 0", adapter.textCalls)
			}
		})
	}
}
