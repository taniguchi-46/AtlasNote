package appcleanup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"atlasnote/internal/database"
)

func TestRegisteredUserMaintenanceFlow(t *testing.T) {
	for _, scenario := range []string{"same-user", "same-name-different-sid", "other-admin-uac", "missing-record", "different-install", "unverified-session", "token-changed-after-parse"} {
		t.Run(scenario, func(t *testing.T) {
			identity := testIdentity(t.TempDir())
			executable := filepath.Join(identity.ProfileDir, "install", "AtlasNote.exe")
			originalSID := identity.SID
			sessionSID := originalSID
			elevated := false
			records := map[string]string{}
			registration := userRegistration{
				current: func() (UserIdentity, error) { return *identity, nil },
				sessionSID: func() (string, error) {
					if sessionSID == "" {
						return "", ErrIdentityUnavailable
					}
					return sessionSID, nil
				},
				elevated: func() bool { return elevated },
				read: func(key string) (string, error) {
					sid, ok := records[key]
					if !ok {
						return "", os.ErrNotExist
					}
					return sid, nil
				},
				write: func(key, sid string) error { records[key] = sid; return nil },
			}
			if err := registration.record(executable); err != nil {
				t.Fatal(err)
			}
			elevated = true // Same-user UAC is allowed; registration itself is not.
			if err := registration.record(executable); err == nil {
				t.Fatal("elevated installer can register users")
			}
			switch scenario {
			case "same-name-different-sid":
				identity.SID = "S-1-5-21-other"
				sessionSID = identity.SID
			case "other-admin-uac":
				identity.SID = "S-1-5-21-admin"
				identity.Account = "administrator"
			case "missing-record":
				clear(records)
			case "different-install":
				executable = filepath.Join(identity.ProfileDir, "other", "AtlasNote.exe")
			case "unverified-session":
				sessionSID = ""
			}
			cache := filepath.Join(identity.AppDataDir, "AtlasNote.exe", filepath.FromSlash(cacheDirectories[0]))
			if err := os.MkdirAll(cache, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(cache, "CURRENT")
			if err := os.WriteFile(marker, []byte("settings"), 0600); err != nil {
				t.Fatal(err)
			}
			dataRoot := filepath.Join(identity.ProfileDir, "storage")
			if err := os.MkdirAll(dataRoot, 0700); err != nil {
				t.Fatal(err)
			}
			dbPath := filepath.Join(dataRoot, "atlasnote.db")
			db, err := database.Open(context.Background(), dbPath)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("INSERT INTO ai_provider_settings(provider_id, model_id, credential_ref, credential_storage, created_at, updated_at) VALUES ('openrouter','model','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','persistent','now','now')")
			if err != nil {
				db.Close()
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, name := range []string{"note.md", "backup.zip", "storage-locations.json"} {
				if err := os.WriteFile(filepath.Join(dataRoot, name), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"atlasnote.db", "note.md", "backup.zip", "storage-locations.json"} {
				path := filepath.Join(dataRoot, name)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				before[path] = data
			}
			store := &memoryCredentialDeleter{}
			request, err := ParseMaintenanceRequest([]string{"--registered-user", "--delete-display-settings", "--delete-credentials"}, func() (string, error) { return registration.target(executable) })
			if scenario == "same-user" || scenario == "token-changed-after-parse" {
				if err != nil || request.ExpectedSID != originalSID {
					t.Fatalf("SID not passed: %v", err)
				}
			} else if err == nil {
				t.Fatal("unverified identity accepted")
			}
			if scenario == "token-changed-after-parse" {
				identity.SID = "S-1-5-21-replaced"
			}
			if err == nil {
				request.Identity = identity
				request.DataRoots = []string{dataRoot}
				service := newFixtureService()
				service.credentialFactory = func(string) (CredentialDeleter, error) {
					return store, nil
				}
				result := service.Run(context.Background(), request)
				if scenario == "same-user" && (result.Error != nil || !result.CredentialsDeleted || !result.DisplaySettingsDeleted) {
					t.Fatalf("normal flow: %#v", result)
				}
				if scenario != "same-user" && result.Error == nil {
					t.Fatal("SID mismatch not enforced by service")
				}
			}
			if scenario != "same-user" {
				if len(store.deleted) != 0 {
					t.Fatal("refused cleanup deleted credentials")
				}
				if data, err := os.ReadFile(marker); err != nil || string(data) != "settings" {
					t.Fatal("refused cleanup changed data")
				}
			} else if len(store.deleted) != 1 {
				t.Fatal("matching SID did not delete fixture credential")
			}
			for path, expected := range before {
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, expected) {
					t.Fatal("cleanup changed retained data")
				}
			}
		})
	}
}
func TestMaintenanceRequiresRegisteredIdentityAndExactFlags(t *testing.T) {
	for _, args := range [][]string{{"--delete-credentials"}, {"--expected-user=name", "--delete-credentials"}, {"--registered-user"}, {"--registered-user", "--delete-credentials", "--unknown"}} {
		if _, err := ParseMaintenanceRequest(args, func() (string, error) { return "S-1-test", nil }); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
