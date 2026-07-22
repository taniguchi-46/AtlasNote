package ai

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

type providerRecord struct {
	ProviderID        ProviderID
	ModelID           string
	CredentialRef     string
	CredentialStorage credentialStorage
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) get(ctx context.Context, providerID ProviderID) (*providerRecord, error) {
	record := providerRecord{}
	err := r.db.QueryRowContext(ctx, `
SELECT provider_id, model_id, credential_ref, credential_storage
FROM ai_provider_settings
WHERE provider_id = ?
`, providerID).Scan(&record.ProviderID, &record.ModelID, &record.CredentialRef, &record.CredentialStorage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get AI provider settings: %w", err)
	}
	return &record, nil
}

func (r *Repository) list(ctx context.Context) ([]providerRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT provider_id, model_id, credential_ref, credential_storage
FROM ai_provider_settings
ORDER BY provider_id
`)
	if err != nil {
		return nil, fmt.Errorf("list AI provider settings: %w", err)
	}
	defer rows.Close()

	records := make([]providerRecord, 0)
	for rows.Next() {
		record := providerRecord{}
		if err := rows.Scan(&record.ProviderID, &record.ModelID, &record.CredentialRef, &record.CredentialStorage); err != nil {
			return nil, fmt.Errorf("scan AI provider settings: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI provider settings: %w", err)
	}
	return records, nil
}

func (r *Repository) save(ctx context.Context, record providerRecord) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO ai_provider_settings(provider_id, model_id, credential_ref, credential_storage, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(provider_id) DO UPDATE SET
	model_id = excluded.model_id,
	credential_ref = excluded.credential_ref,
	credential_storage = excluded.credential_storage,
	updated_at = excluded.updated_at
`, record.ProviderID, record.ModelID, record.CredentialRef, record.CredentialStorage, now, now)
	if err != nil {
		return fmt.Errorf("save AI provider settings: %w", err)
	}
	return nil
}

func (r *Repository) delete(ctx context.Context, providerID ProviderID) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM ai_provider_settings WHERE provider_id = ?", providerID); err != nil {
		return fmt.Errorf("delete AI provider settings: %w", err)
	}
	return nil
}
