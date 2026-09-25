package readapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"atlasnote/internal/note"
)

const (
	defaultLimit = 30
	maxLimit     = 100
)

type noteReader interface {
	BeginExternalRead(context.Context) (context.Context, func())
	ListPage(context.Context, note.NoteListInput) (note.NoteListResult, error)
	Get(context.Context, string) (note.Note, error)
	Search(context.Context, note.SearchInput) (note.SearchResult, error)
	ListBacklinks(context.Context, note.BacklinkListInput) (note.BacklinkListResult, error)
	RelatedNotes(context.Context, note.RelatedNoteInput) (note.RelatedNoteResult, error)
	ListNotebooks(context.Context) ([]note.Notebook, error)
	ListTags(context.Context) ([]note.Tag, error)
	ListNoteTags(context.Context, string) (note.NoteTagsResult, error)
	ValidateSearchIndexSnapshot(context.Context, string, int64) error
	ValidateLinkIndexSnapshot(context.Context, string, int64) error
}

type accessGuard interface {
	NoteLockStatus(context.Context, string) (bool, bool, string, error)
	NotebookLockStatus(context.Context, string) (bool, bool, string, error)
}

type Service struct {
	notes         noteReader
	locks         accessGuard
	activeSpaceID string
}

func New(notes noteReader, locks accessGuard, activeSpaceID string) *Service {
	return &Service{notes: notes, locks: locks, activeSpaceID: strings.TrimSpace(activeSpaceID)}
}

func (s *Service) Execute(ctx context.Context, principal Principal, request Request) Response {
	if request.APIVersion != APIVersion {
		return failure(request.RequestID, StatusRejected, "API_VERSION_UNSUPPORTED", "対応していないAPIバージョンです。", false)
	}
	if strings.TrimSpace(request.RequestID) == "" || len(request.RequestID) > 128 {
		return failure(request.RequestID, StatusRejected, "REQUEST_ID_INVALID", "requestIdが正しくありません。", false)
	}
	if principal.ClientID == "" || request.ClientID != principal.ClientID {
		return failure(request.RequestID, StatusRejected, "CLIENT_ID_INVALID", "クライアントを確認できませんでした。", false)
	}
	if s.activeSpaceID == "" || principal.StorageSpaceID != s.activeSpaceID || request.Scope.StorageSpaceID != s.activeSpaceID {
		return failure(request.RequestID, StatusRejected, "SCOPE_MISMATCH", "要求された保存空間にはアクセスできません。", false)
	}
	required, ok := operationPermission(request.Operation)
	if !ok {
		return failure(request.RequestID, StatusRejected, "OPERATION_NOT_ALLOWED", "この読み取り操作は公開されていません。", false)
	}
	if !principal.Permissions[required] {
		return failure(request.RequestID, StatusRejected, "PERMISSION_DENIED", "この読み取り権限は許可されていません。", false)
	}
	ctx, releaseContent := s.notes.BeginExternalRead(ctx)
	defer releaseContent()

	var response Response
	switch request.Operation {
	case OperationNotesList:
		response = s.listNotes(ctx, principal, request)
	case OperationNotesGet:
		response = s.getNote(ctx, principal, request)
	case OperationNotesSearch:
		response = s.searchNotes(ctx, principal, request)
	case OperationNotebooksList:
		response = s.listNotebooks(ctx, principal, request)
	case OperationTagsList:
		response = s.listTags(ctx, principal, request)
	case OperationBacklinks:
		response = s.listBacklinks(ctx, principal, request)
	case OperationRelated:
		response = s.relatedNotes(ctx, principal, request)
	}
	return response
}

func operationPermission(operation string) (string, bool) {
	switch operation {
	case OperationNotesList, OperationNotebooksList, OperationTagsList:
		return PermissionMetadata, true
	case OperationNotesGet, OperationNotesSearch, OperationBacklinks, OperationRelated:
		return PermissionContent, true
	default:
		return "", false
	}
}

func (s *Service) listNotes(ctx context.Context, principal Principal, request Request) Response {
	var input NoteListInput
	if err := decodeParams(request.Params, &input); err != nil {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil || invalidOptionalID(input.NotebookID) || invalidOptionalID(input.TagID) {
		return invalidParams(request.RequestID)
	}
	fingerprint := cursorFingerprint(OperationNotesList, struct {
		NotebookID    *string `json:"notebookId,omitempty"`
		TagID         *string `json:"tagId,omitempty"`
		SortBy        string  `json:"sortBy,omitempty"`
		SortDirection string  `json:"sortDirection,omitempty"`
	}{input.NotebookID, input.TagID, input.SortBy, input.SortDirection})
	offset, err := decodeCursor(input.Cursor, OperationNotesList, fingerprint)
	if err != nil || offset > note.MaxNoteListPage*note.MaxNoteListPageSize {
		return cursorError(request.RequestID)
	}

	items := make([]NoteSummary, 0)
	requiredItems := offset + limit + 1

scanPages:
	for page := 1; page <= note.MaxNoteListPage; page++ {
		result, listErr := s.notes.ListPage(ctx, note.NoteListInput{
			Page: page, PageSize: note.MaxNoteListPageSize, TagID: input.TagID,
			SortBy: input.SortBy, SortDirection: input.SortDirection,
		})
		if listErr != nil {
			return serviceError(request.RequestID, listErr)
		}
		for _, item := range result.Items {
			if item.IsTrashed || (input.NotebookID != nil && !sameOptionalID(item.NotebookID, input.NotebookID)) {
				continue
			}
			allowed, accessErr := s.noteAllowed(ctx, principal, item.ID, item.NotebookID)
			if accessErr != nil {
				return serviceError(request.RequestID, accessErr)
			}
			if !allowed {
				continue
			}
			items = append(items, NoteSummary{ID: item.ID, Title: item.Title, Revision: item.Revision, UpdatedAt: item.UpdatedAt, NotebookID: item.NotebookID})
			if len(items) >= requiredItems {
				break scanPages
			}
		}
		if !result.HasNext {
			break
		}
	}
	paged, next := pageSlice(items, offset, limit, OperationNotesList, fingerprint)
	return success(request.RequestID, NoteListData{Notes: paged, NextCursor: next})
}

func (s *Service) getNote(ctx context.Context, principal Principal, request Request) Response {
	var input NoteGetInput
	if err := decodeParams(request.Params, &input); err != nil || !validID(input.NoteID) || (input.ExpectedRevision != nil && *input.ExpectedRevision < 1) {
		return invalidParams(request.RequestID)
	}
	allowed, err := s.noteProtectionAllowed(ctx, input.NoteID)
	if err != nil {
		return resourceUnavailable(request.RequestID)
	}
	if !allowed {
		return resourceUnavailable(request.RequestID)
	}
	item, err := s.notes.Get(ctx, input.NoteID)
	if err != nil || item.IsTrashed || !principalAllowsNote(principal, item.ID, item.NotebookID) {
		return resourceUnavailable(request.RequestID)
	}
	if input.ExpectedRevision != nil && item.Revision != *input.ExpectedRevision {
		return failure(request.RequestID, StatusConflict, "REVISION_MISMATCH", "ノートのrevisionが一致しません。", false)
	}
	tagResult, err := s.notes.ListNoteTags(ctx, item.ID)
	if err != nil {
		return serviceError(request.RequestID, err)
	}
	if tagResult.Error != nil {
		return resourceUnavailable(request.RequestID)
	}
	tags := make([]Tag, 0, len(tagResult.Tags))
	for _, value := range tagResult.Tags {
		tags = append(tags, Tag{ID: value.ID, Name: value.Name})
	}
	return success(request.RequestID, NoteGetData{Note: Note{
		ID: item.ID, Title: item.Title, Content: item.Content, Revision: item.Revision,
		NotebookID: item.NotebookID, Tags: tags, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}})
}

func (s *Service) searchNotes(ctx context.Context, principal Principal, request Request) Response {
	var input SearchInput
	if err := decodeParams(request.Params, &input); err != nil || invalidOptionalID(input.NotebookID) {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	fingerprint := cursorFingerprint(OperationNotesSearch, struct {
		Query         string  `json:"query"`
		Scope         string  `json:"scope,omitempty"`
		NotebookID    *string `json:"notebookId,omitempty"`
		SortBy        string  `json:"sortBy,omitempty"`
		SortDirection string  `json:"sortDirection,omitempty"`
	}{input.Query, input.Scope, input.NotebookID, input.SortBy, input.SortDirection})
	offset, err := decodeCursor(input.Cursor, OperationNotesSearch, fingerprint)
	if err != nil {
		return cursorError(request.RequestID)
	}

	items := make([]SearchMatch, 0)
	for page := 1; page <= note.MaxSearchPage; page++ {
		result, searchErr := s.notes.Search(ctx, note.SearchInput{
			Query: input.Query, Scope: input.Scope, NotebookID: input.NotebookID,
			IncludeTrashed: false, Page: page, PageSize: note.MaxSearchPageSize,
			SortBy: input.SortBy, SortDirection: input.SortDirection,
		})
		if searchErr != nil {
			return serviceError(request.RequestID, searchErr)
		}
		if result.Error != nil {
			return failure(request.RequestID, StatusError, result.Error.Code, result.Error.Message, result.Error.Retryable)
		}
		for _, item := range result.Items {
			if item.Note.IsTrashed {
				continue
			}
			allowed, accessErr := s.noteAllowed(ctx, principal, item.Note.ID, item.Note.NotebookID)
			if accessErr != nil {
				return serviceError(request.RequestID, accessErr)
			}
			if !allowed {
				continue
			}
			if input.Scope == "" || input.Scope == note.SearchScopeAll {
				if err := s.notes.ValidateSearchIndexSnapshot(ctx, item.Note.ID, item.Note.Revision); err != nil {
					return indexError(request.RequestID)
				}
			}
			items = append(items, SearchMatch{
				NoteID: item.Note.ID, Title: item.Note.Title, Snippet: item.Snippet,
				Revision: item.Note.Revision, MatchScope: item.MatchScope,
			})
		}
		if !result.HasNext {
			break
		}
	}
	paged, next := pageSlice(items, offset, limit, OperationNotesSearch, fingerprint)
	return success(request.RequestID, SearchData{Matches: paged, NextCursor: next})
}

func (s *Service) listNotebooks(ctx context.Context, principal Principal, request Request) Response {
	var input NotebookListInput
	if err := decodeParams(request.Params, &input); err != nil || invalidOptionalID(input.ParentID) {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	fingerprint := cursorFingerprint(OperationNotebooksList, struct {
		ParentID           *string `json:"parentId,omitempty"`
		IncludeDescendants bool    `json:"includeDescendants"`
	}{input.ParentID, input.IncludeDescendants})
	offset, err := decodeCursor(input.Cursor, OperationNotebooksList, fingerprint)
	if err != nil {
		return cursorError(request.RequestID)
	}
	all, err := s.notes.ListNotebooks(ctx)
	if err != nil {
		return serviceError(request.RequestID, err)
	}
	byID := make(map[string]note.Notebook, len(all))
	children := make(map[string][]string)
	for _, item := range all {
		byID[item.ID] = item
		if item.ParentID != nil {
			children[*item.ParentID] = append(children[*item.ParentID], item.ID)
		}
	}
	if input.ParentID != nil {
		if _, exists := byID[*input.ParentID]; !exists {
			return resourceUnavailable(request.RequestID)
		}
		allowed, accessErr := s.notebookAllowed(ctx, principal, *input.ParentID)
		if accessErr != nil || !allowed {
			return resourceUnavailable(request.RequestID)
		}
	}
	selected := make(map[string]bool)
	if input.ParentID == nil {
		for id := range byID {
			selected[id] = true
		}
	} else if input.IncludeDescendants {
		queue := append([]string(nil), children[*input.ParentID]...)
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if selected[id] {
				continue
			}
			selected[id] = true
			queue = append(queue, children[id]...)
		}
	} else {
		for _, id := range children[*input.ParentID] {
			selected[id] = true
		}
	}
	items := make([]Notebook, 0, len(selected))
	for id := range selected {
		item := byID[id]
		allowed, accessErr := s.notebookAllowed(ctx, principal, id)
		if accessErr != nil {
			return serviceError(request.RequestID, accessErr)
		}
		if allowed {
			items = append(items, Notebook{ID: id, Name: item.Name, ParentID: item.ParentID})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	paged, next := pageSlice(items, offset, limit, OperationNotebooksList, fingerprint)
	return success(request.RequestID, NotebookListData{Notebooks: paged, NextCursor: next})
}

func (s *Service) listTags(ctx context.Context, principal Principal, request Request) Response {
	var input TagListInput
	if err := decodeParams(request.Params, &input); err != nil {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	fingerprint := cursorFingerprint(OperationTagsList, struct{}{})
	offset, err := decodeCursor(input.Cursor, OperationTagsList, fingerprint)
	if err != nil {
		return cursorError(request.RequestID)
	}
	listed, err := s.notes.ListTags(ctx)
	if err != nil {
		return serviceError(request.RequestID, err)
	}
	items := make([]Tag, 0, len(listed))
	var scopedTagIDs map[string]bool
	if principal.ScopeRestricted {
		scopedTagIDs, err = s.scopedTagIDs(ctx, principal)
		if err != nil {
			return serviceError(request.RequestID, err)
		}
	}
	for _, item := range listed {
		if principal.ScopeRestricted && !scopedTagIDs[item.ID] {
			continue
		}
		items = append(items, Tag{ID: item.ID, Name: item.Name})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	paged, next := pageSlice(items, offset, limit, OperationTagsList, fingerprint)
	return success(request.RequestID, TagListData{Tags: paged, NextCursor: next})
}

func (s *Service) listBacklinks(ctx context.Context, principal Principal, request Request) Response {
	var input BacklinkInput
	if err := decodeParams(request.Params, &input); err != nil || !validID(input.NoteID) {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	allowed, err := s.noteProtectionAllowed(ctx, input.NoteID)
	if err != nil || !allowed {
		return resourceUnavailable(request.RequestID)
	}
	target, err := s.notes.Get(ctx, input.NoteID)
	if err != nil || target.IsTrashed || !principalAllowsNote(principal, target.ID, target.NotebookID) {
		return resourceUnavailable(request.RequestID)
	}
	fingerprint := cursorFingerprint(OperationBacklinks, struct {
		NoteID string `json:"noteId"`
	}{input.NoteID})
	offset, err := decodeCursor(input.Cursor, OperationBacklinks, fingerprint)
	if err != nil {
		return cursorError(request.RequestID)
	}
	items := make([]NoteSummary, 0)
	for page := 1; page <= note.MaxBacklinkPage; page++ {
		result, listErr := s.notes.ListBacklinks(ctx, note.BacklinkListInput{NoteID: input.NoteID, Page: page, PageSize: note.MaxBacklinkPageSize})
		if listErr != nil {
			if errors.Is(listErr, note.ErrBacklinkIndexFailed) {
				return indexError(request.RequestID)
			}
			return serviceError(request.RequestID, listErr)
		}
		for _, item := range result.Items {
			allowed, accessErr := s.noteAllowed(ctx, principal, item.ID, item.NotebookID)
			if accessErr != nil {
				return serviceError(request.RequestID, accessErr)
			}
			if !allowed || item.IsTrashed {
				continue
			}
			if err := s.notes.ValidateLinkIndexSnapshot(ctx, item.ID, item.Revision); err != nil {
				return indexError(request.RequestID)
			}
			items = append(items, NoteSummary{ID: item.ID, Title: item.Title, Revision: item.Revision, UpdatedAt: item.UpdatedAt, NotebookID: item.NotebookID})
		}
		if !result.HasNext {
			break
		}
	}
	paged, next := pageSlice(items, offset, limit, OperationBacklinks, fingerprint)
	return success(request.RequestID, BacklinkData{Items: paged, NextCursor: next})
}

func (s *Service) relatedNotes(ctx context.Context, principal Principal, request Request) Response {
	var input RelatedInput
	if err := decodeParams(request.Params, &input); err != nil || !validID(input.NoteID) || invalidOptionalID(input.NotebookID) {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, 20)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	allowed, err := s.noteProtectionAllowed(ctx, input.NoteID)
	if err != nil || !allowed {
		return resourceUnavailable(request.RequestID)
	}
	target, err := s.notes.Get(ctx, input.NoteID)
	if err != nil || target.IsTrashed || !principalAllowsNote(principal, target.ID, target.NotebookID) {
		return resourceUnavailable(request.RequestID)
	}
	result, err := s.notes.RelatedNotes(ctx, note.RelatedNoteInput{
		NoteID: input.NoteID, NotebookID: input.NotebookID, Descendants: input.Descendants, Limit: limit,
	})
	if err != nil {
		if errors.Is(err, note.ErrIndexInconsistent) {
			return indexError(request.RequestID)
		}
		return serviceError(request.RequestID, err)
	}
	items := make([]RelatedItem, 0, len(result.Items))
	for _, item := range result.Items {
		candidate, candidateErr := s.notes.Get(ctx, item.NoteID)
		if candidateErr != nil {
			return serviceError(request.RequestID, candidateErr)
		}
		allowed, accessErr := s.noteAllowed(ctx, principal, candidate.ID, candidate.NotebookID)
		if accessErr != nil {
			return serviceError(request.RequestID, accessErr)
		}
		if !allowed || candidate.IsTrashed {
			continue
		}
		if candidate.Revision != item.Revision {
			return indexError(request.RequestID)
		}
		items = append(items, RelatedItem{
			NoteID: item.NoteID, Title: item.Title, Revision: item.Revision,
			Snippet: item.Snippet, Reasons: append([]string(nil), item.Reasons...),
		})
	}
	return success(request.RequestID, RelatedData{Items: items})
}

func (s *Service) noteAllowed(ctx context.Context, principal Principal, id string, notebookID *string) (bool, error) {
	allowed, err := s.noteProtectionAllowed(ctx, id)
	if err != nil || !allowed {
		return allowed, err
	}
	return principalAllowsNote(principal, id, notebookID), nil
}

func (s *Service) noteProtectionAllowed(ctx context.Context, id string) (bool, error) {
	if s.locks == nil {
		return false, errors.New("content lock status is unavailable")
	}
	protected, locked, _, err := s.locks.NoteLockStatus(ctx, id)
	if err != nil {
		return false, err
	}
	return !protected && !locked, nil
}

func (s *Service) notebookAllowed(ctx context.Context, principal Principal, id string) (bool, error) {
	if s.locks == nil {
		return false, errors.New("content lock status is unavailable")
	}
	protected, locked, _, err := s.locks.NotebookLockStatus(ctx, id)
	if err != nil {
		return false, err
	}
	if protected || locked {
		return false, nil
	}
	return !principal.ScopeRestricted || principal.AllowedNotebookIDs[id], nil
}

func principalAllowsNote(principal Principal, id string, notebookID *string) bool {
	if !principal.ScopeRestricted {
		return true
	}
	if principal.AllowedNoteIDs[id] {
		return true
	}
	return notebookID != nil && principal.AllowedNotebookIDs[*notebookID]
}

func (s *Service) scopedTagIDs(ctx context.Context, principal Principal) (map[string]bool, error) {
	allowedTags := make(map[string]bool)
	for page := 1; page <= note.MaxNoteListPage; page++ {
		result, err := s.notes.ListPage(ctx, note.NoteListInput{Page: page, PageSize: note.MaxNoteListPageSize})
		if err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			if item.IsTrashed {
				continue
			}
			allowed, accessErr := s.noteAllowed(ctx, principal, item.ID, item.NotebookID)
			if accessErr != nil {
				return nil, accessErr
			}
			if !allowed {
				continue
			}
			tags, tagErr := s.notes.ListNoteTags(ctx, item.ID)
			if tagErr != nil {
				return nil, tagErr
			}
			if tags.Error != nil {
				continue
			}
			for _, tag := range tags.Tags {
				allowedTags[tag.ID] = true
			}
		}
		if !result.HasNext {
			break
		}
	}
	return allowedTags, nil
}

func decodeParams(raw json.RawMessage, destination any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing params")
	}
	return nil
}

func normalizeLimit(value, maximum int) (int, error) {
	if value == 0 {
		value = min(defaultLimit, maximum)
	}
	if value < 1 || value > maximum {
		return 0, errors.New("limit is outside the allowed range")
	}
	return value, nil
}

func validID(value string) bool {
	if len(value) != 32 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func invalidOptionalID(value *string) bool {
	return value != nil && !validID(strings.TrimSpace(*value))
}

func sameOptionalID(left, right *string) bool {
	return left != nil && right != nil && *left == *right
}

type cursorState struct {
	Operation   string `json:"operation"`
	Fingerprint string `json:"fingerprint"`
	Offset      int    `json:"offset"`
}

func cursorFingerprint(operation string, input any) string {
	encoded, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte(operation+"\x00"), encoded...))
	return hex.EncodeToString(hash[:16])
}

func decodeCursor(value, operation, fingerprint string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) > 512 {
		return 0, errors.New("invalid cursor")
	}
	var state cursorState
	if err := json.Unmarshal(decoded, &state); err != nil || state.Operation != operation || state.Fingerprint != fingerprint || state.Offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return state.Offset, nil
}

func encodeCursor(operation, fingerprint string, offset int) string {
	encoded, _ := json.Marshal(cursorState{Operation: operation, Fingerprint: fingerprint, Offset: offset})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func pageSlice[T any](items []T, offset, limit int, operation, fingerprint string) ([]T, string) {
	if offset > len(items) {
		offset = len(items)
	}
	end := min(offset+limit, len(items))
	page := append([]T(nil), items[offset:end]...)
	if page == nil {
		page = make([]T, 0)
	}
	if end < len(items) {
		return page, encodeCursor(operation, fingerprint, end)
	}
	return page, ""
}

func success(requestID string, data any) Response {
	return Response{APIVersion: APIVersion, RequestID: requestID, Status: StatusOK, Data: data, Error: nil}
}

func failure(requestID, status, code, message string, retryable bool) Response {
	return Response{
		APIVersion: APIVersion, RequestID: requestID, Status: status,
		Data:  map[string]any{},
		Error: &APIError{Code: code, Message: message, Retryable: retryable},
	}
}

func invalidParams(requestID string) Response {
	return failure(requestID, StatusRejected, "INVALID_ARGUMENT", "入力内容が正しくありません。", false)
}

func cursorError(requestID string) Response {
	return failure(requestID, StatusRejected, "CURSOR_INVALID", "cursorが正しくないか、別の要求用です。", false)
}

func resourceUnavailable(requestID string) Response {
	return failure(requestID, StatusRejected, "RESOURCE_UNAVAILABLE", "対象を読み取れません。", false)
}

func indexError(requestID string) Response {
	return failure(requestID, StatusError, "INDEX_INCONSISTENT", "検索用の派生データを検証できませんでした。", true)
}

func serviceError(requestID string, err error) Response {
	switch {
	case errors.Is(err, note.ErrValidation):
		return invalidParams(requestID)
	case errors.Is(err, note.ErrNotFound):
		return resourceUnavailable(requestID)
	case errors.Is(err, note.ErrIndexInconsistent), errors.Is(err, note.ErrBacklinkIndexFailed):
		return indexError(requestID)
	default:
		return failure(requestID, StatusError, "READ_UNAVAILABLE", "読み取り処理を完了できませんでした。", true)
	}
}
