package localipc

import (
	"os"
	"path/filepath"
)

func writePrivateAtomic(path string, content []byte) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".atlasnote-ipc-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := restrictDescriptor(temporaryPath); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replacePrivateFile(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}
