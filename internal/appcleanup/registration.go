package appcleanup

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

// Registration is made only by normal, non-elevated use, never by installation.
// The session SID is independently resolved by Windows, not by Explorer or an
// account-name environment variable. Every maintenance invocation checks it.
type userRegistration struct {
	current    func() (UserIdentity, error)
	sessionSID func() (string, error)
	elevated   func() bool
	read       func(string) (string, error)
	write      func(string, string) error
}

func registrationKey(executable string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", ErrIdentityUnavailable
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(filepath.Clean(executable))))), nil
}
func (r userRegistration) record(executable string) error {
	identity, err := r.current()
	if err != nil || r.elevated() {
		return ErrIdentityUnavailable
	}
	sid, err := r.sessionSID()
	if err != nil || sid == "" || sid != identity.SID {
		return ErrIdentityUnavailable
	}
	key, err := registrationKey(executable)
	if err != nil {
		return err
	}
	return r.write(key, sid)
}
func (r userRegistration) target(executable string) (string, error) {
	identity, err := r.current()
	if err != nil {
		return "", err
	}
	sid, err := r.sessionSID()
	if err != nil || sid == "" || sid != identity.SID {
		return "", ErrIdentityUnavailable
	}
	key, err := registrationKey(executable)
	if err != nil {
		return "", err
	}
	recorded, err := r.read(key)
	if err != nil || recorded != sid {
		return "", ErrIdentityUnavailable
	}
	return sid, nil
}
