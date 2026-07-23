package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPProviderAdapterListsOnlySummaryCapableModels(t *testing.T) {
	t.Run("OpenRouter", func(t *testing.T) {
		adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != openRouterModelsEndpoint {
				t.Fatalf("request = %s %s", request.Method, request.URL)
			}
			if request.Header.Get("Authorization") != "Bearer draft-key" {
				t.Fatal("OpenRouter model request did not use its draft API key")
			}
			assertDeadline(t, request, modelListTimeout)
			return jsonResponse(http.StatusOK, `{
  "data": [
    {"id":"text/model","name":"Text Model","context_length":32000,"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"top_provider":{"max_completion_tokens":512}},
    {"id":"vision/model","architecture":{"input_modalities":["text"],"output_modalities":["image"]}},
    {"id":"text/model","architecture":{"input_modalities":["text"],"output_modalities":["text"]}}
  ]
}`), nil
		})})

		result, err := adapter.ListModels(context.Background(), ProviderOpenRouter, "draft-key")
		if err != nil {
			t.Fatalf("list OpenRouter models: %v", err)
		}
		if len(result.Models) != 1 {
			t.Fatalf("models = %#v", result.Models)
		}
		model := result.Models[0]
		if model.ID != "text/model" || model.DisplayName != "Text Model" || !model.SupportsSummary || !model.Available {
			t.Fatalf("model = %#v", model)
		}
		if model.InputTokenLimit == nil || *model.InputTokenLimit != 32000 || model.OutputTokenLimit == nil || *model.OutputTokenLimit != 512 {
			t.Fatalf("model token limits = %#v", model)
		}
		if result.RetrievedAt.IsZero() {
			t.Fatal("model list omitted retrieval time")
		}
	})

	t.Run("Gemini pagination", func(t *testing.T) {
		calls := 0
		adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "generativelanguage.googleapis.com" {
				t.Fatalf("request = %s %s", request.Method, request.URL)
			}
			if request.Header.Get("X-Goog-Api-Key") != "draft-key" {
				t.Fatal("Gemini model request did not use its draft API key")
			}
			assertDeadline(t, request, modelListTimeout)
			switch calls {
			case 1:
				if request.URL.String() != geminiModelsEndpoint+"?pageSize=100" {
					t.Fatalf("first page URL = %s", request.URL)
				}
				return jsonResponse(http.StatusOK, `{
  "models": [
    {"name":"models/gemini-2.5-flash","displayName":"Gemini Flash","inputTokenLimit":1048576,"outputTokenLimit":8192,"supportedGenerationMethods":["generateContent"]},
    {"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]}
  ],
  "nextPageToken":"next token"
}`), nil
			case 2:
				if request.URL.Query().Get("pageToken") != "next token" {
					t.Fatalf("second page URL = %s", request.URL)
				}
				return jsonResponse(http.StatusOK, `{
  "models": [
    {"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]},
    {"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]}
  ]
}`), nil
			default:
				t.Fatalf("unexpected model-list request %d", calls)
				return nil, nil
			}
		})})

		result, err := adapter.ListModels(context.Background(), ProviderGemini, "draft-key")
		if err != nil {
			t.Fatalf("list Gemini models: %v", err)
		}
		if calls != 2 || len(result.Models) != 2 {
			t.Fatalf("calls = %d, models = %#v", calls, result.Models)
		}
		if result.Models[0].ID != "gemini-2.5-flash" || result.Models[1].ID != "gemini-2.5-pro" {
			t.Fatalf("model IDs = %#v", result.Models)
		}
	})
}

func TestHTTPProviderAdapterGeneratesBoundedPrivateSummary(t *testing.T) {
	testCases := []struct {
		name     string
		provider ProviderID
		modelID  string
		endpoint string
		response string
		wantText string
		validate func(*testing.T, map[string]any)
	}{
		{
			name:     "OpenRouter",
			provider: ProviderOpenRouter,
			modelID:  "openai/gpt-test",
			endpoint: openRouterSummaryEndpoint,
			response: `{"choices":[{"message":{"content":"summary result"},"finish_reason":"stop"}]}`,
			wantText: "summary result",
			validate: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["model"] != "openai/gpt-test" || payload["stream"] != false || payload["max_tokens"] != float64(summaryOutputTokenLimit) {
					t.Fatalf("OpenRouter payload = %#v", payload)
				}
				provider, ok := payload["provider"].(map[string]any)
				if !ok || provider["zdr"] != true || provider["data_collection"] != "deny" || provider["allow_fallbacks"] != false {
					t.Fatalf("OpenRouter privacy settings = %#v", provider)
				}
				assertSummaryMessages(t, payload)
			},
		},
		{
			name:     "Gemini",
			provider: ProviderGemini,
			modelID:  "gemini-2.5-flash",
			endpoint: geminiSummaryEndpoint + "gemini-2.5-flash:generateContent",
			response: `{"candidates":[{"content":{"parts":[{"text":"summary"},{"text":" result"}]},"finishReason":"STOP"}]}`,
			wantText: "summary result",
			validate: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["store"] != false {
					t.Fatalf("Gemini request must set store=false: %#v", payload)
				}
				config, ok := payload["generationConfig"].(map[string]any)
				if !ok || config["maxOutputTokens"] != float64(summaryOutputTokenLimit) {
					t.Fatalf("Gemini generation config = %#v", config)
				}
				instruction, ok := payload["systemInstruction"].(map[string]any)
				if !ok || !strings.Contains(instructionText(instruction), "要約") {
					t.Fatalf("Gemini system instruction = %#v", instruction)
				}
				contents, ok := payload["contents"].([]any)
				if !ok || len(contents) != 1 {
					t.Fatalf("Gemini contents = %#v", payload["contents"])
				}
				content, ok := contents[0].(map[string]any)
				if !ok || content["role"] != "user" || !strings.Contains(instructionText(content), "note-body-marker") {
					t.Fatalf("Gemini content = %#v", content)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.String() != testCase.endpoint {
					t.Fatalf("request = %s %s", request.Method, request.URL)
				}
				if request.Header.Get("Content-Type") != "application/json" {
					t.Fatal("summary request omitted JSON content type")
				}
				if testCase.provider == ProviderOpenRouter && request.Header.Get("Authorization") != "Bearer stored-key" {
					t.Fatal("OpenRouter summary request omitted authorization")
				}
				if testCase.provider == ProviderGemini && request.Header.Get("X-Goog-Api-Key") != "stored-key" {
					t.Fatal("Gemini summary request omitted API key header")
				}
				assertDeadline(t, request, summaryGenerationTimeout)

				payload := make(map[string]any)
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Fatalf("decode summary request: %v", err)
				}
				for _, forbidden := range []string{"tools", "response_format", "stream_options"} {
					if _, found := payload[forbidden]; found {
						t.Fatalf("summary request unexpectedly included %q", forbidden)
					}
				}
				testCase.validate(t, payload)
				return jsonResponse(http.StatusOK, testCase.response), nil
			})})

			result, err := adapter.GenerateSummary(context.Background(), testCase.provider, "stored-key", GenerateSummaryInput{
				ProviderID: testCase.provider,
				ModelID:    testCase.modelID,
				Content:    "note-body-marker",
			})
			if err != nil {
				t.Fatalf("generate summary: %v", err)
			}
			if result.Text != testCase.wantText {
				t.Fatalf("summary = %#v", result)
			}
		})
	}
}

func TestHTTPProviderAdapterRejectsUnsafeInputAndRedactsProviderErrors(t *testing.T) {
	called := false
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected request")
	})})
	_, err := adapter.GenerateSummary(context.Background(), ProviderOpenRouter, "stored-key", GenerateSummaryInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/gpt-test",
		Content:    strings.Repeat("a", summaryInputLimitBytes+1),
	})
	if !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized input error = %v", err)
	}
	if called {
		t.Fatal("oversized input reached a provider endpoint")
	}
	_, err = adapter.GenerateSummary(context.Background(), ProviderOpenRouter, "stored-key", GenerateSummaryInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openrouter/auto",
		Content:    "safe input",
	})
	if !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("automatic model error = %v", err)
	}
	if called {
		t.Fatal("automatic model selection reached a provider endpoint")
	}

	secretMarker := "provider-response-secret-marker"
	rateLimitCalls := 0
	adapter = NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		rateLimitCalls++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"37"}},
			Body:       io.NopCloser(strings.NewReader(secretMarker)),
		}, nil
	})})
	_, err = adapter.GenerateSummary(context.Background(), ProviderOpenRouter, "stored-key", GenerateSummaryInput{
		ProviderID: ProviderOpenRouter,
		ModelID:    "openai/gpt-test",
		Content:    "safe input",
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit error = %v", err)
	}
	if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("provider error body leaked from summary generation")
	}
	safeError := SafeErrorFrom(err)
	if safeError.RetryAfterSeconds == nil || *safeError.RetryAfterSeconds != 37 {
		t.Fatalf("retry-after = %#v", safeError)
	}
	if rateLimitCalls != 1 {
		t.Fatalf("rate-limited request count = %d, want no automatic retry", rateLimitCalls)
	}
}

func TestHTTPProviderAdapterRejectsIncompleteSummaryResponses(t *testing.T) {
	testCases := []struct {
		name     string
		provider ProviderID
		modelID  string
		response string
	}{
		{
			name:     "OpenRouter output limit",
			provider: ProviderOpenRouter,
			modelID:  "openai/gpt-test",
			response: `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`,
		},
		{
			name:     "Gemini incomplete candidate",
			provider: ProviderGemini,
			modelID:  "gemini-2.5-flash",
			response: `{"candidates":[{"content":{"parts":[{"text":"partial"}]},"finishReason":"MAX_TOKENS"}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calls := 0
			adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(http.StatusOK, testCase.response), nil
			})})
			_, err := adapter.GenerateSummary(context.Background(), testCase.provider, "stored-key", GenerateSummaryInput{
				ProviderID: testCase.provider,
				ModelID:    testCase.modelID,
				Content:    "safe input",
			})
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("incomplete summary error = %v", err)
			}
			if calls != 1 {
				t.Fatalf("incomplete summary request count = %d, want no automatic retry", calls)
			}
		})
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertDeadline(t *testing.T, request *http.Request, timeout time.Duration) {
	t.Helper()
	deadline, ok := request.Context().Deadline()
	if !ok {
		t.Fatal("provider request omitted a deadline")
	}
	remaining := time.Until(deadline)
	if remaining > timeout || remaining < timeout-2*time.Second {
		t.Fatalf("request deadline remaining = %s, want close to %s", remaining, timeout)
	}
}

func assertSummaryMessages(t *testing.T, payload map[string]any) {
	t.Helper()
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("OpenRouter messages = %#v", payload["messages"])
	}
	system, ok := messages[0].(map[string]any)
	if !ok || system["role"] != "system" || !strings.Contains(system["content"].(string), "要約") {
		t.Fatalf("OpenRouter system message = %#v", system)
	}
	user, ok := messages[1].(map[string]any)
	if !ok || user["role"] != "user" || user["content"] != "note-body-marker" {
		t.Fatalf("OpenRouter user message = %#v", user)
	}
}

func instructionText(value map[string]any) string {
	parts, _ := value["parts"].([]any)
	var text strings.Builder
	for _, rawPart := range parts {
		part, _ := rawPart.(map[string]any)
		partText, _ := part["text"].(string)
		text.WriteString(partText)
	}
	return text.String()
}
