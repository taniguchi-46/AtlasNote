package appcleanup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"atlasnote/internal/database"
)

type memoryCredentialDeleter struct {
	deleted []string
	err     error
}

func (store *memoryCredentialDeleter) Delete(ref string) error {
	if store.err != nil {
		return store.err
	}
	store.deleted = append(store.deleted, ref)
	return nil
}

func testIdentity(root string) *UserIdentity {
	return &UserIdentity{
		SID:        "S-1-5-21-test",
		Account:    "test-user",
		ProfileDir: root,
		AppDataDir: filepath.Join(root, "AppData", "Roaming"),
	}
}

func TestRunDeletesOnlyKnownWebViewDataAndKeepsLocationAndStorageData(t *testing.T) {
	root := t.TempDir()
	identity := testIdentity(root)
	if err := os.MkdirAll(identity.AppDataDir, 0o700); err != nil {
		t.Fatalf("create app data: %v", err)
	}
	webviewPath := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
	if err := os.MkdirAll(filepath.Join(webviewPath, filepath.FromSlash(cacheDirectories[0])), 0o700); err != nil {
		t.Fatalf("create webview data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webviewPath, filepath.FromSlash(cacheDirectories[0]), "CURRENT"), []byte("settings"), 0o600); err != nil {
		t.Fatalf("write webview marker: %v", err)
	}
	dataRoot := filepath.Join(root, "notes")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		t.Fatalf("create notes root: %v", err)
	}
	locationFile := filepath.Join(identity.AppDataDir, "AtlasNote", "storage-locations.json")
	if err := os.MkdirAll(filepath.Dir(locationFile), 0o700); err != nil {
		t.Fatalf("create location directory: %v", err)
	}
	if err := os.WriteFile(locationFile, []byte("location"), 0o600); err != nil {
		t.Fatalf("write location file: %v", err)
	}
	noteFile := filepath.Join(dataRoot, "notes", "keep.md")
	if err := os.MkdirAll(filepath.Dir(noteFile), 0o700); err != nil {
		t.Fatalf("create notes directory: %v", err)
	}
	if err := os.WriteFile(noteFile, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	service := newFixtureService()
	result := service.Run(context.Background(), Request{
		DeleteDisplaySettings: true,
		Identity:              identity,
		DataRoots:             []string{},
	})
	if result.Error != nil || !result.DisplaySettingsDeleted {
		t.Fatalf("cleanup result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(webviewPath, filepath.FromSlash(cacheDirectories[0]))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("webview data remains or unexpected error: %v", err)
	}
	if _, err := os.Stat(locationFile); err != nil {
		t.Fatalf("location file was removed: %v", err)
	}
	if _, err := os.Stat(noteFile); err != nil {
		t.Fatalf("note data was removed: %v", err)
	}
}

func TestRunDeletesCredentialReferencesFromAllKnownStorageDatabases(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "atlasnote.db")
	db, err := database.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "INSERT INTO ai_provider_settings(provider_id, model_id, credential_ref, credential_storage, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", "openrouter", "model", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "persistent", "now", "now"); err != nil {
		db.Close()
		t.Fatalf("insert AI credential ref: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "INSERT INTO sync_connections(id, endpoint, remote_root, username, vault_id, credential_ref, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 1, "https://example.com", "/", "user", "vault", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "now", "now"); err != nil {
		db.Close()
		t.Fatalf("insert sync credential ref: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}

	identity := testIdentity(root)
	aiStore := &memoryCredentialDeleter{}
	syncStore := &memoryCredentialDeleter{}
	service := newFixtureService()
	service.credentialFactory = func(serviceName string) (CredentialDeleter, error) {
		switch serviceName {
		case "atlasnote-ai":
			return aiStore, nil
		case "atlasnote-webdav":
			return syncStore, nil
		default:
			t.Fatalf("unexpected credential service: %s", serviceName)
			return nil, errors.New("unexpected service")
		}
	}
	result := service.Run(context.Background(), Request{
		DeleteCredentials: true,
		Identity:          identity,
		DataRoots:         []string{root},
	})
	if result.Error != nil || !result.CredentialsDeleted || result.CredentialCount != 2 {
		t.Fatalf("credential cleanup result = %#v", result)
	}
	if len(aiStore.deleted) != 1 || aiStore.deleted[0] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("AI credential deletions = %#v", aiStore.deleted)
	}
	if len(syncStore.deleted) != 1 || syncStore.deleted[0] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("sync credential deletions = %#v", syncStore.deleted)
	}
	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("database was removed: %v", err)
	}
}

func TestRunRefusesCleanupForDifferentUser(t *testing.T) {
	root := t.TempDir()
	identity := testIdentity(root)
	webviewPath := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
	if err := os.MkdirAll(webviewPath, 0o700); err != nil {
		t.Fatalf("create webview data: %v", err)
	}
	service := newFixtureService()
	result := service.Run(context.Background(), Request{
		DeleteDisplaySettings: true,
		ExpectedSID:           "different-user",
		Identity:              identity,
		DataRoots:             []string{},
	})
	if result.Error == nil || result.Error.Code != ErrorCodeIdentity {
		t.Fatalf("identity mismatch result = %#v", result)
	}
	if _, err := os.Stat(webviewPath); err != nil {
		t.Fatalf("identity mismatch removed target: %v", err)
	}
}

func TestRunRefusesSymlinkedWebViewTarget(t *testing.T) {
	root := t.TempDir()
	identity := testIdentity(root)
	if err := os.MkdirAll(identity.AppDataDir, 0o700); err != nil {
		t.Fatalf("create app data: %v", err)
	}
	target := filepath.Join(root, "outside")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("create outside target: %v", err)
	}
	webviewPath := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
	if err := os.Symlink(target, webviewPath); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	result := newFixtureService().Run(context.Background(), Request{
		DeleteDisplaySettings: true,
		Identity:              identity,
		DataRoots:             []string{},
	})
	if result.Error == nil || result.Error.Code != ErrorCodeSettings {
		t.Fatalf("symlink cleanup result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(target)); err != nil {
		t.Fatalf("symlink target was changed: %v", err)
	}
}

func TestRunRefusesCleanupWhileMigrationOrRecoveryIsPending(t *testing.T) {
	root := t.TempDir()
	identity := testIdentity(root)
	if err := os.MkdirAll(filepath.Join(identity.AppDataDir, "AtlasNote"), 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	webviewPath := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
	if err := os.MkdirAll(webviewPath, 0o700); err != nil {
		t.Fatalf("create webview data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(identity.AppDataDir, "AtlasNote", "storage-location-migration.json"), []byte("pending"), 0o600); err != nil {
		t.Fatalf("write migration marker: %v", err)
	}

	result := newFixtureService().Run(context.Background(), Request{
		DeleteDisplaySettings: true,
		Identity:              identity,
		DataRoots:             []string{},
	})
	if result.Error == nil || result.Error.Code != ErrorCodeRecovery {
		t.Fatalf("pending migration result = %#v", result)
	}
	if _, err := os.Stat(webviewPath); err != nil {
		t.Fatalf("pending migration removed display settings: %v", err)
	}
}

func TestCleanupOverlapAndUnknownDataAreUnchanged(t *testing.T) {
	for _, scenario := range []string{"data-root", "data-child", "backup-root", "backup-child", "ancestor", "unknown-cache-file", "old-layout"} {
		t.Run(scenario, func(t *testing.T) {
			identity := testIdentity(t.TempDir())
			root := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
			cache := filepath.Join(root, filepath.FromSlash(cacheDirectories[0]))
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(cache, "CURRENT")
			if err := os.WriteFile(marker, []byte("settings"), 0600); err != nil {
				t.Fatal(err)
			}
			protected := root
			if scenario == "data-child" || scenario == "backup-child" {
				protected = filepath.Join(root, "user-data")
			}
			if scenario == "ancestor" {
				protected = identity.AppDataDir
			}
			if scenario == "unknown-cache-file" {
				protected = cache
			}
			if scenario == "old-layout" {
				protected = filepath.Join(root, "old-notes")
			}
			if err := os.MkdirAll(protected, 0700); err != nil {
				t.Fatal(err)
			}
			files := []string{marker}
			for _, name := range []string{"note.md", "atlasnote.db", "backup.zip", "storage-locations.json"} {
				file := filepath.Join(protected, name)
				if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
				files = append(files, file)
			}
			req := Request{Identity: identity, DataRoots: []string{}, DeleteDisplaySettings: true, DeleteCredentials: true}
			if scenario == "backup-root" || scenario == "backup-child" {
				req.ArchiveRoots = []string{protected}
			} else if scenario != "unknown-cache-file" && scenario != "old-layout" {
				req.DataRoots = []string{protected}
			}
			service := newFixtureService()
			called := false
			service.credentialFactory = func(string) (CredentialDeleter, error) { called = true; return &memoryCredentialDeleter{}, nil }
			result := service.Run(context.Background(), req)
			if scenario != "old-layout" && result.Error == nil {
				t.Fatal("unsafe cleanup accepted")
			}
			if called {
				t.Fatal("credential store touched on refusal")
			}
			for _, file := range files {
				// A historical unknown sibling is retained; only known cache files may go.
				if scenario == "old-layout" && file == marker {
					continue
				}
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				expected := "preserve"
				if file == marker {
					expected = "settings"
				}
				if string(data) != expected {
					t.Fatal("fixture content changed")
				}
			}
		})
	}
}

func newFixtureService() *Service {
	service := NewService()
	service.acquireAppLock = func() (*ApplicationLock, error) {
		return &ApplicationLock{release: func() error { return nil }}, nil
	}
	return service
}

func TestSavedLocationsInsideCacheRefuseBeforeAnyMutation(t *testing.T) {
	for _, kind := range []string{"dataRoot", "backupRoot"} {
		t.Run(kind, func(t *testing.T) {
			for _, name := range []string{"ATLAS_NOTE_DATA_DIR", "ATLAS_NOTE_STORAGE_LOCATIONS_FILE", "ATLAS_NOTE_DEFAULT_DATA_ROOT"} {
				t.Setenv(name, "")
			}
			identity := testIdentity(t.TempDir())
			root := filepath.Join(identity.AppDataDir, "AtlasNote.exe")
			cache := filepath.Join(root, filepath.FromSlash(cacheDirectories[0]))
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(cache, "CURRENT")
			if err := os.WriteFile(marker, []byte("settings"), 0600); err != nil {
				t.Fatal(err)
			}
			locationFile := filepath.Join(identity.AppDataDir, "AtlasNote", "storage-locations.json")
			if err := os.MkdirAll(filepath.Dir(locationFile), 0700); err != nil {
				t.Fatal(err)
			}
			locations := map[string]any{"version": 1, "dataRoot": filepath.Join(identity.ProfileDir, "storage"), "backupRoot": filepath.Join(identity.ProfileDir, "backup")}
			locations[kind] = root
			content, err := json.Marshal(locations)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(locationFile, content, 0600); err != nil {
				t.Fatal(err)
			}
			result := newFixtureService().Run(context.Background(), Request{Identity: identity, DeleteDisplaySettings: true, DeleteCredentials: true})
			if result.Error == nil || result.DisplaySettingsDeleted || result.CredentialsDeleted {
				t.Fatalf("saved overlap accepted: %#v", result)
			}
			for path, want := range map[string]string{marker: "settings", locationFile: string(content)} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatal("saved overlap mutated files")
				}
			}
		})
	}
}
