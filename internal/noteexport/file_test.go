package noteexport

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomicCreatesAndOverwritesFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "note.html")
	if err := os.WriteFile(path, []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	if err := writeFileAtomic(path, []byte("new content")); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "new content" {
		t.Fatalf("destination = %q", got)
	}
	if matches, err := filepath.Glob(filepath.Join(directory, ".atlasnote-export-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, %v", matches, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat destination: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("destination permissions = %o, want 600", got)
		}
	}
}

func TestWriteFileAtomicCleansTemporaryFileAndPreservesDestinationOnReplaceFailure(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "existing-directory.html")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatalf("create destination directory: %v", err)
	}
	marker := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatalf("create destination marker: %v", err)
	}

	if err := writeFileAtomic(destination, []byte("new content")); err == nil {
		t.Fatal("writeFileAtomic() succeeded for directory destination")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("destination marker = %q, %v", got, err)
	}
	if matches, err := filepath.Glob(filepath.Join(directory, ".atlasnote-export-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files after failure = %v, %v", matches, err)
	}
}

func TestOutputPath(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		path   string
		format Format
		want   string
		code   string
	}{
		{path: filepath.Join(directory, "note"), format: FormatHTML, want: filepath.Join(directory, "note.html")},
		{path: filepath.Join(directory, "note.HTML"), format: FormatHTML, want: filepath.Join(directory, "note.HTML")},
		{path: filepath.Join(directory, "note.pdf"), format: FormatHTML, want: filepath.Join(directory, "note.pdf.html")},
		{path: filepath.Join(directory, "note"), format: FormatPDF, want: filepath.Join(directory, "note.pdf")},
		{path: "", format: FormatHTML, code: ErrorCodeInvalidInput},
		{path: filepath.Join(directory, "note"), format: "invalid", code: ErrorCodeInvalidFormat},
	}
	for _, test := range tests {
		got, apiErr := outputPath(test.path, test.format)
		if got != test.want {
			t.Errorf("outputPath(%q, %q) = %q, want %q", test.path, test.format, got, test.want)
		}
		if test.code == "" && apiErr != nil {
			t.Errorf("outputPath(%q, %q) error = %#v", test.path, test.format, apiErr)
		}
		if test.code != "" && (apiErr == nil || apiErr.Code != test.code) {
			t.Errorf("outputPath(%q, %q) error = %#v, want %q", test.path, test.format, apiErr, test.code)
		}
	}

	_, err := os.Stat(filepath.Join(directory, "never-created"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected test directory state: %v", err)
	}
}
