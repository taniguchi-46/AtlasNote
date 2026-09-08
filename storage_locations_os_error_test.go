package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"

	"atlasnote/internal/config"
	"atlasnote/internal/diagnostics"
)

func TestStorageLocationOSCausesPreserveSafeContract(t *testing.T) {
	for _, tc := range storageOSCauseCases() {
		t.Run(tc.reason, func(t *testing.T) {
			secret := "private-user-secret-token"
			cause := fmt.Errorf("%s: %w", secret, &os.PathError{Op: "write", Path: "C:/Users/" + secret + "/note.md", Err: tc.err})
			validation := &config.RootValidationError{Code: config.RootErrorNotWritable, Stage: config.RootValidationStageWriteFile, Role: config.RootValidationRoleData, Cause: cause}
			wrapped := fmt.Errorf("outer: %w", validation)
			app := &App{diagnostics: diagnostics.NewStore(t.TempDir(), diagnostics.Metadata{})}
			result := app.storageLocationErrorFor(wrapped, "storage-location.select", "storage-recovery", "data", "")
			if result.Code != "STORAGE_LOCATION_UNWRITABLE" || result.Stage != string(validation.Stage) || result.Role != "data" || result.OSErrorNumber != int(tc.err) || result.Reason != tc.reason || !strings.Contains(result.Action, tc.action) {
				t.Fatalf("unexpected result: %#v", result)
			}
			var rootErr *config.RootValidationError
			var pathErr *os.PathError
			var number syscall.Errno
			if !errors.Is(wrapped, tc.err) || !errors.Is(wrapped, config.ErrRootInvalid) || !errors.As(wrapped, &rootErr) || rootErr != validation || !errors.As(wrapped, &pathErr) || !errors.As(wrapped, &number) || number != tc.err {
				t.Fatal("error chain lost")
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(string(encoded), secret) || strings.Contains(app.GetStorageLocationDiagnostics().Report, secret) {
				t.Fatal("secret leaked")
			}
		})
	}
}
