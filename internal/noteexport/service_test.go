package noteexport

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/note"
)

type stubNoteReader struct {
	note  note.Note
	err   error
	calls int
}

type stubProtectionReader struct {
	protected bool
	locked    bool
	err       error
	calls     int
}

func (reader *stubProtectionReader) NoteLockStatus(context.Context, string) (bool, bool, string, error) {
	reader.calls++
	return reader.protected, reader.locked, "note", reader.err
}

func (reader *stubNoteReader) Get(context.Context, string) (note.Note, error) {
	reader.calls++
	return reader.note, reader.err
}

func TestServiceExportHTMLUsesCanonicalNoteSnapshot(t *testing.T) {
	reader := &stubNoteReader{note: note.Note{
		ID:       "0123456789abcdef0123456789abcdef",
		Title:    `正本 <タイトル>`,
		Content:  "# 本文",
		Revision: 7,
	}}
	service := NewService(reader, nil)
	path := filepath.Join(t.TempDir(), "chosen-name")
	input := validHTMLInput()
	input.NoteID = reader.note.ID
	input.ExpectedRevision = reader.note.Revision
	input.Markdown = reader.note.Content
	input.Title = "古いタイトル"
	input.HTMLFragment = "<h1>本文</h1><p>日本語</p>"

	result, err := service.Export(context.Background(), path, input)
	if err != nil || result.Error != nil {
		t.Fatalf("Export() = %#v, %v", result, err)
	}
	if result.ExportedName != "chosen-name.html" || result.Cancelled {
		t.Fatalf("Export() result = %#v", result)
	}
	content, err := os.ReadFile(path + ".html")
	if err != nil {
		t.Fatalf("read exported HTML: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, `<title>正本 &lt;タイトル&gt;</title>`) {
		t.Fatalf("canonical title missing:\n%s", output)
	}
	if strings.Contains(output, "古いタイトル") {
		t.Fatalf("dialog-only input title leaked into document:\n%s", output)
	}
	if strings.Count(output, "正本") != 1 {
		t.Fatalf("canonical title was duplicated into body:\n%s", output)
	}
	if reader.calls != 1 {
		t.Fatalf("reader calls = %d, want 1", reader.calls)
	}
}

func TestServiceExportPDFAndCorrectsDifferentExtension(t *testing.T) {
	pdf := []byte("%PDF-1.7\nbody\n%%EOF\n")
	reader := &stubNoteReader{note: note.Note{
		ID:       "0123456789abcdef0123456789abcdef",
		Title:    "PDF",
		Content:  "本文",
		Revision: 3,
	}}
	service := NewService(reader, nil)
	path := filepath.Join(t.TempDir(), "report.html")
	input := validPDFInput()
	input.NoteID = reader.note.ID
	input.ExpectedRevision = reader.note.Revision
	input.Markdown = reader.note.Content
	input.PDFBase64 = base64.StdEncoding.EncodeToString(pdf)

	result, err := service.Export(context.Background(), path, input)
	if err != nil || result.Error != nil || result.ExportedName != "report.html.pdf" {
		t.Fatalf("Export() = %#v, %v", result, err)
	}
	got, err := os.ReadFile(path + ".pdf")
	if err != nil {
		t.Fatalf("read PDF: %v", err)
	}
	if string(got) != string(pdf) {
		t.Fatalf("PDF bytes = %q, want %q", got, pdf)
	}
}

func TestServiceExportRejectsInvalidOrChangedSnapshotWithoutWriting(t *testing.T) {
	baseNote := note.Note{
		ID:       "0123456789abcdef0123456789abcdef",
		Title:    "Note",
		Content:  "saved",
		Revision: 4,
	}
	tests := []struct {
		name        string
		reader      *stubNoteReader
		mutate      func(*Input)
		wantCode    string
		readerCalls int
	}{
		{
			name:   "invalid input before read",
			reader: &stubNoteReader{note: baseNote},
			mutate: func(input *Input) {
				input.ExpectedRevision = 0
			},
			wantCode: ErrorCodeInvalidInput, readerCalls: 0,
		},
		{
			name:     "not found",
			reader:   &stubNoteReader{err: note.ErrNotFound},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeNoteNotFound, readerCalls: 1,
		},
		{
			name:     "wrapped locked error",
			reader:   &stubNoteReader{err: errors.Join(errors.New("read failed"), contentlock.ErrLocked)},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeLocked, readerCalls: 1,
		},
		{
			name:     "locked result",
			reader:   &stubNoteReader{note: withNote(baseNote, func(current *note.Note) { current.Locked = true })},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeLocked, readerCalls: 1,
		},
		{
			name:     "revision changed",
			reader:   &stubNoteReader{note: withNote(baseNote, func(current *note.Note) { current.Revision++ })},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeStale, readerCalls: 1,
		},
		{
			name:     "source changed at same revision",
			reader:   &stubNoteReader{note: withNote(baseNote, func(current *note.Note) { current.Content = "external edit" })},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeStale, readerCalls: 1,
		},
		{
			name:     "protected confirmation required",
			reader:   &stubNoteReader{note: withNote(baseNote, func(current *note.Note) { current.Protected = true })},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeProtectedConfirmationRequired, readerCalls: 1,
		},
		{
			name:     "unavailable",
			reader:   &stubNoteReader{err: errors.New("storage unavailable")},
			mutate:   func(*Input) {},
			wantCode: ErrorCodeUnavailable, readerCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validHTMLInput()
			input.NoteID = baseNote.ID
			input.ExpectedRevision = baseNote.Revision
			input.Markdown = baseNote.Content
			test.mutate(&input)
			path := filepath.Join(t.TempDir(), "must-not-exist.html")
			result, err := NewService(test.reader, nil).Export(context.Background(), path, input)
			if err != nil || result.Error == nil || result.Error.Code != test.wantCode {
				t.Fatalf("Export() = %#v, %v; want %q", result, err, test.wantCode)
			}
			if test.reader.calls != test.readerCalls {
				t.Fatalf("reader calls = %d, want %d", test.reader.calls, test.readerCalls)
			}
			if _, statError := os.Stat(path); !errors.Is(statError, os.ErrNotExist) {
				t.Fatalf("rejected export created a file: %v", statError)
			}
		})
	}
}

func TestServiceExportAllowsConfirmedProtectedNote(t *testing.T) {
	current := note.Note{
		ID: "0123456789abcdef0123456789abcdef", Title: "Protected",
		Content: "secret", Revision: 1, Protected: true,
	}
	input := validHTMLInput()
	input.NoteID = current.ID
	input.ExpectedRevision = current.Revision
	input.Markdown = current.Content
	input.AllowPlaintextProtected = true
	path := filepath.Join(t.TempDir(), "protected.html")

	result, err := NewService(&stubNoteReader{note: current}, nil).Export(context.Background(), path, input)
	if err != nil || result.Error != nil {
		t.Fatalf("confirmed protected Export() = %#v, %v", result, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("confirmed protected export missing: %v", err)
	}
}

func TestServiceExportFailsClosedOnProtectionStatus(t *testing.T) {
	current := note.Note{
		ID: "0123456789abcdef0123456789abcdef", Title: "Protected",
		Content: "secret", Revision: 1,
	}
	input := validHTMLInput()
	input.NoteID = current.ID
	input.ExpectedRevision = current.Revision
	input.Markdown = current.Content

	tests := []struct {
		name       string
		protection *stubProtectionReader
		wantCode   string
	}{
		{
			name:       "status unavailable",
			protection: &stubProtectionReader{err: errors.New("lock status unavailable")},
			wantCode:   ErrorCodeUnavailable,
		},
		{
			name:       "protected status overrides missing annotation",
			protection: &stubProtectionReader{protected: true},
			wantCode:   ErrorCodeProtectedConfirmationRequired,
		},
		{
			name:       "locked status overrides missing annotation",
			protection: &stubProtectionReader{protected: true, locked: true},
			wantCode:   ErrorCodeLocked,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "must-not-exist.html")
			result, err := NewService(&stubNoteReader{note: current}, test.protection).Export(
				context.Background(),
				path,
				input,
			)
			if err != nil || result.Error == nil || result.Error.Code != test.wantCode {
				t.Fatalf("Export() = %#v, %v; want %q", result, err, test.wantCode)
			}
			if test.protection.calls != 1 {
				t.Fatalf("protection status calls = %d, want 1", test.protection.calls)
			}
			if _, statError := os.Stat(path); !errors.Is(statError, os.ErrNotExist) {
				t.Fatalf("rejected export created a file: %v", statError)
			}
		})
	}
}

func TestServiceExportRejectsEmptyPathAndCancelledContext(t *testing.T) {
	reader := &stubNoteReader{note: note.Note{Content: "body", Revision: 1}}
	input := validHTMLInput()
	input.Markdown = reader.note.Content

	result, err := NewService(reader, nil).Export(context.Background(), " ", input)
	if err != nil || result.Error == nil || result.Error.Code != ErrorCodeInvalidInput || reader.calls != 0 {
		t.Fatalf("empty path Export() = %#v, %v, calls %d", result, err, reader.calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = NewService(reader, nil).Export(ctx, filepath.Join(t.TempDir(), "cancel.html"), input)
	if err != nil || result.Error == nil || result.Error.Code != ErrorCodeUnavailable || reader.calls != 0 {
		t.Fatalf("cancelled Export() = %#v, %v, calls %d", result, err, reader.calls)
	}
}

func validHTMLInput() Input {
	return Input{
		NoteID:           "0123456789abcdef0123456789abcdef",
		ExpectedRevision: 1,
		Title:            "Note",
		Markdown:         "body",
		HTMLFragment:     "<p>body</p>",
		Format:           FormatHTML,
	}
}

func validPDFInput() Input {
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\n%%EOF"))
	return Input{
		NoteID:           "0123456789abcdef0123456789abcdef",
		ExpectedRevision: 1,
		Title:            "Note",
		Markdown:         "body",
		PDFBase64:        pdf,
		Format:           FormatPDF,
	}
}

func withNote(current note.Note, mutate func(*note.Note)) note.Note {
	mutate(&current)
	return current
}
