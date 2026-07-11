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
