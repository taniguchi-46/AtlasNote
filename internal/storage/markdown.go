package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type MarkdownStore struct {
	rootDir string
}

func NewMarkdownStore(rootDir string) (*MarkdownStore, error) {
	rootDir = filepath.Clean(rootDir)
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return nil, fmt.Errorf("create markdown directory: %w", err)
	}

	return &MarkdownStore{rootDir: rootDir}, nil
}

func (s *MarkdownStore) ContentPath(id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}

	return id + ".md", nil
}

func (s *MarkdownStore) Read(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read markdown content: %w", err)
	}

	return string(content), nil
}

func (s *MarkdownStore) WriteTemp(ctx context.Context, id string, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write markdown temp content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) CommitTemp(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit markdown temp content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) RollbackTemp(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("rollback markdown temp content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) Write(ctx context.Context, id string, content string) error {
	if err := s.WriteTemp(ctx, id, content); err != nil {
		return err
	}
	
	if err := s.CommitTemp(ctx, id); err != nil {
		_ = s.RollbackTemp(context.Background(), id)
		return err
	}

	return nil
}

func (s *MarkdownStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete markdown content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) fullPath(id string) (string, error) {
	contentPath, err := s.ContentPath(id)
	if err != nil {
		return "", err
	}

	return filepath.Join(s.rootDir, contentPath), nil
}

func validateID(id string) error {
	if id == "" {
		return errors.New("id is required")
	}
	if !safeIDPattern.MatchString(id) {
		return errors.New("id contains unsafe characters")
	}

	return nil
}
