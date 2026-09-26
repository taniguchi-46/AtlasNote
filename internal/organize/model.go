package organize

import "time"

const (
	KindNotebookAssignment = "notebook-assignment"
	KindNotebookMove       = "notebook-move"
	KindUnclassifiedNote   = "unclassified-note"
	KindTagAssignment      = "tag-assignment"
	KindDuplicateNote      = "duplicate-note"
	KindEmptyNote          = "empty-note"
	KindBrokenLink         = "broken-link"
	KindOrphanNote         = "orphan-note"
	KindRelatedNote        = "related-note"
	KindReciprocalLink     = "reciprocal-link"
	KindTitle              = "title"
	KindDuplicateTag       = "duplicate-tag"
)

type Candidate struct {
	ID                 string         `json:"id"`
	Kind               string         `json:"kind"`
	NoteID             string         `json:"noteId,omitempty"`
	NoteTitle          string         `json:"noteTitle,omitempty"`
	RelatedID          string         `json:"relatedId,omitempty"`
	RelatedTitle       string         `json:"relatedTitle,omitempty"`
	NotebookID         string         `json:"notebookId,omitempty"`
	TagID              string         `json:"tagId,omitempty"`
	SpaceID            string         `json:"spaceId"`
	BaseRevision       int64          `json:"baseRevision,omitempty"`
	RelatedRevision    int64          `json:"relatedRevision,omitempty"`
	RelatedContentHash string         `json:"relatedContentHash,omitempty"`
	Before             map[string]any `json:"before"`
	Proposed           map[string]any `json:"proposed,omitempty"`
	Reason             string         `json:"reason"`
	Applicable         bool           `json:"applicable"`
}

type Analysis struct {
	SessionID     string      `json:"sessionId"`
	SpaceID       string      `json:"spaceId"`
	NotebookID    string      `json:"notebookId,omitempty"`
	NoteID        string      `json:"noteId,omitempty"`
	Scope         string      `json:"scope"`
	StartedAt     time.Time   `json:"startedAt"`
	Candidates    []Candidate `json:"candidates"`
	AnalyzedNotes int         `json:"analyzedNotes"`
	SkippedLocked int         `json:"skippedLocked"`
	SkippedTrash  int         `json:"skippedTrash"`
}

type AnalysisInput struct {
	Scope      string `json:"scope"`
	NotebookID string `json:"notebookId,omitempty"`
	NoteID     string `json:"noteId,omitempty"`
	RequestID  string `json:"requestId,omitempty"`
}

// AnalysisAccess limits an external analysis to the notes and notebooks that
// were explicitly published to one authenticated client session. GUI analyses
// leave ScopeRestricted false and keep the existing whole-space behaviour.
type AnalysisAccess struct {
	OwnerID            string
	ScopeRestricted    bool
	AllowedNoteIDs     map[string]bool
	AllowedNotebookIDs map[string]bool
}

type AnalysisSnapshot struct {
	Analysis              Analysis
	ExpiresAt             time.Time
	Candidates            []Candidate
	AnalyzedNoteIDs       []string
	AnalyzedNoteRevisions map[string]int64
	AnalyzedNoteTagIDs    map[string][]string
	Access                AnalysisAccess
}

type AnalysisProgress struct {
	Phase          string `json:"phase"`
	ProcessedNotes int    `json:"processedNotes"`
	TotalNotes     int    `json:"totalNotes"`
}

type ApplyInput struct {
	SessionID   string `json:"sessionId"`
	CandidateID string `json:"candidateId"`
}

type ApplyCandidatesInput struct {
	SessionID    string   `json:"sessionId"`
	CandidateIDs []string `json:"candidateIds"`
}

type ApplyResult struct {
	CandidateID string `json:"candidateId"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}
