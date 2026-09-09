//go:build windows

package main

import "golang.org/x/sys/windows"

func reportMaintenanceFailurePlatform(message string) {
	text, textErr := windows.UTF16PtrFromString(message)
	caption, captionErr := windows.UTF16PtrFromString("Atlas Note")
	if textErr == nil && captionErr == nil {
		_, _ = windows.MessageBox(0, text, caption, windows.MB_ICONERROR|windows.MB_OK)
	}
}
