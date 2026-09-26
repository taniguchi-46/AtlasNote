package readapi

import (
	"encoding/json"
	"time"
)

const APIVersion = "1"

const (
	StatusOK       = "ok"
	StatusConflict = "conflict"
	StatusRejected = "rejected"
	StatusError    = "error"
)

const (
	PermissionMetadata = "R0"
	PermissionContent  = "R1"
	PermissionProposal = "P"
)

const (
	OperationNotesList             = "notes.list"
	OperationNotesGet              = "notes.get"
	OperationNotesSearch           = "notes.search"
	OperationNotebooksList         = "notebooks.list"
	OperationTagsList              = "tags.list"
	OperationBacklinks             = "notes.backlinks"
	OperationRelated               = "notes.related"
	OperationOrganizeAnalyze       = "organize.analyze"
	OperationOrganizeGetCandidates = "organize.get_candidates"
)

type Principal struct {
	ClientID           string
	Kind               string
	StorageSpaceID     string
	Permissions        map[string]bool
	ScopeRestricted    bool
	AllowedNoteIDs     map[string]bool
	AllowedNotebookIDs map[string]bool
}

type Scope struct {
	StorageSpaceID string `json:"storageSpaceId"`
}

type Request struct {
	APIVersion string          `json:"apiVersion"`
	RequestID  string          `json:"requestId"`
	ClientID   string          `json:"clientId"`
	Scope      Scope           `json:"scope"`
	Operation  string          `json:"operation"`
	Params     json.RawMessage `json:"params"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Response struct {
	APIVersion string    `json:"apiVersion"`
	RequestID  string    `json:"requestId"`
	Status     string    `json:"status"`
	Data       any       `json:"data"`
	Error      *APIError `json:"error"`
}

type NoteSummary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Revision   int64     `json:"revision"`
	UpdatedAt  time.Time `json:"updatedAt"`
	NotebookID *string   `json:"notebookId,omitempty"`
}

type Note struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Revision   int64     `json:"revision"`
	NotebookID *string   `json:"notebookId,omitempty"`
	Tags       []Tag     `json:"tags"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type SearchMatch struct {
	NoteID     string `json:"noteId"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
	Revision   int64  `json:"revision"`
	MatchScope string `json:"matchScope"`
}

type Notebook struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId,omitempty"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RelatedItem struct {
	NoteID   string   `json:"noteId"`
	Title    string   `json:"title"`
	Revision int64    `json:"revision"`
	Snippet  string   `json:"snippet"`
	Reasons  []string `json:"reasons"`
}

type NoteListInput struct {
	NotebookID    *string `json:"notebookId,omitempty"`
	TagID         *string `json:"tagId,omitempty"`
	SortBy        string  `json:"sortBy,omitempty"`
	SortDirection string  `json:"sortDirection,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	Cursor        string  `json:"cursor,omitempty"`
}

type NoteGetInput struct {
	NoteID           string `json:"noteId"`
	ExpectedRevision *int64 `json:"expectedRevision,omitempty"`
}

type SearchInput struct {
	Query         string  `json:"query"`
	Scope         string  `json:"scope,omitempty"`
	NotebookID    *string `json:"notebookId,omitempty"`
	SortBy        string  `json:"sortBy,omitempty"`
	SortDirection string  `json:"sortDirection,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	Cursor        string  `json:"cursor,omitempty"`
}

type NotebookListInput struct {
	ParentID           *string `json:"parentId,omitempty"`
	IncludeDescendants bool    `json:"includeDescendants,omitempty"`
	Limit              int     `json:"limit,omitempty"`
	Cursor             string  `json:"cursor,omitempty"`
}

type TagListInput struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type BacklinkInput struct {
	NoteID string `json:"noteId"`
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type RelatedInput struct {
	NoteID      string  `json:"noteId"`
	NotebookID  *string `json:"notebookId,omitempty"`
	Descendants bool    `json:"descendants,omitempty"`
	Limit       int     `json:"limit,omitempty"`
}

type OrganizeAnalyzeInput struct {
	Scope      string `json:"scope"`
	NotebookID string `json:"notebookId,omitempty"`
	NoteID     string `json:"noteId,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type OrganizeCandidatesInput struct {
	AnalysisID string `json:"analysisId"`
	Kind       string `json:"kind,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
}

type NoteListData struct {
	Notes      []NoteSummary `json:"notes"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type NoteGetData struct {
	Note Note `json:"note"`
}

type SearchData struct {
	Matches    []SearchMatch `json:"matches"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type NotebookListData struct {
	Notebooks  []Notebook `json:"notebooks"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type TagListData struct {
	Tags       []Tag  `json:"tags"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type BacklinkData struct {
	Items      []NoteSummary `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type RelatedData struct {
	Items []RelatedItem `json:"items"`
}

type OrganizationCandidate struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	NoteID          string         `json:"noteId,omitempty"`
	NoteTitle       string         `json:"noteTitle,omitempty"`
	RelatedID       string         `json:"relatedId,omitempty"`
	RelatedTitle    string         `json:"relatedTitle,omitempty"`
	NotebookID      string         `json:"notebookId,omitempty"`
	TagID           string         `json:"tagId,omitempty"`
	Reason          string         `json:"reason"`
	Before          map[string]any `json:"before"`
	Proposed        map[string]any `json:"proposed,omitempty"`
	Applicable      bool           `json:"applicable"`
	BaseRevision    int64          `json:"baseRevision,omitempty"`
	RelatedRevision int64          `json:"relatedRevision,omitempty"`
}

type OrganizationSummary struct {
	Scope          string    `json:"scope"`
	NotebookID     string    `json:"notebookId,omitempty"`
	NoteID         string    `json:"noteId,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	AnalyzedNotes  int       `json:"analyzedNotes"`
	SkippedLocked  int       `json:"skippedLocked"`
	SkippedTrash   int       `json:"skippedTrash"`
	CandidateCount int       `json:"candidateCount"`
}

type OrganizeAnalyzeData struct {
	AnalysisID string                  `json:"analysisId"`
	Summary    OrganizationSummary     `json:"summary"`
	Candidates []OrganizationCandidate `json:"candidates"`
	NextCursor string                  `json:"nextCursor,omitempty"`
}

type OrganizeCandidatesData struct {
	AnalysisID string                  `json:"analysisId"`
	Candidates []OrganizationCandidate `json:"candidates"`
	NextCursor string                  `json:"nextCursor,omitempty"`
	ExpiresAt  time.Time               `json:"expiresAt"`
}
