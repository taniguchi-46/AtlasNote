package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"atlasnote/internal/diagnostics"
)

// wailsConfigBytes is embedded so the diagnostic metadata uses the product
// version maintained by the application configuration instead of a second
// hardcoded version string.
//
//go:embed wails.json
var wailsConfigBytes []byte

type embeddedWailsConfig struct {
	Info struct {
		ProductVersion string `json:"productVersion"`
	} `json:"info"`
}

func appDiagnosticsMetadata() diagnostics.Metadata {
	metadata := diagnostics.Metadata{AppVersion: "unknown", VCSRevision: "unknown"}
	var config embeddedWailsConfig
	if err := json.Unmarshal(wailsConfigBytes, &config); err == nil && strings.TrimSpace(config.Info.ProductVersion) != "" {
		metadata.AppVersion = config.Info.ProductVersion
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

func newAppDiagnosticsStore() *diagnostics.Store {
	metadata := appDiagnosticsMetadata()
	if executable := strings.ToLower(filepath.Base(os.Args[0])); strings.HasSuffix(executable, ".test") || strings.HasSuffix(executable, ".test.exe") {
		if configured := strings.TrimSpace(os.Getenv("ATLAS_NOTE_DIAGNOSTICS_DIR")); configured != "" {
			return diagnostics.NewStore(configured, metadata)
		}
		return diagnostics.NewStore("", metadata)
	}
	return diagnostics.NewDefaultStore(metadata)
}
