//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
)

func defaultShell() string {
	if shell := os.Getenv("SHELL"); filepath.IsAbs(shell) {
		if info, err := os.Stat(shell); err == nil && !info.IsDir() {
			return shell
		}
	}
	return "/bin/sh"
}
