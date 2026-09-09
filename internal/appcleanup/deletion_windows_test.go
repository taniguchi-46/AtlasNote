//go:build windows

package appcleanup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareDedicatedCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "AtlasNote.exe")
	cache := filepath.Join(root, filepath.FromSlash(cacheDirectories[0]))
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "CURRENT"), []byte("MANIFEST-000001"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := prepareCacheDeletion(root)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = p.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache remains: %v", err)
	}
}

func TestPinnedCacheRejectsPathReplacement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "AtlasNote.exe")
	cache := filepath.Join(root, filepath.FromSlash(cacheDirectories[0]))
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cache, "CURRENT")
	if err := os.WriteFile(file, []byte("cache"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := prepareCacheDeletion(root)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, path := range []string{root, filepath.Dir(cache), cache, file} {
		if err := os.Rename(path, path+"-swapped"); err == nil {
			t.Fatalf("pinned object could be replaced: %s", path)
		}
	}
	if err := os.WriteFile(file, []byte("replacement"), 0600); err == nil {
		t.Fatal("pinned file could be overwritten")
	}
	if err := p.Remove(); err != nil {
		t.Fatal(err)
	}
}
func TestAncestorJunctionRejectedBeforeCredentialAccess(t *testing.T) {
	dir := t.TempDir()
	identity := testIdentity(filepath.Join(dir, "profile"))
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(identity.ProfileDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Junctions do not require Developer Mode, unlike symbolic links.
	command := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(identity.ProfileDir, "AppData"), outside)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture: %v %s", err, output)
	}
	marker := filepath.Join(outside, "keep.md")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	result := newFixtureService().Run(context.Background(), Request{Identity: identity, DataRoots: []string{}, DeleteDisplaySettings: true, DeleteCredentials: true})
	if result.Error == nil {
		t.Fatal("ancestor junction accepted")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatal("junction target changed")
	}
}
func TestApplicationActivityAllowsSpacesAndExcludesCleanup(t *testing.T) {
	scope := fmt.Sprintf(`Local\AtlasNote.Test.%d.%d`, os.Getpid(), time.Now().UnixNano())
	first, err := acquireActivityLock(scope, false)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	second, err := acquireActivityLock(scope, false)
	if err != nil {
		t.Fatal("independent spaces blocked", err)
	}
	defer second.Release()
	if cleanup, err := acquireActivityLock(scope, true); err == nil {
		cleanup.Release()
		t.Fatal("cleanup while app running")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if cleanup, err := acquireActivityLock(scope, true); err == nil {
		cleanup.Release()
		t.Fatal("second app not protected")
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	cleanup, err := acquireActivityLock(scope, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup.Release()
	if app, err := acquireActivityLock(scope, false); err == nil {
		app.Release()
		t.Fatal("app started during cleanup")
	}
	if err := cleanup.Release(); err != nil {
		t.Fatal(err)
	}
	if err := cleanup.Release(); err != nil {
		t.Fatal("duplicate release", err)
	}
	child, err := acquireActivityLock(scope, false)
	if err != nil {
		t.Fatal("immediate child could not acquire", err)
	}
	child.Release()
}
