//go:build !windows

package localipc

import "os"

func replacePrivateFile(source, destination string) error {
	return os.Rename(source, destination)
}
