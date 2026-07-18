package note

import (
	"context"
	"fmt"
)

// ExportSyncChanges creates a consistent local snapshot for initial sync or
// remote import preparation. It reads all canonical note content through the
// Markdown store; SQLite-derived indexes are intentionally not exported.
func (s *Service) ExportSyncChanges(ctx context.Context) ([]SyncChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return nil, err
	}
	changeSetID, err := newID()
	if err != nil {
		return nil, err
	}
	changes := make([]SyncChange, 0)

	notebooks, err := s.repository.ListNotebooks(ctx)
	if err != nil {
		return nil, err
	}
	for _, notebook := range notebooks {
		change, err := NewNotebookSyncChange(changeSetID, notebook)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	tags, err := s.repository.ListTagRecords(ctx)
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		change, err := NewTagSyncChange(changeSetID, tag)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	records, err := s.repository.ListRecords(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		content, err := s.store.Read(ctx, record.ID)
		if err != nil {
			return nil, fmt.Errorf("read note %s for sync export: %w", record.ID, err)
		}
		change, err := NewNoteSyncChange(changeSetID, record, content)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)

		tagIDs, err := s.repository.ListNoteTagIDs(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		relation, err := NewNoteTagsSyncChange(changeSetID, record.ID, tagIDs)
		if err != nil {
			return nil, err
		}
		changes = append(changes, relation)
	}

	return changes, nil
}
