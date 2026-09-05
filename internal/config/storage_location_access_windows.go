//go:build windows

package config

import (
	"os"

	"golang.org/x/sys/windows"
)

func isUnsafeStoragePath(path string, info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	attributes, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path))
	return err == nil && attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
