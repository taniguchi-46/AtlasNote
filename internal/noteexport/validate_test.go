package noteexport

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidateInput(t *testing.T) {
	valid := validHTMLInput()
	tests := []struct {
		name  string
		input Input
		code  string
		field string
	}{
		{name: "missing note", input: withInput(valid, func(input *Input) { input.NoteID = " " }), code: ErrorCodeInvalidInput, field: "noteId"},
		{name: "malformed note", input: withInput(valid, func(input *Input) { input.NoteID = "../note" }), code: ErrorCodeInvalidInput, field: "noteId"},
		{name: "uppercase note", input: withInput(valid, func(input *Input) { input.NoteID = strings.Repeat("A", 32) }), code: ErrorCodeInvalidInput, field: "noteId"},
		{name: "invalid revision", input: withInput(valid, func(input *Input) { input.ExpectedRevision = 0 }), code: ErrorCodeInvalidInput, field: "expectedRevision"},
		{name: "invalid format", input: withInput(valid, func(input *Input) { input.Format = "docx" }), code: ErrorCodeInvalidFormat, field: "format"},
		{name: "invalid markdown UTF-8", input: withInput(valid, func(input *Input) { input.Markdown = string([]byte{0xff}) }), code: ErrorCodeInvalidInput, field: "markdown"},
		{name: "markdown too large", input: withInput(valid, func(input *Input) { input.Markdown = strings.Repeat("m", MaxMarkdownBytes+1) }), code: ErrorCodeTooLarge, field: "markdown"},
		{name: "HTML with PDF payload", input: withInput(valid, func(input *Input) { input.PDFBase64 = "cGRm" }), code: ErrorCodeInvalidInput, field: "pdfBase64"},
		{name: "invalid HTML UTF-8", input: withInput(valid, func(input *Input) { input.HTMLFragment = string([]byte{0xff}) }), code: ErrorCodeInvalidInput, field: "htmlFragment"},
		{name: "HTML too large", input: withInput(valid, func(input *Input) { input.HTMLFragment = strings.Repeat("h", MaxHTMLFragmentBytes+1) }), code: ErrorCodeTooLarge, field: "htmlFragment"},
		{name: "PDF with HTML payload", input: withInput(validPDFInput(), func(input *Input) { input.HTMLFragment = "<p>unexpected</p>" }), code: ErrorCodeInvalidInput, field: "htmlFragment"},
		{name: "PDF missing payload", input: withInput(validPDFInput(), func(input *Input) { input.PDFBase64 = "" }), code: ErrorCodeInvalidInput, field: "pdfBase64"},
		{name: "PDF encoded payload too large", input: withInput(validPDFInput(), func(input *Input) {
			input.PDFBase64 = strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxPDFBytes)+1)
		}), code: ErrorCodeTooLarge, field: "pdfBase64"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateInput(test.input)
			if got == nil || got.Code != test.code || got.Field != test.field {
				t.Fatalf("ValidateInput() = %#v, want code %q field %q", got, test.code, test.field)
			}
		})
	}

	if got := ValidateInput(valid); got != nil {
		t.Fatalf("valid HTML input error = %#v", got)
	}
	emptyHTML := valid
	emptyHTML.Markdown = ""
	emptyHTML.HTMLFragment = ""
	if got := ValidateInput(emptyHTML); got != nil {
		t.Fatalf("empty HTML note error = %#v", got)
	}
	if got := ValidateInput(validPDFInput()); got != nil {
		t.Fatalf("valid PDF input error = %#v", got)
	}
}

func TestFormatExtensionAndSuggestedFilename(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		format Format
		want   string
	}{
		{name: "HTML", title: "日本語ノート", format: FormatHTML, want: "日本語ノート.html"},
		{name: "PDF", title: "Report", format: FormatPDF, want: "Report.pdf"},
		{name: "invalid characters", title: ` A<B>:C"D/E\F|G?H* `, format: FormatHTML, want: "A_B__C_D_E_F_G_H_.html"},
		{name: "reserved device", title: "CON.txt", format: FormatPDF, want: "_CON.txt.pdf"},
		{name: "reserved superscript device", title: "LPT¹.txt", format: FormatPDF, want: "_LPT¹.txt.pdf"},
		{name: "reserved device with space", title: "NUL .txt", format: FormatHTML, want: "_NUL .txt.html"},
		{name: "empty", title: " . ", format: FormatHTML, want: "note.html"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SuggestedFilename(test.title, test.format); got != test.want {
				t.Fatalf("SuggestedFilename() = %q, want %q", got, test.want)
			}
		})
	}
	if got := FormatHTML.Extension(); got != ".html" {
		t.Fatalf("HTML extension = %q", got)
	}
	if got := FormatPDF.Extension(); got != ".pdf" {
		t.Fatalf("PDF extension = %q", got)
	}
	if got := Format("invalid").Extension(); got != "" {
		t.Fatalf("invalid extension = %q", got)
	}

	long := strings.Repeat("界", 100)
	filename := SuggestedFilename(long, FormatHTML)
	if len(strings.TrimSuffix(filename, ".html")) > maxSuggestedBaseBytes {
		t.Fatalf("suggested filename base is too long: %d bytes", len(filename))
	}
}

func withInput(input Input, mutate func(*Input)) Input {
	mutate(&input)
	return input
}
