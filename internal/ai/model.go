package ai

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	CredentialStoreServiceName = "atlasnote-ai"
	summaryInputLimitBytes     = 12 * 1024
	summaryOutputTokenLimit    = 512
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
}

type ConfigureProviderInput struct {
	ProviderID ProviderID `json:"providerID"`
	APIKey     string     `json:"apiKey"`
	ModelID    string     `json:"modelID"`
}

type TestConnectionInput struct {
	ProviderID ProviderID `json:"providerID"`
	APIKey     string     `json:"apiKey"`
}

type ConnectionTestResult struct {
	Success bool `json:"success"`
}

// ListModelsInput deliberately accepts a draft key so D-06 can load a model
// list before the user explicitly saves the credential.
type ListModelsInput struct {
	ProviderID ProviderID `json:"providerID"`
	APIKey     string     `json:"apiKey"`
}

// ModelInfo is provider-neutral metadata. Nil token limits mean that the
// provider did not expose the value, rather than that the limit is zero.
type ModelInfo struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	SupportsSummary  bool   `json:"supportsSummary"`
	InputTokenLimit  *int64 `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit *int64 `json:"outputTokenLimit,omitempty"`
	Available        bool   `json:"available"`
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

type ErrorCode string

const (
	ErrorCodeProviderUnsupported      ErrorCode = "AI_PROVIDER_UNSUPPORTED"
	ErrorCodeAPIKeyInvalid            ErrorCode = "AI_API_KEY_INVALID"
	ErrorCodeConfigurationUnavailable ErrorCode = "AI_CONFIGURATION_UNAVAILABLE"
	ErrorCodeCredentialUnavailable    ErrorCode = "AI_CREDENTIAL_UNAVAILABLE"
	ErrorCodeCredentialCleanup        ErrorCode = "AI_CREDENTIAL_CLEANUP_REQUIRED"
	ErrorCodeReauthenticationRequired ErrorCode = "AI_REAUTHENTICATION_REQUIRED"
	ErrorCodeAuthFailed               ErrorCode = "AI_AUTH_FAILED"
	ErrorCodeModelUnavailable         ErrorCode = "AI_MODEL_UNAVAILABLE"
	ErrorCodeInputTooLarge            ErrorCode = "AI_INPUT_TOO_LARGE"
	ErrorCodeInputInvalid             ErrorCode = "AI_INPUT_INVALID"
	ErrorCodeRateLimited              ErrorCode = "AI_RATE_LIMITED"
	ErrorCodeTimeout                  ErrorCode = "AI_TIMEOUT"
	ErrorCodeNetworkUnavailable       ErrorCode = "AI_NETWORK_UNAVAILABLE"
	ErrorCodeProviderUnavailable      ErrorCode = "AI_PROVIDER_UNAVAILABLE"
	ErrorCodeBusy                     ErrorCode = "AI_BUSY"
	ErrorCodeInvalidResponse          ErrorCode = "AI_INVALID_RESPONSE"
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
	ErrProviderUnsupported      = &SafeError{Code: ErrorCodeProviderUnsupported}
	ErrAPIKeyInvalid            = &SafeError{Code: ErrorCodeAPIKeyInvalid}
	ErrConfigurationUnavailable = &SafeError{Code: ErrorCodeConfigurationUnavailable}
	ErrCredentialUnavailable    = &SafeError{Code: ErrorCodeCredentialUnavailable}
	ErrCredentialCleanup        = &SafeError{Code: ErrorCodeCredentialCleanup}
	ErrReauthenticationRequired = &SafeError{Code: ErrorCodeReauthenticationRequired}
	ErrAuthFailed               = &SafeError{Code: ErrorCodeAuthFailed}
	ErrModelUnavailable         = &SafeError{Code: ErrorCodeModelUnavailable}
	ErrInputTooLarge            = &SafeError{Code: ErrorCodeInputTooLarge}
	ErrInputInvalid             = &SafeError{Code: ErrorCodeInputInvalid}
	ErrRateLimited              = &SafeError{Code: ErrorCodeRateLimited}
	ErrTimeout                  = &SafeError{Code: ErrorCodeTimeout}
	ErrNetworkUnavailable       = &SafeError{Code: ErrorCodeNetworkUnavailable}
	ErrProviderUnavailable      = &SafeError{Code: ErrorCodeProviderUnavailable}
	ErrBusy                     = &SafeError{Code: ErrorCodeBusy}
	ErrInvalidResponse          = &SafeError{Code: ErrorCodeInvalidResponse}
)

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
