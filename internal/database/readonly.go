package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

// OpenReadOnly opens an existing SQLite file without schema validation or
// migration. Maintenance commands use this narrow API when they need to read
// metadata from an old installation without changing the user's database.
func OpenReadOnly(ctx context.Context, databasePath string) (*sql.DB, error) {
	info, err := os.Lstat(databasePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("sqlite read-only path is not a regular file: %w", ErrSnapshotInvalid)
	}

	db, err := sql.Open("sqlite", sqliteDSNReadOnly(databasePath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite read-only: %w", err)
	}
	return db, nil
}
