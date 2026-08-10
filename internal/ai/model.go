package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	CredentialStoreServiceName = "atlasnote-ai"
	// summaryInputLimitBytes is an absolute local safety limit. The effective
	// limit is narrowed further from the selected model's context window before
	// a provider request is made.
	summaryInputLimitBytes             = 2 * 1024 * 1024
	summaryUnknownModelInputLimitBytes = 64 * 1024
	summaryInstructionTokenReserve     = 1024
	summaryOutputTokenLimit            = 1024
	librarianInputLimitBytes           = 12 * 1024
	textInstructionLimitBytes          = 16 * 1024
	textMessageLimitBytes              = 64 * 1024
	textMaxMessages                    = 24
	textMaxOutputTokens                = 4096
	structuredNameLimitBytes           = 128
	structuredPromptLimitBytes         = 128 * 1024
	structuredSchemaLimitBytes         = 16 * 1024
)

type ProviderID string

const (
	ProviderOpenRouter ProviderID = "openrouter"
	ProviderGemini     ProviderID = "gemini"
)

var supportedProviders = []ProviderID{ProviderOpenRouter, ProviderGemini}

type CredentialStatus string

const (
	CredentialStatusNotConfigured            CredentialStatus = "not-configured"
	CredentialStatusPersistent               CredentialStatus = "persistent"
	CredentialStatusSessionOnly              CredentialStatus = "session-only"
	CredentialStatusReauthenticationRequired CredentialStatus = "reauthentication-required"
)

type credentialStorage string

const (
	credentialStoragePersistent  credentialStorage = "persistent"
	credentialStorageSessionOnly credentialStorage = "session-only"
)

// ProviderSettings is the safe, UI-facing representation of one provider.
// Credential references and API keys intentionally never cross this boundary.
type ProviderSettings struct {
	ProviderID       ProviderID       `json:"providerID"`
	ModelID          string           `json:"modelID"`
	CredentialStatus CredentialStatus `json:"credentialStatus"`
	IsSelected       bool             `json:"isSelected"`
}

type ConfigureProviderInput struct {
	ProviderID ProviderID `json:"providerID"`
	APIKey     string     `json:"apiKey"`
	ModelID    string     `json:"modelID"`
}

type TestConnectionInput struct {
	ProviderID          ProviderID `json:"providerID"`
	APIKey              string     `json:"apiKey"`
	UseStoredCredential bool       `json:"useStoredCredential"`
}

type ConnectionTestResult struct {
	Success bool `json:"success"`
}

// ListModelsInput accepts either a draft key or an explicitly selected saved
// credential. The key is never persisted by this request.
type ListModelsInput struct {
	ProviderID          ProviderID `json:"providerID"`
	APIKey              string     `json:"apiKey"`
	UseStoredCredential bool       `json:"useStoredCredential"`
}

// UpdateProviderModelInput changes only the non-secret selected model for an
// already configured provider. It deliberately contains no credential field.
type UpdateProviderModelInput struct {
	ProviderID ProviderID `json:"providerID"`
	ModelID    string     `json:"modelID"`
}

// TestGenerationInput sends a fixed, non-user-content probe to the selected
// model. It accepts either a draft key or an explicitly selected saved
// credential and never returns generated text.
type TestGenerationInput struct {
	ProviderID          ProviderID `json:"providerID"`
	ModelID             string     `json:"modelID"`
	APIKey              string     `json:"apiKey"`
	UseStoredCredential bool       `json:"useStoredCredential"`
}

// ModelInfo is provider-neutral metadata. Nil token limits mean that the
// provider did not expose the value, rather than that the limit is zero.
type ModelInfo struct {
	ID                       string `json:"id"`
	DisplayName              string `json:"displayName"`
	SupportsSummary          bool   `json:"supportsSummary"`
	SupportsTextGeneration   bool   `json:"supportsTextGeneration"`
	SupportsStructuredOutput bool   `json:"supportsStructuredOutput"`
	SupportsStreaming        bool   `json:"supportsStreaming"`
	SupportsLibrarian        bool   `json:"supportsLibrarian"`
	InputTokenLimit          *int64 `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit         *int64 `json:"outputTokenLimit,omitempty"`
	Available                bool   `json:"available"`
}

type ModelListResult struct {
	Models      []ModelInfo `json:"models"`
	RetrievedAt time.Time   `json:"retrievedAt"`
}

// GenerateSummaryInput contains only the selected provider/model and the
// current note body. API keys are resolved internally from CredentialStore.
type GenerateSummaryInput struct {
	ProviderID ProviderID `json:"providerID"`
	ModelID    string     `json:"modelID"`
	Content    string     `json:"content"`
}

type SummaryResult struct {
	Text string `json:"text"`
}

// ModelListResponse and SummaryResponse are Wails-safe envelopes. Provider
// errors are data rather than raw Go error strings so Retry-After survives the
// Wails boundary without exposing request or provider internals.
type ModelListResponse struct {
	Models      []ModelInfo `json:"models"`
	RetrievedAt time.Time   `json:"retrievedAt,omitempty"`
	Error       *SafeError  `json:"error,omitempty"`
}

type SummaryResponse struct {
	Text  string     `json:"text,omitempty"`
	Error *SafeError `json:"error,omitempty"`
}

type AssistantKind string

const (
	AssistantKindQA         AssistantKind = "qa"
	AssistantKindBrainstorm AssistantKind = "brainstorm"
)

type ChatMode string

const (
	ChatModeAsk   ChatMode = "ask"
	ChatModeAgent ChatMode = "agent"
)

type WebCitation struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type AIRecordStatus string

const (
	AIRecordStatusSaved    AIRecordStatus = "saved"
	AIRecordStatusStale    AIRecordStatus = "stale"
	AIRecordStatusOrphaned AIRecordStatus = "orphaned"
)

type AIConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIContextInput struct {
	NoteIDs          []string `json:"noteIDs,omitempty"`
	SearchQuery      string   `json:"searchQuery,omitempty"`
	IncludeBacklinks bool     `json:"includeBacklinks,omitempty"`
}

type AIContextSource struct {
	NoteID      string `json:"noteID"`
	Title       string `json:"title"`
	Revision    int64  `json:"revision"`
	Snippet     string `json:"snippet,omitempty"`
	ContentByte int    `json:"contentByte"`
}

type AIContextResponse struct {
	Sources []AIContextSource `json:"sources"`
	Error   *SafeError        `json:"error,omitempty"`
}

type AssistantInput struct {
	ProviderID       ProviderID              `json:"providerID"`
	ModelID          string                  `json:"modelID"`
	Kind             AssistantKind           `json:"kind"`
	Mode             ChatMode                `json:"mode,omitempty"`
	Question         string                  `json:"question"`
	Messages         []AIConversationMessage `json:"messages,omitempty"`
	NoteIDs          []string                `json:"noteIDs,omitempty"`
	SearchQuery      string                  `json:"searchQuery,omitempty"`
	IncludeBacklinks bool                    `json:"includeBacklinks,omitempty"`
	WebSearch        bool                    `json:"webSearch,omitempty"`
	ExpectedSources  []AIHistorySource       `json:"expectedSources,omitempty"`
	AgentTarget      *AgentEditTarget        `json:"agentTarget,omitempty"`
}

type AssistantResult struct {
	ProviderID        ProviderID              `json:"providerID"`
	ModelID           string                  `json:"modelID"`
	Kind              AssistantKind           `json:"kind"`
	Mode              ChatMode                `json:"mode"`
	Messages          []AIConversationMessage `json:"messages"`
	Sources           []AIContextSource       `json:"sources"`
	Citations         []WebCitation           `json:"citations,omitempty"`
	WebSearchRequests int                     `json:"webSearchRequests,omitempty"`
	Proposal          *AgentEditProposal      `json:"proposal,omitempty"`
}

// AgentEditTarget identifies the active note snapshot that a restricted Agent
// may propose a single body-only edit for. The service fills all returned
// target fields from its validated snapshot rather than trusting the model.
type AgentEditTarget struct {
	NoteID       string `json:"noteID"`
	BaseRevision int64  `json:"baseRevision"`
}

// AgentEditProposal is an exact one-hunk replacement for the target note
// body. It is only a proposal; applying it remains an explicit UI action that
// goes through the existing note revision/CAS save path.
type AgentEditProposal struct {
	TargetNoteID   string   `json:"targetNoteID"`
	TargetTitle    string   `json:"targetTitle"`
	BaseRevision   int64    `json:"baseRevision"`
	Reason         string   `json:"reason"`
	Before         string   `json:"before"`
	After          string   `json:"after"`
	AffectedFields []string `json:"affectedFields"`
}

type AssistantResponse struct {
	Result *AssistantResult `json:"result,omitempty"`
	Error  *SafeError       `json:"error,omitempty"`
}

type AIHistorySource struct {
	NoteID        string `json:"noteID"`
	InputRevision int64  `json:"inputRevision"`
}

type AIHistory struct {
	ID         string                  `json:"id"`
	Kind       AssistantKind           `json:"kind"`
	Title      string                  `json:"title"`
	ProviderID ProviderID              `json:"providerID"`
	ModelID    string                  `json:"modelID"`
	Status     AIRecordStatus          `json:"status"`
	Messages   []AIConversationMessage `json:"messages,omitempty"`
	Sources    []AIHistorySource       `json:"sources"`
	CreatedAt  time.Time               `json:"createdAt"`
	UpdatedAt  time.Time               `json:"updatedAt"`
}

type SaveAIHistoryInput struct {
	ID         string                  `json:"id,omitempty"`
	Kind       AssistantKind           `json:"kind"`
	Title      string                  `json:"title"`
	ProviderID ProviderID              `json:"providerID"`
	ModelID    string                  `json:"modelID"`
	Messages   []AIConversationMessage `json:"messages"`
	Sources    []AIHistorySource       `json:"sources"`
}

type AIHistoryResponse struct {
	History *AIHistory `json:"history,omitempty"`
	Error   *SafeError `json:"error,omitempty"`
}

type AIHistoryListResponse struct {
	Items []AIHistory `json:"items"`
	Error *SafeError  `json:"error,omitempty"`
}

type WritingKind string

const (
	WritingKindPrompt            WritingKind = "prompt"
	WritingKindPromptImprovement WritingKind = "prompt-improvement"
	WritingKindREADME            WritingKind = "readme"
	WritingKindDocument          WritingKind = "document"
	WritingKindBlog              WritingKind = "blog"
	WritingKindRequirements      WritingKind = "requirements"
)

type WritingInput struct {
	ProviderID       ProviderID        `json:"providerID"`
	ModelID          string            `json:"modelID"`
	Kind             WritingKind       `json:"kind"`
	Instruction      string            `json:"instruction"`
	NoteIDs          []string          `json:"noteIDs,omitempty"`
	SearchQuery      string            `json:"searchQuery,omitempty"`
	IncludeBacklinks bool              `json:"includeBacklinks,omitempty"`
	ExpectedSources  []AIHistorySource `json:"expectedSources,omitempty"`
}

type WritingResult struct {
	ProviderID ProviderID        `json:"providerID"`
	ModelID    string            `json:"modelID"`
	Kind       WritingKind       `json:"kind"`
	Content    string            `json:"content"`
	Sources    []AIContextSource `json:"sources"`
}

type WritingResponse struct {
	Result *WritingResult `json:"result,omitempty"`
	Error  *SafeError     `json:"error,omitempty"`
}

type ArtifactKind string

const (
	ArtifactKindSummary           ArtifactKind = "summary"
	ArtifactKindPrompt            ArtifactKind = ArtifactKind(WritingKindPrompt)
	ArtifactKindPromptImprovement ArtifactKind = ArtifactKind(WritingKindPromptImprovement)
	ArtifactKindREADME            ArtifactKind = ArtifactKind(WritingKindREADME)
	ArtifactKindDocument          ArtifactKind = ArtifactKind(WritingKindDocument)
	ArtifactKindBlog              ArtifactKind = ArtifactKind(WritingKindBlog)
	ArtifactKindRequirements      ArtifactKind = ArtifactKind(WritingKindRequirements)
)

type AIArtifact struct {
	ID         string            `json:"id"`
	Kind       ArtifactKind      `json:"kind"`
	Title      string            `json:"title"`
	ProviderID ProviderID        `json:"providerID"`
	ModelID    string            `json:"modelID"`
	Content    string            `json:"content"`
	Status     AIRecordStatus    `json:"status"`
	Sources    []AIHistorySource `json:"sources"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

type SaveAIArtifactInput struct {
	ID         string            `json:"id,omitempty"`
	Kind       ArtifactKind      `json:"kind"`
	Title      string            `json:"title"`
	ProviderID ProviderID        `json:"providerID"`
	ModelID    string            `json:"modelID"`
	Content    string            `json:"content"`
	Sources    []AIHistorySource `json:"sources"`
}

type AIArtifactResponse struct {
	Artifact *AIArtifact `json:"artifact,omitempty"`
	Error    *SafeError  `json:"error,omitempty"`
}

type AIArtifactListResponse struct {
	Items []AIArtifact `json:"items"`
	Error *SafeError   `json:"error,omitempty"`
}

type AIDeleteResponse struct {
	Deleted bool       `json:"deleted"`
	Error   *SafeError `json:"error,omitempty"`
}

type ErrorCode string

const (
	ErrorCodeProviderUnsupported        ErrorCode = "AI_PROVIDER_UNSUPPORTED"
	ErrorCodeAPIKeyInvalid              ErrorCode = "AI_API_KEY_INVALID"
	ErrorCodeConfigurationUnavailable   ErrorCode = "AI_CONFIGURATION_UNAVAILABLE"
	ErrorCodeCredentialUnavailable      ErrorCode = "AI_CREDENTIAL_UNAVAILABLE"
	ErrorCodeCredentialCleanup          ErrorCode = "AI_CREDENTIAL_CLEANUP_REQUIRED"
	ErrorCodeReauthenticationRequired   ErrorCode = "AI_REAUTHENTICATION_REQUIRED"
	ErrorCodeAuthFailed                 ErrorCode = "AI_AUTH_FAILED"
	ErrorCodeProviderConfiguration      ErrorCode = "AI_PROVIDER_CONFIGURATION_REQUIRED"
	ErrorCodeModelUnavailable           ErrorCode = "AI_MODEL_UNAVAILABLE"
	ErrorCodeModelCapabilityUnavailable ErrorCode = "AI_MODEL_CAPABILITY_UNAVAILABLE"
	ErrorCodeOutputLimit                ErrorCode = "AI_OUTPUT_LIMIT"
	ErrorCodeContentBlocked             ErrorCode = "AI_CONTENT_BLOCKED"
	ErrorCodeInputTooLarge              ErrorCode = "AI_INPUT_TOO_LARGE"
	ErrorCodeInputInvalid               ErrorCode = "AI_INPUT_INVALID"
	ErrorCodeRateLimited                ErrorCode = "AI_RATE_LIMITED"
	ErrorCodeTimeout                    ErrorCode = "AI_TIMEOUT"
	ErrorCodeNetworkUnavailable         ErrorCode = "AI_NETWORK_UNAVAILABLE"
	ErrorCodeProviderUnavailable        ErrorCode = "AI_PROVIDER_UNAVAILABLE"
	ErrorCodeBusy                       ErrorCode = "AI_BUSY"
	ErrorCodeInvalidResponse            ErrorCode = "AI_INVALID_RESPONSE"
	ErrorCodeCancelled                  ErrorCode = "AI_CANCELLED"
	ErrorCodeHistoryNotFound            ErrorCode = "AI_HISTORY_NOT_FOUND"
	ErrorCodeArtifactNotFound           ErrorCode = "AI_ARTIFACT_NOT_FOUND"
	ErrorCodeContextChanged             ErrorCode = "AI_CONTEXT_CHANGED"
)

// SafeError deliberately contains only stable, user-safe information. It must
// never wrap provider responses, request headers, API keys, or note content.
type SafeError struct {
	Code              ErrorCode `json:"code"`
	RetryAfterSeconds *int      `json:"retryAfterSeconds,omitempty"`
}

func (e *SafeError) Error() string {
	return string(e.Code)
}

func (e *SafeError) Is(target error) bool {
	other, ok := target.(*SafeError)
	return ok && e != nil && other != nil && e.Code == other.Code
}

var (
	ErrProviderUnsupported        = &SafeError{Code: ErrorCodeProviderUnsupported}
	ErrAPIKeyInvalid              = &SafeError{Code: ErrorCodeAPIKeyInvalid}
	ErrConfigurationUnavailable   = &SafeError{Code: ErrorCodeConfigurationUnavailable}
	ErrCredentialUnavailable      = &SafeError{Code: ErrorCodeCredentialUnavailable}
	ErrCredentialCleanup          = &SafeError{Code: ErrorCodeCredentialCleanup}
	ErrReauthenticationRequired   = &SafeError{Code: ErrorCodeReauthenticationRequired}
	ErrAuthFailed                 = &SafeError{Code: ErrorCodeAuthFailed}
	ErrProviderConfiguration      = &SafeError{Code: ErrorCodeProviderConfiguration}
	ErrModelUnavailable           = &SafeError{Code: ErrorCodeModelUnavailable}
	ErrModelCapabilityUnavailable = &SafeError{Code: ErrorCodeModelCapabilityUnavailable}
	ErrOutputLimit                = &SafeError{Code: ErrorCodeOutputLimit}
	ErrContentBlocked             = &SafeError{Code: ErrorCodeContentBlocked}
	ErrInputTooLarge              = &SafeError{Code: ErrorCodeInputTooLarge}
	ErrInputInvalid               = &SafeError{Code: ErrorCodeInputInvalid}
	ErrRateLimited                = &SafeError{Code: ErrorCodeRateLimited}
	ErrTimeout                    = &SafeError{Code: ErrorCodeTimeout}
	ErrNetworkUnavailable         = &SafeError{Code: ErrorCodeNetworkUnavailable}
	ErrProviderUnavailable        = &SafeError{Code: ErrorCodeProviderUnavailable}
	ErrBusy                       = &SafeError{Code: ErrorCodeBusy}
	ErrInvalidResponse            = &SafeError{Code: ErrorCodeInvalidResponse}
	ErrCancelled                  = &SafeError{Code: ErrorCodeCancelled}
	ErrHistoryNotFound            = &SafeError{Code: ErrorCodeHistoryNotFound}
	ErrArtifactNotFound           = &SafeError{Code: ErrorCodeArtifactNotFound}
	ErrContextChanged             = &SafeError{Code: ErrorCodeContextChanged}
)

type LibrarianOperation string

const (
	LibrarianOperationTitle          LibrarianOperation = "title"
	LibrarianOperationTags           LibrarianOperation = "tags"
	LibrarianOperationClassification LibrarianOperation = "classification"
	LibrarianOperationRelated        LibrarianOperation = "related"
	LibrarianOperationDuplicate      LibrarianOperation = "duplicate"
)

const (
	LibrarianEventName         = "ai:librarian:update"
	LibrarianMinCandidateCount = 1
	LibrarianMaxCandidateCount = 10
	LibrarianMaxCandidatePool  = 20
	LibrarianReasonLimitRunes  = 160
	LibrarianSnippetLimitRunes = 240
)

// LibrarianInput is a bounded, note snapshot plus a locally selected
// candidate pool. The backend never performs a whole-vault AI scan.
type LibrarianInput struct {
	ProviderID     ProviderID                  `json:"providerID"`
	ModelID        string                      `json:"modelID"`
	Operation      LibrarianOperation          `json:"operation"`
	NoteID         string                      `json:"noteID"`
	BaseRevision   int64                       `json:"baseRevision"`
	Title          string                      `json:"title"`
	Content        string                      `json:"content"`
	CandidateCount int                         `json:"candidateCount"`
	Candidates     []LibrarianCandidateContext `json:"candidates,omitempty"`
	ExistingTags   []LibrarianTagContext       `json:"existingTags,omitempty"`
	Notebooks      []LibrarianNotebookContext  `json:"notebooks,omitempty"`
}

type LibrarianCandidateContext struct {
	NoteID  string `json:"noteID"`
	Title   string `json:"title"`
	Snippet string `json:"snippet,omitempty"`
}

type LibrarianTagContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LibrarianNotebookContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LibrarianCandidate struct {
	Value      string  `json:"value,omitempty"`
	Name       string  `json:"name,omitempty"`
	NotebookID string  `json:"notebookID,omitempty"`
	NoteID     string  `json:"noteID,omitempty"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason,omitempty"`
	NewTag     bool    `json:"newTag,omitempty"`
}

type LibrarianResult struct {
	Operation  LibrarianOperation   `json:"operation"`
	Quality    string               `json:"quality"`
	Candidates []LibrarianCandidate `json:"candidates"`
}

type LibrarianStartResponse struct {
	RequestID string     `json:"requestID,omitempty"`
	Error     *SafeError `json:"error,omitempty"`
}

type LibrarianCancelResponse struct {
	Canceled bool       `json:"canceled"`
	Error    *SafeError `json:"error,omitempty"`
}

type LibrarianEvent struct {
	RequestID    string             `json:"requestID"`
	NoteID       string             `json:"noteID"`
	BaseRevision int64              `json:"baseRevision"`
	Operation    LibrarianOperation `json:"operation"`
	Phase        string             `json:"phase"`
	Sequence     int                `json:"sequence"`
	PartialText  string             `json:"partialText,omitempty"`
	Result       *LibrarianResult   `json:"result,omitempty"`
	Error        *SafeError         `json:"error,omitempty"`
}

// StructuredGenerationInput is the provider-neutral request built by the Go
// service. Prompt construction and the JSON schema stay on the Go side.
type StructuredGenerationInput struct {
	Name            string
	ModelID         string
	Prompt          string
	Schema          json.RawMessage
	MaxOutputTokens int
}

// StructuredStreamingProviderAdapter is optional so existing v1 test doubles
// and callers keep the ProviderAdapter contract unchanged.
type StructuredStreamingProviderAdapter interface {
	GenerateStructured(ctx context.Context, providerID ProviderID, apiKey string, input StructuredGenerationInput, onChunk func(string) error) (string, error)
}

type TextMessage struct {
	Role    string
	Content string
}

type TextGenerationInput struct {
	ModelID           string
	SystemInstruction string
	Messages          []TextMessage
	MaxOutputTokens   int
	WebSearch         bool
}

type TextGenerationResult struct {
	Text              string
	Citations         []WebCitation
	WebSearchRequests int
}

type TextGenerationProviderAdapter interface {
	GenerateText(ctx context.Context, providerID ProviderID, apiKey string, input TextGenerationInput) (TextGenerationResult, error)
}

func normalizeProviderID(value ProviderID) (ProviderID, error) {
	provider := ProviderID(strings.ToLower(strings.TrimSpace(string(value))))
	for _, supported := range supportedProviders {
		if provider == supported {
			return provider, nil
		}
	}
	return "", ErrProviderUnsupported
}

func validateAPIKey(value string) error {
	if value == "" {
		return ErrAPIKeyInvalid
	}
	for _, character := range value {
		if character == '\r' || character == '\n' || unicode.IsControl(character) {
			return ErrAPIKeyInvalid
		}
	}
	return nil
}

func normalizeModelID(value string) (string, error) {
	modelID := strings.TrimSpace(value)
	for _, character := range modelID {
		if unicode.IsControl(character) {
			return "", ErrConfigurationUnavailable
		}
	}
	return modelID, nil
}

func normalizeSummaryInput(input GenerateSummaryInput) (GenerateSummaryInput, error) {
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return GenerateSummaryInput{}, err
	}
	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return GenerateSummaryInput{}, err
	}
	if !utf8.ValidString(input.Content) || strings.TrimSpace(input.Content) == "" {
		return GenerateSummaryInput{}, ErrInputInvalid
	}
	if len([]byte(input.Content)) > summaryInputLimitBytes {
		return GenerateSummaryInput{}, ErrInputTooLarge
	}
	return GenerateSummaryInput{ProviderID: providerID, ModelID: modelID, Content: input.Content}, nil
}

func normalizeSummaryModelID(providerID ProviderID, value string) (string, error) {
	modelID, err := normalizeModelID(value)
	if err != nil {
		return "", ErrModelUnavailable
	}
	if modelID == "" {
		return "", ErrModelUnavailable
	}
	switch providerID {
	case ProviderOpenRouter:
		if strings.EqualFold(modelID, "openrouter/auto") {
			return "", ErrModelUnavailable
		}
	case ProviderGemini:
		modelID = strings.TrimPrefix(modelID, "models/")
		if modelID == "" || strings.ContainsAny(modelID, "/\\:?&#") {
			return "", ErrModelUnavailable
		}
		for _, character := range modelID {
			if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.') {
				return "", ErrModelUnavailable
			}
		}
	}
	return modelID, nil
}

// SafeErrorFrom converts internal failures into a detached, safe value for
// Wails results. The copy prevents mutation of package-level sentinel errors.
func SafeErrorFrom(err error) *SafeError {
	if err == nil {
		return nil
	}
	var safeError *SafeError
	if !errors.As(err, &safeError) || safeError == nil {
		return &SafeError{Code: ErrorCodeProviderUnavailable}
	}
	copy := &SafeError{Code: safeError.Code}
	if safeError.RetryAfterSeconds != nil {
		seconds := *safeError.RetryAfterSeconds
		copy.RetryAfterSeconds = &seconds
	}
	return copy
}
