package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

const connectionCheckTimeout = 10 * time.Second

var errRedirectBlocked = errors.New("AI provider redirect blocked")

type ConnectionChecker interface {
	Check(ctx context.Context, providerID ProviderID, apiKey string) error
}

type HTTPConnectionChecker struct {
	client *http.Client
}

func NewProviderHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = nil
	return &http.Client{
		Transport: transport,
		Timeout:   connectionCheckTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirectBlocked
		},
	}
}

func NewHTTPConnectionChecker() *HTTPConnectionChecker {
	return NewHTTPConnectionCheckerWithClient(NewProviderHTTPClient())
}

// NewHTTPConnectionCheckerWithClient is for transport injection in tests. The
// application always uses NewHTTPConnectionChecker and never accepts a user URL.
func NewHTTPConnectionCheckerWithClient(client *http.Client) *HTTPConnectionChecker {
	if client == nil {
		client = NewProviderHTTPClient()
	}
	return &HTTPConnectionChecker{client: client}
}

func (c *HTTPConnectionChecker) Check(ctx context.Context, providerID ProviderID, apiKey string) error {
	provider, err := normalizeProviderID(providerID)
	if err != nil {
		return err
	}
	if err := validateAPIKey(apiKey); err != nil {
		return err
	}

	endpoint := ""
	switch provider {
	case ProviderOpenRouter:
		endpoint = "https://openrouter.ai/api/v1/key"
	case ProviderGemini:
		endpoint = "https://generativelanguage.googleapis.com/v1/models?pageSize=1"
	default:
		return ErrProviderUnsupported
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrProviderUnavailable
	}
	if provider == ProviderOpenRouter {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		request.Header.Set("x-goog-api-key", apiKey)
	}

	response, err := c.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return ErrNetworkUnavailable
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	switch {
	case response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices:
		return nil
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		return ErrAuthFailed
	case response.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return ErrProviderUnavailable
	}
}
