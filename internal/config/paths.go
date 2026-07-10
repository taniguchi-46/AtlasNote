package config

import (
	"os"
	"path/filepath"
)

const dataDirEnv = "ATLAS_NOTE_DATA_DIR"

type Paths struct {
	DataDir      string
	DatabasePath string
	NotesDir     string
	LockPath     string
}

func LoadPaths() (Paths, error) {
	dataDir := os.Getenv(dataDirEnv)
	if dataDir == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, err
		}
		dataDir = filepath.Join(userConfigDir, "AtlasNote")
	}

	dataDir = filepath.Clean(dataDir)

	return Paths{
		DataDir:      dataDir,
		DatabasePath: filepath.Join(dataDir, "atlasnote.db"),
		NotesDir:     filepath.Join(dataDir, "notes"),
		LockPath:     filepath.Join(dataDir, "atlasnote.lock"),
	}, nil
}
