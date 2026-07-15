package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const ErrBacklinkIndexFailedMessage = "backlink index is unavailable"

var ErrBacklinkIndexFailed = errors.New(ErrBacklinkIndexFailedMessage)

// GetNoteLinkIndexState returns the last canonical Markdown snapshot indexed
// for one source note. A missing row means the derived index needs rebuilding.
func (r *Repository) GetNoteLinkIndexState(ctx context.Context, noteID string) (NoteLinkIndexState, bool, error) {
	if noteID == "" {
		return NoteLinkIndexState{}, false, fmt.Errorf("get note link state: note ID is required")
	}

	var state NoteLinkIndexState
	var contentMTimeUnix int64
	var indexedAt string
	err := r.db.QueryRowContext(ctx, `
SELECT note_id, indexed_revision, content_hash, content_mtime_ns, indexed_at
FROM note_link_state
WHERE note_id = ?
`, noteID).Scan(&state.NoteID, &state.IndexedRevision, &state.ContentHash, &contentMTimeUnix, &indexedAt)
	if err == sql.ErrNoRows {
		return NoteLinkIndexState{}, false, nil
	}
	if err != nil {
		return NoteLinkIndexState{}, false, fmt.Errorf("get note link state: %w", err)
	}

	state.ContentMTimeUnix = contentMTimeUnix
	parsedAt, err := parseTime(indexedAt)
	if err != nil {
		return NoteLinkIndexState{}, false, err
	}
	state.IndexedAt = parsedAt
	return state, true, nil
}

// ReplaceNoteLinks replaces one source note's outgoing links and its
// reconciliation state in one transaction. Unknown target IDs are ignored so
// a dangling Markdown link never prevents the note itself from being saved.
func (r *Repository) ReplaceNoteLinks(ctx context.Context, document NoteLinkDocument) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note link replace tx: %w", err)
	}
	defer tx.Rollback()

	if err := replaceNoteLinks(ctx, tx, document); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note link replace tx: %w", err)
	}
	return nil
}

// DeleteNoteLinkIndex removes one source note's links and state. Foreign-key
// cascades also remove rows when a target note is permanently deleted.
func (r *Repository) DeleteNoteLinkIndex(ctx context.Context, noteID string) error {
	if noteID == "" {
		return fmt.Errorf("delete note link index: note ID is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note link delete tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM note_links WHERE source_note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete note links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM note_link_state WHERE note_id = ?", noteID); err != nil {
		return fmt.Errorf("delete note link state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note link delete tx: %w", err)
	}
	return nil
}

// RefreshNoteLinkIndexState advances the state for metadata-only note changes.
func (r *Repository) RefreshNoteLinkIndexState(ctx context.Context, noteID string, revision int64, contentHash string) error {
	if noteID == "" {
		return fmt.Errorf("refresh note link state: note ID is required")
	}
	if revision < 1 {
		return fmt.Errorf("refresh note link state: revision must be at least 1")
	}

	result, err := r.db.ExecContext(ctx, `
UPDATE note_link_state
SET indexed_revision = ?, content_hash = ?, indexed_at = ?
WHERE note_id = ?
`, revision, contentHash, formatTime(time.Now().UTC()), noteID)
	if err != nil {
		return fmt.Errorf("refresh note link state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read note link state refresh result: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("refresh note link state: note ID is not indexed")
	}
	return nil
}

// ReplaceNoteLinkIndex rebuilds all outgoing links and their states in one
// transaction from canonical Markdown snapshots.
func (r *Repository) ReplaceNoteLinkIndex(ctx context.Context, documents []NoteLinkDocument) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note link rebuild tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM note_links"); err != nil {
		return fmt.Errorf("clear note links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM note_link_state"); err != nil {
		return fmt.Errorf("clear note link state: %w", err)
	}
	for _, document := range documents {
		if err := replaceNoteLinks(ctx, tx, document); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note link rebuild tx: %w", err)
	}
	return nil
}

func replaceNoteLinks(ctx context.Context, executor sqlQueryExecutor, document NoteLinkDocument) error {
	if document.SourceNoteID == "" {
		return fmt.Errorf("replace note links: source note ID is required")
	}
	if document.Revision < 1 {
		return fmt.Errorf("replace note links: revision must be at least 1")
	}
	if document.IndexedAt.IsZero() {
		document.IndexedAt = time.Now().UTC()
	}

	if _, err := executor.ExecContext(ctx, "DELETE FROM note_links WHERE source_note_id = ?", document.SourceNoteID); err != nil {
		return fmt.Errorf("replace note links: %w", err)
	}

	seenTargets := make(map[string]struct{}, len(document.TargetNoteIDs))
	for _, targetID := range document.TargetNoteIDs {
		if targetID == "" {
			continue
		}
		if _, exists := seenTargets[targetID]; exists {
			continue
		}
		seenTargets[targetID] = struct{}{}

		if _, err := executor.ExecContext(ctx, `
INSERT OR IGNORE INTO note_links(source_note_id, target_note_id)
SELECT ?, ?
WHERE EXISTS (SELECT 1 FROM notes WHERE id = ?)
`, document.SourceNoteID, targetID, targetID); err != nil {
			return fmt.Errorf("insert note link: %w", err)
		}
	}

	contentMTimeUnix := int64(0)
	if !document.ContentMTime.IsZero() {
		contentMTimeUnix = document.ContentMTime.UnixNano()
	}
	if _, err := executor.ExecContext(ctx, `
INSERT INTO note_link_state(note_id, indexed_revision, content_hash, content_mtime_ns, indexed_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(note_id) DO UPDATE SET
    indexed_revision = excluded.indexed_revision,
    content_hash = excluded.content_hash,
    content_mtime_ns = excluded.content_mtime_ns,
    indexed_at = excluded.indexed_at
`, document.SourceNoteID, document.Revision, document.ContentHash, contentMTimeUnix, formatTime(document.IndexedAt)); err != nil {
		return fmt.Errorf("upsert note link state: %w", err)
	}
	return nil
}

func (r *Repository) ListBacklinks(ctx context.Context, input BacklinkListInput) (BacklinkListResult, error) {
	page, pageSize, err := normalizeBacklinkInput(input)
	if err != nil {
		return BacklinkListResult{Items: make([]Summary, 0)}, err
	}

	whereArgs := []any{input.NoteID}
	var total int
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM note_links
JOIN notes ON notes.id = note_links.source_note_id
WHERE note_links.target_note_id = ? AND notes.is_trashed = 0
`, whereArgs...).Scan(&total); err != nil {
		return BacklinkListResult{Items: make([]Summary, 0)}, fmt.Errorf("count backlinks: %w", err)
	}

	result := BacklinkListResult{
		Items:    make([]Summary, 0),
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasNext:  page*pageSize < total,
	}
	if total == 0 {
		return result, nil
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT notes.id, notes.notebook_id, notes.title,
       notes.is_favorite, notes.is_pinned, notes.is_trashed,
       notes.revision, notes.created_at, notes.updated_at
FROM note_links
JOIN notes ON notes.id = note_links.source_note_id
WHERE note_links.target_note_id = ? AND notes.is_trashed = 0
ORDER BY notes.updated_at DESC, notes.id ASC
LIMIT ? OFFSET ?
`, input.NoteID, pageSize, (page-1)*pageSize)
	if err != nil {
		return BacklinkListResult{Items: make([]Summary, 0)}, fmt.Errorf("list backlinks: %w", err)
	}
	defer rows.Close()

	items, err := scanSummaries(rows)
	if err != nil {
		return BacklinkListResult{Items: make([]Summary, 0)}, err
	}
	result.Items = items
	return result, nil
}

func normalizeBacklinkInput(input BacklinkListInput) (int, int, error) {
	if input.NoteID == "" {
		return 0, 0, fmt.Errorf("%w: backlink note ID is required", ErrValidation)
	}

	page := input.Page
	if page == 0 {
		page = DefaultBacklinkPage
	}
	if page < 1 || page > MaxBacklinkPage {
		return 0, 0, fmt.Errorf("%w: backlink page must be between 1 and %d", ErrValidation, MaxBacklinkPage)
	}

	pageSize := input.PageSize
	if pageSize == 0 {
		pageSize = DefaultBacklinkPageSize
	}
	if pageSize < 1 || pageSize > MaxBacklinkPageSize {
		return 0, 0, fmt.Errorf("%w: backlink page size must be between 1 and %d", ErrValidation, MaxBacklinkPageSize)
	}
	return page, pageSize, nil
}
