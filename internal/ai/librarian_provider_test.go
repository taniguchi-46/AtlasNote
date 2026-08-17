package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPProviderAdapterStreamsStrictLibrarianRequests(t *testing.T) {
	testCases := []struct {
		name     string
		provider ProviderID
		modelID  string
		endpoint string
		validate func(*testing.T, map[string]any)
		response string
	}{
		{
			name:     "OpenRouter",
			provider: ProviderOpenRouter,
			modelID:  "openai/gpt-test",
			endpoint: openRouterSummaryEndpoint,
			response: "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"candidates\\\":\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"[]}\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n",
			validate: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["model"] != "openai/gpt-test" || payload["stream"] != true || payload["max_tokens"] != float64(summaryOutputTokenLimit) {
					t.Fatalf("OpenRouter librarian payload = %#v", payload)
				}
				provider, ok := payload["provider"].(map[string]any)
				if !ok || provider["zdr"] != true || provider["data_collection"] != "deny" || provider["allow_fallbacks"] != false || provider["require_parameters"] != true {
					t.Fatalf("OpenRouter privacy settings = %#v", provider)
				}
				format, ok := payload["response_format"].(map[string]any)
				if !ok || format["type"] != "json_schema" {
					t.Fatalf("OpenRouter response format = %#v", payload["response_format"])
				}
				schema, ok := format["json_schema"].(map[string]any)
				if !ok || schema["strict"] != true {
					t.Fatalf("OpenRouter JSON schema = %#v", format["json_schema"])
				}
			},
		},
		{
			name:     "Gemini 3.6 Flash",
			provider: ProviderGemini,
			modelID:  "gemini-3.6-flash",
			endpoint: geminiSummaryEndpoint + "gemini-3.6-flash:streamGenerateContent?alt=sse",
			response: "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"candidates\\\":\"}]}}]}\n\ndata: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"[]}\"}]},\"finishReason\":\"STOP\"}]}\n",
			validate: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["store"] != false {
					t.Fatalf("Gemini librarian request must set store=false: %#v", payload)
				}
				config, ok := payload["generationConfig"].(map[string]any)
				if !ok || config["maxOutputTokens"] != float64(summaryOutputTokenLimit) || config["responseMimeType"] != "application/json" {
					t.Fatalf("Gemini librarian generation config = %#v", config)
				}
				thinkingConfig, ok := config["thinkingConfig"].(map[string]any)
				if !ok || thinkingConfig["thinkingLevel"] != "minimal" {
					t.Fatalf("Gemini librarian thinking config = %#v", config["thinkingConfig"])
				}
				responseJSONSchema, ok := config["responseJsonSchema"].(map[string]any)
				if !ok || responseJSONSchema["additionalProperties"] != false {
					t.Fatalf("Gemini librarian response JSON schema = %#v", config["responseJsonSchema"])
				}
				if _, exists := config["responseSchema"]; exists {
					t.Fatalf("Gemini librarian must not send legacy responseSchema = %#v", config["responseSchema"])
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			chunks := make([]string, 0, 2)
			adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.String() != testCase.endpoint {
					t.Fatalf("librarian request = %s %s", request.Method, request.URL)
				}
				if request.Header.Get("Content-Type") != "application/json" {
					t.Fatal("librarian request omitted JSON content type")
				}
				if testCase.provider == ProviderOpenRouter && request.Header.Get("Authorization") != "Bearer librarian-key" {
					t.Fatal("OpenRouter librarian request omitted authorization")
				}
				if testCase.provider == ProviderGemini && request.Header.Get("X-Goog-Api-Key") != "librarian-key" {
					t.Fatal("Gemini librarian request omitted API key")
				}
				assertDeadline(t, request, summaryGenerationTimeout)
				payload := make(map[string]any)
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Fatalf("decode librarian request: %v", err)
				}
				testCase.validate(t, payload)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(testCase.response)),
				}, nil
			})})

			result, err := adapter.GenerateStructured(context.Background(), testCase.provider, "librarian-key", StructuredGenerationInput{
				Name:            "atlas_note_librarian",
				ModelID:         testCase.modelID,
				Prompt:          "bounded librarian prompt",
				Schema:          json.RawMessage(`{"type":"object","additionalProperties":false}`),
				MaxOutputTokens: summaryOutputTokenLimit,
			}, func(chunk string) error {
				chunks = append(chunks, chunk)
				return nil
			})
			if err != nil {
				t.Fatalf("generate librarian: %v", err)
			}
			if result != `{"candidates":[]}` || len(chunks) != 2 || chunks[0] == "" || chunks[1] == "" {
				t.Fatalf("librarian stream = result:%q chunks:%#v", result, chunks)
			}
		})
	}
}

func TestHTTPProviderAdapterStreamsStrictGeminiAgentRequests(t *testing.T) {
	const endpoint = geminiSummaryEndpoint + "gemini-3.6-flash:streamGenerateContent?alt=sse"
	chunks := make([]string, 0, 1)
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != endpoint {
			t.Fatalf("Gemini Agent request = %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("X-Goog-Api-Key") != "agent-key" {
			t.Fatal("Gemini Agent request headers were incomplete")
		}
		assertDeadline(t, request, summaryGenerationTimeout)
		payload := make(map[string]any)
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode Gemini Agent request: %v", err)
		}
		if payload["store"] != false {
			t.Fatalf("Gemini Agent request must set store=false: %#v", payload)
		}
		config, ok := payload["generationConfig"].(map[string]any)
		if !ok || config["maxOutputTokens"] != float64(summaryOutputTokenLimit) || config["responseMimeType"] != "application/json" {
			t.Fatalf("Gemini Agent generation config = %#v", config)
		}
		if _, exists := config["thinkingConfig"]; exists {
			t.Fatalf("Gemini Agent request must not override thinkingConfig = %#v", config["thinkingConfig"])
		}
		responseJSONSchema, ok := config["responseJsonSchema"].(map[string]any)
		if !ok || responseJSONSchema["additionalProperties"] != false {
			t.Fatalf("Gemini Agent response JSON schema = %#v", config["responseJsonSchema"])
		}
		required, ok := responseJSONSchema["required"].([]any)
		if !ok || len(required) != 1 || required[0] != "message" {
			t.Fatalf("Gemini Agent required fields = %#v", responseJSONSchema["required"])
		}
		if _, exists := config["responseSchema"]; exists {
			t.Fatalf("Gemini Agent must not send legacy responseSchema = %#v", config["responseSchema"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"message\\\":\\\"ok\\\"}\"}]},\"finishReason\":\"STOP\"}]}\n")),
		}, nil
	})})

	result, err := adapter.GenerateStructured(context.Background(), ProviderGemini, "agent-key", StructuredGenerationInput{
		Name:            "atlas_note_agent_edit",
		ModelID:         "gemini-3.6-flash",
		Prompt:          "bounded Agent prompt",
		Schema:          json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		MaxOutputTokens: summaryOutputTokenLimit,
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("generate Gemini Agent structured response: %v", err)
	}
	if result != `{"message":"ok"}` || len(chunks) != 1 || chunks[0] != result {
		t.Fatalf("Gemini Agent stream = result:%q chunks:%#v", result, chunks)
	}
}

func TestHTTPProviderAdapterStreamsStrictOpenRouterAgentRequests(t *testing.T) {
	const endpoint = openRouterSummaryEndpoint
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != endpoint {
			t.Fatalf("OpenRouter Agent request = %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Authorization") != "Bearer agent-key" {
			t.Fatal("OpenRouter Agent request headers were incomplete")
		}
		payload := make(map[string]any)
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode OpenRouter Agent request: %v", err)
		}
		if payload["stream"] != true || payload["model"] != "openai/gpt-test" {
			t.Fatalf("OpenRouter Agent request = %#v", payload)
		}
		provider, ok := payload["provider"].(map[string]any)
		if !ok || provider["require_parameters"] != true || provider["allow_fallbacks"] != false {
			t.Fatalf("OpenRouter Agent provider settings = %#v", payload["provider"])
		}
		format, ok := payload["response_format"].(map[string]any)
		if !ok || format["type"] != "json_schema" {
			t.Fatalf("OpenRouter Agent response format = %#v", payload["response_format"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"message\\\":\\\"ok\\\"}\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n")),
		}, nil
	})})

	result, err := adapter.GenerateStructured(context.Background(), ProviderOpenRouter, "agent-key", StructuredGenerationInput{
		Name:            "atlas_note_agent_edit",
		ModelID:         "openai/gpt-test",
		Prompt:          "bounded Agent prompt",
		Schema:          json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		MaxOutputTokens: summaryOutputTokenLimit,
	}, nil)
	if err != nil {
		t.Fatalf("generate OpenRouter Agent structured response: %v", err)
	}
	if result != `{"message":"ok"}` {
		t.Fatalf("OpenRouter Agent result = %q", result)
	}
}

func TestHTTPProviderAdapterRejectsUnsafeStructuredInputBeforeHTTP(t *testing.T) {
	base := StructuredGenerationInput{
		Name:            "atlas_note_librarian",
		ModelID:         "openai/gpt-test",
		Prompt:          "bounded structured prompt",
		Schema:          json.RawMessage(`{"type":"object","additionalProperties":false}`),
		MaxOutputTokens: summaryOutputTokenLimit,
	}
	for _, testCase := range []struct {
		name  string
		input StructuredGenerationInput
		want  error
	}{
		{
			name:  "unknown structured request name",
			input: StructuredGenerationInput{Name: "untrusted_request", ModelID: base.ModelID, Prompt: base.Prompt, Schema: base.Schema, MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputInvalid,
		},
		{
			name:  "oversized structured request name",
			input: StructuredGenerationInput{Name: strings.Repeat("x", structuredNameLimitBytes+1), ModelID: base.ModelID, Prompt: base.Prompt, Schema: base.Schema, MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputTooLarge,
		},
		{
			name:  "invalid UTF-8 prompt",
			input: StructuredGenerationInput{Name: base.Name, ModelID: base.ModelID, Prompt: string([]byte{0xff}), Schema: base.Schema, MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputInvalid,
		},
		{
			name:  "oversized prompt",
			input: StructuredGenerationInput{Name: base.Name, ModelID: base.ModelID, Prompt: strings.Repeat("x", structuredPromptLimitBytes+1), Schema: base.Schema, MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputTooLarge,
		},
		{
			name:  "malformed schema",
			input: StructuredGenerationInput{Name: base.Name, ModelID: base.ModelID, Prompt: base.Prompt, Schema: json.RawMessage(`{"type":`), MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputInvalid,
		},
		{
			name:  "schema is not an object",
			input: StructuredGenerationInput{Name: base.Name, ModelID: base.ModelID, Prompt: base.Prompt, Schema: json.RawMessage(`[]`), MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputInvalid,
		},
		{
			name:  "oversized schema",
			input: StructuredGenerationInput{Name: base.Name, ModelID: base.ModelID, Prompt: base.Prompt, Schema: json.RawMessage(`{"description":"` + strings.Repeat("x", structuredSchemaLimitBytes) + `"}`), MaxOutputTokens: base.MaxOutputTokens},
			want:  ErrInputTooLarge,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			requests := 0
			adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return nil, errors.New("structured request must not reach the provider")
			})})
			_, err := adapter.GenerateStructured(t.Context(), ProviderOpenRouter, "librarian-key", testCase.input, nil)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("structured input error = %v, want %v", err, testCase.want)
			}
			if requests != 0 {
				t.Fatalf("unsafe structured input made %d provider requests", requests)
			}
		})
	}
}

func TestStructuredStreamErrorClassifiesMachineReadableStatusWithoutLeakingMessage(t *testing.T) {
	secretMarker := "stream-provider-secret-marker"
	for _, testCase := range []struct {
		name string
		raw  string
		want error
	}{
		{name: "OpenRouter capability code", raw: `{"code":400,"message":"` + secretMarker + `"}`, want: ErrModelCapabilityUnavailable},
		{name: "Gemini capability status", raw: `{"status":"INVALID_ARGUMENT","message":"` + secretMarker + `"}`, want: ErrModelCapabilityUnavailable},
		{name: "missing model", raw: `{"status":"NOT_FOUND","message":"` + secretMarker + `"}`, want: ErrModelUnavailable},
		{name: "provider configuration", raw: `{"status":"FAILED_PRECONDITION","message":"` + secretMarker + `"}`, want: ErrProviderConfiguration},
		{name: "rate limit", raw: `{"code":429,"message":"` + secretMarker + `"}`, want: ErrRateLimited},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := structuredStreamError(json.RawMessage(testCase.raw))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("stream error = %v, want %v", err, testCase.want)
			}
			if strings.Contains(err.Error(), secretMarker) {
				t.Fatal("stream provider message leaked")
			}
		})
	}
}

func TestReadLibrarianSSERejectsOversizedStreams(t *testing.T) {
	t.Run("cumulative parsed output", func(t *testing.T) {
		callbacks := 0
		result, err := readLibrarianSSE(
			strings.NewReader("data: first\n\ndata: second\n\n"),
			func(string) error {
				callbacks++
				return nil
			},
			func([]byte) (string, bool, error) {
				return strings.Repeat("x", maxProviderResponseBytes/2+1), false, nil
			},
		)
		if !errors.Is(err, ErrOutputLimit) || result != "" {
			t.Fatalf("oversized cumulative stream = result:%q error:%v", result, err)
		}
		if callbacks != 1 {
			t.Fatalf("oversized cumulative stream callbacks = %d, want 1", callbacks)
		}
	})

	t.Run("raw SSE payload", func(t *testing.T) {
		payload := strings.Repeat(": keepalive\n", maxProviderResponseBytes/len(": keepalive\n")+1)
		result, err := readLibrarianSSE(strings.NewReader(payload), nil, func([]byte) (string, bool, error) {
			return "", false, nil
		})
		if !errors.Is(err, ErrOutputLimit) || result != "" {
			t.Fatalf("oversized raw stream = result:%q error:%v", result, err)
		}
	})
}

func TestHTTPProviderAdapterLibrarianRejectsInvalidStreamData(t *testing.T) {
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {not-json}\n")),
		}, nil
	})})
	_, err := adapter.GenerateStructured(context.Background(), ProviderOpenRouter, "librarian-key", StructuredGenerationInput{
		Name:            "atlas_note_librarian",
		ModelID:         "openai/gpt-test",
		Prompt:          "bounded librarian prompt",
		Schema:          json.RawMessage(`{"type":"object"}`),
		MaxOutputTokens: summaryOutputTokenLimit,
	}, nil)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("invalid librarian stream error = %v", err)
	}

	for _, testCase := range []struct {
		name      string
		provider  ProviderID
		modelID   string
		response  string
		wantError error
	}{
		{
			name:      "OpenRouter output limit",
			provider:  ProviderOpenRouter,
			modelID:   "openai/gpt-test",
			response:  "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"candidates\\\":[]}\"},\"finish_reason\":\"length\"}]}\n",
			wantError: ErrInvalidResponse,
		},
		{
			name:      "Gemini output limit",
			provider:  ProviderGemini,
			modelID:   "gemini-2.5-flash",
			response:  "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"candidates\\\":[]}\"}]},\"finishReason\":\"MAX_TOKENS\"}]}\n",
			wantError: ErrOutputLimit,
		},
		{
			name:      "unexpected EOF",
			provider:  ProviderOpenRouter,
			modelID:   "openai/gpt-test",
			response:  "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"candidates\\\":[]}\"}}]}\n",
			wantError: ErrInvalidResponse,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(testCase.response)),
				}, nil
			})})
			_, err := adapter.GenerateStructured(context.Background(), testCase.provider, "librarian-key", StructuredGenerationInput{
				Name:            "atlas_note_librarian",
				ModelID:         testCase.modelID,
				Prompt:          "bounded librarian prompt",
				Schema:          json.RawMessage(`{"type":"object"}`),
				MaxOutputTokens: summaryOutputTokenLimit,
			}, nil)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("incomplete librarian stream error = %v", err)
			}
		})
	}
}
