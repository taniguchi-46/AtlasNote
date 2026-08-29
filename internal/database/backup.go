package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	sqlite "modernc.org/sqlite"
)

// SnapshotInfo describes the validated schema version of a standalone
// SQLite snapshot. It intentionally contains no database contents.
type SnapshotInfo struct {
	SchemaVersion int
}

var (
	ErrSnapshotInvalid = errors.New("sqlite snapshot is invalid")
	ErrSnapshotSymlink = errors.New("sqlite snapshot path is a symlink")
)

// OnlineBackup copies a consistent SQLite snapshot while the source database
// remains open. The destination is created as a standalone database and does
// not depend on the source WAL or SHM files.
func OnlineBackup(ctx context.Context, source *sql.DB, destinationPath string) error {
	if source == nil {
		return fmt.Errorf("online sqlite backup: %w", ErrSnapshotInvalid)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return fmt.Errorf("create sqlite backup directory: %w", err)
	}
	if info, err := os.Lstat(destinationPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSnapshotSymlink
		}
		return fmt.Errorf("online sqlite backup destination already exists: %w", ErrSnapshotInvalid)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect sqlite backup destination: %w", err)
	}

	conn, err := source.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire sqlite backup connection: %w", err)
	}
	defer conn.Close()

	if err := conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("online sqlite backup: %w", ErrSnapshotInvalid)
		}
		backup, err := backuper.NewBackup(destinationPath)
		if err != nil {
			return fmt.Errorf("create online sqlite backup: %w", err)
		}
		finished := false
		defer func() {
			if !finished {
				_ = backup.Finish()
			}
		}()

		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			more, err := backup.Step(128)
			if err != nil {
				return fmt.Errorf("copy online sqlite backup: %w", err)
			}
			if !more {
				break
			}
		}
		err = backup.Finish()
		finished = true
		if err != nil {
			return fmt.Errorf("finish online sqlite backup: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// ValidateSnapshot checks a standalone database without migrating or writing
// it. Restore callers can therefore reject a tampered or newer snapshot
// before it is allowed to replace the active vault.
func ValidateSnapshot(ctx context.Context, databasePath string) (SnapshotInfo, error) {
	info, err := os.Lstat(databasePath)
	if err != nil {
		return SnapshotInfo{}, fmt.Errorf("inspect sqlite snapshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return SnapshotInfo{}, ErrSnapshotSymlink
	}
	db, err := sql.Open("sqlite", sqliteDSNWithMode(databasePath, "ro"))
	if err != nil {
		return SnapshotInfo{}, fmt.Errorf("open sqlite snapshot: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := validateDatabase(ctx, db); err != nil {
		return SnapshotInfo{}, err
	}
	var schemaVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return SnapshotInfo{}, fmt.Errorf("read sqlite snapshot schema version: %w", err)
	}
	return SnapshotInfo{SchemaVersion: schemaVersion}, nil
}

// ValidateOpen checks integrity of an already-open database. It is used
// before committing a newly created backup and does not mutate schema data.
func ValidateOpen(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("validate sqlite database: %w", ErrSnapshotInvalid)
	}
	return validateDatabase(ctx, db)
}

func validateDatabase(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite snapshot: %w", err)
	}
	var schemaVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return fmt.Errorf("read sqlite snapshot schema version: %w", err)
	}
	if schemaVersion < 0 || schemaVersion > len(migrations) {
		return fmt.Errorf("sqlite snapshot schema version %d: %w", schemaVersion, ErrDatabaseVersionTooNew)
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("run sqlite integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("sqlite integrity check returned %q: %w", integrity, ErrSnapshotInvalid)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("run sqlite foreign key check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("sqlite foreign key check failed: %w", ErrSnapshotInvalid)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read sqlite foreign key check: %w", err)
	}
	return nil
}
