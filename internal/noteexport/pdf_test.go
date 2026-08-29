package noteexport

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDecodePDF(t *testing.T) {
	validPDF := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF \r\n\t")
	tests := []struct {
		name  string
		value string
		code  string
	}{
		{name: "invalid base64", value: "not-base64!", code: ErrorCodeRenderFailed},
		{name: "base64 whitespace", value: base64.StdEncoding.EncodeToString(validPDF)[:8] + "\n" + base64.StdEncoding.EncodeToString(validPDF)[8:], code: ErrorCodeRenderFailed},
		{name: "missing signature", value: base64.StdEncoding.EncodeToString([]byte("PDF-1.7\n%%EOF")), code: ErrorCodeRenderFailed},
		{name: "missing EOF", value: base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\nbody")), code: ErrorCodeRenderFailed},
		{name: "content after EOF", value: base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\n%%EOF\ncontent")), code: ErrorCodeRenderFailed},
		{name: "too large", value: strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxPDFBytes)+1), code: ErrorCodeTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decoded, got := decodePDF(test.value)
			if decoded != nil || got == nil || got.Code != test.code {
				t.Fatalf("decodePDF() = %d bytes, %#v; want %q", len(decoded), got, test.code)
			}
		})
	}

	decoded, got := decodePDF(base64.StdEncoding.EncodeToString(validPDF))
	if got != nil || string(decoded) != string(validPDF) {
		t.Fatalf("valid decode = %q, %#v", decoded, got)
	}
}
