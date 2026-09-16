package contentformat

import (
	"encoding/base64"
	"testing"
)

func TestMaxProtectedContentBytesMatchesEnvelopeShape(t *testing.T) {
	plain := 10 * 1024 * 1024
	encodedSize := MaxProtectedContentBytes(plain)
	if encodedSize <= plain {
		t.Fatalf("encoded size = %d, want larger than plaintext %d", encodedSize, plain)
	}
	if MaxProtectedContentBytes(plain-1) >= encodedSize {
		t.Fatalf("encoded size is not monotonic at the attachment boundary")
	}

	wantEmpty := len(ProtectedContentPrefix) + len(emptyEnvelopeJSON) +
		base64.RawStdEncoding.EncodedLen(EnvelopeNonceBytes) +
		base64.RawStdEncoding.EncodedLen(EnvelopeTagBytes)
	if got := MaxProtectedContentBytes(0); got != wantEmpty {
		t.Fatalf("empty protected size = %d, want %d", got, wantEmpty)
	}
}
