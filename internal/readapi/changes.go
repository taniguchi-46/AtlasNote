package readapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/note"
	"atlasnote/internal/organize"
)

const (
	changeTTL             = 30 * time.Minute
	maxChanges            = 64
	maxChangeContentBytes = 256 << 10
	maxChangeReviewBytes  = 1 << 20
)

type noteWriter interface {
	Create(context.Context, note.CreateInput) (note.Note, error)
	Update(context.Context, string, note.UpdateInput) (note.Note, error)
	SetNoteTagsWithExpectedRevision(context.Context, string, note.SetNoteTagsWithExpectedRevisionInput) (note.NoteTagsResult, error)
}

type changeStore struct {
	applyMu  sync.Mutex
	mu       sync.Mutex
	items    map[string]*pendingChange
	now      func() time.Time
	disabled bool
}

func newChangeStore() *changeStore {
	return &changeStore{items: make(map[string]*pendingChange), now: time.Now}
}

type ChangeReviewItem struct {
	CandidateID string `json:"candidateId,omitempty"`
	NoteID      string `json:"noteId,omitempty"`
	NoteTitle   string `json:"noteTitle,omitempty"`
	Before      any    `json:"before,omitempty"`
	After       any    `json:"after,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ChangeReview is exposed only by the Wails GUI bridge, never by IPC.
type ChangeReview struct {
	OperationID      string             `json:"operationId"`
	Kind             string             `json:"kind"`
	StorageSpaceID   string             `json:"storageSpaceId"`
	TargetIDs        []string           `json:"targetIds"`
	ExpectedRevision int64              `json:"expectedRevision,omitempty"`
	Items            []ChangeReviewItem `json:"items"`
	ImpactCount      int                `json:"impactCount"`
	CreatedAt        time.Time          `json:"createdAt"`
	ExpiresAt        time.Time          `json:"expiresAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
	State            string             `json:"state"`
	Message          string             `json:"message,omitempty"`
}

type ChangeResult struct {
	OperationID string                 `json:"operationId"`
	State       string                 `json:"state"`
	Items       []organize.ApplyResult `json:"items,omitempty"`
	Message     string                 `json:"message,omitempty"`
}

type pendingChange struct {
	review       ChangeReview
	clientID     string
	principal    Principal
	noteID       string
	notebookID   string
	analysisID   string
	candidateIDs []string
	title        *string
	content      *string
	addTags      []string
	removeTags   []string
	baseTags     []string
	resultItems  []organize.ApplyResult
}

type requestApplyInput struct {
	AnalysisID        string           `json:"analysisId"`
	CandidateIDs      []string         `json:"candidateIds"`
	ExpectedRevisions map[string]int64 `json:"expectedRevisions,omitempty"`
}
type proposeEditInput struct {
	NoteID       string `json:"noteId"`
	BaseRevision int64  `json:"baseRevision"`
	Before       string `json:"before"`
	After        string `json:"after"`
	Reason       string `json:"reason"`
}
type requestCreateInput struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	NotebookID string `json:"notebookId"`
}
type requestUpdateInput struct {
	NoteID           string `json:"noteId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Patch            struct {
		Title   *string `json:"title,omitempty"`
		Content *string `json:"content,omitempty"`
	} `json:"patch"`
}
type requestMoveInput struct {
	NoteID           string `json:"noteId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	TargetNotebookID string `json:"targetNotebookId"`
}
type requestTagsInput struct {
	NoteID           string   `json:"noteId"`
	ExpectedRevision int64    `json:"expectedRevision"`
	AddTagIDs        []string `json:"addTagIds"`
	RemoveTagIDs     []string `json:"removeTagIds"`
}
type requestTrashInput struct {
	NoteID           string `json:"noteId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

func newChangeID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *Service) requestChange(ctx context.Context, principal Principal, request Request) Response {
	if s.writes == nil || s.changes == nil {
		return failure(request.RequestID, StatusError, "CHANGE_UNAVAILABLE", "変更要求を受け付けられません。", true)
	}
	now := s.changes.now().UTC()
	change := &pendingChange{clientID: principal.ClientID, principal: cloneChangePrincipal(principal)}
	change.review = ChangeReview{Kind: request.Operation, StorageSpaceID: s.activeSpaceID, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(changeTTL), State: StatusPending}
	switch request.Operation {
	case OperationOrganizeRequestApply:
		var input requestApplyInput
		if decodeParams(request.Params, &input) != nil || s.organizer == nil || !validID(input.AnalysisID) || len(input.CandidateIDs) == 0 || len(input.CandidateIDs) > 100 {
			return invalidParams(request.RequestID)
		}
		snapshot, err := s.organizer.AnalysisSnapshot(s.activeSpaceID, principal.ClientID, input.AnalysisID, organizationAccess(principal))
		if err != nil || !s.organizationSnapshotAllowed(ctx, principal, snapshot) {
			return analysisUnavailable(request.RequestID)
		}
		available := make(map[string]organize.Candidate, len(snapshot.Candidates))
		for _, candidate := range snapshot.Candidates {
			available[candidate.ID] = candidate
		}
		seen := make(map[string]bool)
		for _, id := range input.CandidateIDs {
			candidate, ok := available[id]
			if !validID(id) || seen[id] || !ok || !candidate.Applicable || candidate.NoteID == "" {
				return analysisUnavailable(request.RequestID)
			}
			seen[id] = true
			if input.ExpectedRevisions != nil && input.ExpectedRevisions[candidate.NoteID] != candidate.BaseRevision {
				return failure(request.RequestID, StatusConflict, "REVISION_MISMATCH", "ノートのrevisionが一致しません。", false)
			}
			if candidate.Kind == organize.KindNotebookMove || candidate.Kind == organize.KindNotebookAssignment {
				destination, _ := candidate.Proposed["notebookId"].(string)
				if !s.createDestinationAllowed(ctx, principal, destination) {
					return resourceUnavailable(request.RequestID)
				}
			}
			change.review.TargetIDs = appendUnique(change.review.TargetIDs, candidate.NoteID)
			change.review.Items = append(change.review.Items, ChangeReviewItem{CandidateID: id, NoteID: candidate.NoteID, NoteTitle: candidate.NoteTitle, Before: candidate.Before, After: candidate.Proposed, Reason: candidate.Reason, Status: StatusPending})
		}
		if len(input.ExpectedRevisions) > 0 {
			for id := range input.ExpectedRevisions {
				if !containsString(change.review.TargetIDs, id) {
					return invalidParams(request.RequestID)
				}
			}
		}
		change.analysisID, change.candidateIDs = input.AnalysisID, append([]string(nil), input.CandidateIDs...)
		change.review.ExpiresAt = minTime(change.review.ExpiresAt, snapshot.ExpiresAt)
	case OperationNotesRequestCreate:
		var input requestCreateInput
		if decodeParams(request.Params, &input) != nil || input.NotebookID != "" && !validID(input.NotebookID) || !validChangeText(input.Title, input.Content) {
			return invalidParams(request.RequestID)
		}
		if input.NotebookID == "" && principal.ScopeRestricted || input.NotebookID != "" && !s.createDestinationAllowed(ctx, principal, input.NotebookID) {
			return resourceUnavailable(request.RequestID)
		}
		change.notebookID, change.title, change.content = input.NotebookID, &input.Title, &input.Content
		if input.NotebookID != "" {
			change.review.TargetIDs = []string{input.NotebookID}
		}
		change.review.Items = []ChangeReviewItem{{After: map[string]any{"title": input.Title, "content": input.Content, "notebookId": input.NotebookID}, Reason: "ノートを作成", Status: StatusPending}}
	case OperationNotesProposeEdit, OperationNotesRequestUpdate, OperationNotesRequestMove, OperationNotesRequestTags, OperationNotesRequestTrash:
		response := s.requestExistingNote(ctx, principal, request, change)
		if response.Status != "" {
			return response
		}
	default:
		return invalidParams(request.RequestID)
	}
	change.review.ImpactCount = len(change.review.TargetIDs)
	if request.Operation == OperationNotesRequestCreate {
		change.review.ImpactCount = 1
	}
	reviewBytes, encodeErr := json.Marshal(change.review)
	if encodeErr != nil || len(reviewBytes) > maxChangeReviewBytes {
		return invalidParams(request.RequestID)
	}
	id, err := newChangeID()
	if err != nil {
		return serviceError(request.RequestID, err)
	}
	change.review.OperationID = id
	s.changes.mu.Lock()
	defer s.changes.mu.Unlock()
	if s.changes.disabled {
		return failure(request.RequestID, StatusRejected, "SCOPE_MISMATCH", "要求された保存空間にはアクセスできません。", false)
	}
	s.changes.expireLocked(now)
	if len(s.changes.items) >= maxChanges {
		oldestID := ""
		var oldestCreatedAt time.Time
		for oldID, old := range s.changes.items {
			if old.review.State != "applied" && old.review.State != StatusRejected && old.review.State != "conflict" && old.review.State != "expired" {
				continue
			}
			if oldestID == "" || old.review.CreatedAt.Before(oldestCreatedAt) || old.review.CreatedAt.Equal(oldestCreatedAt) && oldID < oldestID {
				oldestID, oldestCreatedAt = oldID, old.review.CreatedAt
			}
		}
		if oldestID != "" {
			delete(s.changes.items, oldestID)
		}
	}
	if len(s.changes.items) >= maxChanges {
		return failure(request.RequestID, StatusError, "CHANGE_LIMIT", "承認待ちの変更が上限に達しました。", true)
	}
	s.changes.items[id] = change
	data := map[string]any{"operationId": id, "status": StatusPending, "expiresAt": change.review.ExpiresAt, "reviewSummary": map[string]any{"kind": change.review.Kind, "impactCount": change.review.ImpactCount}}
	if request.Operation == OperationNotesProposeEdit {
		data["proposalId"] = id
		beforeValue, _ := change.review.Items[0].Before.(map[string]any)
		before, _ := beforeValue["content"].(string)
		data["diffSummary"] = map[string]any{"beforeBytes": len(before), "afterBytes": len(*change.content)}
	}
	return Response{APIVersion: APIVersion, RequestID: request.RequestID, Status: StatusPending, Data: data}
}

func (s *Service) requestExistingNote(ctx context.Context, principal Principal, request Request, change *pendingChange) Response {
	var id string
	var revision int64
	switch request.Operation {
	case OperationNotesProposeEdit:
		var input proposeEditInput
		if decodeParams(request.Params, &input) != nil || !validID(input.NoteID) || input.BaseRevision < 1 || len(input.After) > maxChangeContentBytes || len(input.Reason) > 2000 || strings.TrimSpace(input.Reason) == "" {
			return invalidParams(request.RequestID)
		}
		id, revision, change.content = input.NoteID, input.BaseRevision, &input.After
		change.review.Items = []ChangeReviewItem{{NoteID: id, Before: map[string]any{"content": input.Before}, After: map[string]any{"content": input.After}, Reason: input.Reason, Status: StatusPending}}
	case OperationNotesRequestUpdate:
		var input requestUpdateInput
		if decodeParams(request.Params, &input) != nil || !validID(input.NoteID) || input.ExpectedRevision < 1 || input.Patch.Title == nil && input.Patch.Content == nil || input.Patch.Title != nil && (strings.TrimSpace(*input.Patch.Title) == "" || len([]rune(*input.Patch.Title)) > 200) || input.Patch.Content != nil && len(*input.Patch.Content) > maxChangeContentBytes {
			return invalidParams(request.RequestID)
		}
		id, revision, change.title, change.content = input.NoteID, input.ExpectedRevision, input.Patch.Title, input.Patch.Content
	case OperationNotesRequestMove:
		var input requestMoveInput
		if decodeParams(request.Params, &input) != nil || !validID(input.NoteID) || input.ExpectedRevision < 1 || !validID(input.TargetNotebookID) {
			return invalidParams(request.RequestID)
		}
		id, revision, change.notebookID = input.NoteID, input.ExpectedRevision, input.TargetNotebookID
		if !s.createDestinationAllowed(ctx, principal, input.TargetNotebookID) {
			return resourceUnavailable(request.RequestID)
		}
	case OperationNotesRequestTags:
		var input requestTagsInput
		if decodeParams(request.Params, &input) != nil || !validID(input.NoteID) || input.ExpectedRevision < 1 || len(input.AddTagIDs)+len(input.RemoveTagIDs) == 0 || len(input.AddTagIDs)+len(input.RemoveTagIDs) > 100 {
			return invalidParams(request.RequestID)
		}
		for _, tagID := range append(append([]string{}, input.AddTagIDs...), input.RemoveTagIDs...) {
			if !validID(tagID) {
				return invalidParams(request.RequestID)
			}
		}
		id, revision, change.addTags, change.removeTags = input.NoteID, input.ExpectedRevision, append([]string(nil), input.AddTagIDs...), append([]string(nil), input.RemoveTagIDs...)
	case OperationNotesRequestTrash:
		var input requestTrashInput
		if decodeParams(request.Params, &input) != nil || !validID(input.NoteID) || input.ExpectedRevision < 1 {
			return invalidParams(request.RequestID)
		}
		id, revision = input.NoteID, input.ExpectedRevision
	}
	allowed, err := s.noteProtectionAllowed(ctx, id)
	if err != nil || !allowed {
		return resourceUnavailable(request.RequestID)
	}
	current, err := s.notes.Get(ctx, id)
	if err != nil || current.IsTrashed || !principalAllowsNote(principal, id, current.NotebookID) {
		return resourceUnavailable(request.RequestID)
	}
	if current.Revision != revision {
		return failure(request.RequestID, StatusConflict, "REVISION_MISMATCH", "ノートのrevisionが一致しません。", false)
	}
	if request.Operation == OperationNotesProposeEdit {
		var input proposeEditInput
		_ = decodeParams(request.Params, &input)
		if current.Content != input.Before {
			return failure(request.RequestID, StatusConflict, "BEFORE_MISMATCH", "変更前の本文が現在の保存内容と一致しません。", false)
		}
	}
	change.noteID = id
	change.review.TargetIDs = []string{id}
	change.review.ExpectedRevision = revision
	if request.Operation == OperationNotesRequestTags {
		result, err := s.notes.ListNoteTags(ctx, id)
		if err != nil || result.Error != nil {
			return resourceUnavailable(request.RequestID)
		}
		for _, tag := range result.Tags {
			change.baseTags = append(change.baseTags, tag.ID)
		}
		sort.Strings(change.baseTags)
		all, err := s.notes.ListTags(ctx)
		if err != nil {
			return serviceError(request.RequestID, err)
		}
		existing := make(map[string]bool, len(all))
		for _, tag := range all {
			existing[tag.ID] = true
		}
		if principal.ScopeRestricted {
			visible, scopeErr := s.scopedTagIDs(ctx, principal)
			if scopeErr != nil {
				return resourceUnavailable(request.RequestID)
			}
			for _, id := range append(append([]string{}, change.addTags...), change.removeTags...) {
				if !visible[id] {
					return resourceUnavailable(request.RequestID)
				}
			}
		}
		for _, id := range change.addTags {
			if !existing[id] {
				return resourceUnavailable(request.RequestID)
			}
		}
	}
	if len(change.review.Items) == 0 {
		change.review.Items = []ChangeReviewItem{{NoteID: id, Status: StatusPending}}
	}
	item := &change.review.Items[0]
	item.NoteTitle = current.Title
	if item.Reason == "" {
		item.Reason = "外部からの変更要求"
	}
	if item.Before == nil {
		before, after := map[string]any{}, map[string]any{}
		switch request.Operation {
		case OperationNotesRequestUpdate:
			if change.title != nil {
				before["title"], after["title"] = current.Title, *change.title
			}
			if change.content != nil {
				before["content"], after["content"] = current.Content, *change.content
			}
		case OperationNotesRequestMove:
			before["notebookId"], after["notebookId"] = current.NotebookID, change.notebookID
		case OperationNotesRequestTags:
			before["tagIds"] = change.baseTags
			next := make(map[string]bool)
			for _, id := range change.baseTags {
				next[id] = true
			}
			for _, id := range change.addTags {
				next[id] = true
			}
			for _, id := range change.removeTags {
				delete(next, id)
			}
			ids := make([]string, 0, len(next))
			for id := range next {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			after["tagIds"] = ids
		case OperationNotesRequestTrash:
			before["isTrashed"], after["isTrashed"] = false, true
		}
		item.Before, item.After = before, after
	}
	return Response{}
}

func (s *Service) createDestinationAllowed(ctx context.Context, principal Principal, id string) bool {
	if !validID(id) || principal.ScopeRestricted && !principal.AllowedNotebookIDs[id] {
		return false
	}
	allowed, err := s.notebookAllowed(ctx, principal, id)
	if err != nil || !allowed {
		return false
	}
	notebooks, err := s.notes.ListNotebooks(ctx)
	if err != nil {
		return false
	}
	for _, notebook := range notebooks {
		if notebook.ID == id {
			return true
		}
	}
	return false
}

func validChangeText(title, content string) bool {
	return strings.TrimSpace(title) != "" && len([]rune(title)) <= 200 && len(content) <= maxChangeContentBytes
}

func appendUnique(values []string, value string) []string {
	if !containsString(values, value) {
		return append(values, value)
	}
	return values
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
func cloneChangePrincipal(p Principal) Principal {
	p.Permissions = clonePrincipalScope(p.Permissions)
	p.AllowedNoteIDs = clonePrincipalScope(p.AllowedNoteIDs)
	p.AllowedNotebookIDs = clonePrincipalScope(p.AllowedNotebookIDs)
	return p
}

func (store *changeStore) expireLocked(now time.Time) {
	for id, item := range store.items {
		if !now.Before(item.review.ExpiresAt) && item.review.State == StatusPending {
			item.review.State = "expired"
			item.review.UpdatedAt = now
		}
		if !now.Before(item.review.ExpiresAt.Add(changeTTL)) {
			delete(store.items, id)
		}
	}
}

// BeginStorageSelection prevents an approved write from crossing the storage
// selection boundary. A failed selection reopens requests in the old space;
// a successful selection destroys all pending operation payloads.
func (s *Service) BeginStorageSelection() func(bool) {
	if s == nil || s.changes == nil {
		return func(bool) {}
	}
	s.changes.applyMu.Lock()
	s.changes.mu.Lock()
	s.changes.disabled = true
	s.changes.mu.Unlock()
	return func(selected bool) {
		s.changes.mu.Lock()
		if selected {
			s.changes.items = make(map[string]*pendingChange)
		} else {
			s.changes.disabled = false
		}
		s.changes.mu.Unlock()
		s.changes.applyMu.Unlock()
	}
}

func (s *Service) getOperation(principal Principal, request Request) Response {
	var input struct {
		OperationID string `json:"operationId"`
	}
	if decodeParams(request.Params, &input) != nil || !validID(input.OperationID) {
		return invalidParams(request.RequestID)
	}
	s.changes.mu.Lock()
	defer s.changes.mu.Unlock()
	if s.changes.disabled {
		return resourceUnavailable(request.RequestID)
	}
	s.changes.expireLocked(s.changes.now().UTC())
	item, ok := s.changes.items[input.OperationID]
	if !ok || item.clientID != principal.ClientID || item.review.StorageSpaceID != principal.StorageSpaceID {
		return resourceUnavailable(request.RequestID)
	}
	return success(request.RequestID, map[string]any{"operationId": input.OperationID, "state": item.review.State, "updatedAt": item.review.UpdatedAt, "items": item.resultItems})
}

// ListChangeReviews is a trusted GUI-only view. It does not accept a client ID.
func (s *Service) ListChangeReviews(spaceID string) []ChangeReview {
	if s == nil || s.changes == nil || spaceID != s.activeSpaceID {
		return nil
	}
	s.changes.mu.Lock()
	if s.changes.disabled {
		s.changes.mu.Unlock()
		return nil
	}
	s.changes.expireLocked(s.changes.now().UTC())
	result := make([]ChangeReview, 0, len(s.changes.items))
	for _, item := range s.changes.items {
		if item.review.StorageSpaceID != spaceID {
			continue
		}
		copy := item.review
		copy.TargetIDs = append([]string(nil), copy.TargetIDs...)
		copy.Items = append([]ChangeReviewItem(nil), copy.Items...)
		result = append(result, copy)
	}
	s.changes.mu.Unlock()
	for index := range result {
		for _, targetID := range result[index].TargetIDs {
			if result[index].Kind == OperationNotesRequestCreate {
				continue
			}
			allowed, err := s.noteProtectionAllowed(context.Background(), targetID)
			if err != nil || !allowed {
				result[index].Items = []ChangeReviewItem{}
				result[index].TargetIDs = []string{}
				result[index].State = "unavailable"
				result[index].Message = "対象を確認できません。"
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

func (s *Service) RejectChange(spaceID, id string) ChangeResult {
	result := ChangeResult{OperationID: id, State: StatusRejected}
	if s == nil || s.changes == nil || spaceID != s.activeSpaceID {
		result.State = "unavailable"
		return result
	}
	s.changes.mu.Lock()
	defer s.changes.mu.Unlock()
	if s.changes.disabled {
		result.State = "unavailable"
		return result
	}
	s.changes.expireLocked(s.changes.now().UTC())
	item, ok := s.changes.items[id]
	if !ok || item.review.StorageSpaceID != spaceID || item.review.State != StatusPending {
		result.State = "unavailable"
		return result
	}
	item.review.State, item.review.UpdatedAt = StatusRejected, s.changes.now().UTC()
	return result
}

// ApproveChange can only be reached from the trusted GUI bridge. The permit is
// generated and consumed inside this call; no token generation or apply route
// exists in the external protocol.
func (s *Service) ApproveChange(ctx context.Context, spaceID, id string) (result ChangeResult) {
	result = ChangeResult{OperationID: id, State: "unavailable"}
	if s == nil || s.changes == nil || s.writes == nil || spaceID != s.activeSpaceID {
		return result
	}
	s.changes.applyMu.Lock()
	defer s.changes.applyMu.Unlock()
	s.changes.mu.Lock()
	if s.changes.disabled {
		s.changes.mu.Unlock()
		return result
	}
	s.changes.expireLocked(s.changes.now().UTC())
	item, ok := s.changes.items[id]
	if !ok || item.review.StorageSpaceID != spaceID || item.review.State != StatusPending {
		s.changes.mu.Unlock()
		return result
	}
	item.review.State = "applying"
	s.changes.mu.Unlock()
	defer func() {
		s.changes.mu.Lock()
		if item.review.Kind == OperationOrganizeRequestApply && len(result.Items) > 0 {
			byID := make(map[string]organize.ApplyResult, len(item.resultItems)+len(result.Items))
			for _, entry := range item.resultItems {
				byID[entry.CandidateID] = entry
			}
			for _, entry := range result.Items {
				byID[entry.CandidateID] = entry
			}
			result.Items = make([]organize.ApplyResult, 0, len(item.review.Items))
			pending := make([]string, 0)
			hasConflict := false
			for index := range item.review.Items {
				candidateID := item.review.Items[index].CandidateID
				entry, exists := byID[candidateID]
				if !exists {
					continue
				}
				result.Items = append(result.Items, entry)
				item.review.Items[index].Status = entry.Status
				if entry.Status == "save-failure" || entry.Status == "not-executed" {
					pending = append(pending, candidateID)
				}
				if entry.Status != "applied" {
					hasConflict = true
				}
			}
			item.candidateIDs = pending
			switch {
			case len(pending) > 0:
				result.State = StatusPending
			case hasConflict:
				result.State = "conflict"
			default:
				result.State = "applied"
			}
		} else if len(item.review.Items) > 0 {
			item.review.Items[0].Status = result.State
		}
		item.review.State, item.review.UpdatedAt = result.State, s.changes.now().UTC()
		item.review.Message = result.Message
		item.resultItems = append([]organize.ApplyResult(nil), result.Items...)
		s.changes.mu.Unlock()
	}()
	verifyCtx, release := s.notes.BeginExternalRead(ctx)
	precondition := s.verifyChange(verifyCtx, item)
	var stateHash [32]byte
	var stateErr error
	if precondition == "" {
		stateHash, stateErr = s.targetStateHash(verifyCtx, item)
	}
	release()
	if precondition != "" {
		result.State, result.Message = "conflict", precondition
		return result
	}
	if stateErr != nil {
		result.State, result.Message = "conflict", "対象状態を確認できません。"
		return result
	}
	permit, token, err := issueApprovalPermit(item, stateHash, s.changes.now().UTC())
	if err != nil {
		result.State = StatusPending
		result.Message = "承認を開始できませんでした。"
		return result
	}
	verifyCtx, release = s.notes.BeginExternalRead(ctx)
	currentHash, stateErr := s.targetStateHash(verifyCtx, item)
	release()
	if stateErr != nil || !permit.consume(item, token, currentHash, s.changes.now().UTC()) {
		result.State = StatusPending
		result.Message = "対象状態が変わりました。変更案は保持されています。"
		return result
	}
	if item.review.Kind == OperationOrganizeRequestApply {
		result.Items = s.organizer.ApplyExternalCandidates(ctx, spaceID, item.clientID, organize.ApplyCandidatesInput{SessionID: item.analysisID, CandidateIDs: item.candidateIDs})
		result.State = "applied"
		return result
	}
	var saveErr error
	switch item.review.Kind {
	case OperationNotesRequestCreate:
		var notebookID *string
		if item.notebookID != "" {
			notebookID = &item.notebookID
		}
		_, saveErr = s.writes.Create(ctx, note.CreateInput{NotebookID: notebookID, Title: *item.title, Content: *item.content})
	case OperationNotesRequestTags:
		next := make(map[string]bool)
		for _, id := range item.baseTags {
			next[id] = true
		}
		for _, id := range item.addTags {
			next[id] = true
		}
		for _, id := range item.removeTags {
			delete(next, id)
		}
		ids := make([]string, 0, len(next))
		for id := range next {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		tags, err := s.writes.SetNoteTagsWithExpectedRevision(ctx, item.noteID, note.SetNoteTagsWithExpectedRevisionInput{TagIDs: ids, ExpectedTagIDs: item.baseTags, ExpectedRevision: item.review.ExpectedRevision})
		saveErr = err
		if tags.RevisionConflict != nil || tags.Error != nil {
			result.State = "conflict"
			result.Message = "revisionまたはタグ状態が変更されました。"
			return result
		}
	default:
		input := note.UpdateInput{ExpectedRevision: &item.review.ExpectedRevision, Title: item.title, Content: item.content}
		if item.review.Kind == OperationNotesRequestMove {
			input.NotebookID = &item.notebookID
		}
		if item.review.Kind == OperationNotesRequestTrash {
			trashed := true
			input.IsTrashed = &trashed
		}
		_, saveErr = s.writes.Update(ctx, item.noteID, input)
	}
	if saveErr != nil {
		var conflict *note.RevisionConflict
		if errors.As(saveErr, &conflict) || errors.Is(saveErr, note.ErrRevisionConflict) {
			result.State = "conflict"
			result.Message = "revisionが変更されました。"
		} else {
			result.State = StatusPending
			result.Message = "保存できませんでした。変更案は保持されています。"
		}
		return result
	}
	result.State = "applied"
	return result
}

func (s *Service) verifyChange(ctx context.Context, item *pendingChange) string {
	if item.review.StorageSpaceID != s.activeSpaceID || !s.changes.now().UTC().Before(item.review.ExpiresAt) {
		return "保存空間または有効期限が変わりました。"
	}
	var visibleTagIDs map[string]bool
	tagsAllowed := func(ids []string) bool {
		if !item.principal.ScopeRestricted || len(ids) == 0 {
			return true
		}
		if visibleTagIDs == nil {
			var err error
			visibleTagIDs, err = s.scopedTagIDs(ctx, item.principal)
			if err != nil {
				return false
			}
		}
		for _, id := range ids {
			if !visibleTagIDs[id] {
				return false
			}
		}
		return true
	}
	if item.notebookID != "" && !s.createDestinationAllowed(ctx, item.principal, item.notebookID) {
		return "移動先または作成先を確認できません。"
	}
	if item.review.Kind == OperationOrganizeRequestApply {
		snapshot, err := s.organizer.AnalysisSnapshot(s.activeSpaceID, item.clientID, item.analysisID, organizationAccess(item.principal))
		if err != nil {
			return "解析結果が古くなりました。"
		}
		found := make(map[string]bool)
		for _, candidate := range snapshot.Candidates {
			if !containsString(item.candidateIDs, candidate.ID) {
				continue
			}
			found[candidate.ID] = true
			allowed, accessErr := s.noteProtectionAllowed(ctx, candidate.NoteID)
			if accessErr != nil || !allowed {
				return "対象を確認できません。"
			}
			current, getErr := s.notes.Get(ctx, candidate.NoteID)
			if getErr != nil || current.IsTrashed || !principalAllowsNote(item.principal, current.ID, current.NotebookID) {
				return "対象を確認できません。"
			}
			if candidate.Kind == organize.KindTagAssignment && !tagsAllowed([]string{candidate.TagID}) {
				return "対象を確認できません。"
			}
			if candidate.Kind == organize.KindNotebookMove || candidate.Kind == organize.KindNotebookAssignment {
				dest, _ := candidate.Proposed["notebookId"].(string)
				if !s.createDestinationAllowed(ctx, item.principal, dest) {
					return "移動先を確認できません。"
				}
			}
		}
		if len(found) != len(item.candidateIDs) {
			return "解析結果が古くなりました。"
		}
		return ""
	}
	if item.review.Kind == OperationNotesRequestCreate {
		return ""
	}
	allowed, err := s.noteProtectionAllowed(ctx, item.noteID)
	if err != nil || !allowed {
		return "対象を確認できません。"
	}
	current, err := s.notes.Get(ctx, item.noteID)
	if err != nil || current.IsTrashed || !principalAllowsNote(item.principal, current.ID, current.NotebookID) {
		return "対象を確認できません。"
	}
	if current.Revision != item.review.ExpectedRevision {
		return "revisionが変更されました。"
	}
	if item.review.Kind == OperationNotesRequestTags {
		if !tagsAllowed(append(append([]string{}, item.addTags...), item.removeTags...)) {
			return "対象を確認できません。"
		}
		currentTags, err := s.notes.ListNoteTags(ctx, item.noteID)
		if err != nil || currentTags.Error != nil {
			return "タグ状態を確認できません。"
		}
		ids := make([]string, 0, len(currentTags.Tags))
		for _, tag := range currentTags.Tags {
			ids = append(ids, tag.ID)
		}
		sort.Strings(ids)
		if !equalOrganizationStrings(ids, item.baseTags) {
			return "タグ状態が変更されました。"
		}
	}
	return ""
}

type approvalPermit struct {
	tokenHash   [32]byte
	operationID string
	spaceID     string
	stateHash   [32]byte
	expiresAt   time.Time
	used        bool
}

func issueApprovalPermit(item *pendingChange, stateHash [32]byte, now time.Time) (*approvalPermit, [32]byte, error) {
	p := &approvalPermit{operationID: item.review.OperationID, spaceID: item.review.StorageSpaceID, expiresAt: now.Add(30 * time.Second)}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, token, err
	}
	p.tokenHash = sha256.Sum256(token[:])
	p.stateHash = stateHash
	return p, token, nil
}

func (s *Service) targetStateHash(ctx context.Context, item *pendingChange) ([32]byte, error) {
	type targetState struct {
		ID          string
		Revision    int64
		ContentHash [32]byte
		NotebookID  *string
		Tags        []string
	}
	targetIDs := item.review.TargetIDs
	if item.review.Kind == OperationOrganizeRequestApply {
		targetIDs = nil
		for _, review := range item.review.Items {
			if containsString(item.candidateIDs, review.CandidateID) {
				targetIDs = appendUnique(targetIDs, review.NoteID)
			}
		}
	}
	states := make([]targetState, 0, len(targetIDs))
	if item.review.Kind != OperationNotesRequestCreate {
		for _, id := range targetIDs {
			current, err := s.notes.Get(ctx, id)
			if err != nil {
				return [32]byte{}, err
			}
			tags, err := s.notes.ListNoteTags(ctx, id)
			if err != nil || tags.Error != nil {
				return [32]byte{}, errors.New("target tags unavailable")
			}
			state := targetState{ID: id, Revision: current.Revision, ContentHash: sha256.Sum256([]byte(current.Content)), NotebookID: current.NotebookID}
			for _, tag := range tags.Tags {
				state.Tags = append(state.Tags, tag.ID)
			}
			sort.Strings(state.Tags)
			states = append(states, state)
		}
	}
	encoded, _ := json.Marshal(struct {
		OperationID string
		SpaceID     string
		NotebookID  string
		Targets     []targetState
	}{item.review.OperationID, item.review.StorageSpaceID, item.notebookID, states})
	return sha256.Sum256(encoded), nil
}
func (p *approvalPermit) consume(item *pendingChange, token [32]byte, stateHash [32]byte, now time.Time) bool {
	actual := sha256.Sum256(token[:])
	if p.used || !now.Before(p.expiresAt) || p.operationID != item.review.OperationID || p.spaceID != item.review.StorageSpaceID || p.stateHash != stateHash || subtle.ConstantTimeCompare(actual[:], p.tokenHash[:]) != 1 {
		return false
	}
	p.used = true
	return true
}
