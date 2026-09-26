package organize

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"atlasnote/internal/note"
)

func (s *Service) ApplyCandidates(ctx context.Context, spaceID string, input ApplyCandidatesInput) []ApplyResult {
	return s.applyCandidates(ctx, spaceID, "", input)
}

// ApplyExternalCandidates is called only after a trusted GUI approval. The
// owner is checked against the server-side analysis session, never supplied
// through the public IPC operation.
func (s *Service) ApplyExternalCandidates(ctx context.Context, spaceID, ownerID string, input ApplyCandidatesInput) []ApplyResult {
	if ownerID == "" {
		return nil
	}
	return s.applyCandidates(ctx, spaceID, ownerID, input)
}

func (s *Service) applyCandidates(ctx context.Context, spaceID, ownerID string, input ApplyCandidatesInput) []ApplyResult {
	results := make(map[string]ApplyResult, len(input.CandidateIDs))
	if len(input.CandidateIDs) == 0 {
		return []ApplyResult{}
	}

	ctx, release := s.notes.BeginOrganizationExclusive(ctx)
	defer release()

	s.mu.Lock()
	session, sessionOK := s.sessions[input.SessionID]
	candidates := make(map[string]Candidate, len(input.CandidateIDs))
	if sessionOK && session.spaceID == spaceID && session.ownerID == ownerID &&
		(ownerID == "" || session.created.Add(analysisSessionTTL).After(time.Now().UTC())) {
		for _, id := range input.CandidateIDs {
			if candidate, ok := session.candidate[id]; ok && candidate.SpaceID == spaceID {
				candidates[id] = candidate
			}
		}
	}
	s.mu.Unlock()
	if !sessionOK || session.spaceID != spaceID || session.ownerID != ownerID ||
		(ownerID != "" && !session.created.Add(analysisSessionTTL).After(time.Now().UTC())) {
		for _, id := range input.CandidateIDs {
			results[id] = staleResult(id)
		}
		return orderedResults(input.CandidateIDs, results)
	}

	groups := make(map[string][]Candidate)
	for _, id := range input.CandidateIDs {
		candidate, ok := candidates[id]
		if !ok {
			results[id] = staleResult(id)
			continue
		}
		if !candidate.Applicable || candidate.NoteID == "" {
			results[id] = ApplyResult{CandidateID: id, Status: "not-applicable", Message: "この候補は検出のみで、適用できません。"}
			continue
		}
		groups[candidate.NoteID] = append(groups[candidate.NoteID], candidate)
	}

	// One locked batch per source note lets all reviewed changes share the same
	// expected revision and commit at most one Markdown/revision update.
	noteIDs := make([]string, 0, len(groups))
	for noteID := range groups {
		noteIDs = append(noteIDs, noteID)
	}
	sort.Strings(noteIDs)
	for _, noteID := range noteIDs {
		for id, result := range s.applyNoteGroup(ctx, groups[noteID]) {
			results[id] = result
		}
	}
	return orderedResults(input.CandidateIDs, results)
}

func orderedResults(ids []string, byID map[string]ApplyResult) []ApplyResult {
	results := make([]ApplyResult, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		if result, ok := byID[id]; ok {
			results = append(results, result)
		} else {
			results = append(results, staleResult(id))
		}
	}
	return results
}

func staleResult(candidateID string) ApplyResult {
	return ApplyResult{CandidateID: candidateID, Status: "stale", Message: "解析結果が古いか、現在の保存空間と一致しません。再解析してください。"}
}

func (s *Service) applyNoteGroup(ctx context.Context, candidates []Candidate) map[string]ApplyResult {
	results := make(map[string]ApplyResult, len(candidates))
	if len(candidates) == 0 {
		return results
	}
	current, err := s.notes.Get(ctx, candidates[0].NoteID)
	if err != nil {
		for _, candidate := range candidates {
			results[candidate.ID] = ApplyResult{CandidateID: candidate.ID, Status: "not-executed", Message: "対象状態を確認できません。再解析してください。"}
		}
		return results
	}
	baseRevision := candidates[0].BaseRevision
	if current.Protected || current.Locked || current.IsTrashed || current.Revision != baseRevision {
		for _, candidate := range candidates {
			results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
		}
		return results
	}
	for _, candidate := range candidates {
		if candidate.BaseRevision != baseRevision || candidate.NoteID != current.ID || candidate.SpaceID != candidates[0].SpaceID {
			for _, item := range candidates {
				results[item.ID] = applyConflict(ApplyResult{CandidateID: item.ID})
			}
			return results
		}
	}

	eligible := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.RelatedID != "" && !s.relatedCandidateIsCurrent(ctx, candidate) {
			results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
			continue
		}
		eligible = append(eligible, candidate)
	}
	if len(eligible) == 0 {
		return results
	}

	currentTags, err := s.notes.ListNoteTags(ctx, current.ID)
	if err != nil {
		for _, candidate := range eligible {
			results[candidate.ID] = applyFailure(candidate.ID)
		}
		return results
	}
	currentTagIDs := make([]string, 0, len(currentTags.Tags))
	for _, tag := range currentTags.Tags {
		currentTagIDs = append(currentTagIDs, tag.ID)
	}
	sort.Strings(currentTagIDs)

	tagCandidates := make([]Candidate, 0)
	noteCandidates := make([]Candidate, 0)
	var update note.UpdateInput
	update.ExpectedRevision = &baseRevision
	var trashCandidate *Candidate
	var titleSet, notebookSet bool
	linkTargets := make([]string, 0)
	for _, candidate := range eligible {
		switch candidate.Kind {
		case KindTagAssignment:
			before, ok := candidate.Before["tagIds"].([]string)
			tagID := stringValue(candidate.Proposed["tagId"])
			if !ok || !equalStrings(sortedCopy(before), currentTagIDs) || tagID == "" {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			tagCandidates = append(tagCandidates, candidate)
		case KindTitle:
			before := stringValue(candidate.Before["title"])
			title := stringValue(candidate.Proposed["title"])
			if current.Title != before || title == "" || titleSet {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			update.Title = &title
			titleSet = true
			noteCandidates = append(noteCandidates, candidate)
		case KindNotebookAssignment, KindNotebookMove:
			beforeID := stringValue(candidate.Before["notebookId"])
			proposedID := stringValue(candidate.Proposed["notebookId"])
			matchesBefore := current.NotebookID == nil && beforeID == "" || current.NotebookID != nil && *current.NotebookID == beforeID
			if !matchesBefore || proposedID == "" || notebookSet || !s.notebookTargetIsCurrent(ctx, proposedID) {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			update.NotebookID = &proposedID
			notebookSet = true
			noteCandidates = append(noteCandidates, candidate)
		case KindDuplicateNote, KindEmptyNote:
			contentHash := stringValue(candidate.Before["contentHash"])
			wasTrashed, _ := candidate.Before["isTrashed"].(bool)
			if wasTrashed || contentHash != contentHashOf(current.Content) || trashCandidate != nil {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			copy := candidate
			trashCandidate = &copy
		case KindReciprocalLink:
			contentHash := stringValue(candidate.Before["contentHash"])
			targetID := stringValue(candidate.Proposed["targetId"])
			if contentHash != contentHashOf(current.Content) || targetID == "" || containsString(linkTargets, targetID) {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			if containsString(note.ExtractNoteLinkTargets(current.Content), targetID) {
				results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				continue
			}
			linkTargets = append(linkTargets, targetID)
			noteCandidates = append(noteCandidates, candidate)
		default:
			results[candidate.ID] = ApplyResult{CandidateID: candidate.ID, Status: "not-applicable", Message: "この候補には適用操作がありません。"}
		}
	}
	if trashCandidate != nil && len(noteCandidates) > 0 {
		results[trashCandidate.ID] = applyConflict(ApplyResult{CandidateID: trashCandidate.ID})
		for _, candidate := range noteCandidates {
			results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
		}
		trashCandidate = nil
		noteCandidates = nil
	}

	if len(tagCandidates) > 0 {
		nextIDs := append([]string(nil), currentTagIDs...)
		for _, candidate := range tagCandidates {
			nextIDs = append(nextIDs, stringValue(candidate.Proposed["tagId"]))
		}
		nextIDs = uniqueSorted(nextIDs)
		tagResult, tagErr := s.notes.SetNoteTagsWithExpectedRevision(ctx, current.ID, note.SetNoteTagsWithExpectedRevisionInput{
			TagIDs:           nextIDs,
			ExpectedTagIDs:   currentTagIDs,
			ExpectedRevision: baseRevision,
		})
		if tagErr != nil || tagResult.Error != nil {
			for _, candidate := range tagCandidates {
				if tagResult.Error != nil && tagResult.Error.Code == note.TagErrorStateConflict || tagResult.RevisionConflict != nil {
					results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				} else {
					results[candidate.ID] = applyFailure(candidate.ID)
				}
			}
		} else {
			for _, candidate := range tagCandidates {
				results[candidate.ID] = appliedResult(candidate.ID)
			}
		}
	}

	if trashCandidate != nil {
		trashed := true
		update.IsTrashed = &trashed
		noteCandidates = append(noteCandidates, *trashCandidate)
	}
	if len(noteCandidates) > 0 {
		if len(linkTargets) > 0 {
			content := strings.TrimRight(current.Content, "\n")
			if content != "" {
				content += "\n\n"
			}
			for index, targetID := range linkTargets {
				if index > 0 {
					content += "\n"
				}
				content += "[関連ノート](atlasnote://note/" + targetID + ")"
			}
			content += "\n"
			update.Content = &content
		}
		if _, err := s.notes.Update(ctx, current.ID, update); err != nil {
			var revisionConflict *note.RevisionConflict
			for _, candidate := range noteCandidates {
				if errors.As(err, &revisionConflict) {
					results[candidate.ID] = applyConflict(ApplyResult{CandidateID: candidate.ID})
				} else {
					results[candidate.ID] = applyFailure(candidate.ID)
				}
			}
		} else {
			for _, candidate := range noteCandidates {
				results[candidate.ID] = appliedResult(candidate.ID)
			}
		}
	}
	return results
}

func (s *Service) relatedCandidateIsCurrent(ctx context.Context, candidate Candidate) bool {
	if candidate.RelatedRevision < 1 || candidate.RelatedContentHash == "" {
		return false
	}
	related, err := s.notes.Get(ctx, candidate.RelatedID)
	return err == nil && !related.Protected && !related.Locked && !related.IsTrashed &&
		related.Revision == candidate.RelatedRevision && contentHashOf(related.Content) == candidate.RelatedContentHash
}

func (s *Service) notebookTargetIsCurrent(ctx context.Context, notebookID string) bool {
	notebooks, err := s.notes.ListNotebooks(ctx)
	if err != nil {
		return false
	}
	for _, notebook := range notebooks {
		if notebook.ID == notebookID {
			return !notebook.Protected && !notebook.Locked
		}
	}
	return false
}

func sortedCopy(items []string) []string {
	result := append([]string(nil), items...)
	sort.Strings(result)
	return result
}

func uniqueSorted(items []string) []string {
	result := sortedCopy(items)
	if len(result) < 2 {
		return result
	}
	unique := result[:1]
	for _, item := range result[1:] {
		if item != unique[len(unique)-1] {
			unique = append(unique, item)
		}
	}
	return unique
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func appliedResult(candidateID string) ApplyResult {
	return ApplyResult{CandidateID: candidateID, Status: "applied", Message: "候補を適用しました。"}
}

func applyFailure(candidateID string) ApplyResult {
	return ApplyResult{CandidateID: candidateID, Status: "save-failure", Message: "変更を保存できませんでした。候補は保持されています。"}
}
