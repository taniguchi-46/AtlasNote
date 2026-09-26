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
	"time"

	"atlasnote/internal/note"
	"atlasnote/internal/organize"
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
	organizer     *organize.Service
	activeSpaceID string
}

func New(notes noteReader, locks accessGuard, activeSpaceID string, organizer ...*organize.Service) *Service {
	service := &Service{notes: notes, locks: locks, activeSpaceID: strings.TrimSpace(activeSpaceID)}
	if len(organizer) > 0 {
		service.organizer = organizer[0]
	}
	return service
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
	required, ok := operationPermissions(request.Operation)
	if !ok {
		return failure(request.RequestID, StatusRejected, "OPERATION_NOT_ALLOWED", "この読み取り操作は公開されていません。", false)
	}
	for _, permission := range required {
		if !principal.Permissions[permission] {
			return failure(request.RequestID, StatusRejected, "PERMISSION_DENIED", "この読み取り権限は許可されていません。", false)
		}
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
	case OperationOrganizeAnalyze:
		response = s.analyzeOrganization(ctx, principal, request)
	case OperationOrganizeGetCandidates:
		response = s.getOrganizationCandidates(ctx, principal, request)
	}
	return response
}

func operationPermissions(operation string) ([]string, bool) {
	switch operation {
	case OperationNotesList, OperationNotebooksList, OperationTagsList:
		return []string{PermissionMetadata}, true
	case OperationNotesGet, OperationNotesSearch, OperationBacklinks, OperationRelated:
		return []string{PermissionContent}, true
	case OperationOrganizeAnalyze, OperationOrganizeGetCandidates:
		return []string{PermissionProposal, PermissionContent}, true
	default:
		return nil, false
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

const (
	maxOrganizationCandidateBytes     = 256 << 10
	maxOrganizationCandidatePageBytes = 512 << 10
)

func (s *Service) analyzeOrganization(ctx context.Context, principal Principal, request Request) Response {
	var input OrganizeAnalyzeInput
	if err := decodeParams(request.Params, &input); err != nil || s.organizer == nil {
		return invalidParams(request.RequestID)
	}
	if input.Scope == "" {
		input.Scope = "space"
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil || !validOrganizationScope(input) {
		return invalidParams(request.RequestID)
	}
	if !s.organizationScopeAllowed(ctx, principal, input) {
		return resourceUnavailable(request.RequestID)
	}
	access := organizationAccess(principal)
	analysis, err := s.organizer.AnalyzeExternal(ctx, s.activeSpaceID, organize.AnalysisInput{
		Scope: input.Scope, NotebookID: input.NotebookID, NoteID: input.NoteID, RequestID: request.RequestID,
	}, access)
	if err != nil {
		return organizationServiceError(request.RequestID, err)
	}
	snapshot, err := s.organizer.AnalysisSnapshot(s.activeSpaceID, principal.ClientID, analysis.SessionID, access)
	if err != nil || !s.organizationSnapshotAllowed(ctx, principal, snapshot) {
		return analysisUnavailable(request.RequestID)
	}
	candidates := organizationCandidatesForPrincipal(principal, snapshot)
	page, next := pageOrganizationCandidates(candidates, 0, limit, OperationOrganizeGetCandidates, organizationCursorFingerprint(analysis.SessionID, ""))
	return success(request.RequestID, OrganizeAnalyzeData{
		AnalysisID: analysis.SessionID,
		Summary: OrganizationSummary{
			Scope: analysis.Scope, NotebookID: analysis.NotebookID, NoteID: analysis.NoteID,
			StartedAt: analysis.StartedAt, ExpiresAt: snapshot.ExpiresAt, AnalyzedNotes: analysis.AnalyzedNotes,
			SkippedLocked: analysis.SkippedLocked, SkippedTrash: analysis.SkippedTrash, CandidateCount: len(candidates),
		},
		Candidates: page, NextCursor: next,
	})
}

func (s *Service) getOrganizationCandidates(ctx context.Context, principal Principal, request Request) Response {
	var input OrganizeCandidatesInput
	if err := decodeParams(request.Params, &input); err != nil || s.organizer == nil || !validID(input.AnalysisID) || !validOrganizationKind(input.Kind) {
		return invalidParams(request.RequestID)
	}
	limit, err := normalizeLimit(input.Limit, maxLimit)
	if err != nil {
		return invalidParams(request.RequestID)
	}
	fingerprint := organizationCursorFingerprint(input.AnalysisID, input.Kind)
	offset, err := decodeCursor(input.Cursor, OperationOrganizeGetCandidates, fingerprint)
	if err != nil {
		return cursorError(request.RequestID)
	}
	access := organizationAccess(principal)
	snapshot, err := s.organizer.AnalysisSnapshot(s.activeSpaceID, principal.ClientID, input.AnalysisID, access)
	if err != nil || !s.organizationSnapshotAllowed(ctx, principal, snapshot) {
		return analysisUnavailable(request.RequestID)
	}
	candidates := organizationCandidatesForPrincipal(principal, snapshot)
	if input.Kind != "" {
		filtered := make([]OrganizationCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.Kind == input.Kind {
				filtered = append(filtered, candidate)
			}
		}
		candidates = filtered
	}
	if offset > len(candidates) {
		return cursorError(request.RequestID)
	}
	page, next := pageOrganizationCandidates(candidates, offset, limit, OperationOrganizeGetCandidates, fingerprint)
	return success(request.RequestID, OrganizeCandidatesData{
		AnalysisID: input.AnalysisID, Candidates: page, NextCursor: next, ExpiresAt: snapshot.ExpiresAt,
	})
}

func validOrganizationScope(input OrganizeAnalyzeInput) bool {
	switch input.Scope {
	case "space":
		return input.NotebookID == "" && input.NoteID == ""
	case "notebook", "descendants":
		return validID(input.NotebookID) && input.NoteID == ""
	case "note":
		return validID(input.NoteID) && input.NotebookID == ""
	default:
		return false
	}
}

func validOrganizationKind(kind string) bool {
	if kind == "" {
		return true
	}
	switch kind {
	case organize.KindNotebookAssignment, organize.KindNotebookMove, organize.KindUnclassifiedNote,
		organize.KindTagAssignment, organize.KindDuplicateNote, organize.KindEmptyNote, organize.KindBrokenLink,
		organize.KindOrphanNote, organize.KindRelatedNote, organize.KindReciprocalLink, organize.KindTitle,
		organize.KindDuplicateTag:
		return true
	default:
		return false
	}
}

func (s *Service) organizationScopeAllowed(ctx context.Context, principal Principal, input OrganizeAnalyzeInput) bool {
	switch input.Scope {
	case "space":
		if !principal.ScopeRestricted {
			return true
		}
		if len(principal.AllowedNoteIDs)+len(principal.AllowedNotebookIDs) == 0 {
			return false
		}
		for id := range principal.AllowedNoteIDs {
			allowed, err := s.noteProtectionAllowed(ctx, id)
			if err != nil || !allowed {
				return false
			}
			item, err := s.notes.Get(ctx, id)
			if err != nil || item.IsTrashed {
				return false
			}
		}
		for id := range principal.AllowedNotebookIDs {
			allowed, err := s.notebookAllowed(ctx, principal, id)
			if err != nil || !allowed {
				return false
			}
		}
		return true
	case "notebook", "descendants":
		allowed, err := s.notebookAllowed(ctx, principal, input.NotebookID)
		return err == nil && allowed
	case "note":
		protected, err := s.noteProtectionAllowed(ctx, input.NoteID)
		if err != nil || !protected {
			return false
		}
		item, err := s.notes.Get(ctx, input.NoteID)
		if err != nil || item.IsTrashed {
			return false
		}
		allowed, err := s.noteAllowed(ctx, principal, item.ID, item.NotebookID)
		return err == nil && allowed
	default:
		return false
	}
}

func organizationAccess(principal Principal) organize.AnalysisAccess {
	return organize.AnalysisAccess{
		OwnerID: principal.ClientID, ScopeRestricted: principal.ScopeRestricted,
		AllowedNoteIDs:     clonePrincipalScope(principal.AllowedNoteIDs),
		AllowedNotebookIDs: clonePrincipalScope(principal.AllowedNotebookIDs),
	}
}

func clonePrincipalScope(values map[string]bool) map[string]bool {
	result := make(map[string]bool, len(values))
	for id, allowed := range values {
		if allowed {
			result[id] = true
		}
	}
	return result
}

func (s *Service) organizationSnapshotAllowed(ctx context.Context, principal Principal, snapshot organize.AnalysisSnapshot) bool {
	if snapshot.Analysis.SpaceID != s.activeSpaceID || time.Now().UTC().Before(snapshot.Analysis.StartedAt) {
		return false
	}
	if snapshot.Analysis.NotebookID != "" {
		allowed, err := s.notebookAllowed(ctx, principal, snapshot.Analysis.NotebookID)
		if err != nil || !allowed {
			return false
		}
	}
	for _, id := range snapshot.AnalyzedNoteIDs {
		protected, err := s.noteProtectionAllowed(ctx, id)
		if err != nil || !protected {
			return false
		}
		item, err := s.notes.Get(ctx, id)
		if err != nil || item.IsTrashed || item.Revision != snapshot.AnalyzedNoteRevisions[id] {
			return false
		}
		allowed, err := s.noteAllowed(ctx, principal, item.ID, item.NotebookID)
		if err != nil || !allowed {
			return false
		}
		noteTags, tagErr := s.notes.ListNoteTags(ctx, id)
		if tagErr != nil || noteTags.Error != nil {
			return false
		}
		currentTagIDs := make([]string, 0, len(noteTags.Tags))
		for _, tag := range noteTags.Tags {
			currentTagIDs = append(currentTagIDs, tag.ID)
		}
		sort.Strings(currentTagIDs)
		if !equalOrganizationStrings(currentTagIDs, snapshot.AnalyzedNoteTagIDs[id]) {
			return false
		}
	}
	tags, err := s.notes.ListTags(ctx)
	if err != nil {
		return false
	}
	existingTags := make(map[string]bool, len(tags))
	for _, item := range tags {
		existingTags[item.ID] = true
	}
	scopedTags := make(map[string]bool)
	for _, tagIDs := range snapshot.AnalyzedNoteTagIDs {
		for _, id := range tagIDs {
			scopedTags[id] = true
		}
	}
	validatedNotebooks := make(map[string]bool)
	for _, candidate := range snapshot.Candidates {
		if candidate.SpaceID != s.activeSpaceID || candidate.TagID != "" && (!existingTags[candidate.TagID] || principal.ScopeRestricted && !scopedTags[candidate.TagID]) {
			return false
		}
		if candidate.NoteID != "" {
			revision, exists := snapshot.AnalyzedNoteRevisions[candidate.NoteID]
			if !exists || candidate.BaseRevision > 0 && revision != candidate.BaseRevision {
				return false
			}
		}
		if candidate.RelatedID != "" && (candidate.RelatedTitle != "" || candidate.RelatedRevision > 0) {
			revision, exists := snapshot.AnalyzedNoteRevisions[candidate.RelatedID]
			if !exists || candidate.RelatedRevision > 0 && revision != candidate.RelatedRevision {
				return false
			}
		}
		if candidate.NotebookID != "" && !validatedNotebooks[candidate.NotebookID] {
			allowed, notebookErr := s.notebookAllowed(ctx, principal, candidate.NotebookID)
			if notebookErr != nil || !allowed {
				return false
			}
			validatedNotebooks[candidate.NotebookID] = true
		}
	}
	return true
}

func equalOrganizationStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func organizationCandidatesForPrincipal(principal Principal, snapshot organize.AnalysisSnapshot) []OrganizationCandidate {
	result := make([]OrganizationCandidate, 0, len(snapshot.Candidates))
	scopedTags := make(map[string]bool)
	for _, tagIDs := range snapshot.AnalyzedNoteTagIDs {
		for _, id := range tagIDs {
			scopedTags[id] = true
		}
	}
	for _, candidate := range snapshot.Candidates {
		relatedID := candidate.RelatedID
		if principal.ScopeRestricted && relatedID != "" && candidate.RelatedTitle == "" && candidate.RelatedRevision == 0 {
			relatedID = ""
		}
		external := OrganizationCandidate{
			ID: candidate.ID, Kind: candidate.Kind, NoteID: candidate.NoteID, NoteTitle: candidate.NoteTitle,
			RelatedID: relatedID, RelatedTitle: candidate.RelatedTitle, NotebookID: candidate.NotebookID, TagID: candidate.TagID,
			Reason: candidate.Reason, Before: sanitizeOrganizationMap(candidate.Before, principal, scopedTags),
			Proposed: sanitizeOrganizationMap(candidate.Proposed, principal, scopedTags), Applicable: candidate.Applicable,
			BaseRevision: candidate.BaseRevision, RelatedRevision: candidate.RelatedRevision,
		}
		encoded, _ := json.Marshal(external)
		if len(encoded) > maxOrganizationCandidateBytes {
			continue
		}
		result = append(result, external)
	}
	return result
}

func sanitizeOrganizationMap(value map[string]any, principal Principal, scopedTags map[string]bool) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		if !principal.ScopeRestricted {
			result[key] = item
			continue
		}
		switch key {
		case "notebookId":
			id, _ := item.(string)
			if id == "" || principal.AllowedNotebookIDs[id] {
				result[key] = item
			}
		case "tagId":
			id, _ := item.(string)
			if id == "" || scopedTags[id] {
				result[key] = item
			}
		case "tagIds":
			if ids, ok := item.([]string); ok {
				filtered := make([]string, 0, len(ids))
				for _, id := range ids {
					if scopedTags[id] {
						filtered = append(filtered, id)
					}
				}
				result[key] = filtered
			}
		case "relatedId", "targetId":
			id, _ := item.(string)
			if principal.AllowedNoteIDs[id] {
				result[key] = item
			}
		default:
			result[key] = item
		}
	}
	return result
}

func organizationCursorFingerprint(analysisID, kind string) string {
	return cursorFingerprint(OperationOrganizeGetCandidates, struct {
		AnalysisID string `json:"analysisId"`
		Kind       string `json:"kind,omitempty"`
	}{analysisID, kind})
}

func pageOrganizationCandidates(items []OrganizationCandidate, offset, limit int, operation, fingerprint string) ([]OrganizationCandidate, string) {
	page := make([]OrganizationCandidate, 0, limit)
	encodedBytes := 0
	end := offset
	for end < len(items) && len(page) < limit {
		encoded, _ := json.Marshal(items[end])
		if len(page) > 0 && encodedBytes+len(encoded) > maxOrganizationCandidatePageBytes {
			break
		}
		page = append(page, items[end])
		encodedBytes += len(encoded)
		end++
	}
	if end < len(items) {
		return page, encodeCursor(operation, fingerprint, end)
	}
	return page, ""
}

func analysisUnavailable(requestID string) Response {
	return failure(requestID, StatusRejected, "ANALYSIS_UNAVAILABLE", "解析結果を取得できません。再解析してください。", false)
}

func organizationServiceError(requestID string, err error) Response {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return failure(requestID, StatusError, "ANALYSIS_CANCELLED", "整理解析は完了しませんでした。", true)
	case errors.Is(err, note.ErrValidation):
		return invalidParams(requestID)
	case errors.Is(err, note.ErrNotFound), errors.Is(err, organize.ErrAnalysisUnavailable):
		return analysisUnavailable(requestID)
	default:
		return failure(requestID, StatusError, "ANALYSIS_UNAVAILABLE", "整理解析を完了できませんでした。", true)
	}
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
