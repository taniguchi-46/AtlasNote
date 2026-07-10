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
