//go:build !windows

package diagnostics

import "os"

func isUnsafePath(path string, info os.FileInfo) bool {
	_ = path
	return info.Mode()&os.ModeSymlink != 0
}
