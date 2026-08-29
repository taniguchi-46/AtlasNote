package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"atlasnote/internal/note"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnqueueChanges(ctx context.Context, changes []note.SyncChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync outbox tx: %w", err)
	}
	defer tx.Rollback()
	if err := r.RecordSyncChanges(ctx, tx, changes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync outbox tx: %w", err)
	}
	return nil
}

func (r *Repository) RecordSyncChanges(ctx context.Context, tx *sql.Tx, changes []note.SyncChange) error {
	if len(changes) == 0 {
		return nil
	}
	changeSetID := changes[0].ChangeSetID
	if changeSetID == "" {
		var err error
		changeSetID, err = randomHex(16)
		if err != nil {
			return fmt.Errorf("generate sync change set id: %w", err)
		}
	}

	baseManifestHash, baseHeadETag, err := syncHeadBase(ctx, tx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, change := range changes {
		if change.EntityKey == "" || change.EntityType == "" {
			return errors.New("sync change entity key and type are required")
		}
		payload := change.ObjectJSON
		if !change.Deleted && len(payload) == 0 {
			return fmt.Errorf("sync change %s has no payload", change.EntityKey)
		}
		objectJSON, err := objectDocument(change.EntityType, change.EntityKey, payload, change.Deleted)
		if err != nil {
			return err
		}
		objectHash := hashBytes(objectJSON)
		baseObjectHash, err := syncItemBase(ctx, tx, change.EntityKey)
		if err != nil {
			return err
		}
		previousSnapshot, err := syncItemSnapshot(ctx, tx, change.EntityKey)
		if err != nil {
			return err
		}
		if baseObjectHash != "" && previousSnapshot != "" {
			if err := saveSnapshotTx(ctx, tx, change.EntityKey, change.EntityType, baseObjectHash, previousSnapshot); err != nil {
				return err
			}
		}
		if change.ChangeSetID != "" {
			changeSetID = change.ChangeSetID
		}

		if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_item_states(
	entity_key, entity_type, local_object_hash, base_object_hash,
	remote_object_hash, body_hash, metadata_hash, resolution_state,
	snapshot_json, updated_at
)
VALUES(?, ?, ?, ?, '', '', ?, 'pending', ?, ?)
ON CONFLICT(entity_key) DO UPDATE SET
	entity_type = excluded.entity_type,
	local_object_hash = excluded.local_object_hash,
	metadata_hash = excluded.metadata_hash,
	resolution_state = 'pending',
	snapshot_json = excluded.snapshot_json,
	updated_at = excluded.updated_at
`, change.EntityKey, change.EntityType, objectHash, baseObjectHash, objectHash, string(objectJSON), formatTimestamp(now)); err != nil {
			return fmt.Errorf("upsert sync item state: %w", err)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM sync_outbox WHERE entity_key = ?", change.EntityKey); err != nil {
			return fmt.Errorf("coalesce sync outbox: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_outbox(
	change_set_id, entity_key, entity_type, object_hash,
	base_manifest_hash, base_head_etag, object_json, deleted,
	attempt_count, next_retry_at, failed_class, created_at
)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, 0, ?, '', ?)
ON CONFLICT(change_set_id, entity_key) DO UPDATE SET
	entity_type = excluded.entity_type,
	object_hash = excluded.object_hash,
	base_manifest_hash = excluded.base_manifest_hash,
	base_head_etag = excluded.base_head_etag,
	object_json = excluded.object_json,
	deleted = excluded.deleted,
	attempt_count = 0,
	next_retry_at = excluded.next_retry_at,
	failed_class = '',
	created_at = excluded.created_at
`, changeSetID, change.EntityKey, change.EntityType, objectHash, baseManifestHash, baseHeadETag, string(objectJSON), change.Deleted, formatTimestamp(now), formatTimestamp(now)); err != nil {
			return fmt.Errorf("insert sync outbox: %w", err)
		}
	}

	return nil
}

// DiscardUnsyncedChanges removes a local create that was rolled back before
// its Markdown file became canonical. It refuses to discard an entity that
// already has a remote/base version, because doing so would erase history that
// belongs to an earlier synchronization state.
func (r *Repository) DiscardUnsyncedChanges(ctx context.Context, tx *sql.Tx, entityKeys []string) error {
	for _, entityKey := range entityKeys {
		result, err := tx.ExecContext(ctx, `
DELETE FROM sync_item_states
WHERE entity_key = ? AND base_object_hash = '' AND remote_object_hash = ''
`, entityKey)
		if err != nil {
			return fmt.Errorf("discard rolled back sync item state: %w", err)
		}
		removed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read rolled back sync item result: %w", err)
		}
		if removed != 1 {
			return errors.New("rolled back sync item has existing remote history")
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sync_outbox WHERE entity_key = ?", entityKey); err != nil {
			return fmt.Errorf("discard rolled back sync outbox item: %w", err)
		}
	}
	return nil
}

func syncHeadBase(ctx context.Context, tx *sql.Tx) (string, string, error) {
	var manifestHash, etag string
	err := tx.QueryRowContext(ctx, `
SELECT head_manifest_hash, head_etag
FROM sync_connections
WHERE id = 1
`).Scan(&manifestHash, &etag)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read sync head base: %w", err)
	}
	return manifestHash, etag, nil
}

func syncItemBase(ctx context.Context, tx *sql.Tx, entityKey string) (string, error) {
	var value string
	err := tx.QueryRowContext(ctx, `
SELECT base_object_hash
FROM sync_item_states
WHERE entity_key = ?
`, entityKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read sync item base: %w", err)
	}
	return value, nil
}

func syncItemSnapshot(ctx context.Context, tx *sql.Tx, entityKey string) (string, error) {
	var value string
	err := tx.QueryRowContext(ctx, `
SELECT snapshot_json
FROM sync_item_states
WHERE entity_key = ?
`, entityKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read sync item snapshot: %w", err)
	}
	return value, nil
}

func (r *Repository) GetConnection(ctx context.Context) (*Connection, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT endpoint, remote_root, username, vault_id, head_manifest_hash, head_etag,
       last_sync_at, status, auto_sync, sync_interval_seconds, allow_insecure_http,
       custom_tls_certificates, ignore_tls_errors, proxy_enabled, proxy_url,
       proxy_timeout_seconds, fail_safe, credential_ref
FROM sync_connections
WHERE id = 1
`)
	connection, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync connection: %w", err)
	}
	return &connection, nil
}

func (r *Repository) SaveConnection(ctx context.Context, connection Connection) error {
	return saveConnection(ctx, r.db, connection)
}

func saveConnection(ctx context.Context, execer syncExecer, connection Connection) error {
	now := time.Now().UTC()
	if connection.ProxyTimeoutSeconds == 0 {
		connection.ProxyTimeoutSeconds = DefaultProxyTimeoutSeconds
	}
	lastSyncAt := any(nil)
	if connection.HasLastSync {
		lastSyncAt = formatTimestamp(connection.LastSyncAt)
	}
	_, err := execer.ExecContext(ctx, `
INSERT INTO sync_connections(
	id, endpoint, remote_root, username, vault_id, head_manifest_hash, head_etag,
	last_sync_at, status, auto_sync, sync_interval_seconds, allow_insecure_http,
	custom_tls_certificates, ignore_tls_errors, proxy_enabled, proxy_url,
	proxy_timeout_seconds, fail_safe, credential_ref, created_at, updated_at
)
VALUES(1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
          ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	endpoint = excluded.endpoint,
	remote_root = excluded.remote_root,
	username = excluded.username,
	vault_id = excluded.vault_id,
	head_manifest_hash = excluded.head_manifest_hash,
	head_etag = excluded.head_etag,
	last_sync_at = excluded.last_sync_at,
	status = excluded.status,
	auto_sync = excluded.auto_sync,
	sync_interval_seconds = excluded.sync_interval_seconds,
	allow_insecure_http = excluded.allow_insecure_http,
	custom_tls_certificates = excluded.custom_tls_certificates,
	ignore_tls_errors = excluded.ignore_tls_errors,
	proxy_enabled = excluded.proxy_enabled,
	proxy_url = excluded.proxy_url,
	proxy_timeout_seconds = excluded.proxy_timeout_seconds,
	fail_safe = excluded.fail_safe,
	credential_ref = excluded.credential_ref,
	updated_at = excluded.updated_at
	`, connection.Endpoint, connection.RemoteRoot, connection.Username, connection.VaultID, connection.HeadManifestHash,
		connection.HeadETag, lastSyncAt, connection.Status, connection.AutoSync, connection.SyncIntervalSeconds,
		connection.AllowInsecureHTTP, connection.CustomTLSCertificates, connection.IgnoreTLSErrors,
		connection.ProxyEnabled, connection.ProxyURL, connection.ProxyTimeoutSeconds, connection.FailSafe, connection.CredentialRef,
		formatTimestamp(now), formatTimestamp(now))
	if err != nil {
		return fmt.Errorf("save sync connection: %w", err)
	}
	return nil
}

// ConfigureConnection commits the connection identity, any target-specific
// tracking reset, and the initial local snapshot as one SQLite transaction.
// A crash cannot leave a new target saved without its corresponding outbox.
func (r *Repository) ConfigureConnection(ctx context.Context, connection Connection, resetTracking bool, initialChanges []note.SyncChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync configuration tx: %w", err)
	}
	defer tx.Rollback()
	if err := saveConnection(ctx, tx, connection); err != nil {
		return err
	}
	if resetTracking {
		for _, table := range []string{"sync_outbox", "sync_conflicts", "sync_snapshots", "sync_item_states"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return fmt.Errorf("reset sync tracking during configuration: %w", err)
			}
		}
	}
	if err := r.RecordSyncChanges(ctx, tx, initialChanges); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync configuration tx: %w", err)
	}
	return nil
}

func (r *Repository) UpdateConnectionStatus(ctx context.Context, status Status, message string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sync_connections
SET status = ?, updated_at = ?
WHERE id = 1
`, status, formatTimestamp(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("update sync status: %w", err)
	}
	return nil
}

func (r *Repository) UpdateConnectionHead(ctx context.Context, manifestHash string, etag string, status Status) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE sync_connections
SET head_manifest_hash = ?, head_etag = ?, last_sync_at = ?, status = ?, updated_at = ?
WHERE id = 1
`, manifestHash, etag, formatTimestamp(now), status, formatTimestamp(now))
	if err != nil {
		return fmt.Errorf("update sync head: %w", err)
	}
	return nil
}

func (r *Repository) DeleteConnection(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sync_connections WHERE id = 1"); err != nil {
		return fmt.Errorf("delete sync connection: %w", err)
	}
	return nil
}

// ResetSyncTracking clears only remote-derived synchronization metadata when
// the user explicitly chooses a different target. Canonical local notes and
// Markdown files are never touched.
func (r *Repository) ResetSyncTracking(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync tracking reset: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{"sync_outbox", "sync_conflicts", "sync_snapshots", "sync_item_states"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("reset sync tracking: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync tracking reset: %w", err)
	}
	return nil
}

func (r *Repository) CommitRecoveryState(ctx context.Context, manifestHash string, headETag string, items []recoverySyncedItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync recovery state commit: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{"sync_outbox", "sync_conflicts", "sync_snapshots", "sync_item_states"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("reset sync recovery state: %w", err)
		}
	}
	now := formatTimestamp(time.Now().UTC())
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_item_states(
	entity_key, entity_type, local_object_hash, base_object_hash,
	remote_object_hash, body_hash, metadata_hash, resolution_state,
	snapshot_json, updated_at
)
VALUES(?, ?, ?, ?, ?, '', '', 'synced', ?, ?)
`, item.EntityKey, item.EntityType, item.ObjectHash, item.ObjectHash, item.ObjectHash, item.ObjectJSON, now); err != nil {
			return fmt.Errorf("insert sync recovery item state: %w", err)
		}
		if err := saveSnapshotTx(ctx, tx, item.EntityKey, item.EntityType, item.ObjectHash, item.ObjectJSON); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE sync_connections
SET head_manifest_hash = ?, head_etag = ?, last_sync_at = ?, status = ?, updated_at = ?
WHERE id = 1
`, manifestHash, headETag, now, StatusSynced, now); err != nil {
		return fmt.Errorf("update sync connection after recovery: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync recovery state: %w", err)
	}
	return nil
}

func (r *Repository) ListOutbox(ctx context.Context, limit int) ([]OutboxItem, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT sequence, change_set_id, entity_key, entity_type, object_hash,
       base_manifest_hash, base_head_etag, object_json, deleted,
       attempt_count, next_retry_at, failed_class, created_at
FROM sync_outbox
WHERE next_retry_at <= ?
ORDER BY sequence
LIMIT ?
`, formatTimestamp(time.Now().UTC()), limit)
	if err != nil {
		return nil, fmt.Errorf("list sync outbox: %w", err)
	}
	defer rows.Close()

	items := make([]OutboxItem, 0)
	for rows.Next() {
		item, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync outbox: %w", err)
	}
	return items, nil
}

func (r *Repository) CountOutbox(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_outbox").Scan(&count); err != nil {
		return 0, fmt.Errorf("count sync outbox: %w", err)
	}
	return count, nil
}

func (r *Repository) DeleteOutbox(ctx context.Context, sequence int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sync_outbox WHERE sequence = ?", sequence); err != nil {
		return fmt.Errorf("delete sync outbox item: %w", err)
	}
	return nil
}

func (r *Repository) DeleteOutboxEntity(ctx context.Context, entityKey string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sync_outbox WHERE entity_key = ?", entityKey); err != nil {
		return fmt.Errorf("delete sync outbox entity: %w", err)
	}
	return nil
}

func (r *Repository) RequeueConflictLocal(ctx context.Context, conflict Conflict) error {
	connection, err := r.GetConnection(ctx)
	if err != nil {
		return err
	}
	baseManifestHash, baseETag := "", ""
	if connection != nil {
		baseManifestHash, baseETag = connection.HeadManifestHash, connection.HeadETag
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin local conflict requeue tx: %w", err)
	}
	defer tx.Rollback()

	var objectHash, objectJSON, entityType string
	var deleted bool
	err = tx.QueryRowContext(ctx, `
SELECT object_hash, object_json, entity_type, deleted
FROM sync_outbox
WHERE entity_key = ?
ORDER BY sequence DESC
LIMIT 1
`, conflict.EntityKey).Scan(&objectHash, &objectJSON, &entityType, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		objectJSON = conflict.LocalSnapshot
		objectHash = hashBytes([]byte(objectJSON))
		object, decodeErr := decodeObject([]byte(objectJSON))
		if decodeErr != nil || object.EntityKey != conflict.EntityKey || object.EntityType != conflict.EntityType {
			return ErrInvalidRemoteFormat
		}
		if conflict.LocalObjectHash != "" && conflict.LocalObjectHash != objectHash {
			return ErrInvalidRemoteFormat
		}
		entityType = object.EntityType
		deleted = object.Deleted
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_outbox(
	change_set_id, entity_key, entity_type, object_hash,
	base_manifest_hash, base_head_etag, object_json, deleted,
	attempt_count, next_retry_at, failed_class, created_at
)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, 0, ?, '', ?)
`, conflict.ID, conflict.EntityKey, entityType, objectHash, baseManifestHash, baseETag,
			objectJSON, deleted, formatTimestamp(time.Now().UTC()), formatTimestamp(time.Now().UTC())); err != nil {
			return fmt.Errorf("requeue local sync conflict: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("read local conflict outbox: %w", err)
	} else {
		object, decodeErr := decodeObject([]byte(objectJSON))
		if decodeErr != nil || hashBytes([]byte(objectJSON)) != objectHash ||
			object.EntityKey != conflict.EntityKey || object.EntityType != conflict.EntityType || entityType != conflict.EntityType || object.Deleted != deleted {
			return ErrInvalidRemoteFormat
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE sync_outbox
SET base_manifest_hash = ?, base_head_etag = ?, attempt_count = 0,
	next_retry_at = ?, failed_class = ''
WHERE entity_key = ?
`, baseManifestHash, baseETag, formatTimestamp(time.Now().UTC()), conflict.EntityKey); err != nil {
			return fmt.Errorf("requeue latest local sync conflict: %w", err)
		}
	}

	remoteObject, err := decodeObject([]byte(conflict.RemoteSnapshot))
	if err != nil || conflict.RemoteObjectHash == "" || hashBytes([]byte(conflict.RemoteSnapshot)) != conflict.RemoteObjectHash ||
		remoteObject.EntityKey != conflict.EntityKey || remoteObject.EntityType != conflict.EntityType {
		return ErrInvalidRemoteFormat
	}
	if err := saveSnapshotTx(ctx, tx, conflict.EntityKey, conflict.EntityType, conflict.RemoteObjectHash, conflict.RemoteSnapshot); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE sync_item_states
SET base_object_hash = ?, remote_object_hash = ?, resolution_state = 'pending', updated_at = ?
WHERE entity_key = ? AND local_object_hash = ?
`, conflict.RemoteObjectHash, conflict.RemoteObjectHash, formatTimestamp(time.Now().UTC()), conflict.EntityKey, objectHash)
	if err != nil {
		return fmt.Errorf("rebase local sync conflict: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read local sync conflict rebase result: %w", err)
	}
	if updated != 1 {
		return ErrInvalidRemoteFormat
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit local conflict requeue tx: %w", err)
	}
	return nil
}

func (r *Repository) MarkOutboxRetry(ctx context.Context, sequence int64, attempt int, nextRetry time.Time, failedClass FailedClass) error {
	if _, err := r.db.ExecContext(ctx, `
UPDATE sync_outbox
SET attempt_count = ?, next_retry_at = ?, failed_class = ?
WHERE sequence = ?
`, attempt, formatTimestamp(nextRetry), failedClass, sequence); err != nil {
		return fmt.Errorf("mark sync outbox retry: %w", err)
	}
	return nil
}

func (r *Repository) ResetRetryableOutbox(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sync_outbox
SET attempt_count = 0, next_retry_at = ?, failed_class = ''
WHERE failed_class <> '' AND failed_class <> ?
`, formatTimestamp(time.Now().UTC()), FailedClassConflict)
	if err != nil {
		return fmt.Errorf("reset sync outbox retry state: %w", err)
	}
	return nil
}

func (r *Repository) MarkOutboxFailed(ctx context.Context, sequence int64, failedClass FailedClass) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sync_outbox
SET attempt_count = 3, next_retry_at = ?, failed_class = ?
WHERE sequence = ?
`, formatTimestamp(time.Now().UTC().Add(365*24*time.Hour)), failedClass, sequence)
	if err != nil {
		return fmt.Errorf("mark sync outbox failed: %w", err)
	}
	return nil
}

func (r *Repository) GetItemState(ctx context.Context, entityKey string) (*ItemState, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT entity_key, entity_type, local_object_hash, base_object_hash,
       remote_object_hash, body_hash, metadata_hash, resolution_state,
       snapshot_json, updated_at
FROM sync_item_states
WHERE entity_key = ?
`, entityKey)
	state, err := scanItemState(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync item state: %w", err)
	}
	return &state, nil
}

func (r *Repository) ListItemStates(ctx context.Context) ([]ItemState, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT entity_key, entity_type, local_object_hash, base_object_hash,
       remote_object_hash, body_hash, metadata_hash, resolution_state,
       snapshot_json, updated_at
FROM sync_item_states
ORDER BY entity_key
`)
	if err != nil {
		return nil, fmt.Errorf("list sync item states: %w", err)
	}
	defer rows.Close()
	states := make([]ItemState, 0)
	for rows.Next() {
		state, err := scanItemState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync item states: %w", err)
	}
	return states, nil
}

func (r *Repository) MarkItemSynced(ctx context.Context, entityKey string, entityType string, objectHash string, snapshotJSON string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sync_item_states
SET base_object_hash = ?, remote_object_hash = ?, local_object_hash = ?,
    resolution_state = 'synced', snapshot_json = ?, updated_at = ?
WHERE entity_key = ?
`, objectHash, objectHash, objectHash, snapshotJSON, formatTimestamp(time.Now().UTC()), entityKey)
	if err != nil {
		return fmt.Errorf("mark sync item synced: %w", err)
	}
	if err := saveSnapshot(ctx, r.db, entityKey, entityType, objectHash, snapshotJSON); err != nil {
		return err
	}
	return nil
}

func (r *Repository) MarkItemRemote(ctx context.Context, entityKey string, entityType string, objectHash string, snapshotJSON string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO sync_item_states(
	entity_key, entity_type, local_object_hash, base_object_hash,
	remote_object_hash, body_hash, metadata_hash, resolution_state,
	snapshot_json, updated_at
)
VALUES(?, ?, ?, ?, ?, '', '', 'synced', ?, ?)
ON CONFLICT(entity_key) DO UPDATE SET
	entity_type = excluded.entity_type,
	local_object_hash = excluded.local_object_hash,
	base_object_hash = excluded.base_object_hash,
	remote_object_hash = excluded.remote_object_hash,
	resolution_state = 'synced',
	snapshot_json = excluded.snapshot_json,
	updated_at = excluded.updated_at
`, entityKey, entityType, objectHash, objectHash, objectHash, snapshotJSON, formatTimestamp(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("mark remote sync item: %w", err)
	}
	if err := saveSnapshot(ctx, r.db, entityKey, entityType, objectHash, snapshotJSON); err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetSnapshot(ctx context.Context, entityKey string, objectHash string) (string, error) {
	if entityKey == "" || objectHash == "" {
		return "", nil
	}
	var snapshot string
	err := r.db.QueryRowContext(ctx, `
SELECT object_json
FROM sync_snapshots
WHERE snapshot_id = ?
`, snapshotID(entityKey, objectHash)).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get sync snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *Repository) CreateConflict(ctx context.Context, conflict Conflict) error {
	if conflict.ID == "" {
		var err error
		conflict.ID, err = randomHex(16)
		if err != nil {
			return fmt.Errorf("generate sync conflict id: %w", err)
		}
	}
	if conflict.CreatedAt.IsZero() {
		conflict.CreatedAt = time.Now().UTC()
	}
	if conflict.ResolutionStatus == "" {
		conflict.ResolutionStatus = "open"
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
SELECT EXISTS(
	SELECT 1 FROM sync_conflicts
	WHERE entity_key = ? AND resolution_status = 'open'
)
`, conflict.EntityKey).Scan(&exists); err != nil {
		return fmt.Errorf("check existing sync conflict: %w", err)
	}
	if exists {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO sync_conflicts(
	conflict_id, entity_key, entity_type, local_object_hash,
	base_object_hash, remote_object_hash, local_snapshot_json,
	base_snapshot_json, remote_snapshot_json, conflict_type,
	resolution_status, created_at, resolved_at
)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, conflict.ID, conflict.EntityKey, conflict.EntityType, conflict.LocalObjectHash,
		conflict.BaseObjectHash, conflict.RemoteObjectHash, conflict.LocalSnapshot,
		conflict.BaseSnapshot, conflict.RemoteSnapshot, conflict.ConflictType,
		conflict.ResolutionStatus, formatTimestamp(conflict.CreatedAt), nullableTime(conflict.ResolvedAt, conflict.HasResolvedAt))
	if err != nil {
		return fmt.Errorf("create sync conflict: %w", err)
	}
	return nil
}

func (r *Repository) ListConflicts(ctx context.Context) ([]Conflict, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT conflict_id, entity_key, entity_type, local_object_hash,
       base_object_hash, remote_object_hash, local_snapshot_json,
       base_snapshot_json, remote_snapshot_json, conflict_type,
       resolution_status, created_at, resolved_at
FROM sync_conflicts
WHERE resolution_status = 'open'
ORDER BY created_at, conflict_id
`)
	if err != nil {
		return nil, fmt.Errorf("list sync conflicts: %w", err)
	}
	defer rows.Close()
	conflicts := make([]Conflict, 0)
	for rows.Next() {
		conflict, err := scanConflict(rows)
		if err != nil {
			return nil, err
		}
		conflicts = append(conflicts, conflict)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync conflicts: %w", err)
	}
	return conflicts, nil
}

func (r *Repository) CountConflicts(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sync_conflicts
WHERE resolution_status = 'open'
`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sync conflicts: %w", err)
	}
	return count, nil
}

func (r *Repository) GetConflict(ctx context.Context, conflictID string) (*Conflict, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT conflict_id, entity_key, entity_type, local_object_hash,
       base_object_hash, remote_object_hash, local_snapshot_json,
       base_snapshot_json, remote_snapshot_json, conflict_type,
       resolution_status, created_at, resolved_at
FROM sync_conflicts
WHERE conflict_id = ?
`, conflictID)
	conflict, err := scanConflict(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sync conflict: %w", err)
	}
	return &conflict, nil
}

func (r *Repository) ResolveConflict(ctx context.Context, conflictID string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE sync_conflicts
SET resolution_status = 'resolved', resolved_at = ?
WHERE conflict_id = ? AND resolution_status = 'open'
`, formatTimestamp(time.Now().UTC()), conflictID)
	if err != nil {
		return fmt.Errorf("resolve sync conflict: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read sync conflict resolution result: %w", err)
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanConnection(scanner interface{ Scan(...any) error }) (Connection, error) {
	var connection Connection
	var lastSyncAt sql.NullString
	var autoSync bool
	if err := scanner.Scan(&connection.Endpoint, &connection.RemoteRoot, &connection.Username, &connection.VaultID,
		&connection.HeadManifestHash, &connection.HeadETag, &lastSyncAt, &connection.Status,
		&autoSync, &connection.SyncIntervalSeconds, &connection.AllowInsecureHTTP,
		&connection.CustomTLSCertificates, &connection.IgnoreTLSErrors, &connection.ProxyEnabled,
		&connection.ProxyURL, &connection.ProxyTimeoutSeconds, &connection.FailSafe,
		&connection.CredentialRef); err != nil {
		return Connection{}, err
	}
	connection.AutoSync = autoSync
	if lastSyncAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastSyncAt.String)
		if err != nil {
			return Connection{}, fmt.Errorf("parse sync last sync time: %w", err)
		}
		connection.LastSyncAt = parsed
		connection.HasLastSync = true
	}
	return connection, nil
}

func scanOutbox(scanner interface{ Scan(...any) error }) (OutboxItem, error) {
	var item OutboxItem
	var nextRetryAt, createdAt string
	if err := scanner.Scan(&item.Sequence, &item.ChangeSetID, &item.EntityKey, &item.EntityType,
		&item.ObjectHash, &item.BaseManifestHash, &item.BaseHeadETag, &item.ObjectJSON,
		&item.Deleted, &item.AttemptCount, &nextRetryAt, &item.FailedClass, &createdAt); err != nil {
		return OutboxItem{}, fmt.Errorf("scan sync outbox: %w", err)
	}
	var err error
	item.NextRetryAt, err = time.Parse(time.RFC3339Nano, nextRetryAt)
	if err != nil {
		return OutboxItem{}, fmt.Errorf("parse sync outbox retry time: %w", err)
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return OutboxItem{}, fmt.Errorf("parse sync outbox created time: %w", err)
	}
	return item, nil
}

func scanItemState(scanner interface{ Scan(...any) error }) (ItemState, error) {
	var state ItemState
	var updatedAt string
	if err := scanner.Scan(&state.EntityKey, &state.EntityType, &state.LocalObjectHash,
		&state.BaseObjectHash, &state.RemoteObjectHash, &state.BodyHash, &state.MetadataHash,
		&state.ResolutionState, &state.SnapshotJSON, &updatedAt); err != nil {
		return ItemState{}, fmt.Errorf("scan sync item state: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return ItemState{}, fmt.Errorf("parse sync item updated time: %w", err)
	}
	state.UpdatedAt = parsed
	return state, nil
}

func scanConflict(scanner interface{ Scan(...any) error }) (Conflict, error) {
	var conflict Conflict
	var createdAt string
	var resolvedAt sql.NullString
	if err := scanner.Scan(&conflict.ID, &conflict.EntityKey, &conflict.EntityType,
		&conflict.LocalObjectHash, &conflict.BaseObjectHash, &conflict.RemoteObjectHash,
		&conflict.LocalSnapshot, &conflict.BaseSnapshot, &conflict.RemoteSnapshot,
		&conflict.ConflictType, &conflict.ResolutionStatus, &createdAt, &resolvedAt); err != nil {
		return Conflict{}, fmt.Errorf("scan sync conflict: %w", err)
	}
	var err error
	conflict.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Conflict{}, fmt.Errorf("parse sync conflict created time: %w", err)
	}
	if resolvedAt.Valid {
		conflict.ResolvedAt, err = time.Parse(time.RFC3339Nano, resolvedAt.String)
		if err != nil {
			return Conflict{}, fmt.Errorf("parse sync conflict resolved time: %w", err)
		}
		conflict.HasResolvedAt = true
	}
	return conflict, nil
}

func nullableTime(value time.Time, valid bool) any {
	if !valid || value.IsZero() {
		return nil
	}
	return formatTimestamp(value)
}

type syncExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func snapshotID(entityKey string, objectHash string) string {
	return entityKey + "@" + objectHash
}

func saveSnapshot(ctx context.Context, execer syncExecer, entityKey string, entityType string, objectHash string, objectJSON string) error {
	if entityKey == "" || objectHash == "" || objectJSON == "" {
		return nil
	}
	if _, err := execer.ExecContext(ctx, `
INSERT OR IGNORE INTO sync_snapshots(
	snapshot_id, entity_key, entity_type, object_hash, object_json, created_at
)
VALUES(?, ?, ?, ?, ?, ?)
`, snapshotID(entityKey, objectHash), entityKey, entityType, objectHash, objectJSON, formatTimestamp(time.Now().UTC())); err != nil {
		return fmt.Errorf("save sync snapshot: %w", err)
	}
	return nil
}

func saveSnapshotTx(ctx context.Context, tx *sql.Tx, entityKey string, entityType string, objectHash string, objectJSON string) error {
	return saveSnapshot(ctx, tx, entityKey, entityType, objectHash, objectJSON)
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
