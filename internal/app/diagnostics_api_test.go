package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/diagnostics"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestRecordOperationFailureStoresOnlyFixedMetadata(t *testing.T) {
	app := &App{diagnostics: diagnostics.NewStore("", diagnostics.Metadata{AppVersion: "0.1.0"})}
	secretMarker := "diagnostics-secret-marker"

	result := app.RecordOperationFailure(OperationFailureInput{
		Operation:     "frontend/" + secretMarker,
		Phase:         "phase/" + secretMarker,
		Role:          "role/" + secretMarker,
		Stage:         secretMarker,
		ErrorCategory: secretMarker,
	})

	if result.Code != diagnostics.FailureCodeOperation || result.Operation != "frontend" || result.Phase != "runtime" || result.Role != "frontend" {
		t.Fatalf("normalized operation failure = %#v", result)
	}
	if result.Stage != "unknown-stage" || result.Reason != "unknown-category" {
		t.Fatalf("unknown operation failure enums = %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal operation failure = %v", err)
	}
	if strings.Contains(string(encoded), secretMarker) {
		t.Fatalf("operation failure response leaked secret marker: %s", encoded)
	}
	if strings.Contains(app.GetDiagnostics().Report, secretMarker) {
		t.Fatal("diagnostics report leaked secret marker")
	}
}

func TestSaveDiagnosticsUsesNativeDialogAndAtomicOutput(t *testing.T) {
	app := &App{diagnostics: diagnostics.NewStore("", diagnostics.Metadata{AppVersion: "0.1.0"})}
	app.RecordOperationFailure(OperationFailureInput{
		Operation:     "frontend",
		Phase:         "runtime",
		Role:          "frontend",
		Stage:         "note-editor.mermaid-paste",
		ErrorCategory: "parse-failed",
	})

	target := filepath.Join(t.TempDir(), "diagnostics.json")
	var selected runtime.SaveDialogOptions
	app.saveDiagnosticsFile = func(_ context.Context, options runtime.SaveDialogOptions) (string, error) {
		selected = options
		return target, nil
	}

	result := app.SaveDiagnostics()
	if !result.Saved || result.Cancelled || result.Error != "" || result.SavedName != "diagnostics.json" {
		t.Fatalf("saved diagnostics result = %#v", result)
	}
	if selected.Title != "診断情報を保存" || selected.DefaultFilename != "atlasnote-diagnostics.json" || !selected.CanCreateDirectories {
		t.Fatalf("diagnostics save dialog options = %#v", selected)
	}
	if len(selected.Filters) != 1 || selected.Filters[0].Pattern != "*.json" {
		t.Fatalf("diagnostics save dialog filter = %#v", selected.Filters)
	}

	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read diagnostics report: %v", err)
	}
	if !json.Valid(contents) || !strings.Contains(string(contents), `"schema": 1`) || !strings.Contains(string(contents), `"events"`) {
		t.Fatalf("invalid diagnostics report: %s", contents)
	}
}

func TestSaveDiagnosticsTreatsCancelAndDialogFailureAsSafeResults(t *testing.T) {
	app := &App{diagnostics: diagnostics.NewStore("", diagnostics.Metadata{})}

	app.saveDiagnosticsFile = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		return "", nil
	}
	cancelled := app.SaveDiagnostics()
	if !cancelled.Cancelled || cancelled.Saved || cancelled.Error != "" {
		t.Fatalf("cancelled diagnostics result = %#v", cancelled)
	}

	app.saveDiagnosticsFile = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		return "", errors.New("raw dialog failure with a path")
	}
	failed := app.SaveDiagnostics()
	if failed.Cancelled || failed.Saved || failed.Error != "診断情報の保存先を選択できませんでした。" {
		t.Fatalf("failed diagnostics result = %#v", failed)
	}
	if strings.Contains(failed.Error, "raw dialog failure") {
		t.Fatal("diagnostics dialog error exposed raw error text")
	}
}
