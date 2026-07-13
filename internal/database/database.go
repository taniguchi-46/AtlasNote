package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// ErrDatabaseVersionTooNew indicates that the database requires a newer Atlas Note version.
var ErrDatabaseVersionTooNew = errors.New("database version is newer than supported")

func Open(ctx context.Context, databasePath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", sqliteDSN(databasePath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if _, err := validateSchemaVersion(ctx, db, len(migrations)); err != nil {
		db.Close()
		return nil, err
	}

	if err := configure(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func sqliteDSN(databasePath string) string {
	dsn := &url.URL{
		Scheme: "file",
		Opaque: filepath.ToSlash(databasePath),
	}
	query := dsn.Query()
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_pragma", "busy_timeout(5000)")
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func configure(ctx context.Context, db *sql.DB) error {
	var journalMode string
	// デフォルトのDELETEモードではなくWAL（Write-Ahead Logging）モードを明示的に指定する。
	// これにより、読み込みと書き込みが互いにブロックしにくくなり並行処理性能が向上するだけでなく、
	// アプリケーションがクラッシュした際のデータベース破損リスクも低減される。
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("configure sqlite journal mode: %w", err)
	}
	if journalMode != "wal" {
		return fmt.Errorf("configure sqlite journal mode: got %q, want %q", journalMode, "wal")
	}

	return nil
}

var migrations = []string{
	`
CREATE TABLE IF NOT EXISTS notebooks (
	id TEXT PRIMARY KEY,
	parent_id TEXT,
	name TEXT NOT NULL,
	icon TEXT NOT NULL DEFAULT 'default:note',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(parent_id) REFERENCES notebooks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	notebook_id TEXT,
	title TEXT NOT NULL,
	content_path TEXT NOT NULL UNIQUE,
	is_favorite BOOLEAN NOT NULL DEFAULT 0,
	is_pinned BOOLEAN NOT NULL DEFAULT 0,
	is_trashed BOOLEAN NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(notebook_id) REFERENCES notebooks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_updated_at ON notes(updated_at);
CREATE INDEX IF NOT EXISTS idx_notebooks_parent_id ON notebooks(parent_id);
`,
	`
CREATE TABLE IF NOT EXISTS note_storage_operations (
	operation_id TEXT PRIMARY KEY,
	note_id TEXT NOT NULL UNIQUE,
	operation_type TEXT NOT NULL CHECK(operation_type IN ('upsert', 'delete')),
	content_hash TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_note_storage_operations_note_id
	ON note_storage_operations(note_id);
`,
	`
ALTER TABLE notes
	ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1);
	`,
	`
CREATE VIRTUAL TABLE IF NOT EXISTS note_search USING fts5(
	note_id UNINDEXED,
	title,
	body,
	tokenize = 'trigram'
);

CREATE TABLE IF NOT EXISTS note_search_state (
	note_id TEXT PRIMARY KEY,
	indexed_revision INTEGER NOT NULL CHECK(indexed_revision >= 1),
	content_hash TEXT NOT NULL,
	indexed_at TEXT NOT NULL,
	FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_search_state_revision
	ON note_search_state(indexed_revision);
	`,
	`
ALTER TABLE note_search_state
	ADD COLUMN content_mtime_ns INTEGER NOT NULL DEFAULT 0;
	`,
	`
CREATE TABLE IF NOT EXISTS tags (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
	normalized_name TEXT NOT NULL UNIQUE CHECK(length(normalized_name) > 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS note_tags (
	note_id TEXT NOT NULL,
	tag_id TEXT NOT NULL,
	PRIMARY KEY (note_id, tag_id),
	FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE,
	FOREIGN KEY(tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_tags_tag_id_note_id
	ON note_tags(tag_id, note_id);
	`,
}

func Migrate(ctx context.Context, db *sql.DB) error {
	return migrate(ctx, db, migrations)
}

func migrate(ctx context.Context, db *sql.DB, migrationSet []string) error {
	userVersion, err := validateSchemaVersion(ctx, db, len(migrationSet))
	if err != nil {
		return err
	}

	if userVersion == len(migrationSet) {
		return ensureCompatibleSchema(ctx, db)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	for i := userVersion; i < len(migrationSet); i++ {
		if _, err := tx.ExecContext(ctx, migrationSet[i]); err != nil {
			return fmt.Errorf("migrate version %d: %w", i+1, err)
		}
	}

	newVersion := len(migrationSet)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", newVersion)); err != nil {
		return fmt.Errorf("update user_version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}

	return ensureCompatibleSchema(ctx, db)
}

func validateSchemaVersion(ctx context.Context, db *sql.DB, currentVersion int) (int, error) {
	var userVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}

	// 現在のアプリが想定しているマイグレーションの数（currentVersion）よりも
	// DBファイルのバージョン（userVersion）が新しい場合、起動をブロックする。
	// これを許可してしまうと、古い仕様のアプリが新しいスキーマのデータを誤って読み書きし、
	// データ構造を修復不能な状態に破壊してしまう恐れがあるため。
	if userVersion > currentVersion {
		return 0, fmt.Errorf("%w: database version %d, supported version %d", ErrDatabaseVersionTooNew, userVersion, currentVersion)
	}

	return userVersion, nil
}

func ensureCompatibleSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS notebooks (
	id TEXT PRIMARY KEY,
	parent_id TEXT,
	name TEXT NOT NULL,
	icon TEXT NOT NULL DEFAULT 'default:note',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(parent_id) REFERENCES notebooks(id) ON DELETE CASCADE
);
`); err != nil {
		return fmt.Errorf("ensure notebooks table: %w", err)
	}

	notebookColumns, err := tableColumns(ctx, db, "notebooks")
	if err != nil {
		return err
	}
	if !notebookColumns["icon"] {
		if _, err := db.ExecContext(ctx, "ALTER TABLE notebooks ADD COLUMN icon TEXT NOT NULL DEFAULT 'default:note'"); err != nil {
			return fmt.Errorf("add notebooks.icon column: %w", err)
		}
	}

	columns, err := tableColumns(ctx, db, "notes")
	if err != nil {
		return err
	}

	requiredColumns := map[string]string{
		"notebook_id": "TEXT",
		"is_favorite": "BOOLEAN NOT NULL DEFAULT 0",
		"is_pinned":   "BOOLEAN NOT NULL DEFAULT 0",
		"is_trashed":  "BOOLEAN NOT NULL DEFAULT 0",
		"revision":    "INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1)",
	}
	for name, definition := range requiredColumns {
		if columns[name] {
			continue
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE notes ADD COLUMN %s %s", name, definition)); err != nil {
			return fmt.Errorf("add notes.%s column: %w", name, err)
		}
	}

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS note_storage_operations (
	operation_id TEXT PRIMARY KEY,
	note_id TEXT NOT NULL UNIQUE,
	operation_type TEXT NOT NULL CHECK(operation_type IN ('upsert', 'delete')),
	content_hash TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_updated_at ON notes(updated_at);
CREATE INDEX IF NOT EXISTS idx_notes_notebook_id ON notes(notebook_id);
CREATE INDEX IF NOT EXISTS idx_notebooks_parent_id ON notebooks(parent_id);
CREATE INDEX IF NOT EXISTS idx_note_storage_operations_note_id
	ON note_storage_operations(note_id);

CREATE VIRTUAL TABLE IF NOT EXISTS note_search USING fts5(
	note_id UNINDEXED,
	title,
	body,
	tokenize = 'trigram'
);

CREATE TABLE IF NOT EXISTS note_search_state (
	note_id TEXT PRIMARY KEY,
	indexed_revision INTEGER NOT NULL CHECK(indexed_revision >= 1),
	content_hash TEXT NOT NULL,
	content_mtime_ns INTEGER NOT NULL DEFAULT 0,
	indexed_at TEXT NOT NULL,
	FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_search_state_revision
	ON note_search_state(indexed_revision);

CREATE TABLE IF NOT EXISTS tags (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
	normalized_name TEXT NOT NULL UNIQUE CHECK(length(normalized_name) > 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS note_tags (
	note_id TEXT NOT NULL,
	tag_id TEXT NOT NULL,
	PRIMARY KEY (note_id, tag_id),
	FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE,
	FOREIGN KEY(tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_tags_tag_id_note_id
	ON note_tags(tag_id, note_id);
`); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	return nil
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("read %s columns: %w", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan %s column: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s columns: %w", table, err)
	}

	return columns, nil
}
