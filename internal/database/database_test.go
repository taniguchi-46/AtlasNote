package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesStorageOperationMigration(t *testing.T) {
	t.Parallel()

	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var userVersion int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read user version: %v", err)
	}
	if userVersion != len(migrations) {
		t.Fatalf("user version = %d, want %d", userVersion, len(migrations))
	}

	var tableName string
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'note_storage_operations'",
	).Scan(&tableName); err != nil {
		t.Fatalf("read storage operation table: %v", err)
	}
	if tableName != "note_storage_operations" {
		t.Fatalf("table name = %q", tableName)
	}
}

func TestOpenMigratesVersionOneDatabaseWithoutChangingExistingData(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := legacyDB.Exec(migrations[0]); err != nil {
		t.Fatalf("create version one schema: %v", err)
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("set version one: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notebooks (id, name, icon, created_at, updated_at)
VALUES ('existing', 'Existing', 'default:note', '2026-07-10T00:00:00Z', '2026-07-10T00:00:00Z')
`); err != nil {
		t.Fatalf("insert legacy data: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var name string
	if err := db.QueryRowContext(t.Context(), "SELECT name FROM notebooks WHERE id = 'existing'").Scan(&name); err != nil {
		t.Fatalf("read migrated legacy data: %v", err)
	}
	if name != "Existing" {
		t.Fatalf("migrated notebook name = %q", name)
	}

	var operationTable string
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'note_storage_operations'",
	).Scan(&operationTable); err != nil {
		t.Fatalf("read migrated operation table: %v", err)
	}
}
