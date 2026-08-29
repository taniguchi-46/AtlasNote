package sync

import (
	"errors"

	"atlasnote/internal/credential"
)

var (
	ErrCredentialNotFound         = credential.ErrNotFound
	ErrCredentialStoreUnavailable = credential.ErrStoreUnavailable
)

// These aliases preserve the sync package API while sharing the OS credential
// boundary with other local-only features such as AI settings.
type CredentialStore = credential.Store
type KeyringCredentialStore = credential.KeyringStore
type SessionCredentialStore = credential.SessionStore

func NewKeyringCredentialStore(service string) *KeyringCredentialStore {
	return credential.NewKeyringStore(service)
}

func NewSessionCredentialStore() *SessionCredentialStore {
	return credential.NewSessionStore()
}

// CredentialManager keeps sync's existing error contract while delegating the
// secure-store and session-only behavior to the shared implementation.
type CredentialManager struct {
	manager *credential.Manager
}

func NewCredentialManager(secure CredentialStore) *CredentialManager {
	return &CredentialManager{manager: credential.NewManager(secure)}
}

func (m *CredentialManager) Save(ref string, secret string, remember bool) (bool, error) {
	return m.manager.Save(ref, secret, remember)
}

func (m *CredentialManager) Get(ref string) (string, error) {
	value, err := m.manager.Get(ref)
	if err == nil {
		return value, nil
	}
	if errors.Is(err, credential.ErrNotFound) {
		return "", ErrCredentialNotFound
	}
	return "", ErrCredentialsUnavailable
}

func (m *CredentialManager) Delete(ref string) error {
	return m.manager.Delete(ref)
}
