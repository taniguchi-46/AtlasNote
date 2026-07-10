package note

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/storage"
)

const (
	maxTitleLength   = 200
	maxContentLength = 2 * 1024 * 1024
)

var (
	ErrNotFound         = errors.New("note not found")
	ErrValidation       = errors.New("note validation failed")
	ErrContentAvailable = errors.New("markdown content is available")
)

type Service struct {
	repository *Repository
	store      markdownStore
	mu         sync.Mutex
}

type markdownStore interface {
	CommitDelete(context.Context, string, string) error
	CommitTemp(context.Context, string, string) error
	ContentMatches(context.Context, string, string) (bool, error)
	ContentPath(string) (string, error)
	Delete(context.Context, string) error
	DeleteStagedExists(context.Context, string, string) (bool, error)
	Exists(context.Context, string) (bool, error)
	QuarantineOrphans(context.Context, map[string]struct{}) error
	Read(context.Context, string) (string, error)
	RestoreDelete(context.Context, string, string) error
	RollbackTemp(context.Context, string, string) error
	StageDelete(context.Context, string, string) error
	TempContentMatches(context.Context, string, string, string) (bool, error)
	TempExists(context.Context, string, string) (bool, error)
	WriteTemp(context.Context, string, string, string) error
}

func NewService(repository *Repository, store markdownStore) *Service {
	return &Service{
		repository: repository,
		store:      store,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return Note{}, err
	}

	title, content, err := validateInput(input.Title, input.Content)
	if err != nil {
		return Note{}, err
	}
	operationID, err := newID()
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
		NotebookID:  input.NotebookID,
		Title:       title,
		ContentPath: contentPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	operation := StorageOperation{
		ID:          operationID,
		NoteID:      id,
		Type:        StorageOperationUpsert,
		ContentHash: storage.HashContent(content),
		CreatedAt:   now,
	}

	if err := s.store.WriteTemp(ctx, id, operationID, content); err != nil {
		return Note{}, err
	}

	if err := s.repository.CreateWithStorageOperation(ctx, record, operation); err != nil {
		_ = s.store.RollbackTemp(context.Background(), id, operationID)
		return Note{}, fmt.Errorf("create note record: %w", err)
	}

	if err := s.store.CommitTemp(ctx, id, operationID); err != nil {
		rollbackErr := s.repository.RollbackCreatedNote(context.Background(), id, operationID)
		if rollbackErr == nil {
			_ = s.store.RollbackTemp(context.Background(), id, operationID)
			return Note{}, fmt.Errorf("commit markdown: %w", err)
		}
		return Note{}, fmt.Errorf("commit markdown: %w; rollback note record: %v", err, rollbackErr)
	}
	_ = s.repository.CompleteStorageOperation(context.Background(), operationID)

	return Note{
		ID:         record.ID,
		NotebookID: record.NotebookID,
		Title:      record.Title,
		Content:    content,
		IsFavorite: record.IsFavorite,
		IsPinned:   record.IsPinned,
		IsTrashed:  record.IsTrashed,
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}, nil
}

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return nil, err
	}
	return s.repository.List(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return Note{}, err
	}

	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}

	content, err := s.store.Read(ctx, record.ID)
	if err != nil {
		return Note{}, err
	}

	return Note{
		ID:         record.ID,
		NotebookID: record.NotebookID,
		Title:      record.Title,
		Content:    content,
		IsFavorite: record.IsFavorite,
		IsPinned:   record.IsPinned,
		IsTrashed:  record.IsTrashed,
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}, nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return Note{}, err
	}

	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return Note{}, err
	}
	previous := record

	content, err := s.store.Read(ctx, record.ID)
	if err != nil {
		return Note{}, err
	}

	if input.Title != nil {
		record.Title = *input.Title
	}
	if input.Content != nil {
		content = *input.Content
	}
	if input.ClearNotebook != nil && *input.ClearNotebook {
		record.NotebookID = nil
	} else if input.NotebookID != nil {
		record.NotebookID = input.NotebookID
	}
	if input.IsFavorite != nil {
		record.IsFavorite = *input.IsFavorite
	}
	if input.IsPinned != nil {
		record.IsPinned = *input.IsPinned
	}
	if input.IsTrashed != nil {
		record.IsTrashed = *input.IsTrashed
	}

	title, content, err := validateInput(record.Title, content)
	if err != nil {
		return Note{}, err
	}
	record.Title = title
	record.UpdatedAt = time.Now().UTC()

	if input.Content != nil {
		operationID, err := newID()
		if err != nil {
			return Note{}, err
		}
		operation := StorageOperation{
			ID:          operationID,
			NoteID:      record.ID,
			Type:        StorageOperationUpsert,
			ContentHash: storage.HashContent(content),
			CreatedAt:   time.Now().UTC(),
		}

		if err := s.store.WriteTemp(ctx, record.ID, operationID, content); err != nil {
			return Note{}, err
		}
		if err := s.repository.UpdateWithStorageOperation(ctx, record, operation); err != nil {
			_ = s.store.RollbackTemp(context.Background(), record.ID, operationID)
			return Note{}, fmt.Errorf("update note record: %w", err)
		}
		if err := s.store.CommitTemp(ctx, record.ID, operationID); err != nil {
			rollbackErr := s.repository.RollbackUpdatedNote(context.Background(), previous, operationID)
			if rollbackErr == nil {
				_ = s.store.RollbackTemp(context.Background(), record.ID, operationID)
				return Note{}, fmt.Errorf("commit markdown update: %w", err)
			}
			return Note{}, fmt.Errorf("commit markdown update: %w; rollback note record: %v", err, rollbackErr)
		}
		_ = s.repository.CompleteStorageOperation(context.Background(), operationID)
	} else {
		if err := s.repository.Update(ctx, record); err != nil {
			return Note{}, fmt.Errorf("update note record: %w", err)
		}
	}

	return Note{
		ID:         record.ID,
		NotebookID: record.NotebookID,
		Title:      record.Title,
		Content:    content,
		IsFavorite: record.IsFavorite,
		IsPinned:   record.IsPinned,
		IsTrashed:  record.IsTrashed,
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}

	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}

	operationID, err := newID()
	if err != nil {
		return err
	}
	operation := StorageOperation{
		ID:        operationID,
		NoteID:    record.ID,
		Type:      StorageOperationDelete,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repository.BeginStorageOperation(ctx, operation); err != nil {
		return fmt.Errorf("begin markdown delete: %w", err)
	}
	if err := s.store.StageDelete(ctx, record.ID, operationID); err != nil {
		_ = s.repository.CompleteStorageOperation(context.Background(), operationID)
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		restoreErr := s.store.RestoreDelete(context.Background(), record.ID, operationID)
		if restoreErr == nil {
			_ = s.repository.CompleteStorageOperation(context.Background(), operationID)
			return err
		}
		return fmt.Errorf("delete note record: %w; restore markdown: %v", err, restoreErr)
	}
	if err := s.store.CommitDelete(ctx, record.ID, operationID); err != nil {
		return fmt.Errorf("commit markdown delete: %w", err)
	}
	_ = s.repository.CompleteStorageOperation(context.Background(), operationID)

	return nil
}

func (s *Service) Recover(ctx context.Context) (RecoveryReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return RecoveryReport{}, err
	}

	records, err := s.repository.ListRecords(ctx)
	if err != nil {
		return RecoveryReport{}, err
	}
	report := RecoveryReport{MissingNotes: make([]MissingContent, 0)}
	expected := make(map[string]struct{}, len(records))
	for _, record := range records {
		contentPath, err := s.store.ContentPath(record.ID)
		if err != nil {
			return RecoveryReport{}, err
		}
		if record.ContentPath != contentPath {
			return RecoveryReport{}, fmt.Errorf("note %s has invalid content path", record.ID)
		}
		exists, err := s.store.Exists(ctx, record.ID)
		if err != nil {
			return RecoveryReport{}, err
		}
		if !exists {
			report.MissingNotes = append(report.MissingNotes, MissingContent{
				ID:          record.ID,
				Title:       record.Title,
				ContentPath: contentPath,
			})
		}
		expected[contentPath] = struct{}{}
	}

	if err := s.store.QuarantineOrphans(ctx, expected); err != nil {
		return RecoveryReport{}, err
	}
	return report, nil
}

func (s *Service) DeleteMissing(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	contentPath, err := s.store.ContentPath(record.ID)
	if err != nil {
		return err
	}
	if record.ContentPath != contentPath {
		return fmt.Errorf("note %s has invalid content path", record.ID)
	}
	exists, err := s.store.Exists(ctx, record.ID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w for note %s", ErrContentAvailable, record.ID)
	}

	return s.repository.Delete(ctx, record.ID)
}

func (s *Service) recoverPendingLocked(ctx context.Context) error {
	operations, err := s.repository.ListStorageOperations(ctx)
	if err != nil {
		return err
	}

	for _, operation := range operations {
		switch operation.Type {
		case StorageOperationUpsert:
			if err := s.recoverUpsertLocked(ctx, operation); err != nil {
				return err
			}
		case StorageOperationDelete:
			if err := s.recoverDeleteLocked(ctx, operation); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown note storage operation %q", operation.Type)
		}
	}

	return nil
}

func (s *Service) recoverUpsertLocked(ctx context.Context, operation StorageOperation) error {
	if _, err := s.repository.Get(ctx, operation.NoteID); err != nil {
		return fmt.Errorf("recover markdown for note %s: %w", operation.NoteID, err)
	}

	tempExists, err := s.store.TempExists(ctx, operation.NoteID, operation.ID)
	if err != nil {
		return err
	}
	if tempExists {
		matches, err := s.store.TempContentMatches(ctx, operation.NoteID, operation.ID, operation.ContentHash)
		if err != nil {
			return err
		}
		if !matches {
			return fmt.Errorf("markdown recovery temp hash mismatch for note %s", operation.NoteID)
		}
		if err := s.store.CommitTemp(ctx, operation.NoteID, operation.ID); err != nil {
			return fmt.Errorf("recover markdown update: %w", err)
		}
	} else {
		matches, err := s.store.ContentMatches(ctx, operation.NoteID, operation.ContentHash)
		if err != nil {
			return fmt.Errorf("verify recovered markdown: %w", err)
		}
		if !matches {
			return fmt.Errorf("markdown recovery hash mismatch for note %s", operation.NoteID)
		}
	}

	return s.repository.CompleteStorageOperation(ctx, operation.ID)
}

func (s *Service) recoverDeleteLocked(ctx context.Context, operation StorageOperation) error {
	_, recordErr := s.repository.Get(ctx, operation.NoteID)
	recordExists := recordErr == nil
	if recordErr != nil && !errors.Is(recordErr, ErrNotFound) {
		return recordErr
	}

	stagedExists, err := s.store.DeleteStagedExists(ctx, operation.NoteID, operation.ID)
	if err != nil {
		return err
	}
	contentExists, err := s.store.Exists(ctx, operation.NoteID)
	if err != nil {
		return err
	}

	if recordExists {
		if !stagedExists {
			if !contentExists {
				return fmt.Errorf("markdown content is missing during delete recovery for note %s", operation.NoteID)
			}
			return s.repository.CompleteStorageOperation(ctx, operation.ID)
		}
		if err := s.repository.Delete(ctx, operation.NoteID); err != nil {
			return err
		}
	}

	if stagedExists {
		if err := s.store.CommitDelete(ctx, operation.NoteID, operation.ID); err != nil {
			return err
		}
	} else if contentExists {
		if err := s.store.Delete(ctx, operation.NoteID); err != nil {
			return err
		}
	}

	return s.repository.CompleteStorageOperation(ctx, operation.ID)
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
