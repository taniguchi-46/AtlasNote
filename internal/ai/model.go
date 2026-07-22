package ai

import (
	"strings"
	"unicode"
)

const CredentialStoreServiceName = "atlasnote-ai"

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

type ErrorCode string

const (
	ErrorCodeProviderUnsupported      ErrorCode = "AI_PROVIDER_UNSUPPORTED"
	ErrorCodeAPIKeyInvalid            ErrorCode = "AI_API_KEY_INVALID"
	ErrorCodeConfigurationUnavailable ErrorCode = "AI_CONFIGURATION_UNAVAILABLE"
	ErrorCodeCredentialUnavailable    ErrorCode = "AI_CREDENTIAL_UNAVAILABLE"
	ErrorCodeCredentialCleanup        ErrorCode = "AI_CREDENTIAL_CLEANUP_REQUIRED"
	ErrorCodeReauthenticationRequired ErrorCode = "AI_REAUTHENTICATION_REQUIRED"
	ErrorCodeAuthFailed               ErrorCode = "AI_AUTH_FAILED"
	ErrorCodeRateLimited              ErrorCode = "AI_RATE_LIMITED"
	ErrorCodeTimeout                  ErrorCode = "AI_TIMEOUT"
	ErrorCodeNetworkUnavailable       ErrorCode = "AI_NETWORK_UNAVAILABLE"
	ErrorCodeProviderUnavailable      ErrorCode = "AI_PROVIDER_UNAVAILABLE"
)

// SafeError deliberately contains only a stable, user-safe code. It must not
// wrap provider responses, request headers, or any secret-bearing input.
type SafeError struct {
	Code ErrorCode `json:"code"`
}

func (e *SafeError) Error() string {
	return string(e.Code)
}

var (
	ErrProviderUnsupported      = &SafeError{Code: ErrorCodeProviderUnsupported}
	ErrAPIKeyInvalid            = &SafeError{Code: ErrorCodeAPIKeyInvalid}
	ErrConfigurationUnavailable = &SafeError{Code: ErrorCodeConfigurationUnavailable}
	ErrCredentialUnavailable    = &SafeError{Code: ErrorCodeCredentialUnavailable}
	ErrCredentialCleanup        = &SafeError{Code: ErrorCodeCredentialCleanup}
	ErrReauthenticationRequired = &SafeError{Code: ErrorCodeReauthenticationRequired}
	ErrAuthFailed               = &SafeError{Code: ErrorCodeAuthFailed}
	ErrRateLimited              = &SafeError{Code: ErrorCodeRateLimited}
	ErrTimeout                  = &SafeError{Code: ErrorCodeTimeout}
	ErrNetworkUnavailable       = &SafeError{Code: ErrorCodeNetworkUnavailable}
	ErrProviderUnavailable      = &SafeError{Code: ErrorCodeProviderUnavailable}
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
