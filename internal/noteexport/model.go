package noteexport

const (
	ErrorCodeBusy                          = "NOTE_EXPORT_BUSY"
	ErrorCodeInvalidInput                  = "NOTE_EXPORT_INVALID_INPUT"
	ErrorCodeInvalidFormat                 = "NOTE_EXPORT_INVALID_FORMAT"
	ErrorCodeNoteNotFound                  = "NOTE_EXPORT_NOTE_NOT_FOUND"
	ErrorCodeLocked                        = "NOTE_EXPORT_LOCKED"
	ErrorCodeStale                         = "NOTE_EXPORT_STALE"
	ErrorCodeProtectedConfirmationRequired = "NOTE_EXPORT_PROTECTED_CONFIRMATION_REQUIRED"
	ErrorCodeTooLarge                      = "NOTE_EXPORT_TOO_LARGE"
	ErrorCodeRenderFailed                  = "NOTE_EXPORT_RENDER_FAILED"
	ErrorCodeWriteFailed                   = "NOTE_EXPORT_WRITE_FAILED"
	ErrorCodeUnavailable                   = "NOTE_EXPORT_UNAVAILABLE"
)

const (
	MaxMarkdownBytes     = 2 * 1024 * 1024
	MaxHTMLFragmentBytes = 8 * 1024 * 1024
	MaxPDFBytes          = 32 * 1024 * 1024
)

type Format string

const (
	FormatHTML Format = "html"
	FormatPDF  Format = "pdf"
)

func (format Format) Extension() string {
	switch format {
	case FormatHTML:
		return ".html"
	case FormatPDF:
		return ".pdf"
	default:
		return ""
	}
}

type Input struct {
	NoteID                  string `json:"noteId"`
	ExpectedRevision        int64  `json:"expectedRevision"`
	Title                   string `json:"title"`
	Markdown                string `json:"markdown"`
	HTMLFragment            string `json:"htmlFragment"`
	PDFBase64               string `json:"pdfBase64"`
	Format                  Format `json:"format"`
	AllowPlaintextProtected bool   `json:"allowPlaintextProtected"`
}

type Result struct {
	Cancelled    bool      `json:"cancelled"`
	ExportedName string    `json:"exportedName,omitempty"`
	Error        *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Field     string `json:"field,omitempty"`
	Retryable bool   `json:"retryable"`
}

func NewErrorResult(code string, message string, field string, retryable bool) Result {
	return Result{Error: &APIError{
		Code:      code,
		Message:   message,
		Field:     field,
		Retryable: retryable,
	}}
}
