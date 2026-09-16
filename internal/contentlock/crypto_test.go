package contentlock

import (
	"bytes"
	"testing"

	"atlasnote/internal/contentformat"
)

func TestEncryptScopedContentFitsSharedMaximumAtAttachmentLimit(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, kdfKeyLength)
	materials := []keyMaterial{{ID: "lock", Key: key}}
	plain := bytes.Repeat([]byte("x"), 10*1024*1024)

	encoded, err := encryptAttachment("note", "attachment", materials, plain)
	if err != nil {
		t.Fatalf("encrypt attachment: %v", err)
	}
	if len(encoded) != contentformat.MaxProtectedContentBytes(len(plain)) {
		t.Fatalf("encoded length = %d, want exact shared maximum %d", len(encoded), contentformat.MaxProtectedContentBytes(len(plain)))
	}
}
