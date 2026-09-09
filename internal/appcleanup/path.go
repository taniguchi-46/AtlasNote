package appcleanup

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func knownWebViewUserDataPath(appDataDir string) (string, error) {
	if !filepath.IsAbs(appDataDir) {
		return "", ErrUnsafeCleanupPath
	}
	return filepath.Join(appDataDir, "AtlasNote.exe"), nil
}

func validateCleanupSeparation(target string, roots []string) error {
	if err := validateUnlinkedPaths(append([]string{target}, roots...)); err != nil {
		return err
	}
	var err error
	target, err = canonicalExistingAncestor(target)
	if err != nil {
		return err
	}
	for _, root := range roots {
		root, err = canonicalExistingAncestor(root)
		if err != nil {
			return err
		}
		if containsPath(root, target) || containsPath(target, root) {
			return ErrUnsafeCleanupPath
		}
	}
	return nil
}

func validateUnlinkedPaths(paths []string) error {
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return ErrUnsafeCleanupPath
		}
		for current := filepath.Clean(path); ; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil && isUnsafeFileInfo(current, info) {
				return ErrUnsafeCleanupPath
			}
			if filepath.Dir(current) == current {
				break
			}
		}
	}
	return nil
}

func canonicalExistingAncestor(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) || filepath.Dir(path) == path {
		return "", ErrUnsafeCleanupPath
	}
	parent, err := canonicalExistingAncestor(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func containsPath(parent, child string) bool {
	parent = strings.ToLower(filepath.Clean(parent))
	child = strings.ToLower(filepath.Clean(child))
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Only these WebView-owned leaf directories are removed. Other profile data,
// old storage layouts and unrecognized files are retained, never inferred to
// be disposable merely because they reside beneath AtlasNote.exe.
var cacheDirectories = []string{
	"EBWebView/Default/Local Storage/leveldb",
	"EBWebView/Default/Cache/Cache_Data",
	"EBWebView/Default/Code Cache/js",
	"EBWebView/Default/Code Cache/wasm",
	"EBWebView/Default/GPUCache",
}
var levelDBFile = regexp.MustCompile(`^(CURRENT|LOCK|LOG(\.old)?|MANIFEST-[0-9]+|[0-9]+\.(log|ldb|sst))$`)
var cacheFile = regexp.MustCompile(`^(index|index-dir|the-real-index|data_[0-9]+|f_[0-9a-f]+|[0-9a-f]{16}_[0-9]+)$`)

func knownCacheEntry(relative string, directory bool, levelDB bool) bool {
	relative = filepath.ToSlash(relative)
	if levelDB {
		return !directory && levelDBFile.MatchString(relative)
	}
	if directory {
		return relative == "index-dir"
	}
	if strings.HasPrefix(relative, "index-dir/") {
		return relative == "index-dir/the-real-index"
	}
	return cacheFile.MatchString(relative) && relative != "index-dir"
}
