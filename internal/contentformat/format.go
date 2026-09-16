package contentformat

import "encoding/base64"

// ProtectedContentPrefix and the envelope constants describe the on-disk
// format used by contentlock for Markdown and attachment bytes. Keeping the
// size calculation here prevents the attachment store from guessing how much
// authenticated-encryption overhead the current format adds.
const (
	ProtectedContentPrefix = "ATLASNOTE-LOCK-1\n"
	EnvelopeNonceBytes     = 12
	EnvelopeTagBytes       = 16
	emptyEnvelopeJSON      = `{"version":1,"nonce":"","ciphertext":""}`
)

// MaxProtectedContentBytes returns the exact maximum encoded size for a
// plaintext of the supplied size. The current envelope uses raw Base64 for a
// fixed-size nonce and the AES-GCM ciphertext (plaintext plus tag).
func MaxProtectedContentBytes(plaintextSize int) int {
	if plaintextSize < 0 {
		return 0
	}
	return len(ProtectedContentPrefix) + len(emptyEnvelopeJSON) +
		base64.RawStdEncoding.EncodedLen(EnvelopeNonceBytes) +
		base64.RawStdEncoding.EncodedLen(plaintextSize+EnvelopeTagBytes)
}
