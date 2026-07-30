package ai

import (
	"context"
	"errors"

	"atlasnote/internal/note"
)

type ContextNote struct {
	NoteID    string
	Title     string
	Content   string
	Revision  int64
	Snippet   string
	IsTrashed bool
}

type NoteContextProvider interface {
	Get(ctx context.Context, noteID string) (ContextNote, error)
	Search(ctx context.Context, query string, limit int) ([]ContextNote, error)
	ListBacklinks(ctx context.Context, noteID string, limit int) ([]ContextNote, error)
}

type noteServiceContextProvider struct {
	service *note.Service
}

func NewNoteContextProvider(service *note.Service) NoteContextProvider {
	if service == nil {
		return nil
	}
	return noteServiceContextProvider{service: service}
}

func (p noteServiceContextProvider) Get(ctx context.Context, noteID string) (ContextNote, error) {
	current, err := p.service.Get(ctx, noteID)
	if err != nil {
		return ContextNote{}, err
	}
	return ContextNote{
		NoteID:    current.ID,
		Title:     current.Title,
		Content:   current.Content,
		Revision:  current.Revision,
		IsTrashed: current.IsTrashed,
	}, nil
}

func (p noteServiceContextProvider) Search(ctx context.Context, query string, limit int) ([]ContextNote, error) {
	result, err := p.service.Search(ctx, note.SearchInput{
		Query:          query,
		Scope:          note.SearchScopeAll,
		IncludeTrashed: false,
		Page:           1,
		PageSize:       limit,
	})
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, errors.New("AI context search index is unavailable")
	}

	items := make([]ContextNote, 0, len(result.Items))
	for _, item := range result.Items {
		current, err := p.Get(ctx, item.Note.ID)
		if err != nil {
			return nil, err
		}
		current.Snippet = item.Snippet
		items = append(items, current)
	}
	return items, nil
}

func (p noteServiceContextProvider) ListBacklinks(ctx context.Context, noteID string, limit int) ([]ContextNote, error) {
	result, err := p.service.ListBacklinks(ctx, note.BacklinkListInput{
		NoteID:   noteID,
		Page:     1,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ContextNote, 0, len(result.Items))
	for _, item := range result.Items {
		current, err := p.Get(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, current)
	}
	return items, nil
}
