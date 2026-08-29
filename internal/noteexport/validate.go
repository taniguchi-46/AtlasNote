package noteexport

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

func ValidateInput(input Input) *APIError {
	if !validNoteID(input.NoteID) {
		return apiError(ErrorCodeInvalidInput, "エクスポートするノートを確認できません。", "noteId", false)
	}
	if input.ExpectedRevision < 1 {
		return apiError(ErrorCodeInvalidInput, "ノートのrevisionを確認できません。", "expectedRevision", false)
	}
	if input.Format.Extension() == "" {
		return apiError(ErrorCodeInvalidFormat, "エクスポート形式を確認できません。", "format", false)
	}
	if !utf8.ValidString(input.Markdown) {
		return apiError(ErrorCodeInvalidInput, "ノート本文をUTF-8として検証できません。", "markdown", false)
	}
	if len(input.Markdown) > MaxMarkdownBytes {
		return apiError(ErrorCodeTooLarge, "ノート本文がエクスポート上限を超えています。", "markdown", false)
	}

	switch input.Format {
	case FormatHTML:
		if input.PDFBase64 != "" {
			return apiError(ErrorCodeInvalidInput, "HTMLエクスポートにはPDFデータを指定できません。", "pdfBase64", false)
		}
		if !utf8.ValidString(input.HTMLFragment) {
			return apiError(ErrorCodeInvalidInput, "HTMLをUTF-8として検証できません。", "htmlFragment", false)
		}
		if len(input.HTMLFragment) > MaxHTMLFragmentBytes {
			return apiError(ErrorCodeTooLarge, "HTMLがエクスポート上限を超えています。", "htmlFragment", false)
		}
	case FormatPDF:
		if input.HTMLFragment != "" {
			return apiError(ErrorCodeInvalidInput, "PDFエクスポートにはHTMLデータを指定できません。", "htmlFragment", false)
		}
		if input.PDFBase64 == "" {
			return apiError(ErrorCodeInvalidInput, "PDFデータがありません。", "pdfBase64", false)
		}
		if len(input.PDFBase64) > base64.StdEncoding.EncodedLen(MaxPDFBytes) {
			return apiError(ErrorCodeTooLarge, "PDFがエクスポート上限を超えています。", "pdfBase64", false)
		}
	}

	return nil
}

func validNoteID(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func apiError(code string, message string, field string, retryable bool) *APIError {
	return &APIError{Code: code, Message: message, Field: field, Retryable: retryable}
}
