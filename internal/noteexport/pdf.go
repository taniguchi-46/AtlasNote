package noteexport

import (
	"bytes"
	"encoding/base64"
	"strings"
	"unicode"
)

func decodePDF(value string) ([]byte, *APIError) {
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return nil, apiError(ErrorCodeRenderFailed, "PDFデータを検証できません。", "pdfBase64", false)
	}
	if len(value) > base64.StdEncoding.EncodedLen(MaxPDFBytes) {
		return nil, apiError(ErrorCodeTooLarge, "PDFがエクスポート上限を超えています。", "pdfBase64", false)
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, apiError(ErrorCodeRenderFailed, "PDFデータを検証できません。", "pdfBase64", false)
	}
	if len(decoded) > MaxPDFBytes {
		return nil, apiError(ErrorCodeTooLarge, "PDFがエクスポート上限を超えています。", "pdfBase64", false)
	}
	if !bytes.HasPrefix(decoded, []byte("%PDF-")) {
		return nil, apiError(ErrorCodeRenderFailed, "PDFの開始位置を検証できません。", "pdfBase64", false)
	}
	trimmed := bytes.TrimRight(decoded, " \t\r\n\f")
	if !bytes.HasSuffix(trimmed, []byte("%%EOF")) {
		return nil, apiError(ErrorCodeRenderFailed, "PDFの終端を検証できません。", "pdfBase64", false)
	}
	return decoded, nil
}
