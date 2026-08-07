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
