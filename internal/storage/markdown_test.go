package storage

import (
	"context"
	"testing"
)

func TestMarkdownStoreRejectsUnsafeID(t *testing.T) {
	t.Parallel()

	store, err := NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	if err := store.Write(context.Background(), "../secret", "content"); err == nil {
		t.Fatal("expected unsafe id to fail")
	}
}

func TestMarkdownStoreRejectsUnsafeOperationID(t *testing.T) {
	t.Parallel()

	store, err := NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}

	if err := store.WriteTemp(context.Background(), "safe", "../operation", "content"); err == nil {
		t.Fatal("expected unsafe operation id to fail")
	}
}

func TestMarkdownStoreListManagedFilesReturnsSnapshot(t *testing.T) {
	t.Parallel()

	store, err := NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	if err := store.Write(context.Background(), "note-a", "content"); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	if err := store.WriteTemp(context.Background(), "note-b", "operation", "pending"); err != nil {
		t.Fatalf("write temp markdown: %v", err)
	}

	files, err := store.ListManagedFiles(context.Background())
	if err != nil {
		t.Fatalf("list managed files: %v", err)
	}
	for _, name := range []string{"note-a.md", "note-b.md.operation.tmp"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("managed files missing %q: %#v", name, files)
		}
	}
}
