//go:build windows

package appcleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type pinnedEntry struct {
	handle windows.Handle
	remove bool
}
type preparedDeletion struct{ entries []pinnedEntry }

func (p *preparedDeletion) Close() {
	for i := len(p.entries) - 1; i >= 0; i-- {
		if p.entries[i].handle != 0 {
			_ = windows.CloseHandle(p.entries[i].handle)
			p.entries[i].handle = 0
		}
	}
}

func (p *preparedDeletion) pin(path string, remove bool) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	access := uint32(windows.FILE_READ_ATTRIBUTES)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if remove {
		access |= windows.DELETE | windows.GENERIC_READ
		share = windows.FILE_SHARE_READ
	}
	h, err := windows.CreateFile(name, access, share, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Errorf("pin %s: %w", path, err)
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(h)
		return ErrUnsafeCleanupPath
	}
	if remove && (info.NumberOfLinks > 1 || info.FileAttributes&windows.FILE_ATTRIBUTE_READONLY != 0) {
		_ = windows.CloseHandle(h)
		return ErrUnsafeCleanupPath
	}
	p.entries = append(p.entries, pinnedEntry{h, remove})
	return nil
}

func prepareCacheDeletion(root string) (_ *preparedDeletion, err error) {
	p := &preparedDeletion{}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	// Pin ancestors from the volume downwards before resolving any descendants.
	ancestors := []string{}
	for path := root; ; path = filepath.Dir(path) {
		ancestors = append(ancestors, path)
		if filepath.Dir(path) == path {
			break
		}
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		if err = p.pin(ancestors[i], false); errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return p, nil
		}
		if err != nil {
			return nil, err
		}
	}
	pinned := map[string]bool{root: true}
	for _, relative := range cacheDirectories {
		target := filepath.Join(root, filepath.FromSlash(relative))
		parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
		parent := root
		missing := false
		for _, part := range parts[:len(parts)-1] {
			parent = filepath.Join(parent, part)
			if pinned[parent] {
				continue
			}
			if err = p.pin(parent, false); errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
				missing = true
				break
			}
			if err != nil {
				return nil, err
			}
			pinned[parent] = true
		}
		if missing {
			continue
		}
		if err = p.pin(target, true); errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			continue
		}
		if err != nil {
			return nil, err
		}
		err = p.walkCache(target, "", strings.HasSuffix(relative, "leveldb"))
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *preparedDeletion) Remove() error {
	for i := len(p.entries) - 1; i >= 0; i-- {
		entry := &p.entries[i]
		if !entry.remove || entry.handle == 0 {
			continue
		}
		// Delete the validated object by handle, never resolve its path again.
		disposition := byte(1)
		if err := windows.SetFileInformationByHandle(entry.handle, windows.FileDispositionInfo, &disposition, 1); err != nil {
			return err
		}
		if err := windows.CloseHandle(entry.handle); err != nil {
			return err
		}
		entry.handle = 0
	}
	return nil
}

func (p *preparedDeletion) walkCache(path, relative string, levelDB bool) error {
	h := p.entries[len(p.entries)-1].handle
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return ErrUnsafeCleanupPath
	}
	var duplicate windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), h, windows.CurrentProcess(), &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	directory := os.NewFile(uintptr(duplicate), path)
	children, err := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for _, child := range children {
		rel := filepath.Join(relative, child.Name())
		if !knownCacheEntry(rel, child.IsDir(), levelDB) {
			return ErrUnsafeCleanupPath
		}
		childPath := filepath.Join(path, child.Name())
		if err := p.pin(childPath, true); err != nil {
			return err
		}
		if child.IsDir() {
			if err := p.walkCache(childPath, rel, levelDB); err != nil {
				return err
			}
		}
	}
	return nil
}
