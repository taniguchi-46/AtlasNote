package datalock_test

import (
	"errors"
	"path/filepath"
	"testing"

	"atlasnote/internal/datalock"
)

func TestAcquireRejectsSecondWriter(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "atlasnote.lock")

	first, err := datalock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := datalock.Acquire(lockPath)
	if second != nil {
		_ = second.Release()
		t.Fatal("second lock was acquired")
	}
	if !errors.Is(err, datalock.ErrAlreadyLocked) {
		t.Fatalf("expected ErrAlreadyLocked, got %v", err)
	}
}

func TestAcquireSucceedsAfterRelease(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "atlasnote.lock")

	first, err := datalock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}

	second, err := datalock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquire second lock: %v", err)
	}
	t.Cleanup(func() { _ = second.Release() })
}

func TestAcquireAllowsDifferentDataDirectories(t *testing.T) {
	root := t.TempDir()
	first, err := datalock.Acquire(filepath.Join(root, "first", "atlasnote.lock"))
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := datalock.Acquire(filepath.Join(root, "second", "atlasnote.lock"))
	if err != nil {
		t.Fatalf("acquire second lock: %v", err)
	}
	t.Cleanup(func() { _ = second.Release() })
}
