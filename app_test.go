package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aiservice "atlasnote/internal/ai"
	"atlasnote/internal/credential"
	"atlasnote/internal/database"
	"atlasnote/internal/datalock"
	"atlasnote/internal/note"
)

type appTestAIConnectionChecker struct {
	err error
}

func (c *appTestAIConnectionChecker) Check(context.Context, aiservice.ProviderID, string) error {
	return c.err
}

type appTestAIProviderAdapter struct {
	listResult    aiservice.ModelListResult
	listErr       error
	summaryResult aiservice.SummaryResult
	summaryErr    error
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
