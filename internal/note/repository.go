package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const notesTable = "notes"

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Question)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, record Record) error {
	query, args, err := psql.Insert(notesTable).
		Columns("id", "notebook_id", "title", "content_path", "is_favorite", "is_pinned", "is_trashed", "created_at", "updated_at").
		Values(record.ID, record.NotebookID, record.Title, record.ContentPath, record.IsFavorite, record.IsPinned, record.IsTrashed, formatTime(record.CreatedAt), formatTime(record.UpdatedAt)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note insert: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert note: %w", err)
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]Summary, error) {
	query, args, err := psql.Select("id", "notebook_id", "title", "is_favorite", "is_pinned", "is_trashed", "created_at", "updated_at").
		From(notesTable).
		OrderBy("updated_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build note list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	notes := make([]Summary, 0)
	for rows.Next() {
		var note Summary
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&note.ID, &note.NotebookID, &note.Title, &note.IsFavorite, &note.IsPinned, &note.IsTrashed, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan note summary: %w", err)
		}

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

func (r *Repository) Get(ctx context.Context, id string) (Record, error) {
	query, args, err := psql.Select("id", "notebook_id", "title", "content_path", "is_favorite", "is_pinned", "is_trashed", "created_at", "updated_at").
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
	query, args, err := psql.Update(notesTable).
		Set("notebook_id", record.NotebookID).
		Set("title", record.Title).
		Set("is_favorite", record.IsFavorite).
		Set("is_pinned", record.IsPinned).
		Set("is_trashed", record.IsTrashed).
		Set("updated_at", formatTime(record.UpdatedAt)).
		Where(sq.Eq{"id": record.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note update: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
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

func (r *Repository) Delete(ctx context.Context, id string) error {
	query, args, err := psql.Delete(notesTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build note delete: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
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
