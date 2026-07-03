package note

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"atlasnote/internal/storage"
)

const (
	maxTitleLength   = 200
	maxContentLength = 2 * 1024 * 1024
)

var (
	ErrNotFound   = errors.New("note not found")
	ErrValidation = errors.New("note validation failed")
)

type Service struct {
	repository *Repository
	store      *storage.MarkdownStore
}

func NewService(repository *Repository, store *storage.MarkdownStore) *Service {
	return &Service{
		repository: repository,
		store:      store,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Note, error) {
	title, content, err := validateInput(input.Title, input.Content)
	if err != nil {
		return Note{}, err
	}

	id, err := newID()
	if err != nil {
		return Note{}, err
	}

	contentPath, err := s.store.ContentPath(id)
	if err != nil {
		return Note{}, err
	}

	now := time.Now().UTC()
	record := Record{
		ID:          id,
		Title:       title,
		ContentPath: contentPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.Write(ctx, id, content); err != nil {
		return Note{}, err
	}

	if err := s.repository.Create(ctx, record); err != nil {
		_ = s.store.Delete(context.Background(), id)
		return Note{}, err
	}

	return Note{
		ID:        record.ID,
		Title:     record.Title,
		Content:   content,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	return s.repository.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (Note, error) {
	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}

	content, err := s.store.Read(ctx, record.ID)
	if err != nil {
		return Note{}, err
	}

	return Note{
		ID:        record.ID,
		Title:     record.Title,
		Content:   content,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Note, error) {
	title, content, err := validateInput(input.Title, input.Content)
	if err != nil {
		return Note{}, err
	}

	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}

	record.Title = title
	record.UpdatedAt = time.Now().UTC()

	if err := s.store.Write(ctx, record.ID, content); err != nil {
		return Note{}, err
	}

	if err := s.repository.Update(ctx, record); err != nil {
		return Note{}, err
	}

	return Note{
		ID:        record.ID,
		Title:     record.Title,
		Content:   content,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}

	return s.store.Delete(ctx, record.ID)
}

func validateInput(title string, content string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len([]rune(title)) > maxTitleLength {
		return "", "", fmt.Errorf("%w: title is too long", ErrValidation)
	}
	if len(content) > maxContentLength {
		return "", "", fmt.Errorf("%w: content is too large", ErrValidation)
	}

	return title, content, nil
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate note id: %w", err)
	}

	return hex.EncodeToString(bytes[:]), nil
}
