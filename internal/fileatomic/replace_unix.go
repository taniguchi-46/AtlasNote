//go:build !windows

package fileatomic

import "os"

func replaceFile(source string, destination string) error {
	return os.Rename(source, destination)
}
