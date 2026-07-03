package note

import "time"

type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type UpdateInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Record struct {
	ID          string
	Title       string
	ContentPath string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
