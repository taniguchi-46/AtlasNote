package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const notesTable = "notes"
const storageOperationsTable = "note_storage_operations"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Question)

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type sqlQueryExecutor interface {
	sqlExecutor
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, record Record) error {
	return insertRecord(ctx, r.db, record)
}

func insertRecord(ctx context.Context, executor sqlExecutor, record Record) error {
	query, args, err := psql.Insert(notesTable).
		Columns("id", "notebook_id", "title", "content_path", "is_favorite", "is_pinned", "is_trashed", "revision", "created_at", "updated_at").
		Values(record.ID, record.NotebookID, record.Title, record.ContentPath, record.IsFavorite, record.IsPinned, record.IsTrashed, record.Revision, formatTime(record.CreatedAt), formatTime(record.UpdatedAt)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note insert: %w", err)
	}

	if _, err := executor.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert note: %w", err)
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]Summary, error) {
	query, args, err := psql.Select("id", "notebook_id", "title", "is_favorite", "is_pinned", "is_trashed", "revision", "created_at", "updated_at").
		From(notesTable).
		OrderBy("updated_at DESC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build note list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	return scanSummaries(rows)
}

func (r *Repository) ListPage(ctx context.Context, input NoteListInput) (NoteListResult, error) {
	page, pageSize, err := normalizeNoteListInput(input)
	if err != nil {
		return NoteListResult{}, err
	}

	where, err := noteListWhere(input)
	if err != nil {
		return NoteListResult{}, err
	}

	countQuery := psql.Select("COUNT(*)").From(notesTable)
	if len(where) > 0 {
		countQuery = countQuery.Where(where)
	}
	countSQL, countArgs, err := countQuery.ToSql()
	if err != nil {
		return NoteListResult{}, fmt.Errorf("build note count: %w", err)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return NoteListResult{}, fmt.Errorf("count notes: %w", err)
	}

	noteQuery := psql.Select("id", "notebook_id", "title", "is_favorite", "is_pinned", "is_trashed", "revision", "created_at", "updated_at").
		From(notesTable)
	if len(where) > 0 {
		noteQuery = noteQuery.Where(where)
	}
	query, args, err := noteQuery.
		OrderBy("updated_at DESC", "id ASC").
		Limit(uint64(pageSize)).
		Offset(uint64((page - 1) * pageSize)).
		ToSql()
	if err != nil {
		return NoteListResult{}, fmt.Errorf("build note list page: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return NoteListResult{}, fmt.Errorf("list note page: %w", err)
	}
	defer rows.Close()

	items, err := scanSummaries(rows)
	if err != nil {
		return NoteListResult{}, err
	}

	return NoteListResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasNext:  page*pageSize < total,
	}, nil
}

func noteListWhere(input NoteListInput) (sq.And, error) {
	if input.TagID == nil {
		return nil, nil
	}

	tagID := strings.TrimSpace(*input.TagID)
	if tagID == "" {
		return nil, fmt.Errorf("%w: tag id must not be empty", ErrValidation)
	}

	// Tag navigation is an active-note view, so trashed notes are excluded at
	// the same time as the tag relation is applied. The unfiltered list keeps
	// its existing behavior and lets the UI switch between active and trash.
	return sq.And{
		sq.Expr("EXISTS (SELECT 1 FROM note_tags WHERE note_tags.note_id = notes.id AND note_tags.tag_id = ?)", tagID),
		sq.Eq{"notes.is_trashed": false},
	}, nil
}

func scanSummaries(rows *sql.Rows) ([]Summary, error) {
	notes := make([]Summary, 0)
	for rows.Next() {
		var note Summary
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&note.ID, &note.NotebookID, &note.Title, &note.IsFavorite, &note.IsPinned, &note.IsTrashed, &note.Revision, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan note summary: %w", err)
		}

		var err error
		note.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		note.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note summaries: %w", err)
	}

	return notes, nil
}

func normalizeNoteListInput(input NoteListInput) (int, int, error) {
	page := input.Page
	if page == 0 {
		page = DefaultNoteListPage
	}
	if page < 1 || page > MaxNoteListPage {
		return 0, 0, fmt.Errorf("%w: note list page must be between 1 and %d", ErrValidation, MaxNoteListPage)
	}

	pageSize := input.PageSize
	if pageSize == 0 {
		pageSize = DefaultNoteListPageSize
	}
	if pageSize < 1 || pageSize > MaxNoteListPageSize {
		return 0, 0, fmt.Errorf("%w: note list page size must be between 1 and %d", ErrValidation, MaxNoteListPageSize)
	}

	return page, pageSize, nil
}

func (r *Repository) Get(ctx context.Context, id string) (Record, error) {
	query, args, err := psql.Select("id", "notebook_id", "title", "content_path", "is_favorite", "is_pinned", "is_trashed", "revision", "created_at", "updated_at").
		From(notesTable).
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return Record{}, fmt.Errorf("build note get: %w", err)
	}

	var record Record
	var createdAt string
	var updatedAt string
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&record.ID,
		&record.NotebookID,
		&record.Title,
		&record.ContentPath,
		&record.IsFavorite,
		&record.IsPinned,
		&record.IsTrashed,
		&record.Revision,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("get note: %w", err)
	}

	record.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Record{}, err
	}
	record.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}

func (r *Repository) Update(ctx context.Context, record Record) error {
	return updateRecord(ctx, r.db, record)
}

func (r *Repository) UpdateCAS(ctx context.Context, record Record, expectedRevision int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin note CAS update tx: %w", err)
	}
	defer tx.Rollback()

	nextRevision, err := updateRecordCAS(ctx, tx, record, expectedRevision)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit note CAS update tx: %w", err)
	}

	return nextRevision, nil
}

func updateRecord(ctx context.Context, executor sqlExecutor, record Record) error {
	query, args, err := psql.Update(notesTable).
		Set("notebook_id", record.NotebookID).
		Set("title", record.Title).
		Set("is_favorite", record.IsFavorite).
		Set("is_pinned", record.IsPinned).
		Set("is_trashed", record.IsTrashed).
		Set("revision", record.Revision).
		Set("updated_at", formatTime(record.UpdatedAt)).
		Where(sq.Eq{"id": record.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note update: %w", err)
	}

	result, err := executor.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read note update result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func updateRecordCAS(ctx context.Context, executor sqlQueryExecutor, record Record, expectedRevision int64) (int64, error) {
	query, args, err := psql.Update(notesTable).
		Set("notebook_id", record.NotebookID).
		Set("title", record.Title).
		Set("is_favorite", record.IsFavorite).
		Set("is_pinned", record.IsPinned).
		Set("is_trashed", record.IsTrashed).
		Set("revision", sq.Expr("revision + 1")).
		Set("updated_at", formatTime(record.UpdatedAt)).
		Where(sq.Eq{"id": record.ID, "revision": expectedRevision}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build note CAS update: %w", err)
	}

	result, err := executor.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("update note with CAS: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read note CAS update result: %w", err)
	}
	if updated == 0 {
		return 0, revisionMismatchError(ctx, executor, record.ID, expectedRevision)
	}

	return expectedRevision + 1, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return deleteRecord(ctx, r.db, id)
}

func (r *Repository) DeleteCAS(ctx context.Context, id string, expectedRevision int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note CAS delete tx: %w", err)
	}
	defer tx.Rollback()

	if err := deleteRecordCAS(ctx, tx, id, expectedRevision); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note CAS delete tx: %w", err)
	}

	return nil
}

func deleteRecord(ctx context.Context, executor sqlExecutor, id string) error {
	query, args, err := psql.Delete(notesTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build note delete: %w", err)
	}

	result, err := executor.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read note delete result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func deleteRecordCAS(ctx context.Context, executor sqlQueryExecutor, id string, expectedRevision int64) error {
	query, args, err := psql.Delete(notesTable).
		Where(sq.Eq{"id": id, "revision": expectedRevision}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note CAS delete: %w", err)
	}

	result, err := executor.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete note with CAS: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read note CAS delete result: %w", err)
	}
	if deleted == 0 {
		return revisionMismatchError(ctx, executor, id, expectedRevision)
	}

	return nil
}

func (r *Repository) CreateWithStorageOperation(ctx context.Context, record Record, operation StorageOperation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note create tx: %w", err)
	}
	defer tx.Rollback()

	if err := insertRecord(ctx, tx, record); err != nil {
		return err
	}
	if err := insertStorageOperation(ctx, tx, operation); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note create tx: %w", err)
	}

	return nil
}

func (r *Repository) UpdateWithStorageOperation(ctx context.Context, record Record, operation StorageOperation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note update tx: %w", err)
	}
	defer tx.Rollback()

	if err := updateRecord(ctx, tx, record); err != nil {
		return err
	}
	if err := insertStorageOperation(ctx, tx, operation); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note update tx: %w", err)
	}

	return nil
}

func (r *Repository) UpdateWithStorageOperationCAS(
	ctx context.Context,
	record Record,
	operation StorageOperation,
	expectedRevision int64,
) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin note CAS update tx: %w", err)
	}
	defer tx.Rollback()

	nextRevision, err := updateRecordCAS(ctx, tx, record, expectedRevision)
	if err != nil {
		return 0, err
	}
	if err := insertStorageOperation(ctx, tx, operation); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit note CAS update tx: %w", err)
	}

	return nextRevision, nil
}

func (r *Repository) BeginStorageOperation(ctx context.Context, operation StorageOperation) error {
	return insertStorageOperation(ctx, r.db, operation)
}

func (r *Repository) RollbackCreatedNote(ctx context.Context, noteID string, operationID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note create rollback tx: %w", err)
	}
	defer tx.Rollback()

	if err := deleteRecord(ctx, tx, noteID); err != nil {
		return err
	}
	if err := deleteStorageOperation(ctx, tx, operationID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note create rollback tx: %w", err)
	}

	return nil
}

func (r *Repository) RollbackUpdatedNote(ctx context.Context, previous Record, operationID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin note update rollback tx: %w", err)
	}
	defer tx.Rollback()

	if err := updateRecord(ctx, tx, previous); err != nil {
		return err
	}
	if err := deleteStorageOperation(ctx, tx, operationID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit note update rollback tx: %w", err)
	}

	return nil
}

func (r *Repository) CompleteStorageOperation(ctx context.Context, operationID string) error {
	return deleteStorageOperation(ctx, r.db, operationID)
}

func (r *Repository) ListStorageOperations(ctx context.Context) ([]StorageOperation, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT operation_id, note_id, operation_type, content_hash, created_at
FROM note_storage_operations
ORDER BY created_at, operation_id
`)
	if err != nil {
		return nil, fmt.Errorf("list note storage operations: %w", err)
	}
	defer rows.Close()

	operations := make([]StorageOperation, 0)
	for rows.Next() {
		var operation StorageOperation
		var createdAt string
		if err := rows.Scan(&operation.ID, &operation.NoteID, &operation.Type, &operation.ContentHash, &createdAt); err != nil {
			return nil, fmt.Errorf("scan note storage operation: %w", err)
		}
		operation.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note storage operations: %w", err)
	}

	return operations, nil
}

func (r *Repository) ListRecords(ctx context.Context) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, notebook_id, title, content_path, is_favorite, is_pinned, is_trashed, revision, created_at, updated_at
FROM notes
`)
	if err != nil {
		return nil, fmt.Errorf("list note records: %w", err)
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var record Record
		var createdAt string
		var updatedAt string
		if err := rows.Scan(
			&record.ID,
			&record.NotebookID,
			&record.Title,
			&record.ContentPath,
			&record.IsFavorite,
			&record.IsPinned,
			&record.IsTrashed,
			&record.Revision,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan note record: %w", err)
		}

		var err error
		record.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		record.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note records: %w", err)
	}

	return records, nil
}

func revisionMismatchError(
	ctx context.Context,
	executor sqlQueryExecutor,
	noteID string,
	expectedRevision int64,
) error {
	query, args, err := psql.Select("revision").
		From(notesTable).
		Where(sq.Eq{"id": noteID}).
		Limit(1).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note revision lookup: %w", err)
	}

	var actualRevision int64
	err = executor.QueryRowContext(ctx, query, args...).Scan(&actualRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read note revision: %w", err)
	}

	return &RevisionConflict{
		Code:             ErrorCodeRevisionConflict,
		NoteID:           noteID,
		ExpectedRevision: expectedRevision,
		ActualRevision:   actualRevision,
	}
}

func insertStorageOperation(ctx context.Context, executor sqlExecutor, operation StorageOperation) error {
	query, args, err := psql.Insert(storageOperationsTable).
		Columns("operation_id", "note_id", "operation_type", "content_hash", "created_at").
		Values(operation.ID, operation.NoteID, operation.Type, operation.ContentHash, formatTime(operation.CreatedAt)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note storage operation insert: %w", err)
	}
	if _, err := executor.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert note storage operation: %w", err)
	}

	return nil
}

func deleteStorageOperation(ctx context.Context, executor sqlExecutor, operationID string) error {
	query, args, err := psql.Delete(storageOperationsTable).
		Where(sq.Eq{"operation_id": operationID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note storage operation delete: %w", err)
	}
	if _, err := executor.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete note storage operation: %w", err)
	}

	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse note timestamp: %w", err)
	}

	return parsed, nil
}
