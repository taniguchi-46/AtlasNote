package note

import "time"

type Summary struct {
	ID         string    `json:"id"`
	NotebookID *string   `json:"notebookId"`
	Title      string    `json:"title"`
	IsFavorite bool      `json:"isFavorite"`
	IsPinned   bool      `json:"isPinned"`
	IsTrashed  bool      `json:"isTrashed"`
	Revision   int64     `json:"revision"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

const (
	DefaultNoteListPage     = 1
	DefaultNoteListPageSize = 100
	MaxNoteListPage         = 10000
	MaxNoteListPageSize     = 100
)

const (
	NoteSortByUpdatedAt = "updatedAt"
	NoteSortByCreatedAt = "createdAt"
	NoteSortByTitle     = "title"

	NoteSortDirectionAsc  = "asc"
	NoteSortDirectionDesc = "desc"
)

type NoteListInput struct {
	Page          int     `json:"page"`
	PageSize      int     `json:"pageSize"`
	TagID         *string `json:"tagId,omitempty"`
	SortBy        string  `json:"sortBy,omitempty"`
	SortDirection string  `json:"sortDirection,omitempty"`
	TodayOnly     bool    `json:"todayOnly,omitempty"`
}

type NoteListResult struct {
	Items    []Summary `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
	HasNext  bool      `json:"hasNext"`
}

type Note struct {
	ID         string    `json:"id"`
	NotebookID *string   `json:"notebookId"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	IsFavorite bool      `json:"isFavorite"`
	IsPinned   bool      `json:"isPinned"`
	IsTrashed  bool      `json:"isTrashed"`
	Revision   int64     `json:"revision"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type CreateInput struct {
	NotebookID *string `json:"notebookId"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
}

type UpdateInput struct {
	NotebookID       *string `json:"notebookId"`
	ClearNotebook    *bool   `json:"clearNotebook"`
	Title            *string `json:"title"`
	Content          *string `json:"content"`
	IsFavorite       *bool   `json:"isFavorite"`
	IsPinned         *bool   `json:"isPinned"`
	IsTrashed        *bool   `json:"isTrashed"`
	ExpectedRevision *int64  `json:"expectedRevision,omitempty"`
}

type DeleteInput struct {
	ExpectedRevision int64 `json:"expectedRevision"`
}

const MaxTagNameLength = 100

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TagCreateInput struct {
	Name string `json:"name"`
}

type TagUpdateInput struct {
	Name string `json:"name"`
}

type SetNoteTagsInput struct {
	TagIDs []string `json:"tagIds"`
}

type TagError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	Retryable bool   `json:"retryable"`
}

type TagMutationResult struct {
	Tag   *Tag      `json:"tag,omitempty"`
	Error *TagError `json:"error,omitempty"`
}

type TagDeleteResult struct {
	Deleted bool      `json:"deleted"`
	Error   *TagError `json:"error,omitempty"`
}

type NoteTagsResult struct {
	Tags  []Tag     `json:"tags"`
	Error *TagError `json:"error,omitempty"`
}

const (
	TagErrorNameEmpty    = "TAG_NAME_EMPTY"
	TagErrorNameTooLong  = "TAG_NAME_TOO_LONG"
	TagErrorNameInvalid  = "TAG_NAME_INVALID"
	TagErrorNameConflict = "TAG_NAME_CONFLICT"
	TagErrorNotFound     = "TAG_NOT_FOUND"
	TagErrorNoteNotFound = "TAG_NOTE_NOT_FOUND"
)

const (
	SearchScopeAll   = "all"
	SearchScopeTitle = "title"

	DefaultSearchPage     = 1
	DefaultSearchPageSize = 30
	MaxSearchPage         = 10000
	MaxSearchPageSize     = 100
	MaxSearchQueryLength  = 200
)

type SearchInput struct {
	Query          string  `json:"query"`
	Scope          string  `json:"scope"`
	NotebookID     *string `json:"notebookId,omitempty"`
	IncludeTrashed bool    `json:"includeTrashed"`
	Page           int     `json:"page"`
	PageSize       int     `json:"pageSize"`
	SortBy         string  `json:"sortBy,omitempty"`
	SortDirection  string  `json:"sortDirection,omitempty"`
}

type SearchItem struct {
	Note       Summary `json:"note"`
	Snippet    string  `json:"snippet"`
	MatchScope string  `json:"matchScope"`
}

type SearchError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	Retryable bool   `json:"retryable"`
}

type SearchResult struct {
	Items    []SearchItem `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int          `json:"total"`
	HasNext  bool         `json:"hasNext"`
	Error    *SearchError `json:"error,omitempty"`
}

const (
	SearchErrorQueryTooLong         = "SEARCH_QUERY_TOO_LONG"
	SearchErrorQueryInvalid         = "SEARCH_QUERY_INVALID"
	SearchErrorScopeInvalid         = "SEARCH_SCOPE_INVALID"
	SearchErrorPageInvalid          = "SEARCH_PAGE_INVALID"
	SearchErrorPageSizeInvalid      = "SEARCH_PAGE_SIZE_INVALID"
	SearchErrorSortByInvalid        = "SEARCH_SORT_BY_INVALID"
	SearchErrorSortDirectionInvalid = "SEARCH_SORT_DIRECTION_INVALID"
	SearchErrorIndexNotReady        = "SEARCH_INDEX_NOT_READY"
	SearchErrorIndexInconsistent    = "SEARCH_INDEX_INCONSISTENT"
	SearchErrorIndexFailed          = "SEARCH_INDEX_FAILED"
)

const ErrorCodeRevisionConflict = "NOTE_REVISION_CONFLICT"

type RevisionConflict struct {
	Code             string `json:"code"`
	NoteID           string `json:"noteId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	ActualRevision   int64  `json:"actualRevision"`
}

func (c *RevisionConflict) Error() string {
	return ErrorCodeRevisionConflict
}

func (c *RevisionConflict) Is(target error) bool {
	return target == ErrRevisionConflict
}

type UpdateNoteResult struct {
	Note     *Note             `json:"note,omitempty"`
	Conflict *RevisionConflict `json:"conflict,omitempty"`
}

type DeleteNoteResult struct {
	Deleted  bool              `json:"deleted"`
	Conflict *RevisionConflict `json:"conflict,omitempty"`
}

type Record struct {
	ID          string
	NotebookID  *string
	Title       string
	ContentPath string
	IsFavorite  bool
	IsPinned    bool
	IsTrashed   bool
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const (
	StorageOperationUpsert = "upsert"
	StorageOperationDelete = "delete"
)

type StorageOperation struct {
	ID          string
	NoteID      string
	Type        string
	ContentHash string
	CreatedAt   time.Time
}

type MissingContent struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ContentPath string `json:"contentPath"`
}

type RecoveryReport struct {
	MissingNotes []MissingContent `json:"missingNotes"`
}

type Notebook struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parentId"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NotebookCreateInput struct {
	ParentID *string `json:"parentId"`
	Name     string  `json:"name"`
	Icon     *string `json:"icon"`
}

type NotebookUpdateInput struct {
	ParentID    *string `json:"parentId"`
	ClearParent *bool   `json:"clearParent"`
	Name        *string `json:"name"`
	Icon        *string `json:"icon"`
}

type NotebookDeleteInput struct {
	Mode string `json:"mode"`
}
