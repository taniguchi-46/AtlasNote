package organize

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

func TestAnalyzeAndApplyUsesRevisionAndCurrentTagState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	notes := note.NewService(note.NewRepository(db), markdown)
	organizer := NewService(notes)

	_, err = notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Folder"})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	titleNote, err := notes.Create(ctx, note.CreateInput{Title: "Writer", Content: "# New title\nGoLang notes"})
	if err != nil {
		t.Fatalf("create title candidate: %v", err)
	}
	duplicateFirst, err := notes.Create(ctx, note.CreateInput{Title: "First", Content: "same\ntext"})
	if err != nil {
		t.Fatalf("create duplicate keeper: %v", err)
	}
	duplicateSecond, err := notes.Create(ctx, note.CreateInput{Title: "Second", Content: "same  \r\ntext\t"})
	if err != nil {
		t.Fatalf("create duplicate: %v", err)
	}
	emptyNote, err := notes.Create(ctx, note.CreateInput{Title: "Empty", Content: " \n\t"})
	if err != nil {
		t.Fatalf("create empty note: %v", err)
	}
	linkTarget, err := notes.Create(ctx, note.CreateInput{Title: "Link target", Content: "reference"})
	if err != nil {
		t.Fatalf("create link target: %v", err)
	}
	linkSource, err := notes.Create(ctx, note.CreateInput{
		Title:   "Link source",
		Content: "[Target](atlasnote://note/" + linkTarget.ID + ")",
	})
	if err != nil {
		t.Fatalf("create link source: %v", err)
	}
	tag, err := notes.CreateTag(ctx, note.TagCreateInput{Name: "GoLang"})
	if err != nil || tag.Tag == nil {
		t.Fatalf("create tag: %#v, %v", tag, err)
	}

	analysis, err := organizer.Analyze(ctx, "space-a", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.SessionID == "" || analysis.AnalyzedNotes != 6 {
		t.Fatalf("analysis summary = %#v", analysis)
	}
	initialSessionID := analysis.SessionID
	find := func(kind, noteID string) Candidate {
		t.Helper()
		for _, candidate := range analysis.Candidates {
			if candidate.Kind == kind && candidate.NoteID == noteID {
				return candidate
			}
		}
		t.Fatalf("candidate %s for %s not found in %#v", kind, noteID, analysis.Candidates)
		return Candidate{}
	}
	titleCandidate := find(KindTitle, titleNote.ID)
	if titleCandidate.Proposed["title"] != "New title" {
		t.Fatalf("title proposal = %#v", titleCandidate.Proposed)
	}
	if applied := organizer.Apply(ctx, "space-a", ApplyInput{SessionID: analysis.SessionID, CandidateID: titleCandidate.ID}); applied.Status != "applied" {
		t.Fatalf("title apply = %#v", applied)
	}
	analysis, err = organizer.Analyze(ctx, "space-a", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatalf("reanalyze after title apply: %v", err)
	}
	tagCandidate := find(KindTagAssignment, titleNote.ID)
	if applied := organizer.Apply(ctx, "space-a", ApplyInput{SessionID: analysis.SessionID, CandidateID: tagCandidate.ID}); applied.Status != "applied" {
		t.Fatalf("tag apply = %#v", applied)
	}
	tags, err := notes.ListNoteTags(ctx, titleNote.ID)
	if err != nil || len(tags.Tags) != 1 || tags.Tags[0].ID != tag.Tag.ID {
		t.Fatalf("note tags after apply = %#v, %v", tags, err)
	}
	var duplicateCandidate Candidate
	for _, candidate := range analysis.Candidates {
		if candidate.Kind == KindDuplicateNote {
			duplicateCandidate = candidate
			break
		}
	}
	keeperID := duplicateFirst.ID
	if duplicateSecond.ID < keeperID {
		keeperID = duplicateSecond.ID
	}
	if duplicateCandidate.NoteID == "" || duplicateCandidate.RelatedID != keeperID {
		t.Fatalf("duplicate candidate = %#v", duplicateCandidate)
	}
	if applied := organizer.Apply(ctx, "space-a", ApplyInput{SessionID: analysis.SessionID, CandidateID: duplicateCandidate.ID}); applied.Status != "applied" {
		t.Fatalf("duplicate apply = %#v", applied)
	}
	trashedNote, err := notes.Get(ctx, duplicateCandidate.NoteID)
	if err != nil || !trashedNote.IsTrashed {
		t.Fatalf("duplicate note trash state = %#v, %v", trashedNote, err)
	}
	emptyCandidate := find(KindEmptyNote, emptyNote.ID)
	if emptyCandidate.Proposed["isTrashed"] != true {
		t.Fatalf("empty proposal = %#v", emptyCandidate.Proposed)
	}
	var reciprocal Candidate
	for _, candidate := range analysis.Candidates {
		if candidate.Kind == KindReciprocalLink && candidate.NoteID == linkTarget.ID && candidate.RelatedID == linkSource.ID {
			reciprocal = candidate
			break
		}
	}
	if reciprocal.ID == "" {
		t.Fatal("reciprocal link candidate was not found")
	}
	if applied := organizer.Apply(ctx, "space-a", ApplyInput{SessionID: analysis.SessionID, CandidateID: reciprocal.ID}); applied.Status != "applied" {
		t.Fatalf("reciprocal link apply = %#v", applied)
	}
	updatedTarget, err := notes.Get(ctx, linkTarget.ID)
	if err != nil || !strings.Contains(updatedTarget.Content, "atlasnote://note/"+linkSource.ID) {
		t.Fatalf("reciprocal link content = %q, %v", updatedTarget.Content, err)
	}

	if stale := organizer.Apply(ctx, "space-other", ApplyInput{SessionID: analysis.SessionID, CandidateID: emptyCandidate.ID}); stale.Status != "stale" {
		t.Fatalf("cross-space apply = %#v", stale)
	}
	changedTitle := "Changed after review"
	currentTitleNote, err := notes.Get(ctx, titleNote.ID)
	if err != nil {
		t.Fatalf("get current title note: %v", err)
	}
	if _, err := notes.Update(ctx, titleNote.ID, note.UpdateInput{Title: &changedTitle, ExpectedRevision: &currentTitleNote.Revision}); err != nil {
		t.Fatalf("update after review: %v", err)
	}
	if conflict := organizer.Apply(ctx, "space-a", ApplyInput{SessionID: initialSessionID, CandidateID: titleCandidate.ID}); conflict.Status != "conflict" {
		t.Fatalf("stale revision apply = %#v", conflict)
	}
}

func TestAnalyzeReportsScopedProgress(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	notes := note.NewService(note.NewRepository(db), markdown)
	notebook, err := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Scoped"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := notes.Create(ctx, note.CreateInput{Title: "In scope", Content: "body", NotebookID: &notebook.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.Create(ctx, note.CreateInput{Title: "Outside", Content: "body"}); err != nil {
		t.Fatal(err)
	}
	var events []AnalysisProgress
	analysis, err := NewService(notes).Analyze(ctx, "space", AnalysisInput{Scope: "notebook", NotebookID: notebook.ID}, func(event AnalysisProgress) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.AnalyzedNotes != 1 {
		t.Fatalf("analyzed notes = %d", analysis.AnalyzedNotes)
	}
	want := []AnalysisProgress{
		{Phase: "reading", ProcessedNotes: 0, TotalNotes: 1},
		{Phase: "reading", ProcessedNotes: 1, TotalNotes: 1},
		{Phase: "proposing", ProcessedNotes: 1, TotalNotes: 1},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("progress = %#v, want %#v", events, want)
	}
}

func TestFirstMarkdownHeadingIgnoresCodeFenceAndNormalizesTrailingHashes(t *testing.T) {
	t.Parallel()
	content := "```md\n# ignored\n```\n\n###  Useful title ###\n"
	if got := firstMarkdownHeading(content); got != "Useful title" {
		t.Fatalf("firstMarkdownHeading() = %q", got)
	}
	if got := lightweightContentSignature(" same  \r\ntext\t\n"); got != "same\ntext" {
		t.Fatalf("lightweightContentSignature() = %q", got)
	}
	if !strings.Contains(firstMarkdownHeading("# x"), "x") {
		t.Fatal("heading was not detected")
	}
}

func TestApplyCandidatesGroupsSameNoteAndValidatesRelatedNotes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes, organizer := newOrganizationFixture(t)
	target, err := notes.Create(ctx, note.CreateInput{Title: "Old title", Content: "# New title\nbody"})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Source A", "Source B"} {
		if _, err := notes.Create(ctx, note.CreateInput{Title: title, Content: "[target](atlasnote://note/" + target.ID + ")"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := notes.CreateTag(ctx, note.TagCreateInput{Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := notes.CreateTag(ctx, note.TagCreateInput{Name: "Beta"}); err != nil {
		t.Fatal(err)
	}
	tagNote, err := notes.Create(ctx, note.CreateInput{Title: "Tags", Content: "Alpha Beta"})
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatal(err)
	}
	targetBefore, _ := notes.Get(ctx, target.ID)
	selected := make([]string, 0)
	for _, candidate := range analysis.Candidates {
		if candidate.NoteID == target.ID && (candidate.Kind == KindTitle || candidate.Kind == KindReciprocalLink) {
			selected = append(selected, candidate.ID)
		}
	}
	if len(selected) != 3 {
		t.Fatalf("target candidates = %d, want title plus two reciprocal links", len(selected))
	}
	results := organizer.ApplyCandidates(ctx, "space", ApplyCandidatesInput{SessionID: analysis.SessionID, CandidateIDs: selected})
	for _, result := range results {
		if result.Status != "applied" {
			t.Fatalf("same-note batch result = %#v", result)
		}
	}
	targetAfter, err := notes.Get(ctx, target.ID)
	if err != nil || targetAfter.Revision != targetBefore.Revision+1 || targetAfter.Title != "New title" {
		t.Fatalf("combined title/link update = %#v, %v", targetAfter, err)
	}
	if !strings.Contains(targetAfter.Content, "atlasnote://note/") || strings.Count(targetAfter.Content, "[関連ノート]") != 2 {
		t.Fatalf("reciprocal links were not combined: %q", targetAfter.Content)
	}

	tagRevision := tagNote.Revision
	tagCandidates := make([]string, 0)
	for _, candidate := range analysis.Candidates {
		if candidate.Kind == KindTagAssignment && candidate.NoteID == tagNote.ID {
			tagCandidates = append(tagCandidates, candidate.ID)
		}
	}
	if len(tagCandidates) != 2 {
		t.Fatalf("tag candidates = %d, want 2", len(tagCandidates))
	}
	results = organizer.ApplyCandidates(ctx, "space", ApplyCandidatesInput{SessionID: analysis.SessionID, CandidateIDs: tagCandidates})
	for _, result := range results {
		if result.Status != "applied" {
			t.Fatalf("combined tag result = %#v", result)
		}
	}
	gotTags, err := notes.ListNoteTags(ctx, tagNote.ID)
	if err != nil || len(gotTags.Tags) != 2 {
		t.Fatalf("combined tags = %#v, %v", gotTags, err)
	}
	unchangedTagNote, _ := notes.Get(ctx, tagNote.ID)
	if unchangedTagNote.Revision != tagRevision {
		t.Fatalf("tag assignment changed note revision: %d -> %d", tagRevision, unchangedTagNote.Revision)
	}
}

func TestDuplicateKeeperAndReciprocalTargetMustRemainCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes, organizer := newOrganizationFixture(t)
	first, err := notes.Create(ctx, note.CreateInput{Title: "First", Content: "same text"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := notes.Create(ctx, note.CreateInput{Title: "Second", Content: "same text"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := notes.Create(ctx, note.CreateInput{Title: "Target", Content: "body"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := notes.Create(ctx, note.CreateInput{Title: "Source", Content: "[target](atlasnote://note/" + target.ID + ")"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatal(err)
	}
	keeperID := first.ID
	duplicateID := second.ID
	if second.ID < first.ID {
		keeperID, duplicateID = second.ID, first.ID
	}
	var duplicateCandidate, reciprocalCandidate Candidate
	for _, candidate := range analysis.Candidates {
		if candidate.Kind == KindDuplicateNote && candidate.NoteID == duplicateID && candidate.RelatedID == keeperID {
			duplicateCandidate = candidate
		}
		if candidate.Kind == KindReciprocalLink && candidate.NoteID == target.ID && candidate.RelatedID == source.ID {
			reciprocalCandidate = candidate
		}
	}
	if duplicateCandidate.ID == "" || reciprocalCandidate.ID == "" {
		t.Fatal("expected duplicate and reciprocal candidates")
	}
	keeper, _ := notes.Get(ctx, keeperID)
	trashed := true
	if _, err := notes.Update(ctx, keeperID, note.UpdateInput{IsTrashed: &trashed, ExpectedRevision: &keeper.Revision}); err != nil {
		t.Fatal(err)
	}
	if got := organizer.Apply(ctx, "space", ApplyInput{SessionID: analysis.SessionID, CandidateID: duplicateCandidate.ID}); got.Status != "conflict" {
		t.Fatalf("duplicate with trashed keeper = %#v", got)
	}
	stillActive, _ := notes.Get(ctx, duplicateID)
	if stillActive.IsTrashed {
		t.Fatal("duplicate was trashed after its keeper disappeared")
	}

	targetNow, _ := notes.Get(ctx, target.ID)
	if _, err := notes.Update(ctx, target.ID, note.UpdateInput{IsTrashed: &trashed, ExpectedRevision: &targetNow.Revision}); err != nil {
		t.Fatal(err)
	}
	if got := organizer.Apply(ctx, "space", ApplyInput{SessionID: analysis.SessionID, CandidateID: reciprocalCandidate.ID}); got.Status != "conflict" {
		t.Fatalf("reciprocal link with trashed target = %#v", got)
	}
	unchangedSource, _ := notes.Get(ctx, source.ID)
	if strings.Contains(unchangedSource.Content, "atlasnote://note/"+source.ID) {
		t.Fatalf("reciprocal link was appended after target changed: %q", unchangedSource.Content)
	}
}

func TestAnalysisScopesAndNotebookMovementCandidates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes, organizer := newOrganizationFixture(t)
	parent, _ := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Parent"})
	child, _ := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Child", ParentID: &parent.ID})
	other, _ := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Other"})
	destination, _ := notes.CreateNotebook(ctx, note.NotebookCreateInput{Name: "Move me"})
	direct, _ := notes.Create(ctx, note.CreateInput{Title: "Direct", Content: "body", NotebookID: &parent.ID})
	childNote, _ := notes.Create(ctx, note.CreateInput{Title: "Child note", Content: "body", NotebookID: &child.ID})
	moveNote, _ := notes.Create(ctx, note.CreateInput{Title: "Move me", Content: "body", NotebookID: &other.ID})
	rootNote, _ := notes.Create(ctx, note.CreateInput{Title: "Unsorted", Content: "body"})

	for _, test := range []struct {
		input AnalysisInput
		want  int
	}{{AnalysisInput{Scope: "notebook", NotebookID: parent.ID}, 1}, {AnalysisInput{Scope: "descendants", NotebookID: parent.ID}, 2}, {AnalysisInput{Scope: "space"}, 4}} {
		analysis, err := organizer.Analyze(ctx, "space", test.input)
		if err != nil || analysis.AnalyzedNotes != test.want {
			t.Fatalf("scope %#v: analyzed=%d err=%v", test.input, analysis.AnalyzedNotes, err)
		}
		if test.input.Scope == "space" {
			var foundUnclassified, foundMove bool
			for _, candidate := range analysis.Candidates {
				foundUnclassified = foundUnclassified || candidate.Kind == KindUnclassifiedNote && candidate.NoteID == rootNote.ID
				foundMove = foundMove || candidate.Kind == KindNotebookMove && candidate.NoteID == moveNote.ID && candidate.NotebookID == destination.ID
			}
			if !foundUnclassified || !foundMove {
				t.Fatalf("missing unclassified/move suggestions: %#v", analysis.Candidates)
			}
			var move Candidate
			for _, candidate := range analysis.Candidates {
				if candidate.Kind == KindNotebookMove && candidate.NoteID == moveNote.ID {
					move = candidate
				}
			}
			if got := organizer.Apply(ctx, "space", ApplyInput{SessionID: analysis.SessionID, CandidateID: move.ID}); got.Status != "applied" {
				t.Fatalf("move candidate apply = %#v", got)
			}
			moved, _ := notes.Get(ctx, moveNote.ID)
			if moved.NotebookID == nil || *moved.NotebookID != destination.ID {
				t.Fatalf("note notebook after move = %#v", moved.NotebookID)
			}
		}
		_ = direct
		_ = childNote
	}
}

func TestLockedBodyIsNotAnalyzedAndInvalidatesSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	notes := note.NewService(note.NewRepository(db), markdown)
	locks := contentlock.NewManager(db, markdown)
	notes.SetContentLockGuard(locks)
	organizer := NewService(notes)
	created, err := notes.Create(ctx, note.CreateInput{Title: "Private", Content: "# Hidden body"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil || len(before.Candidates) == 0 {
		t.Fatalf("pre-lock analysis = %#v, %v", before, err)
	}
	if _, _, err := locks.Enable(ctx, contentlock.EnableInput{TargetType: contentlock.TargetNote, TargetID: created.ID, Passphrase: "test passphrase"}); err != nil {
		t.Fatal(err)
	}
	if _, err := locks.LockNow(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	organizer.InvalidateSessions()
	after, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatal(err)
	}
	if after.SkippedLocked != 1 || after.AnalyzedNotes != 0 || len(after.Candidates) != 0 {
		t.Fatalf("post-lock analysis leaked derived content: %#v", after)
	}
	if got := organizer.Apply(ctx, "space", ApplyInput{SessionID: before.SessionID, CandidateID: before.Candidates[0].ID}); got.Status != "stale" {
		t.Fatalf("invalidated analysis apply = %#v", got)
	}
}

func TestDiscardSessionLeavesOtherScopeApplicable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes, organizer := newOrganizationFixture(t)
	created, err := notes.Create(ctx, note.CreateInput{Title: "Old title", Content: "# New title"})
	if err != nil {
		t.Fatal(err)
	}
	spaceAnalysis, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatal(err)
	}
	noteAnalysis, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "note", NoteID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	var noteCandidate Candidate
	for _, candidate := range noteAnalysis.Candidates {
		if candidate.Kind == KindTitle {
			noteCandidate = candidate
			break
		}
	}
	if noteCandidate.ID == "" {
		t.Fatal("expected note-scoped title candidate")
	}

	organizer.DiscardSession(spaceAnalysis.SessionID)
	if result := organizer.Apply(ctx, "space", ApplyInput{SessionID: noteAnalysis.SessionID, CandidateID: noteCandidate.ID}); result.Status != "applied" {
		t.Fatalf("unrelated scope was invalidated: %#v", result)
	}
	if result := organizer.Apply(ctx, "space", ApplyInput{SessionID: spaceAnalysis.SessionID, CandidateID: spaceAnalysis.Candidates[0].ID}); result.Status != "stale" {
		t.Fatalf("discarded session remained applicable: %#v", result)
	}
}

func TestExternalAnalysisIsScopedOwnedExpiringAndPreviewOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	notes, organizer := newOrganizationFixture(t)
	publicNote, err := notes.Create(ctx, note.CreateInput{Title: "Public", Content: "# Public proposal"})
	if err != nil {
		t.Fatal(err)
	}
	privateNote, err := notes.Create(ctx, note.CreateInput{Title: "Private marker", Content: "# Private proposal marker"})
	if err != nil {
		t.Fatal(err)
	}
	access := AnalysisAccess{
		OwnerID: "mcp-session-a", ScopeRestricted: true,
		AllowedNoteIDs: map[string]bool{publicNote.ID: true}, AllowedNotebookIDs: map[string]bool{},
	}
	analysis, err := organizer.AnalyzeExternal(ctx, "space", AnalysisInput{Scope: "space"}, access)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.AnalyzedNotes != 1 || len(analysis.Candidates) == 0 {
		t.Fatalf("scoped analysis = %#v", analysis)
	}
	for _, candidate := range analysis.Candidates {
		encoded := candidate.Reason + candidate.NoteTitle + candidate.RelatedTitle
		if candidate.NoteID == privateNote.ID || candidate.RelatedID == privateNote.ID || strings.Contains(encoded, "Private") {
			t.Fatalf("external analysis exposed private note: %#v", candidate)
		}
	}
	if _, err := organizer.AnalysisSnapshot("space", "mcp-session-b", analysis.SessionID, AnalysisAccess{
		OwnerID: "mcp-session-b", ScopeRestricted: true, AllowedNoteIDs: map[string]bool{publicNote.ID: true}, AllowedNotebookIDs: map[string]bool{},
	}); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatalf("other owner snapshot error = %v", err)
	}
	if result := organizer.Apply(ctx, "space", ApplyInput{SessionID: analysis.SessionID, CandidateID: analysis.Candidates[0].ID}); result.Status != "stale" {
		t.Fatalf("external preview became applicable: %#v", result)
	}

	organizer.mu.Lock()
	session := organizer.sessions[analysis.SessionID]
	session.created = time.Now().Add(-analysisSessionTTL)
	organizer.sessions[analysis.SessionID] = session
	organizer.mu.Unlock()
	if _, err := organizer.AnalysisSnapshot("space", access.OwnerID, analysis.SessionID, access); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatalf("expired snapshot error = %v", err)
	}
}

func TestRestrictedExternalAnalysisSeparatesExistingAndVisibleLinkTargets(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatal(err)
	}
	notes := note.NewService(note.NewRepository(db), markdown)
	locks := contentlock.NewManager(db, markdown)
	t.Cleanup(locks.Close)
	notes.SetContentLockGuard(locks)
	organizer := NewService(notes)

	protectedNote, err := notes.Create(ctx, note.CreateInput{Title: "Protected target marker", Content: "protected body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := locks.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote, TargetID: protectedNote.ID, Passphrase: "protected target passphrase",
	}); err != nil {
		t.Fatal(err)
	}
	lockedNote, err := notes.Create(ctx, note.CreateInput{Title: "Locked target marker", Content: "locked body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := locks.Enable(ctx, contentlock.EnableInput{
		TargetType: contentlock.TargetNote, TargetID: lockedNote.ID, Passphrase: "locked target passphrase",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := locks.LockNow(ctx, contentlock.Target{Type: contentlock.TargetNote, ID: lockedNote.ID}); err != nil {
		t.Fatal(err)
	}
	trashedNote, err := notes.Create(ctx, note.CreateInput{Title: "Trashed target marker", Content: "trashed body"})
	if err != nil {
		t.Fatal(err)
	}
	trashed := true
	if _, err := notes.Update(ctx, trashedNote.ID, note.UpdateInput{IsTrashed: &trashed, ExpectedRevision: &trashedNote.Revision}); err != nil {
		t.Fatal(err)
	}
	outOfScopeNote, err := notes.Create(ctx, note.CreateInput{Title: "Out of scope target marker", Content: "private body"})
	if err != nil {
		t.Fatal(err)
	}
	missingID := strings.Repeat("f", 32)
	publicNote, err := notes.Create(ctx, note.CreateInput{
		Title: "Public source", Content: "[target](atlasnote://note/" + outOfScopeNote.ID + ")",
	})
	if err != nil {
		t.Fatal(err)
	}
	access := AnalysisAccess{
		OwnerID: "restricted-link-test", ScopeRestricted: true,
		AllowedNoteIDs: map[string]bool{
			publicNote.ID: true, protectedNote.ID: true, lockedNote.ID: true, trashedNote.ID: true, missingID: true,
		},
	}
	existingAnalysis, err := organizer.AnalyzeExternal(ctx, "space", AnalysisInput{Scope: "space"}, access)
	if err != nil {
		t.Fatal(err)
	}
	missingContent := "[target](atlasnote://note/" + missingID + ")"
	publicNote, err = notes.Update(ctx, publicNote.ID, note.UpdateInput{Content: &missingContent, ExpectedRevision: &publicNote.Revision})
	if err != nil {
		t.Fatal(err)
	}
	missingAnalysis, err := organizer.AnalyzeExternal(ctx, "space", AnalysisInput{Scope: "space"}, access)
	if err != nil {
		t.Fatal(err)
	}
	normalizedCandidates := func(analysis Analysis) []Candidate {
		candidates := append([]Candidate(nil), analysis.Candidates...)
		for index := range candidates {
			candidates[index].ID = ""
			candidates[index].BaseRevision = 0
			candidates[index].RelatedRevision = 0
			candidates[index].RelatedContentHash = ""
		}
		return candidates
	}
	if !reflect.DeepEqual(normalizedCandidates(existingAnalysis), normalizedCandidates(missingAnalysis)) {
		t.Fatalf("scope-out existing/missing results differ: existing=%#v missing=%#v", existingAnalysis.Candidates, missingAnalysis.Candidates)
	}
	for _, analysis := range []Analysis{existingAnalysis, missingAnalysis} {
		for _, candidate := range analysis.Candidates {
			if candidate.Kind == KindBrokenLink && candidate.NoteID == publicNote.ID {
				t.Fatalf("restricted scope-out link produced broken candidate: %#v", candidate)
			}
		}
	}
	pairedResults, err := json.Marshal([][]Candidate{existingAnalysis.Candidates, missingAnalysis.Candidates})
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{outOfScopeNote.ID, outOfScopeNote.Title, missingID} {
		if strings.Contains(string(pairedResults), hidden) {
			t.Fatalf("scope-out comparison exposed hidden target value %q: %s", hidden, pairedResults)
		}
	}

	hiddenContent := strings.Join([]string{
		"[protected](atlasnote://note/" + protectedNote.ID + ")",
		"[locked](atlasnote://note/" + lockedNote.ID + ")",
		"[trashed](atlasnote://note/" + trashedNote.ID + ")",
		"[out-of-scope](atlasnote://note/" + outOfScopeNote.ID + ")",
		"[missing](atlasnote://note/" + missingID + ")",
	}, "\n")
	publicNote, err = notes.Update(ctx, publicNote.ID, note.UpdateInput{Content: &hiddenContent, ExpectedRevision: &publicNote.Revision})
	if err != nil {
		t.Fatal(err)
	}
	hiddenAnalysis, err := organizer.AnalyzeExternal(ctx, "space", AnalysisInput{Scope: "space"}, access)
	if err != nil {
		t.Fatal(err)
	}
	if hiddenAnalysis.AnalyzedNotes != 1 || hiddenAnalysis.SkippedLocked != 0 || hiddenAnalysis.SkippedTrash != 0 {
		t.Fatalf("restricted analysis leaked hidden target counts: %#v", hiddenAnalysis)
	}
	if !reflect.DeepEqual(normalizedCandidates(existingAnalysis), normalizedCandidates(hiddenAnalysis)) {
		t.Fatalf("protected/locked/trashed result differs from other hidden targets: existing=%#v hidden=%#v", existingAnalysis.Candidates, hiddenAnalysis.Candidates)
	}
	for _, candidate := range hiddenAnalysis.Candidates {
		if candidate.Kind == KindBrokenLink && candidate.NoteID == publicNote.ID {
			t.Fatalf("restricted hidden target produced broken candidate: %#v", candidate)
		}
	}
	encoded, err := json.Marshal(hiddenAnalysis.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{
		protectedNote.ID, protectedNote.Title, lockedNote.ID, lockedNote.Title,
		trashedNote.ID, trashedNote.Title, outOfScopeNote.ID, outOfScopeNote.Title, missingID,
		`"protected":true`, `"locked":true`, `"isTrashed":true`,
	} {
		if strings.Contains(string(encoded), hidden) {
			t.Fatalf("restricted candidates exposed hidden target value %q: %s", hidden, encoded)
		}
	}

	guiAnalysis, err := organizer.Analyze(ctx, "space", AnalysisInput{Scope: "space"})
	if err != nil {
		t.Fatal(err)
	}
	var guiBroken bool
	for _, candidate := range guiAnalysis.Candidates {
		if candidate.Kind == KindBrokenLink && candidate.NoteID == publicNote.ID && candidate.RelatedID == missingID {
			guiBroken = true
		}
	}
	if !guiBroken {
		t.Fatal("unrestricted GUI analysis no longer reports a genuinely missing link")
	}
}

func TestExternalAnalysisCancellationDoesNotCreateSession(t *testing.T) {
	t.Parallel()
	notes, organizer := newOrganizationFixture(t)
	created, err := notes.Create(t.Context(), note.CreateInput{Title: "Cancelled", Content: "# Candidate"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = organizer.AnalyzeExternal(ctx, "space", AnalysisInput{Scope: "space"}, AnalysisAccess{
		OwnerID: "cancelled-owner", ScopeRestricted: true, AllowedNoteIDs: map[string]bool{created.ID: true},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled analysis error = %v", err)
	}
	organizer.mu.Lock()
	defer organizer.mu.Unlock()
	if len(organizer.sessions) != 0 {
		t.Fatalf("cancelled analysis created %d sessions", len(organizer.sessions))
	}
}

func newOrganizationFixture(t *testing.T) (*note.Service, *Service) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	markdown, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		t.Fatalf("create markdown store: %v", err)
	}
	notes := note.NewService(note.NewRepository(db), markdown)
	return notes, NewService(notes)
}
