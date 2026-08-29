package ai

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

type testV3TextAdapter struct {
	*testProviderAdapter
	text                string
	structured          string
	structuredErr       error
	citations           []WebCitation
	webSearches         int
	lastInput           TextGenerationInput
	lastProvider        ProviderID
	lastStructuredInput StructuredGenerationInput
	structuredCalls     int
	textCalls           int
	started             chan<- struct{}
	release             <-chan struct{}
}

func (a *testV3TextAdapter) GenerateText(ctx context.Context, providerID ProviderID, _ string, input TextGenerationInput) (TextGenerationResult, error) {
	a.textCalls++
	a.lastProvider = providerID
	a.lastInput = input
	if a.started != nil {
		a.started <- struct{}{}
	}
	if a.release != nil {
		select {
		case <-a.release:
		case <-ctx.Done():
			return TextGenerationResult{}, ctx.Err()
		}
	}
	if a.text == "" {
		return TextGenerationResult{}, ErrInvalidResponse
	}
	return TextGenerationResult{
		Text:              a.text + "|" + input.Messages[len(input.Messages)-1].Content,
		Citations:         a.citations,
		WebSearchRequests: a.webSearches,
	}, nil
}

func (a *testV3TextAdapter) GenerateStructured(ctx context.Context, providerID ProviderID, _ string, input StructuredGenerationInput, _ func(string) error) (string, error) {
	a.lastProvider = providerID
	a.lastStructuredInput = input
	a.structuredCalls++
	if a.started != nil {
		a.started <- struct{}{}
	}
	if a.release != nil {
		select {
		case <-a.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if a.structuredErr != nil {
		return "", a.structuredErr
	}
	if a.structured == "" {
		return "", ErrInvalidResponse
	}
	return a.structured, nil
}

type testContextProvider struct {
	notes map[string]ContextNote
}

func (p testContextProvider) Get(_ context.Context, noteID string) (ContextNote, error) {
	item, ok := p.notes[noteID]
	if !ok {
		return ContextNote{}, errors.New("missing note")
	}
	return item, nil
}

func (p testContextProvider) Search(_ context.Context, _ string, _ int) ([]ContextNote, error) {
	return []ContextNote{p.notes["note-2"]}, nil
}

func (p testContextProvider) ListBacklinks(_ context.Context, _ string, _ int) ([]ContextNote, error) {
	return []ContextNote{p.notes["note-3"]}, nil
}

func newV3Service(t *testing.T, adapter *testV3TextAdapter) (*Service, *sql.DB) {
	t.Helper()
	store := newMemoryCredentialStore()
	service, db := newTestServiceWithAdapter(t, store, adapter)
	service.SetNoteContextProvider(testContextProvider{notes: map[string]ContextNote{
		"note-1": {NoteID: "note-1", Title: "Current", Content: "current body", Revision: 4},
		"note-2": {NoteID: "note-2", Title: "Search result", Content: "search body", Revision: 2},
		"note-3": {NoteID: "note-3", Title: "Backlink", Content: "backlink body", Revision: 1},
	}})
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderOpenRouter,
		APIKey:     "v3-test-key",
		ModelID:    "openai/test",
	}); err != nil {
		t.Fatalf("configure provider: %v", err)
	}
	return service, db
}

func TestServiceRunsAssistantAndWritingOnlyPersistAfterExplicitSave(t *testing.T) {
	adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"}
	service, db := newV3Service(t, adapter)

	assistant, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID:       ProviderOpenRouter,
		ModelID:          "openai/test",
		Kind:             AssistantKindQA,
		Question:         "What is this note?",
		NoteIDs:          []string{"note-1"},
		SearchQuery:      "body",
		IncludeBacklinks: true,
	})
	if err != nil {
		t.Fatalf("run assistant: %v", err)
	}
	if len(assistant.Messages) != 2 || assistant.Messages[0].Role != "user" || assistant.Messages[1].Role != "assistant" {
		t.Fatalf("assistant messages = %#v", assistant.Messages)
	}
	if len(assistant.Sources) != 3 {
		t.Fatalf("assistant sources = %#v", assistant.Sources)
	}

	writing, err := service.RunWriting(t.Context(), WritingInput{
		ProviderID:  ProviderOpenRouter,
		ModelID:     "openai/test",
		Kind:        WritingKindDocument,
		Instruction: "Create a short document",
		NoteIDs:     []string{"note-1"},
	})
	if err != nil {
		t.Fatalf("run writing: %v", err)
	}
	if writing.Content == "" || len(writing.Sources) != 1 {
		t.Fatalf("writing result = %#v", writing)
	}

	var histories int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_histories").Scan(&histories); err != nil {
		t.Fatalf("count transient histories: %v", err)
	}
	if histories != 0 {
		t.Fatalf("transient histories = %d, want 0", histories)
	}

	savedHistory, err := service.SaveHistory(t.Context(), SaveAIHistoryInput{
		Kind:       assistant.Kind,
		Title:      "Saved Q&A",
		ProviderID: assistant.ProviderID,
		ModelID:    assistant.ModelID,
		Messages:   assistant.Messages,
		Sources: []AIHistorySource{
			{NoteID: assistant.Sources[0].NoteID, InputRevision: assistant.Sources[0].Revision},
		},
	})
	if err != nil {
		t.Fatalf("save history: %v", err)
	}
	if savedHistory.ID == "" || len(savedHistory.Messages) != 2 {
		t.Fatalf("saved history = %#v", savedHistory)
	}

	savedArtifact, err := service.SaveArtifact(t.Context(), SaveAIArtifactInput{
		Kind:       ArtifactKind(writing.Kind),
		Title:      "Saved document",
		ProviderID: writing.ProviderID,
		ModelID:    writing.ModelID,
		Content:    writing.Content,
		Sources: []AIHistorySource{
			{NoteID: writing.Sources[0].NoteID, InputRevision: writing.Sources[0].Revision},
		},
	})
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if savedArtifact.ID == "" || savedArtifact.Content == "" {
		t.Fatalf("saved artifact = %#v", savedArtifact)
	}
	assertAIExecutionHasNoPersistentSideEffects(t, db)
}

func TestServiceSharesGenerationSlotAcrossAssistantAndWriting(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{},
		text:                "generated",
		started:             started,
		release:             release,
	}
	service, _ := newV3Service(t, adapter)

	assistantDone := make(chan error, 1)
	go func() {
		_, err := service.RunAssistant(t.Context(), AssistantInput{
			ProviderID: ProviderOpenRouter,
			ModelID:    "openai/test",
			Kind:       AssistantKindQA,
			Question:   "Question",
			NoteIDs:    []string{"note-1"},
		})
		assistantDone <- err
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("assistant did not start text generation")
	}
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	if _, err := service.RunWriting(t.Context(), WritingInput{
		ProviderID:  ProviderOpenRouter,
		ModelID:     "openai/test",
		Kind:        WritingKindDocument,
		Instruction: "Create a document",
		NoteIDs:     []string{"note-1"},
	}); !errors.Is(err, ErrBusy) {
		t.Fatalf("writing while assistant is running error = %v, want ErrBusy", err)
	}
	if adapter.textCalls != 1 {
		t.Fatalf("text calls while assistant is running = %d, want 1", adapter.textCalls)
	}

	close(release)
	released = true
	if err := <-assistantDone; err != nil {
		t.Fatalf("assistant completion error = %v", err)
	}
}

func TestServiceCancelAssistantCorrelatesRequestAndReleasesGenerationSlot(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{},
		text:                "generated",
		started:             started,
		release:             release,
	}
	service, db := newV3Service(t, adapter)

	type assistantOutcome struct {
		result AssistantResult
		err    error
	}
	run := func(requestID string) <-chan assistantOutcome {
		outcome := make(chan assistantOutcome, 1)
		go func() {
			result, err := service.RunAssistant(t.Context(), AssistantInput{
				RequestID:  requestID,
				ProviderID: ProviderOpenRouter,
				ModelID:    "openai/test",
				Kind:       AssistantKindQA,
				Question:   "Question",
				NoteIDs:    []string{"note-1"},
			})
			outcome <- assistantOutcome{result: result, err: err}
		}()
		return outcome
	}

	first := run("assistant-request-1")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Assistant request did not reach the provider adapter")
	}
	if service.CancelAssistant("wrong-request") {
		t.Fatal("a mismatched request ID canceled the active Assistant")
	}
	select {
	case outcome := <-first:
		t.Fatalf("mismatched cancellation completed Assistant: %#v", outcome)
	default:
	}
	if !service.CancelAssistant("assistant-request-1") {
		t.Fatal("matching request ID did not cancel the active Assistant")
	}
	if service.CancelAssistant("assistant-request-1") {
		t.Fatal("Assistant cancellation was not idempotent")
	}
	select {
	case outcome := <-first:
		if !errors.Is(outcome.err, ErrCancelled) || outcome.result.Messages != nil {
			t.Fatalf("canceled Assistant outcome = %#v, want ErrCancelled and no result", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled Assistant request did not finish")
	}
	if !service.tryStartGeneration() {
		t.Fatal("canceled Assistant kept the app-wide generation lock")
	}
	service.finishGeneration()

	secondStarted := make(chan struct{}, 1)
	secondRelease := make(chan struct{})
	adapter.started = secondStarted
	adapter.release = secondRelease
	second := run("assistant-request-2")
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second Assistant request did not reach the provider adapter")
	}
	if service.CancelAssistant("assistant-request-1") {
		t.Fatal("a stale request ID canceled the newer Assistant")
	}
	select {
	case outcome := <-second:
		t.Fatalf("stale cancellation completed newer Assistant: %#v", outcome)
	default:
	}
	close(secondRelease)
	select {
	case outcome := <-second:
		if outcome.err != nil || len(outcome.result.Messages) != 2 {
			t.Fatalf("second Assistant outcome = %#v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("second Assistant request did not finish")
	}
	if service.CancelAssistant("assistant-request-2") {
		t.Fatal("a terminal Assistant remained cancelable")
	}
	assertAIExecutionHasNoPersistentSideEffects(t, db)
}

func TestAssistantRequestCancellationAndTerminalStateAreAtomic(t *testing.T) {
	for index := 0; index < 100; index++ {
		request := &assistantRequest{}
		start := make(chan struct{})
		canceled := make(chan bool, 1)
		finishedCanceled := make(chan bool, 1)
		go func() {
			<-start
			canceled <- request.markCanceled()
		}()
		go func() {
			<-start
			finishedCanceled <- request.finish()
		}()
		close(start)
		if cancelWon, finishSawCancel := <-canceled, <-finishedCanceled; cancelWon != finishSawCancel {
			t.Fatalf("race %d produced inconsistent terminal state: cancel=%t finish=%t", index, cancelWon, finishSawCancel)
		}
		if request.markCanceled() {
			t.Fatalf("race %d allowed cancellation after terminal state", index)
		}
	}
}

func TestServiceRejectsAssistantWithoutContextAndMarksSavedDataStale(t *testing.T) {
	adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"}
	service, db := newV3Service(t, adapter)

	if _, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Question:   "Question",
	}); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("missing context error = %v, want ErrInputInvalid", err)
	}

	if _, err := db.ExecContext(t.Context(), `
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES ('note-1', 'Current', 'note-1.md', 0, 0, 0, 4, '2026-07-28T00:00:00Z', '2026-07-28T00:00:00Z');
INSERT INTO ai_histories(id, kind, title, provider_id, model_id, status, created_at, updated_at)
VALUES ('history-stale', 'qa', 'Stale', 'openrouter', 'openai/test', 'saved', '2026-07-28T00:00:00Z', '2026-07-28T00:00:00Z');
INSERT INTO ai_history_messages(history_id, sequence, role, content, created_at)
VALUES ('history-stale', 1, 'user', 'Question', '2026-07-28T00:00:00Z'),
       ('history-stale', 2, 'assistant', 'Answer', '2026-07-28T00:00:00Z');
INSERT INTO ai_history_sources(history_id, note_id, input_revision)
VALUES ('history-stale', 'note-1', 3);
`); err != nil {
		t.Fatalf("insert stale history fixture: %v", err)
	}
	histories, err := service.ListHistories(t.Context())
	if err != nil {
		t.Fatalf("list histories: %v", err)
	}
	if len(histories) != 1 || histories[0].Status != AIRecordStatusStale {
		t.Fatalf("listed stale histories = %#v", histories)
	}
}

func TestServiceRoutesAgentModeAndOpenRouterWebSearchWithoutPersistingTrace(t *testing.T) {
	adapter := &testV3TextAdapter{
		testProviderAdapter: &testProviderAdapter{},
		text:                "grounded",
		citations:           []WebCitation{{URL: "https://example.com/source", Title: "Example"}},
		webSearches:         1,
	}
	service, db := newV3Service(t, adapter)

	result, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAgent,
		Question:   "Find current supporting information",
		NoteIDs:    []string{"note-1"},
		WebSearch:  true,
	})
	if err != nil {
		t.Fatalf("run agent web search: %v", err)
	}
	if result.Mode != ChatModeAgent ||
		result.WebSearchRequests != 1 ||
		!adapter.lastInput.WebSearch ||
		adapter.lastProvider != ProviderOpenRouter {
		t.Fatalf("agent routing result=%#v input=%#v provider=%q", result, adapter.lastInput, adapter.lastProvider)
	}
	if !strings.Contains(adapter.lastInput.SystemInstruction, "Agentモード規則") {
		t.Fatalf("agent prompt missing mode contract: %q", adapter.lastInput.SystemInstruction)
	}
	if len(result.Citations) != 1 || result.Citations[0].URL != "https://example.com/source" {
		t.Fatalf("agent citations = %#v", result.Citations)
	}
	assertAIExecutionHasNoPersistentSideEffects(t, db)
}

func TestServiceRejectsWebSearchWhenAdapterDoesNotReportExactlyOneSearch(t *testing.T) {
	for _, searchCount := range []int{0, 2} {
		t.Run(string(rune('0'+searchCount)), func(t *testing.T) {
			adapter := &testV3TextAdapter{
				testProviderAdapter: &testProviderAdapter{},
				text:                "ungrounded",
				webSearches:         searchCount,
			}
			service, _ := newV3Service(t, adapter)

			_, err := service.RunAssistant(t.Context(), AssistantInput{
				ProviderID: ProviderOpenRouter,
				ModelID:    "openai/test",
				Kind:       AssistantKindQA,
				Mode:       ChatModeAsk,
				Question:   "Find current information",
				NoteIDs:    []string{"note-1"},
				WebSearch:  true,
			})
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("search count %d error = %v, want ErrInvalidResponse", searchCount, err)
			}
		})
	}
}

func TestServiceRejectsUnknownModeAndGeminiWebSearchBeforeProviderRequest(t *testing.T) {
	adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"}
	service, _ := newV3Service(t, adapter)

	if _, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Mode:       ChatMode("automatic"),
		Question:   "Question",
		NoteIDs:    []string{"note-1"},
	}); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("unknown mode error = %v, want ErrInputInvalid", err)
	}
	if _, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderGemini,
		ModelID:    "gemini-2.5-flash",
		Kind:       AssistantKindQA,
		Mode:       ChatModeAsk,
		Question:   "Question",
		NoteIDs:    []string{"note-1"},
		WebSearch:  true,
	}); !errors.Is(err, ErrModelCapabilityUnavailable) {
		t.Fatalf("Gemini web search error = %v, want ErrModelCapabilityUnavailable", err)
	}
	if adapter.lastInput.WebSearch {
		t.Fatal("provider was called for rejected Gemini web search")
	}
}

func TestServiceRejectsChangedOrTrashedContextBeforeProviderRequest(t *testing.T) {
	adapter := &testV3TextAdapter{testProviderAdapter: &testProviderAdapter{}, text: "generated"}
	service, _ := newV3Service(t, adapter)

	if _, err := service.RunAssistant(t.Context(), AssistantInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/test",
		Kind:       AssistantKindQA,
		Question:   "Question",
		NoteIDs:    []string{"note-1"},
		ExpectedSources: []AIHistorySource{
			{NoteID: "note-1", InputRevision: 3},
		},
	}); !errors.Is(err, ErrContextChanged) {
		t.Fatalf("changed context error = %v, want ErrContextChanged", err)
	}

	service.SetNoteContextProvider(testContextProvider{notes: map[string]ContextNote{
		"note-1": {NoteID: "note-1", Title: "Trashed", Content: "private body", Revision: 4, IsTrashed: true},
	}})
	if _, err := service.RunWriting(t.Context(), WritingInput{
		ProviderID:  ProviderOpenRouter,
		ModelID:     "openai/test",
		Kind:        WritingKindDocument,
		Instruction: "Create a document",
		NoteIDs:     []string{"note-1"},
	}); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("trashed context error = %v, want ErrInputInvalid", err)
	}
}
