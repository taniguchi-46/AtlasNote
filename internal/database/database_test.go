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

	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'note_search'",
	).Scan(&tableName); err != nil {
		t.Fatalf("read search index table: %v", err)
	}
	if tableName != "note_search" {
		t.Fatalf("search table name = %q", tableName)
	}

	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'note_search_state'",
	).Scan(&tableName); err != nil {
		t.Fatalf("read search index state table: %v", err)
	}
	if tableName != "note_search_state" {
		t.Fatalf("search index state table name = %q", tableName)
	}

	for _, expectedTable := range []string{
		"tags",
		"note_tags",
		"ai_provider_settings",
		"ai_histories",
		"ai_history_messages",
		"ai_history_sources",
		"ai_artifacts",
		"ai_artifact_sources",
	} {
		if err := db.QueryRowContext(
			t.Context(),
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
			expectedTable,
		).Scan(&tableName); err != nil {
			t.Fatalf("read %s table: %v", expectedTable, err)
		}
		if tableName != expectedTable {
			t.Fatalf("table name = %q, want %q", tableName, expectedTable)
		}
	}

	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_note_tags_tag_id_note_id'",
	).Scan(&tableName); err != nil {
		t.Fatalf("read note tag reverse index: %v", err)
	}
	if tableName != "idx_note_tags_tag_id_note_id" {
		t.Fatalf("note tag reverse index = %q", tableName)
	}

	aiColumns, err := tableColumns(t.Context(), db, "ai_provider_settings")
	if err != nil {
		t.Fatalf("read AI provider settings columns: %v", err)
	}
	for _, requiredColumn := range []string{"provider_id", "model_id", "credential_ref", "credential_storage", "is_selected"} {
		if !aiColumns[requiredColumn] {
			t.Fatalf("AI provider settings missing %s", requiredColumn)
		}
	}
	if aiColumns["api_key"] || aiColumns["secret"] {
		t.Fatal("AI provider settings schema contains a secret column")
	}
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_ai_provider_settings_one_selected'",
	).Scan(&tableName); err != nil {
		t.Fatalf("read selected AI provider index: %v", err)
	}
	if tableName != "idx_ai_provider_settings_one_selected" {
		t.Fatalf("selected AI provider index = %q", tableName)
	}
}

func TestOpenRepairsOrphanedNoteNotebookReferences(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:len(migrations)-1] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 15"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set legacy schema version: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notebooks (id, name, icon, created_at, updated_at)
VALUES
	('valid-notebook', 'Valid', 'default:note', '2026-08-28T00:00:00Z', '2026-08-28T00:00:00Z');
INSERT INTO notes (
	id, notebook_id, title, content_path, is_favorite, is_pinned, is_trashed,
	revision, created_at, updated_at
)
VALUES
	('orphan-note', 'deleted-notebook', 'Orphan', 'orphan-note.md', 0, 0, 1,
	7, '2026-08-28T00:01:00Z', '2026-08-28T00:02:00Z'),
	('valid-note', 'valid-notebook', 'Valid note', 'valid-note.md', 1, 0, 0,
	3, '2026-08-28T00:03:00Z', '2026-08-28T00:04:00Z');
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy orphan fixture: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var orphanNotebookID sql.NullString
	var orphanTitle string
	var orphanRevision int64
	var orphanUpdatedAt string
	if err := db.QueryRowContext(t.Context(), `
SELECT notebook_id, title, revision, updated_at
FROM notes
WHERE id = 'orphan-note'
`).Scan(&orphanNotebookID, &orphanTitle, &orphanRevision, &orphanUpdatedAt); err != nil {
		t.Fatalf("read repaired orphan note: %v", err)
	}
	if orphanNotebookID.Valid {
		t.Fatalf("orphan notebook id = %q, want NULL", orphanNotebookID.String)
	}
	if orphanTitle != "Orphan" || orphanRevision != 7 || orphanUpdatedAt != "2026-08-28T00:02:00Z" {
		t.Fatalf("repaired orphan metadata = title:%q revision:%d updated_at:%q", orphanTitle, orphanRevision, orphanUpdatedAt)
	}

	var validNotebookID string
	if err := db.QueryRowContext(t.Context(), "SELECT notebook_id FROM notes WHERE id = 'valid-note'").Scan(&validNotebookID); err != nil {
		t.Fatalf("read valid note reference: %v", err)
	}
	if validNotebookID != "valid-notebook" {
		t.Fatalf("valid notebook id = %q, want %q", validNotebookID, "valid-notebook")
	}

	var userVersion int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read migrated user version: %v", err)
	}
	if userVersion != len(migrations) {
		t.Fatalf("migrated user version = %d, want %d", userVersion, len(migrations))
	}
}

func TestAIV3SourceReferencesSurvivePermanentNoteDeletion(t *testing.T) {
	t.Parallel()

	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(t.Context(), `
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES ('source-note', 'Source', 'source-note.md', 0, 0, 0, 3, '2026-07-28T00:00:00Z', '2026-07-28T00:00:00Z');
INSERT INTO ai_histories (
	id, kind, title, provider_id, model_id, status, created_at, updated_at
)
VALUES ('history-1', 'qa', 'Saved Q&A', 'openrouter', 'test-model', 'saved', '2026-07-28T00:00:00Z', '2026-07-28T00:00:00Z');
INSERT INTO ai_history_messages(history_id, sequence, role, content, created_at)
VALUES ('history-1', 1, 'user', 'Question', '2026-07-28T00:00:00Z');
INSERT INTO ai_history_sources(history_id, note_id, input_revision)
VALUES ('history-1', 'source-note', 3);
`); err != nil {
		t.Fatalf("insert v3 fixture: %v", err)
	}

	if _, err := db.ExecContext(t.Context(), "DELETE FROM notes WHERE id = ?", "source-note"); err != nil {
		t.Fatalf("delete source note: %v", err)
	}

	var sourceCount int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_history_sources WHERE history_id = ?", "history-1").Scan(&sourceCount); err != nil {
		t.Fatalf("count preserved source reference: %v", err)
	}
	if sourceCount != 1 {
		t.Fatalf("preserved source references = %d, want 1", sourceCount)
	}

	if _, err := db.ExecContext(t.Context(), "DELETE FROM ai_histories WHERE id = ?", "history-1"); err != nil {
		t.Fatalf("delete history: %v", err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_history_messages WHERE history_id = ?", "history-1").Scan(&sourceCount); err != nil {
		t.Fatalf("count cascaded history messages: %v", err)
	}
	if sourceCount != 0 {
		t.Fatalf("cascaded history messages = %d, want 0", sourceCount)
	}
}

func TestSQLiteSupportsFTS5TrigramSearch(t *testing.T) {
	t.Parallel()

	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.ExecContext(t.Context(), `
CREATE VIRTUAL TABLE note_search_probe USING fts5(
	note_id UNINDEXED,
	title,
	body,
	tokenize = 'trigram'
);
INSERT INTO note_search_probe(note_id, title, body)
VALUES ('note-1', '検索テスト', 'Markdown本文を全文検索する');
`); err != nil {
		t.Fatalf("create and populate FTS5 trigram table: %v", err)
	}

	var matched int
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT COUNT(*) FROM note_search_probe WHERE note_search_probe MATCH ?",
		"全文検索",
	).Scan(&matched); err != nil {
		t.Fatalf("search Japanese trigram: %v", err)
	}
	if matched != 1 {
		t.Fatalf("Japanese trigram matches = %d, want 1", matched)
	}

	if err := db.QueryRowContext(
		t.Context(),
		"SELECT COUNT(*) FROM note_search_probe WHERE body LIKE ?",
		"%検索%",
	).Scan(&matched); err != nil {
		t.Fatalf("search two-character LIKE fallback: %v", err)
	}
	if matched != 1 {
		t.Fatalf("two-character LIKE matches = %d, want 1", matched)
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

func TestOpenMigratesVersionTwoDatabaseAndBackfillsNoteRevision(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:2] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 2"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version two: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, created_at, updated_at
)
VALUES (
	'existing-note', 'Existing note', 'existing-note.md', 1, 0, 0,
	'2026-07-10T00:00:00Z', '2026-07-10T01:00:00Z'
)
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy note: %v", err)
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

	var title string
	var contentPath string
	var revision int64
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT title, content_path, revision FROM notes WHERE id = 'existing-note'",
	).Scan(&title, &contentPath, &revision); err != nil {
		t.Fatalf("read migrated note: %v", err)
	}
	if title != "Existing note" {
		t.Fatalf("migrated title = %q, want %q", title, "Existing note")
	}
	if contentPath != "existing-note.md" {
		t.Fatalf("migrated content path = %q, want %q", contentPath, "existing-note.md")
	}
	if revision != 1 {
		t.Fatalf("migrated revision = %d, want 1", revision)
	}

	if _, err := db.ExecContext(t.Context(), "UPDATE notes SET revision = 0 WHERE id = 'existing-note'"); err == nil {
		t.Fatal("revision constraint accepted zero")
	}
}

func TestOpenMigratesVersionFiveDatabaseWithoutChangingExistingNote(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:5] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 5"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version five: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES (
	'existing-note', 'Existing note', 'existing-note.md', 1, 0, 0, 7,
	'2026-07-10T00:00:00Z', '2026-07-10T01:00:00Z'
)
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy note: %v", err)
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

	var title string
	var contentPath string
	var revision int64
	var createdAt string
	var updatedAt string
	if err := db.QueryRowContext(
		t.Context(),
		"SELECT title, content_path, revision, created_at, updated_at FROM notes WHERE id = 'existing-note'",
	).Scan(&title, &contentPath, &revision, &createdAt, &updatedAt); err != nil {
		t.Fatalf("read migrated note: %v", err)
	}
	if title != "Existing note" || contentPath != "existing-note.md" || revision != 7 ||
		createdAt != "2026-07-10T00:00:00Z" || updatedAt != "2026-07-10T01:00:00Z" {
		t.Fatalf("migrated note changed: title=%q path=%q revision=%d created=%q updated=%q", title, contentPath, revision, createdAt, updatedAt)
	}

	for _, table := range []string{"tags", "note_tags"} {
		var count int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count migrated %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("migrated %s count = %d, want 0", table, count)
		}
	}
}

func TestOpenMigratesVersionEightDatabaseWithHTTPPolicyDefault(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:8] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 8"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version eight: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO sync_connections (
	id, endpoint, remote_root, username, vault_id, credential_ref, created_at, updated_at
)
VALUES (
	1, 'https://dav.example.test', '/', 'alice', 'vault-1', 'credential-ref',
	'2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z'
)
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy sync connection: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate version eight database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var allowInsecureHTTP bool
	if err := db.QueryRowContext(t.Context(), "SELECT allow_insecure_http FROM sync_connections WHERE id = 1").Scan(&allowInsecureHTTP); err != nil {
		t.Fatalf("read migrated HTTP policy: %v", err)
	}
	if allowInsecureHTTP {
		t.Fatal("HTTP policy must default to disabled when migrating")
	}
}

func TestOpenMigratesVersionNineSyncSettingsToVersionTen(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:9] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 9"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version nine: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO sync_connections (
	id, endpoint, remote_root, username, vault_id, status, auto_sync,
	allow_insecure_http, credential_ref, created_at, updated_at
)
VALUES (
	1, 'https://dav.example.test', '/atlasnote', 'alice', 'vault-1', 'idle', 1,
	1, 'credential-ref', '2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z'
)
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert version nine sync connection: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate version nine database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var interval, proxyTimeout int
	var failSafe, allowHTTP, ignoreTLS, proxyEnabled bool
	var certificates, proxyURL string
	if err := db.QueryRowContext(t.Context(), `
SELECT sync_interval_seconds, fail_safe, allow_insecure_http,
       custom_tls_certificates, ignore_tls_errors, proxy_enabled,
       proxy_url, proxy_timeout_seconds
FROM sync_connections WHERE id = 1
`).Scan(&interval, &failSafe, &allowHTTP, &certificates, &ignoreTLS, &proxyEnabled, &proxyURL, &proxyTimeout); err != nil {
		t.Fatalf("read migrated sync settings: %v", err)
	}
	if interval != 300 || !failSafe || !allowHTTP || certificates != "" || ignoreTLS || proxyEnabled || proxyURL != "" || proxyTimeout != 1 {
		t.Fatalf("unexpected migrated sync settings: interval=%d failSafe=%v allowHTTP=%v certificates=%q ignoreTLS=%v proxy=%v proxyURL=%q timeout=%d",
			interval, failSafe, allowHTTP, certificates, ignoreTLS, proxyEnabled, proxyURL, proxyTimeout)
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE sync_connections SET sync_interval_seconds = 42 WHERE id = 1"); err == nil {
		t.Fatal("invalid sync interval was accepted")
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE sync_connections SET proxy_timeout_seconds = 0 WHERE id = 1"); err == nil {
		t.Fatal("invalid proxy timeout was accepted")
	}
}

func TestVersionTenMigrationKeepsDisabledSyncIntervalDisabled(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	defer db.Close()
	for index, migration := range migrations[:9] {
		if _, err := db.Exec(migration); err != nil {
			t.Fatalf("apply fixture migration %d: %v", index+1, err)
		}
	}
	if _, err := db.Exec(`
INSERT INTO sync_connections (
	id, endpoint, remote_root, username, vault_id, auto_sync,
	credential_ref, created_at, updated_at
)
VALUES (1, 'https://dav.example.test', '/', 'alice', 'vault', 0, 'ref', 'now', 'now')
`); err != nil {
		t.Fatalf("insert disabled sync fixture: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 9"); err != nil {
		t.Fatalf("set fixture version: %v", err)
	}
	if err := migrate(t.Context(), db, migrations); err != nil {
		t.Fatalf("migrate disabled sync fixture: %v", err)
	}
	var interval int
	if err := db.QueryRow("SELECT sync_interval_seconds FROM sync_connections WHERE id = 1").Scan(&interval); err != nil {
		t.Fatalf("read disabled interval: %v", err)
	}
	if interval != 0 {
		t.Fatalf("disabled interval = %d, want 0", interval)
	}
}

func TestOpenMigratesVersionTenDatabaseWithExistingDataAndAISchema(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open version ten database: %v", err)
	}
	for index, migration := range migrations[:10] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply version ten migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notebooks(id, parent_id, name, icon, created_at, updated_at)
VALUES ('legacy-book', NULL, 'Legacy notebook', 'default:note', '2026-07-27T00:00:00Z', '2026-07-27T00:00:00Z');
INSERT INTO notes(
	id, notebook_id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES (
	'legacy-note', 'legacy-book', 'Legacy note', 'legacy-note.md', 1, 0, 0, 4,
	'2026-07-27T00:00:00Z', '2026-07-27T00:01:00Z'
);
INSERT INTO tags(id, name, normalized_name, created_at, updated_at)
VALUES ('legacy-tag', 'Legacy tag', 'legacy tag', '2026-07-27T00:00:00Z', '2026-07-27T00:00:00Z');
INSERT INTO note_tags(note_id, tag_id) VALUES ('legacy-note', 'legacy-tag');
INSERT INTO sync_item_states(
	entity_key, entity_type, local_object_hash, base_object_hash, remote_object_hash,
	body_hash, metadata_hash, resolution_state, snapshot_json, updated_at
)
VALUES (
	'note:legacy-note', 'note', 'local-hash', 'base-hash', 'remote-hash',
	'body-hash', 'metadata-hash', 'synced', '{"legacy":true}', '2026-07-27T00:02:00Z'
);
INSERT INTO sync_outbox(
	change_set_id, entity_key, entity_type, object_hash, base_manifest_hash, base_head_etag,
	object_json, deleted, attempt_count, next_retry_at, failed_class, created_at
)
VALUES (
	'legacy-change', 'note:legacy-note', 'note', 'object-hash', 'manifest-hash', 'head-etag',
	'{"legacy":"outbox"}', 0, 2, '2026-07-27T00:03:00Z', 'network', '2026-07-27T00:02:00Z'
);
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert version ten data: %v", err)
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 10"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version ten: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close version ten database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate version ten database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var userVersion int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read migrated user version: %v", err)
	}
	if userVersion != len(migrations) {
		t.Fatalf("migrated user version = %d, want %d", userVersion, len(migrations))
	}
	for _, table := range []string{
		"ai_provider_settings",
		"ai_histories",
		"ai_history_messages",
		"ai_history_sources",
		"ai_artifacts",
		"ai_artifact_sources",
	} {
		var tableName string
		if err := db.QueryRowContext(t.Context(), `
SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?
`, table).Scan(&tableName); err != nil {
			t.Fatalf("read %s table: %v", table, err)
		}
		if tableName != table {
			t.Fatalf("AI table = %q, want %q", tableName, table)
		}
	}

	var title string
	var notebookID string
	var revision int
	if err := db.QueryRowContext(t.Context(), `
SELECT title, notebook_id, revision FROM notes WHERE id = 'legacy-note'
`).Scan(&title, &notebookID, &revision); err != nil {
		t.Fatalf("read migrated legacy note: %v", err)
	}
	if title != "Legacy note" || notebookID != "legacy-book" || revision != 4 {
		t.Fatalf("migrated legacy note = title:%q notebook:%q revision:%d", title, notebookID, revision)
	}
	var relationCount int
	if err := db.QueryRowContext(t.Context(), `
SELECT COUNT(*) FROM note_tags WHERE note_id = 'legacy-note' AND tag_id = 'legacy-tag'
`).Scan(&relationCount); err != nil {
		t.Fatalf("read migrated legacy note tag: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("migrated legacy note tag count = %d, want 1", relationCount)
	}
	var snapshotJSON string
	if err := db.QueryRowContext(t.Context(), `
SELECT snapshot_json FROM sync_item_states WHERE entity_key = 'note:legacy-note'
`).Scan(&snapshotJSON); err != nil {
		t.Fatalf("read migrated legacy sync item: %v", err)
	}
	if snapshotJSON != `{"legacy":true}` {
		t.Fatalf("migrated legacy sync snapshot = %q", snapshotJSON)
	}
	var objectJSON string
	var attemptCount int
	if err := db.QueryRowContext(t.Context(), `
SELECT object_json, attempt_count FROM sync_outbox WHERE change_set_id = 'legacy-change'
`).Scan(&objectJSON, &attemptCount); err != nil {
		t.Fatalf("read migrated legacy outbox: %v", err)
	}
	if objectJSON != `{"legacy":"outbox"}` || attemptCount != 2 {
		t.Fatalf("migrated legacy outbox = object:%q attempts:%d", objectJSON, attemptCount)
	}
}

func TestMigrateVersionThirteenAIProviderSettingsAddsSelectedProvider(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	for index, migration := range migrations[:13] {
		if _, err := db.Exec(migration); err != nil {
			t.Fatalf("apply version thirteen migration %d: %v", index+1, err)
		}
	}
	if _, err := db.Exec(`
INSERT INTO ai_provider_settings(provider_id, model_id, credential_ref, credential_storage, created_at, updated_at)
VALUES
	('openrouter', 'openrouter-model', 'openrouter-ref', 'persistent', '2026-08-01T00:00:00Z', '2026-08-01T00:00:00Z'),
	('gemini', 'gemini-model', 'gemini-ref', 'persistent', '2026-08-01T00:00:00Z', '2026-08-01T00:01:00Z');
PRAGMA user_version = 13;
`); err != nil {
		t.Fatalf("insert version thirteen AI provider settings: %v", err)
	}

	if err := migrate(t.Context(), db, migrations); err != nil {
		t.Fatalf("migrate version thirteen AI provider settings: %v", err)
	}

	var selectedProvider string
	if err := db.QueryRowContext(t.Context(), `
SELECT provider_id
FROM ai_provider_settings
WHERE is_selected = 1
`).Scan(&selectedProvider); err != nil {
		t.Fatalf("read selected AI provider: %v", err)
	}
	if selectedProvider != "gemini" {
		t.Fatalf("selected provider = %q, want gemini", selectedProvider)
	}

	var selectedCount int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_provider_settings WHERE is_selected = 1").Scan(&selectedCount); err != nil {
		t.Fatalf("count selected AI providers: %v", err)
	}
	if selectedCount != 1 {
		t.Fatalf("selected provider count = %d, want 1", selectedCount)
	}
	if _, err := db.ExecContext(t.Context(), `
UPDATE ai_provider_settings
SET is_selected = 1
WHERE provider_id = 'openrouter'
`); err == nil {
		t.Fatal("AI provider settings allowed multiple selected providers")
	}

	for providerID, want := range map[string]struct {
		ref   string
		model string
	}{
		"openrouter": {ref: "openrouter-ref", model: "openrouter-model"},
		"gemini":     {ref: "gemini-ref", model: "gemini-model"},
	} {
		var gotRef string
		var gotModel string
		if err := db.QueryRowContext(t.Context(), `
SELECT credential_ref, model_id
FROM ai_provider_settings
WHERE provider_id = ?
`, providerID).Scan(&gotRef, &gotModel); err != nil {
			t.Fatalf("read migrated %s settings: %v", providerID, err)
		}
		if gotRef != want.ref || gotModel != want.model {
			t.Fatalf("migrated %s settings = ref %q, model %q", providerID, gotRef, gotModel)
		}
	}
}

func TestMigrateVersionTwelveArtifactsAddsSummaryKindWithoutDataLoss(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	for index, migration := range migrations[:12] {
		if _, err := db.Exec(migration); err != nil {
			t.Fatalf("apply version twelve migration %d: %v", index+1, err)
		}
	}
	if _, err := db.Exec(`
INSERT INTO ai_artifacts(id, kind, title, provider_id, model_id, content, status, created_at, updated_at)
VALUES ('artifact-v12', 'document', 'Existing artifact', 'openrouter', 'test-model', 'preserve me', 'saved', '2026-07-30T00:00:00Z', '2026-07-30T00:00:00Z');
INSERT INTO ai_artifact_sources(artifact_id, note_id, input_revision)
VALUES ('artifact-v12', 'source-note', 7);
PRAGMA user_version = 12;
`); err != nil {
		t.Fatalf("insert version twelve fixture: %v", err)
	}

	if err := migrate(t.Context(), db, migrations); err != nil {
		t.Fatalf("migrate version twelve fixture: %v", err)
	}

	var kind string
	var content string
	if err := db.QueryRow("SELECT kind, content FROM ai_artifacts WHERE id = 'artifact-v12'").Scan(&kind, &content); err != nil {
		t.Fatalf("read migrated artifact: %v", err)
	}
	if kind != "document" || content != "preserve me" {
		t.Fatalf("migrated artifact = kind:%q content:%q", kind, content)
	}
	var revision int
	if err := db.QueryRow("SELECT input_revision FROM ai_artifact_sources WHERE artifact_id = 'artifact-v12'").Scan(&revision); err != nil {
		t.Fatalf("read migrated artifact source: %v", err)
	}
	if revision != 7 {
		t.Fatalf("migrated artifact source revision = %d, want 7", revision)
	}
	if _, err := db.Exec(`
INSERT INTO ai_artifacts(id, kind, title, provider_id, model_id, content, status, created_at, updated_at)
VALUES ('summary-v13', 'summary', 'Summary', 'openrouter', 'test-model', '## 概要', 'saved', '2026-07-30T00:00:00Z', '2026-07-30T00:00:00Z')
`); err != nil {
		t.Fatalf("insert summary artifact after migration: %v", err)
	}
}

func TestMigrateVersionTwelveArtifactsRollsBackFailedV13Rebuild(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	for index, migration := range migrations[:12] {
		if _, err := db.Exec(migration); err != nil {
			t.Fatalf("apply version twelve migration %d: %v", index+1, err)
		}
	}
	if _, err := db.Exec(`
INSERT INTO ai_artifacts(id, kind, title, provider_id, model_id, content, status, created_at, updated_at)
VALUES ('rollback-artifact', 'document', 'Rollback artifact', 'openrouter', 'test-model', 'preserve rollback data', 'saved', '2026-07-30T00:00:00Z', '2026-07-30T00:00:00Z');
INSERT INTO ai_artifact_sources(artifact_id, note_id, input_revision)
VALUES ('rollback-artifact', 'deleted-source-note', 9);
PRAGMA user_version = 12;
`); err != nil {
		t.Fatalf("insert version twelve rollback fixture: %v", err)
	}

	failingMigrations := append([]string(nil), migrations...)
	failingMigrations[12] += `
INSERT INTO table_that_does_not_exist(id) VALUES ('force-v13-rollback');
`
	if err := migrate(t.Context(), db, failingMigrations); err == nil {
		t.Fatal("v13 artifact rebuild succeeded, want forced migration error")
	}

	var userVersion int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read rolled back user version: %v", err)
	}
	if userVersion != 12 {
		t.Fatalf("rolled back user version = %d, want 12", userVersion)
	}
	var kind string
	var content string
	if err := db.QueryRowContext(t.Context(), `
SELECT kind, content FROM ai_artifacts WHERE id = 'rollback-artifact'
`).Scan(&kind, &content); err != nil {
		t.Fatalf("read artifact after v13 rollback: %v", err)
	}
	if kind != "document" || content != "preserve rollback data" {
		t.Fatalf("artifact after v13 rollback = kind:%q content:%q", kind, content)
	}
	var sourceRevision int
	if err := db.QueryRowContext(t.Context(), `
SELECT input_revision FROM ai_artifact_sources WHERE artifact_id = 'rollback-artifact'
`).Scan(&sourceRevision); err != nil {
		t.Fatalf("read artifact source after v13 rollback: %v", err)
	}
	if sourceRevision != 9 {
		t.Fatalf("artifact source after v13 rollback = %d, want 9", sourceRevision)
	}
	var temporaryTableCount int
	if err := db.QueryRowContext(t.Context(), `
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'ai_artifacts_v13'
`).Scan(&temporaryTableCount); err != nil {
		t.Fatalf("check v13 temporary table after rollback: %v", err)
	}
	if temporaryTableCount != 0 {
		t.Fatalf("v13 temporary table count after rollback = %d, want 0", temporaryTableCount)
	}
	var indexName string
	if err := db.QueryRowContext(t.Context(), `
SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_ai_artifacts_status_updated_at'
`).Scan(&indexName); err != nil {
		t.Fatalf("read restored artifact index after rollback: %v", err)
	}
	if indexName != "idx_ai_artifacts_status_updated_at" {
		t.Fatalf("restored artifact index = %q", indexName)
	}
}

func TestOpenMigratesVersionSixDatabaseWithEmptyNoteLinkIndex(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "atlasnote.db")
	legacyDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for index, migration := range migrations[:6] {
		if _, err := legacyDB.Exec(migration); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("apply legacy migration %d: %v", index+1, err)
		}
	}
	if _, err := legacyDB.Exec("PRAGMA user_version = 6"); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("set version six: %v", err)
	}
	if _, err := legacyDB.Exec(`
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES (
	'existing-note', 'Existing note', 'existing-note.md', 0, 0, 0, 3,
	'2026-07-10T00:00:00Z', '2026-07-10T01:00:00Z'
)
`); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy note: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	db, err := Open(t.Context(), databasePath)
	if err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"note_links", "note_link_state"} {
		var count int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count migrated %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("migrated %s count = %d, want 0", table, count)
		}
	}

	var title string
	var revision int64
	if err := db.QueryRowContext(t.Context(), "SELECT title, revision FROM notes WHERE id = 'existing-note'").Scan(&title, &revision); err != nil {
		t.Fatalf("read migrated note: %v", err)
	}
	if title != "Existing note" || revision != 3 {
		t.Fatalf("migrated note changed: title=%q revision=%d", title, revision)
	}
}

func TestTagForeignKeysCascadeRelations(t *testing.T) {
	t.Parallel()

	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.ExecContext(t.Context(), `
INSERT INTO notes (
	id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
)
VALUES ('note-1', 'Note', 'note-1.md', 0, 0, 0, 1, '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z');
INSERT INTO tags (id, name, normalized_name, created_at, updated_at)
VALUES ('tag-1', 'Work', 'work', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z');
INSERT INTO note_tags (note_id, tag_id) VALUES ('note-1', 'tag-1');
`); err != nil {
		t.Fatalf("create note tag relation: %v", err)
	}

	if _, err := db.ExecContext(t.Context(), "DELETE FROM tags WHERE id = 'tag-1'"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	assertTableCount(t, db, "notes", 1)
	assertTableCount(t, db, "note_tags", 0)

	if _, err := db.ExecContext(t.Context(), `
INSERT INTO tags (id, name, normalized_name, created_at, updated_at)
VALUES ('tag-2', 'Personal', 'personal', '2026-07-13T00:00:00Z', '2026-07-13T00:00:00Z');
INSERT INTO note_tags (note_id, tag_id) VALUES ('note-1', 'tag-2');
DELETE FROM notes WHERE id = 'note-1';
`); err != nil {
		t.Fatalf("delete note with tag relation: %v", err)
	}
	assertTableCount(t, db, "tags", 1)
	assertTableCount(t, db, "note_tags", 0)
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

func assertTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
