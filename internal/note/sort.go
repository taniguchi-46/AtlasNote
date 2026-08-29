package note

import (
	"errors"
	"fmt"
)

var (
	ErrSortByInvalid        = errors.New("note sort field is invalid")
	ErrSortDirectionInvalid = errors.New("note sort direction is invalid")
)

type noteSortSpec struct {
	Column    string
	Direction string
}

var noteSortColumns = map[string]string{
	NoteSortByUpdatedAt: "updated_at",
	NoteSortByCreatedAt: "created_at",
	NoteSortByTitle:     "title",
}

var noteSortDirections = map[string]string{
	NoteSortDirectionAsc:  "ASC",
	NoteSortDirectionDesc: "DESC",
}

func normalizeNoteSort(sortBy, sortDirection string) (noteSortSpec, error) {
	if sortBy == "" && sortDirection == "" {
		return noteSortSpec{
			Column:    noteSortColumns[NoteSortByUpdatedAt],
			Direction: noteSortDirections[NoteSortDirectionDesc],
		}, nil
	}

	column, ok := noteSortColumns[sortBy]
	if !ok {
		return noteSortSpec{}, fmt.Errorf("%w: %q", ErrSortByInvalid, sortBy)
	}
	direction, ok := noteSortDirections[sortDirection]
	if !ok {
		return noteSortSpec{}, fmt.Errorf("%w: %q", ErrSortDirectionInvalid, sortDirection)
	}

	return noteSortSpec{Column: column, Direction: direction}, nil
}
