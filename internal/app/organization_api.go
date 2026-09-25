package app

import (
	"errors"

	"atlasnote/internal/organize"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const organizationProgressEvent = "organization:progress"

type organizationProgressPayload struct {
	RequestID string `json:"requestId"`
	organize.AnalysisProgress
}

func (a *App) AnalyzeOrganization(input organize.AnalysisInput) (organize.Analysis, error) {
	if a.organizer == nil || a.notes == nil {
		return organize.Analysis{}, errors.New("organization service is not initialized")
	}
	if !validOrganizationRequestID(input.RequestID) || a.ctx == nil {
		return a.organizer.Analyze(a.ctx, a.activeSpace.ID, input)
	}
	return a.organizer.Analyze(a.ctx, a.activeSpace.ID, input, func(progress organize.AnalysisProgress) {
		runtime.EventsEmit(a.ctx, organizationProgressEvent, organizationProgressPayload{
			RequestID: input.RequestID, AnalysisProgress: progress,
		})
	})
}

func validOrganizationRequestID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for index := range id {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if id[index] != '-' {
				return false
			}
			continue
		}
		character := id[index]
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
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
