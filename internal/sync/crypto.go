package sync

import (
	"crypto/rand"
	"encoding/hex"
)

func secureRandomHex(buffer []byte) (string, error) {
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
