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
