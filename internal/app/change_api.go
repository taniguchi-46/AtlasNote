package app

import "atlasnote/internal/readapi"

// These methods are Wails GUI bindings. They are intentionally absent from
// local IPC, CLI and MCP tool dispatch.
func (a *App) ListExternalChangeReviews() []readapi.ChangeReview {
	if a.changeService == nil || a.notes == nil || a.activeSpace.ID == "" {
		return nil
	}
	return a.changeService.ListChangeReviews(a.activeSpace.ID)
}

func (a *App) ApproveExternalChange(operationID string) readapi.ChangeResult {
	if a.changeService == nil || a.notes == nil || a.activeSpace.ID == "" {
		return readapi.ChangeResult{OperationID: operationID, State: "unavailable"}
	}
	return a.changeService.ApproveChange(a.operationContext(), a.activeSpace.ID, operationID)
}

func (a *App) RejectExternalChange(operationID string) readapi.ChangeResult {
	if a.changeService == nil || a.notes == nil || a.activeSpace.ID == "" {
		return readapi.ChangeResult{OperationID: operationID, State: "unavailable"}
	}
	return a.changeService.RejectChange(a.activeSpace.ID, operationID)
}
