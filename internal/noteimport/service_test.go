package noteimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/note"
)

type fakeNoteWriter struct {
	createdNotes     []note.CreateInput
	createdNotebooks []note.NotebookCreateInput
	createErrorAt    int
	createErr        error
	notebookErr      error
}

func (f *fakeNoteWriter) Create(_ context.Context, input note.CreateInput) (note.Note, error) {
	f.createdNotes = append(f.createdNotes, input)
	if f.createErr != nil && len(f.createdNotes) == f.createErrorAt {
		return note.Note{}, f.createErr
	}
	return note.Note{ID: fmt.Sprintf("note-%d", len(f.createdNotes)), Title: input.Title}, nil
}

func (f *fakeNoteWriter) CreateNotebook(_ context.Context, input note.NotebookCreateInput) (note.Notebook, error) {
	f.createdNotebooks = append(f.createdNotebooks, input)
	if f.notebookErr != nil {
		return note.Notebook{}, f.notebookErr
	}
	return note.Notebook{ID: fmt.Sprintf("notebook-%d", len(f.createdNotebooks)), Name: input.Name}, nil
}

func TestImportCreatesOneNotePerSupportedFileAndPreservesMarkdown(t *testing.T) {
	directory := t.TempDir()
	markdownPath := writeImportSource(t, directory, "heading.MD", "\uFEFF# Markdown title\r\nbody\r\n")
	textPath := writeImportSource(t, directory, "plain.TXT", "plain text")
	htmlPath := writeImportSource(t, directory, "page.html", "<head><title>HTML title</title></head><p>Hello <b>world</b></p>")
	unsupportedPath := writeImportSource(t, directory, "skip.pdf", "not supported")

	writer := &fakeNoteWriter{}
	result := NewService(writer).Import(context.Background(), []string{markdownPath, textPath, htmlPath, unsupportedPath}, Input{})

	if result.Error != nil {
		t.Fatalf("import error = %#v", result.Error)
	}
	if len(result.Imported) != 3 || len(result.Failures) != 1 {
		t.Fatalf("import result = %#v", result)
	}
	if result.Failures[0].SourceName != "skip.pdf" || result.Failures[0].Code != FailureCodeUnsupportedFile {
		t.Fatalf("unsupported result = %#v", result.Failures[0])
	}
	if len(writer.createdNotes) != 3 {
		t.Fatalf("created notes = %#v", writer.createdNotes)
	}
	if writer.createdNotes[0].Title != "Markdown title" || writer.createdNotes[0].Content != "# Markdown title\r\nbody\r\n" {
		t.Fatalf("markdown import = %#v", writer.createdNotes[0])
	}
	if writer.createdNotes[1].Title != "plain" || writer.createdNotes[1].Content != "plain text" {
		t.Fatalf("text import = %#v", writer.createdNotes[1])
	}
	if writer.createdNotes[2].Title != "HTML title" || writer.createdNotes[2].Content != "Hello **world**" {
		t.Fatalf("HTML import = %#v", writer.createdNotes[2])
	}
	for _, item := range result.Imported {
		if strings.Contains(item.SourceName, directory) {
			t.Fatalf("source path leaked from import result: %#v", item)
		}
	}
	for _, failure := range result.Failures {
		if strings.Contains(failure.SourceName, directory) {
			t.Fatalf("source path leaked from import failure: %#v", failure)
		}
	}
}

func TestImportCreatesNewNotebookOnlyAfterFirstConvertibleFile(t *testing.T) {
	directory := t.TempDir()
	invalidPath := writeImportSource(t, directory, "invalid.bin", "invalid")
	validPath := writeImportSource(t, directory, "valid.md", "content")

	writer := &fakeNoteWriter{}
	result := NewService(writer).Import(context.Background(), []string{invalidPath, validPath}, Input{NewNotebookName: importString("Imported")})
	if result.Error != nil || result.CreatedNotebook == nil {
		t.Fatalf("import result = %#v", result)
	}
	if len(writer.createdNotebooks) != 1 || writer.createdNotebooks[0].Name != "Imported" {
		t.Fatalf("created notebooks = %#v", writer.createdNotebooks)
	}
	if len(writer.createdNotes) != 1 || writer.createdNotes[0].NotebookID == nil || *writer.createdNotes[0].NotebookID != result.CreatedNotebook.ID {
		t.Fatalf("new-notebook destination = %#v", writer.createdNotes)
	}

	allInvalid := NewService(&fakeNoteWriter{})
	allInvalidResult := allInvalid.Import(context.Background(), []string{invalidPath}, Input{NewNotebookName: importString("Not created")})
	if allInvalidResult.CreatedNotebook != nil {
		t.Fatalf("new notebook must not be created without a convertible source: %#v", allInvalidResult)
	}
}

func TestImportStopsAfterPersistenceFailureAndKeepsCompletedNotes(t *testing.T) {
	directory := t.TempDir()
	first := writeImportSource(t, directory, "first.md", "first")
	second := writeImportSource(t, directory, "second.md", "second")
	third := writeImportSource(t, directory, "third.md", "third")
	writer := &fakeNoteWriter{createErrorAt: 2, createErr: errors.New("disk unavailable")}

	result := NewService(writer).Import(context.Background(), []string{first, second, third}, Input{})
	if result.Error == nil || result.Error.Code != ErrorCodePersistence {
		t.Fatalf("persistence failure result = %#v", result)
	}
	if len(result.Imported) != 1 || len(result.Failures) != 1 || len(writer.createdNotes) != 2 {
		t.Fatalf("partial import result = %#v, calls = %#v", result, writer.createdNotes)
	}
	if result.Failures[0].SourceName != "second.md" || result.Failures[0].Code != FailureCodeCreate {
		t.Fatalf("failed source = %#v", result.Failures[0])
	}
}

func TestImportRejectsInvalidDestinationAndInvalidSourceEncoding(t *testing.T) {
	directory := t.TempDir()
	valid := writeImportSource(t, directory, "valid.md", "body")
	invalidEncoding := filepath.Join(directory, "invalid.txt")
	if err := os.WriteFile(invalidEncoding, []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatalf("write invalid encoding: %v", err)
	}
	notebookID := "notebook-1"
	writer := &fakeNoteWriter{}
	invalidDestination := NewService(writer).Import(context.Background(), []string{valid}, Input{NotebookID: &notebookID, NewNotebookName: importString("also new")})
	if invalidDestination.Error == nil || invalidDestination.Error.Code != ErrorCodeInvalidDestination || len(writer.createdNotes) != 0 {
		t.Fatalf("invalid destination = %#v", invalidDestination)
	}
	emptyName := "   "
	emptyNewNotebook := NewService(&fakeNoteWriter{}).Import(context.Background(), []string{valid}, Input{NewNotebookName: &emptyName})
	if emptyNewNotebook.Error == nil || emptyNewNotebook.Error.Code != ErrorCodeInvalidDestination {
		t.Fatalf("empty new notebook name = %#v", emptyNewNotebook)
	}

	result := NewService(&fakeNoteWriter{}).Import(context.Background(), []string{invalidEncoding}, Input{})
	if result.Error != nil || len(result.Failures) != 1 || result.Failures[0].Code != FailureCodeEncoding {
		t.Fatalf("invalid encoding result = %#v", result)
	}
}

func TestImportUsesSelectedTitleMode(t *testing.T) {
	directory := t.TempDir()
	htmlPath := writeImportSource(t, directory, "html-file.html", "<head><title>HTML metadata</title></head><body><h1>HTML heading</h1><p>body</p></body>")
	markdownPath := writeImportSource(t, directory, "markdown-file.md", "# Markdown heading\nbody")
	plainTextPath := writeImportSource(t, directory, "plain-file.txt", "plain text")

	testCases := []struct {
		name     string
		path     string
		mode     TitleMode
		expected string
	}{
		{name: "auto HTML heading", path: htmlPath, mode: TitleModeAuto, expected: "HTML heading"},
		{name: "filename", path: htmlPath, mode: TitleModeFilename, expected: "html-file"},
		{name: "HTML heading", path: htmlPath, mode: TitleModeHeading, expected: "HTML heading"},
		{name: "HTML metadata", path: htmlPath, mode: TitleModeMetadata, expected: "HTML metadata"},
		{name: "missing heading falls back to filename", path: plainTextPath, mode: TitleModeHeading, expected: "plain-file"},
		{name: "missing metadata falls back to filename", path: markdownPath, mode: TitleModeMetadata, expected: "markdown-file"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writer := &fakeNoteWriter{}
			result := NewService(writer).Import(context.Background(), []string{testCase.path}, Input{TitleMode: testCase.mode})
			if result.Error != nil || len(result.Imported) != 1 || len(writer.createdNotes) != 1 {
				t.Fatalf("import result = %#v, created notes = %#v", result, writer.createdNotes)
			}
			if got := writer.createdNotes[0].Title; got != testCase.expected {
				t.Fatalf("title = %q, want %q", got, testCase.expected)
			}
		})
	}
}

func TestImportRejectsInvalidTitleModeBeforeReadingOrPersisting(t *testing.T) {
	writer := &fakeNoteWriter{}
	service := NewService(writer)
	service.readSource = func(string) ([]byte, error) {
		t.Fatal("source must not be read for an invalid title mode")
		return nil, nil
	}

	result := service.Import(context.Background(), []string{"valid.md"}, Input{TitleMode: TitleMode("unexpected")})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidTitleMode {
		t.Fatalf("invalid title mode result = %#v", result)
	}
	if len(writer.createdNotes) != 0 || len(writer.createdNotebooks) != 0 {
		t.Fatalf("invalid title mode must not persist: %#v", writer)
	}
}

func TestReadCandidateRejectsOversizedSource(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "large.md")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(MaxSourceBytes)+1)), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	_, failure := NewService(&fakeNoteWriter{}).readCandidate(path, TitleModeAuto)
	if failure == nil || failure.Code != FailureCodeTooLarge {
		t.Fatalf("large source failure = %#v", failure)
	}
}

func TestImportRejectsConcurrentBatch(t *testing.T) {
	service := NewService(&fakeNoteWriter{})
	started := make(chan struct{})
	release := make(chan struct{})
	service.readSource = func(string) ([]byte, error) {
		close(started)
		<-release
		return []byte("body"), nil
	}

	completed := make(chan Result, 1)
	go func() {
		completed <- service.Import(context.Background(), []string{"first.md"}, Input{})
	}()
	<-started

	busy := service.Import(context.Background(), []string{"second.md"}, Input{})
	if busy.Error == nil || busy.Error.Code != ErrorCodeBusy || !busy.Error.Retryable {
		t.Fatalf("concurrent import result = %#v", busy)
	}

	close(release)
	result := <-completed
	if result.Error != nil || len(result.Imported) != 1 {
		t.Fatalf("first import result = %#v", result)
	}
}

func writeImportSource(t *testing.T, directory string, name string, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write source %s: %v", name, err)
	}
	return path
}

func importString(value string) *string {
	return &value
}
