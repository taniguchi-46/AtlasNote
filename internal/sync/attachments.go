package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"

	attachmentstore "atlasnote/internal/attachment"
	"atlasnote/internal/note"
)

var managedAttachmentReferencePattern = regexp.MustCompile(`atlasnote-attachment://[A-Za-z0-9_-]+/[A-Za-z0-9_-]+`)

var errAttachmentParentDeleted = errors.New("attachment parent note is deleted remotely")

type attachmentSyncStore interface {
	ListAll(context.Context) ([]attachmentstore.Metadata, error)
	Read(context.Context, string, string) (attachmentstore.Metadata, []byte, error)
	ImportSyncAttachment(context.Context, attachmentstore.Metadata, []byte) error
	DeleteAttachment(context.Context, string, string) error
}

type syncAttachmentStager interface {
	StageSyncAttachment(context.Context, string, string, attachmentstore.Metadata, []byte) error
	CommitSyncAttachments(context.Context, string, string) error
}

type syncAttachmentRecovery interface {
	RecoverSyncCopy(context.Context, string) (bool, error)
}

func (s *Service) collectAttachmentSyncChanges(ctx context.Context, changeSetID string) ([]note.SyncChange, error) {
	if s.attachments == nil {
		return nil, nil
	}
	items, err := s.attachments.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	changes := make([]note.SyncChange, 0, len(items))
	for _, item := range items {
		_, data, err := s.attachments.Read(ctx, item.NoteID, item.ID)
		if err != nil {
			return nil, err
		}
		change, err := newAttachmentSyncChange(changeSetID, item, data)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func newAttachmentSyncChange(changeSetID string, item attachmentstore.Metadata, data []byte) (note.SyncChange, error) {
	return note.NewAttachmentSyncChange(changeSetID, note.SyncAttachmentPayload{
		ID: item.ID, NoteID: item.NoteID, Kind: item.Kind, MIMEType: item.MIMEType,
		Name: item.Name, Size: item.Size, SHA256: item.SHA256, Width: item.Width,
		Height: item.Height, CreatedAt: item.CreatedAt,
		Data: base64.StdEncoding.EncodeToString(data),
	})
}

func (s *Service) ensureAttachmentOutbox(ctx context.Context, remote remoteState) error {
	if s.attachments == nil {
		return nil
	}
	changes, err := s.collectAttachmentSyncChanges(ctx, "")
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	pending := make([]note.SyncChange, 0, len(changes))
	for _, change := range changes {
		object, err := objectDocument(change.EntityType, change.EntityKey, change.ObjectJSON, false)
		if err != nil {
			return err
		}
		objectHash := hashBytes(object)
		state, err := s.repository.GetItemState(ctx, change.EntityKey)
		if err != nil {
			return err
		}
		remoteEntry, remoteExists := remote.entries[change.EntityKey]
		parentChanged := false
		if !remoteExists {
			attachmentNoteID, _, ok := attachmentIDsFromKey(change.EntityKey)
			if !ok {
				return ErrInvalidRemoteFormat
			}
			parentState, parentErr := s.repository.GetItemState(ctx, note.SyncEntityKey(note.SyncEntityNote, attachmentNoteID))
			if parentErr != nil {
				return parentErr
			}
			parentChanged = parentState != nil && parentState.LocalObjectHash != parentState.BaseObjectHash
		}
		if state == nil || state.LocalObjectHash != state.BaseObjectHash ||
			(remoteExists && remoteEntry.ObjectHash != objectHash) || (!remoteExists && parentChanged) {
			pending = append(pending, change)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	return s.repository.EnqueueChanges(ctx, pending)
}

func (s *Service) recoverSyncCopyForNote(ctx context.Context, entityKey string) (bool, error) {
	recovery, ok := s.attachments.(syncAttachmentRecovery)
	if !ok {
		return true, nil
	}
	noteID, valid := entityIDFromKey(note.SyncEntityNote, entityKey)
	if !valid {
		return false, ErrInvalidRemoteFormat
	}
	return recovery.RecoverSyncCopy(ctx, noteID)
}

// ensureMissingLocalOutbox reconstructs the durable sync registration for a
// canonical local entity that was committed while the normal mutation path
// was intentionally suppressing automatic outbox recording. This is the
// recovery path for sync-side note copies: the note is already canonical, but
// its outbox transaction may not have completed. Attachments are reconstructed
// by ensureAttachmentOutbox immediately after this note registration.
// Existing item states are left untouched so retry counts and open conflicts
// are not reset. Sync-copy recovery is intentionally independent of the item
// state: a normal edit may have created that state before the staged
// attachments were recovered.
func (s *Service) ensureMissingLocalOutbox(ctx context.Context) error {
	if s.notes == nil {
		return nil
	}
	changes, err := s.notes.ExportSyncChanges(ctx)
	if err != nil {
		return err
	}
	pending := make([]note.SyncChange, 0, len(changes))
	for _, change := range changes {
		if change.EntityType != note.SyncEntityNote {
			continue
		}
		ready, err := s.recoverSyncCopyForNote(ctx, change.EntityKey)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		state, err := s.repository.GetItemState(ctx, change.EntityKey)
		if err != nil {
			return err
		}
		if state != nil {
			continue
		}
		pending = append(pending, change)
	}
	if len(pending) == 0 {
		return nil
	}
	return s.repository.EnqueueChanges(ctx, pending)
}

func (s *Service) ensureLocalNoteOutbox(ctx context.Context, noteID string) error {
	if s.notes == nil {
		return errors.New("note service is unavailable")
	}
	entityKey := note.SyncEntityKey(note.SyncEntityNote, noteID)
	state, err := s.repository.GetItemState(ctx, entityKey)
	if err != nil {
		return err
	}
	if state != nil && state.LocalObjectHash != state.BaseObjectHash {
		return nil
	}
	current, err := s.notes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	change, err := note.NewNoteSyncChange("", note.Record{
		ID: current.ID, NotebookID: current.NotebookID, Title: current.Title,
		IsFavorite: current.IsFavorite, IsPinned: current.IsPinned, IsTrashed: current.IsTrashed,
		Revision: current.Revision, CreatedAt: current.CreatedAt, UpdatedAt: current.UpdatedAt,
	}, current.Content)
	if err != nil {
		return err
	}
	return s.repository.EnqueueChanges(ctx, []note.SyncChange{change})
}

func (s *Service) ensureLocalNoteAttachmentsOutbox(ctx context.Context, noteID string, changeSetID string) error {
	if s.attachments == nil {
		return nil
	}
	changes, err := s.collectAttachmentSyncChanges(ctx, changeSetID)
	if err != nil {
		return err
	}
	pending := make([]note.SyncChange, 0, len(changes))
	for _, change := range changes {
		attachmentNoteID, _, ok := attachmentIDsFromKey(change.EntityKey)
		if ok && attachmentNoteID == noteID {
			pending = append(pending, change)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	return s.repository.EnqueueChanges(ctx, pending)
}

func (s *Service) remoteAttachmentParentDeleted(ctx context.Context, client RemoteClient, remote remoteState, item OutboxItem) (bool, error) {
	noteID, _, ok := attachmentIDsFromKey(item.EntityKey)
	if !ok {
		return false, ErrInvalidRemoteFormat
	}
	parentKey := note.SyncEntityKey(note.SyncEntityNote, noteID)
	parent, ok := remote.entries[parentKey]
	if !ok || parent.EntityType != note.SyncEntityNote {
		return false, ErrInvalidRemoteFormat
	}
	if remote.noteDeleted != nil {
		if deleted, known := remote.noteDeleted[noteID]; known {
			return deleted, nil
		}
	}
	raw, err := s.fetchRemoteObject(ctx, client, parent)
	if err != nil {
		return false, err
	}
	object, err := decodeObject(raw)
	if err != nil {
		return false, err
	}
	if remote.noteDeleted != nil {
		remote.noteDeleted[noteID] = object.Deleted
	}
	return object.Deleted, nil
}

func (s *Service) ensureAttachmentParentConflict(ctx context.Context, client RemoteClient, remote remoteState, item OutboxItem) (bool, error) {
	noteID, _, ok := attachmentIDsFromKey(item.EntityKey)
	if !ok {
		return false, ErrInvalidRemoteFormat
	}
	noteKey := note.SyncEntityKey(note.SyncEntityNote, noteID)
	open, err := s.repository.HasOpenConflict(ctx, noteKey)
	if err != nil {
		return false, err
	}
	if open {
		return true, nil
	}
	state, err := s.repository.GetItemState(ctx, noteKey)
	if err != nil {
		return false, err
	}
	if state != nil && state.LocalObjectHash != state.BaseObjectHash {
		return false, nil
	}
	if err := s.ensureLocalNoteOutbox(ctx, noteID); err != nil {
		return false, err
	}
	state, err = s.repository.GetItemState(ctx, noteKey)
	if err != nil {
		return false, err
	}
	if state == nil || state.SnapshotJSON == "" {
		return false, ErrInvalidRemoteFormat
	}
	parent, ok := remote.entries[noteKey]
	if !ok || parent.EntityType != note.SyncEntityNote {
		return false, ErrInvalidRemoteFormat
	}
	remoteRaw, err := s.fetchRemoteObject(ctx, client, parent)
	if err != nil {
		return false, err
	}
	remoteObject, err := decodeObject(remoteRaw)
	if err != nil || !remoteObject.Deleted {
		return false, ErrInvalidRemoteFormat
	}
	baseSnapshot, err := baseSnapshotForState(ctx, s.repository, *state)
	if err != nil {
		return false, err
	}
	conflictID, err := randomHex(16)
	if err != nil {
		return false, err
	}
	conflict := Conflict{
		ID:               conflictID,
		EntityKey:        noteKey,
		EntityType:       note.SyncEntityNote,
		LocalObjectHash:  state.LocalObjectHash,
		BaseObjectHash:   state.BaseObjectHash,
		RemoteObjectHash: parent.ObjectHash,
		LocalSnapshot:    state.SnapshotJSON,
		BaseSnapshot:     baseSnapshot,
		RemoteSnapshot:   string(remoteRaw),
		ConflictType:     "attachment-parent-deleted",
		ResolutionStatus: "open",
	}
	if err := s.repository.CreateConflict(ctx, conflict); err != nil {
		return false, err
	}
	return true, nil
}

func attachmentNoteID(item OutboxItem) string {
	noteID, _, _ := attachmentIDsFromKey(item.EntityKey)
	return noteID
}

func (s *Service) copyNoteAttachments(ctx context.Context, sourceNoteID string, targetNoteID string, content string, changeSetID string) (string, []note.SyncChange, error) {
	references := make(map[string]struct{})
	forEachManagedAttachmentReference(content, func(reference string) {
		noteID, attachmentID, ok := attachmentstore.ParseReference(reference)
		if ok && noteID == sourceNoteID {
			references[attachmentID] = struct{}{}
		}
	})
	if len(references) == 0 {
		return content, nil, nil
	}
	if s.attachments == nil {
		return "", nil, errors.New("attachment store is unavailable")
	}
	stager, ok := s.attachments.(syncAttachmentStager)
	if !ok {
		return "", nil, errors.New("attachment staging is unavailable")
	}
	items, err := s.attachments.ListAll(ctx)
	if err != nil {
		return "", nil, err
	}
	byID := make(map[string]attachmentstore.Metadata)
	for _, item := range items {
		if item.NoteID == sourceNoteID {
			byID[item.ID] = item
		}
	}
	attachmentIDs := make([]string, 0, len(references))
	for attachmentID := range references {
		attachmentIDs = append(attachmentIDs, attachmentID)
	}
	sort.Strings(attachmentIDs)
	replacements := make(map[string]string, len(attachmentIDs))
	changes := make([]note.SyncChange, 0, len(attachmentIDs))
	for _, attachmentID := range attachmentIDs {
		_, ok := byID[attachmentID]
		if !ok {
			return "", nil, attachmentstore.ErrAttachmentNotFound
		}
		metadata, data, err := s.attachments.Read(ctx, sourceNoteID, attachmentID)
		if err != nil {
			return "", nil, err
		}
		copyID := hashBytes([]byte("conflict-remote-copy-attachment:" + changeSetID + ":" + attachmentID))[:32]
		copied := metadata
		copied.ID = copyID
		copied.NoteID = targetNoteID
		if err := stager.StageSyncAttachment(ctx, targetNoteID, changeSetID, copied, data); err != nil {
			return "", nil, err
		}
		change, err := newAttachmentSyncChange(changeSetID, copied, data)
		if err != nil {
			return "", nil, err
		}
		changes = append(changes, change)
		replacements[attachmentstore.Reference(sourceNoteID, attachmentID)] = attachmentstore.Reference(targetNoteID, copyID)
	}
	return rewriteManagedAttachmentReferences(content, replacements), changes, nil
}

func forEachManagedAttachmentReference(content string, visit func(string)) {
	for _, match := range managedAttachmentReferencePattern.FindAllStringIndex(content, -1) {
		if !isStandaloneManagedAttachmentReference(content, match[0], match[1]) {
			continue
		}
		visit(content[match[0]:match[1]])
	}
}

func rewriteManagedAttachmentReferences(content string, replacements map[string]string) string {
	matches := managedAttachmentReferencePattern.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var rewritten strings.Builder
	previousEnd := 0
	changed := false
	for _, match := range matches {
		if !isStandaloneManagedAttachmentReference(content, match[0], match[1]) {
			continue
		}
		replacement, ok := replacements[content[match[0]:match[1]]]
		if !ok {
			continue
		}
		if !changed {
			rewritten.Grow(len(content))
		}
		rewritten.WriteString(content[previousEnd:match[0]])
		rewritten.WriteString(replacement)
		previousEnd = match[1]
		changed = true
	}
	if !changed {
		return content
	}
	rewritten.WriteString(content[previousEnd:])
	return rewritten.String()
}

func isStandaloneManagedAttachmentReference(content string, start int, end int) bool {
	if start > 0 && isAttachmentReferenceContinuation(content[start-1]) {
		return false
	}
	if end < len(content) && isAttachmentReferenceContinuation(content[end]) {
		return false
	}
	segmentStart := start
	for segmentStart > 0 && content[segmentStart-1] > ' ' {
		segmentStart--
	}
	return !strings.Contains(content[segmentStart:start], "://")
}

func isAttachmentReferenceContinuation(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') || strings.ContainsRune("_-./?#&%+:", rune(value))
}

func (s *Service) attachmentOutboxIsActive(ctx context.Context, item OutboxItem) (bool, error) {
	if item.EntityType != note.SyncEntityAttachment || s.attachments == nil {
		return true, nil
	}
	noteID, attachmentID, ok := attachmentIDsFromKey(item.EntityKey)
	if !ok {
		return false, ErrInvalidRemoteFormat
	}
	_, _, err := s.attachments.Read(ctx, noteID, attachmentID)
	if errors.Is(err, attachmentstore.ErrAttachmentNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func applyRemoteAttachment(ctx context.Context, notes *note.Service, attachments attachmentSyncStore, object ObjectDocument) error {
	if attachments == nil || notes == nil {
		return ErrInvalidRemoteFormat
	}
	var payload note.SyncAttachmentPayload
	if err := json.Unmarshal(object.Payload, &payload); err != nil {
		return ErrInvalidRemoteFormat
	}
	if !isEntityID(payload.ID) || !isEntityID(payload.NoteID) || object.EntityKey != note.SyncAttachmentEntityKey(payload.NoteID, payload.ID) ||
		payload.Kind != "image" || (payload.MIMEType != "image/png" && payload.MIMEType != "image/jpeg") ||
		payload.Size < 1 || payload.Size > attachmentstore.MaxAttachmentBytes || !isSHA256Hex(payload.SHA256) {
		return ErrInvalidRemoteFormat
	}
	if _, err := notes.Get(ctx, payload.NoteID); err != nil {
		return ErrInvalidRemoteFormat
	}
	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil || int64(len(data)) != payload.Size {
		return ErrInvalidRemoteFormat
	}
	return attachments.ImportSyncAttachment(ctx, attachmentstore.Metadata{
		ID: payload.ID, NoteID: payload.NoteID, Kind: payload.Kind, MIMEType: payload.MIMEType,
		Name: payload.Name, Size: payload.Size, SHA256: payload.SHA256, Width: payload.Width,
		Height: payload.Height, CreatedAt: payload.CreatedAt,
	}, data)
}
