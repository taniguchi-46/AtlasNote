package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHTTPConnectionCheckerUsesFixedHTTPSRequests(t *testing.T) {
	cases := []struct {
		provider ProviderID
		endpoint string
		header   string
	}{
		{ProviderOpenRouter, "https://openrouter.ai/api/v1/key", "Authorization"},
		{ProviderGemini, "https://generativelanguage.googleapis.com/v1/models?pageSize=1", "X-Goog-Api-Key"},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.provider), func(t *testing.T) {
			client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.URL.String() != testCase.endpoint || request.URL.Scheme != "https" {
					t.Fatal("connection check did not use its fixed HTTPS endpoint")
				}
				if request.Body != nil {
					t.Fatal("connection check sent a request body")
				}
				if request.Header.Get(testCase.header) == "" {
					t.Fatal("connection check omitted its provider authentication header")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})}
			checker := NewHTTPConnectionCheckerWithClient(client)
			if err := checker.Check(context.Background(), testCase.provider, "test-api-key"); err != nil {
				t.Fatalf("check fixed endpoint: %v", err)
			}
		})
	}
}

func TestProviderHTTPClientBlocksProxyRedirectAndInsecureTLS(t *testing.T) {
	client := NewProviderHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("provider client did not use an HTTP transport")
	}
	if transport.Proxy != nil {
		t.Fatal("provider client enables a proxy")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("provider client disables TLS verification")
	}
	if client.CheckRedirect == nil {
		t.Fatal("provider client permits redirects")
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, errRedirectBlocked) {
		t.Fatalf("redirect policy error = %v", err)
	}
}

func TestHTTPConnectionCheckerRedactsProviderResponseBodies(t *testing.T) {
	secretMarker := "provider-response-secret-marker"
	checker := NewHTTPConnectionCheckerWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(secretMarker)),
			Header:     make(http.Header),
		}, nil
	})})

	err := checker.Check(context.Background(), ProviderOpenRouter, "test-api-key")
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("authentication error = %v", err)
	}
	if strings.Contains(err.Error(), secretMarker) {
		t.Fatal("provider error body leaked from the connection checker")
	}
}
