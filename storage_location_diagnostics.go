package main

import (
	"errors"

	"atlasnote/internal/config"
	"atlasnote/internal/diagnostics"
)

type StorageLocationDiagnostic struct {
	Schema        int    `json:"schema"`
	Timestamp     string `json:"timestamp"`
	DiagnosticID  string `json:"diagnosticId"`
	Operation     string `json:"operation"`
	Phase         string `json:"phase,omitempty"`
	Role          string `json:"role,omitempty"`
	Code          string `json:"code"`
	Reason        string `json:"reason,omitempty"`
	Stage         string `json:"stage,omitempty"`
	OSErrorNumber int    `json:"osErrorNumber,omitempty"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	AppVersion    string `json:"appVersion"`
	VCSRevision   string `json:"vcsRevision"`
}

type StorageLocationDiagnosticsResult struct {
	Events []StorageLocationDiagnostic `json:"events"`
	Report string                      `json:"report"`
}

func (a *App) GetStorageLocationDiagnostics() StorageLocationDiagnosticsResult {
	if a.diagnostics == nil {
		return StorageLocationDiagnosticsResult{Events: []StorageLocationDiagnostic{}, Report: diagnostics.FormatReport(nil)}
	}
	events := a.diagnostics.Events()
	result := StorageLocationDiagnosticsResult{Events: make([]StorageLocationDiagnostic, 0, len(events)), Report: diagnostics.FormatReport(events)}
	for _, event := range events {
		result.Events = append(result.Events, StorageLocationDiagnostic{
			Schema: event.Schema, Timestamp: event.Timestamp, DiagnosticID: event.DiagnosticID,
			Operation: event.Operation, Phase: event.Phase, Role: event.Role, Code: event.Code,
			Reason: event.Reason, Stage: event.Stage, OSErrorNumber: event.OSErrorNumber,
			OS: event.OS, Arch: event.Arch, AppVersion: event.AppVersion, VCSRevision: event.VCSRevision,
		})
	}
	return result
}

func (a *App) storageLocationErrorFor(err error, operation string, phase string, role string, onceKey string) *StorageLocationError {
	result := storageLocationError(err)
	if result == nil {
		return nil
	}
	if result.Role == "" {
		result.Role = role
	}
	if a.diagnostics == nil {
		return result
	}
	event := diagnostics.Event{
		Operation: operation, Phase: phase, Role: result.Role, Code: result.Code,
		Reason: result.Reason, Stage: result.Stage, OSErrorNumber: result.OSErrorNumber,
	}
	var recorded diagnostics.Event
	if onceKey != "" {
		recorded = a.diagnostics.RecordOnce(onceKey, event)
	} else {
		recorded = a.diagnostics.Record(event)
	}
	result.DiagnosticID = recorded.DiagnosticID
	return result
}

func storageLocationStatusDiagnosticKey(err error) string {
	apiError := storageLocationError(err)
	if apiError == nil {
		return "storage-location-status"
	}
	return "storage-location-status|" + apiError.Code + "|" + apiError.Stage + "|" + apiError.Role
}

func storageLocationRole(err error) string {
	apiError := storageLocationError(err)
	if apiError != nil && apiError.Role != "" {
		return apiError.Role
	}
	var validationErr *config.RootValidationError
	if err != nil && errorsAsRootValidation(err, &validationErr) {
		return string(validationErr.Role)
	}
	return string(config.RootValidationRoleUnknown)
}

func (a *App) storageLocationFailure(err error, operation string, role string) *StorageLocationError {
	return a.storageLocationErrorFor(err, operation, string(a.startupPhase), role, "")
}

func (a *App) storageLocationStatusFailure(err error) *StorageLocationError {
	return a.storageLocationErrorFor(
		err,
		"storage-location.status",
		string(a.startupPhase),
		storageLocationRole(err),
		storageLocationStatusDiagnosticKey(err),
	)
}

func (a *App) storageLocationSelectionFailure(kind string, path string, err error) StorageLocationSelectionResult {
	status, _ := a.storageLocationStatus()
	return StorageLocationSelectionResult{
		Kind: kind, Path: path, Status: &status,
		Error: a.storageLocationFailure(err, "storage-location.select", kind),
	}
}

func (a *App) storageLocationMutationFailure(err error, operation string, role string) StorageLocationMutationResult {
	status, _ := a.storageLocationStatus()
	return StorageLocationMutationResult{
		Status: &status,
		Error:  a.storageLocationFailure(err, operation, role),
	}
}

func errorsAsRootValidation(err error, target **config.RootValidationError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, target)
}
