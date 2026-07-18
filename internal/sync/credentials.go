package sync

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

var (
	ErrCredentialNotFound         = errors.New("sync credential not found")
	ErrCredentialStoreUnavailable = errors.New("sync secure credential store unavailable")
)

type CredentialStore interface {
	Get(ref string) (string, error)
	Set(ref string, secret string) error
	Delete(ref string) error
}

type KeyringCredentialStore struct {
	service string
}

func NewKeyringCredentialStore(service string) *KeyringCredentialStore {
	return &KeyringCredentialStore{service: service}
}

func (s *KeyringCredentialStore) Get(ref string) (string, error) {
	value, err := keyring.Get(s.service, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrCredentialNotFound
	}
	if err != nil {
		return "", ErrCredentialStoreUnavailable
	}
	return value, nil
}

func (s *KeyringCredentialStore) Set(ref string, secret string) error {
	if err := keyring.Set(s.service, ref, secret); err != nil {
		return ErrCredentialStoreUnavailable
	}
	return nil
}

func (s *KeyringCredentialStore) Delete(ref string) error {
	if err := keyring.Delete(s.service, ref); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return ErrCredentialStoreUnavailable
	}
	return nil
}

type SessionCredentialStore struct {
	mu     sync.RWMutex
	values map[string]string
}

func NewSessionCredentialStore() *SessionCredentialStore {
	return &SessionCredentialStore{values: make(map[string]string)}
}

func (s *SessionCredentialStore) Get(ref string) (string, error) {
	s.mu.RLock()
	value, ok := s.values[ref]
	s.mu.RUnlock()
	if !ok {
		return "", ErrCredentialNotFound
	}
	return value, nil
}

func (s *SessionCredentialStore) Set(ref string, secret string) error {
	s.mu.Lock()
	s.values[ref] = secret
	s.mu.Unlock()
	return nil
}

func (s *SessionCredentialStore) Delete(ref string) error {
	s.mu.Lock()
	delete(s.values, ref)
	s.mu.Unlock()
	return nil
}

// CredentialManager attempts the OS store for persistent credentials and keeps
// a session-only copy when the OS integration is unavailable. The fallback is
// deliberately process-local and never serialized to SQLite or localStorage.
type CredentialManager struct {
	secure  CredentialStore
	session *SessionCredentialStore
}

func NewCredentialManager(secure CredentialStore) *CredentialManager {
	return &CredentialManager{secure: secure, session: NewSessionCredentialStore()}
}

func (m *CredentialManager) Save(ref string, secret string, remember bool) (bool, error) {
	if !remember {
		return false, m.session.Set(ref, secret)
	}
	if err := m.secure.Set(ref, secret); err == nil {
		return true, nil
	}
	if err := m.session.Set(ref, secret); err != nil {
		return false, err
	}
	return false, nil
}

func (m *CredentialManager) Get(ref string) (string, error) {
	if value, err := m.session.Get(ref); err == nil {
		return value, nil
	}
	value, err := m.secure.Get(ref)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			return "", ErrCredentialNotFound
		}
		return "", ErrCredentialsUnavailable
	}
	return value, nil
}

func (m *CredentialManager) Delete(ref string) error {
	_, sessionErr := m.session.Get(ref)
	_ = m.session.Delete(ref)
	if sessionErr == nil {
		return nil
	}
	if err := m.secure.Delete(ref); err != nil && !errors.Is(err, ErrCredentialNotFound) {
		return err
	}
	return nil
}
