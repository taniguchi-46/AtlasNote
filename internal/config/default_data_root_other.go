//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

func defaultDataRootFromPlatform() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(userConfigDir, "AtlasNote")), nil
}
