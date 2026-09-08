//go:build !windows

package diagnostics

import "os"

func renameAtomic(source string, target string) error {
	return os.Rename(source, target)
}
