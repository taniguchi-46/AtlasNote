package sync

import (
	"errors"
	"testing"
)

type unavailableCredentialStore struct {
	deleteCalls int
}

func (s *unavailableCredentialStore) Get(string) (string, error) {
	return "", ErrCredentialStoreUnavailable
}

func (s *unavailableCredentialStore) Set(string, string) error {
	return ErrCredentialStoreUnavailable
}

func (s *unavailableCredentialStore) Delete(string) error {
	s.deleteCalls++
	return ErrCredentialStoreUnavailable
}

func TestCredentialManagerDeletesSessionFallbackWithoutUnavailableKeyring(t *testing.T) {
	secure := &unavailableCredentialStore{}
	manager := NewCredentialManager(secure)
	persisted, err := manager.Save("session-ref", "secret", true)
	if err != nil || persisted {
		t.Fatalf("save session fallback: persisted=%v err=%v", persisted, err)
	}
	if err := manager.Delete("session-ref"); err != nil {
		t.Fatalf("delete session fallback: %v", err)
	}
	if secure.deleteCalls != 0 {
		t.Fatalf("session fallback attempted unavailable keyring delete %d times", secure.deleteCalls)
	}
	if _, err := manager.Get("session-ref"); !errors.Is(err, ErrCredentialsUnavailable) {
		t.Fatalf("deleted fallback credential error = %v", err)
	}
}

func TestCredentialManagerDeletesPersistedCredentialFromKeyring(t *testing.T) {
	secure := newCountingCredentialStore()
	manager := NewCredentialManager(secure)
	persisted, err := manager.Save("secure-ref", "secret", true)
	if err != nil || !persisted {
		t.Fatalf("save persisted credential: persisted=%v err=%v", persisted, err)
	}
	if err := manager.Delete("secure-ref"); err != nil {
		t.Fatalf("delete persisted credential: %v", err)
	}
	if secure.deleteCalls != 1 {
		t.Fatalf("persisted credential delete calls = %d", secure.deleteCalls)
	}
}
