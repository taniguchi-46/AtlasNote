package contentlock

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	kdfMemoryKiB   = 64 * 1024
	kdfIterations  = 3
	kdfParallelism = 1
	kdfKeyLength   = 32
	contentPrefix  = "ATLASNOTE-LOCK-1\n"
)

type contentEnvelope struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type keyMaterial struct {
	ID  string
	Key []byte
}

func newID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate content lock ID: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func randomBytes(size int) ([]byte, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return nil, fmt.Errorf("generate content lock secret: %w", err)
	}
	return buffer, nil
}

func validatePassphrase(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrValidation
	}
	if len([]rune(value)) == 0 {
		return "", ErrPassphraseRequired
	}
	if len([]rune(value)) < 8 || len([]rune(value)) > 1024 {
		return "", ErrValidation
	}
	return value, nil
}

func derivePassphraseKey(passphrase string, salt []byte, memoryKiB int, iterations int, parallelism int) ([]byte, error) {
	if memoryKiB < 8192 || iterations < 1 || parallelism < 1 || len(salt) < 16 {
		return nil, ErrIntegrity
	}
	return argon2.IDKey([]byte(passphrase), salt, uint32(iterations), uint32(memoryKiB), uint8(parallelism), kdfKeyLength), nil
}

func wrapLockKey(lockID string, passphrase string, salt []byte, memoryKiB int, iterations int, parallelism int, lockKey []byte) ([]byte, []byte, error) {
	passphraseKey, err := derivePassphraseKey(passphrase, salt, memoryKiB, iterations, parallelism)
	if err != nil {
		return nil, nil, err
	}
	defer zeroBytes(passphraseKey)
	nonce, ciphertext, err := seal(passphraseKey, []byte("atlasnote-content-lock-key/v1:"+lockID), lockKey)
	if err != nil {
		return nil, nil, err
	}
	return nonce, ciphertext, nil
}

func unwrapLockKey(lockID string, passphrase string, salt []byte, memoryKiB int, iterations int, parallelism int, nonce []byte, wrappedKey []byte) ([]byte, error) {
	passphraseKey, err := derivePassphraseKey(passphrase, salt, memoryKiB, iterations, parallelism)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(passphraseKey)
	plain, err := open(passphraseKey, []byte("atlasnote-content-lock-key/v1:"+lockID), nonce, wrappedKey)
	if err != nil {
		return nil, ErrPassphraseInvalid
	}
	if len(plain) != kdfKeyLength {
		zeroBytes(plain)
		return nil, ErrIntegrity
	}
	return plain, nil
}

func deriveContentKey(noteID string, materials []keyMaterial) ([]byte, error) {
	if noteID == "" || len(materials) == 0 {
		return nil, ErrIntegrity
	}
	ordered := append([]keyMaterial(nil), materials...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	hash := sha256.New()
	_, _ = hash.Write([]byte("atlasnote-content-key/v1\x00"))
	_, _ = hash.Write([]byte(noteID))
	for _, material := range ordered {
		if material.ID == "" || len(material.Key) != kdfKeyLength {
			return nil, ErrIntegrity
		}
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(material.ID))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(material.Key)
	}
	return hash.Sum(nil), nil
}

func encryptContent(noteID string, materials []keyMaterial, plain []byte) ([]byte, error) {
	key, err := deriveContentKey(noteID, materials)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)
	nonce, ciphertext, err := seal(key, []byte("atlasnote-content/v1:"+noteID), plain)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(contentEnvelope{
		Version: 1, Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return nil, fmt.Errorf("encode protected content: %w", err)
	}
	return append([]byte(contentPrefix), encoded...), nil
}

func decryptContent(noteID string, materials []keyMaterial, encoded []byte) ([]byte, error) {
	if !isEncryptedContent(encoded) {
		return nil, ErrIntegrity
	}
	var envelope contentEnvelope
	if err := json.Unmarshal(encoded[len(contentPrefix):], &envelope); err != nil || envelope.Version != 1 {
		return nil, ErrIntegrity
	}
	nonce, err := base64.RawStdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, ErrIntegrity
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, ErrIntegrity
	}
	key, err := deriveContentKey(noteID, materials)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)
	plain, err := open(key, []byte("atlasnote-content/v1:"+noteID), nonce, ciphertext)
	if err != nil {
		return nil, ErrIntegrity
	}
	return plain, nil
}

func isEncryptedContent(value []byte) bool {
	return strings.HasPrefix(string(value), contentPrefix)
}

func seal(key []byte, additionalData []byte, plain []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	nonce, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plain, additionalData), nil
}

func open(key []byte, additionalData []byte, nonce []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrIntegrity
	}
	return gcm.Open(nil, nonce, ciphertext, additionalData)
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
