package organize

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"atlasnote/internal/note"
)

type Service struct {
	notes    *note.Service
	mu       sync.Mutex
	sessions map[string]analysisSession
}

type analysisSession struct {
	spaceID   string
	created   time.Time
	candidate map[string]Candidate
}

func NewService(notes *note.Service) *Service {
	return &Service{notes: notes, sessions: make(map[string]analysisSession)}
}

func (s *Service) InvalidateSessions() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sessions = make(map[string]analysisSession)
	s.mu.Unlock()
}

// DiscardSession removes one analysis result without invalidating other scopes.
func (s *Service) DiscardSession(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

func (s *Service) Analyze(ctx context.Context, spaceID string, input AnalysisInput, onProgress ...func(AnalysisProgress)) (Analysis, error) {
	if s == nil || s.notes == nil {
		return Analysis{}, fmt.Errorf("organization service is not initialized")
	}
	ctx, releaseAccess := s.notes.BeginOrganizationRead(ctx)
	defer releaseAccess()
	if input.Scope == "" {
		input.Scope = "space"
	}
	if input.Scope != "space" && input.Scope != "notebook" && input.Scope != "descendants" && input.Scope != "note" {
		return Analysis{}, fmt.Errorf("unsupported organization scope")
	}
	if (input.Scope == "notebook" || input.Scope == "descendants") && input.NotebookID == "" {
		return Analysis{}, fmt.Errorf("notebook scope requires a notebook ID")
	}
	if input.Scope == "note" && input.NoteID == "" {
		return Analysis{}, fmt.Errorf("note scope requires a note ID")
	}
	startedAt := time.Now().UTC()
	result := Analysis{
		SpaceID:    spaceID,
		Scope:      input.Scope,
		StartedAt:  startedAt,
		Candidates: make([]Candidate, 0),
	}
	sessionID, err := newSessionID()
	if err != nil {
		return Analysis{}, err
	}
	result.SessionID = sessionID
	if input.NotebookID != "" {
		result.NotebookID = input.NotebookID
	}
	if input.NoteID != "" {
		result.NoteID = input.NoteID
	}

	notebooks, err := s.notes.ListNotebooks(ctx)
	if err != nil {
		return Analysis{}, err
	}
	var inScope map[string]struct{}
	if input.Scope == "notebook" || input.Scope == "descendants" {
		inScope, err = notebookScope(notebooks, input.NotebookID, input.Scope == "descendants")
		if err != nil {
			return Analysis{}, err
		}
	}
	summaries, err := s.notes.List(ctx)
	if err != nil {
		return Analysis{}, err
	}
	tags, err := s.notes.ListTags(ctx)
	if err != nil {
		return Analysis{}, err
	}

	knownIDs := make(map[string]struct{}, len(summaries))
	byID := make(map[string]note.Summary, len(summaries))
	for _, summary := range summaries {
		knownIDs[summary.ID] = struct{}{}
		byID[summary.ID] = summary
	}
	var scopedNoteIDs map[string]struct{}
	if input.Scope == "note" {
		target, exists := byID[input.NoteID]
		if !exists {
			return Analysis{}, note.ErrNotFound
		}
		scopedNoteIDs = map[string]struct{}{input.NoteID: {}}
		if !target.IsTrashed && !target.Protected && !target.Locked {
			if targetNote, getErr := s.notes.Get(ctx, input.NoteID); getErr == nil && !targetNote.Protected && !targetNote.Locked {
				for _, relatedID := range note.ExtractNoteLinkTargets(targetNote.Content) {
					if _, exists := byID[relatedID]; exists {
						scopedNoteIDs[relatedID] = struct{}{}
					}
				}
				for page := 1; ; page++ {
					backlinks, backlinkErr := s.notes.ListBacklinks(ctx, note.BacklinkListInput{NoteID: input.NoteID, Page: page, PageSize: note.MaxBacklinkPageSize})
					if backlinkErr != nil {
						break
					}
					for _, backlink := range backlinks.Items {
						scopedNoteIDs[backlink.ID] = struct{}{}
					}
					if !backlinks.HasNext {
						break
					}
				}
			}
		}
	}
	selected := make([]note.Summary, 0, len(summaries))
	for _, summary := range summaries {
		if summaryInScope(summary, input, inScope, scopedNoteIDs) {
			selected = append(selected, summary)
		}
	}
	active := make([]analyzedNote, 0, len(selected))
	reportReading := func(processed int) {
		if len(onProgress) > 0 && onProgress[0] != nil && (processed == 0 || processed == len(selected) || processed%100 == 0) {
			onProgress[0](AnalysisProgress{Phase: "reading", ProcessedNotes: processed, TotalNotes: len(selected)})
		}
	}
	reportReading(0)
	for index, summary := range selected {
		if err := ctx.Err(); err != nil {
			return Analysis{}, err
		}
		processed := index + 1
		if summary.IsTrashed {
			result.SkippedTrash++
			reportReading(processed)
			continue
		}
		if summary.Protected || summary.Locked {
			result.SkippedLocked++
			reportReading(processed)
			continue
		}
		item, getErr := s.notes.Get(ctx, summary.ID)
		if getErr != nil {
			// A lock can be enabled between List and Get. Fail closed and report it
			// as skipped instead of returning protected content to the analyzer.
			result.SkippedLocked++
			reportReading(processed)
			continue
		}
		if item.Protected || item.Locked {
			result.SkippedLocked++
			reportReading(processed)
			continue
		}
		tagIDs := make(map[string]struct{})
		if len(tags) > 0 {
			noteTags, tagErr := s.notes.ListNoteTags(ctx, summary.ID)
			if tagErr != nil {
				return Analysis{}, tagErr
			}
			for _, tag := range noteTags.Tags {
				tagIDs[tag.ID] = struct{}{}
			}
		}
		summary.Revision = item.Revision
		summary.Title = item.Title
		summary.NotebookID = item.NotebookID
		summary.IsTrashed = item.IsTrashed
		active = append(active, analyzedNote{summary: summary, note: item, tagIDs: tagIDs})
		reportReading(processed)
	}
	result.AnalyzedNotes = len(active)
	if len(onProgress) > 0 && onProgress[0] != nil {
		onProgress[0](AnalysisProgress{Phase: "proposing", ProcessedNotes: len(selected), TotalNotes: len(selected)})
	}

	result.Candidates = append(result.Candidates, titleCandidates(spaceID, active)...)
	result.Candidates = append(result.Candidates, notebookCandidates(spaceID, active, notebooks)...)
	result.Candidates = append(result.Candidates, tagAssignmentCandidates(spaceID, active, tags)...)
	result.Candidates = append(result.Candidates, duplicateCandidates(spaceID, active)...)
	result.Candidates = append(result.Candidates, emptyCandidates(spaceID, active)...)
	result.Candidates = append(result.Candidates, linkCandidates(spaceID, active, knownIDs)...)
	result.Candidates = append(result.Candidates, reciprocalLinkCandidates(spaceID, active)...)
	result.Candidates = append(result.Candidates, orphanCandidates(spaceID, active)...)
	result.Candidates = append(result.Candidates, relatedCandidates(spaceID, active, tags)...)
	result.Candidates = append(result.Candidates, duplicateTagCandidates(spaceID, tags)...)
	if input.Scope == "note" {
		filtered := result.Candidates[:0]
		for _, candidate := range result.Candidates {
			if candidate.NoteID == input.NoteID || candidate.RelatedID == input.NoteID {
				filtered = append(filtered, candidate)
			}
		}
		result.Candidates = filtered
	}
	sort.SliceStable(result.Candidates, func(i, j int) bool {
		if result.Candidates[i].Kind != result.Candidates[j].Kind {
			return result.Candidates[i].Kind < result.Candidates[j].Kind
		}
		if result.Candidates[i].NoteID != result.Candidates[j].NoteID {
			return result.Candidates[i].NoteID < result.Candidates[j].NoteID
		}
		return result.Candidates[i].ID < result.Candidates[j].ID
	})
	session := analysisSession{spaceID: spaceID, created: startedAt, candidate: make(map[string]Candidate, len(result.Candidates))}
	for _, candidate := range result.Candidates {
		session.candidate[candidate.ID] = candidate
	}
	s.mu.Lock()
	s.sessions[sessionID] = session
	for id, previous := range s.sessions {
		if time.Since(previous.created) > 30*time.Minute {
			delete(s.sessions, id)
		}
	}
	for len(s.sessions) > 5 {
		oldestID := ""
		var oldest time.Time
		for id, previous := range s.sessions {
			if oldestID == "" || previous.created.Before(oldest) {
				oldestID, oldest = id, previous.created
			}
		}
		delete(s.sessions, oldestID)
	}
	s.mu.Unlock()
	return result, nil
}

func summaryInScope(summary note.Summary, input AnalysisInput, notebookIDs, noteIDs map[string]struct{}) bool {
	switch input.Scope {
	case "notebook":
		return summary.NotebookID != nil && *summary.NotebookID == input.NotebookID
	case "descendants":
		if summary.NotebookID == nil {
			return false
		}
		_, ok := notebookIDs[*summary.NotebookID]
		return ok
	case "note":
		_, ok := noteIDs[summary.ID]
		return ok
	default:
		return true
	}
}

func newSessionID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("create organization analysis session: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (s *Service) Apply(ctx context.Context, spaceID string, input ApplyInput) ApplyResult {
	results := s.ApplyCandidates(ctx, spaceID, ApplyCandidatesInput{
		SessionID:    input.SessionID,
		CandidateIDs: []string{input.CandidateID},
	})
	if len(results) == 0 {
		return ApplyResult{CandidateID: input.CandidateID, Status: "stale", Message: "解析結果が古いか、現在の保存空間と一致しません。再解析してください。"}
	}
	return results[0]
}

func applyConflict(result ApplyResult) ApplyResult {
	result.Status = "conflict"
	result.Message = "対象状態が解析時から変わりました。変更は適用されていません。再解析してください。"
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type analyzedNote struct {
	summary note.Summary
	note    note.Note
	tagIDs  map[string]struct{}
}

func notebookScope(notebooks []note.Notebook, selected string, includeDescendants bool) (map[string]struct{}, error) {
	if selected == "" {
		return nil, nil
	}
	byID := make(map[string]note.Notebook, len(notebooks))
	for _, item := range notebooks {
		byID[item.ID] = item
	}
	if _, ok := byID[selected]; !ok {
		return nil, fmt.Errorf("notebook not found")
	}
	result := map[string]struct{}{selected: {}}
	if !includeDescendants {
		return result, nil
	}
	changed := true
	for changed {
		changed = false
		for _, item := range notebooks {
			if item.ParentID == nil {
				continue
			}
			if _, exists := result[*item.ParentID]; exists {
				if _, already := result[item.ID]; !already {
					result[item.ID] = struct{}{}
					changed = true
				}
			}
		}
	}
	return result, nil
}

func candidateID(kind, noteID, relatedID, spaceID string, revision int64, discriminator string) string {
	payload := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s", kind, noteID, relatedID, spaceID, revision, discriminator)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}

func makeCandidate(spaceID, kind string, item analyzedNote, relatedID, reason string, before, proposed map[string]any, applicable bool, discriminator string) Candidate {
	return Candidate{
		ID:           candidateID(kind, item.summary.ID, relatedID, spaceID, item.summary.Revision, discriminator),
		Kind:         kind,
		NoteID:       item.summary.ID,
		NoteTitle:    item.note.Title,
		RelatedID:    relatedID,
		NotebookID:   stringValue(proposed["notebookId"]),
		TagID:        stringValue(proposed["tagId"]),
		SpaceID:      spaceID,
		BaseRevision: item.summary.Revision,
		Before:       before,
		Proposed:     proposed,
		Reason:       reason,
		Applicable:   applicable,
	}
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func titleCandidates(spaceID string, items []analyzedNote) []Candidate {
	result := make([]Candidate, 0)
	for _, item := range items {
		heading := firstMarkdownHeading(item.note.Content)
		if heading == "" || heading == item.note.Title {
			continue
		}
		result = append(result, makeCandidate(spaceID, KindTitle, item, "", "Markdown本文の先頭見出しをタイトル候補として検出しました。", map[string]any{"title": item.note.Title}, map[string]any{"title": heading}, true, heading))
	}
	return result
}

func firstMarkdownHeading(content string) string {
	inFence := false
	fenceMarker := byte(0)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if marker, ok := markdownFenceMarker(trimmed); ok {
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
				fenceMarker = 0
			}
			continue
		}
		if inFence {
			continue
		}
		marks := 0
		for marks < len(trimmed) && marks < 6 && trimmed[marks] == '#' {
			marks++
		}
		if marks == 0 || marks < len(trimmed) && trimmed[marks] != ' ' && trimmed[marks] != '\t' {
			continue
		}
		heading := strings.TrimSpace(trimmed[marks:])
		if suffix := strings.LastIndex(heading, " #"); suffix >= 0 && strings.Trim(strings.TrimSpace(heading[suffix:]), "#") == "" {
			heading = strings.TrimSpace(heading[:suffix])
		}
		if heading != "" {
			return heading
		}
	}
	return ""
}

func markdownFenceMarker(line string) (byte, bool) {
	if len(line) < 3 || line[0] != '`' && line[0] != '~' {
		return 0, false
	}
	marker := line[0]
	count := 0
	for count < len(line) && line[count] == marker {
		count++
	}
	return marker, count >= 3
}

func notebookCandidates(spaceID string, items []analyzedNote, notebooks []note.Notebook) []Candidate {
	result := make([]Candidate, 0)
	byName := make(map[string][]note.Notebook)
	for _, item := range notebooks {
		name := strings.TrimSpace(item.Name)
		byName[name] = append(byName[name], item)
	}
	for _, item := range items {
		matches := byName[strings.TrimSpace(item.note.Title)]
		if item.summary.NotebookID == nil {
			candidate := makeCandidate(spaceID, KindUnclassifiedNote, item, "", "このノートはNotebookへ分類されていません。", map[string]any{"notebookId": nil}, nil, false, "unclassified")
			result = append(result, candidate)
		}
		if len(matches) != 1 || matches[0].Protected || matches[0].Locked {
			continue
		}
		target := matches[0]
		if item.summary.NotebookID == nil {
			result = append(result, makeCandidate(spaceID, KindNotebookAssignment, item, "", "未分類ノートのタイトルと既存Notebook名が完全一致しています。", map[string]any{"notebookId": nil}, map[string]any{"notebookId": target.ID, "notebookName": target.Name}, true, target.ID))
			continue
		}
		if *item.summary.NotebookID == target.ID {
			continue
		}
		currentName := ""
		for _, notebook := range notebooks {
			if notebook.ID == *item.summary.NotebookID {
				currentName = notebook.Name
				break
			}
		}
		candidate := makeCandidate(spaceID, KindNotebookMove, item, "", "ノートタイトルと別の既存Notebook名が完全一致しています。移動先と既存階層を確認してください。", map[string]any{"notebookId": *item.summary.NotebookID, "notebookName": currentName}, map[string]any{"notebookId": target.ID, "notebookName": target.Name}, true, target.ID)
		result = append(result, candidate)
	}
	return result
}

func tagAssignmentCandidates(spaceID string, items []analyzedNote, tags []note.Tag) []Candidate {
	result := make([]Candidate, 0)
	folder := cases.Fold()
	for _, item := range items {
		searchable := folder.String(norm.NFC.String(item.note.Title + "\n" + item.note.Content))
		for _, tag := range tags {
			if _, exists := item.tagIDs[tag.ID]; exists {
				continue
			}
			name := folder.String(norm.NFC.String(strings.TrimSpace(tag.Name)))
			if len([]rune(name)) < 2 || !strings.Contains(searchable, name) {
				continue
			}
			result = append(result, makeCandidate(spaceID, KindTagAssignment, item, "", fmt.Sprintf("本文またはタイトルに既存タグ「%s」の表記があります。", tag.Name), map[string]any{"tagIds": sortedKeys(item.tagIDs)}, map[string]any{"tagId": tag.ID, "tagName": tag.Name}, true, tag.ID))
		}
	}
	return result
}

func duplicateCandidates(spaceID string, items []analyzedNote) []Candidate {
	groups := make(map[string][]analyzedNote)
	for _, item := range items {
		if strings.TrimSpace(item.note.Content) == "" {
			continue
		}
		signature := lightweightContentSignature(item.note.Content)
		if signature == "" {
			continue
		}
		groups[signature] = append(groups[signature], item)
	}
	result := make([]Candidate, 0)
	for signature, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].summary.ID < group[j].summary.ID })
		keeper := group[0]
		for _, duplicate := range group[1:] {
			exact := duplicate.note.Content == keeper.note.Content
			reason := "Markdown本文が完全一致します。残すノートを確認してからゴミ箱へ移動できます。"
			if !exact {
				reason = "改行・行末空白などの軽微な表記差を除くと本文が一致します。残すノートを確認してからゴミ箱へ移動できます。"
			}
			candidate := makeCandidate(spaceID, KindDuplicateNote, duplicate, keeper.summary.ID, reason, map[string]any{"isTrashed": false, "contentHash": contentHashOf(duplicate.note.Content), "characterCount": len([]rune(duplicate.note.Content))}, map[string]any{"isTrashed": true}, true, signature)
			candidate.RelatedTitle = keeper.note.Title
			candidate.RelatedRevision = keeper.summary.Revision
			candidate.RelatedContentHash = contentHashOf(keeper.note.Content)
			result = append(result, candidate)
		}
	}
	return result
}

func lightweightContentSignature(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for index, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		fields := strings.Fields(line)
		lines[index] = strings.Join(fields, " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func emptyCandidates(spaceID string, items []analyzedNote) []Candidate {
	result := make([]Candidate, 0)
	for _, item := range items {
		if strings.TrimSpace(item.note.Content) != "" {
			continue
		}
		result = append(result, makeCandidate(spaceID, KindEmptyNote, item, "", "本文が空または空白だけです。確認後にゴミ箱へ移動できます。", map[string]any{"isTrashed": false, "contentHash": contentHashOf(item.note.Content)}, map[string]any{"isTrashed": true}, true, "empty"))
	}
	return result
}

func reciprocalLinkCandidates(spaceID string, items []analyzedNote) []Candidate {
	byID := make(map[string]analyzedNote, len(items))
	links := make(map[string]map[string]struct{}, len(items))
	for _, item := range items {
		byID[item.summary.ID] = item
		links[item.summary.ID] = make(map[string]struct{})
		for _, targetID := range note.ExtractNoteLinkTargets(item.note.Content) {
			links[item.summary.ID][targetID] = struct{}{}
		}
	}
	result := make([]Candidate, 0)
	seen := make(map[string]struct{})
	for sourceID, targets := range links {
		for targetID := range targets {
			target, exists := byID[targetID]
			if !exists {
				continue
			}
			if _, reciprocal := links[targetID][sourceID]; reciprocal {
				continue
			}
			pair := sourceID + "\x00" + targetID
			if _, exists := seen[pair]; exists {
				continue
			}
			seen[pair] = struct{}{}
			candidate := makeCandidate(spaceID, KindReciprocalLink, target, sourceID,
				"相手のノートからこのノートへのリンクがあります。逆方向リンクを追記できます。",
				map[string]any{"contentHash": contentHashOf(target.note.Content)},
				map[string]any{"targetId": sourceID}, true, sourceID)
			candidate.RelatedTitle = byID[sourceID].note.Title
			candidate.RelatedRevision = byID[sourceID].summary.Revision
			candidate.RelatedContentHash = contentHashOf(byID[sourceID].note.Content)
			result = append(result, candidate)
		}
	}
	return result
}

func contentHashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func linkCandidates(spaceID string, items []analyzedNote, knownIDs map[string]struct{}) []Candidate {
	result := make([]Candidate, 0)
	for _, item := range items {
		for _, targetID := range note.ExtractNoteLinkTargets(item.note.Content) {
			if _, exists := knownIDs[targetID]; exists {
				continue
			}
			result = append(result, makeCandidate(spaceID, KindBrokenLink, item, targetID, "Markdown本文のノートリンク先IDがこの保存空間にありません。", map[string]any{"targetId": targetID}, nil, false, targetID))
		}
	}
	return result
}

func orphanCandidates(spaceID string, items []analyzedNote) []Candidate {
	degree := make(map[string]int, len(items))
	ids := make(map[string]struct{}, len(items))
	for _, item := range items {
		ids[item.summary.ID] = struct{}{}
	}
	for _, item := range items {
		for _, target := range note.ExtractNoteLinkTargets(item.note.Content) {
			if _, exists := ids[target]; exists {
				degree[item.summary.ID]++
				degree[target]++
			}
		}
	}
	result := make([]Candidate, 0)
	for _, item := range items {
		if degree[item.summary.ID] == 0 {
			result = append(result, makeCandidate(spaceID, KindOrphanNote, item, "", "このノートからのリンクも、このノートへのリンクも見つかりません。", map[string]any{"linkCount": 0}, nil, false, "orphan"))
		}
	}
	return result
}

func relatedCandidates(spaceID string, items []analyzedNote, tags []note.Tag) []Candidate {
	byTag := make(map[string][]string)
	tagNames := make(map[string]string, len(tags))
	for _, tag := range tags {
		tagNames[tag.ID] = tag.Name
	}
	byWord := make(map[string][]string)
	contentByID := make(map[string]analyzedNote, len(items))
	for _, item := range items {
		contentByID[item.summary.ID] = item
		for tagID := range item.tagIDs {
			byTag[tagID] = append(byTag[tagID], item.summary.ID)
		}
		for word := range meaningfulWords(item.note.Title + " " + item.note.Content) {
			byWord[word] = append(byWord[word], item.summary.ID)
		}
	}
	pairs := make(map[string]map[string][]string)
	tagIDs := make([]string, 0, len(byTag))
	for tagID := range byTag {
		tagIDs = append(tagIDs, tagID)
	}
	sort.Strings(tagIDs)
	for _, tagID := range tagIDs {
		addRelatedPairs(pairs, byTag[tagID], "タグ「"+tagNames[tagID]+"」")
	}
	words := make([]string, 0, len(byWord))
	for word := range byWord {
		words = append(words, word)
	}
	sort.Strings(words)
	for _, word := range words {
		noteIDs := byWord[word]
		if len([]rune(word)) >= 3 {
			addRelatedPairs(pairs, noteIDs, "語句「"+word+"」")
		}
	}
	result := make([]Candidate, 0)
	for firstID, related := range pairs {
		for secondID, reasons := range related {
			if firstID >= secondID {
				continue
			}
			sort.Strings(reasons)
			reason := strings.Join(reasons, ", ")
			item := contentByID[firstID]
			candidate := makeCandidate(spaceID, KindRelatedNote, item, secondID, "共通する既存タグまたは語句: "+reason, map[string]any{"relatedId": ""}, nil, false, secondID+reason)
			candidate.RelatedTitle = contentByID[secondID].note.Title
			result = append(result, candidate)
		}
	}
	return result
}

func addRelatedPairs(pairs map[string]map[string][]string, ids []string, reason string) {
	sort.Strings(ids)
	if len(ids) > 40 {
		return
	}
	for i, first := range ids {
		for _, second := range ids[i+1:] {
			if pairs[first] == nil {
				pairs[first] = make(map[string][]string)
			}
			if _, exists := pairs[first][second]; !exists && len(pairs[first]) >= 5 {
				continue
			}
			pairs[first][second] = append(pairs[first][second], reason)
		}
	}
}

func meaningfulWords(content string) map[string]struct{} {
	words := make(map[string]struct{})
	for _, field := range strings.FieldsFunc(strings.ToLower(content), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len([]rune(field)) >= 3 {
			words[field] = struct{}{}
		}
	}
	return words
}

func duplicateTagCandidates(spaceID string, tags []note.Tag) []Candidate {
	groups := make(map[string][]note.Tag)
	folder := cases.Fold()
	for _, tag := range tags {
		name := folder.String(norm.NFC.String(strings.Join(strings.Fields(tag.Name), " ")))
		groups[name] = append(groups[name], tag)
	}
	result := make([]Candidate, 0)
	for key, group := range groups {
		if key == "" || len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
		for _, tag := range group {
			result = append(result, Candidate{
				ID:         candidateID(KindDuplicateTag, "", tag.ID, spaceID, 0, key),
				Kind:       KindDuplicateTag,
				TagID:      tag.ID,
				SpaceID:    spaceID,
				Before:     map[string]any{"tagId": tag.ID, "name": tag.Name},
				Reason:     "表記を正規化すると同じタグ名になります。タグ統合は未対応のため、検出だけを表示します。",
				Applicable: false,
			})
		}
	}
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
