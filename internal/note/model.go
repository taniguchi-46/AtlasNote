package note

import "time"

type Summary struct {
	ID         string    `json:"id"`
	NotebookID *string   `json:"notebookId"`
	Title      string    `json:"title"`
	IsFavorite bool      `json:"isFavorite"`
	IsPinned   bool      `json:"isPinned"`
	IsTrashed  bool      `json:"isTrashed"`
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
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type CreateInput struct {
	NotebookID *string `json:"notebookId"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
}

type UpdateInput struct {
	NotebookID    *string `json:"notebookId"`
	ClearNotebook *bool   `json:"clearNotebook"`
	Title         *string `json:"title"`
	Content       *string `json:"content"`
	IsFavorite    *bool   `json:"isFavorite"`
	IsPinned      *bool   `json:"isPinned"`
	IsTrashed     *bool   `json:"isTrashed"`
}

type Record struct {
	ID          string
	NotebookID  *string
	Title       string
	ContentPath string
	IsFavorite  bool
	IsPinned    bool
	IsTrashed   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Notebook struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parentId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NotebookCreateInput struct {
	ParentID *string `json:"parentId"`
	Name     string  `json:"name"`
}

type NotebookUpdateInput struct {
	ParentID    *string `json:"parentId"`
	ClearParent *bool   `json:"clearParent"`
	Name        *string `json:"name"`
}
