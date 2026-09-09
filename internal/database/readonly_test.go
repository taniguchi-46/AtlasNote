package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReadOnlyDoesNotCreateMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "atlasnote.db")
	if _, err := OpenReadOnly(context.Background(), path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open missing database error = %v, want not-exist", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only open created database: %v", err)
	}
}

func TestOpenReadOnlyDoesNotMigrateNewerDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "atlasnote.db")
	fixture, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	newerVersion := len(migrations) + 1
	if _, err := fixture.ExecContext(context.Background(), fmt.Sprintf("PRAGMA user_version = %d", newerVersion)); err != nil {
		_ = fixture.Close()
		t.Fatalf("set fixture schema version: %v", err)
	}
	if err := fixture.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}

	readonly, err := OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatalf("open newer database read-only: %v", err)
	}
	defer readonly.Close()
	var version int
	if err := readonly.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != newerVersion {
		t.Fatalf("schema version = %d, want %d", version, newerVersion)
	}
}

func TestOpenReadOnlyReadsCommittedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "atlasnote.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open writable fixture database: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), "INSERT INTO notebooks(id, parent_id, name, icon, created_at, updated_at) VALUES (?, NULL, ?, ?, ?, ?)", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "WAL fixture", "default:note", "now", "now"); err != nil {
		t.Fatalf("write WAL fixture: %v", err)
	}

	readonly, err := OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatalf("open WAL database read-only: %v", err)
	}
	defer readonly.Close()
	var name string
	if err := readonly.QueryRowContext(context.Background(), "SELECT name FROM notebooks WHERE id = ?", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").Scan(&name); err != nil {
		t.Fatalf("read committed WAL row: %v", err)
	}
	if name != "WAL fixture" {
		t.Fatalf("WAL row name = %q, want %q", name, "WAL fixture")
	}
}
