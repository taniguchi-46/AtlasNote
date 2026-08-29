package note

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
)

const (
	SyncEntityNote     = "note"
	SyncEntityNotebook = "notebook"
	SyncEntityTag      = "tag"
	SyncEntityNoteTags = "note-tags"
)

// SyncChange is the durable hand-off from a local mutation to the sync
// outbox. ObjectJSON is an immutable, canonical entity payload; tombstones
// intentionally have no payload.
type SyncChange struct {
	ChangeSetID string
	EntityKey   string
	EntityType  string
	ObjectJSON  []byte
	Deleted     bool
}

type SyncChangeRecorder interface {
	RecordSyncChanges(context.Context, *sql.Tx, []SyncChange) error
	DiscardUnsyncedChanges(context.Context, *sql.Tx, []string) error
}

type SyncNotePayload struct {
	ID         string  `json:"id"`
	NotebookID *string `json:"notebookId"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	IsFavorite bool    `json:"isFavorite"`
	IsPinned   bool    `json:"isPinned"`
	IsTrashed  bool    `json:"isTrashed"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type SyncNotebookPayload struct {
	ID        string  `json:"id"`
	ParentID  *string `json:"parentId"`
	Name      string  `json:"name"`
	Icon      string  `json:"icon"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

type SyncTagPayload struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type SyncNoteTagsPayload struct {
	NoteID string   `json:"noteId"`
	TagIDs []string `json:"tagIds"`
}

type syncApplyContextKey struct{}
type syncExclusiveContextKey struct{}
type mutationGateContextKey struct{}

// BeginSyncExclusive prevents local mutations from changing the vault while a
// sync operation is comparing, applying, or committing one consistent state.
// The returned context lets sync-originated note mutations use the held gate.
func (s *Service) BeginSyncExclusive(ctx context.Context) (context.Context, func()) {
	s.syncGate.Lock()
	return context.WithValue(ctx, syncExclusiveContextKey{}, s), s.syncGate.Unlock
}

func (s *Service) lockMutation(ctx context.Context) (context.Context, func()) {
	if owner, _ := ctx.Value(syncExclusiveContextKey{}).(*Service); owner == s {
		return ctx, func() {}
	}
	if owner, _ := ctx.Value(mutationGateContextKey{}).(*Service); owner == s {
		return ctx, func() {}
	}
	s.syncGate.RLock()
	return context.WithValue(ctx, mutationGateContextKey{}, s), s.syncGate.RUnlock
}

func WithSyncApply(ctx context.Context) context.Context {
	return context.WithValue(ctx, syncApplyContextKey{}, true)
}

func isSyncApply(ctx context.Context) bool {
	value, _ := ctx.Value(syncApplyContextKey{}).(bool)
	return value
}

func SyncEntityKey(entityType string, id string) string {
	return entityType + ":" + id
}

func marshalSyncPayload(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal sync payload: %w", err)
	}
	return data, nil
}

func NewNoteSyncChange(changeSetID string, record Record, content string) (SyncChange, error) {
	payload, err := marshalSyncPayload(SyncNotePayload{
		ID:         record.ID,
		NotebookID: record.NotebookID,
		Title:      record.Title,
		Content:    content,
		IsFavorite: record.IsFavorite,
		IsPinned:   record.IsPinned,
		IsTrashed:  record.IsTrashed,
		CreatedAt:  formatTime(record.CreatedAt),
		UpdatedAt:  formatTime(record.UpdatedAt),
	})
	if err != nil {
		return SyncChange{}, err
	}
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNote, record.ID),
		EntityType:  SyncEntityNote,
		ObjectJSON:  payload,
	}, nil
}

func NewNoteTombstoneChange(changeSetID string, noteID string) SyncChange {
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNote, noteID),
		EntityType:  SyncEntityNote,
		Deleted:     true,
	}
}

func NewNotebookSyncChange(changeSetID string, notebook Notebook) (SyncChange, error) {
	payload, err := marshalSyncPayload(SyncNotebookPayload{
		ID:        notebook.ID,
		ParentID:  notebook.ParentID,
		Name:      notebook.Name,
		Icon:      notebook.Icon,
		CreatedAt: formatTime(notebook.CreatedAt),
		UpdatedAt: formatTime(notebook.UpdatedAt),
	})
	if err != nil {
		return SyncChange{}, err
	}
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNotebook, notebook.ID),
		EntityType:  SyncEntityNotebook,
		ObjectJSON:  payload,
	}, nil
}

func NewNotebookTombstoneChange(changeSetID string, notebookID string) SyncChange {
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNotebook, notebookID),
		EntityType:  SyncEntityNotebook,
		Deleted:     true,
	}
}

func NewTagSyncChange(changeSetID string, record tagRecord) (SyncChange, error) {
	payload, err := marshalSyncPayload(SyncTagPayload{
		ID:             record.ID,
		Name:           record.Name,
		NormalizedName: record.NormalizedName,
		CreatedAt:      formatTime(record.CreatedAt),
		UpdatedAt:      formatTime(record.UpdatedAt),
	})
	if err != nil {
		return SyncChange{}, err
	}
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityTag, record.ID),
		EntityType:  SyncEntityTag,
		ObjectJSON:  payload,
	}, nil
}

func NewTagTombstoneChange(changeSetID string, tagID string) SyncChange {
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityTag, tagID),
		EntityType:  SyncEntityTag,
		Deleted:     true,
	}
}

func NewNoteTagsSyncChange(changeSetID string, noteID string, tagIDs []string) (SyncChange, error) {
	ids := append([]string(nil), tagIDs...)
	sort.Strings(ids)
	uniqueIDs := ids[:0]
	for _, id := range ids {
		if len(uniqueIDs) == 0 || uniqueIDs[len(uniqueIDs)-1] != id {
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	payload, err := marshalSyncPayload(SyncNoteTagsPayload{NoteID: noteID, TagIDs: uniqueIDs})
	if err != nil {
		return SyncChange{}, err
	}
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNoteTags, noteID),
		EntityType:  SyncEntityNoteTags,
		ObjectJSON:  payload,
	}, nil
}

func NewNoteTagsTombstoneChange(changeSetID string, noteID string) SyncChange {
	return SyncChange{
		ChangeSetID: changeSetID,
		EntityKey:   SyncEntityKey(SyncEntityNoteTags, noteID),
		EntityType:  SyncEntityNoteTags,
		Deleted:     true,
	}
}

func (r *Repository) SetSyncChangeRecorder(recorder SyncChangeRecorder) {
	r.syncRecorder = recorder
}

func (r *Repository) recordSyncChanges(ctx context.Context, tx *sql.Tx, changes []SyncChange) error {
	if r.syncRecorder == nil || isSyncApply(ctx) || len(changes) == 0 {
		return nil
	}
	return r.syncRecorder.RecordSyncChanges(ctx, tx, changes)
}

func (r *Repository) discardUnsyncedChanges(ctx context.Context, tx *sql.Tx, entityKeys []string) error {
	if r.syncRecorder == nil || isSyncApply(ctx) || len(entityKeys) == 0 {
		return nil
	}
	return r.syncRecorder.DiscardUnsyncedChanges(ctx, tx, entityKeys)
}
