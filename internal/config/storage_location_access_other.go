//go:build !windows

package config

import "os"

func isUnsafeStoragePath(path string, info os.FileInfo) bool {
	_ = path
	return info.Mode()&os.ModeSymlink != 0
}
