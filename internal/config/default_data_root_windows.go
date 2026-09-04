//go:build windows

package config

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func defaultDataRootFromPlatform() (string, error) {
	if documents, err := windows.KnownFolderPath(windows.FOLDERID_LocalDocuments, 0); err == nil && documents != "" {
		return filepath.Clean(filepath.Join(documents, "AtlasNote")), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(home, "Documents", "AtlasNote")), nil
}
