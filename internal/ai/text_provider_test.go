package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPProviderAdapterGeneratesPrivateTextWithoutFallback(t *testing.T) {
	t.Run("OpenRouter", func(t *testing.T) {
		adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != openRouterSummaryEndpoint {
				t.Fatalf("text endpoint = %s", request.URL)
			}
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode text payload: %v", err)
			}
			provider, ok := payload["provider"].(map[string]any)
			if !ok || provider["zdr"] != true || provider["data_collection"] != "deny" || provider["allow_fallbacks"] != false {
				t.Fatalf("privacy payload = %#v", provider)
			}
			if payload["stream"] != false || payload["max_tokens"] != float64(128) {
				t.Fatalf("generation payload = %#v", payload)
			}
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"generated text"},"finish_reason":"stop"}]}`), nil
		})})

		result, err := adapter.GenerateText(context.Background(), ProviderOpenRouter, "test-key", TextGenerationInput{
			ModelID:           "openai/test",
			SystemInstruction: "system instruction",
			Messages:          []TextMessage{{Role: "user", Content: "question"}},
			MaxOutputTokens:   128,
		})
		if err != nil || result.Text != "generated text" {
			t.Fatalf("text result = %#v, err=%v", result, err)
		}
	})

	t.Run("Gemini", func(t *testing.T) {
		adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode Gemini text payload: %v", err)
			}
			if payload["store"] != false {
				t.Fatalf("Gemini store flag = %#v", payload["store"])
			}
			contents, ok := payload["contents"].([]any)
			if !ok || len(contents) != 2 || contents[1].(map[string]any)["role"] != "model" {
				t.Fatalf("Gemini conversation = %#v", payload["contents"])
			}
			return jsonResponse(http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"answer"}]},"finishReason":"STOP"}]}`), nil
		})})

		result, err := adapter.GenerateText(context.Background(), ProviderGemini, "test-key", TextGenerationInput{
			ModelID:           "gemini-2.5-flash",
			SystemInstruction: "system instruction",
			Messages: []TextMessage{
				{Role: "user", Content: "question"},
				{Role: "assistant", Content: "previous"},
			},
			MaxOutputTokens: 128,
		})
		if err != nil || !strings.Contains(result.Text, "answer") {
			t.Fatalf("Gemini text result = %#v, err=%v", result, err)
		}
	})
}
