package datalock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrAlreadyLocked = errors.New("data directory is already in use")

type Lock struct {
	mu   sync.Mutex
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open data directory lock: %w", err)
	}
	if err := tryLock(file); err != nil {
		_ = file.Close()
		return nil, err
	}

	return &Lock{file: file}, nil
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}

	file := l.file
	l.file = nil
	if err := unlock(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
