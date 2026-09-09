//go:build !windows

package appcleanup

import "os"

func isUnsafeFileInfo(_ string, info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}
