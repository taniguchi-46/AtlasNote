package note

import (
	"context"
	"errors"
	"fmt"
	"time"

	"atlasnote/internal/storage"
)

func (s *Service) ApplySyncNote(ctx context.Context, payload SyncNotePayload) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	applyCtx := WithSyncApply(ctx)
	record, err := s.repository.Get(applyCtx, payload.ID)
	if errors.Is(err, ErrNotFound) {
		return s.createSyncNote(applyCtx, payload)
	}
	if err != nil {
		return err
	}
	updated, err := s.Update(applyCtx, record.ID, UpdateInput{
		NotebookID:       payload.NotebookID,
		ClearNotebook:    boolPointer(payload.NotebookID == nil),
		Title:            stringPointer(payload.Title),
		Content:          stringPointer(payload.Content),
		IsFavorite:       boolPointer(payload.IsFavorite),
		IsPinned:         boolPointer(payload.IsPinned),
		IsTrashed:        boolPointer(payload.IsTrashed),
		ExpectedRevision: int64Pointer(record.Revision),
	})
	if err != nil {
		return err
	}
	createdAt, updatedAt := syncTimes(payload.CreatedAt, payload.UpdatedAt, record.CreatedAt, updated.UpdatedAt)
	return s.repository.SetNoteSyncTimes(applyCtx, record.ID, createdAt, updatedAt)
}

func (s *Service) createSyncNote(ctx context.Context, payload SyncNotePayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	title, content, err := validateInput(payload.Title, payload.Content)
	if err != nil {
		return err
	}
	contentPath, err := s.store.ContentPath(payload.ID)
	if err != nil {
		return err
	}
	createdAt := parseSyncTime(payload.CreatedAt)
	updatedAt := parseSyncTime(payload.UpdatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	operationID, err := newID()
	if err != nil {
		return err
	}
	record := Record{
		ID:          payload.ID,
		NotebookID:  payload.NotebookID,
		Title:       title,
		ContentPath: contentPath,
		IsFavorite:  payload.IsFavorite,
		IsPinned:    payload.IsPinned,
		IsTrashed:   payload.IsTrashed,
		Revision:    1,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	operation := StorageOperation{
		ID:          operationID,
		NoteID:      payload.ID,
		Type:        StorageOperationUpsert,
		ContentHash: storage.HashContent(content),
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.store.WriteTemp(ctx, payload.ID, operationID, content); err != nil {
		return err
	}
	if err := s.repository.CreateWithStorageOperation(ctx, record, operation); err != nil {
		_ = s.store.RollbackTemp(context.Background(), payload.ID, operationID)
		return err
	}
	if err := s.store.CommitTemp(ctx, payload.ID, operationID); err != nil {
		_ = s.repository.RollbackCreatedNote(context.WithoutCancel(ctx), payload.ID, operationID)
		_ = s.store.RollbackTemp(context.Background(), payload.ID, operationID)
		return fmt.Errorf("commit synced markdown: %w", err)
	}
	s.updateSearchIndexLocked(ctx, record, content)
	s.updateNoteLinkIndexLocked(ctx, record, content)
	return s.repository.CompleteStorageOperation(context.Background(), operationID)
}

func (s *Service) DeleteSyncNote(ctx context.Context, noteID string) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	applyCtx := WithSyncApply(ctx)
	record, err := s.repository.Get(applyCtx, noteID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Delete(applyCtx, noteID, DeleteInput{ExpectedRevision: record.Revision})
}

func (s *Service) ApplySyncNotebook(ctx context.Context, payload SyncNotebookPayload) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	applyCtx := WithSyncApply(ctx)
	existing, err := s.repository.GetNotebook(applyCtx, payload.ID)
	if errors.Is(err, ErrNotFound) {
		return s.createSyncNotebook(applyCtx, payload)
	} else if err != nil {
		return err
	}
	updated, err := s.UpdateNotebook(applyCtx, payload.ID, NotebookUpdateInput{
		ParentID:    payload.ParentID,
		ClearParent: boolPointer(payload.ParentID == nil),
		Name:        stringPointer(payload.Name),
		Icon:        stringPointer(payload.Icon),
	})
	if err != nil {
		return err
	}
	createdAt, updatedAt := syncTimes(payload.CreatedAt, payload.UpdatedAt, existing.CreatedAt, updated.UpdatedAt)
	return s.repository.SetNotebookSyncTimes(applyCtx, payload.ID, createdAt, updatedAt)
}

func (s *Service) createSyncNotebook(ctx context.Context, payload SyncNotebookPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	name := payload.Name
	if name == "" {
		return fmt.Errorf("%w: notebook name is required", ErrValidation)
	}
	icon := payload.Icon
	if icon == "" {
		icon = defaultNotebookIcon
	}
	createdAt := parseSyncTime(payload.CreatedAt)
	updatedAt := parseSyncTime(payload.UpdatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return s.repository.CreateNotebook(ctx, Notebook{
		ID:        payload.ID,
		ParentID:  payload.ParentID,
		Name:      name,
		Icon:      icon,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
}

func (s *Service) DeleteSyncNotebook(ctx context.Context, notebookID string) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	applyCtx := WithSyncApply(ctx)
	if _, err := s.repository.GetNotebook(applyCtx, notebookID); errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	return s.DeleteNotebook(applyCtx, notebookID, NotebookDeleteInput{Mode: NotebookDeleteModeKeepNotes})
}

func (s *Service) ApplySyncTag(ctx context.Context, payload SyncTagPayload) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()
	_, normalizedName, validationError := normalizeTagName(payload.Name)
	if validationError != nil || payload.NormalizedName != normalizedName {
		return fmt.Errorf("%w: synced tag normalized name is invalid", ErrValidation)
	}

	applyCtx := WithSyncApply(ctx)
	existing, err := s.repository.GetTag(applyCtx, payload.ID)
	if errors.Is(err, ErrTagNotFound) {
		return s.createSyncTag(applyCtx, payload)
	} else if err != nil {
		return err
	}
	result, err := s.UpdateTag(applyCtx, payload.ID, TagUpdateInput{Name: payload.Name})
	if err != nil {
		return err
	}
	if result.Error != nil {
		return fmt.Errorf("apply synced tag: %s", result.Error.Message)
	}
	updatedAt := existing.UpdatedAt
	if result.Tag != nil {
		updatedAt = result.Tag.UpdatedAt
	}
	createdAt, updatedAt := syncTimes(payload.CreatedAt, payload.UpdatedAt, existing.CreatedAt, updatedAt)
	return s.repository.SetTagSyncTimes(applyCtx, payload.ID, createdAt, updatedAt)
}

func (s *Service) createSyncTag(ctx context.Context, payload SyncTagPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return err
	}
	name, normalizedName, validationError := normalizeTagName(payload.Name)
	if validationError != nil {
		return fmt.Errorf("%s", validationError.Message)
	}
	if payload.NormalizedName != normalizedName {
		return fmt.Errorf("%w: synced tag normalized name is invalid", ErrValidation)
	}
	createdAt := parseSyncTime(payload.CreatedAt)
	updatedAt := parseSyncTime(payload.UpdatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return s.repository.CreateTag(ctx, tagRecord{
		Tag:            Tag{ID: payload.ID, Name: name, CreatedAt: createdAt, UpdatedAt: updatedAt},
		NormalizedName: normalizedName,
	})
}

func (s *Service) DeleteSyncTag(ctx context.Context, tagID string) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	applyCtx := WithSyncApply(ctx)
	if _, err := s.repository.GetTag(applyCtx, tagID); errors.Is(err, ErrTagNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	_, err := s.DeleteTag(applyCtx, tagID)
	return err
}

func (s *Service) ApplySyncNoteTags(ctx context.Context, payload SyncNoteTagsPayload) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	result, err := s.SetNoteTags(WithSyncApply(ctx), payload.NoteID, SetNoteTagsInput{TagIDs: payload.TagIDs})
	if err != nil {
		return err
	}
	if result.Error != nil {
		return fmt.Errorf("apply synced note tags: %s", result.Error.Message)
	}
	return nil
}

func (s *Service) DeleteSyncNoteTags(ctx context.Context, noteID string) error {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	_, err := s.SetNoteTags(WithSyncApply(ctx), noteID, SetNoteTagsInput{TagIDs: nil})
	return err
}

func boolPointer(value bool) *bool       { return &value }
func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }

func parseSyncTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func syncTimes(createdValue string, updatedValue string, fallbackCreated time.Time, fallbackUpdated time.Time) (time.Time, time.Time) {
	createdAt := parseSyncTime(createdValue)
	if createdAt.IsZero() {
		createdAt = fallbackCreated
	}
	updatedAt := parseSyncTime(updatedValue)
	if updatedAt.IsZero() {
		updatedAt = fallbackUpdated
	}
	return createdAt, updatedAt
}
