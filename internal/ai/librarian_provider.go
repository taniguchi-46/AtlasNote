package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func (a *HTTPProviderAdapter) GenerateLibrarian(ctx context.Context, providerID ProviderID, apiKey string, input LibrarianProviderInput, onChunk func(string) error) (string, error) {
	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return "", err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return "", err
	}
	if !isLibrarianOperation(input.Operation) || strings.TrimSpace(input.ModelID) == "" || strings.TrimSpace(input.Prompt) == "" || len(input.Schema) == 0 {
		return "", ErrInputInvalid
	}
	operationCtx, cancel := context.WithTimeout(nonNilContext(ctx), summaryGenerationTimeout)
	defer cancel()

	switch providerID {
	case ProviderOpenRouter:
		return a.generateOpenRouterLibrarian(operationCtx, apiKey, input, onChunk)
	case ProviderGemini:
		return a.generateGeminiLibrarian(operationCtx, apiKey, input, onChunk)
	default:
		return "", ErrProviderUnsupported
	}
}

func (a *HTTPProviderAdapter) generateOpenRouterLibrarian(ctx context.Context, apiKey string, input LibrarianProviderInput, onChunk func(string) error) (string, error) {
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
		Model: input.ModelID, Stream: true, MaxTokens: summaryOutputTokenLimit,
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
	// support the strict schema contract required by the librarian.
	payload.Provider.RequireParameters = true
	payload.ResponseFormat.Type = "json_schema"
	payload.ResponseFormat.JSONSchema.Name = "atlas_note_librarian"
	payload.ResponseFormat.JSONSchema.Strict = true
	payload.ResponseFormat.JSONSchema.Schema = input.Schema

	body, err := json.Marshal(payload)
	if err != nil {
		return "", ErrProviderUnavailable
	}
	return a.streamLibrarianRequest(ctx, apiKey, ProviderOpenRouter, openRouterSummaryEndpoint, body, onChunk, parseOpenRouterLibrarianChunk)
}

func (a *HTTPProviderAdapter) generateGeminiLibrarian(ctx context.Context, apiKey string, input LibrarianProviderInput, onChunk func(string) error) (string, error) {
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
			MaxOutputTokens  int             `json:"maxOutputTokens"`
			ResponseMIMEType string          `json:"responseMimeType"`
			ResponseSchema   json.RawMessage `json:"responseSchema"`
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
	payload.GenerationConfig.MaxOutputTokens = summaryOutputTokenLimit
	payload.GenerationConfig.ResponseMIMEType = "application/json"
	payload.GenerationConfig.ResponseSchema = input.Schema

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
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), maxProviderResponseBytes)
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
		return "", false, ErrProviderUnavailable
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
		Error      json.RawMessage `json:"error"`
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
		return "", false, ErrProviderUnavailable
	}
	if len(payload.Candidates) == 0 {
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
		return "", false, ErrInvalidResponse
	}
	return parts.String(), true, nil
}

var _ StructuredStreamingProviderAdapter = (*HTTPProviderAdapter)(nil)
