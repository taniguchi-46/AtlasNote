package note_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func BenchmarkRelatedNotes5000(b *testing.B) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(b.TempDir(), "related.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	store, err := storage.NewMarkdownStore(filepath.Join(b.TempDir(), "notes"))
	if err != nil {
		b.Fatal(err)
	}
	repo := note.NewRepository(db)
	documents := make([]note.SearchDocument, 0, 5000)
	now := time.Now().UTC()
	body := "合成データ " + strings.Repeat("a", 2048-len("合成データ "))
	contentHash := storage.HashContent(body)
	for i := 0; i < 5000; i++ {
		id := fmt.Sprintf("related-%06d", i)
		title := fmt.Sprintf("日本語の設計 %06d", i)
		if err := repo.Create(ctx, note.Record{ID: id, Title: title, ContentPath: id + ".md", Revision: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
			b.Fatal(err)
		}
		if err := store.Write(ctx, id, body); err != nil {
			b.Fatal(err)
		}
		documents = append(documents, note.SearchDocument{NoteID: id, Title: title, Body: body, Revision: 1, ContentHash: contentHash})
	}
	if err := repo.ReplaceSearchIndex(ctx, documents); err != nil {
		b.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO note_link_state(note_id,indexed_revision,content_hash,content_mtime_ns,indexed_at) SELECT id, revision, ?, 0, ? FROM notes`, contentHash, now.Format(time.RFC3339Nano)); err != nil {
		b.Fatal(err)
	}
	service := note.NewService(repo, store)
	b.ReportAllocs()
	b.ReportMetric(5000, "fixture-notes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := service.RelatedNotes(ctx, note.RelatedNoteInput{NoteID: "related-000000", Limit: 20})
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Items) != 20 {
			b.Fatalf("candidate count = %d", len(result.Items))
		}
	}
}
