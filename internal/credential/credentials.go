package credential

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

var (
	ErrNotFound         = errors.New("credential not found")
	ErrStoreUnavailable = errors.New("secure credential store unavailable")
)

// Store keeps a secret outside of Atlas Note's SQLite, Markdown, and settings
// storage. Implementations must never include the secret in returned errors.
type Store interface {
	Get(ref string) (string, error)
	Set(ref string, secret string) error
	Delete(ref string) error
}

// KeyringStore stores credentials in the OS credential facility selected by
// go-keyring. The service name separates independent credential domains.
type KeyringStore struct {
	service string
}

func NewKeyringStore(service string) *KeyringStore {
	return &KeyringStore{service: service}
}

func (s *KeyringStore) Get(ref string) (string, error) {
	value, err := keyring.Get(s.service, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", ErrStoreUnavailable
	}
	return value, nil
}

func (s *KeyringStore) Set(ref string, secret string) error {
	if err := keyring.Set(s.service, ref, secret); err != nil {
		return ErrStoreUnavailable
	}
	return nil
}

func (s *KeyringStore) Delete(ref string) error {
	if err := keyring.Delete(s.service, ref); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return ErrStoreUnavailable
	}
	return nil
}

// SessionStore is deliberately process-local. It is used only when the OS
// credential facility cannot be used or when a caller explicitly requests a
// non-persistent credential.
type SessionStore struct {
	mu     sync.RWMutex
	values map[string]string
}

func NewSessionStore() *SessionStore {
	return &SessionStore{values: make(map[string]string)}
}

func (s *SessionStore) Get(ref string) (string, error) {
	s.mu.RLock()
	value, ok := s.values[ref]
	s.mu.RUnlock()
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (s *SessionStore) Set(ref string, secret string) error {
	s.mu.Lock()
	s.values[ref] = secret
	s.mu.Unlock()
	return nil
}

func (s *SessionStore) Delete(ref string) error {
	s.mu.Lock()
	delete(s.values, ref)
	s.mu.Unlock()
	return nil
}

func (s *SessionStore) has(ref string) bool {
	s.mu.RLock()
	_, ok := s.values[ref]
	s.mu.RUnlock()
	return ok
}

// Manager prefers the OS credential store and falls back to a process-local
// session store only when persistence is unavailable. It never serializes a
// secret itself.
type Manager struct {
	secure  Store
	session *SessionStore
}

func NewManager(secure Store) *Manager {
	return &Manager{secure: secure, session: NewSessionStore()}
}

// Save returns whether the credential was persisted in the OS store. A false
// result with nil error means the credential is available for this process only.
func (m *Manager) Save(ref string, secret string, persist bool) (bool, error) {
	if !persist {
		return false, m.session.Set(ref, secret)
	}
	if m.secure != nil {
		if err := m.secure.Set(ref, secret); err == nil {
			return true, nil
		}
	}
	if err := m.session.Set(ref, secret); err != nil {
		return false, err
	}
	return false, nil
}

func (m *Manager) Get(ref string) (string, error) {
	if value, err := m.session.Get(ref); err == nil {
		return value, nil
	}
	if m.secure == nil {
		return "", ErrStoreUnavailable
	}
	value, err := m.secure.Get(ref)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFound
		}
		return "", ErrStoreUnavailable
	}
	return value, nil
}

// Has checks availability without returning the credential to a caller.
func (m *Manager) Has(ref string) (bool, error) {
	_, err := m.Get(ref)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (m *Manager) Delete(ref string) error {
	if m.session.has(ref) {
		return m.session.Delete(ref)
	}
	if m.secure == nil {
		return ErrStoreUnavailable
	}
	if err := m.secure.Delete(ref); err != nil && !errors.Is(err, ErrNotFound) {
		return ErrStoreUnavailable
	}
	return nil
}
