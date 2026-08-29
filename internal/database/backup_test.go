package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOnlineBackupProducesReadableSnapshot(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.db")
	source, err := Open(t.Context(), sourcePath)
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}
	if _, err := source.ExecContext(t.Context(), "CREATE TABLE backup_test (value TEXT)"); err != nil {
		_ = source.Close()
		t.Fatalf("write source database: %v", err)
	}
	destinationPath := filepath.Join(root, "snapshot", "atlasnote.db")
	if err := OnlineBackup(t.Context(), source, destinationPath); err != nil {
		_ = source.Close()
		t.Fatalf("create online backup: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close source database: %v", err)
	}

	info, err := ValidateSnapshot(t.Context(), destinationPath)
	if err != nil {
		t.Fatalf("validate online backup: %v", err)
	}
	if info.SchemaVersion != len(migrations) {
		t.Fatalf("snapshot schema version = %d, want %d", info.SchemaVersion, len(migrations))
	}
}

func TestValidateSnapshotDoesNotCreateMissingFile(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "missing.db")
	if _, err := ValidateSnapshot(t.Context(), databasePath); err == nil {
		t.Fatal("missing snapshot was accepted")
	}
	if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing snapshot was created: %v", err)
	}
}
