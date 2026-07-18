package note

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var ErrTagNotFound = errors.New("tag not found")

var tagCaseFolder = cases.Fold()

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return nil, err
	}

	return s.repository.ListTags(ctx)
}

func (s *Service) ListNoteTags(ctx context.Context, noteID string) (NoteTagsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := NoteTagsResult{Tags: make([]Tag, 0)}
	if err := s.recoverPendingLocked(ctx); err != nil {
		return result, err
	}

	if _, err := s.repository.Get(ctx, noteID); err != nil {
		if errors.Is(err, ErrNotFound) {
			result.Error = tagError(TagErrorNoteNotFound, "ノートが見つかりません。", "noteId")
			return result, nil
		}
		return result, err
	}

	tags, err := s.repository.ListNoteTags(ctx, noteID)
	if err != nil {
		return result, err
	}
	result.Tags = tags
	return result, nil
}

func (s *Service) CreateTag(ctx context.Context, input TagCreateInput) (TagMutationResult, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	result := TagMutationResult{}
	if err := s.recoverPendingLocked(ctx); err != nil {
		return result, err
	}

	name, normalizedName, validationError := normalizeTagName(input.Name)
	if validationError != nil {
		result.Error = validationError
		return result, nil
	}
	if _, err := s.repository.GetTagByNormalizedName(ctx, normalizedName); err == nil {
		result.Error = tagError(TagErrorNameConflict, "同じ名前のタグが既に存在します。", "name")
		return result, nil
	} else if !errors.Is(err, ErrTagNotFound) {
		return result, err
	}

	id, err := newID()
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	record := tagRecord{
		Tag: Tag{
			ID:        id,
			Name:      name,
			CreatedAt: now,
			UpdatedAt: now,
		},
		NormalizedName: normalizedName,
	}
	change, err := NewTagSyncChange(id, record)
	if err != nil {
		return result, err
	}
	if err := s.repository.CreateTagWithSync(ctx, record, []SyncChange{change}); err != nil {
		return result, fmt.Errorf("create tag: %w", err)
	}

	result.Tag = &record.Tag
	return result, nil
}

func (s *Service) UpdateTag(ctx context.Context, id string, input TagUpdateInput) (TagMutationResult, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	result := TagMutationResult{}
	if err := s.recoverPendingLocked(ctx); err != nil {
		return result, err
	}

	record, err := s.repository.GetTag(ctx, id)
	if errors.Is(err, ErrTagNotFound) {
		result.Error = tagError(TagErrorNotFound, "タグが見つかりません。", "tagId")
		return result, nil
	}
	if err != nil {
		return result, err
	}

	name, normalizedName, validationError := normalizeTagName(input.Name)
	if validationError != nil {
		result.Error = validationError
		return result, nil
	}
	existing, err := s.repository.GetTagByNormalizedName(ctx, normalizedName)
	if err == nil && existing.ID != record.ID {
		result.Error = tagError(TagErrorNameConflict, "同じ名前のタグが既に存在します。", "name")
		return result, nil
	}
	if err != nil && !errors.Is(err, ErrTagNotFound) {
		return result, err
	}

	record.Name = name
	record.NormalizedName = normalizedName
	record.UpdatedAt = time.Now().UTC()
	changeSetID, err := newID()
	if err != nil {
		return result, err
	}
	change, err := NewTagSyncChange(changeSetID, record)
	if err != nil {
		return result, err
	}
	if err := s.repository.UpdateTagWithSync(ctx, record, []SyncChange{change}); err != nil {
		return result, fmt.Errorf("update tag: %w", err)
	}

	result.Tag = &record.Tag
	return result, nil
}

func (s *Service) DeleteTag(ctx context.Context, id string) (TagDeleteResult, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	result := TagDeleteResult{}
	if err := s.recoverPendingLocked(ctx); err != nil {
		return result, err
	}

	noteIDs, err := s.repository.ListNoteIDsForTag(ctx, id)
	if err != nil {
		return result, err
	}
	changeSetID, err := newID()
	if err != nil {
		return result, err
	}
	changes := []SyncChange{NewTagTombstoneChange(changeSetID, id)}
	for _, noteID := range noteIDs {
		tagIDs, listErr := s.repository.ListNoteTagIDs(ctx, noteID)
		if listErr != nil {
			return result, listErr
		}
		remainingTagIDs := make([]string, 0, len(tagIDs))
		for _, tagID := range tagIDs {
			if tagID != id {
				remainingTagIDs = append(remainingTagIDs, tagID)
			}
		}
		relationChange, changeErr := NewNoteTagsSyncChange(changeSetID, noteID, remainingTagIDs)
		if changeErr != nil {
			return result, changeErr
		}
		changes = append(changes, relationChange)
	}
	if err := s.repository.DeleteTagWithSync(ctx, id, changes); err != nil {
		if errors.Is(err, ErrTagNotFound) {
			result.Error = tagError(TagErrorNotFound, "タグが見つかりません。", "tagId")
			return result, nil
		}
		return result, err
	}

	result.Deleted = true
	return result, nil
}

func (s *Service) SetNoteTags(ctx context.Context, noteID string, input SetNoteTagsInput) (NoteTagsResult, error) {
	ctx, unlockMutation := s.lockMutation(ctx)
	defer unlockMutation()

	s.mu.Lock()
	defer s.mu.Unlock()
	result := NoteTagsResult{Tags: make([]Tag, 0)}
	if err := s.recoverPendingLocked(ctx); err != nil {
		return result, err
	}

	tagIDs := deduplicateTagIDs(input.TagIDs)
	changeSetID, err := newID()
	if err != nil {
		return result, err
	}
	change, err := NewNoteTagsSyncChange(changeSetID, noteID, tagIDs)
	if err != nil {
		return result, err
	}
	if err := s.repository.ReplaceNoteTagsWithSync(ctx, noteID, tagIDs, []SyncChange{change}); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			result.Error = tagError(TagErrorNoteNotFound, "ノートが見つかりません。", "noteId")
			return result, nil
		case errors.Is(err, ErrTagNotFound):
			result.Error = tagError(TagErrorNotFound, "タグが見つかりません。", "tagIds")
			return result, nil
		default:
			return result, err
		}
	}

	tags, err := s.repository.ListNoteTags(ctx, noteID)
	if err != nil {
		return result, err
	}
	result.Tags = tags
	return result, nil
}

func normalizeTagName(value string) (string, string, *TagError) {
	for _, runeValue := range value {
		if unicode.IsControl(runeValue) {
			return "", "", tagError(TagErrorNameInvalid, "タグ名に制御文字は使用できません。", "name")
		}
	}

	var builder strings.Builder
	normalizedValue := norm.NFC.String(value)
	previousWasSpace := true
	for _, runeValue := range normalizedValue {
		if unicode.IsSpace(runeValue) {
			if !previousWasSpace {
				builder.WriteByte(' ')
				previousWasSpace = true
			}
			continue
		}
		builder.WriteRune(runeValue)
		previousWasSpace = false
	}

	name := strings.TrimSpace(builder.String())
	if name == "" {
		return "", "", tagError(TagErrorNameEmpty, "タグ名を入力してください。", "name")
	}
	if len([]rune(name)) > MaxTagNameLength {
		return "", "", tagError(TagErrorNameTooLong, "タグ名は100文字以内で入力してください。", "name")
	}

	return name, norm.NFC.String(tagCaseFolder.String(name)), nil
}

func deduplicateTagIDs(tagIDs []string) []string {
	unique := make(map[string]struct{}, len(tagIDs))
	result := make([]string, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if _, exists := unique[tagID]; exists {
			continue
		}
		unique[tagID] = struct{}{}
		result = append(result, tagID)
	}

	return result
}

func tagError(code string, message string, field string) *TagError {
	return &TagError{
		Code:      code,
		Message:   message,
		Field:     field,
		Retryable: false,
	}
}
