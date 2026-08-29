package noteexport

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func writeFileAtomic(path string, content []byte) (err error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".atlasnote-export-*")
	if err != nil {
		return fmt.Errorf("create export temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict export temporary file permissions: %w", err)
	}
	if _, err := io.Copy(temporary, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write export temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync export temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close export temporary file: %w", err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace export file: %w", err)
	}
	committed = true
	return nil
}
