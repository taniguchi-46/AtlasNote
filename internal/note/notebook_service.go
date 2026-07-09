package note

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const defaultNotebookIcon = "default:note"

var notebookIconPattern = regexp.MustCompile(`^(default|user):[A-Za-z0-9_-]+$`)

func (s *Service) CreateNotebook(ctx context.Context, input NotebookCreateInput) (Notebook, error) {
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

	if err := s.repository.CreateNotebook(ctx, nb); err != nil {
		return Notebook{}, fmt.Errorf("create notebook: %w", err)
	}

	return nb, nil
}

func (s *Service) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	return s.repository.ListNotebooks(ctx)
}

func (s *Service) UpdateNotebook(ctx context.Context, id string, input NotebookUpdateInput) (Notebook, error) {
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
		nb.ParentID = input.ParentID
	}

	nb.UpdatedAt = time.Now().UTC()

	if err := s.repository.UpdateNotebook(ctx, nb); err != nil {
		return Notebook{}, fmt.Errorf("update notebook: %w", err)
	}

	return nb, nil
}

func (s *Service) DeleteNotebook(ctx context.Context, id string) error {
	return s.repository.DeleteNotebook(ctx, id)
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
