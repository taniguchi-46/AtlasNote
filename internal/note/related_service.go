package note

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type rankedRelatedNote struct {
	item  RelatedNoteItem
	score int
}

// RelatedNotes returns explainable candidates from the active storage space.
// It never writes note content or calls the organization analysis.
func (s *Service) RelatedNotes(ctx context.Context, input RelatedNoteInput) (RelatedNoteResult, error) {
	empty := RelatedNoteResult{Items: make([]RelatedNoteItem, 0)}
	if strings.TrimSpace(input.NoteID) == "" || input.Limit < 0 || input.Limit > 20 ||
		(input.NotebookID != nil && strings.TrimSpace(*input.NotebookID) == "") {
		return empty, fmt.Errorf("%w: invalid related note input", ErrValidation)
	}
	if input.Limit == 0 {
		input.Limit = 20
	}
	releaseContent := s.beginContentAccess(ctx)
	defer releaseContent()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.recoverPendingLocked(ctx); err != nil {
		return empty, err
	}
	if s.searchIndexFailed || s.noteLinkIndexFailed {
		return empty, fmt.Errorf("related index update failed")
	}

	source, err := s.repository.Get(ctx, input.NoteID)
	if err != nil {
		return empty, err
	}
	if source.IsTrashed {
		return empty, fmt.Errorf("%w: note is in trash", ErrValidation)
	}
	if allowed, err := s.relatedNoteAllowed(ctx, source.ID); err != nil {
		return empty, err
	} else if !allowed {
		return empty, fmt.Errorf("%w: protected note", ErrValidation)
	}
	if err := s.relatedIndexCurrent(ctx, source.ID, source.Revision); err != nil {
		return empty, err
	}

	allowedNotebooks := map[string]bool(nil)
	if input.NotebookID != nil {
		allowedNotebooks = map[string]bool{*input.NotebookID: true}
		if input.Descendants {
			tree, err := s.repository.ListNotebookTree(ctx, *input.NotebookID)
			if err != nil {
				return empty, err
			}
			for _, notebook := range tree {
				allowedNotebooks[notebook.ID] = true
			}
		} else if _, err := s.repository.GetNotebook(ctx, *input.NotebookID); err != nil {
			return empty, err
		}
	}

	signals := make(map[string]*relatedSignals)
	add := func(id string) *relatedSignals {
		if id == source.ID {
			return nil
		}
		if signals[id] == nil {
			signals[id] = &relatedSignals{}
		}
		return signals[id]
	}
	for _, outbound := range []bool{true, false} {
		ids, err := s.repository.relatedLinks(ctx, source.ID, outbound, allowedNotebooks)
		if err != nil {
			return empty, err
		}
		for _, id := range ids {
			if signal := add(id); signal != nil {
				if outbound {
					signal.outbound = true
				} else {
					signal.backlink = true
				}
			}
		}
	}
	tags, err := s.repository.relatedTags(ctx, source.ID, allowedNotebooks)
	if err != nil {
		return empty, err
	}
	for id, names := range tags {
		if signal := add(id); signal != nil {
			signal.tags = names
		}
	}

	textMatches := make(map[string]string)
	term := relatedSearchTerm(source.Title)
	if term != "" {
		found, err := s.repository.relatedTextMatches(ctx, source.ID, term, allowedNotebooks)
		if err != nil {
			return empty, err
		}
		for id, matchScope := range found {
			if add(id) != nil {
				textMatches[id] = matchScope
			}
		}
	}

	ranked := make([]rankedRelatedNote, 0, len(signals))
	for id, signal := range signals {
		record, err := s.repository.Get(ctx, id)
		if err != nil {
			return empty, err
		}
		if record.IsTrashed || (allowedNotebooks != nil && (record.NotebookID == nil || !allowedNotebooks[*record.NotebookID])) {
			continue
		}
		allowed, err := s.relatedNoteAllowed(ctx, id)
		if err != nil {
			return empty, err
		}
		if !allowed {
			continue
		}
		match := textMatches[id]
		if !signal.outbound && !signal.backlink && match == "" && len(signal.tags) < 2 {
			continue
		}
		if err := s.relatedIndexCurrent(ctx, id, record.Revision); err != nil {
			return empty, err
		}
		snippet, err := s.repository.relatedSnippet(ctx, id, record.Revision)
		if err != nil {
			return empty, err
		}
		item := RelatedNoteItem{NoteID: id, Title: record.Title, Revision: record.Revision, Snippet: snippet, Reasons: make([]string, 0, 3)}
		score := 0
		if signal.outbound {
			score += 8
			item.Reasons = append(item.Reasons, "このノートからリンク")
		}
		if signal.backlink {
			score += 7
			item.Reasons = append(item.Reasons, "このノートへのリンク")
		}
		if match == SearchScopeTitle || match == "both" {
			score += 5
			item.Reasons = append(item.Reasons, "タイトルに共通する語句")
		}
		if match == "body" || match == "both" {
			score += 2
			item.Reasons = append(item.Reasons, "本文に共通する語句")
		}
		if len(signal.tags) > 0 {
			score += min(len(signal.tags), 2)
			item.Reasons = append(item.Reasons, "共通タグ: "+strings.Join(signal.tags, "、"))
		}
		if len(item.Reasons) > 3 {
			item.Reasons = item.Reasons[:3]
		}
		ranked = append(ranked, rankedRelatedNote{item: item, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].item.NoteID < ranked[j].item.NoteID
	})
	for _, candidate := range ranked {
		if len(empty.Items) >= input.Limit {
			break
		}
		empty.Items = append(empty.Items, candidate.item)
	}
	return empty, nil
}

func (s *Service) relatedNoteAllowed(ctx context.Context, id string) (bool, error) {
	if s.contentLocks == nil {
		return true, nil
	}
	protected, locked, _, err := s.contentLocks.NoteLockStatus(ctx, id)
	if err != nil {
		return false, err
	}
	return !protected && !locked, nil
}

func (s *Service) relatedIndexCurrent(ctx context.Context, id string, revision int64) error {
	searchState, found, err := s.repository.GetSearchIndexState(ctx, id)
	if err != nil {
		return err
	}
	if !found || searchState.IndexedRevision != revision {
		return fmt.Errorf("related search index is inconsistent")
	}
	matches, err := s.store.ContentMatches(ctx, id, searchState.ContentHash)
	if err != nil {
		return err
	}
	if !matches {
		return fmt.Errorf("related search index is inconsistent")
	}
	linkState, found, err := s.repository.GetNoteLinkIndexState(ctx, id)
	if err != nil {
		return err
	}
	if !found || linkState.IndexedRevision != revision || linkState.ContentHash != searchState.ContentHash {
		return fmt.Errorf("related link index is inconsistent")
	}
	return nil
}

func relatedSearchTerm(title string) string {
	fields := strings.Fields(strings.TrimSpace(title))
	if len(fields) == 0 {
		return ""
	}
	runes := []rune(fields[0])
	if len(runes) > 12 {
		runes = runes[:12]
	}
	return string(runes)
}
