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
