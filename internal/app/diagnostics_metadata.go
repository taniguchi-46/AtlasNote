package app

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"atlasnote/internal/diagnostics"
)

func appDiagnosticsMetadata(productVersion string) diagnostics.Metadata {
	metadata := diagnostics.Metadata{AppVersion: "unknown", VCSRevision: "unknown"}
	if strings.TrimSpace(productVersion) != "" {
		metadata.AppVersion = productVersion
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" && strings.TrimSpace(setting.Value) != "" {
				metadata.VCSRevision = setting.Value
				break
			}
		}
	}
	return metadata
}

func newAppDiagnosticsStore(productVersion string) *diagnostics.Store {
	metadata := appDiagnosticsMetadata(productVersion)
	if executable := strings.ToLower(filepath.Base(os.Args[0])); strings.HasSuffix(executable, ".test") || strings.HasSuffix(executable, ".test.exe") {
		if configured := strings.TrimSpace(os.Getenv("ATLAS_NOTE_DIAGNOSTICS_DIR")); configured != "" {
			return diagnostics.NewStore(configured, metadata)
		}
		return diagnostics.NewStore("", metadata)
	}
	return diagnostics.NewDefaultStore(metadata)
}
