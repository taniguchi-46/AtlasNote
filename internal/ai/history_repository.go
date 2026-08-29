package ai

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const aiRecordListLimit = 100

const historyStatusExpression = `CASE
	WHEN EXISTS (
		SELECT 1
		FROM ai_history_sources source
		LEFT JOIN notes source_note ON source_note.id = source.note_id
		WHERE source.history_id = history.id AND source_note.id IS NULL
	) THEN 'orphaned'
	WHEN EXISTS (
		SELECT 1
		FROM ai_history_sources source
		JOIN notes source_note ON source_note.id = source.note_id
		WHERE source.history_id = history.id AND source_note.revision <> source.input_revision
	) THEN 'stale'
	ELSE 'saved'
END`

const artifactStatusExpression = `CASE
	WHEN EXISTS (
		SELECT 1
		FROM ai_artifact_sources source
		LEFT JOIN notes source_note ON source_note.id = source.note_id
		WHERE source.artifact_id = artifact.id AND source_note.id IS NULL
	) THEN 'orphaned'
	WHEN EXISTS (
		SELECT 1
		FROM ai_artifact_sources source
		JOIN notes source_note ON source_note.id = source.note_id
		WHERE source.artifact_id = artifact.id AND source_note.revision <> source.input_revision
	) THEN 'stale'
	ELSE 'saved'
END`

func (r *Repository) saveHistory(ctx context.Context, input SaveAIHistoryInput) (AIHistory, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := now
	if input.ID != "" {
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM ai_histories WHERE id = ?", input.ID).Scan(&createdAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return AIHistory{}, fmt.Errorf("read AI history creation time: %w", err)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AIHistory{}, fmt.Errorf("begin save AI history tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
INSERT INTO ai_histories(id, kind, title, provider_id, model_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'saved', ?, ?)
ON CONFLICT(id) DO UPDATE SET
	kind = excluded.kind,
	title = excluded.title,
	provider_id = excluded.provider_id,
	model_id = excluded.model_id,
	status = 'saved',
	updated_at = excluded.updated_at
`, input.ID, input.Kind, input.Title, input.ProviderID, input.ModelID, createdAt, now)
	if err != nil {
		return AIHistory{}, fmt.Errorf("save AI history: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_history_messages WHERE history_id = ?", input.ID); err != nil {
		return AIHistory{}, fmt.Errorf("replace AI history messages: %w", err)
	}
	for sequence, message := range input.Messages {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ai_history_messages(history_id, sequence, role, content, created_at)
VALUES (?, ?, ?, ?, ?)
`, input.ID, sequence+1, message.Role, message.Content, now); err != nil {
			return AIHistory{}, fmt.Errorf("insert AI history message: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_history_sources WHERE history_id = ?", input.ID); err != nil {
		return AIHistory{}, fmt.Errorf("replace AI history sources: %w", err)
	}
	for _, source := range input.Sources {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ai_history_sources(history_id, note_id, input_revision)
VALUES (?, ?, ?)
`, input.ID, source.NoteID, source.InputRevision); err != nil {
			return AIHistory{}, fmt.Errorf("insert AI history source: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return AIHistory{}, fmt.Errorf("commit AI history: %w", err)
	}
	return r.getHistory(ctx, input.ID)
}

func (r *Repository) listHistories(ctx context.Context) ([]AIHistory, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
SELECT history.id, history.kind, history.title, history.provider_id, history.model_id,
       %s, history.created_at, history.updated_at
FROM ai_histories history
ORDER BY history.updated_at DESC, history.id ASC
LIMIT %d
`, historyStatusExpression, aiRecordListLimit))
	if err != nil {
		return nil, fmt.Errorf("list AI histories: %w", err)
	}
	defer rows.Close()

	items := make([]AIHistory, 0)
	for rows.Next() {
		item, err := scanHistory(rows)
		if err != nil {
			return nil, err
		}
		item.Sources, err = r.listHistorySources(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI histories: %w", err)
	}
	return items, nil
}

func (r *Repository) getHistory(ctx context.Context, id string) (AIHistory, error) {
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`
SELECT history.id, history.kind, history.title, history.provider_id, history.model_id,
       %s, history.created_at, history.updated_at
FROM ai_histories history
WHERE history.id = ?
`, historyStatusExpression), id)
	item, err := scanHistory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AIHistory{}, ErrHistoryNotFound
	}
	if err != nil {
		return AIHistory{}, err
	}
	item.Messages, err = r.listHistoryMessages(ctx, id)
	if err != nil {
		return AIHistory{}, err
	}
	item.Sources, err = r.listHistorySources(ctx, id)
	if err != nil {
		return AIHistory{}, err
	}
	return item, nil
}

func (r *Repository) deleteHistory(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete AI history tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, "DELETE FROM ai_histories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete AI history: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted AI history count: %w", err)
	}
	if affected == 0 {
		return ErrHistoryNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete AI history: %w", err)
	}
	return nil
}

func (r *Repository) deleteAllHistories(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete all AI histories tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_histories"); err != nil {
		return fmt.Errorf("delete all AI histories: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete all AI histories: %w", err)
	}
	return nil
}

func (r *Repository) listHistoryMessages(ctx context.Context, historyID string) ([]AIConversationMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT role, content
FROM ai_history_messages
WHERE history_id = ?
ORDER BY sequence ASC
`, historyID)
	if err != nil {
		return nil, fmt.Errorf("list AI history messages: %w", err)
	}
	defer rows.Close()

	items := make([]AIConversationMessage, 0)
	for rows.Next() {
		item := AIConversationMessage{}
		if err := rows.Scan(&item.Role, &item.Content); err != nil {
			return nil, fmt.Errorf("scan AI history message: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI history messages: %w", err)
	}
	return items, nil
}

func (r *Repository) listHistorySources(ctx context.Context, historyID string) ([]AIHistorySource, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT note_id, input_revision
FROM ai_history_sources
WHERE history_id = ?
ORDER BY note_id ASC
`, historyID)
	if err != nil {
		return nil, fmt.Errorf("list AI history sources: %w", err)
	}
	defer rows.Close()

	items := make([]AIHistorySource, 0)
	for rows.Next() {
		item := AIHistorySource{}
		if err := rows.Scan(&item.NoteID, &item.InputRevision); err != nil {
			return nil, fmt.Errorf("scan AI history source: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI history sources: %w", err)
	}
	return items, nil
}

func (r *Repository) saveArtifact(ctx context.Context, input SaveAIArtifactInput) (AIArtifact, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := now
	if input.ID != "" {
		if err := r.db.QueryRowContext(ctx, "SELECT created_at FROM ai_artifacts WHERE id = ?", input.ID).Scan(&createdAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return AIArtifact{}, fmt.Errorf("read AI artifact creation time: %w", err)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AIArtifact{}, fmt.Errorf("begin save AI artifact tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
INSERT INTO ai_artifacts(id, kind, title, provider_id, model_id, content, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 'saved', ?, ?)
ON CONFLICT(id) DO UPDATE SET
	kind = excluded.kind,
	title = excluded.title,
	provider_id = excluded.provider_id,
	model_id = excluded.model_id,
	content = excluded.content,
	status = 'saved',
	updated_at = excluded.updated_at
`, input.ID, input.Kind, input.Title, input.ProviderID, input.ModelID, input.Content, createdAt, now)
	if err != nil {
		return AIArtifact{}, fmt.Errorf("save AI artifact: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_artifact_sources WHERE artifact_id = ?", input.ID); err != nil {
		return AIArtifact{}, fmt.Errorf("replace AI artifact sources: %w", err)
	}
	for _, source := range input.Sources {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ai_artifact_sources(artifact_id, note_id, input_revision)
VALUES (?, ?, ?)
`, input.ID, source.NoteID, source.InputRevision); err != nil {
			return AIArtifact{}, fmt.Errorf("insert AI artifact source: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return AIArtifact{}, fmt.Errorf("commit AI artifact: %w", err)
	}
	return r.getArtifact(ctx, input.ID)
}

func (r *Repository) listArtifacts(ctx context.Context) ([]AIArtifact, error) {
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
SELECT artifact.id, artifact.kind, artifact.title, artifact.provider_id, artifact.model_id,
       artifact.content, %s, artifact.created_at, artifact.updated_at
FROM ai_artifacts artifact
ORDER BY artifact.updated_at DESC, artifact.id ASC
LIMIT %d
`, artifactStatusExpression, aiRecordListLimit))
	if err != nil {
		return nil, fmt.Errorf("list AI artifacts: %w", err)
	}
	defer rows.Close()

	items := make([]AIArtifact, 0)
	for rows.Next() {
		item, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		item.Sources, err = r.listArtifactSources(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI artifacts: %w", err)
	}
	return items, nil
}

func (r *Repository) getArtifact(ctx context.Context, id string) (AIArtifact, error) {
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`
SELECT artifact.id, artifact.kind, artifact.title, artifact.provider_id, artifact.model_id,
       artifact.content, %s, artifact.created_at, artifact.updated_at
FROM ai_artifacts artifact
WHERE artifact.id = ?
`, artifactStatusExpression), id)
	item, err := scanArtifact(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AIArtifact{}, ErrArtifactNotFound
	}
	if err != nil {
		return AIArtifact{}, err
	}
	item.Sources, err = r.listArtifactSources(ctx, id)
	if err != nil {
		return AIArtifact{}, err
	}
	return item, nil
}

func (r *Repository) deleteArtifact(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete AI artifact tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, "DELETE FROM ai_artifacts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete AI artifact: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted AI artifact count: %w", err)
	}
	if affected == 0 {
		return ErrArtifactNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete AI artifact: %w", err)
	}
	return nil
}

func (r *Repository) deleteAllArtifacts(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete all AI artifacts tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_artifacts"); err != nil {
		return fmt.Errorf("delete all AI artifacts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete all AI artifacts: %w", err)
	}
	return nil
}

func (r *Repository) deleteArtifactsByKinds(ctx context.Context, kinds []ArtifactKind) error {
	if len(kinds) == 0 {
		return ErrInputInvalid
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")
	args := make([]any, 0, len(kinds))
	for _, kind := range kinds {
		args = append(args, kind)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete selected AI artifacts tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_artifacts WHERE kind IN ("+placeholders+")", args...); err != nil {
		return fmt.Errorf("delete selected AI artifacts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete selected AI artifacts: %w", err)
	}
	return nil
}

func (r *Repository) listArtifactSources(ctx context.Context, artifactID string) ([]AIHistorySource, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT note_id, input_revision
FROM ai_artifact_sources
WHERE artifact_id = ?
ORDER BY note_id ASC
`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("list AI artifact sources: %w", err)
	}
	defer rows.Close()

	items := make([]AIHistorySource, 0)
	for rows.Next() {
		item := AIHistorySource{}
		if err := rows.Scan(&item.NoteID, &item.InputRevision); err != nil {
			return nil, fmt.Errorf("scan AI artifact source: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI artifact sources: %w", err)
	}
	return items, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHistory(row rowScanner) (AIHistory, error) {
	item := AIHistory{}
	var kind string
	var providerID string
	var status string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&item.ID, &kind, &item.Title, &providerID, &item.ModelID, &status, &createdAt, &updatedAt); err != nil {
		return AIHistory{}, err
	}
	item.Kind = AssistantKind(kind)
	item.ProviderID = ProviderID(providerID)
	item.Status = AIRecordStatus(status)
	var err error
	item.CreatedAt, err = parseAIRecordTime(createdAt)
	if err != nil {
		return AIHistory{}, err
	}
	item.UpdatedAt, err = parseAIRecordTime(updatedAt)
	if err != nil {
		return AIHistory{}, err
	}
	return item, nil
}

func scanArtifact(row rowScanner) (AIArtifact, error) {
	item := AIArtifact{}
	var kind string
	var providerID string
	var status string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&item.ID, &kind, &item.Title, &providerID, &item.ModelID, &item.Content, &status, &createdAt, &updatedAt); err != nil {
		return AIArtifact{}, err
	}
	item.Kind = ArtifactKind(kind)
	item.ProviderID = ProviderID(providerID)
	item.Status = AIRecordStatus(status)
	var err error
	item.CreatedAt, err = parseAIRecordTime(createdAt)
	if err != nil {
		return AIArtifact{}, err
	}
	item.UpdatedAt, err = parseAIRecordTime(updatedAt)
	if err != nil {
		return AIArtifact{}, err
	}
	return item, nil
}

func parseAIRecordTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse AI record timestamp: %w", err)
	}
	return parsed, nil
}

func normalizeRecordTitle(value string) string {
	return strings.TrimSpace(value)
}
