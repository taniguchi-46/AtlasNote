package note

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const defaultNotebookIcon = "default:note"

const (
	NotebookDeleteModeTrashNotes = "trashNotes"
	NotebookDeleteModeKeepNotes  = "keepNotes"
)

var notebookIconPattern = regexp.MustCompile(`^(default|user):[A-Za-z0-9_-]+$`)

func (s *Service) CreateNotebook(ctx context.Context, input NotebookCreateInput) (Notebook, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return Notebook{}, err
	}
	if input.ParentID != nil && s.contentLocks != nil {
		if err := s.contentLocks.AssertNotebookAccess(ctx, *input.ParentID); err != nil {
			return Notebook{}, err
		}
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrValidation)
	}

	icon, err := normalizeNotebookIcon(input.Icon)
	if err != nil {
		return Notebook{}, err
	}

	id, err := newID()
	if err != nil {
		return Notebook{}, err
	}

	now := time.Now().UTC()
	nb := Notebook{
		ID:        id,
		ParentID:  input.ParentID,
		Name:      name,
		Icon:      icon,
		CreatedAt: now,
		UpdatedAt: now,
	}

	change, err := NewNotebookSyncChange(id, nb)
	if err != nil {
		return Notebook{}, err
	}
	if err := s.repository.CreateNotebookWithSync(ctx, nb, []SyncChange{change}); err != nil {
		return Notebook{}, fmt.Errorf("create notebook: %w", err)
	}

	return s.annotateNotebook(ctx, nb), nil
}

func (s *Service) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return nil, err
	}
	items, err := s.repository.ListNotebooks(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index] = s.annotateNotebook(ctx, items[index])
	}
	return items, nil
}

func (s *Service) UpdateNotebook(ctx context.Context, id string, input NotebookUpdateInput) (Notebook, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return Notebook{}, err
	}

	nb, err := s.repository.GetNotebook(ctx, id)
	if err != nil {
		return Notebook{}, err
	}
	if s.contentLocks != nil {
		if err := s.contentLocks.AssertNotebookAccess(ctx, nb.ID); err != nil {
			return Notebook{}, err
		}
		if input.ParentID != nil {
			if err := s.contentLocks.AssertNotebookAccess(ctx, *input.ParentID); err != nil {
				return Notebook{}, err
			}
		}
		if input.ParentID != nil || (input.ClearParent != nil && *input.ClearParent) {
			hasLocks, err := s.contentLocks.HasContentLocks(ctx)
			if err != nil {
				return Notebook{}, err
			}
			if hasLocks {
				return Notebook{}, ErrValidation
			}
		}
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrValidation)
		}
		nb.Name = name
	}

	if input.Icon != nil {
		icon, err := normalizeNotebookIcon(input.Icon)
		if err != nil {
			return Notebook{}, err
		}
		nb.Icon = icon
	}

	if input.ClearParent != nil && *input.ClearParent {
		nb.ParentID = nil
	} else if input.ParentID != nil {
		if *input.ParentID == id {
			return Notebook{}, fmt.Errorf("%w: notebook cannot be its own parent", ErrValidation)
		}
		isDescendant, err := s.repository.IsNotebookDescendant(ctx, id, *input.ParentID)
		if err != nil {
			return Notebook{}, err
		}
		if isDescendant {
			return Notebook{}, fmt.Errorf("%w: notebook cannot be moved under its descendant", ErrValidation)
		}
		nb.ParentID = input.ParentID
	}

	nb.UpdatedAt = time.Now().UTC()

	changeSetID, err := newID()
	if err != nil {
		return Notebook{}, err
	}
	change, err := NewNotebookSyncChange(changeSetID, nb)
	if err != nil {
		return Notebook{}, err
	}
	if err := s.repository.UpdateNotebookWithSync(ctx, nb, []SyncChange{change}); err != nil {
		return Notebook{}, fmt.Errorf("update notebook: %w", err)
	}

	return s.annotateNotebook(ctx, nb), nil
}

func (s *Service) DeleteNotebook(ctx context.Context, id string, input NotebookDeleteInput) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	if s.contentLocks != nil {
		if err := s.contentLocks.AssertNotebookDeletion(ctx, id); err != nil {
			return err
		}
		if input.Mode == NotebookDeleteModeKeepNotes {
			hasLocks, err := s.contentLocks.HasContentLocks(ctx)
			if err != nil {
				return err
			}
			// Keeping notes moves both those notes and child notebooks. That can
			// change the effective key set of every descendant, so it needs the
			// same dedicated multi-file re-encryption operation as hierarchy drag
			// and drop before it can be supported safely.
			if hasLocks {
				return fmt.Errorf("%w: notebook hierarchy cannot be moved while content locks are configured", ErrValidation)
			}
		}
	}

	switch input.Mode {
	case NotebookDeleteModeTrashNotes:
		tree, err := s.repository.ListNotebookTree(ctx, id)
		if err != nil {
			return err
		}
		records, err := s.repository.ListRecordsInNotebookTree(ctx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		changeSetID, err := newID()
		if err != nil {
			return err
		}
		changes := make([]SyncChange, 0, len(tree)+len(records))
		for _, notebook := range tree {
			changes = append(changes, NewNotebookTombstoneChange(changeSetID, notebook.ID))
		}
		for _, record := range records {
			record.IsTrashed = true
			record.Revision++
			record.UpdatedAt = now
			if s.noteProtected(ctx, record.ID) {
				// Metadata-only notebook deletion may proceed for an unlocked
				// inherited lock, but its protected body must never be recreated in
				// the plaintext sync outbox.
				continue
			}
			content, readErr := s.store.Read(ctx, record.ID)
			if readErr != nil {
				return fmt.Errorf("read notebook note %s for sync: %w", record.ID, readErr)
			}
			noteChanges, changeErr := s.noteSyncChanges(ctx, changeSetID, record, content)
			if changeErr != nil {
				return changeErr
			}
			changes = append(changes, noteChanges...)
		}
		return s.repository.DeleteNotebookWithNotesTrashedAndSync(ctx, id, now, changes)
	case NotebookDeleteModeKeepNotes:
		notebook, err := s.repository.GetNotebook(ctx, id)
		if err != nil {
			return err
		}
		notebooks, err := s.repository.ListNotebooks(ctx)
		if err != nil {
			return err
		}
		records, err := s.repository.ListRecordsInNotebook(ctx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		changeSetID, err := newID()
		if err != nil {
			return err
		}
		changes := []SyncChange{NewNotebookTombstoneChange(changeSetID, notebook.ID)}
		for _, child := range notebooks {
			if child.ParentID == nil || *child.ParentID != id {
				continue
			}
			child.ParentID = nil
			child.UpdatedAt = now
			change, changeErr := NewNotebookSyncChange(changeSetID, child)
			if changeErr != nil {
				return changeErr
			}
			changes = append(changes, change)
		}
		for _, record := range records {
			content, readErr := s.store.Read(ctx, record.ID)
			if readErr != nil {
				return fmt.Errorf("read notebook note %s for sync: %w", record.ID, readErr)
			}
			record.NotebookID = nil
			record.Revision++
			record.UpdatedAt = now
			noteChanges, changeErr := s.noteSyncChanges(ctx, changeSetID, record, content)
			if changeErr != nil {
				return changeErr
			}
			changes = append(changes, noteChanges...)
		}
		return s.repository.DeleteNotebookKeepingNotesAndSync(ctx, id, now, changes)
	default:
		return fmt.Errorf("%w: notebook delete mode is invalid", ErrValidation)
	}
}

func normalizeNotebookIcon(icon *string) (string, error) {
	if icon == nil {
		return defaultNotebookIcon, nil
	}

	value := strings.TrimSpace(*icon)
	if value == "" {
		return defaultNotebookIcon, nil
	}
	if len(value) > 80 || !notebookIconPattern.MatchString(value) {
		return "", fmt.Errorf("%w: notebook icon is invalid", ErrValidation)
	}

	return value, nil
}
