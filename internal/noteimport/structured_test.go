package noteimport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportJSONCreatesOneNotePerRecordAndPreservesContent(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "notes.json")
	payload := map[string]any{
		"format":  structuredFormat,
		"version": structuredVersion,
		"notes": []map[string]string{
			{"title": "First", "content": "# First heading\nbody"},
			{"title": "Second", "content": "body\nwith\nnewlines"},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write JSON: %v", err)
	}

	writer := &fakeNoteWriter{}
	result := NewService(writer).Import(context.Background(), []string{path}, Input{})
	if result.Error != nil || len(result.Imported) != 2 || len(result.Failures) != 0 {
		t.Fatalf("JSON import result = %#v", result)
	}
	if len(writer.createdNotes) != 2 {
		t.Fatalf("created notes = %#v", writer.createdNotes)
	}
	if writer.createdNotes[0].Title != "First" || writer.createdNotes[0].Content != payload["notes"].([]map[string]string)[0]["content"] {
		t.Fatalf("first JSON note = %#v", writer.createdNotes[0])
	}
	if writer.createdNotes[1].Title != "Second" || result.Imported[1].RecordNumber != 2 {
		t.Fatalf("second JSON note = %#v, result = %#v", writer.createdNotes[1], result.Imported[1])
	}
}

func TestImportCSVSupportsQuotedContentAndRecordTitleFallback(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "notes.csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create CSV: %v", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"title", "content"}); err != nil {
		t.Fatalf("write CSV header: %v", err)
	}
	if err := writer.Write([]string{"CSV title", "line 1\nline 2"}); err != nil {
		t.Fatalf("write CSV first row: %v", err)
	}
	if err := writer.Write([]string{"", "plain"}); err != nil {
		t.Fatalf("write CSV second row: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush CSV: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close CSV: %v", err)
	}

	noteWriter := &fakeNoteWriter{}
	result := NewService(noteWriter).Import(context.Background(), []string{path}, Input{})
	if result.Error != nil || len(result.Imported) != 2 {
		t.Fatalf("CSV import result = %#v", result)
	}
	if noteWriter.createdNotes[0].Content != "line 1\nline 2" {
		t.Fatalf("quoted CSV content = %q", noteWriter.createdNotes[0].Content)
	}
	if noteWriter.createdNotes[1].Title != "notes" {
		t.Fatalf("empty record title fallback = %q", noteWriter.createdNotes[1].Title)
	}
	if result.Imported[0].RecordNumber != 1 || result.Imported[1].RecordNumber != 2 {
		t.Fatalf("CSV record numbers = %#v", result.Imported)
	}
}

func TestImportStructuredFileValidatesAllRecordsBeforePersisting(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "invalid.json")
	content := fmt.Sprintf(`{"format":%q,"version":1,"notes":[{"title":"ok","content":"saved"},{"title":"bad","content":%q}]}`,
		structuredFormat, strings.Repeat("x", maxImportedContentSize+1))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write invalid JSON: %v", err)
	}

	writer := &fakeNoteWriter{}
	result := NewService(writer).Import(context.Background(), []string{path}, Input{NewNotebookName: importString("Not created")})
	if result.Error != nil || len(result.Imported) != 0 || len(result.Failures) != 1 {
		t.Fatalf("invalid structured result = %#v", result)
	}
	if result.Failures[0].Code != FailureCodeStructured || result.Failures[0].RecordNumber != 2 {
		t.Fatalf("invalid structured failure = %#v", result.Failures[0])
	}
	if len(writer.createdNotes) != 0 || len(writer.createdNotebooks) != 0 {
		t.Fatalf("invalid structured file was persisted = %#v", writer)
	}
}

func TestImportStructuredJSONRejectsDuplicateAndUnknownKeys(t *testing.T) {
	tests := []string{
		`{"format":"atlasnote-notes","format":"atlasnote-notes","version":1,"notes":[]}`,
		`{"format":"atlasnote-notes","version":1,"notes":[],"extra":true}`,
		`{"format":"atlasnote-notes","version":1,"notes":[{"title":"x","title":"y","content":"body"}]}`,
	}
	for index, content := range tests {
		t.Run(fmt.Sprintf("case-%d", index+1), func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "invalid.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("write JSON: %v", err)
			}
			writer := &fakeNoteWriter{}
			result := NewService(writer).Import(context.Background(), []string{path}, Input{})
			if result.Error != nil || len(result.Failures) != 1 || result.Failures[0].Code != FailureCodeStructured {
				t.Fatalf("duplicate/unknown result = %#v", result)
			}
			if len(writer.createdNotes) != 0 {
				t.Fatalf("invalid JSON was persisted = %#v", writer.createdNotes)
			}
		})
	}
}

func TestStructuredSourceLargerThanLegacyLimit(t *testing.T) {
	for _, ext := range []string{"json", "csv"} {
		t.Run(ext, func(t *testing.T) {
			body := strings.Repeat("x", int(MaxSourceBytes))
			payload := "title,content\nfirst," + body + "\nsecond,valid\n"
			if ext == "json" {
				data, err := json.Marshal(map[string]any{"format": structuredFormat, "version": 1, "notes": []map[string]string{{"title": "first", "content": body}, {"title": "second", "content": "valid"}}})
				if err != nil {
					t.Fatal(err)
				}
				payload = string(data)
			}
			path := writeImportSource(t, t.TempDir(), "notes."+ext, payload)
			writer := &fakeNoteWriter{}
			result := NewService(writer).Import(context.Background(), []string{path}, Input{})
			if result.Error != nil || len(result.Failures) != 0 || len(result.Imported) != 2 {
				t.Fatalf("structured input rejected: %#v", result)
			}
		})
	}
}
