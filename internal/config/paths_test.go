package config

import (
	"path/filepath"
	"testing"
)

func TestLoadPathsIncludesDataDirectoryLock(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv(dataDirEnv, dataDir)

	paths, err := LoadPaths()
	if err != nil {
		t.Fatalf("load paths: %v", err)
	}
	if paths.LockPath != filepath.Join(dataDir, "atlasnote.lock") {
		t.Fatalf("lock path = %q", paths.LockPath)
	}
}

func TestPathsForDataDirKeepsAllStorageInsideSelectedSpace(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "spaces", "0123456789abcdef0123456789abcdef")

	paths := PathsForDataDir(dataDir)

	if paths.DataDir != filepath.Clean(dataDir) {
		t.Fatalf("data dir = %q", paths.DataDir)
	}
	if paths.DatabasePath != filepath.Join(dataDir, "atlasnote.db") {
		t.Fatalf("database path = %q", paths.DatabasePath)
	}
	if paths.NotesDir != filepath.Join(dataDir, "notes") {
		t.Fatalf("notes path = %q", paths.NotesDir)
	}
	if paths.LockPath != filepath.Join(dataDir, "atlasnote.lock") {
		t.Fatalf("lock path = %q", paths.LockPath)
	}
}
