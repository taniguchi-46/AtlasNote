package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const tagsTable = "tags"
const noteTagsTable = "note_tags"

type tagRecord struct {
	Tag
	NormalizedName string
}

type tagScanner interface {
	Scan(...any) error
}

func (r *Repository) CreateTag(ctx context.Context, record tagRecord) error {
	return r.CreateTagWithSync(ctx, record, nil)
}

func (r *Repository) CreateTagWithSync(ctx context.Context, record tagRecord, changes []SyncChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag insert tx: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Insert(tagsTable).
		Columns("id", "name", "normalized_name", "created_at", "updated_at").
		Values(record.ID, record.Name, record.NormalizedName, formatTime(record.CreatedAt), formatTime(record.UpdatedAt)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build tag insert: %w", err)
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert tag: %w", err)
	}
	if err := r.recordSyncChanges(ctx, tx, changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tag insert tx: %w", err)
	}

	return nil
}

func (r *Repository) GetTag(ctx context.Context, id string) (tagRecord, error) {
	query, args, err := psql.Select("id", "name", "normalized_name", "created_at", "updated_at").
		From(tagsTable).
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return tagRecord{}, fmt.Errorf("build tag get: %w", err)
	}

	record, err := scanTag(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return tagRecord{}, ErrTagNotFound
	}
	if err != nil {
		return tagRecord{}, fmt.Errorf("get tag: %w", err)
	}

	return record, nil
}

func (r *Repository) GetTagByNormalizedName(ctx context.Context, normalizedName string) (tagRecord, error) {
	query, args, err := psql.Select("id", "name", "normalized_name", "created_at", "updated_at").
		From(tagsTable).
		Where(sq.Eq{"normalized_name": normalizedName}).
		Limit(1).
		ToSql()
	if err != nil {
		return tagRecord{}, fmt.Errorf("build tag normalized name lookup: %w", err)
	}

	record, err := scanTag(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return tagRecord{}, ErrTagNotFound
	}
	if err != nil {
		return tagRecord{}, fmt.Errorf("get tag by normalized name: %w", err)
	}

	return record, nil
}

func (r *Repository) ListTags(ctx context.Context) ([]Tag, error) {
	query, args, err := psql.Select("id", "name", "normalized_name", "created_at", "updated_at").
		From(tagsTable).
		OrderBy("normalized_name ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build tag list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

func (r *Repository) ListTagRecords(ctx context.Context) ([]tagRecord, error) {
	query, args, err := psql.Select("id", "name", "normalized_name", "created_at", "updated_at").
		From(tagsTable).
		OrderBy("normalized_name ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build tag record list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tag records: %w", err)
	}
	defer rows.Close()
	records := make([]tagRecord, 0)
	for rows.Next() {
		record, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tag records: %w", err)
	}
	return records, nil
}

func (r *Repository) ListNoteTags(ctx context.Context, noteID string) ([]Tag, error) {
	query, args, err := psql.Select(
		"tags.id",
		"tags.name",
		"tags.normalized_name",
		"tags.created_at",
		"tags.updated_at",
	).
		From(tagsTable).
		Join("note_tags ON note_tags.tag_id = tags.id").
		Where(sq.Eq{"note_tags.note_id": noteID}).
		OrderBy("tags.normalized_name ASC", "tags.id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build note tag list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list note tags: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

func (r *Repository) ListNoteTagIDs(ctx context.Context, noteID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT tag_id
FROM note_tags
WHERE note_id = ?
ORDER BY tag_id
`, noteID)
	if err != nil {
		return nil, fmt.Errorf("list note tag ids: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan note tag id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note tag ids: %w", err)
	}
	return ids, nil
}

func (r *Repository) UpdateTag(ctx context.Context, record tagRecord) error {
	return r.UpdateTagWithSync(ctx, record, nil)
}

func (r *Repository) SetTagSyncTimes(ctx context.Context, id string, createdAt time.Time, updatedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE tags
SET created_at = ?, updated_at = ?
WHERE id = ?
`, formatTime(createdAt), formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("set synced tag timestamps: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read synced tag timestamp result: %w", err)
	}
	if affected != 1 {
		return ErrTagNotFound
	}
	return nil
}

func (r *Repository) UpdateTagWithSync(ctx context.Context, record tagRecord, changes []SyncChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag update tx: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Update(tagsTable).
		Set("name", record.Name).
		Set("normalized_name", record.NormalizedName).
		Set("updated_at", formatTime(record.UpdatedAt)).
		Where(sq.Eq{"id": record.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build tag update: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read tag update result: %w", err)
	}
	if updated == 0 {
		return ErrTagNotFound
	}
	if err := r.recordSyncChanges(ctx, tx, changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tag update tx: %w", err)
	}

	return nil
}

func (r *Repository) DeleteTag(ctx context.Context, id string) error {
	return r.DeleteTagWithSync(ctx, id, nil)
}

func (r *Repository) DeleteTagWithSync(ctx context.Context, id string, changes []SyncChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag delete tx: %w", err)
	}
	defer tx.Rollback()

	query, args, err := psql.Delete(tagsTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build tag delete: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read tag delete result: %w", err)
	}
	if deleted == 0 {
		return ErrTagNotFound
	}
	if err := r.recordSyncChanges(ctx, tx, changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tag delete tx: %w", err)
	}

	return nil
}

func (r *Repository) ReplaceNoteTags(ctx context.Context, noteID string, tagIDs []string) error {
	return r.ReplaceNoteTagsWithSync(ctx, noteID, tagIDs, nil)
}

func (r *Repository) ReplaceNoteTagsWithSync(ctx context.Context, noteID string, tagIDs []string, changes []SyncChange) error {
	tagIDs = deduplicateTagIDs(tagIDs)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note tag replace tx: %w", err)
	}
	defer tx.Rollback()

	if err := ensureNoteExists(ctx, tx, noteID); err != nil {
		return err
	}
	if err := ensureTagsExist(ctx, tx, tagIDs); err != nil {
		return err
	}

	deleteQuery, deleteArgs, err := psql.Delete(noteTagsTable).
		Where(sq.Eq{"note_id": noteID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note tag delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("delete note tags: %w", err)
	}

	for _, tagID := range tagIDs {
		insertQuery, insertArgs, err := psql.Insert(noteTagsTable).
			Columns("note_id", "tag_id").
			Values(noteID, tagID).
			ToSql()
		if err != nil {
			return fmt.Errorf("build note tag insert: %w", err)
		}
		if _, err := tx.ExecContext(ctx, insertQuery, insertArgs...); err != nil {
			return fmt.Errorf("insert note tag: %w", err)
		}
	}
	if err := r.recordSyncChanges(ctx, tx, changes); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note tag replace tx: %w", err)
	}

	return nil
}

func (r *Repository) ListNoteIDsForTag(ctx context.Context, tagID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT note_id
FROM note_tags
WHERE tag_id = ?
ORDER BY note_id
`, tagID)
	if err != nil {
		return nil, fmt.Errorf("list notes for tag: %w", err)
	}
	defer rows.Close()

	noteIDs := make([]string, 0)
	for rows.Next() {
		var noteID string
		if err := rows.Scan(&noteID); err != nil {
			return nil, fmt.Errorf("scan note for tag: %w", err)
		}
		noteIDs = append(noteIDs, noteID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes for tag: %w", err)
	}
	return noteIDs, nil
}

func ensureNoteExists(ctx context.Context, executor sqlQueryExecutor, noteID string) error {
	query, args, err := psql.Select("id").
		From(notesTable).
		Where(sq.Eq{"id": noteID}).
		Limit(1).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note tag note lookup: %w", err)
	}

	var id string
	err = executor.QueryRowContext(ctx, query, args...).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check note for tag replace: %w", err)
	}

	return nil
}

func ensureTagsExist(ctx context.Context, executor sqlQueryExecutor, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}

	query, args, err := psql.Select("COUNT(*)").
		From(tagsTable).
		Where(sq.Eq{"id": tagIDs}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build tag existence lookup: %w", err)
	}

	var count int
	if err := executor.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return fmt.Errorf("check tags for note replace: %w", err)
	}
	if count != len(tagIDs) {
		return ErrTagNotFound
	}

	return nil
}

func scanTags(rows *sql.Rows) ([]Tag, error) {
	tags := make([]Tag, 0)
	for rows.Next() {
		record, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, record.Tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

func scanTag(scanner tagScanner) (tagRecord, error) {
	var record tagRecord
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&record.ID,
		&record.Name,
		&record.NormalizedName,
		&createdAt,
		&updatedAt,
	); err != nil {
		return tagRecord{}, err
	}

	var err error
	record.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return tagRecord{}, err
	}
	record.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return tagRecord{}, err
	}

	return record, nil
}
