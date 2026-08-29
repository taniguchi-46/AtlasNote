//go:build !windows

package noteexport

import "os"

func replaceFile(source string, destination string) error {
	return os.Rename(source, destination)
}
