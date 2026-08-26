package noteimport

const (
	ErrorCodeBusy               = "NOTE_IMPORT_BUSY"
	ErrorCodeInvalidDestination = "NOTE_IMPORT_INVALID_DESTINATION"
	ErrorCodeInvalidTitleMode   = "NOTE_IMPORT_INVALID_TITLE_MODE"
	ErrorCodeNotebookCreate     = "NOTE_IMPORT_NOTEBOOK_CREATE_FAILED"
	ErrorCodePersistence        = "NOTE_IMPORT_PERSISTENCE_FAILED"
	ErrorCodeCancelled          = "NOTE_IMPORT_CANCELLED"

	FailureCodeUnsupportedFile = "NOTE_IMPORT_UNSUPPORTED_FILE"
	FailureCodeRead            = "NOTE_IMPORT_READ_FAILED"
	FailureCodeTooLarge        = "NOTE_IMPORT_FILE_TOO_LARGE"
	FailureCodeEncoding        = "NOTE_IMPORT_INVALID_ENCODING"
	FailureCodeHTML            = "NOTE_IMPORT_INVALID_HTML"
	FailureCodeEmptyHTML       = "NOTE_IMPORT_EMPTY_HTML"
	FailureCodeCreate          = "NOTE_IMPORT_CREATE_FAILED"
)

// Input contains the destination and title mode selected in the UI. Source
// paths are deliberately selected by the backend-native file dialog and never
// accepted from the frontend bridge.
type Input struct {
	NotebookID      *string   `json:"notebookId,omitempty"`
	NewNotebookName *string   `json:"newNotebookName,omitempty"`
	TitleMode       TitleMode `json:"titleMode,omitempty"`
}

type TitleMode string

const (
	TitleModeAuto     TitleMode = "auto"
	TitleModeFilename TitleMode = "filename"
	TitleModeHeading  TitleMode = "heading"
	TitleModeMetadata TitleMode = "metadata"
)

type Result struct {
	Cancelled       bool             `json:"cancelled"`
	Imported        []ImportedNote   `json:"imported"`
	Failures        []FileFailure    `json:"failures"`
	CreatedNotebook *CreatedNotebook `json:"createdNotebook,omitempty"`
	Error           *APIError        `json:"error,omitempty"`
}

type ImportedNote struct {
	SourceName string `json:"sourceName"`
	NoteID     string `json:"noteId"`
	Title      string `json:"title"`
}

type FileFailure struct {
	SourceName string `json:"sourceName"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type CreatedNotebook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func NewResult() Result {
	return Result{
		Imported: make([]ImportedNote, 0),
		Failures: make([]FileFailure, 0),
	}
}

func NewErrorResult(code string, message string, retryable bool) Result {
	result := NewResult()
	result.Error = &APIError{Code: code, Message: message, Retryable: retryable}
	return result
}
