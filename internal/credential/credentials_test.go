package credential

import (
	"errors"
	"testing"
)

type unavailableStore struct {
	deleteCalls int
}

func (s *unavailableStore) Get(string) (string, error) {
	return "", ErrStoreUnavailable
}

func (s *unavailableStore) Set(string, string) error {
	return ErrStoreUnavailable
}

func (s *unavailableStore) Delete(string) error {
	s.deleteCalls++
	return ErrStoreUnavailable
}

func TestManagerFallsBackToSessionOnlyAndDeletesWithoutKeyring(t *testing.T) {
	secure := &unavailableStore{}
	manager := NewManager(secure)

	persisted, err := manager.Save("session-ref", "test-secret", true)
	if err != nil || persisted {
		t.Fatalf("save session-only credential: persisted=%v err=%v", persisted, err)
	}
	available, err := manager.Has("session-ref")
	if err != nil || !available {
		t.Fatalf("session credential availability: available=%v err=%v", available, err)
	}
	if err := manager.Delete("session-ref"); err != nil {
		t.Fatalf("delete session-only credential: %v", err)
	}
	if secure.deleteCalls != 0 {
		t.Fatalf("session-only delete called unavailable secure store %d times", secure.deleteCalls)
	}
	if _, err := manager.Get("session-ref"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("deleted session credential error = %v", err)
	}
}
