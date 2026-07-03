package note

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) CreateNotebook(ctx context.Context, input NotebookCreateInput) (Notebook, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Notebook{}, fmt.Errorf("%w: notebook name is required", ErrValidation)
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
