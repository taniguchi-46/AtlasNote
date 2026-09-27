package terminal

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultShell() string {
	if shell := os.Getenv("COMSPEC"); filepath.IsAbs(shell) && strings.EqualFold(filepath.Ext(shell), ".exe") {
		if info, err := os.Stat(shell); err == nil && !info.IsDir() {
			return shell
		}
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "cmd.exe")
}
