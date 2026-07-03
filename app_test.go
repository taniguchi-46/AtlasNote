package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
