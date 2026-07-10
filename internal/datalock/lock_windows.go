//go:build windows

package datalock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func tryLock(file *os.File) error {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		new(windows.Overlapped),
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrAlreadyLocked
	}
	if err != nil {
		return fmt.Errorf("lock data directory: %w", err)
	}
	return nil
}

func unlock(file *os.File) error {
	if err := windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		1,
		0,
		new(windows.Overlapped),
	); err != nil {
		return fmt.Errorf("unlock data directory: %w", err)
	}
	return nil
}
