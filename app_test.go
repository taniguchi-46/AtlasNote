package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	aiservice "atlasnote/internal/ai"
	"atlasnote/internal/credential"
	"atlasnote/internal/database"
	"atlasnote/internal/datalock"
	"atlasnote/internal/note"
	syncservice "atlasnote/internal/sync"
)

type appTestAIConnectionChecker struct {
	err error
}

func (c *appTestAIConnectionChecker) Check(context.Context, aiservice.ProviderID, string) error {
	return c.err
}

type appTestAIProviderAdapter struct {
	listResult       aiservice.ModelListResult
	listErr          error
	summaryResult    aiservice.SummaryResult
	summaryErr       error
	textResult       aiservice.TextGenerationResult
	textErr          error
	textInput        aiservice.TextGenerationInput
	textCalls        int
	structuredResult string
	structuredErr    error
	structuredChunks []string
	structuredInput  aiservice.StructuredGenerationInput
	structuredCalls  int
	started          chan<- struct{}
	release          <-chan struct{}
}

func (a *appTestAIProviderAdapter) CheckConnection(context.Context, aiservice.ProviderID, string) error {
	return nil
}

func (a *appTestAIProviderAdapter) ListModels(context.Context, aiservice.ProviderID, string) (aiservice.ModelListResult, error) {
	if a.listErr != nil {
		return aiservice.ModelListResult{}, a.listErr
	}
	return a.listResult, nil
}

func (a *appTestAIProviderAdapter) GenerateSummary(context.Context, aiservice.ProviderID, string, aiservice.GenerateSummaryInput) (aiservice.SummaryResult, error) {
	if a.summaryErr != nil {
		return aiservice.SummaryResult{}, a.summaryErr
	}
	return a.summaryResult, nil
}

func (a *appTestAIProviderAdapter) GenerateText(ctx context.Context, _ aiservice.ProviderID, _ string, input aiservice.TextGenerationInput) (aiservice.TextGenerationResult, error) {
	a.textCalls++
	a.textInput = input
	if a.started != nil {
		a.started <- struct{}{}
	}
	if a.release != nil {
		select {
		case <-a.release:
		case <-ctx.Done():
			return aiservice.TextGenerationResult{}, ctx.Err()
		}
	}
	if a.textErr != nil {
		return aiservice.TextGenerationResult{}, a.textErr
	}
	return a.textResult, nil
}

func (a *appTestAIProviderAdapter) GenerateStructured(ctx context.Context, _ aiservice.ProviderID, _ string, input aiservice.StructuredGenerationInput, onChunk func(string) error) (string, error) {
	a.structuredCalls++
	a.structuredInput = input
	for _, chunk := range a.structuredChunks {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return "", err
			}
		}
	}
	if a.structuredErr != nil {
		return "", a.structuredErr
	}
	return a.structuredResult, nil
}

type appTestAIContextProvider struct {
	notes map[string]aiservice.ContextNote
}

func (p appTestAIContextProvider) Get(_ context.Context, noteID string) (aiservice.ContextNote, error) {
	item, ok := p.notes[noteID]
	if !ok {
		return aiservice.ContextNote{}, errors.New("missing AI context note")
	}
	return item, nil
}

func (appTestAIContextProvider) Search(context.Context, string, int) ([]aiservice.ContextNote, error) {
	return []aiservice.ContextNote{}, nil
}

func (appTestAIContextProvider) ListBacklinks(context.Context, string, int) ([]aiservice.ContextNote, error) {
	return []aiservice.ContextNote{}, nil
}

type appTestAIInvariantSnapshot struct {
	Note              note.Note
	Markdown          []byte
	Search            note.SearchResult
	SyncStatus        syncservice.StatusResult
	Conflicts         []syncservice.ConflictSummary
	StorageOperations string
	SyncConnection    string
	SyncOutbox        string
	SyncItemStates    string
	SyncSnapshots     string
	SyncConflicts     string
	SearchIndex       string
	SearchState       string
	AIHistories       string
	AIHistoryMessages string
	AIHistorySources  string
	AIArtifacts       string
	AIArtifactSources string
}

func captureAppTestAIInvariantSnapshot(t *testing.T, app *App, noteID string, markdownPath string) appTestAIInvariantSnapshot {
	t.Helper()

	storedNote, err := app.GetNote(noteID)
	if err != nil {
		t.Fatalf("get note for AI invariant snapshot: %v", err)
	}
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown for AI invariant snapshot: %v", err)
	}
	search, err := app.SearchNotes(note.SearchInput{Query: "d07"})
	if err != nil {
		t.Fatalf("search notes for AI invariant snapshot: %v", err)
	}
	if search.Error != nil {
		t.Fatal("search returned an error for AI invariant snapshot")
	}
	status, err := app.GetSyncStatus()
	if err != nil {
		t.Fatalf("get sync status for AI invariant snapshot: %v", err)
	}
	conflicts, err := app.ListSyncConflicts()
	if err != nil {
		t.Fatalf("list sync conflicts for AI invariant snapshot: %v", err)
	}

	return appTestAIInvariantSnapshot{
		Note:              storedNote,
		Markdown:          markdown,
		Search:            search,
		SyncStatus:        status,
		Conflicts:         conflicts,
		StorageOperations: appTestRowsSnapshot(t, app.db, "SELECT * FROM note_storage_operations ORDER BY created_at, operation_id"),
		SyncConnection:    appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_connections ORDER BY id"),
		SyncOutbox:        appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_outbox ORDER BY sequence"),
		SyncItemStates:    appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_item_states ORDER BY entity_key"),
		SyncSnapshots:     appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_snapshots ORDER BY snapshot_id"),
		SyncConflicts:     appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_conflicts ORDER BY conflict_id"),
		SearchIndex:       appTestRowsSnapshot(t, app.db, "SELECT rowid, note_id, title, body FROM note_search ORDER BY rowid"),
		SearchState:       appTestRowsSnapshot(t, app.db, "SELECT * FROM note_search_state ORDER BY note_id"),
		AIHistories:       appTestRowsSnapshot(t, app.db, "SELECT * FROM ai_histories ORDER BY id"),
		AIHistoryMessages: appTestRowsSnapshot(t, app.db, "SELECT * FROM ai_history_messages ORDER BY history_id, sequence"),
		AIHistorySources:  appTestRowsSnapshot(t, app.db, "SELECT * FROM ai_history_sources ORDER BY history_id, note_id"),
		AIArtifacts:       appTestRowsSnapshot(t, app.db, "SELECT * FROM ai_artifacts ORDER BY id"),
		AIArtifactSources: appTestRowsSnapshot(t, app.db, "SELECT * FROM ai_artifact_sources ORDER BY artifact_id, note_id"),
	}
}

func appTestRowsSnapshot(t *testing.T, db *sql.DB, query string) string {
	t.Helper()

	rows, err := db.QueryContext(t.Context(), query)
	if err != nil {
		t.Fatalf("query AI invariant snapshot: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("read AI invariant snapshot columns: %v", err)
	}
	result := make([][]string, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			t.Fatalf("scan AI invariant snapshot row: %v", err)
		}
		row := make([]string, len(values))
		for index, value := range values {
			switch typed := value.(type) {
			case nil:
				row[index] = "<null>"
			case []byte:
				row[index] = string(typed)
			default:
				row[index] = fmt.Sprint(typed)
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate AI invariant snapshot rows: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("encode AI invariant snapshot: %v", err)
	}
	return string(encoded)
}

func appTestDatabaseContainsMarker(t *testing.T, db *sql.DB, marker string) bool {
	t.Helper()

	tableRows, err := db.QueryContext(t.Context(), `
SELECT name
FROM sqlite_master
WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
ORDER BY name`)
	if err != nil {
		t.Fatalf("list database tables for AI marker check: %v", err)
	}
	tables := make([]string, 0)
	for tableRows.Next() {
		var table string
		if err := tableRows.Scan(&table); err != nil {
			tableRows.Close()
			t.Fatalf("scan database table for AI marker check: %v", err)
		}
		tables = append(tables, table)
	}
	if err := tableRows.Err(); err != nil {
		tableRows.Close()
		t.Fatalf("iterate database tables for AI marker check: %v", err)
	}
	if err := tableRows.Close(); err != nil {
		t.Fatalf("close database table list for AI marker check: %v", err)
	}

	for _, table := range tables {
		quotedTable := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		rows, err := db.QueryContext(t.Context(), "SELECT * FROM "+quotedTable)
		if err != nil {
			t.Fatalf("query database table for AI marker check: %v", err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatalf("read database table columns for AI marker check: %v", err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for index := range values {
				targets[index] = &values[index]
			}
			if err := rows.Scan(targets...); err != nil {
				rows.Close()
				t.Fatalf("scan database value for AI marker check: %v", err)
			}
			for _, value := range values {
				var text string
				switch typed := value.(type) {
				case []byte:
					text = string(typed)
				default:
					text = fmt.Sprint(typed)
				}
				if strings.Contains(text, marker) {
					rows.Close()
					return true
				}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("iterate database values for AI marker check: %v", err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close database values for AI marker check: %v", err)
		}
	}
	return false
}

func appTestFindMarkerInDataFiles(t *testing.T, dataDir string, marker string) string {
	t.Helper()

	var found string
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "atlasnote.lock" {
			return nil
		}
		if strings.Contains(path, marker) {
			found = path
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read AI secret scan file %q: %w", path, err)
		}
		if bytes.Contains(contents, []byte(marker)) {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan application data files for AI secret marker: %v", err)
	}
	return found
}

func TestGetStartupStatusReady(t *testing.T) {
	app := &App{dataDir: "C:\\AtlasNote"}

	status := app.GetStartupStatus()

	if !status.Ready {
		t.Fatal("expected startup status to be ready")
	}
	if status.Message != "" {
		t.Fatalf("expected empty startup message, got %q", status.Message)
	}
	if status.DataDir != "C:\\AtlasNote" {
		t.Fatalf("data dir = %q", status.DataDir)
	}
}

func TestAppAIAPIsDoNotExposeCredentialsOrPersistFailedConnectionChecks(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	checker := &appTestAIConnectionChecker{}
	aiService := aiservice.NewService(
		aiservice.NewRepository(db),
		credential.NewManager(credential.NewSessionStore()),
		checker,
	)
	app := &App{ctx: t.Context(), aiService: aiService}
	secretMarker := "wails-api-secret-marker"

	settings, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     secretMarker,
		ModelID:    "openrouter/model",
	})
	if err != nil {
		t.Fatalf("configure AI provider: %v", err)
	}
	serialized, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("serialize safe settings: %v", err)
	}
	if strings.Contains(string(serialized), secretMarker) {
		t.Fatal("Wails AI settings response exposed an API key")
	}

	checker.err = errors.New("raw provider failure " + secretMarker)
	if _, err := app.TestAIConnection(aiservice.TestConnectionInput{ProviderID: aiservice.ProviderGemini, APIKey: secretMarker}); !errors.Is(err, aiservice.ErrProviderUnavailable) {
		t.Fatalf("test AI connection error = %v", err)
	} else if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("Wails AI connection error exposed an API key")
	}
	var configured int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_provider_settings WHERE provider_id = ?", aiservice.ProviderGemini).Scan(&configured); err != nil {
		t.Fatalf("count Gemini settings: %v", err)
	}
	if configured != 0 {
		t.Fatalf("failed connection test persisted %d Gemini settings", configured)
	}
}

func TestAppAIExecutionAPIsReturnOnlySafeResponses(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	adapter := &appTestAIProviderAdapter{}
	aiService := aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	app := &App{ctx: t.Context(), aiService: aiService}
	secretMarker := "wails-execution-secret-marker"
	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     secretMarker,
		ModelID:    "openai/gpt-test",
	}); err != nil {
		t.Fatalf("configure AI provider: %v", err)
	}

	adapter.listErr = errors.New("raw list provider error " + secretMarker)
	models := app.ListAIModels(aiservice.ListModelsInput{ProviderID: aiservice.ProviderOpenRouter, APIKey: secretMarker})
	if models.Error == nil || models.Error.Code != aiservice.ErrorCodeProviderUnavailable || len(models.Models) != 0 {
		t.Fatalf("safe model list response = %#v", models)
	}
	serializedModels, err := json.Marshal(models)
	if err != nil {
		t.Fatalf("serialize model response: %v", err)
	}
	if strings.Contains(string(serializedModels), secretMarker) {
		t.Fatal("Wails model-list response exposed an API key or provider message")
	}

	adapter.summaryErr = errors.New("raw summary provider error " + secretMarker)
	summary := app.GenerateAISummary(aiservice.GenerateSummaryInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/gpt-test",
		Content:    "note-body-marker",
	})
	if summary.Error == nil || summary.Error.Code != aiservice.ErrorCodeProviderUnavailable || summary.Text != "" {
		t.Fatalf("safe summary response = %#v", summary)
	}
	serializedSummary, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("serialize summary response: %v", err)
	}
	if strings.Contains(string(serializedSummary), secretMarker) {
		t.Fatal("Wails summary response exposed an API key or provider message")
	}
}

func TestAppAIProviderSelectionAndModelUpdatePreserveCredential(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	adapter := &appTestAIProviderAdapter{}
	aiService := aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	app := &App{ctx: t.Context(), aiService: aiService}
	const openRouterKey = "wails-openrouter-selection-key-marker"
	const geminiKey = "wails-gemini-selection-key-marker"

	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     openRouterKey,
		ModelID:    "openrouter/initial-model",
	}); err != nil {
		t.Fatalf("configure OpenRouter: %v", err)
	}
	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderGemini,
		APIKey:     geminiKey,
		ModelID:    "gemini-2.5-flash",
	}); err != nil {
		t.Fatalf("configure Gemini: %v", err)
	}

	var beforeCredentialRef string
	if err := db.QueryRowContext(t.Context(), `SELECT credential_ref FROM ai_provider_settings WHERE provider_id = ?`, aiservice.ProviderOpenRouter).Scan(&beforeCredentialRef); err != nil {
		t.Fatalf("read OpenRouter credential reference before model update: %v", err)
	}
	beforeKey, err := aiService.GetCredential(t.Context(), aiservice.ProviderOpenRouter)
	if err != nil {
		t.Fatalf("read OpenRouter credential before model update: %v", err)
	}

	settings, err := app.UpdateAIProviderModel(aiservice.UpdateProviderModelInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openrouter/updated-model",
	})
	if err != nil {
		t.Fatalf("update OpenRouter model: %v", err)
	}
	var openRouterSetting aiservice.ProviderSettings
	var geminiSetting aiservice.ProviderSettings
	for _, setting := range settings {
		switch setting.ProviderID {
		case aiservice.ProviderOpenRouter:
			openRouterSetting = setting
		case aiservice.ProviderGemini:
			geminiSetting = setting
		}
	}
	if openRouterSetting.ModelID != "openrouter/updated-model" || !openRouterSetting.IsSelected {
		t.Fatalf("updated OpenRouter setting = %#v", openRouterSetting)
	}
	if geminiSetting.IsSelected {
		t.Fatalf("Gemini remained selected after OpenRouter model update: %#v", geminiSetting)
	}
	afterKey, err := aiService.GetCredential(t.Context(), aiservice.ProviderOpenRouter)
	if err != nil {
		t.Fatalf("read OpenRouter credential after model update: %v", err)
	}
	if afterKey != beforeKey || afterKey != openRouterKey {
		t.Fatal("model-only provider selection replaced the saved credential")
	}
	var afterCredentialRef string
	if err := db.QueryRowContext(t.Context(), `SELECT credential_ref FROM ai_provider_settings WHERE provider_id = ?`, aiservice.ProviderOpenRouter).Scan(&afterCredentialRef); err != nil {
		t.Fatalf("read OpenRouter credential reference after model update: %v", err)
	}
	if afterCredentialRef != beforeCredentialRef {
		t.Fatalf("model-only provider selection changed credential reference from %q to %q", beforeCredentialRef, afterCredentialRef)
	}
	var selectedCount int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM ai_provider_settings WHERE is_selected = 1`).Scan(&selectedCount); err != nil {
		t.Fatalf("count selected AI providers: %v", err)
	}
	if selectedCount != 1 {
		t.Fatalf("selected AI provider count = %d, want 1", selectedCount)
	}
	serialized, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("serialize provider settings: %v", err)
	}
	if strings.Contains(string(serialized), openRouterKey) || strings.Contains(string(serialized), geminiKey) {
		t.Fatal("provider selection response exposed an API key")
	}
}

func TestAppAIAssistantReturnsAgentProposalWithoutApplyingIt(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const secretMarker = "wails-agent-secret-marker"
	adapter := &appTestAIProviderAdapter{
		structuredResult: `{"message":"本文を簡潔にします。確認してから適用してください。","hasProposal":true,"reason":"重複を減らすため","before":"current body","after":"revised body"}`,
	}
	aiService := aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	aiService.SetNoteContextProvider(appTestAIContextProvider{notes: map[string]aiservice.ContextNote{
		"note-1": {NoteID: "note-1", Title: "Current", Content: "current body", Revision: 4},
	}})
	app := &App{ctx: t.Context(), aiService: aiService}
	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     secretMarker,
		ModelID:    "openai/agent-test",
	}); err != nil {
		t.Fatalf("configure AI provider: %v", err)
	}

	input := aiservice.AssistantInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/agent-test",
		Kind:       aiservice.AssistantKindQA,
		Mode:       aiservice.ChatModeAgent,
		Question:   "本文を短くして",
		NoteIDs:    []string{"note-1"},
		ExpectedSources: []aiservice.AIHistorySource{
			{NoteID: "note-1", InputRevision: 4},
		},
		AgentTarget: &aiservice.AgentEditTarget{NoteID: "note-1", BaseRevision: 4},
	}
	response := app.RunAIAssistant(input)
	if response.Error != nil || response.Result == nil || response.Result.Proposal == nil {
		t.Fatalf("safe Agent response = %#v", response)
	}
	proposal := response.Result.Proposal
	if proposal.TargetNoteID != "note-1" || proposal.TargetTitle != "Current" || proposal.BaseRevision != 4 || proposal.Before != "current body" || proposal.After != "revised body" {
		t.Fatalf("Agent proposal = %#v", proposal)
	}
	if adapter.structuredCalls != 1 || adapter.structuredInput.Name != "atlas_note_agent_edit" || adapter.structuredInput.MaxOutputTokens < 1 || len(adapter.structuredInput.Schema) == 0 {
		t.Fatalf("Agent structured request = calls:%d input:%#v", adapter.structuredCalls, adapter.structuredInput)
	}
	serialized, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("serialize Agent response: %v", err)
	}
	if strings.Contains(string(serialized), secretMarker) {
		t.Fatal("Agent response exposed an API key")
	}
	var histories int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_histories").Scan(&histories); err != nil {
		t.Fatalf("count Agent histories: %v", err)
	}
	if histories != 0 {
		t.Fatalf("Agent proposal unexpectedly persisted %d histories", histories)
	}

	input.ExpectedSources = nil
	rejected := app.RunAIAssistant(input)
	if rejected.Error == nil || rejected.Error.Code != aiservice.ErrorCodeInputInvalid || rejected.Result != nil {
		t.Fatalf("missing Agent expected sources response = %#v", rejected)
	}
	if adapter.structuredCalls != 1 {
		t.Fatalf("missing Agent expected sources made %d structured provider calls", adapter.structuredCalls)
	}
	serialized, err = json.Marshal(rejected)
	if err != nil {
		t.Fatalf("serialize rejected Agent response: %v", err)
	}
	if strings.Contains(string(serialized), secretMarker) {
		t.Fatal("rejected Agent response exposed an API key")
	}
}

func TestAppCancelAIAssistantReturnsSafeCancellationWithoutPersistence(t *testing.T) {
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const (
		secretMarker   = "wails-cancel-secret-marker"
		questionMarker = "wails-cancel-question-marker"
		requestID      = "wails-assistant-request-1"
	)
	started := make(chan struct{}, 1)
	adapter := &appTestAIProviderAdapter{
		textResult: aiservice.TextGenerationResult{Text: "must not complete"},
		started:    started,
		release:    make(chan struct{}),
	}
	aiService := aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	aiService.SetNoteContextProvider(appTestAIContextProvider{notes: map[string]aiservice.ContextNote{
		"note-1": {NoteID: "note-1", Title: "Current", Content: "current body", Revision: 4},
	}})
	app := &App{ctx: t.Context(), aiService: aiService}
	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     secretMarker,
		ModelID:    "openai/cancel-test",
	}); err != nil {
		t.Fatalf("configure AI provider: %v", err)
	}

	response := make(chan aiservice.AssistantResponse, 1)
	go func() {
		response <- app.RunAIAssistant(aiservice.AssistantInput{
			RequestID:  requestID,
			ProviderID: aiservice.ProviderOpenRouter,
			ModelID:    "openai/cancel-test",
			Kind:       aiservice.AssistantKindQA,
			Mode:       aiservice.ChatModeAsk,
			Question:   questionMarker,
			NoteIDs:    []string{"note-1"},
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Assistant request did not reach the Wails test adapter")
	}
	if canceled := app.CancelAIAssistant("wrong-request"); canceled.Error != nil || canceled.Canceled {
		t.Fatalf("mismatched Wails cancellation = %#v", canceled)
	}
	canceled := app.CancelAIAssistant(requestID)
	if canceled.Error != nil || !canceled.Canceled {
		t.Fatalf("matching Wails cancellation = %#v", canceled)
	}
	select {
	case result := <-response:
		if result.Error == nil || result.Error.Code != aiservice.ErrorCodeCancelled || result.Result != nil {
			t.Fatalf("canceled Wails Assistant response = %#v", result)
		}
		serialized, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("serialize canceled Assistant response: %v", err)
		}
		if strings.Contains(string(serialized), secretMarker) || strings.Contains(string(serialized), questionMarker) {
			t.Fatal("canceled Assistant response exposed request or credential content")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled Wails Assistant request did not finish")
	}
	if repeated := app.CancelAIAssistant(requestID); repeated.Error != nil || repeated.Canceled {
		t.Fatalf("terminal Wails cancellation = %#v", repeated)
	}
	var histories int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_histories").Scan(&histories); err != nil {
		t.Fatalf("count histories after cancellation: %v", err)
	}
	if histories != 0 {
		t.Fatalf("canceled Assistant persisted %d histories", histories)
	}

	unavailable := (&App{}).CancelAIAssistant(requestID)
	if unavailable.Error == nil || unavailable.Error.Code != aiservice.ErrorCodeConfigurationUnavailable || unavailable.Canceled {
		t.Fatalf("unavailable Wails cancellation = %#v", unavailable)
	}
}

func TestAppAIRecordLifecycleUsesLocalDatabaseWithoutChangingSyncState(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatal("test app is not ready")
	}

	created, err := app.CreateNote(note.CreateInput{
		Title:   "AI record source",
		Content: "local source content",
	})
	if err != nil {
		t.Fatalf("create AI record source note: %v", err)
	}

	captureSyncState := func() []string {
		return []string{
			appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_connections ORDER BY id"),
			appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_outbox ORDER BY sequence"),
			appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_item_states ORDER BY entity_key"),
			appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_snapshots ORDER BY snapshot_id"),
			appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_conflicts ORDER BY conflict_id"),
		}
	}

	syncBeforeSave := captureSyncState()
	historyResponse := app.SaveAIHistory(aiservice.SaveAIHistoryInput{
		Kind:       aiservice.AssistantKindQA,
		Title:      "Saved local Q&A",
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/local-test",
		Messages: []aiservice.AIConversationMessage{
			{Role: "user", Content: "Local question"},
			{Role: "assistant", Content: "Local answer"},
		},
		Sources: []aiservice.AIHistorySource{{NoteID: created.ID, InputRevision: created.Revision}},
	})
	if historyResponse.Error != nil || historyResponse.History == nil || historyResponse.History.ID == "" {
		t.Fatalf("save AI history through App API = %#v", historyResponse)
	}
	historyID := historyResponse.History.ID

	artifactResponse := app.SaveAIArtifact(aiservice.SaveAIArtifactInput{
		Kind:       aiservice.ArtifactKindDocument,
		Title:      "Saved local document",
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/local-test",
		Content:    "Local generated document",
		Sources:    []aiservice.AIHistorySource{{NoteID: created.ID, InputRevision: created.Revision}},
	})
	if artifactResponse.Error != nil || artifactResponse.Artifact == nil || artifactResponse.Artifact.ID == "" {
		t.Fatalf("save AI artifact through App API = %#v", artifactResponse)
	}
	artifactID := artifactResponse.Artifact.ID

	histories := app.ListAIHistories()
	if histories.Error != nil || len(histories.Items) != 1 || histories.Items[0].ID != historyID {
		t.Fatalf("list AI histories through App API = %#v", histories)
	}
	artifacts := app.ListAIArtifacts()
	if artifacts.Error != nil || len(artifacts.Items) != 1 || artifacts.Items[0].ID != artifactID {
		t.Fatalf("list AI artifacts through App API = %#v", artifacts)
	}
	if fetched := app.GetAIHistory(historyID); fetched.Error != nil || fetched.History == nil || fetched.History.Status != aiservice.AIRecordStatusSaved {
		t.Fatalf("get saved AI history through App API = %#v", fetched)
	}
	if fetched := app.GetAIArtifact(artifactID); fetched.Error != nil || fetched.Artifact == nil || fetched.Artifact.Status != aiservice.AIRecordStatusSaved {
		t.Fatalf("get saved AI artifact through App API = %#v", fetched)
	}
	if syncAfterSave := captureSyncState(); !reflect.DeepEqual(syncAfterSave, syncBeforeSave) {
		t.Fatal("saving or reading local AI records changed WebDAV sync state")
	}

	nextContent := "local source content after AI save"
	expectedRevision := created.Revision
	updatedResult, err := app.UpdateNote(created.ID, note.UpdateInput{
		Content:          &nextContent,
		ExpectedRevision: &expectedRevision,
	})
	if err != nil || updatedResult.Note == nil || updatedResult.Conflict != nil {
		t.Fatalf("update source note after AI save = %#v, %v", updatedResult, err)
	}
	updated := updatedResult.Note
	if fetched := app.GetAIHistory(historyID); fetched.Error != nil || fetched.History == nil || fetched.History.Status != aiservice.AIRecordStatusStale {
		t.Fatalf("get stale AI history through App API = %#v", fetched)
	}
	if fetched := app.GetAIArtifact(artifactID); fetched.Error != nil || fetched.Artifact == nil || fetched.Artifact.Status != aiservice.AIRecordStatusStale {
		t.Fatalf("get stale AI artifact through App API = %#v", fetched)
	}

	deletedNote, err := app.DeleteNote(created.ID, note.DeleteInput{ExpectedRevision: updated.Revision})
	if err != nil || !deletedNote.Deleted || deletedNote.Conflict != nil {
		t.Fatalf("delete source note for orphaned AI records = %#v, %v", deletedNote, err)
	}
	if fetched := app.GetAIHistory(historyID); fetched.Error != nil || fetched.History == nil || fetched.History.Status != aiservice.AIRecordStatusOrphaned {
		t.Fatalf("get orphaned AI history through App API = %#v", fetched)
	}
	if fetched := app.GetAIArtifact(artifactID); fetched.Error != nil || fetched.Artifact == nil || fetched.Artifact.Status != aiservice.AIRecordStatusOrphaned {
		t.Fatalf("get orphaned AI artifact through App API = %#v", fetched)
	}

	syncBeforeDelete := captureSyncState()
	if deleted := app.DeleteAIHistory(historyID); deleted.Error != nil || !deleted.Deleted {
		t.Fatalf("delete AI history through App API = %#v", deleted)
	}
	if deleted := app.DeleteAIArtifact(artifactID); deleted.Error != nil || !deleted.Deleted {
		t.Fatalf("delete AI artifact through App API = %#v", deleted)
	}
	if missing := app.GetAIHistory(historyID); missing.Error == nil || missing.Error.Code != aiservice.ErrorCodeHistoryNotFound || missing.History != nil {
		t.Fatalf("get deleted AI history through App API = %#v", missing)
	}
	if missing := app.GetAIArtifact(artifactID); missing.Error == nil || missing.Error.Code != aiservice.ErrorCodeArtifactNotFound || missing.Artifact != nil {
		t.Fatalf("get deleted AI artifact through App API = %#v", missing)
	}

	secondHistory := app.SaveAIHistory(aiservice.SaveAIHistoryInput{
		Kind:       aiservice.AssistantKindBrainstorm,
		Title:      "Second local history",
		ProviderID: aiservice.ProviderGemini,
		ModelID:    "gemini-local-test",
		Messages: []aiservice.AIConversationMessage{
			{Role: "user", Content: "Second question"},
			{Role: "assistant", Content: "Second answer"},
		},
	})
	if secondHistory.Error != nil || secondHistory.History == nil {
		t.Fatalf("save source-free AI history through App API = %#v", secondHistory)
	}
	secondArtifact := app.SaveAIArtifact(aiservice.SaveAIArtifactInput{
		Kind:       aiservice.ArtifactKindREADME,
		Title:      "Second local artifact",
		ProviderID: aiservice.ProviderGemini,
		ModelID:    "gemini-local-test",
		Content:    "Second local content",
	})
	if secondArtifact.Error != nil || secondArtifact.Artifact == nil {
		t.Fatalf("save source-free AI artifact through App API = %#v", secondArtifact)
	}
	if deleted := app.DeleteAllAIHistories(); deleted.Error != nil || !deleted.Deleted {
		t.Fatalf("delete all AI histories through App API = %#v", deleted)
	}
	if deleted := app.DeleteAllAIArtifacts(); deleted.Error != nil || !deleted.Deleted {
		t.Fatalf("delete all AI artifacts through App API = %#v", deleted)
	}
	if listed := app.ListAIHistories(); listed.Error != nil || len(listed.Items) != 0 {
		t.Fatalf("AI histories after delete all = %#v", listed)
	}
	if listed := app.ListAIArtifacts(); listed.Error != nil || len(listed.Items) != 0 {
		t.Fatalf("AI artifacts after delete all = %#v", listed)
	}
	if syncAfterDelete := captureSyncState(); !reflect.DeepEqual(syncAfterDelete, syncBeforeDelete) {
		t.Fatal("deleting local AI records changed WebDAV sync state")
	}
}

func TestAppLibrarianExecutionKeepsV2OutputTransient(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatal("test app is not ready")
	}

	const candidateMarker = "v2-librarian-candidate-marker"
	const chunkMarker = "v2-librarian-partial-marker"
	const promptMarker = "v2-librarian-prompt-marker"
	const resultMarker = "v2-librarian-result-marker"
	adapter := &appTestAIProviderAdapter{
		structuredResult: `{"candidates":[{"noteId":"` + candidateMarker + `","score":0.91,"reason":"` + resultMarker + `"}]}`,
		structuredChunks: []string{chunkMarker},
	}
	app.aiService.Shutdown()
	app.aiService = aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(app.db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	app.aiService.SetNoteContextProvider(aiservice.NewNoteContextProvider(app.notes))

	created, err := app.CreateNote(note.CreateInput{
		Title:   "D-07 librarian invariant",
		Content: "d07 stable librarian baseline",
	})
	if err != nil {
		t.Fatalf("create librarian invariant note: %v", err)
	}
	if _, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     "v2-librarian-session-only-key",
		ModelID:    "openai/v2-librarian-test",
	}); err != nil {
		t.Fatalf("configure librarian invariant provider: %v", err)
	}

	markdownPath := filepath.Join(dataDir, "notes", created.ID+".md")
	before := captureAppTestAIInvariantSnapshot(t, app, created.ID, markdownPath)
	events := make(chan aiservice.LibrarianEvent, 4)
	started, err := app.aiService.StartLibrarian(t.Context(), aiservice.LibrarianInput{
		ProviderID:     aiservice.ProviderOpenRouter,
		ModelID:        "openai/v2-librarian-test",
		Operation:      aiservice.LibrarianOperationRelated,
		NoteID:         created.ID,
		BaseRevision:   created.Revision,
		Title:          created.Title,
		Content:        created.Content,
		CandidateCount: 1,
		Candidates: []aiservice.LibrarianCandidateContext{{
			NoteID:  candidateMarker,
			Title:   "Transient candidate",
			Snippet: promptMarker,
		}},
	}, func(event aiservice.LibrarianEvent) {
		events <- event
	})
	if err != nil || started.RequestID == "" {
		t.Fatalf("start librarian invariant execution = %#v, %v", started, err)
	}

	partial := appTestReceiveLibrarianEvent(t, events)
	if partial.RequestID != started.RequestID || partial.Phase != "partial" || partial.PartialText != chunkMarker {
		t.Fatalf("librarian partial event = %#v", partial)
	}
	completed := appTestReceiveLibrarianEvent(t, events)
	if completed.RequestID != started.RequestID || completed.Phase != "completed" || completed.Result == nil || len(completed.Result.Candidates) != 1 {
		t.Fatalf("librarian completed event = %#v", completed)
	}
	if completed.Result.Candidates[0].NoteID != candidateMarker || completed.Result.Candidates[0].Reason != resultMarker {
		t.Fatalf("librarian transient result = %#v", completed.Result)
	}
	if adapter.structuredCalls != 1 || !strings.Contains(adapter.structuredInput.Prompt, promptMarker) || !strings.Contains(adapter.structuredInput.Prompt, candidateMarker) {
		t.Fatalf("librarian structured input = calls:%d input:%#v", adapter.structuredCalls, adapter.structuredInput)
	}

	after := captureAppTestAIInvariantSnapshot(t, app, created.ID, markdownPath)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("librarian execution changed a v2 persistence boundary\nbefore: %#v\nafter:  %#v", before, after)
	}
	for _, marker := range []string{candidateMarker, chunkMarker, promptMarker, resultMarker} {
		if appTestDatabaseContainsMarker(t, app.db, marker) {
			t.Fatalf("librarian marker %q was persisted in SQLite", marker)
		}
		if path := appTestFindMarkerInDataFiles(t, dataDir, marker); path != "" {
			t.Fatalf("librarian marker %q was persisted in application data: %s", marker, path)
		}
	}
}

func appTestReceiveLibrarianEvent(t *testing.T, events <-chan aiservice.LibrarianEvent) aiservice.LibrarianEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for librarian event")
		return aiservice.LibrarianEvent{}
	}
}

func TestAppAIConfigurationAndSummaryPreserveLocalAndSyncArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatal("test app is not ready")
	}

	const credentialMarker = "d07-synthetic-credential-marker"
	const summaryMarker = "d07-synthetic-summary-marker"
	const providerErrorMarker = "d07-synthetic-provider-error-marker"
	adapter := &appTestAIProviderAdapter{
		summaryResult: aiservice.SummaryResult{Text: summaryMarker},
	}
	app.aiService.Shutdown()
	app.aiService = aiservice.NewServiceWithAdapter(
		aiservice.NewRepository(app.db),
		credential.NewManager(credential.NewSessionStore()),
		adapter,
	)
	app.aiService.SetNoteContextProvider(aiservice.NewNoteContextProvider(app.notes))

	syncRepository := syncservice.NewRepository(app.db)
	if err := syncRepository.SaveConnection(t.Context(), syncservice.Connection{
		Endpoint:         "https://sync.invalid",
		RemoteRoot:       "/d07",
		Username:         "synthetic-user",
		VaultID:          strings.Repeat("a", 32),
		HeadManifestHash: strings.Repeat("b", 64),
		HeadETag:         `"d07-etag"`,
		Status:           syncservice.StatusSynced,
		FailSafe:         true,
		CredentialRef:    "d07-sync-reference",
	}); err != nil {
		t.Fatalf("seed sync connection: %v", err)
	}
	created, err := app.CreateNote(note.CreateInput{
		Title:   "D-07 local note",
		Content: "d07 stable local note content",
	})
	if err != nil {
		t.Fatalf("create local note: %v", err)
	}
	tagKey := note.SyncEntityKey(note.SyncEntityTag, strings.Repeat("c", 32))
	if err := syncRepository.MarkItemRemote(t.Context(), tagKey, note.SyncEntityTag, strings.Repeat("d", 64), `{"tag":"d07"}`); err != nil {
		t.Fatalf("seed sync snapshot: %v", err)
	}
	if err := syncRepository.CreateConflict(t.Context(), syncservice.Conflict{
		ID:               strings.Repeat("e", 32),
		EntityKey:        tagKey,
		EntityType:       note.SyncEntityTag,
		LocalObjectHash:  strings.Repeat("f", 64),
		BaseObjectHash:   strings.Repeat("1", 64),
		RemoteObjectHash: strings.Repeat("2", 64),
		LocalSnapshot:    `{"tag":"local"}`,
		BaseSnapshot:     `{"tag":"base"}`,
		RemoteSnapshot:   `{"tag":"remote"}`,
		ConflictType:     "both-changed",
		ResolutionStatus: "open",
	}); err != nil {
		t.Fatalf("seed sync conflict: %v", err)
	}

	markdownPath := filepath.Join(dataDir, "notes", created.ID+".md")
	before := captureAppTestAIInvariantSnapshot(t, app, created.ID, markdownPath)
	if before.Note.Content != created.Content || before.Note.Revision != created.Revision || !bytes.Equal(before.Markdown, []byte(created.Content)) {
		t.Fatal("local note fixture is not stable before AI operations")
	}
	if before.SyncStatus.Status != syncservice.StatusConflict || before.SyncStatus.OutboxCount == 0 || before.SyncStatus.ConflictCount != 1 || len(before.Conflicts) != 1 {
		t.Fatal("sync artifacts were not seeded before AI operations")
	}

	settings, err := app.ConfigureAIProvider(aiservice.ConfigureProviderInput{
		ProviderID: aiservice.ProviderOpenRouter,
		APIKey:     credentialMarker,
		ModelID:    "openai/d07-summary-model",
	})
	if err != nil {
		t.Fatalf("configure AI provider: %v", err)
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("serialize configured AI settings: %v", err)
	}
	if strings.Contains(string(settingsJSON), credentialMarker) || strings.Contains(string(settingsJSON), summaryMarker) || strings.Contains(string(settingsJSON), providerErrorMarker) {
		t.Fatal("AI settings response exposed a synthetic marker")
	}

	success := app.GenerateAISummary(aiservice.GenerateSummaryInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/d07-summary-model",
		Content:    before.Note.Content,
	})
	if success.Error != nil || success.Text != summaryMarker {
		t.Fatal("successful AI summary was not returned")
	}

	adapter.summaryErr = errors.New("provider failure " + providerErrorMarker)
	failure := app.GenerateAISummary(aiservice.GenerateSummaryInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/d07-summary-model",
		Content:    before.Note.Content,
	})
	if failure.Error == nil || failure.Error.Code != aiservice.ErrorCodeProviderUnavailable || failure.Text != "" {
		t.Fatal("failed AI summary did not return a safe error")
	}
	failureJSON, err := json.Marshal(failure)
	if err != nil {
		t.Fatalf("serialize failed AI summary: %v", err)
	}
	if strings.Contains(string(failureJSON), credentialMarker) || strings.Contains(string(failureJSON), summaryMarker) || strings.Contains(string(failureJSON), providerErrorMarker) {
		t.Fatal("failed AI summary exposed a synthetic marker")
	}

	adapter.textErr = errors.New("assistant and writing provider failure " + providerErrorMarker)
	assistantFailure := app.RunAIAssistant(aiservice.AssistantInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/d07-summary-model",
		Kind:       aiservice.AssistantKindQA,
		Mode:       aiservice.ChatModeAsk,
		Question:   "Can local features continue?",
		NoteIDs:    []string{created.ID},
	})
	if assistantFailure.Error == nil || assistantFailure.Error.Code != aiservice.ErrorCodeProviderUnavailable || assistantFailure.Result != nil {
		t.Fatalf("failed AI assistant response = %#v", assistantFailure)
	}
	writingFailure := app.RunAIWriting(aiservice.WritingInput{
		ProviderID:  aiservice.ProviderOpenRouter,
		ModelID:     "openai/d07-summary-model",
		Kind:        aiservice.WritingKindDocument,
		Instruction: "Create a local-only failure fixture",
		NoteIDs:     []string{created.ID},
	})
	if writingFailure.Error == nil || writingFailure.Error.Code != aiservice.ErrorCodeProviderUnavailable || writingFailure.Result != nil {
		t.Fatalf("failed AI writing response = %#v", writingFailure)
	}

	adapter.structuredErr = errors.New("agent provider failure " + providerErrorMarker)
	agentFailure := app.RunAIAssistant(aiservice.AssistantInput{
		ProviderID: aiservice.ProviderOpenRouter,
		ModelID:    "openai/d07-summary-model",
		Kind:       aiservice.AssistantKindQA,
		Mode:       aiservice.ChatModeAgent,
		Question:   "Change the note",
		NoteIDs:    []string{created.ID},
		ExpectedSources: []aiservice.AIHistorySource{
			{NoteID: created.ID, InputRevision: before.Note.Revision},
		},
		AgentTarget: &aiservice.AgentEditTarget{NoteID: created.ID, BaseRevision: before.Note.Revision},
	})
	if agentFailure.Error == nil || agentFailure.Error.Code != aiservice.ErrorCodeProviderUnavailable || agentFailure.Result != nil {
		t.Fatalf("failed AI Agent response = %#v", agentFailure)
	}
	for name, response := range map[string]any{
		"assistant": assistantFailure,
		"writing":   writingFailure,
		"agent":     agentFailure,
	} {
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("serialize failed AI %s response: %v", name, err)
		}
		if strings.Contains(string(encoded), credentialMarker) || strings.Contains(string(encoded), providerErrorMarker) {
			t.Fatalf("failed AI %s response exposed a synthetic marker", name)
		}
	}

	after := captureAppTestAIInvariantSnapshot(t, app, created.ID, markdownPath)
	if after.Note.Content != before.Note.Content || after.Note.Revision != before.Note.Revision || !bytes.Equal(after.Markdown, before.Markdown) {
		t.Fatal("AI operations changed the local note or Markdown")
	}
	if !reflect.DeepEqual(after.Search, before.Search) || !reflect.DeepEqual(after.SyncStatus, before.SyncStatus) || !reflect.DeepEqual(after.Conflicts, before.Conflicts) {
		t.Fatal("AI operations changed normal search or sync API results")
	}
	if after.StorageOperations != before.StorageOperations || after.SyncConnection != before.SyncConnection || after.SyncOutbox != before.SyncOutbox || after.SyncItemStates != before.SyncItemStates || after.SyncSnapshots != before.SyncSnapshots || after.SyncConflicts != before.SyncConflicts {
		t.Fatal("AI operations changed operation journal or sync artifacts")
	}

	for _, marker := range []string{credentialMarker, summaryMarker, providerErrorMarker} {
		if appTestDatabaseContainsMarker(t, app.db, marker) {
			t.Fatal("AI marker was persisted in the database")
		}
		if path := appTestFindMarkerInDataFiles(t, dataDir, marker); path != "" {
			t.Fatalf("AI marker was persisted in an application data file: %s", path)
		}
	}
	currentSettings, err := app.GetAISettings()
	if err != nil {
		t.Fatalf("get current AI settings: %v", err)
	}
	currentSettingsJSON, err := json.Marshal(currentSettings)
	if err != nil {
		t.Fatalf("serialize current AI settings: %v", err)
	}
	if strings.Contains(string(currentSettingsJSON), credentialMarker) || strings.Contains(string(currentSettingsJSON), summaryMarker) || strings.Contains(string(currentSettingsJSON), providerErrorMarker) {
		t.Fatal("persisted AI settings exposed a synthetic marker")
	}

	continuedContent := "d07 local continuation after provider failure"
	expectedRevision := after.Note.Revision
	continuedResult, err := app.UpdateNote(created.ID, note.UpdateInput{
		Content:          &continuedContent,
		ExpectedRevision: &expectedRevision,
	})
	if err != nil || continuedResult.Note == nil || continuedResult.Conflict != nil {
		t.Fatalf("update local note after AI failure = %#v, %v", continuedResult, err)
	}
	if continuedResult.Note.Content != continuedContent || continuedResult.Note.Revision != expectedRevision+1 {
		t.Fatalf("local note after AI failure = %#v", continuedResult.Note)
	}
	continuedMarkdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read Markdown after AI failure: %v", err)
	}
	if !bytes.Equal(continuedMarkdown, []byte(continuedContent)) {
		t.Fatalf("Markdown after AI failure = %q, want %q", continuedMarkdown, continuedContent)
	}
	continuedSearch, err := app.SearchNotes(note.SearchInput{Query: "continuation"})
	if err != nil || continuedSearch.Error != nil || len(continuedSearch.Items) != 1 || continuedSearch.Items[0].Note.ID != created.ID {
		t.Fatalf("search after AI failure = %#v, %v", continuedSearch, err)
	}
	continuedSyncStatus, err := app.GetSyncStatus()
	if err != nil || continuedSyncStatus.OutboxCount == 0 {
		t.Fatalf("sync status after AI failure = %#v, %v", continuedSyncStatus, err)
	}
	continuedSyncOutbox := appTestRowsSnapshot(t, app.db, "SELECT * FROM sync_outbox ORDER BY sequence")
	if continuedSyncOutbox == after.SyncOutbox {
		t.Fatal("local update after AI failure did not advance the normal sync outbox")
	}
}

func TestGetStartupStatusError(t *testing.T) {
	app := &App{
		dataDir:    "C:\\AtlasNote",
		startupErr: errors.New("create markdown directory: access denied"),
	}

	status := app.GetStartupStatus()

	if status.Ready {
		t.Fatal("expected startup status to be not ready")
	}
	if status.Message != "create markdown directory: access denied" {
		t.Fatalf("message = %q", status.Message)
	}
	if status.DataDir != "C:\\AtlasNote" {
		t.Fatalf("data dir = %q", status.DataDir)
	}
}

func TestAppReturnsStructuredRevisionConflict(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(context.Background())
	t.Cleanup(func() {
		app.shutdown(t.Context())
	})
	created, err := app.CreateNote(note.CreateInput{Title: "Original", Content: "original content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}

	updatedTitle := "Updated"
	expectedRevision := created.Revision
	updatedResult, err := app.UpdateNote(created.ID, note.UpdateInput{
		Title:            &updatedTitle,
		ExpectedRevision: &expectedRevision,
	})
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updatedResult.Note == nil || updatedResult.Conflict != nil {
		t.Fatalf("update result = %#v", updatedResult)
	}
	if updatedResult.Note.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updatedResult.Note.Revision)
	}

	staleTitle := "Stale overwrite"
	conflictResult, err := app.UpdateNote(created.ID, note.UpdateInput{
		Title:            &staleTitle,
		ExpectedRevision: &expectedRevision,
	})
	if err != nil {
		t.Fatalf("stale update returned system error: %v", err)
	}
	if conflictResult.Note != nil || conflictResult.Conflict == nil {
		t.Fatalf("conflict result = %#v", conflictResult)
	}
	conflict := conflictResult.Conflict
	if conflict.Code != note.ErrorCodeRevisionConflict ||
		conflict.NoteID != created.ID ||
		conflict.ExpectedRevision != 1 ||
		conflict.ActualRevision != 2 {
		t.Fatalf("update conflict = %#v", conflict)
	}

	deleteConflict, err := app.DeleteNote(created.ID, note.DeleteInput{ExpectedRevision: 1})
	if err != nil {
		t.Fatalf("stale delete returned system error: %v", err)
	}
	if deleteConflict.Deleted || deleteConflict.Conflict == nil {
		t.Fatalf("delete conflict result = %#v", deleteConflict)
	}

	deletedResult, err := app.DeleteNote(created.ID, note.DeleteInput{ExpectedRevision: 2})
	if err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if !deletedResult.Deleted || deletedResult.Conflict != nil {
		t.Fatalf("delete result = %#v", deletedResult)
	}
}

func TestAppSearchNotesReturnsStructuredValidationError(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(context.Background())
	t.Cleanup(func() {
		app.shutdown(t.Context())
	})

	result, err := app.SearchNotes(note.SearchInput{Query: "ok\x00"})
	if err != nil {
		t.Fatalf("search notes returned system error: %v", err)
	}
	if result.Error == nil || result.Error.Code != note.SearchErrorQueryInvalid {
		t.Fatalf("search result = %#v", result)
	}
}

func TestAppTagOperationsReturnStructuredErrors(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(context.Background())
	t.Cleanup(func() {
		app.shutdown(t.Context())
	})

	emptyResult, err := app.CreateTag(note.TagCreateInput{Name: "\u00a0\u2003"})
	if err != nil {
		t.Fatalf("create empty tag: %v", err)
	}
	if emptyResult.Error == nil || emptyResult.Error.Code != note.TagErrorNameEmpty {
		t.Fatalf("empty tag result = %#v", emptyResult)
	}

	createdResult, err := app.CreateTag(note.TagCreateInput{Name: "Project"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if createdResult.Error != nil || createdResult.Tag == nil {
		t.Fatalf("create tag result = %#v", createdResult)
	}

	conflictResult, err := app.CreateTag(note.TagCreateInput{Name: "project"})
	if err != nil {
		t.Fatalf("create duplicate tag: %v", err)
	}
	if conflictResult.Error == nil || conflictResult.Error.Code != note.TagErrorNameConflict {
		t.Fatalf("duplicate tag result = %#v", conflictResult)
	}

	setResult, err := app.SetNoteTags("missing-note", note.SetNoteTagsInput{TagIDs: []string{createdResult.Tag.ID}})
	if err != nil {
		t.Fatalf("set missing note tags: %v", err)
	}
	if setResult.Error == nil || setResult.Error.Code != note.TagErrorNoteNotFound {
		t.Fatalf("missing note tag result = %#v", setResult)
	}
}

func TestNewAppReportsInitializationError(t *testing.T) {
	tempDir := t.TempDir()
	blockedDataDir := filepath.Join(tempDir, "blocked")
	if err := os.WriteFile(blockedDataDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create blocked data dir file: %v", err)
	}
	t.Setenv("ATLAS_NOTE_DATA_DIR", blockedDataDir)

	app := NewApp()
	t.Cleanup(func() {
		app.shutdown(t.Context())
	})

	status := app.GetStartupStatus()
	if status.Ready {
		t.Fatal("expected startup status to be not ready")
	}
	if status.Message == "" {
		t.Fatal("expected startup error message")
	}
	if status.DataDir != blockedDataDir {
		t.Fatalf("data dir = %q", status.DataDir)
	}
}

func TestNewAppRejectsSecondWriterForSameDataDirectory(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	first := NewApp()
	t.Cleanup(func() { first.shutdown(t.Context()) })
	if status := first.GetStartupStatus(); !status.Ready {
		t.Fatalf("first app is not ready: %s", status.Message)
	}
	created, err := first.notes.Create(context.Background(), note.CreateInput{
		Title:   "Current",
		Content: "new markdown content",
	})
	if err != nil {
		t.Fatalf("create note with first app: %v", err)
	}
	markdownPath := filepath.Join(dataDir, "notes", created.ID+".md")
	before, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown before second app: %v", err)
	}

	second := NewApp()
	t.Cleanup(func() { second.shutdown(t.Context()) })
	status := second.GetStartupStatus()
	if status.Ready {
		t.Fatal("expected second app to be rejected")
	}
	if !strings.Contains(status.Message, datalock.ErrAlreadyLocked.Error()) {
		t.Fatalf("startup message = %q", status.Message)
	}
	if second.notes != nil || second.db != nil {
		t.Fatal("second app initialized writer resources")
	}
	after, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown after second app: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("markdown changed after rejected startup: before %q, after %q", before, after)
	}
}

func TestNewAppCanAcquireDataDirectoryAfterShutdown(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	first := NewApp()
	if status := first.GetStartupStatus(); !status.Ready {
		t.Fatalf("first app is not ready: %s", status.Message)
	}
	first.shutdown(t.Context())

	second := NewApp()
	t.Cleanup(func() { second.shutdown(t.Context()) })
	if status := second.GetStartupStatus(); !status.Ready {
		t.Fatalf("second app is not ready after shutdown: %s", status.Message)
	}
}

func TestAppReportsDegradedRecoveryAndKeepsHealthyNotesAvailable(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	missing, err := app.CreateNote(note.CreateInput{Title: "Missing", Content: "missing content"})
	if err != nil {
		t.Fatalf("create missing note: %v", err)
	}
	healthy, err := app.CreateNote(note.CreateInput{Title: "Healthy", Content: "healthy content"})
	if err != nil {
		t.Fatalf("create healthy note: %v", err)
	}
	missingPath := filepath.Join(dataDir, "notes", missing.ID+".md")
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove markdown fixture: %v", err)
	}

	status, err := app.ReinspectRecovery()
	if err != nil {
		t.Fatalf("reinspect recovery: %v", err)
	}
	if !status.Ready || !status.Degraded {
		t.Fatalf("startup status = %#v", status)
	}
	if len(status.MissingNotes) != 1 || status.MissingNotes[0].ID != missing.ID {
		t.Fatalf("missing notes = %#v", status.MissingNotes)
	}
	if status.MissingNotes[0].FilePath != missingPath {
		t.Fatalf("missing path = %q", status.MissingNotes[0].FilePath)
	}
	got, err := app.GetNote(healthy.ID)
	if err != nil || got.Content != "healthy content" {
		t.Fatalf("healthy note = %#v, %v", got, err)
	}

	if err := os.WriteFile(missingPath, []byte("restored content"), 0o600); err != nil {
		t.Fatalf("restore markdown fixture: %v", err)
	}
	status, err = app.ReinspectRecovery()
	if err != nil {
		t.Fatalf("reinspect restored content: %v", err)
	}
	if status.Degraded || len(status.MissingNotes) != 0 {
		t.Fatalf("status remains degraded = %#v", status)
	}
}

func TestAppDeleteMissingNoteRequiresExplicitRecoveryAPI(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	created, err := app.CreateNote(note.CreateInput{Title: "Missing", Content: "content"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := os.Remove(filepath.Join(dataDir, "notes", created.ID+".md")); err != nil {
		t.Fatalf("remove markdown fixture: %v", err)
	}
	if _, err := app.ReinspectRecovery(); err != nil {
		t.Fatalf("reinspect recovery: %v", err)
	}

	status, err := app.DeleteMissingNote(created.ID)
	if err != nil {
		t.Fatalf("delete missing note: %v", err)
	}
	if status.Degraded || len(status.MissingNotes) != 0 {
		t.Fatalf("status remains degraded = %#v", status)
	}
	if _, err := app.GetNote(created.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("deleted note is still available: %v", err)
	}
}

func TestCancelCloseClearsPendingCloseRequest(t *testing.T) {
	app := &App{closeRequested: true}

	app.CancelClose()

	if app.closeRequested {
		t.Fatal("expected pending close request to be cleared")
	}
	if app.allowClose {
		t.Fatal("expected close to remain blocked")
	}
}

func TestCompleteCloseAllowsNextCloseRequest(t *testing.T) {
	app := &App{closeRequested: true}

	app.CompleteClose()

	if !app.allowClose {
		t.Fatal("expected close to be allowed")
	}
	if app.closeRequested {
		t.Fatal("expected pending close request to be cleared")
	}
}
