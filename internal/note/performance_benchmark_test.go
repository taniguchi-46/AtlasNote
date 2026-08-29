package note_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

const defaultLargeNoteBenchmarkCount = 5000

type largeNoteBenchmarkFixture struct {
	ctx       context.Context
	noteCount int
	service   *note.Service
}

func BenchmarkLargeNoteListPage(b *testing.B) {
	for _, testCase := range []struct {
		name string
		page int
	}{
		{name: "first-page", page: 1},
		{name: "deep-page", page: 50},
	} {
		b.Run(testCase.name, func(b *testing.B) {
			fixture := newLargeNoteBenchmarkFixture(b)
			page := testCase.page
			lastPage := (fixture.noteCount + note.DefaultNoteListPageSize - 1) / note.DefaultNoteListPageSize
			if page > lastPage {
				page = lastPage
			}
			b.ReportAllocs()
			b.ReportMetric(float64(fixture.noteCount), "fixture-notes")
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := fixture.service.ListPage(fixture.ctx, note.NoteListInput{
					Page:     page,
					PageSize: note.DefaultNoteListPageSize,
				})
				if err != nil {
					b.Fatalf("list page: %v", err)
				}
				if len(result.Items) == 0 {
					b.Fatal("list page returned no items")
				}
			}
		})
	}
}

func BenchmarkLargeNoteSearch(b *testing.B) {
	fixture := newLargeNoteBenchmarkFixture(b)
	b.ReportAllocs()
	b.ReportMetric(float64(fixture.noteCount), "fixture-notes")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := fixture.service.Search(fixture.ctx, note.SearchInput{
			Query:    "benchmark",
			Page:     1,
			PageSize: note.DefaultSearchPageSize,
		})
		if err != nil {
			b.Fatalf("search notes: %v", err)
		}
		if result.Error != nil {
			b.Fatalf("search result error: %#v", result.Error)
		}
		if len(result.Items) == 0 {
			b.Fatal("search returned no items")
		}
	}
}

func BenchmarkLargeNoteRecovery(b *testing.B) {
	fixture := newLargeNoteBenchmarkFixture(b)
	b.ReportAllocs()
	b.ReportMetric(float64(fixture.noteCount), "fixture-notes")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		report, err := fixture.service.Recover(fixture.ctx)
		if err != nil {
			b.Fatalf("recover notes: %v", err)
		}
		if len(report.MissingNotes) != 0 {
			b.Fatalf("recovery reported missing notes: %d", len(report.MissingNotes))
		}
	}
}

func newLargeNoteBenchmarkFixture(b *testing.B) largeNoteBenchmarkFixture {
	b.Helper()

	ctx := context.Background()
	tempDir := b.TempDir()
	db, err := database.Open(ctx, filepath.Join(tempDir, "atlasnote.db"))
	if err != nil {
		b.Fatalf("open benchmark database: %v", err)
	}
	b.Cleanup(func() {
		_ = db.Close()
	})

	store, err := storage.NewMarkdownStore(filepath.Join(tempDir, "notes"))
	if err != nil {
		b.Fatalf("create benchmark markdown store: %v", err)
	}
	repository := note.NewRepository(db)
	noteCount := largeNoteBenchmarkCount(b)
	documents := make([]note.SearchDocument, 0, noteCount)
	baseTime := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	for i := 0; i < noteCount; i++ {
		id := fmt.Sprintf("benchmark-note-%06d", i)
		now := baseTime.Add(time.Duration(i) * time.Second)
		title := fmt.Sprintf("Benchmark note %06d", i)
		content := fmt.Sprintf("# %s\n\nbenchmark keyword content %06d", title, i)
		record := note.Record{
			ID:          id,
			Title:       title,
			ContentPath: id + ".md",
			Revision:    1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := repository.Create(ctx, record); err != nil {
			b.Fatalf("create benchmark note %q: %v", id, err)
		}
		if err := store.Write(ctx, id, content); err != nil {
			b.Fatalf("write benchmark markdown %q: %v", id, err)
		}
		contentMTime, err := store.ModTime(ctx, id)
		if err != nil {
			b.Fatalf("stat benchmark markdown %q: %v", id, err)
		}
		documents = append(documents, note.SearchDocument{
			NoteID:       id,
			Title:        title,
			Body:         content,
			Revision:     1,
			ContentHash:  storage.HashContent(content),
			ContentMTime: contentMTime,
			IndexedAt:    now,
		})
	}
	if err := repository.ReplaceSearchIndex(ctx, documents); err != nil {
		b.Fatalf("build benchmark search index: %v", err)
	}

	return largeNoteBenchmarkFixture{
		ctx:       ctx,
		noteCount: noteCount,
		service:   note.NewService(repository, store),
	}
}

func largeNoteBenchmarkCount(b *testing.B) int {
	b.Helper()

	value := strings.TrimSpace(os.Getenv("ATLASNOTE_BENCH_NOTES"))
	if value == "" {
		return defaultLargeNoteBenchmarkCount
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		b.Fatalf("ATLASNOTE_BENCH_NOTES must be a positive integer, got %q", value)
	}
	return count
}
