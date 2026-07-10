package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/datalock"
	"atlasnote/internal/note"
)

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
