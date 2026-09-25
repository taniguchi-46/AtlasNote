//go:build !windows

package localipc

import (
	"errors"
	"os"
)

func restrictDescriptor(path string) error {
	return os.Chmod(path, 0o600)
}

func validateDescriptorSecurity(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("IPC descriptor permissions are too broad")
	}
	return nil
}
