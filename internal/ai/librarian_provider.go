package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (a *HTTPProviderAdapter) GenerateStructured(ctx context.Context, providerID ProviderID, apiKey string, input StructuredGenerationInput, onChunk func(string) error) (string, error) {
	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return "", err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return "", err
	}
	normalized, err := normalizeStructuredGenerationInput(providerID, input)
	if err != nil {
		return "", err
	}
	operationCtx, cancel := context.WithTimeout(nonNilContext(ctx), summaryGenerationTimeout)
	defer cancel()

	switch providerID {
	case ProviderOpenRouter:
		return a.generateOpenRouterStructured(operationCtx, apiKey, normalized, onChunk)
	case ProviderGemini:
		return a.generateGeminiStructured(operationCtx, apiKey, normalized, onChunk)
	default:
		return "", ErrProviderUnsupported
	}
}

func normalizeStructuredGenerationInput(providerID ProviderID, input StructuredGenerationInput) (StructuredGenerationInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || !utf8.ValidString(name) {
		return StructuredGenerationInput{}, ErrInputInvalid
	}
	if len([]byte(name)) > structuredNameLimitBytes {
		return StructuredGenerationInput{}, ErrInputTooLarge
	}
	switch name {
	case "atlas_note_librarian", "atlas_note_agent_edit":
	default:
		return StructuredGenerationInput{}, ErrInputInvalid
	}

	modelID, err := normalizeSummaryModelID(providerID, input.ModelID)
	if err != nil {
		return StructuredGenerationInput{}, err
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" || !utf8.ValidString(prompt) {
		return StructuredGenerationInput{}, ErrInputInvalid
	}
	if len([]byte(prompt)) > structuredPromptLimitBytes {
		return StructuredGenerationInput{}, ErrInputTooLarge
	}
	schema := bytes.TrimSpace(input.Schema)
	if len(schema) == 0 || !utf8.Valid(schema) || !json.Valid(schema) {
		return StructuredGenerationInput{}, ErrInputInvalid
	}
	if len(schema) > structuredSchemaLimitBytes {
		return StructuredGenerationInput{}, ErrInputTooLarge
	}
	var schemaObject map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaObject); err != nil || schemaObject == nil {
		return StructuredGenerationInput{}, ErrInputInvalid
	}
	if input.MaxOutputTokens < 1 || input.MaxOutputTokens > textMaxOutputTokens {
		return StructuredGenerationInput{}, ErrInputInvalid
	}
	return StructuredGenerationInput{
		Name:            name,
		ModelID:         modelID,
		Prompt:          prompt,
		Schema:          append(json.RawMessage(nil), schema...),
		MaxOutputTokens: input.MaxOutputTokens,
	}, nil
}

func (a *HTTPProviderAdapter) generateOpenRouterStructured(ctx context.Context, apiKey string, input StructuredGenerationInput, onChunk func(string) error) (string, error) {
	payload := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream    bool `json:"stream"`
		MaxTokens int  `json:"max_tokens"`
		Provider  struct {
			ZDR               bool   `json:"zdr"`
			DataCollection    string `json:"data_collection"`
			AllowFallbacks    bool   `json:"allow_fallbacks"`
			RequireParameters bool   `json:"require_parameters"`
		} `json:"provider"`
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string          `json:"name"`
				Strict bool            `json:"strict"`
				Schema json.RawMessage `json:"schema"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}{
		Model: input.ModelID, Stream: true, MaxTokens: input.MaxOutputTokens,
	}
	payload.Messages = append(payload.Messages, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "system", Content: "構造化JSONだけを返してください。"}, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: input.Prompt})
	payload.Provider.ZDR = true
	payload.Provider.DataCollection = "deny"
	payload.Provider.AllowFallbacks = false
	// Avoid silently routing the structured request to a fallback that does not
	// support the strict schema contract required by the caller.
	payload.Provider.RequireParameters = true
	payload.ResponseFormat.Type = "json_schema"
	payload.ResponseFormat.JSONSchema.Name = input.Name
	payload.ResponseFormat.JSONSchema.Strict = true
	payload.ResponseFormat.JSONSchema.Schema = input.Schema

	body, err := json.Marshal(payload)
	if err != nil {
		return "", ErrProviderUnavailable
	}
	return a.streamLibrarianRequest(ctx, apiKey, ProviderOpenRouter, openRouterSummaryEndpoint, body, onChunk, parseOpenRouterLibrarianChunk)
}

func (a *HTTPProviderAdapter) generateGeminiStructured(ctx context.Context, apiKey string, input StructuredGenerationInput, onChunk func(string) error) (string, error) {
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
			MaxOutputTokens    int                   `json:"maxOutputTokens"`
			ResponseMIMEType   string                `json:"responseMimeType"`
			ResponseJSONSchema json.RawMessage       `json:"responseJsonSchema"`
			ThinkingConfig     *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
		} `json:"generationConfig"`
		Store bool `json:"store"`
	}{Store: false}
	payload.SystemInstruction.Parts = append(payload.SystemInstruction.Parts, struct {
		Text string `json:"text"`
	}{Text: "構造化JSONだけを返してください。"})
	payload.Contents = append(payload.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user", Parts: []struct {
		Text string `json:"text"`
	}{{Text: input.Prompt}}})
	payload.GenerationConfig.MaxOutputTokens = input.MaxOutputTokens
	payload.GenerationConfig.ResponseMIMEType = "application/json"
	payload.GenerationConfig.ResponseJSONSchema = input.Schema
	if input.Name == "atlas_note_librarian" {
		payload.GenerationConfig.ThinkingConfig = geminiSummaryThinkingConfig(input.ModelID)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", ErrProviderUnavailable
	}
	endpoint := geminiSummaryEndpoint + input.ModelID + ":streamGenerateContent?alt=sse"
	return a.streamLibrarianRequest(ctx, apiKey, ProviderGemini, endpoint, body, onChunk, parseGeminiLibrarianChunk)
}

type librarianChunkParser func([]byte) (string, bool, error)

func (a *HTTPProviderAdapter) streamLibrarianRequest(ctx context.Context, apiKey string, providerID ProviderID, endpoint string, body []byte, onChunk func(string) error, parser librarianChunkParser) (string, error) {
	request, err := a.newRequest(ctx, http.MethodPost, endpoint, providerID, apiKey, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if err := statusError(response, true); err != nil {
		return "", err
	}
	return readLibrarianSSE(response.Body, onChunk, parser)
}

func readLibrarianSSE(body io.Reader, onChunk func(string) error, parser librarianChunkParser) (string, error) {
	limitedBody := &io.LimitedReader{R: body, N: int64(maxProviderResponseBytes + 1)}
	scanner := bufio.NewScanner(limitedBody)
	scanner.Buffer(make([]byte, 4096), maxProviderResponseBytes+1)
	var content strings.Builder
	streamFinished := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			streamFinished = true
			break
		}
		part, finished, err := parser([]byte(data))
		if err != nil {
			return "", err
		}
		if part != "" {
			if len(part) > maxProviderResponseBytes-content.Len() {
				return "", ErrOutputLimit
			}
			content.WriteString(part)
			if onChunk != nil {
				if err := onChunk(part); err != nil {
					return "", err
				}
			}
		}
		if finished {
			streamFinished = true
			break
		}
	}
	if limitedBody.N == 0 {
		return "", ErrOutputLimit
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return "", context.Canceled
		}
		return "", ErrNetworkUnavailable
	}
	if !streamFinished || strings.TrimSpace(content.String()) == "" {
		return "", ErrInvalidResponse
	}
	return content.String(), nil
}

func parseOpenRouterLibrarianChunk(data []byte) (string, bool, error) {
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			Delta struct {
				Content *string `json:"content"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false, ErrInvalidResponse
	}
	if len(payload.Error) > 0 && string(payload.Error) != "null" {
		return "", false, structuredStreamError(payload.Error)
	}
	if len(payload.Choices) == 0 {
		return "", false, nil
	}
	choice := payload.Choices[0]
	part := ""
	if choice.Delta.Content != nil {
		part = *choice.Delta.Content
	}
	if choice.FinishReason == nil || strings.TrimSpace(*choice.FinishReason) == "" {
		return part, false, nil
	}
	if strings.TrimSpace(*choice.FinishReason) != "stop" {
		return "", false, ErrInvalidResponse
	}
	return part, true, nil
}

func parseGeminiLibrarianChunk(data []byte) (string, bool, error) {
	var payload struct {
		Error          json.RawMessage `json:"error"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text *string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false, ErrInvalidResponse
	}
	if len(payload.Error) > 0 && string(payload.Error) != "null" {
		return "", false, structuredStreamError(payload.Error)
	}
	if len(payload.Candidates) == 0 {
		if strings.TrimSpace(payload.PromptFeedback.BlockReason) != "" {
			return "", false, ErrContentBlocked
		}
		return "", false, nil
	}
	candidate := payload.Candidates[0]
	var parts strings.Builder
	for _, item := range candidate.Content.Parts {
		if item.Text != nil {
			parts.WriteString(*item.Text)
		}
	}
	if strings.TrimSpace(candidate.FinishReason) == "" {
		return parts.String(), false, nil
	}
	if strings.TrimSpace(candidate.FinishReason) != "STOP" {
		return "", false, geminiFinishReasonError(candidate.FinishReason)
	}
	return parts.String(), true, nil
}

// structuredStreamError maps only machine-readable provider fields. The
// provider message and details are intentionally ignored so an SSE error
// cannot leak note content, credentials, or provider internals across Wails.
func structuredStreamError(raw json.RawMessage) error {
	var payload struct {
		Code   json.RawMessage `json:"code"`
		Status string          `json:"status"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ErrProviderUnavailable
	}
	switch strings.ToUpper(strings.TrimSpace(payload.Status)) {
	case "UNAUTHENTICATED", "PERMISSION_DENIED":
		return ErrAuthFailed
	case "NOT_FOUND":
		return ErrModelUnavailable
	case "INVALID_ARGUMENT", "UNSUPPORTED":
		return ErrModelCapabilityUnavailable
	case "FAILED_PRECONDITION":
		return ErrProviderConfiguration
	case "RESOURCE_EXHAUSTED":
		return ErrRateLimited
	}

	statusCode := 0
	if len(payload.Code) > 0 {
		if err := json.Unmarshal(payload.Code, &statusCode); err != nil {
			var codeText string
			if textErr := json.Unmarshal(payload.Code, &codeText); textErr == nil {
				statusCode, _ = strconv.Atoi(strings.TrimSpace(codeText))
			}
		}
	}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuthFailed
	case http.StatusNotFound:
		return ErrModelUnavailable
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrModelCapabilityUnavailable
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return ErrProviderUnavailable
	}
}

var _ StructuredStreamingProviderAdapter = (*HTTPProviderAdapter)(nil)
