package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

func (s *MarkdownStore) WriteTemp(ctx context.Context, id string, operationID string, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath, err := s.tempPath(id, operationID)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write markdown temp content: %w", err)
	}
	written := false
	defer func() {
		if !written {
			_ = file.Close()
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("write markdown temp content: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync markdown temp content: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close markdown temp content: %w", err)
	}
	written = true

	return nil
}

func (s *MarkdownStore) CommitTemp(ctx context.Context, id string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}

	tempPath, err := s.tempPath(id, operationID)
	if err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit markdown temp content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) RollbackTemp(ctx context.Context, id string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath, err := s.tempPath(id, operationID)
	if err != nil {
		return err
	}

	if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("rollback markdown temp content: %w", err)
	}

	return nil
}

func (s *MarkdownStore) Write(ctx context.Context, id string, content string) error {
	const operationID = "direct"
	if err := s.WriteTemp(ctx, id, operationID, content); err != nil {
		return err
	}

	if err := s.CommitTemp(ctx, id, operationID); err != nil {
		_ = s.RollbackTemp(context.Background(), id, operationID)
		return err
	}

	return nil
}

func (s *MarkdownStore) Exists(ctx context.Context, id string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat markdown content: %w", err)
}

func (s *MarkdownStore) TempExists(ctx context.Context, id string, operationID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	path, err := s.tempPath(id, operationID)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat markdown temp content: %w", err)
}

func (s *MarkdownStore) ContentMatches(ctx context.Context, id string, expectedHash string) (bool, error) {
	content, err := s.Read(ctx, id)
	if err != nil {
		return false, err
	}

	return HashContent(content) == expectedHash, nil
}

func (s *MarkdownStore) TempContentMatches(ctx context.Context, id string, operationID string, expectedHash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	path, err := s.tempPath(id, operationID)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read markdown temp content: %w", err)
	}

	return HashContent(string(content)) == expectedHash, nil
}

func (s *MarkdownStore) StageDelete(ctx context.Context, id string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}
	stagedPath, err := s.deletePath(id, operationID)
	if err != nil {
		return err
	}

	if err := os.Rename(path, stagedPath); err != nil {
		return fmt.Errorf("stage markdown delete: %w", err)
	}

	return nil
}

func (s *MarkdownStore) RestoreDelete(ctx context.Context, id string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.fullPath(id)
	if err != nil {
		return err
	}
	stagedPath, err := s.deletePath(id, operationID)
	if err != nil {
		return err
	}

	if err := os.Rename(stagedPath, path); err != nil {
		return fmt.Errorf("restore staged markdown delete: %w", err)
	}

	return nil
}

func (s *MarkdownStore) CommitDelete(ctx context.Context, id string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	stagedPath, err := s.deletePath(id, operationID)
	if err != nil {
		return err
	}

	if err := os.Remove(stagedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("commit staged markdown delete: %w", err)
	}

	return nil
}

func (s *MarkdownStore) DeleteStagedExists(ctx context.Context, id string, operationID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	path, err := s.deletePath(id, operationID)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat staged markdown delete: %w", err)
}

func (s *MarkdownStore) QuarantineOrphans(ctx context.Context, expected map[string]struct{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return fmt.Errorf("list markdown directory: %w", err)
	}

	recoveryDir := filepath.Join(s.rootDir, "recovery")
	for _, entry := range entries {
		if entry.IsDir() || !isManagedFile(entry.Name()) {
			continue
		}
		if _, ok := expected[entry.Name()]; ok {
			continue
		}

		if err := os.MkdirAll(recoveryDir, 0o700); err != nil {
			return fmt.Errorf("create markdown recovery directory: %w", err)
		}
		source := filepath.Join(s.rootDir, entry.Name())
		target := filepath.Join(recoveryDir, fmt.Sprintf("%s.%d", entry.Name(), time.Now().UnixNano()))
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("quarantine orphan markdown file: %w", err)
		}
	}

	return nil
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
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

func (s *MarkdownStore) tempPath(id string, operationID string) (string, error) {
	if err := validateID(operationID); err != nil {
		return "", fmt.Errorf("invalid operation id: %w", err)
	}
	path, err := s.fullPath(id)
	if err != nil {
		return "", err
	}

	return path + "." + operationID + ".tmp", nil
}

func (s *MarkdownStore) deletePath(id string, operationID string) (string, error) {
	if err := validateID(operationID); err != nil {
		return "", fmt.Errorf("invalid operation id: %w", err)
	}
	path, err := s.fullPath(id)
	if err != nil {
		return "", err
	}

	return path + "." + operationID + ".delete", nil
}

func isManagedFile(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".delete")
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
