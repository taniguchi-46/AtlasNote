package organize

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

// BenchmarkAnalyzeOrganization uses a disposable SQLite and Markdown vault.
// ATLASNOTE_BENCH_NOTES and ATLASNOTE_BENCH_BODY_BYTES control the fixture size.
func BenchmarkAnalyzeOrganization(b *testing.B) {
	ctx := context.Background()
	root := b.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		b.Fatal(err)
	}
	repository := note.NewRepository(db)
	count := benchmarkPositiveEnv(b, "ATLASNOTE_BENCH_NOTES", 1000)
	bodyBytes := benchmarkPositiveEnv(b, "ATLASNOTE_BENCH_BODY_BYTES", 2048)
	baseTime := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("organization-benchmark-%06d", i)
		title := fmt.Sprintf("Synthetic note %06d", i)
		content := fmt.Sprintf("# %s\n\n%s\n\nunique%06d", title,
			strings.Repeat(fmt.Sprintf("topic%03d reference text. ", i/20), bodyBytes/25+1), i)
		if len(content) > bodyBytes {
			content = content[:bodyBytes]
		}
		record := note.Record{
			ID: id, Title: title, ContentPath: id + ".md", Revision: 1,
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
			UpdatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := repository.Create(ctx, record); err != nil {
			b.Fatalf("create note %d: %v", i, err)
		}
		if err := markdown.Write(ctx, id, content); err != nil {
			b.Fatalf("write note %d: %v", i, err)
		}
	}
	organizer := NewService(note.NewService(repository, markdown))
	b.ReportAllocs()
	b.ReportMetric(float64(count), "fixture-notes")
	b.ReportMetric(float64(bodyBytes), "body-bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analysis, err := organizer.Analyze(ctx, "synthetic-space", AnalysisInput{Scope: "space"})
		if err != nil {
			b.Fatal(err)
		}
		if analysis.AnalyzedNotes != count {
			b.Fatalf("analyzed %d notes, want %d", analysis.AnalyzedNotes, count)
		}
		b.ReportMetric(float64(len(analysis.Candidates)), "candidates")
	}
}

func benchmarkPositiveEnv(b *testing.B, name string, fallback int) int {
	b.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		b.Fatalf("%s must be a positive integer", name)
	}
	return count
}
