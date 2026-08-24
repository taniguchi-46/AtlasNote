package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type testLibrarianAdapter struct {
	*testProviderAdapter

	librarianResult       string
	librarianErr          error
	librarianChunks       []string
	librarianStarted      chan<- struct{}
	librarianContinue     <-chan struct{}
	librarianWaitForClose bool

	mu                  sync.Mutex
	librarianCalls      int
	receivedLibrarianIn StructuredGenerationInput
}

func (a *testLibrarianAdapter) GenerateStructured(ctx context.Context, _ ProviderID, _ string, input StructuredGenerationInput, onChunk func(string) error) (string, error) {
	a.mu.Lock()
	a.librarianCalls++
	a.receivedLibrarianIn = input
	a.mu.Unlock()
	if a.librarianStarted != nil {
		a.librarianStarted <- struct{}{}
	}
	for _, chunk := range a.librarianChunks {
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return "", err
			}
		}
	}
	if a.librarianContinue != nil {
		<-a.librarianContinue
	}
	if a.librarianWaitForClose {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if a.librarianErr != nil {
		return "", a.librarianErr
	}
	return a.librarianResult, nil
}

func TestServiceLibrarianStreamsStructuredResultAndSharesGenerationLock(t *testing.T) {
	store := newMemoryCredentialStore()
	started := make(chan struct{}, 1)
	continueGeneration := make(chan struct{})
	continued := false
	defer func() {
		if !continued {
			close(continueGeneration)
		}
	}()
	adapter := &testLibrarianAdapter{
		testProviderAdapter: &testProviderAdapter{},
		librarianResult:     `{"candidates":[{"noteId":"candidate-1","score":0.91,"reason":"same topic"}]}`,
		librarianChunks:     []string{"{\"candidates\":"},
		librarianStarted:    started,
		librarianContinue:   continueGeneration,
	}
	service, db := newTestServiceWithAdapter(t, store, adapter)
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderOpenRouter,
		APIKey:     "stored-librarian-key",
		ModelID:    "openai/gpt-test",
	}); err != nil {
		t.Fatalf("configure librarian provider: %v", err)
	}

	events := make(chan LibrarianEvent, 4)
	response, err := service.StartLibrarian(context.Background(), testLibrarianInput(LibrarianOperationRelated), func(event LibrarianEvent) {
		events <- event
	})
	if err != nil || response.RequestID == "" {
		t.Fatalf("start librarian = %#v, %v", response, err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("librarian did not reach the provider adapter")
	}

	if _, err := service.GenerateSummary(t.Context(), GenerateSummaryInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/gpt-test",
		Content:    "summary must wait while librarian runs",
	}); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlapping summary error = %v", err)
	}
	close(continueGeneration)
	continued = true

	partial := receiveLibrarianEvent(t, events)
	if partial.Phase != librarianPartialPhase || partial.Sequence != 1 || partial.RequestID != response.RequestID {
		t.Fatalf("partial event = %#v", partial)
	}
	completed := receiveLibrarianEvent(t, events)
	if completed.Phase != librarianCompletedPhase || completed.Sequence != 2 || completed.Result == nil {
		t.Fatalf("completed event = %#v", completed)
	}
	if completed.Result.Quality != librarianQualityNormal || len(completed.Result.Candidates) != 1 || completed.Result.Candidates[0].NoteID != "candidate-1" {
		t.Fatalf("librarian result = %#v", completed.Result)
	}

	adapter.mu.Lock()
	received := adapter.receivedLibrarianIn
	calls := adapter.librarianCalls
	adapter.mu.Unlock()
	if calls != 1 || received.Name != "atlas_note_librarian" || received.MaxOutputTokens != summaryOutputTokenLimit || !strings.Contains(received.Prompt, "note-body-marker") {
		t.Fatalf("librarian adapter input = calls:%d input:%#v", calls, received)
	}
	var schema map[string]any
	if err := json.Unmarshal(received.Schema, &schema); err != nil {
		t.Fatalf("librarian schema: %v", err)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("librarian schema is not strict: %#v", schema)
	}
	assertAIExecutionHasNoPersistentSideEffects(t, db)
}

func TestServiceLibrarianReportsKnownCapabilityMismatchWithoutCallingProvider(t *testing.T) {
	store := newMemoryCredentialStore()
	adapter := &testLibrarianAdapter{testProviderAdapter: &testProviderAdapter{}}
	service, _ := newTestServiceWithAdapter(t, store, adapter)
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderOpenRouter,
		APIKey:     "stored-librarian-key",
		ModelID:    "openai/gpt-test",
	}); err != nil {
		t.Fatalf("configure librarian provider: %v", err)
	}
	service.cacheModelMetadata(ProviderOpenRouter, ModelListResult{Models: []ModelInfo{{
		ID:                "openai/gpt-test",
		Available:         true,
		SupportsSummary:   true,
		SupportsLibrarian: false,
	}}})

	events := make(chan LibrarianEvent, 1)
	response, err := service.StartLibrarian(t.Context(), testLibrarianInput(LibrarianOperationRelated), func(event LibrarianEvent) {
		events <- event
	})
	if err != nil || response.RequestID == "" {
		t.Fatalf("start librarian = %#v, %v", response, err)
	}
	event := receiveLibrarianEvent(t, events)
	if event.Phase != librarianFailedPhase || event.Error == nil || event.Error.Code != ErrorCodeModelCapabilityUnavailable {
		t.Fatalf("capability failure event = %#v", event)
	}
	adapter.mu.Lock()
	calls := adapter.librarianCalls
	adapter.mu.Unlock()
	if calls != 0 {
		t.Fatalf("capability mismatch called provider %d times", calls)
	}
}

func TestServiceCancelLibrarianIsIdempotentAndEmitsCanceled(t *testing.T) {
	store := newMemoryCredentialStore()
	started := make(chan struct{}, 1)
	adapter := &testLibrarianAdapter{
		testProviderAdapter:   &testProviderAdapter{},
		librarianResult:       `{"candidates":[]}`,
		librarianChunks:       []string{"partial-json"},
		librarianStarted:      started,
		librarianWaitForClose: true,
	}
	service, _ := newTestServiceWithAdapter(t, store, adapter)
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderGemini,
		APIKey:     "stored-cancel-key",
		ModelID:    "gemini-2.5-flash",
	}); err != nil {
		t.Fatalf("configure cancel provider: %v", err)
	}

	events := make(chan LibrarianEvent, 4)
	response, err := service.StartLibrarian(context.Background(), testLibrarianInputForProvider(LibrarianOperationDuplicate, ProviderGemini, "gemini-2.5-flash"), func(event LibrarianEvent) {
		events <- event
	})
	if err != nil {
		t.Fatalf("start cancelable librarian: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("cancelable librarian did not reach the provider adapter")
	}
	partial := receiveLibrarianEvent(t, events)
	if partial.Phase != librarianPartialPhase {
		t.Fatalf("cancel partial event = %#v", partial)
	}

	service.CancelLibrarian(response.RequestID)
	service.CancelLibrarian(response.RequestID)
	service.CancelLibrarian("unknown-request")
	canceled := receiveLibrarianEvent(t, events)
	if canceled.Phase != librarianCanceledPhase || canceled.Error == nil || canceled.Error.Code != ErrorCodeCancelled {
		t.Fatalf("canceled event = %#v", canceled)
	}
	if _, err := service.GenerateSummary(t.Context(), GenerateSummaryInput{
		ProviderID: ProviderGemini,
		ModelID:    "gemini-2.5-flash",
		Content:    "generation lock is released",
	}); err != nil && errors.Is(err, ErrBusy) {
		t.Fatal("canceled librarian kept the app-wide generation lock")
	}
}

func TestServiceLibrarianReturnsEmptyWithoutProviderForMissingCandidates(t *testing.T) {
	adapter := &testLibrarianAdapter{testProviderAdapter: &testProviderAdapter{}}
	service, _ := newTestServiceWithAdapter(t, newMemoryCredentialStore(), adapter)
	events := make(chan LibrarianEvent, 1)
	response, err := service.StartLibrarian(context.Background(), LibrarianInput{
		ProviderID:     ProviderOpenRouter,
		ModelID:        "openai/gpt-test",
		Operation:      LibrarianOperationRelated,
		NoteID:         "note-1",
		BaseRevision:   1,
		Title:          "Target note",
		Content:        "note-body-marker",
		CandidateCount: LibrarianMinCandidateCount,
	}, func(event LibrarianEvent) { events <- event })
	if err != nil || response.RequestID == "" {
		t.Fatalf("empty candidate start = %#v, %v", response, err)
	}
	event := receiveLibrarianEvent(t, events)
	if event.Phase != librarianCompletedPhase || event.Result == nil || event.Result.Quality != librarianQualityEmpty || len(event.Result.Candidates) != 0 {
		t.Fatalf("empty candidate event = %#v", event)
	}
	adapter.mu.Lock()
	calls := adapter.librarianCalls
	adapter.mu.Unlock()
	if calls != 0 {
		t.Fatalf("empty candidate request called provider %d times", calls)
	}
}

func TestServiceLibrarianEmitsSafeFailureWithoutProviderDetails(t *testing.T) {
	adapter := &testLibrarianAdapter{
		testProviderAdapter: &testProviderAdapter{},
		librarianErr:        errors.New("raw-provider-detail-marker"),
	}
	service, _ := newTestServiceWithAdapter(t, newMemoryCredentialStore(), adapter)
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderOpenRouter,
		APIKey:     "stored-safe-error-key",
		ModelID:    "openai/gpt-test",
	}); err != nil {
		t.Fatalf("configure safe error provider: %v", err)
	}

	events := make(chan LibrarianEvent, 1)
	if _, err := service.StartLibrarian(context.Background(), testLibrarianInput(LibrarianOperationTitle), func(event LibrarianEvent) {
		events <- event
	}); err != nil {
		t.Fatalf("start failing librarian: %v", err)
	}
	event := receiveLibrarianEvent(t, events)
	if event.Phase != librarianFailedPhase || event.Error == nil || event.Error.Code != ErrorCodeProviderUnavailable {
		t.Fatalf("safe failure event = %#v", event)
	}
	if strings.Contains(event.Error.Error(), "raw-provider-detail-marker") {
		t.Fatal("provider details leaked into librarian safe error")
	}
}

func TestNormalizeLibrarianResultRejectsUnsafeOrOutOfPoolOutput(t *testing.T) {
	input := testLibrarianInput(LibrarianOperationRelated)
	for _, testCase := range []struct {
		name string
		raw  string
	}{
		{name: "missing envelope candidates", raw: `{}`},
		{name: "unknown envelope field", raw: `{"candidates":[],"unexpected":true}`},
		{name: "unknown candidate field", raw: `{"candidates":[{"noteId":"candidate-1","score":0.8,"unexpected":true}]}`},
		{name: "missing score", raw: `{"candidates":[{"noteId":"candidate-1"}]}`},
		{name: "out of pool", raw: `{"candidates":[{"noteId":"other-note","score":0.8}]}`},
		{name: "duplicate", raw: `{"candidates":[{"noteId":"candidate-1","score":0.8},{"noteId":"candidate-1","score":0.7}]}`},
		{name: "invalid score", raw: `{"candidates":[{"noteId":"candidate-1","score":1.1}]}`},
		{name: "long reason", raw: `{"candidates":[{"noteId":"candidate-1","score":0.8,"reason":"` + strings.Repeat("a", LibrarianReasonLimitRunes+1) + `"}]}`},
		{name: "multiple JSON values", raw: `{"candidates":[]} {"candidates":[]}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := normalizeLibrarianResult(input, testCase.raw); !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("normalize error = %v", err)
			}
		})
	}

	low, err := normalizeLibrarianResult(input, `{"candidates":[{"noteId":"candidate-1","score":0.59}]}`)
	if err != nil || low.Quality != librarianQualityLow {
		t.Fatalf("low quality result = %#v, %v", low, err)
	}
}

func TestLibrarianLargeCandidatePoolIsBoundedDeduplicatedAndSafelyNarrowed(t *testing.T) {
	input := testLibrarianInput(LibrarianOperationRelated)
	input.CandidateCount = LibrarianMaxCandidateCount
	input.Candidates = make([]LibrarianCandidateContext, 0, LibrarianMaxCandidatePool)
	for index := range LibrarianMaxCandidatePool {
		input.Candidates = append(input.Candidates, LibrarianCandidateContext{
			NoteID:  fmt.Sprintf("candidate-%02d", index),
			Title:   fmt.Sprintf("Candidate %02d", index),
			Snippet: strings.Repeat("候補", LibrarianSnippetLimitRunes),
		})
	}

	normalized, err := normalizeLibrarianInput(input)
	if err != nil {
		t.Fatalf("normalize maximum candidate pool: %v", err)
	}
	if len(normalized.Candidates) != LibrarianMaxCandidatePool {
		t.Fatalf("normalized candidate pool size = %d, want %d", len(normalized.Candidates), LibrarianMaxCandidatePool)
	}
	for _, candidate := range normalized.Candidates {
		if len([]rune(candidate.Snippet)) != LibrarianSnippetLimitRunes {
			t.Fatalf("candidate %q snippet runes = %d, want %d", candidate.NoteID, len([]rune(candidate.Snippet)), LibrarianSnippetLimitRunes)
		}
	}

	overLimit := input
	overLimit.Candidates = append(append([]LibrarianCandidateContext{}, input.Candidates...), LibrarianCandidateContext{NoteID: "candidate-over-limit"})
	if _, err := normalizeLibrarianInput(overLimit); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("over-limit candidate pool error = %v", err)
	}

	filtered := normalizeCandidateContexts(input.NoteID, []LibrarianCandidateContext{
		{NoteID: " candidate-safe ", Title: " First candidate ", Snippet: " first snippet "},
		{NoteID: "candidate-safe", Title: "duplicate must be removed"},
		{NoteID: input.NoteID, Title: "current note must be removed"},
		{NoteID: "", Title: "empty ID must be removed"},
		{NoteID: "unsafe\nID", Title: "control character must be removed"},
		{NoteID: "candidate-second", Title: " Second candidate ", Snippet: strings.Repeat("長", LibrarianSnippetLimitRunes+1)},
	})
	if len(filtered) != 2 || filtered[0].NoteID != "candidate-safe" || filtered[1].NoteID != "candidate-second" {
		t.Fatalf("filtered candidate pool = %#v", filtered)
	}
	if filtered[0].Title != "First candidate" || filtered[0].Snippet != "first snippet" {
		t.Fatalf("trimmed candidate = %#v", filtered[0])
	}
	if len([]rune(filtered[1].Snippet)) != LibrarianSnippetLimitRunes {
		t.Fatalf("filtered snippet runes = %d, want %d", len([]rune(filtered[1].Snippet)), LibrarianSnippetLimitRunes)
	}

	unsafePool := input
	unsafePool.Candidates = []LibrarianCandidateContext{
		{NoteID: "candidate-safe"},
		{NoteID: "candidate-safe"},
		{NoteID: input.NoteID},
	}
	if _, err := normalizeLibrarianInput(unsafePool); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("duplicate/current-note candidate pool error = %v", err)
	}

	narrowed := normalized
	narrowed.CandidateCount = 2
	if _, err := normalizeLibrarianResult(narrowed, `{"candidates":[{"noteId":"candidate-00","score":0.9},{"noteId":"candidate-01","score":0.8},{"noteId":"candidate-02","score":0.7}]}`); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("result exceeding requested candidate count error = %v", err)
	}
}

func testLibrarianInput(operation LibrarianOperation) LibrarianInput {
	return testLibrarianInputForProvider(operation, ProviderOpenRouter, "openai/gpt-test")
}

func testLibrarianInputForProvider(operation LibrarianOperation, providerID ProviderID, modelID string) LibrarianInput {
	return LibrarianInput{
		ProviderID:     providerID,
		ModelID:        modelID,
		Operation:      operation,
		NoteID:         "note-1",
		BaseRevision:   4,
		Title:          "Target note",
		Content:        "note-body-marker",
		CandidateCount: 5,
		Candidates: []LibrarianCandidateContext{{
			NoteID:  "candidate-1",
			Title:   "Candidate note",
			Snippet: "related snippet",
		}},
	}
}

func receiveLibrarianEvent(t *testing.T, events <-chan LibrarianEvent) LibrarianEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for librarian event")
		return LibrarianEvent{}
	}
}
