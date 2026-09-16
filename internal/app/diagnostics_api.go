package app

import (
	"path/filepath"
	"strings"

	"atlasnote/internal/diagnostics"
	"atlasnote/internal/noteexport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// OperationFailureInput is intentionally metadata-only. The frontend sends
// fixed classifications; the diagnostics store collapses unknown values to
// fixed fallbacks before persistence.
type OperationFailureInput struct {
	Operation     string `json:"operation"`
	Phase         string `json:"phase"`
	Role          string `json:"role"`
	Stage         string `json:"stage"`
	ErrorCategory string `json:"errorCategory"`
}

type DiagnosticsSaveResult struct {
	Saved     bool   `json:"saved"`
	Cancelled bool   `json:"cancelled"`
	SavedName string `json:"savedName,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GetDiagnostics exposes the same bounded, safe report used by the storage
// location screen under a feature-neutral API name for the Help screen.
func (a *App) GetDiagnostics() StorageLocationDiagnosticsResult {
	return a.GetStorageLocationDiagnostics()
}

// RecordOperationFailure is best-effort and never returns an application
// error. It deliberately does not accept an Error, message, path, note body,
// title, prompt, or tool trace.
func (a *App) RecordOperationFailure(input OperationFailureInput) StorageLocationDiagnostic {
	if a.diagnostics == nil {
		return StorageLocationDiagnostic{}
	}
	event := a.diagnostics.RecordFailure(diagnostics.FailureInput{
		Operation:     input.Operation,
		Phase:         input.Phase,
		Role:          input.Role,
		Stage:         input.Stage,
		ErrorCategory: input.ErrorCategory,
	})
	return storageLocationDiagnosticFromEvent(event)
}

func (a *App) SaveDiagnostics() DiagnosticsSaveResult {
	if a.diagnostics == nil {
		return DiagnosticsSaveResult{Error: "診断情報を保存できませんでした。"}
	}
	if !a.exportMu.TryLock() {
		return DiagnosticsSaveResult{Error: "別のファイル保存処理が実行中です。"}
	}
	defer a.exportMu.Unlock()

	options := runtime.SaveDialogOptions{
		Title:                "診断情報を保存",
		DefaultFilename:      "atlasnote-diagnostics.json",
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{{
			DisplayName: "JSONファイル (*.json)",
			Pattern:     "*.json",
		}},
	}
	selectFile := a.saveDiagnosticsFile
	if selectFile == nil {
		selectFile = runtime.SaveFileDialog
	}
	path, err := selectFile(a.operationContext(), options)
	if err != nil {
		return DiagnosticsSaveResult{Error: "診断情報の保存先を選択できませんでした。"}
	}
	if strings.TrimSpace(path) == "" {
		return DiagnosticsSaveResult{Cancelled: true}
	}

	report := diagnostics.FormatReport(a.diagnostics.Events())
	if err := noteexport.WriteFileAtomic(path, []byte(report)); err != nil {
		return DiagnosticsSaveResult{Error: "診断情報をファイルへ保存できませんでした。"}
	}
	return DiagnosticsSaveResult{Saved: true, SavedName: filepath.Base(path)}
}
