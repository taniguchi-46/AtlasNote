package app

import (
	"errors"

	"atlasnote/internal/organize"
)

func (a *App) AnalyzeOrganization(input organize.AnalysisInput) (organize.Analysis, error) {
	if a.organizer == nil || a.notes == nil {
		return organize.Analysis{}, errors.New("organization service is not initialized")
	}
	return a.organizer.Analyze(a.ctx, a.activeSpace.ID, input)
}

func (a *App) ApplyOrganizationCandidate(input organize.ApplyInput) organize.ApplyResult {
	if a.organizer == nil || a.notes == nil {
		return organize.ApplyResult{
			CandidateID: input.CandidateID,
			Status:      "save-failure",
			Message:     "整理サービスを利用できません。",
		}
	}
	return a.organizer.Apply(a.ctx, a.activeSpace.ID, input)
}

func (a *App) ApplyOrganizationCandidates(input organize.ApplyCandidatesInput) []organize.ApplyResult {
	if a.organizer == nil || a.notes == nil {
		results := make([]organize.ApplyResult, 0, len(input.CandidateIDs))
		for _, candidateID := range input.CandidateIDs {
			results = append(results, organize.ApplyResult{
				CandidateID: candidateID,
				Status:      "save-failure",
				Message:     "整理サービスを利用できません。",
			})
		}
		return results
	}
	return a.organizer.ApplyCandidates(a.ctx, a.activeSpace.ID, input)
}

func (a *App) InvalidateOrganizationAnalyses() {
	if a.organizer != nil {
		a.organizer.InvalidateSessions()
	}
}

func (a *App) DiscardOrganizationAnalysis(sessionID string) {
	if a.organizer != nil {
		a.organizer.DiscardSession(sessionID)
	}
}
