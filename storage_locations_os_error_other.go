//go:build !windows

package main

import (
	"errors"
	"syscall"
)

func storageLocationPlatformOSCause(err error) string {
	switch {
	case errors.Is(err, syscall.EBUSY), errors.Is(err, syscall.ETXTBSY):
		return "busy"
	case errors.Is(err, syscall.ENOSPC):
		return "full"
	}
	return ""
}
