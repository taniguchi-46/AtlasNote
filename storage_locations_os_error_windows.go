//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

func storageLocationPlatformOSCause(err error) string {
	switch {
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION), errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return "busy"
	case errors.Is(err, windows.ERROR_DISK_FULL), errors.Is(err, windows.ERROR_HANDLE_DISK_FULL):
		return "full"
	}
	return ""
}
