package note

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const notebooksTable = "notebooks"

func (r *Repository) CreateNotebook(ctx context.Context, nb Notebook) error {
	query, args, err := psql.Insert(notebooksTable).
		Columns("id", "parent_id", "name", "icon", "created_at", "updated_at").
		Values(nb.ID, nb.ParentID, nb.Name, nb.Icon, formatTime(nb.CreatedAt), formatTime(nb.UpdatedAt)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build notebook insert: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert notebook: %w", err)
	}

	return nil
}

func (r *Repository) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	query, args, err := psql.Select("id", "parent_id", "name", "icon", "created_at", "updated_at").
		From(notebooksTable).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build notebook list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	defer rows.Close()

	notebooks := make([]Notebook, 0)
	for rows.Next() {
		var nb Notebook
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&nb.ID, &nb.ParentID, &nb.Name, &nb.Icon, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan notebook: %w", err)
		}

		var err error
		nb.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		nb.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}

		notebooks = append(notebooks, nb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notebooks: %w", err)
	}

	return notebooks, nil
}

func (r *Repository) GetNotebook(ctx context.Context, id string) (Notebook, error) {
	query, args, err := psql.Select("id", "parent_id", "name", "icon", "created_at", "updated_at").
		From(notebooksTable).
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return Notebook{}, fmt.Errorf("build notebook get: %w", err)
	}

	var nb Notebook
	var createdAt string
	var updatedAt string
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&nb.ID,
		&nb.ParentID,
		&nb.Name,
		&nb.Icon,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Notebook{}, ErrNotFound
	}
	if err != nil {
		return Notebook{}, fmt.Errorf("get notebook: %w", err)
	}

	nb.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Notebook{}, err
	}
	nb.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Notebook{}, err
	}

	return nb, nil
}

func (r *Repository) UpdateNotebook(ctx context.Context, nb Notebook) error {
	query, args, err := psql.Update(notebooksTable).
		Set("parent_id", nb.ParentID).
		Set("name", nb.Name).
		Set("icon", nb.Icon).
		Set("updated_at", formatTime(nb.UpdatedAt)).
		Where(sq.Eq{"id": nb.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build notebook update: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update notebook: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read notebook update result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteNotebook(ctx context.Context, id string) error {
	query, args, err := psql.Delete(notebooksTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build notebook delete: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete notebook: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read notebook delete result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteNotebookWithNotesTrashed(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin notebook delete tx: %w", err)
	}
	defer tx.Rollback()

	if err := trashNotesInNotebookTree(ctx, tx, id, time.Now().UTC()); err != nil {
		return err
	}

	if err := deleteNotebookInTx(ctx, tx, id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit notebook delete tx: %w", err)
	}

	return nil
}

func (r *Repository) DeleteNotebookKeepingNotes(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin notebook delete tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if err := detachChildNotebooks(ctx, tx, id, now); err != nil {
		return err
	}
	if err := detachNotesFromNotebook(ctx, tx, id, now); err != nil {
		return err
	}
	if err := deleteNotebookInTx(ctx, tx, id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit notebook delete tx: %w", err)
	}

	return nil
}

func trashNotesInNotebookTree(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `
WITH RECURSIVE notebook_tree(id) AS (
	SELECT id FROM notebooks WHERE id = ?
	UNION ALL
	SELECT notebooks.id
	FROM notebooks
	INNER JOIN notebook_tree ON notebooks.parent_id = notebook_tree.id
)
UPDATE notes
SET is_trashed = 1,
	updated_at = ?
WHERE notebook_id IN (SELECT id FROM notebook_tree)
`, id, formatTime(now))
	if err != nil {
		return fmt.Errorf("trash notes in notebook tree: %w", err)
	}
	if _, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("read notebook notes trash result: %w", err)
	}

	return nil
}

func detachChildNotebooks(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	query, args, err := psql.Update(notebooksTable).
		Set("parent_id", nil).
		Set("updated_at", formatTime(now)).
		Where(sq.Eq{"parent_id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build child notebook detach: %w", err)
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("detach child notebooks: %w", err)
	}

	return nil
}

func detachNotesFromNotebook(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	query, args, err := psql.Update(notesTable).
		Set("notebook_id", nil).
		Set("updated_at", formatTime(now)).
		Where(sq.Eq{"notebook_id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build note notebook detach: %w", err)
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("detach notes from notebook: %w", err)
	}

	return nil
}

func deleteNotebookInTx(ctx context.Context, tx *sql.Tx, id string) error {
	query, args, err := psql.Delete(notebooksTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("build notebook delete: %w", err)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete notebook: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read notebook delete result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}
