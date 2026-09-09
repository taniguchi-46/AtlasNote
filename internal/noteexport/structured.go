package noteexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"

	"atlasnote/internal/note"
)

const (
	structuredExportFormat  = "atlasnote-notes"
	structuredExportVersion = 1
)

type structuredExport struct {
	Format  string           `json:"format"`
	Version int              `json:"version"`
	Notes   []structuredNote `json:"notes"`
}

type structuredNote struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func renderJSON(current note.Note) ([]byte, error) {
	value := structuredExport{
		Format:  structuredExportFormat,
		Version: structuredExportVersion,
		Notes: []structuredNote{{
			Title:   current.Title,
			Content: current.Content,
		}},
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func renderCSV(current note.Note) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write([]string{"title", "content"}); err != nil {
		return nil, err
	}
	if err := writer.Write([]string{safeCSVCell(current.Title), safeCSVCell(current.Content)}); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func safeCSVCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}
