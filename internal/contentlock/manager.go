package contentlock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/storage"
)

type lockRecord struct {
	Lock
	Salt        []byte
	MemoryKiB   int
	Iterations  int
	Parallelism int
	WrapNonce   []byte
	WrappedKey  []byte
}

type lockOperation struct {
	ID              string
	Kind            string
	Target          Target
	LockID          string
	Lock            *lockRecord
	DeleteAIRecords bool
	Stage           string
	CreatedAt       time.Time
	NoteIDs         []string
}

type pendingWrite struct {
	setNotebookID  bool
	notebookID     *string
	additionalLock *lockRecord
	removeLockID   string
}

type unlockFailure struct {
	attempts   int
	retryAfter time.Time
}

// Manager stores encrypted lock metadata in SQLite and keeps decrypted lock
// keys only in process memory. It also implements storage.ContentProtector.
type Manager struct {
	db    *sql.DB
	store *storage.MarkdownStore

	// aiOperationMu is separate from operationMu so an AI request can hold its
	// access gate while it asks the note service for context. Lock conversions
	// take aiOperationMu first, then operationMu; this order prevents a writer
	// from waiting on an AI request that is itself blocked behind the writer.
	aiOperationMu sync.RWMutex
	operationMu   sync.RWMutex
	mu            sync.RWMutex
	sessionKeys   map[string][]byte
	pending       map[string]pendingWrite
	failures      map[string]unlockFailure
}

func NewManager(db *sql.DB, store *storage.MarkdownStore) *Manager {
	manager := &Manager{
		db:          db,
		store:       store,
		sessionKeys: make(map[string][]byte),
		pending:     make(map[string]pendingWrite),
		failures:    make(map[string]unlockFailure),
	}
	if store != nil {
		store.SetContentProtector(manager)
	}
	return manager
}

// Close removes all in-memory lock keys. The underlying stored ciphertext is
// unchanged and can only be read after a later successful unlock.
func (m *Manager) Close() {
	m.mu.Lock()
	for id, key := range m.sessionKeys {
		zeroBytes(key)
		delete(m.sessionKeys, id)
	}
	m.pending = make(map[string]pendingWrite)
	m.failures = make(map[string]unlockFailure)
	m.mu.Unlock()
}

// BeginContentAccess serializes normal note service requests with a lock
// conversion. A conversion takes the writer side of this gate, so no read or
// mutation can observe mismatched file and metadata state.
func (m *Manager) BeginContentAccess(context.Context) func() {
	m.operationMu.RLock()
	return m.operationMu.RUnlock
}

// BeginAIContentAccess keeps a complete AI request (including the provider
// call) on one side of a lock conversion. It intentionally uses a different
// gate from normal note reads because those reads occur inside an AI request.
func (m *Manager) BeginAIContentAccess(context.Context) func() {
	m.aiOperationMu.RLock()
	return m.aiOperationMu.RUnlock
}

// beginLockOperation establishes the only writer ordering for a content-lock
// conversion. AI access is stopped before normal note access so neither side
// can deadlock while collecting a note snapshot.
func (m *Manager) beginLockOperation() func() {
	m.aiOperationMu.Lock()
	m.operationMu.Lock()
	return func() {
		m.operationMu.Unlock()
		m.aiOperationMu.Unlock()
	}
}

func (m *Manager) List(ctx context.Context) ([]Lock, error) {
	locks, err := m.listLockRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Lock, 0, len(locks))
	for _, record := range locks {
		item := record.Lock
		item.TargetName = m.targetName(ctx, Target{Type: item.TargetType, ID: item.TargetID})
		item.Unlocked = m.hasSessionKey(item.ID)
		result = append(result, item)
	}
	return result, nil
}

// ListRequiredLocks returns only the configured locks that currently prevent
// access to target. It uses the same scope resolution as normal note and
// notebook access so callers never need to recreate inheritance rules.
func (m *Manager) ListRequiredLocks(ctx context.Context, target Target) ([]Lock, error) {
	target, err := normalizeTarget(target)
	if err != nil {
		return nil, err
	}

	var records []lockRecord
	switch target.Type {
	case TargetNote:
		records, err = m.locksForNote(ctx, target.ID, m.pendingFor(target.ID))
	case TargetNotebook:
		records, err = m.locksForNotebook(ctx, target.ID)
	case TargetSpace:
		var record lockRecord
		var found bool
		record, found, err = m.getLockByTarget(ctx, target)
		if found {
			records = []lockRecord{record}
		}
	}
	if err != nil {
		return nil, err
	}

	required := make([]Lock, 0, len(records))
	for _, record := range records {
		if m.hasSessionKey(record.ID) {
			continue
		}
		lock := record.Lock
		lock.TargetName = m.targetName(ctx, Target{Type: lock.TargetType, ID: lock.TargetID})
		lock.Unlocked = false
		required = append(required, lock)
	}
	return required, nil
}

func (m *Manager) GetTargetStatus(ctx context.Context, target Target) (TargetStatus, error) {
	target, err := normalizeTarget(target)
	if err != nil {
		return TargetStatus{}, err
	}
	if target.Type == TargetNote {
		return m.NoteStatus(ctx, target.ID)
	}
	if target.Type == TargetNotebook {
		return m.NotebookStatus(ctx, target.ID)
	}
	lock, found, err := m.getLockByTarget(ctx, target)
	if err != nil {
		return TargetStatus{}, err
	}
	if !found {
		return TargetStatus{}, nil
	}
	return TargetStatus{Protected: true, Locked: !m.hasSessionKey(lock.ID), ExplicitLock: true, Source: TargetSpace}, nil
}

func (m *Manager) NoteStatus(ctx context.Context, noteID string) (TargetStatus, error) {
	if !validEntityID(noteID) {
		return TargetStatus{}, ErrValidation
	}
	locks, err := m.locksForNote(ctx, noteID, m.pendingFor(noteID))
	if err != nil {
		return TargetStatus{}, err
	}
	status := statusForLocks(locks, m.hasSessionKey)
	explicit, found, err := m.getLockByTarget(ctx, Target{Type: TargetNote, ID: noteID})
	if err != nil {
		return TargetStatus{}, err
	}
	status.ExplicitLock = found
	if found && status.Source == "" {
		status.Source = explicit.TargetType
	}
	return status, nil
}

func (m *Manager) NoteLockStatus(ctx context.Context, noteID string) (bool, bool, string, error) {
	status, err := m.NoteStatus(ctx, noteID)
	return status.Protected, status.Locked, status.Source, err
}

func (m *Manager) NotebookStatus(ctx context.Context, notebookID string) (TargetStatus, error) {
	if !validEntityID(notebookID) {
		return TargetStatus{}, ErrValidation
	}
	locks, err := m.locksForNotebook(ctx, notebookID)
	if err != nil {
		return TargetStatus{}, err
	}
	status := statusForLocks(locks, m.hasSessionKey)
	_, found, err := m.getLockByTarget(ctx, Target{Type: TargetNotebook, ID: notebookID})
	if err != nil {
		return TargetStatus{}, err
	}
	status.ExplicitLock = found
	return status, nil
}

func (m *Manager) NotebookLockStatus(ctx context.Context, notebookID string) (bool, bool, string, error) {
	status, err := m.NotebookStatus(ctx, notebookID)
	return status.Protected, status.Locked, status.Source, err
}

func statusForLocks(locks []lockRecord, hasKey func(string) bool) TargetStatus {
	if len(locks) == 0 {
		return TargetStatus{}
	}
	status := TargetStatus{Protected: true}
	for _, lock := range locks {
		if !hasKey(lock.ID) {
			status.Locked = true
		}
	}
	for _, targetType := range []string{TargetNote, TargetNotebook, TargetSpace} {
		for _, lock := range locks {
			if lock.TargetType == targetType {
				status.Source = targetType
				return status
			}
		}
	}
	return status
}

func (m *Manager) AssertNoteAccess(ctx context.Context, noteID string) error {
	status, err := m.NoteStatus(ctx, noteID)
	if err != nil {
		return err
	}
	if status.Locked {
		return ErrLocked
	}
	return nil
}

func (m *Manager) AssertNotebookAccess(ctx context.Context, notebookID string) error {
	status, err := m.NotebookStatus(ctx, notebookID)
	if err != nil {
		return err
	}
	if status.Locked {
		return ErrLocked
	}
	return nil
}

func (m *Manager) IsNoteProtected(ctx context.Context, noteID string) (bool, error) {
	status, err := m.NoteStatus(ctx, noteID)
	return status.Protected, err
}

// HasContentLocks reports whether this storage space contains any explicit
// content-lock configuration. It is used to reject notebook hierarchy moves:
// moving a subtree can change the key set for every descendant note, so it
// must not proceed without a dedicated multi-file re-encryption operation.
func (m *Manager) HasContentLocks(ctx context.Context) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_locks").Scan(&count); err != nil {
		return false, fmt.Errorf("count content locks: %w", err)
	}
	return count > 0, nil
}

func (m *Manager) AssertNoteDeletion(ctx context.Context, noteID string) error {
	if err := m.AssertNoteAccess(ctx, noteID); err != nil {
		return err
	}
	_, found, err := m.getLockByTarget(ctx, Target{Type: TargetNote, ID: noteID})
	if err != nil {
		return err
	}
	if found {
		return ErrValidation
	}
	return nil
}

func (m *Manager) AssertNotebookDeletion(ctx context.Context, notebookID string) error {
	if err := m.AssertNotebookAccess(ctx, notebookID); err != nil {
		return err
	}
	locked, err := m.hasExplicitLockInNotebookSubtree(ctx, notebookID)
	if err != nil {
		return err
	}
	if locked {
		return ErrValidation
	}
	return nil
}

// AssertAIAllowed deliberately rejects every protected note, even when that
// note is unlocked for editing. This keeps note content out of provider input
// and avoids creating new persisted AI-derived plaintext.
func (m *Manager) AssertAIAllowed(ctx context.Context, noteID string) error {
	protected, err := m.IsNoteProtected(ctx, noteID)
	if err != nil {
		return err
	}
	if protected {
		return ErrLocked
	}
	return nil
}

func (m *Manager) PrepareNewNote(ctx context.Context, noteID string, notebookID *string) (func(), error) {
	if !validEntityID(noteID) {
		return nil, ErrValidation
	}
	pending := pendingWrite{setNotebookID: true, notebookID: copyStringPointer(notebookID)}
	if err := m.requireMaterials(ctx, noteID, pending); err != nil {
		return nil, err
	}
	return m.installPending(noteID, pending), nil
}

func (m *Manager) PrepareNoteWrite(ctx context.Context, noteID string, notebookID *string) (func(), error) {
	if !validEntityID(noteID) {
		return nil, ErrValidation
	}
	if err := m.requireMaterials(ctx, noteID, pendingWrite{}); err != nil {
		return nil, err
	}
	pending := pendingWrite{setNotebookID: true, notebookID: copyStringPointer(notebookID)}
	if err := m.requireMaterials(ctx, noteID, pending); err != nil {
		return nil, err
	}
	return m.installPending(noteID, pending), nil
}

func (m *Manager) installPending(noteID string, pending pendingWrite) func() {
	m.mu.Lock()
	previous, hadPrevious := m.pending[noteID]
	m.pending[noteID] = pending
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		if hadPrevious {
			m.pending[noteID] = previous
		} else {
			delete(m.pending, noteID)
		}
		m.mu.Unlock()
	}
}

// Encode is called by MarkdownStore before a normal write reaches disk.
func (m *Manager) Encode(ctx context.Context, noteID string, plain []byte) ([]byte, error) {
	pending := m.pendingFor(noteID)
	locks, err := m.locksForNote(ctx, noteID, pending)
	if err != nil {
		return nil, err
	}
	if len(locks) == 0 {
		return plain, nil
	}
	materials, err := m.materialsForLocks(locks)
	if err != nil {
		return nil, err
	}
	defer zeroMaterials(materials)
	return encryptContent(noteID, materials, plain)
}

// Decode is called by MarkdownStore after bytes are read from disk.
func (m *Manager) Decode(ctx context.Context, noteID string, stored []byte) ([]byte, error) {
	locks, err := m.locksForNote(ctx, noteID, pendingWrite{})
	if err != nil {
		return nil, err
	}
	if len(locks) == 0 {
		if isEncryptedContent(stored) {
			return nil, ErrIntegrity
		}
		return stored, nil
	}
	if !isEncryptedContent(stored) {
		return nil, ErrIntegrity
	}
	materials, err := m.materialsForLocks(locks)
	if err != nil {
		return nil, err
	}
	defer zeroMaterials(materials)
	return decryptContent(noteID, materials, stored)
}

func (m *Manager) Enable(ctx context.Context, input EnableInput) (Lock, int, error) {
	target, err := normalizeTarget(Target{Type: input.TargetType, ID: input.TargetID})
	if err != nil {
		return Lock{}, 0, err
	}
	passphrase, err := validatePassphrase(input.Passphrase)
	if err != nil {
		return Lock{}, 0, err
	}

	endOperation := m.beginLockOperation()
	defer endOperation()
	if err := m.ensureNoOperation(ctx); err != nil {
		return Lock{}, 0, err
	}
	if err := m.requireTargetExists(ctx, target); err != nil {
		return Lock{}, 0, err
	}
	if _, found, err := m.getLockByTarget(ctx, target); err != nil {
		return Lock{}, 0, err
	} else if found {
		return Lock{}, 0, ErrAlreadyEnabled
	}
	if configured, err := m.hasConfiguredSync(ctx); err != nil {
		return Lock{}, 0, err
	} else if configured {
		return Lock{}, 0, ErrSyncDestinationChange
	}

	noteIDs, err := m.affectedNoteIDs(ctx, target)
	if err != nil {
		return Lock{}, 0, err
	}
	aiRecordCount, err := m.countAIRecords(ctx, noteIDs)
	if err != nil {
		return Lock{}, 0, err
	}
	if aiRecordCount > 0 && !input.DeleteAIRecords {
		return Lock{}, aiRecordCount, newAIRecordsRequiredError(aiRecordCount)
	}
	for _, noteID := range noteIDs {
		if err := m.requireMaterials(ctx, noteID, pendingWrite{}); err != nil {
			return Lock{}, aiRecordCount, err
		}
	}

	lockID, err := newID()
	if err != nil {
		return Lock{}, aiRecordCount, err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return Lock{}, aiRecordCount, err
	}
	lockKey, err := randomBytes(kdfKeyLength)
	if err != nil {
		return Lock{}, aiRecordCount, err
	}
	nonce, wrappedKey, err := wrapLockKey(lockID, passphrase, salt, kdfMemoryKiB, kdfIterations, kdfParallelism, lockKey)
	if err != nil {
		zeroBytes(lockKey)
		return Lock{}, aiRecordCount, err
	}
	now := time.Now().UTC()
	record := lockRecord{
		Lock: Lock{ID: lockID, TargetType: target.Type, TargetID: target.ID, CreatedAt: now, UpdatedAt: now},
		Salt: salt, MemoryKiB: kdfMemoryKiB, Iterations: kdfIterations, Parallelism: kdfParallelism, WrapNonce: nonce, WrappedKey: wrappedKey,
	}
	operationID, err := newID()
	if err != nil {
		zeroBytes(lockKey)
		return Lock{}, aiRecordCount, err
	}
	operation := lockOperation{
		ID: operationID, Kind: "enable", Target: target, LockID: lockID, Lock: &record,
		DeleteAIRecords: input.DeleteAIRecords, Stage: "staging", CreatedAt: now, NoteIDs: noteIDs,
	}
	if err := m.insertOperation(ctx, operation); err != nil {
		zeroBytes(lockKey)
		return Lock{}, aiRecordCount, err
	}
	// The new key must be available while staging the future ciphertext, but it
	// is removed again if the operation never commits metadata.
	m.putSessionKey(lockID, lockKey)

	cleanups := make([]func(), 0, len(noteIDs))
	staged := false
	completed := false
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
		if !staged {
			m.rollbackStaging(context.Background(), operation)
		}
		if !completed {
			m.removeSessionKey(lockID)
		}
	}()
	for _, noteID := range noteIDs {
		cleanup := m.installPending(noteID, pendingWrite{additionalLock: &record})
		cleanups = append(cleanups, cleanup)
		content, readErr := m.store.Read(ctx, noteID)
		if readErr != nil {
			return Lock{}, aiRecordCount, readErr
		}
		if writeErr := m.store.WriteTemp(ctx, noteID, operationID, content); writeErr != nil {
			return Lock{}, aiRecordCount, writeErr
		}
	}
	if err := m.commitEnableMetadata(ctx, operation); err != nil {
		return Lock{}, aiRecordCount, err
	}
	staged = true
	if err := m.promoteOperation(ctx, operation); err != nil {
		return Lock{}, aiRecordCount, err
	}
	if err := m.finalizeOperation(ctx, operation); err != nil {
		return Lock{}, aiRecordCount, err
	}
	completed = true
	lock := record.Lock
	lock.TargetName = m.targetName(ctx, target)
	lock.Unlocked = true
	return lock, aiRecordCount, nil
}

func (m *Manager) Unlock(ctx context.Context, input UnlockInput) (Lock, error) {
	target, err := normalizeTarget(Target{Type: input.TargetType, ID: input.TargetID})
	if err != nil {
		return Lock{}, err
	}
	passphrase, err := validatePassphrase(input.Passphrase)
	if err != nil {
		return Lock{}, err
	}
	// Unlock and lock-now both mutate the in-memory key set. Use the same
	// writer gate so an expiring timer cannot discard a key while a successful
	// passphrase verification is adding it.
	endOperation := m.beginLockOperation()
	defer endOperation()
	record, found, err := m.getLockByTarget(ctx, target)
	if err != nil {
		return Lock{}, err
	}
	if !found {
		return Lock{}, ErrNotFound
	}
	if m.isUnlockThrottled(record.ID) {
		return Lock{}, ErrPassphraseInvalid
	}
	key, err := unwrapLockKey(record.ID, passphrase, record.Salt, record.MemoryKiB, record.Iterations, record.Parallelism, record.WrapNonce, record.WrappedKey)
	if err != nil {
		m.recordUnlockFailure(record.ID)
		return Lock{}, err
	}
	m.putSessionKey(record.ID, key)
	m.clearUnlockFailure(record.ID)
	lock := record.Lock
	lock.TargetName = m.targetName(ctx, target)
	lock.Unlocked = true
	return lock, nil
}

func (m *Manager) LockNow(ctx context.Context, target Target) (Lock, error) {
	target, err := normalizeTarget(target)
	if err != nil {
		return Lock{}, err
	}
	// Wait for an in-flight content read/write to finish before discarding its
	// key. This prevents a save from observing a session key halfway through a
	// lock-now request.
	endOperation := m.beginLockOperation()
	defer endOperation()
	record, found, err := m.getLockByTarget(ctx, target)
	if err != nil {
		return Lock{}, err
	}
	if !found {
		return Lock{}, ErrNotFound
	}
	m.removeSessionKey(record.ID)
	lock := record.Lock
	lock.TargetName = m.targetName(ctx, target)
	return lock, nil
}

// LockTargetsNow discards the available session keys for the requested lock
// targets as one writer operation. Missing, disabled, or already locked
// targets are deliberately treated as no-ops so a scheduled expiry can safely
// race with a user disabling a lock.
func (m *Manager) LockTargetsNow(ctx context.Context, targets []Target) ([]Lock, error) {
	unique := make(map[string]struct{}, len(targets))
	normalized := make([]Target, 0, len(targets))
	for _, target := range targets {
		target, err := normalizeTarget(target)
		if err != nil {
			return nil, err
		}
		key := target.Type + ":" + target.ID
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = struct{}{}
		normalized = append(normalized, target)
	}

	endOperation := m.beginLockOperation()
	defer endOperation()
	locked := make([]Lock, 0, len(normalized))
	for _, target := range normalized {
		record, found, err := m.getLockByTarget(ctx, target)
		if err != nil {
			return nil, err
		}
		if !found || !m.hasSessionKey(record.ID) {
			continue
		}
		m.removeSessionKey(record.ID)
		lock := record.Lock
		lock.TargetName = m.targetName(ctx, target)
		locked = append(locked, lock)
	}
	return locked, nil
}

func (m *Manager) ChangePassphrase(ctx context.Context, input ChangePassphraseInput) (Lock, error) {
	target, err := normalizeTarget(Target{Type: input.TargetType, ID: input.TargetID})
	if err != nil {
		return Lock{}, err
	}
	current, err := validatePassphrase(input.CurrentPassphrase)
	if err != nil {
		return Lock{}, err
	}
	next, err := validatePassphrase(input.NewPassphrase)
	if err != nil {
		return Lock{}, err
	}
	endOperation := m.beginLockOperation()
	defer endOperation()
	if err := m.ensureNoOperation(ctx); err != nil {
		return Lock{}, err
	}
	record, found, err := m.getLockByTarget(ctx, target)
	if err != nil {
		return Lock{}, err
	}
	if !found {
		return Lock{}, ErrNotFound
	}
	key, err := unwrapLockKey(record.ID, current, record.Salt, record.MemoryKiB, record.Iterations, record.Parallelism, record.WrapNonce, record.WrappedKey)
	if err != nil {
		return Lock{}, err
	}
	defer zeroBytes(key)
	salt, err := randomBytes(16)
	if err != nil {
		return Lock{}, err
	}
	nonce, wrappedKey, err := wrapLockKey(record.ID, next, salt, kdfMemoryKiB, kdfIterations, kdfParallelism, key)
	if err != nil {
		return Lock{}, err
	}
	now := time.Now().UTC()
	if _, err := m.db.ExecContext(ctx, `
UPDATE content_locks
SET kdf_salt = ?, kdf_memory_kib = ?, kdf_iterations = ?, kdf_parallelism = ?,
    wrap_nonce = ?, wrapped_key = ?, updated_at = ?
WHERE id = ?
`, salt, kdfMemoryKiB, kdfIterations, kdfParallelism, nonce, wrappedKey, formatTime(now), record.ID); err != nil {
		return Lock{}, fmt.Errorf("change lock passphrase: %w", err)
	}
	m.putSessionKey(record.ID, append([]byte(nil), key...))
	lock := record.Lock
	lock.UpdatedAt = now
	lock.TargetName = m.targetName(ctx, target)
	lock.Unlocked = true
	return lock, nil
}

func (m *Manager) Disable(ctx context.Context, input DisableInput) error {
	target, err := normalizeTarget(Target{Type: input.TargetType, ID: input.TargetID})
	if err != nil {
		return err
	}
	passphrase, err := validatePassphrase(input.Passphrase)
	if err != nil {
		return err
	}
	endOperation := m.beginLockOperation()
	defer endOperation()
	if err := m.ensureNoOperation(ctx); err != nil {
		return err
	}
	record, found, err := m.getLockByTarget(ctx, target)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	key, err := unwrapLockKey(record.ID, passphrase, record.Salt, record.MemoryKiB, record.Iterations, record.Parallelism, record.WrapNonce, record.WrappedKey)
	if err != nil {
		return err
	}
	m.putSessionKey(record.ID, key)
	completed := false
	defer func() {
		// A failed disable must not leave a passphrase-derived key resident in
		// memory. The caller can explicitly unlock again after resolving the
		// reported error.
		if !completed {
			m.removeSessionKey(record.ID)
		}
	}()
	noteIDs, err := m.affectedNoteIDs(ctx, target)
	if err != nil {
		return err
	}
	for _, noteID := range noteIDs {
		if err := m.requireMaterials(ctx, noteID, pendingWrite{}); err != nil {
			return err
		}
		if err := m.requireMaterials(ctx, noteID, pendingWrite{removeLockID: record.ID}); err != nil {
			return err
		}
	}
	operationID, err := newID()
	if err != nil {
		return err
	}
	operation := lockOperation{ID: operationID, Kind: "disable", Target: target, LockID: record.ID, Stage: "staging", CreatedAt: time.Now().UTC(), NoteIDs: noteIDs}
	if err := m.insertOperation(ctx, operation); err != nil {
		return err
	}
	staged := false
	cleanups := make([]func(), 0, len(noteIDs))
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
		if !staged {
			m.rollbackStaging(context.Background(), operation)
		}
	}()
	for _, noteID := range noteIDs {
		cleanup := m.installPending(noteID, pendingWrite{removeLockID: record.ID})
		cleanups = append(cleanups, cleanup)
		content, readErr := m.store.Read(ctx, noteID)
		if readErr != nil {
			return readErr
		}
		if writeErr := m.store.WriteTemp(ctx, noteID, operationID, content); writeErr != nil {
			return writeErr
		}
	}
	if err := m.commitDisableMetadata(ctx, operation); err != nil {
		return err
	}
	staged = true
	if err := m.promoteOperation(ctx, operation); err != nil {
		return err
	}
	if err := m.finalizeOperation(ctx, operation); err != nil {
		return err
	}
	m.removeSessionKey(record.ID)
	completed = true
	return nil
}

// Recover completes only an already committed file promotion. A staging
// operation has never changed visible lock metadata and is safely discarded.
func (m *Manager) Recover(ctx context.Context) error {
	endOperation := m.beginLockOperation()
	defer endOperation()
	operations, err := m.listOperations(ctx)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		switch operation.Stage {
		case "staging":
			if err := m.rollbackStaging(ctx, operation); err != nil {
				return err
			}
		case "committing":
			if err := m.recoverCommitting(ctx, operation); err != nil {
				return err
			}
		default:
			return ErrIntegrity
		}
	}
	return nil
}

func (m *Manager) recoverCommitting(ctx context.Context, operation lockOperation) error {
	_, lockFound, err := m.getLockByID(ctx, operation.LockID)
	if err != nil {
		return err
	}
	if operation.Kind == "enable" && !lockFound {
		return m.rollbackStaging(ctx, operation)
	}
	if operation.Kind == "disable" && lockFound {
		return m.rollbackStaging(ctx, operation)
	}
	if err := m.promoteOperation(ctx, operation); err != nil {
		return err
	}
	return m.finalizeOperation(ctx, operation)
}

func (m *Manager) commitEnableMetadata(ctx context.Context, operation lockOperation) error {
	if operation.Lock == nil {
		return ErrIntegrity
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin enable content lock tx: %w", err)
	}
	defer tx.Rollback()
	lock := operation.Lock
	if _, err := tx.ExecContext(ctx, `
INSERT INTO content_locks(
  id, target_type, target_id, kdf_salt, kdf_memory_kib, kdf_iterations,
  kdf_parallelism, wrap_nonce, wrapped_key, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, lock.ID, lock.TargetType, lock.TargetID, lock.Salt, lock.MemoryKiB, lock.Iterations, lock.Parallelism, lock.WrapNonce, lock.WrappedKey, formatTime(lock.CreatedAt), formatTime(lock.UpdatedAt)); err != nil {
		return fmt.Errorf("insert content lock: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE content_lock_operations SET stage = 'committing' WHERE operation_id = ? AND stage = 'staging'", operation.ID); err != nil {
		return fmt.Errorf("commit content lock operation metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit enable content lock tx: %w", err)
	}
	return nil
}

func (m *Manager) commitDisableMetadata(ctx context.Context, operation lockOperation) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin disable content lock tx: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "DELETE FROM content_locks WHERE id = ?", operation.LockID)
	if err != nil {
		return fmt.Errorf("delete content lock: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted content lock count: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, "UPDATE content_lock_operations SET stage = 'committing' WHERE operation_id = ? AND stage = 'staging'", operation.ID); err != nil {
		return fmt.Errorf("commit content unlock operation metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit disable content lock tx: %w", err)
	}
	return nil
}

func (m *Manager) promoteOperation(ctx context.Context, operation lockOperation) error {
	for _, noteID := range operation.NoteIDs {
		tempExists, err := m.store.TempExists(ctx, noteID, operation.ID)
		if err != nil {
			return err
		}
		if tempExists {
			if err := m.store.CommitTemp(ctx, noteID, operation.ID); err != nil {
				return err
			}
			continue
		}
		exists, err := m.store.Exists(ctx, noteID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrIntegrity
		}
	}
	return nil
}

func (m *Manager) finalizeOperation(ctx context.Context, operation lockOperation) error {
	if err := m.purgeDerivedData(ctx, operation.NoteIDs); err != nil {
		return err
	}
	if operation.Kind == "enable" {
		if operation.DeleteAIRecords {
			if err := m.deleteAIRecords(ctx, operation.NoteIDs); err != nil {
				return err
			}
		}
		if err := m.purgeLocalSyncState(ctx, operation.NoteIDs); err != nil {
			return err
		}
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM content_lock_operations WHERE operation_id = ?", operation.ID); err != nil {
		return fmt.Errorf("complete content lock operation: %w", err)
	}
	return nil
}

func (m *Manager) rollbackStaging(ctx context.Context, operation lockOperation) error {
	for _, noteID := range operation.NoteIDs {
		if err := m.store.RollbackTemp(ctx, noteID, operation.ID); err != nil {
			return err
		}
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM content_lock_operations WHERE operation_id = ?", operation.ID); err != nil {
		return fmt.Errorf("rollback content lock operation: %w", err)
	}
	return nil
}

func (m *Manager) requireMaterials(ctx context.Context, noteID string, pending pendingWrite) error {
	locks, err := m.locksForNote(ctx, noteID, pending)
	if err != nil {
		return err
	}
	// materialsForLocks returns independent key copies. Do not retain them just
	// for an availability check.
	materials, err := m.materialsForLocks(locks)
	if err != nil {
		return err
	}
	zeroMaterials(materials)
	return nil
}

func (m *Manager) materialsForLocks(locks []lockRecord) ([]keyMaterial, error) {
	if len(locks) == 0 {
		return nil, nil
	}
	materials := make([]keyMaterial, 0, len(locks))
	for _, lock := range locks {
		key, ok := m.sessionKey(lock.ID)
		if !ok {
			zeroMaterials(materials)
			return nil, ErrLocked
		}
		materials = append(materials, keyMaterial{ID: lock.ID, Key: key})
	}
	return materials, nil
}

func zeroMaterials(materials []keyMaterial) {
	for _, material := range materials {
		zeroBytes(material.Key)
	}
}

func (m *Manager) pendingFor(noteID string) pendingWrite {
	m.mu.RLock()
	pending := m.pending[noteID]
	pending.notebookID = copyStringPointer(pending.notebookID)
	m.mu.RUnlock()
	return pending
}

func (m *Manager) putSessionKey(lockID string, key []byte) {
	m.mu.Lock()
	if previous, ok := m.sessionKeys[lockID]; ok {
		zeroBytes(previous)
	}
	m.sessionKeys[lockID] = key
	m.mu.Unlock()
}

func (m *Manager) removeSessionKey(lockID string) {
	m.mu.Lock()
	if key, ok := m.sessionKeys[lockID]; ok {
		zeroBytes(key)
		delete(m.sessionKeys, lockID)
	}
	m.mu.Unlock()
}

func (m *Manager) sessionKey(lockID string) ([]byte, bool) {
	m.mu.RLock()
	key, ok := m.sessionKeys[lockID]
	copy := append([]byte(nil), key...)
	m.mu.RUnlock()
	return copy, ok
}

func (m *Manager) hasSessionKey(lockID string) bool {
	m.mu.RLock()
	_, ok := m.sessionKeys[lockID]
	m.mu.RUnlock()
	return ok
}

func (m *Manager) isUnlockThrottled(lockID string) bool {
	m.mu.RLock()
	failure := m.failures[lockID]
	m.mu.RUnlock()
	return time.Now().Before(failure.retryAfter)
}

func (m *Manager) recordUnlockFailure(lockID string) {
	m.mu.Lock()
	failure := m.failures[lockID]
	failure.attempts++
	if failure.attempts >= 5 {
		failure.attempts = 0
		failure.retryAfter = time.Now().Add(30 * time.Second)
	}
	m.failures[lockID] = failure
	m.mu.Unlock()
}

func (m *Manager) clearUnlockFailure(lockID string) {
	m.mu.Lock()
	delete(m.failures, lockID)
	m.mu.Unlock()
}

func normalizeTarget(target Target) (Target, error) {
	target.Type = strings.TrimSpace(target.Type)
	target.ID = strings.TrimSpace(target.ID)
	switch target.Type {
	case TargetSpace:
		return Target{Type: TargetSpace, ID: SpaceTargetID}, nil
	case TargetNotebook, TargetNote:
		if !validEntityID(target.ID) {
			return Target{}, ErrValidation
		}
		return target, nil
	default:
		return Target{}, ErrValidation
	}
}

func validEntityID(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse content lock timestamp: %w", err)
	}
	return parsed.UTC(), nil
}

func (m *Manager) ensureNoOperation(ctx context.Context) error {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_lock_operations").Scan(&count); err != nil {
		return fmt.Errorf("check content lock operation: %w", err)
	}
	if count > 0 {
		return ErrOperationInProgress
	}
	return nil
}

func (m *Manager) hasConfiguredSync(ctx context.Context) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_connections WHERE status <> 'disabled'").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check sync connection before locking: %w", err)
	}
	return count > 0, nil
}

func (m *Manager) targetName(ctx context.Context, target Target) string {
	switch target.Type {
	case TargetSpace:
		return "この保存空間"
	case TargetNotebook:
		var name string
		if err := m.db.QueryRowContext(ctx, "SELECT name FROM notebooks WHERE id = ?", target.ID).Scan(&name); err == nil {
			return name
		}
		return "ノートブック"
	case TargetNote:
		var title string
		if err := m.db.QueryRowContext(ctx, "SELECT title FROM notes WHERE id = ?", target.ID).Scan(&title); err == nil {
			return title
		}
		return "ノート"
	default:
		return ""
	}
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

func sortLockRecords(locks []lockRecord) {
	sort.Slice(locks, func(i, j int) bool {
		if locks[i].TargetType != locks[j].TargetType {
			return locks[i].TargetType < locks[j].TargetType
		}
		return locks[i].ID < locks[j].ID
	})
}
