package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestNormalizeWebCitationsAllowsOnlySafeUniqueHTTPSURLs(t *testing.T) {
	values := []WebCitation{
		{URL: "https://example.com/article", Title: " Example "},
		{URL: "https://example.com/article", Title: "duplicate"},
		{URL: "http://example.com/insecure", Title: "insecure"},
		{URL: "javascript:alert(1)", Title: "unsafe"},
		{URL: "https://localhost/private", Title: "local"},
		{URL: "https://foo.localhost/private", Title: "local suffix"},
		{URL: "https://127.0.0.1/private", Title: "loopback"},
		{URL: "https://127.1/private", Title: "short loopback"},
		{URL: "https://2130706433/private", Title: "decimal loopback"},
		{URL: "https://0x7f000001/private", Title: "hex loopback"},
		{URL: "https://[::ffff:127.0.0.1]/private", Title: "mapped loopback"},
		{URL: "https://intranet/private", Title: "single-label host"},
		{URL: "https://user:pass@example.com/private", Title: "credential"},
	}

	got := normalizeWebCitations(values)
	if len(got) != 1 {
		t.Fatalf("citations length = %d, want 1: %#v", len(got), got)
	}
	if got[0].URL != "https://example.com/article" || got[0].Title != "Example" {
		t.Fatalf("citation = %#v", got[0])
	}
}

func TestNormalizeWebCitationsCapsResultCount(t *testing.T) {
	values := make([]WebCitation, 0, aiMaxWebCitations+2)
	for index := 0; index < aiMaxWebCitations+2; index++ {
		values = append(values, WebCitation{URL: "https://example.com/" + string(rune('a'+index))})
	}

	got := normalizeWebCitations(values)
	if len(got) != aiMaxWebCitations {
		t.Fatalf("citations length = %d, want %d", len(got), aiMaxWebCitations)
	}
}

func TestHTTPProviderAdapterAddsBoundedOpenRouterWebSearchAndNormalizesCitations(t *testing.T) {
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode web search payload: %v", err)
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("tools = %#v", payload["tools"])
		}
		tool, ok := tools[0].(map[string]any)
		if !ok || tool["type"] != "openrouter:web_search" {
			t.Fatalf("web search tool = %#v", tool)
		}
		parameters, ok := tool["parameters"].(map[string]any)
		if !ok ||
			parameters["engine"] != "exa" ||
			parameters["max_results"] != float64(3) ||
			parameters["max_total_results"] != float64(3) ||
			parameters["search_context_size"] != "low" {
			t.Fatalf("web search parameters = %#v", parameters)
		}
		if payload["parallel_tool_calls"] != false {
			t.Fatalf("parallel_tool_calls = %#v", payload["parallel_tool_calls"])
		}
		if payload["tool_choice"] != "required" {
			t.Fatalf("tool_choice = %#v", payload["tool_choice"])
		}
		provider, ok := payload["provider"].(map[string]any)
		if !ok || provider["zdr"] != true || provider["data_collection"] != "deny" || provider["allow_fallbacks"] != false {
			t.Fatalf("privacy payload = %#v", provider)
		}
		return jsonResponse(http.StatusOK, `{
			"choices":[{
				"message":{
					"content":"grounded answer",
					"annotations":[
						{"type":"url_citation","url_citation":{"url":"https://example.com/source","title":"Example"}},
						{"type":"url_citation","url_citation":{"url":"javascript:alert(1)","title":"Unsafe"}}
					]
				},
				"finish_reason":"stop"
			}],
			"usage":{"server_tool_use":{"web_search_requests":1}}
		}`), nil
	})})

	result, err := adapter.GenerateText(context.Background(), ProviderOpenRouter, "test-key", TextGenerationInput{
		ModelID:           "openai/test",
		SystemInstruction: "system instruction",
		Messages:          []TextMessage{{Role: "user", Content: "current question"}},
		MaxOutputTokens:   128,
		WebSearch:         true,
	})
	if err != nil {
		t.Fatalf("GenerateText web search: %v", err)
	}
	if result.Text != "grounded answer" ||
		result.WebSearchRequests != 1 ||
		len(result.Citations) != 1 ||
		result.Citations[0].URL != "https://example.com/source" {
		t.Fatalf("web search result = %#v", result)
	}
}

func TestHTTPProviderAdapterRejectsWebSearchResponseWithoutExecutedSearch(t *testing.T) {
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"choices":[{
				"message":{"content":"answer without search","annotations":[]},
				"finish_reason":"stop"
			}],
			"usage":{"server_tool_use":{"web_search_requests":0}}
		}`), nil
	})})

	_, err := adapter.GenerateText(context.Background(), ProviderOpenRouter, "test-key", TextGenerationInput{
		ModelID:           "openai/test",
		SystemInstruction: "system instruction",
		Messages:          []TextMessage{{Role: "user", Content: "current question"}},
		MaxOutputTokens:   128,
		WebSearch:         true,
	})
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidResponse)
	}
}

func TestHTTPProviderAdapterRejectsGeminiWebSearchForPrivacyContract(t *testing.T) {
	adapter := NewHTTPProviderAdapterWithClient(&http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("Gemini web search must be rejected before transport")
		return nil, nil
	})})

	_, err := adapter.GenerateText(context.Background(), ProviderGemini, "test-key", TextGenerationInput{
		ModelID:           "gemini-2.5-flash",
		SystemInstruction: "system instruction",
		Messages:          []TextMessage{{Role: "user", Content: "current question"}},
		MaxOutputTokens:   128,
		WebSearch:         true,
	})
	if !errors.Is(err, ErrModelCapabilityUnavailable) {
		t.Fatalf("error = %v, want %v", err, ErrModelCapabilityUnavailable)
	}
}
