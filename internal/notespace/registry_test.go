package notespace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestOpenBootstrapsLegacySpaceWithoutMovingExistingData(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "atlasnote.db")
	notesDir := filepath.Join(root, "notes")
	notePath := filepath.Join(notesDir, "existing.md")
	if err := os.MkdirAll(notesDir, 0o700); err != nil {
		t.Fatalf("create notes directory: %v", err)
	}
	if err := os.WriteFile(databasePath, []byte("existing-database"), 0o600); err != nil {
		t.Fatalf("write database fixture: %v", err)
	}
	if err := os.WriteFile(notePath, []byte("existing-note"), 0o600); err != nil {
		t.Fatalf("write note fixture: %v", err)
	}

	registry, err := Open(root)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	result, err := registry.List()
	if err != nil {
		t.Fatalf("list spaces: %v", err)
	}
	if len(result.Spaces) != 1 || result.Spaces[0].Name != legacySpaceName || !result.Spaces[0].Legacy || !result.Spaces[0].Active {
		t.Fatalf("bootstrapped spaces = %#v", result)
	}
	active, dataDir, err := registry.Active()
	if err != nil {
		t.Fatalf("resolve active space: %v", err)
	}
	if active.ID != result.ActiveSpaceID || dataDir != filepath.Clean(root) {
		t.Fatalf("active = %#v, data dir = %q", active, dataDir)
	}
	assertFileContent(t, databasePath, "existing-database")
	assertFileContent(t, notePath, "existing-note")
	if _, err := os.Stat(filepath.Join(root, spacesDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bootstrap created or changed nested spaces directory: %v", err)
	}
	catalogInfo, err := os.Stat(filepath.Join(root, catalogFileName))
	if err != nil {
		t.Fatalf("stat catalog: %v", err)
	}
	if runtime.GOOS != "windows" && catalogInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("catalog permissions = %o", catalogInfo.Mode().Perm())
	}
}

func TestCreateUsesGeneratedInternalDirectoryAndKeepsCurrentActive(t *testing.T) {
	registry, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	before, err := registry.List()
	if err != nil {
		t.Fatalf("list initial spaces: %v", err)
	}

	created, activeSpaceID, err := registry.Create(t.Context(), " 仕事 ", createPreparedDirectory)
	if err != nil {
		t.Fatalf("create space: %v", err)
	}
	if created.Name != "仕事" || created.Legacy || created.Active {
		t.Fatalf("created space = %#v", created)
	}
	if !spaceIDPattern.MatchString(created.ID) {
		t.Fatalf("created id = %q", created.ID)
	}
	if activeSpaceID != before.ActiveSpaceID {
		t.Fatalf("active id changed from %q to %q", before.ActiveSpaceID, activeSpaceID)
	}
	markerPath := filepath.Join(registry.root, spacesDirectory, created.ID, "prepared")
	assertFileContent(t, markerPath, created.ID)
	if _, _, err := registry.Create(t.Context(), "仕事", createPreparedDirectory); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate name error = %v", err)
	}
}

func TestDataDirResolvesExistingInactiveSpaceWithoutSelectingIt(t *testing.T) {
	registry, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	initial, err := registry.List()
	if err != nil {
		t.Fatalf("list initial spaces: %v", err)
	}
	created, _, err := registry.Create(t.Context(), "個人", createPreparedDirectory)
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	space, dataDir, err := registry.DataDir(created.ID)
	if err != nil {
		t.Fatalf("resolve inactive data directory: %v", err)
	}
	if space.ID != created.ID || space.Active || dataDir != filepath.Join(registry.root, spacesDirectory, created.ID) {
		t.Fatalf("resolved space = %#v, data directory = %q", space, dataDir)
	}
	after, err := registry.List()
	if err != nil {
		t.Fatalf("list after data directory resolution: %v", err)
	}
	if after.ActiveSpaceID != initial.ActiveSpaceID {
		t.Fatalf("data directory resolution changed active space to %q", after.ActiveSpaceID)
	}
	if _, _, err := registry.DataDir("not-a-space-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid data directory lookup error = %v", err)
	}
}

func TestCreateReleasesCatalogLockDuringPreflight(t *testing.T) {
	root := t.TempDir()
	registry, err := Open(root)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	secondRegistry, err := Open(root)
	if err != nil {
		t.Fatalf("open second registry: %v", err)
	}
	preflightStarted := make(chan struct{})
	releasePreflight := make(chan struct{})
	createDone := make(chan error, 1)
	go func() {
		_, _, err := registry.Create(context.Background(), "仕事", func(ctx context.Context, dataDir string) error {
			if err := createPreparedDirectory(ctx, dataDir); err != nil {
				return err
			}
			close(preflightStarted)
			<-releasePreflight
			return nil
		})
		createDone <- err
	}()
	<-preflightStarted

	listDone := make(chan error, 1)
	go func() {
		result, err := secondRegistry.List()
		if err == nil && len(result.Spaces) != 1 {
			err = errors.New("uncommitted storage space became visible")
		}
		listDone <- err
	}()
	select {
	case err := <-listDone:
		if err != nil {
			t.Fatalf("list during preflight: %v", err)
		}
	case <-time.After(2 * time.Second):
		close(releasePreflight)
		t.Fatal("catalog remained locked during storage-space preflight")
	}
	close(releasePreflight)
	if err := <-createDone; err != nil {
		t.Fatalf("complete create: %v", err)
	}
}

func TestCreateNormalizesNamesAndRejectsFormattingControls(t *testing.T) {
	registry, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	created, _, err := registry.Create(t.Context(), "Cafe\u0301", createPreparedDirectory)
	if err != nil {
		t.Fatalf("create normalized name: %v", err)
	}
	if created.Name != "Café" {
		t.Fatalf("normalized name = %q", created.Name)
	}
	if _, _, err := registry.Create(t.Context(), "Café", createPreparedDirectory); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("normalized duplicate error = %v", err)
	}
	if _, _, err := registry.Create(t.Context(), "仕事\u202e", createPreparedDirectory); !errors.Is(err, ErrNameInvalid) {
		t.Fatalf("formatting control error = %v", err)
	}
}

func TestSelectPersistsOnlyAfterTargetPreflightSucceeds(t *testing.T) {
	registry, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	initial, err := registry.List()
	if err != nil {
		t.Fatalf("list initial spaces: %v", err)
	}
	created, _, err := registry.Create(t.Context(), "仕事", createPreparedDirectory)
	if err != nil {
		t.Fatalf("create space: %v", err)
	}
	preflightFailure := errors.New("preflight failure")
	if _, _, err := registry.Select(t.Context(), created.ID, func(context.Context, string) error {
		return preflightFailure
	}); !errors.Is(err, preflightFailure) {
		t.Fatalf("select failure = %v", err)
	}
	afterFailure, err := registry.List()
	if err != nil {
		t.Fatalf("list after failed selection: %v", err)
	}
	if afterFailure.ActiveSpaceID != initial.ActiveSpaceID {
		t.Fatalf("failed preflight changed active space to %q", afterFailure.ActiveSpaceID)
	}

	selected, restartRequired, err := registry.Select(t.Context(), created.ID, func(_ context.Context, dataDir string) error {
		if dataDir != filepath.Join(registry.root, spacesDirectory, created.ID) {
			t.Fatalf("preflight data dir = %q", dataDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("select space: %v", err)
	}
	if !restartRequired || !selected.Active {
		t.Fatalf("selection = %#v, restart = %v", selected, restartRequired)
	}

	reopened, err := Open(registry.root)
	if err != nil {
		t.Fatalf("reopen registry: %v", err)
	}
	active, dataDir, err := reopened.Active()
	if err != nil {
		t.Fatalf("resolve persisted active space: %v", err)
	}
	if active.ID != created.ID || dataDir != filepath.Join(registry.root, spacesDirectory, created.ID) {
		t.Fatalf("persisted active = %#v, data dir = %q", active, dataDir)
	}
}

func TestCorruptCatalogFailsWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	if _, err := Open(root); err != nil {
		t.Fatalf("bootstrap registry: %v", err)
	}
	catalogPath := filepath.Join(root, catalogFileName)
	invalid := []byte(`{"version":1,"activeSpaceId":"broken","spaces":[]}`)
	if err := os.WriteFile(catalogPath, invalid, 0o600); err != nil {
		t.Fatalf("write corrupt catalog: %v", err)
	}

	if _, err := Open(root); !errors.Is(err, ErrCatalogInvalid) {
		t.Fatalf("open corrupt catalog error = %v", err)
	}
	encoded, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read corrupt catalog: %v", err)
	}
	if string(encoded) != string(invalid) {
		t.Fatalf("corrupt catalog was overwritten: %q", encoded)
	}
}

func TestCreateRejectsUnsafeSpacesPathBeforePreflight(t *testing.T) {
	root := t.TempDir()
	registry, err := Open(root)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, spacesDirectory), []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("write unsafe spaces path: %v", err)
	}
	preflightCalled := false
	_, _, err = registry.Create(t.Context(), "仕事", func(context.Context, string) error {
		preflightCalled = true
		return nil
	})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("unsafe path error = %v", err)
	}
	if preflightCalled {
		t.Fatal("preflight was called for an unsafe path")
	}
}

func TestCatalogRejectsTraversalLikeSpaceID(t *testing.T) {
	root := t.TempDir()
	if _, err := Open(root); err != nil {
		t.Fatalf("bootstrap registry: %v", err)
	}
	invalid := []byte(`{
  "version": 1,
  "activeSpaceId": "../../outside",
  "spaces": [{
    "id": "../../outside",
    "name": "メイン",
    "legacy": true,
    "createdAt": "2026-08-25T00:00:00Z"
  }]
}`)
	if err := os.WriteFile(filepath.Join(root, catalogFileName), invalid, 0o600); err != nil {
		t.Fatalf("write traversal catalog: %v", err)
	}
	if _, err := Open(root); !errors.Is(err, ErrCatalogInvalid) {
		t.Fatalf("traversal catalog error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("traversal target was touched: %v", err)
	}
}

func createPreparedDirectory(_ context.Context, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "prepared"), []byte(filepath.Base(dataDir)), 0o600)
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("content at %s = %q, want %q", path, content, expected)
	}
}
