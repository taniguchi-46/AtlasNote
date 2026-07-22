package ai

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/credential"
	"atlasnote/internal/database"
)

type memoryCredentialStore struct {
	values      map[string]string
	setErr      error
	getErr      error
	deleteErr   error
	setCalls    int
	deleteCalls int
}

func newMemoryCredentialStore() *memoryCredentialStore {
	return &memoryCredentialStore{values: make(map[string]string)}
}

func (s *memoryCredentialStore) Get(ref string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	value, ok := s.values[ref]
	if !ok {
		return "", credential.ErrNotFound
	}
	return value, nil
}

func (s *memoryCredentialStore) Set(ref string, secret string) error {
	s.setCalls++
	if s.setErr != nil {
		return s.setErr
	}
	s.values[ref] = secret
	return nil
}

func (s *memoryCredentialStore) Delete(ref string) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.values, ref)
	return nil
}

type testChecker struct {
	err      error
	calls    int
	provider ProviderID
}

func (c *testChecker) Check(_ context.Context, providerID ProviderID, _ string) error {
	c.calls++
	c.provider = providerID
	return c.err
}

func newTestService(t *testing.T, store credential.Store, checker ConnectionChecker) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(NewRepository(db), credential.NewManager(store), checker), db
}

func findSettings(t *testing.T, settings []ProviderSettings, providerID ProviderID) ProviderSettings {
	t.Helper()
	for _, setting := range settings {
		if setting.ProviderID == providerID {
			return setting
		}
	}
	t.Fatalf("missing provider settings for %s", providerID)
	return ProviderSettings{}
}

func TestConfigureSeparatesProviderCredentialsAndDoesNotPersistSecrets(t *testing.T) {
	store := newMemoryCredentialStore()
	service, db := newTestService(t, store, &testChecker{})
	openRouterSecret := "openrouter-test-secret-marker"
	geminiSecret := "gemini-test-secret-marker"

	settings, err := service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderOpenRouter,
		APIKey:     openRouterSecret,
		ModelID:    "openrouter/model",
	})
	if err != nil {
		t.Fatalf("configure OpenRouter: %v", err)
	}
	if got := findSettings(t, settings, ProviderOpenRouter); got.CredentialStatus != CredentialStatusPersistent || got.ModelID != "openrouter/model" {
		t.Fatalf("OpenRouter settings = %#v", got)
	}

	settings, err = service.Configure(t.Context(), ConfigureProviderInput{
		ProviderID: ProviderGemini,
		APIKey:     geminiSecret,
		ModelID:    "gemini-2.5-flash",
	})
	if err != nil {
		t.Fatalf("configure Gemini: %v", err)
	}
	if got := findSettings(t, settings, ProviderGemini); got.CredentialStatus != CredentialStatusPersistent || got.ModelID != "gemini-2.5-flash" {
		t.Fatalf("Gemini settings = %#v", got)
	}

	if strings.Contains(fmt.Sprintf("%#v", settings), openRouterSecret) || strings.Contains(fmt.Sprintf("%#v", settings), geminiSecret) {
		t.Fatal("safe provider settings exposed an API key")
	}
	openRouterKey, err := service.GetCredential(t.Context(), ProviderOpenRouter)
	if err != nil || openRouterKey != openRouterSecret {
		t.Fatal("OpenRouter credential was not isolated in its own store reference")
	}
	geminiKey, err := service.GetCredential(t.Context(), ProviderGemini)
	if err != nil || geminiKey != geminiSecret {
		t.Fatal("Gemini credential was not isolated in its own store reference")
	}

	rows, err := db.QueryContext(t.Context(), "SELECT credential_ref, model_id FROM ai_provider_settings")
	if err != nil {
		t.Fatalf("query AI provider settings: %v", err)
	}
	defer rows.Close()
	refs := make(map[string]struct{})
	for rows.Next() {
		var ref string
		var modelID string
		if err := rows.Scan(&ref, &modelID); err != nil {
			t.Fatalf("scan AI provider settings: %v", err)
		}
		if strings.Contains(ref, "secret-marker") || strings.Contains(modelID, "secret-marker") {
			t.Fatal("SQLite stored an API key")
		}
		refs[ref] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate AI provider settings: %v", err)
	}
	if len(refs) != 2 || len(store.values) != 2 {
		t.Fatal("provider credentials did not receive distinct references")
	}

	var outboxCount int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sync_outbox").Scan(&outboxCount); err != nil {
		t.Fatalf("count sync outbox: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("AI configuration changed the sync outbox: %d", outboxCount)
	}
}

func TestConnectionCheckFailureHasNoSideEffectsOrSecretLeak(t *testing.T) {
	store := newMemoryCredentialStore()
	secret := "connection-test-secret-marker"
	checker := &testChecker{err: errors.New("raw provider error " + secret)}
	service, db := newTestService(t, store, checker)

	_, err := service.TestConnection(t.Context(), TestConnectionInput{ProviderID: ProviderOpenRouter, APIKey: secret})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("connection error = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("connection error exposed an API key")
	}
	if checker.calls != 1 || checker.provider != ProviderOpenRouter {
		t.Fatal("connection checker did not receive the selected provider")
	}
	if store.setCalls != 0 {
		t.Fatalf("connection test persisted a credential %d times", store.setCalls)
	}
	var saved int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_provider_settings").Scan(&saved); err != nil {
		t.Fatalf("count AI settings after connection test: %v", err)
	}
	if saved != 0 {
		t.Fatalf("connection test persisted %d settings", saved)
	}
}

func TestSessionOnlyFallbackRequiresReauthenticationAfterRestart(t *testing.T) {
	unavailable := &memoryCredentialStore{values: make(map[string]string), setErr: credential.ErrStoreUnavailable, getErr: credential.ErrStoreUnavailable}
	service, db := newTestService(t, unavailable, &testChecker{})

	settings, err := service.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderGemini, APIKey: "session-only-secret"})
	if err != nil {
		t.Fatalf("configure session-only credential: %v", err)
	}
	if got := findSettings(t, settings, ProviderGemini); got.CredentialStatus != CredentialStatusSessionOnly {
		t.Fatalf("session-only status = %#v", got)
	}

	restarted := NewService(NewRepository(db), credential.NewManager(unavailable), &testChecker{})
	settings, err = restarted.GetSettings(t.Context())
	if err != nil {
		t.Fatalf("read settings after restart: %v", err)
	}
	if got := findSettings(t, settings, ProviderGemini); got.CredentialStatus != CredentialStatusReauthenticationRequired {
		t.Fatalf("restart credential status = %#v", got)
	}
	if _, err := restarted.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderGemini, APIKey: "reauthenticated-session-secret"}); err != nil {
		t.Fatalf("reauthenticate session-only credential: %v", err)
	}
	settings, err = restarted.GetSettings(t.Context())
	if err != nil {
		t.Fatalf("read settings after reauthentication: %v", err)
	}
	if got := findSettings(t, settings, ProviderGemini); got.CredentialStatus != CredentialStatusSessionOnly {
		t.Fatalf("reauthenticated session-only status = %#v", got)
	}
}

func TestConfigureUpdateAndDeletionReplaceOnlySelectedProviderCredential(t *testing.T) {
	store := newMemoryCredentialStore()
	service, _ := newTestService(t, store, &testChecker{})

	if _, err := service.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderOpenRouter, APIKey: "old-secret", ModelID: "first-model"}); err != nil {
		t.Fatalf("configure old OpenRouter credential: %v", err)
	}
	oldRecord, err := service.repository.get(t.Context(), ProviderOpenRouter)
	if err != nil || oldRecord == nil {
		t.Fatal("read old OpenRouter credential reference")
	}
	if _, err := service.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderOpenRouter, APIKey: "new-secret"}); err != nil {
		t.Fatalf("update OpenRouter credential: %v", err)
	}
	newRecord, err := service.repository.get(t.Context(), ProviderOpenRouter)
	if err != nil || newRecord == nil || newRecord.CredentialRef == oldRecord.CredentialRef || newRecord.ModelID != "first-model" {
		t.Fatal("update did not create a new reference while preserving the selected model")
	}
	if _, exists := store.values[oldRecord.CredentialRef]; exists {
		t.Fatal("old credential remained after a successful update")
	}
	if _, exists := store.values[newRecord.CredentialRef]; !exists {
		t.Fatal("new credential was not retained")
	}

	if _, err := service.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderGemini, APIKey: "gemini-secret"}); err != nil {
		t.Fatalf("configure Gemini credential: %v", err)
	}
	if _, err := service.DeleteProvider(t.Context(), ProviderOpenRouter); err != nil {
		t.Fatalf("delete OpenRouter credential: %v", err)
	}
	if _, err := service.GetCredential(t.Context(), ProviderOpenRouter); !errors.Is(err, ErrReauthenticationRequired) {
		t.Fatalf("deleted provider credential error = %v", err)
	}
	if _, err := service.GetCredential(t.Context(), ProviderGemini); err != nil {
		t.Fatalf("deleting OpenRouter affected Gemini: %v", err)
	}
	if _, err := service.DeleteAll(t.Context()); err != nil {
		t.Fatalf("delete all AI credentials: %v", err)
	}
	if len(store.values) != 0 {
		t.Fatal("delete all left a credential in the store")
	}
}

func TestConfigureRejectsInvalidKeysWithoutWriting(t *testing.T) {
	store := newMemoryCredentialStore()
	service, db := newTestService(t, store, &testChecker{})
	for _, apiKey := range []string{"", "line\nbreak", "control\x00character"} {
		if _, err := service.Configure(t.Context(), ConfigureProviderInput{ProviderID: ProviderOpenRouter, APIKey: apiKey}); !errors.Is(err, ErrAPIKeyInvalid) {
			t.Fatalf("invalid key returned %v", err)
		}
	}
	if store.setCalls != 0 {
		t.Fatalf("invalid keys wrote %d credentials", store.setCalls)
	}
	var saved int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM ai_provider_settings").Scan(&saved); err != nil {
		t.Fatalf("count AI settings: %v", err)
	}
	if saved != 0 {
		t.Fatalf("invalid keys saved %d provider settings", saved)
	}
}
