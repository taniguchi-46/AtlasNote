package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, databasePath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
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

func configure(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure sqlite: %w", err)
		}
	}

	return nil
}

var migrations = []string{
	`
CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	content_path TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_updated_at ON notes(updated_at);
`,
}

func Migrate(ctx context.Context, db *sql.DB) error {
	var userVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	if userVersion >= len(migrations) {
		return nil // Up to date
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	for i := userVersion; i < len(migrations); i++ {
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			return fmt.Errorf("migrate version %d: %w", i+1, err)
		}
	}

	newVersion := len(migrations)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", newVersion)); err != nil {
		return fmt.Errorf("update user_version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration tx: %w", err)
	}

	return nil
}
