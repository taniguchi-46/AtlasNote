package contentlock

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (m *Manager) listLockRecords(ctx context.Context) ([]lockRecord, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT id, target_type, target_id, kdf_salt, kdf_memory_kib, kdf_iterations,
       kdf_parallelism, wrap_nonce, wrapped_key, created_at, updated_at
FROM content_locks
ORDER BY target_type ASC, id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list content locks: %w", err)
	}
	defer rows.Close()
	locks := make([]lockRecord, 0)
	for rows.Next() {
		record, err := scanLockRecord(rows)
		if err != nil {
			return nil, err
		}
		locks = append(locks, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content locks: %w", err)
	}
	return locks, nil
}

func (m *Manager) getLockByTarget(ctx context.Context, target Target) (lockRecord, bool, error) {
	row := m.db.QueryRowContext(ctx, `
SELECT id, target_type, target_id, kdf_salt, kdf_memory_kib, kdf_iterations,
       kdf_parallelism, wrap_nonce, wrapped_key, created_at, updated_at
FROM content_locks
WHERE target_type = ? AND target_id = ?
`, target.Type, target.ID)
	record, err := scanLockRecord(row)
	if isNoRows(err) {
		return lockRecord{}, false, nil
	}
	if err != nil {
		return lockRecord{}, false, err
	}
	return record, true, nil
}

func (m *Manager) getLockByID(ctx context.Context, lockID string) (lockRecord, bool, error) {
	row := m.db.QueryRowContext(ctx, `
SELECT id, target_type, target_id, kdf_salt, kdf_memory_kib, kdf_iterations,
       kdf_parallelism, wrap_nonce, wrapped_key, created_at, updated_at
FROM content_locks
WHERE id = ?
`, lockID)
	record, err := scanLockRecord(row)
	if isNoRows(err) {
		return lockRecord{}, false, nil
	}
	if err != nil {
		return lockRecord{}, false, err
	}
	return record, true, nil
}

type lockScanner interface {
	Scan(...any) error
}

func scanLockRecord(row lockScanner) (lockRecord, error) {
	var record lockRecord
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&record.ID, &record.TargetType, &record.TargetID, &record.Salt, &record.MemoryKiB,
		&record.Iterations, &record.Parallelism, &record.WrapNonce, &record.WrappedKey,
		&createdAt, &updatedAt,
	); err != nil {
		return lockRecord{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return lockRecord{}, err
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return lockRecord{}, err
	}
	record.CreatedAt = created
	record.UpdatedAt = updated
	return record, nil
}

func (m *Manager) requireTargetExists(ctx context.Context, target Target) error {
	if target.Type == TargetSpace {
		return nil
	}
	table := "notes"
	if target.Type == TargetNotebook {
		table = "notebooks"
	}
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE id = ?", target.ID).Scan(&count); err != nil {
		return fmt.Errorf("verify lock target: %w", err)
	}
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (m *Manager) affectedNoteIDs(ctx context.Context, target Target) ([]string, error) {
	var query string
	var args []any
	switch target.Type {
	case TargetSpace:
		query = "SELECT id FROM notes ORDER BY id ASC"
	case TargetNote:
		query = "SELECT id FROM notes WHERE id = ? ORDER BY id ASC"
		args = []any{target.ID}
	case TargetNotebook:
		query = `
WITH RECURSIVE descendants(id) AS (
  SELECT id FROM notebooks WHERE id = ?
  UNION ALL
  SELECT notebook.id
  FROM notebooks notebook
  JOIN descendants parent ON notebook.parent_id = parent.id
)
SELECT id FROM notes WHERE notebook_id IN (SELECT id FROM descendants) ORDER BY id ASC
`
		args = []any{target.ID}
	default:
		return nil, ErrValidation
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list content lock affected notes: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan content lock affected note: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content lock affected notes: %w", err)
	}
	if target.Type == TargetNote && len(ids) != 1 {
		return nil, ErrNotFound
	}
	return ids, nil
}

func (m *Manager) hasExplicitLockInNotebookSubtree(ctx context.Context, notebookID string) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx, `
WITH RECURSIVE descendants(id) AS (
  SELECT id FROM notebooks WHERE id = ?
  UNION ALL
  SELECT notebook.id
  FROM notebooks notebook
  JOIN descendants parent ON notebook.parent_id = parent.id
)
SELECT COUNT(*)
FROM content_locks
WHERE (target_type = ? AND target_id IN (SELECT id FROM descendants))
   OR (target_type = ? AND target_id IN (
     SELECT id FROM notes WHERE notebook_id IN (SELECT id FROM descendants)
   ))
`, notebookID, TargetNotebook, TargetNote).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check notebook subtree content locks: %w", err)
	}
	return count > 0, nil
}

func (m *Manager) locksForNote(ctx context.Context, noteID string, pending pendingWrite) ([]lockRecord, error) {
	var notebookID *string
	if pending.setNotebookID {
		notebookID = copyStringPointer(pending.notebookID)
	} else {
		var value sql.NullString
		err := m.db.QueryRowContext(ctx, "SELECT notebook_id FROM notes WHERE id = ?", noteID).Scan(&value)
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("read note notebook for content lock: %w", err)
		}
		if value.Valid {
			copy := value.String
			notebookID = &copy
		}
	}
	return m.locksForScope(ctx, noteID, notebookID, pending.additionalLock, pending.removeLockID)
}

func (m *Manager) locksForNotebook(ctx context.Context, notebookID string) ([]lockRecord, error) {
	if err := m.requireTargetExists(ctx, Target{Type: TargetNotebook, ID: notebookID}); err != nil {
		return nil, err
	}
	copy := notebookID
	return m.locksForScope(ctx, "", &copy, nil, "")
}

func (m *Manager) locksForScope(ctx context.Context, noteID string, notebookID *string, additional *lockRecord, removeLockID string) ([]lockRecord, error) {
	locks, err := m.listLockRecords(ctx)
	if err != nil {
		return nil, err
	}
	ancestorIDs, err := m.notebookAncestorIDs(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	ancestors := make(map[string]struct{}, len(ancestorIDs))
	for _, id := range ancestorIDs {
		ancestors[id] = struct{}{}
	}
	result := make([]lockRecord, 0, len(locks)+1)
	for _, lock := range locks {
		if lock.ID == removeLockID {
			continue
		}
		switch lock.TargetType {
		case TargetSpace:
			if lock.TargetID == SpaceTargetID {
				result = append(result, lock)
			}
		case TargetNotebook:
			if _, ok := ancestors[lock.TargetID]; ok {
				result = append(result, lock)
			}
		case TargetNote:
			if noteID != "" && lock.TargetID == noteID {
				result = append(result, lock)
			}
		default:
			return nil, ErrIntegrity
		}
	}
	if additional != nil && additional.ID != removeLockID {
		result = append(result, *additional)
	}
	sortLockRecords(result)
	return result, nil
}

func (m *Manager) notebookAncestorIDs(ctx context.Context, notebookID *string) ([]string, error) {
	if notebookID == nil || *notebookID == "" {
		return []string{}, nil
	}
	rows, err := m.db.QueryContext(ctx, `
WITH RECURSIVE ancestors(id, parent_id) AS (
  SELECT id, parent_id FROM notebooks WHERE id = ?
  UNION ALL
  SELECT notebook.id, notebook.parent_id
  FROM notebooks notebook
  JOIN ancestors child ON child.parent_id = notebook.id
)
SELECT id FROM ancestors
`, *notebookID)
	if err != nil {
		return nil, fmt.Errorf("list notebook ancestors for content lock: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan content lock notebook ancestor: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content lock notebook ancestors: %w", err)
	}
	if len(ids) == 0 {
		return nil, ErrNotFound
	}
	return ids, nil
}

func (m *Manager) insertOperation(ctx context.Context, operation lockOperation) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin content lock operation tx: %w", err)
	}
	defer tx.Rollback()
	var salt, nonce, wrappedKey []byte
	var memoryKiB, iterations, parallelism any
	if operation.Lock != nil {
		salt = operation.Lock.Salt
		nonce = operation.Lock.WrapNonce
		wrappedKey = operation.Lock.WrappedKey
		memoryKiB = operation.Lock.MemoryKiB
		iterations = operation.Lock.Iterations
		parallelism = operation.Lock.Parallelism
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO content_lock_operations(
  operation_id, operation_kind, target_type, target_id, lock_id,
  kdf_salt, kdf_memory_kib, kdf_iterations, kdf_parallelism, wrap_nonce,
  wrapped_key, delete_ai_records, stage, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, operation.ID, operation.Kind, operation.Target.Type, operation.Target.ID, operation.LockID,
		salt, memoryKiB, iterations, parallelism, nonce, wrappedKey, operation.DeleteAIRecords, operation.Stage, formatTime(operation.CreatedAt)); err != nil {
		return fmt.Errorf("insert content lock operation: %w", err)
	}
	for _, noteID := range operation.NoteIDs {
		if _, err := tx.ExecContext(ctx, "INSERT INTO content_lock_operation_notes(operation_id, note_id) VALUES (?, ?)", operation.ID, noteID); err != nil {
			return fmt.Errorf("insert content lock operation note: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit content lock operation tx: %w", err)
	}
	return nil
}

func (m *Manager) listOperations(ctx context.Context) ([]lockOperation, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT operation_id, operation_kind, target_type, target_id, lock_id,
       kdf_salt, kdf_memory_kib, kdf_iterations, kdf_parallelism, wrap_nonce,
       wrapped_key, delete_ai_records, stage, created_at
FROM content_lock_operations
ORDER BY created_at ASC, operation_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list content lock operations: %w", err)
	}
	defer rows.Close()
	operations := make([]lockOperation, 0)
	for rows.Next() {
		var operation lockOperation
		var salt, nonce, wrappedKey []byte
		var memoryKiB, iterations, parallelism sql.NullInt64
		var createdAt string
		if err := rows.Scan(
			&operation.ID, &operation.Kind, &operation.Target.Type, &operation.Target.ID, &operation.LockID,
			&salt, &memoryKiB, &iterations, &parallelism, &nonce, &wrappedKey,
			&operation.DeleteAIRecords, &operation.Stage, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan content lock operation: %w", err)
		}
		created, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		operation.CreatedAt = created
		if operation.Kind == "enable" {
			if !memoryKiB.Valid || !iterations.Valid || !parallelism.Valid {
				return nil, ErrIntegrity
			}
			operation.Lock = &lockRecord{
				Lock: Lock{ID: operation.LockID, TargetType: operation.Target.Type, TargetID: operation.Target.ID, CreatedAt: created, UpdatedAt: created},
				Salt: salt, MemoryKiB: int(memoryKiB.Int64), Iterations: int(iterations.Int64), Parallelism: int(parallelism.Int64), WrapNonce: nonce, WrappedKey: wrappedKey,
			}
		}
		noteIDs, err := m.operationNoteIDs(ctx, operation.ID)
		if err != nil {
			return nil, err
		}
		operation.NoteIDs = noteIDs
		operations = append(operations, operation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content lock operations: %w", err)
	}
	return operations, nil
}

func (m *Manager) operationNoteIDs(ctx context.Context, operationID string) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT note_id FROM content_lock_operation_notes WHERE operation_id = ? ORDER BY note_id ASC", operationID)
	if err != nil {
		return nil, fmt.Errorf("list content lock operation notes: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan content lock operation note: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content lock operation notes: %w", err)
	}
	return ids, nil
}

func (m *Manager) countAIRecords(ctx context.Context, noteIDs []string) (int, error) {
	if len(noteIDs) == 0 {
		return 0, nil
	}
	placeholders, args := questionMarks(noteIDs)
	var count int
	query := `
SELECT COUNT(*) FROM (
  SELECT DISTINCT history_id AS id FROM ai_history_sources WHERE note_id IN (` + placeholders + `)
  UNION
  SELECT DISTINCT artifact_id AS id FROM ai_artifact_sources WHERE note_id IN (` + placeholders + `)
)
`
	combined := append(append([]any(nil), args...), args...)
	if err := m.db.QueryRowContext(ctx, query, combined...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count content lock AI records: %w", err)
	}
	return count, nil
}

func (m *Manager) deleteAIRecords(ctx context.Context, noteIDs []string) error {
	if len(noteIDs) == 0 {
		return nil
	}
	placeholders, args := questionMarks(noteIDs)
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete protected AI records tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
DELETE FROM ai_histories
WHERE id IN (SELECT DISTINCT history_id FROM ai_history_sources WHERE note_id IN (`+placeholders+`))
`, args...); err != nil {
		return fmt.Errorf("delete protected AI histories: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM ai_artifacts
WHERE id IN (SELECT DISTINCT artifact_id FROM ai_artifact_sources WHERE note_id IN (`+placeholders+`))
`, args...); err != nil {
		return fmt.Errorf("delete protected AI artifacts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete protected AI records tx: %w", err)
	}
	return nil
}

func (m *Manager) purgeDerivedData(ctx context.Context, noteIDs []string) error {
	if len(noteIDs) == 0 {
		return nil
	}
	placeholders, args := questionMarks(noteIDs)
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin purge protected derived data tx: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		"DELETE FROM note_search WHERE note_id IN (" + placeholders + ")",
		"DELETE FROM note_search_state WHERE note_id IN (" + placeholders + ")",
		"DELETE FROM note_links WHERE source_note_id IN (" + placeholders + ") OR target_note_id IN (" + placeholders + ")",
		"DELETE FROM note_link_state WHERE note_id IN (" + placeholders + ")",
	}
	for index, statement := range statements {
		statementArgs := args
		if index == 2 {
			statementArgs = append(append([]any(nil), args...), args...)
		}
		if _, err := tx.ExecContext(ctx, statement, statementArgs...); err != nil {
			return fmt.Errorf("purge protected derived data: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purge protected derived data tx: %w", err)
	}
	return nil
}

// purgeLocalSyncState removes local plaintext snapshots for the newly locked
// notes. A configured remote is rejected before enabling a lock, so this never
// attempts an unsafe remote deletion or history rewrite.
func (m *Manager) purgeLocalSyncState(ctx context.Context, noteIDs []string) error {
	if len(noteIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(noteIDs)*2)
	for _, noteID := range noteIDs {
		keys = append(keys, "note:"+noteID, "note-tags:"+noteID)
	}
	placeholders, args := questionMarks(keys)
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin purge protected sync state tx: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"DELETE FROM sync_outbox WHERE entity_key IN (" + placeholders + ")",
		"DELETE FROM sync_item_states WHERE entity_key IN (" + placeholders + ")",
		"DELETE FROM sync_snapshots WHERE entity_key IN (" + placeholders + ")",
		"DELETE FROM sync_conflicts WHERE entity_key IN (" + placeholders + ")",
	} {
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return fmt.Errorf("purge protected sync state: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purge protected sync state tx: %w", err)
	}
	return nil
}

func questionMarks(values []string) (string, []any) {
	placeholders := make([]string, len(values))
	args := make([]any, 0, len(values))
	for index, value := range values {
		placeholders[index] = "?"
		args = append(args, value)
	}
	return strings.Join(placeholders, ","), args
}

// time is used by database/sql scanners on all supported platforms; keep this
// compile-time reference near the repository helpers to avoid accidental
// removal when the operation model evolves.
var _ = time.RFC3339Nano
