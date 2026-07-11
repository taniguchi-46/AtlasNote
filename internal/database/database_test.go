package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
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

func TestOpenRejectsDatabaseFromNewerVersionWithoutModification(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	newerDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open newer database: %v", err)
	}
	newerVersion := len(migrations) + 1
	if _, err := newerDB.Exec(`
CREATE TABLE future_data (value TEXT NOT NULL);
INSERT INTO future_data (value) VALUES ('preserve me');
`); err != nil {
		t.Fatalf("create future data: %v", err)
	}
	if _, err := newerDB.Exec("PRAGMA user_version = " + strconv.Itoa(newerVersion)); err != nil {
		t.Fatalf("set newer version: %v", err)
	}
	if err := newerDB.Close(); err != nil {
		t.Fatalf("close newer database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if db != nil {
		_ = db.Close()
		t.Fatal("Open() returned a database for a newer schema")
	}
	if !errors.Is(err, ErrDatabaseVersionTooNew) {
		t.Fatalf("Open() error = %v, want ErrDatabaseVersionTooNew", err)
	}

	verificationDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("reopen newer database: %v", err)
	}
	t.Cleanup(func() {
		_ = verificationDB.Close()
	})

	var userVersion int
	if err := verificationDB.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read newer version: %v", err)
	}
	if userVersion != newerVersion {
		t.Fatalf("user version = %d, want %d", userVersion, newerVersion)
	}

	var value string
	if err := verificationDB.QueryRow("SELECT value FROM future_data").Scan(&value); err != nil {
		t.Fatalf("read preserved future data: %v", err)
	}
	if value != "preserve me" {
		t.Fatalf("future data = %q, want %q", value, "preserve me")
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	t.Parallel()

	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	failingMigrations := append([]string(nil), migrations...)
	failingMigrations = append(failingMigrations, `
CREATE TABLE rollback_probe (id TEXT PRIMARY KEY);
INSERT INTO table_that_does_not_exist (id) VALUES ('fail');
`)

	if err := migrate(t.Context(), db, failingMigrations); err == nil {
		t.Fatal("migrate() succeeded, want migration error")
	}

	var userVersion int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read user version: %v", err)
	}
	if userVersion != len(migrations) {
		t.Fatalf("user version = %d, want %d", userVersion, len(migrations))
	}

	var tableCount int
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'rollback_probe'",
	).Scan(&tableCount); err != nil {
		t.Fatalf("check rollback probe table: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("rollback probe table count = %d, want 0", tableCount)
	}
}

func TestOpenAppliesPragmasToConcurrentConnectionsAndReconnect(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "data with space", "atlasnote.db")
	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(2)

	firstConn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	secondConn, err := db.Conn(t.Context())
	if err != nil {
		_ = firstConn.Close()
		t.Fatalf("acquire second connection: %v", err)
	}

	assertConnectionPragmas(t, firstConn)
	assertConnectionPragmas(t, secondConn)
	assertForeignKeyViolation(t, firstConn, "first-invalid")
	assertForeignKeyViolation(t, secondConn, "second-invalid")

	if err := secondConn.Close(); err != nil {
		t.Fatalf("close second connection: %v", err)
	}
	if err := firstConn.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopenedDB, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() {
		_ = reopenedDB.Close()
	})
	reopenedConn, err := reopenedDB.Conn(t.Context())
	if err != nil {
		t.Fatalf("acquire reopened connection: %v", err)
	}
	t.Cleanup(func() {
		_ = reopenedConn.Close()
	})
	assertConnectionPragmas(t, reopenedConn)
	assertForeignKeyViolation(t, reopenedConn, "reopened-invalid")
}

func assertConnectionPragmas(t *testing.T, conn *sql.Conn) {
	t.Helper()

	var foreignKeys int
	if err := conn.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := conn.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}

	var journalMode string
	if err := conn.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want %q", journalMode, "wal")
	}
}

func assertForeignKeyViolation(t *testing.T, conn *sql.Conn, id string) {
	t.Helper()

	_, err := conn.ExecContext(t.Context(), `
INSERT INTO notebooks (id, parent_id, name, created_at, updated_at)
VALUES (?, 'missing-parent', 'Invalid', '2026-07-11T00:00:00Z', '2026-07-11T00:00:00Z')
`, id)
	if err == nil {
		t.Fatal("foreign key violating insert succeeded")
	}
}
