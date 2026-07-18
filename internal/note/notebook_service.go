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

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return Notebook{}, err
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

	return nb, nil
}

func (s *Service) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return nil, err
	}
	return s.repository.ListNotebooks(ctx)
}

func (s *Service) UpdateNotebook(ctx context.Context, id string, input NotebookUpdateInput) (Notebook, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return Notebook{}, err
	}

	nb, err := s.repository.GetNotebook(ctx, id)
	if err != nil {
		return Notebook{}, err
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

	return nb, nil
}

func (s *Service) DeleteNotebook(ctx context.Context, id string, input NotebookDeleteInput) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
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
			content, readErr := s.store.Read(ctx, record.ID)
			if readErr != nil {
				return fmt.Errorf("read notebook note %s for sync: %w", record.ID, readErr)
			}
			record.IsTrashed = true
			record.Revision++
			record.UpdatedAt = now
			change, changeErr := NewNoteSyncChange(changeSetID, record, content)
			if changeErr != nil {
				return changeErr
			}
			changes = append(changes, change)
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
			change, changeErr := NewNoteSyncChange(changeSetID, record, content)
			if changeErr != nil {
				return changeErr
			}
			changes = append(changes, change)
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
