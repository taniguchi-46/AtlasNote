package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	connectionCheckTimeout   = 10 * time.Second
	modelListTimeout         = 10 * time.Second
	summaryGenerationTimeout = 60 * time.Second
	maxProviderResponseBytes = 1024 * 1024

	openRouterKeyEndpoint     = "https://openrouter.ai/api/v1/key"
	openRouterModelsEndpoint  = "https://openrouter.ai/api/v1/models"
	openRouterSummaryEndpoint = "https://openrouter.ai/api/v1/chat/completions"
	geminiModelsEndpoint      = "https://generativelanguage.googleapis.com/v1/models"
	geminiSummaryEndpoint     = "https://generativelanguage.googleapis.com/v1/models/"

	summaryInstruction = "次のメモを、事実を補わずに簡潔に要約してください。"
)

var errRedirectBlocked = errors.New("AI provider redirect blocked")

// ProviderAdapter is the only provider-specific boundary used by the AI
// application service. It deliberately exposes no conversation, streaming,
// routing, or credential-management operations.
type ProviderAdapter interface {
	CheckConnection(ctx context.Context, providerID ProviderID, apiKey string) error
	ListModels(ctx context.Context, providerID ProviderID, apiKey string) (ModelListResult, error)
	GenerateSummary(ctx context.Context, providerID ProviderID, apiKey string, input GenerateSummaryInput) (SummaryResult, error)
}

// ConnectionChecker remains as a narrow compatibility seam for D-02 callers.
// HTTPProviderAdapter implements both interfaces.
type ConnectionChecker interface {
	Check(ctx context.Context, providerID ProviderID, apiKey string) error
}

type HTTPProviderAdapter struct {
	client *http.Client
	now    func() time.Time
}

// HTTPConnectionChecker is retained for the existing D-02 tests and callers.
// New code should use HTTPProviderAdapter through ProviderAdapter.
type HTTPConnectionChecker = HTTPProviderAdapter

func NewProviderHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = nil
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirectBlocked
		},
	}
}

func NewHTTPProviderAdapter() *HTTPProviderAdapter {
	return NewHTTPProviderAdapterWithClient(NewProviderHTTPClient())
}

// NewHTTPProviderAdapterWithClient is for transport injection in tests. The
// application always uses NewHTTPProviderAdapter and never accepts a user URL.
func NewHTTPProviderAdapterWithClient(client *http.Client) *HTTPProviderAdapter {
	if client == nil {
		client = NewProviderHTTPClient()
	}
	return &HTTPProviderAdapter{client: client, now: time.Now}
}

func NewHTTPConnectionChecker() *HTTPProviderAdapter {
	return NewHTTPProviderAdapter()
}

func NewHTTPConnectionCheckerWithClient(client *http.Client) *HTTPProviderAdapter {
	return NewHTTPProviderAdapterWithClient(client)
}

func (a *HTTPProviderAdapter) Check(ctx context.Context, providerID ProviderID, apiKey string) error {
	return a.CheckConnection(ctx, providerID, apiKey)
}

func (a *HTTPProviderAdapter) CheckConnection(ctx context.Context, providerID ProviderID, apiKey string) error {
	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return err
	}

	endpoint := openRouterKeyEndpoint
	if providerID == ProviderGemini {
		endpoint = geminiModelsEndpoint + "?pageSize=1"
	}
	operationCtx, cancel := context.WithTimeout(nonNilContext(ctx), connectionCheckTimeout)
	defer cancel()

	request, err := a.newRequest(operationCtx, http.MethodGet, endpoint, providerID, apiKey, nil)
	if err != nil {
		return err
	}
	response, err := a.do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return statusError(response, false)
}

func (a *HTTPProviderAdapter) ListModels(ctx context.Context, providerID ProviderID, apiKey string) (ModelListResult, error) {
	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return ModelListResult{}, err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return ModelListResult{}, err
	}
	operationCtx, cancel := context.WithTimeout(nonNilContext(ctx), modelListTimeout)
	defer cancel()

	var models []ModelInfo
	switch providerID {
	case ProviderOpenRouter:
		models, err = a.listOpenRouterModels(operationCtx, apiKey)
	case ProviderGemini:
		models, err = a.listGeminiModels(operationCtx, apiKey)
	default:
		return ModelListResult{}, ErrProviderUnsupported
	}
	if err != nil {
		return ModelListResult{}, err
	}
	return ModelListResult{Models: models, RetrievedAt: a.now().UTC()}, nil
}

func (a *HTTPProviderAdapter) GenerateSummary(ctx context.Context, providerID ProviderID, apiKey string, input GenerateSummaryInput) (SummaryResult, error) {
	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return SummaryResult{}, err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return SummaryResult{}, err
	}
	normalized, err := normalizeSummaryInput(input)
	if err != nil {
		return SummaryResult{}, err
	}
	if normalized.ProviderID != providerID {
		return SummaryResult{}, ErrProviderUnsupported
	}
	operationCtx, cancel := context.WithTimeout(nonNilContext(ctx), summaryGenerationTimeout)
	defer cancel()

	switch providerID {
	case ProviderOpenRouter:
		return a.generateOpenRouterSummary(operationCtx, apiKey, normalized)
	case ProviderGemini:
		return a.generateGeminiSummary(operationCtx, apiKey, normalized)
	default:
		return SummaryResult{}, ErrProviderUnsupported
	}
}

func (a *HTTPProviderAdapter) listOpenRouterModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	request, err := a.newRequest(ctx, http.MethodGet, openRouterModelsEndpoint, ProviderOpenRouter, apiKey, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := statusError(response, false); err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength *int64 `json:"context_length"`
			Architecture  struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			TopProvider struct {
				MaxCompletionTokens *int64 `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := decodeProviderJSON(response.Body, &payload); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(payload.Data))
	seen := make(map[string]struct{}, len(payload.Data))
	for _, model := range payload.Data {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" || !includesText(model.Architecture.InputModalities) || !includesText(model.Architecture.OutputModalities) {
			continue
		}
		if _, duplicate := seen[modelID]; duplicate {
			continue
		}
		seen[modelID] = struct{}{}
		displayName := strings.TrimSpace(model.Name)
		if displayName == "" {
			displayName = modelID
		}
		models = append(models, ModelInfo{
			ID:               modelID,
			DisplayName:      displayName,
			SupportsSummary:  true,
			InputTokenLimit:  copyInt64(model.ContextLength),
			OutputTokenLimit: copyInt64(model.TopProvider.MaxCompletionTokens),
			Available:        true,
		})
	}
	return models, nil
}

func (a *HTTPProviderAdapter) listGeminiModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	models := make([]ModelInfo, 0)
	seenModels := make(map[string]struct{})
	seenTokens := make(map[string]struct{})
	pageToken := ""

	for {
		endpoint := geminiModelsEndpoint + "?pageSize=100"
		if pageToken != "" {
			endpoint += "&pageToken=" + url.QueryEscape(pageToken)
		}
		request, err := a.newRequest(ctx, http.MethodGet, endpoint, ProviderGemini, apiKey, nil)
		if err != nil {
			return nil, err
		}
		response, err := a.do(request)
		if err != nil {
			return nil, err
		}
		if err := statusError(response, false); err != nil {
			response.Body.Close()
			return nil, err
		}
		var payload struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				InputTokenLimit            *int64   `json:"inputTokenLimit"`
				OutputTokenLimit           *int64   `json:"outputTokenLimit"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		decodeErr := decodeProviderJSON(response.Body, &payload)
		response.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}

		for _, model := range payload.Models {
			if !includesGenerationMethod(model.SupportedGenerationMethods) {
				continue
			}
			modelID, err := normalizeSummaryModelID(ProviderGemini, model.Name)
			if err != nil {
				continue
			}
			if _, duplicate := seenModels[modelID]; duplicate {
				continue
			}
			seenModels[modelID] = struct{}{}
			displayName := strings.TrimSpace(model.DisplayName)
			if displayName == "" {
				displayName = modelID
			}
			models = append(models, ModelInfo{
				ID:               modelID,
				DisplayName:      displayName,
				SupportsSummary:  true,
				InputTokenLimit:  copyInt64(model.InputTokenLimit),
				OutputTokenLimit: copyInt64(model.OutputTokenLimit),
				Available:        true,
			})
		}

		pageToken = strings.TrimSpace(payload.NextPageToken)
		if pageToken == "" {
			return models, nil
		}
		if _, repeated := seenTokens[pageToken]; repeated {
			return nil, ErrInvalidResponse
		}
		seenTokens[pageToken] = struct{}{}
	}
}

func (a *HTTPProviderAdapter) generateOpenRouterSummary(ctx context.Context, apiKey string, input GenerateSummaryInput) (SummaryResult, error) {
	payload := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream    bool `json:"stream"`
		MaxTokens int  `json:"max_tokens"`
		Provider  struct {
			ZDR            bool   `json:"zdr"`
			DataCollection string `json:"data_collection"`
			AllowFallbacks bool   `json:"allow_fallbacks"`
		} `json:"provider"`
	}{
		Model:     input.ModelID,
		Stream:    false,
		MaxTokens: summaryOutputTokenLimit,
	}
	payload.Messages = append(payload.Messages,
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "system", Content: summaryInstruction},
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "user", Content: input.Content},
	)
	payload.Provider.ZDR = true
	payload.Provider.DataCollection = "deny"
	payload.Provider.AllowFallbacks = false
	return a.postOpenRouterSummary(ctx, apiKey, payload)
}

func (a *HTTPProviderAdapter) postOpenRouterSummary(ctx context.Context, apiKey string, payload any) (SummaryResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return SummaryResult{}, ErrProviderUnavailable
	}
	request, err := a.newRequest(ctx, http.MethodPost, openRouterSummaryEndpoint, ProviderOpenRouter, apiKey, bytes.NewReader(body))
	if err != nil {
		return SummaryResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.do(request)
	if err != nil {
		return SummaryResult{}, err
	}
	defer response.Body.Close()
	if err := statusError(response, true); err != nil {
		return SummaryResult{}, err
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := decodeProviderJSON(response.Body, &result); err != nil {
		return SummaryResult{}, err
	}
	if len(result.Choices) == 0 || result.Choices[0].FinishReason != "stop" {
		return SummaryResult{}, ErrInvalidResponse
	}
	var text string
	if err := json.Unmarshal(result.Choices[0].Message.Content, &text); err != nil || strings.TrimSpace(text) == "" {
		return SummaryResult{}, ErrInvalidResponse
	}
	return SummaryResult{Text: text}, nil
}

func (a *HTTPProviderAdapter) generateGeminiSummary(ctx context.Context, apiKey string, input GenerateSummaryInput) (SummaryResult, error) {
	payload := struct {
		SystemInstruction struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"systemInstruction"`
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"generationConfig"`
		Store bool `json:"store"`
	}{Store: false}
	payload.SystemInstruction.Parts = append(payload.SystemInstruction.Parts, struct {
		Text string `json:"text"`
	}{Text: summaryInstruction})
	content := struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user"}
	content.Parts = append(content.Parts, struct {
		Text string `json:"text"`
	}{Text: input.Content})
	payload.Contents = append(payload.Contents, content)
	payload.GenerationConfig.MaxOutputTokens = summaryOutputTokenLimit

	body, err := json.Marshal(payload)
	if err != nil {
		return SummaryResult{}, ErrProviderUnavailable
	}
	endpoint := geminiSummaryEndpoint + url.PathEscape(input.ModelID) + ":generateContent"
	request, err := a.newRequest(ctx, http.MethodPost, endpoint, ProviderGemini, apiKey, bytes.NewReader(body))
	if err != nil {
		return SummaryResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.do(request)
	if err != nil {
		return SummaryResult{}, err
	}
	defer response.Body.Close()
	if err := statusError(response, true); err != nil {
		return SummaryResult{}, err
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text *string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := decodeProviderJSON(response.Body, &result); err != nil {
		return SummaryResult{}, err
	}
	if len(result.Candidates) == 0 || result.Candidates[0].FinishReason != "STOP" || len(result.Candidates[0].Content.Parts) == 0 {
		return SummaryResult{}, ErrInvalidResponse
	}
	parts := make([]string, 0, len(result.Candidates[0].Content.Parts))
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text == nil {
			return SummaryResult{}, ErrInvalidResponse
		}
		parts = append(parts, *part.Text)
	}
	text := strings.Join(parts, "")
	if strings.TrimSpace(text) == "" {
		return SummaryResult{}, ErrInvalidResponse
	}
	return SummaryResult{Text: text}, nil
}

func (a *HTTPProviderAdapter) newRequest(ctx context.Context, method, endpoint string, providerID ProviderID, apiKey string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	switch providerID {
	case ProviderOpenRouter:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	case ProviderGemini:
		request.Header.Set("x-goog-api-key", apiKey)
	default:
		return nil, ErrProviderUnsupported
	}
	return request, nil
}

func (a *HTTPProviderAdapter) do(request *http.Request) (*http.Response, error) {
	response, err := a.client.Do(request)
	if err == nil {
		return response, nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(request.Context().Err(), context.DeadlineExceeded) {
		return nil, ErrTimeout
	}
	if errors.Is(err, context.Canceled) || errors.Is(request.Context().Err(), context.Canceled) {
		return nil, ErrProviderUnavailable
	}
	return nil, ErrNetworkUnavailable
}

func statusError(response *http.Response, generation bool) error {
	switch {
	case response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices:
		return nil
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		return ErrAuthFailed
	case response.StatusCode == http.StatusTooManyRequests:
		return rateLimitError(response.Header.Get("Retry-After"))
	case generation && (response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity):
		return ErrModelUnavailable
	default:
		return ErrProviderUnavailable
	}
}

func rateLimitError(value string) error {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 || seconds > 24*60*60 {
		return ErrRateLimited
	}
	return &SafeError{Code: ErrorCodeRateLimited, RetryAfterSeconds: &seconds}
}

func decodeProviderJSON(body io.Reader, target any) error {
	limited := io.LimitReader(body, maxProviderResponseBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil || len(content) > maxProviderResponseBytes {
		return ErrInvalidResponse
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidResponse
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidResponse
	}
	return nil
}

func includesText(values []string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), "text") {
			return true
		}
	}
	return false
}

func includesGenerationMethod(values []string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), "generateContent") {
			return true
		}
	}
	return false
}

func copyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
