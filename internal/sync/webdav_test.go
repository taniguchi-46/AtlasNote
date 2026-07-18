package sync

import (
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPClientPropfindUsesConfiguredWebDAVRoot(t *testing.T) {
	t.Parallel()
	var requestPath, depth, requestBody string
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		requestPath = request.URL.Path
		depth = request.Header.Get("Depth")
		body, _ := io.ReadAll(request.Body)
		requestBody = string(body)
		if requestPath != "/dav/atlasnote/" || depth != "0" ||
			!strings.HasPrefix(request.Header.Get("Content-Type"), "text/xml") || !strings.Contains(requestBody, `<?xml version="1.0"`) ||
			!strings.Contains(requestBody, `<d:propfind xmlns:d="DAV:">`) ||
			request.Header.Get("Cache-Control") != "no-store" || request.Header.Get("User-Agent") != defaultWebDAVUserAgent {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		username, password, authenticated := request.BasicAuth()
		if !authenticated {
			writer.Header().Set("WWW-Authenticate", `Basic realm="atlasnote"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if username != "alice" || password != "secret" {
			writer.Header().Set("WWW-Authenticate", `Basic realm="atlasnote"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL+"/dav", "/atlasnote", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	if _, err := client.Propfind(context.Background(), "", "0"); err != nil {
		t.Fatalf("PROPFIND root: %v", err)
	}
	if requestPath != "/dav/atlasnote/" || depth != "0" {
		t.Fatalf("PROPFIND target = %q depth=%q", requestPath, depth)
	}
	if requestCount != 2 {
		t.Fatalf("Basic authentication request count = %d, want 2", requestCount)
	}
}

func TestHTTPClientPropfindUsesDigestAuthentication(t *testing.T) {
	t.Parallel()
	requestCount := 0
	authenticatedRequests := 0
	failure := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		authorization := request.Header.Get("Authorization")
		if authorization == "" {
			writer.Header().Set("WWW-Authenticate", `Digest realm="atlasnote", nonce="nonce-1", algorithm=MD5, qop="auth"`)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(authorization, "Digest ") {
			failure = "Digest retry used a non-Digest Authorization header"
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		authenticatedRequests++
		parameters := parseAuthenticationParameters(strings.TrimPrefix(authorization, "Digest "))
		expectedNonceCount := "00000001"
		if authenticatedRequests == 2 {
			expectedNonceCount = "00000002"
		}
		if parameters["username"] != "alice" || parameters["realm"] != "atlasnote" || parameters["nonce"] != "nonce-1" ||
			parameters["uri"] != "/dav/atlasnote/" || parameters["algorithm"] != "MD5" || parameters["qop"] != "auth" ||
			parameters["nc"] != expectedNonceCount || parameters["cnonce"] == "" {
			failure = "Digest Authorization header was incomplete"
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		expectedResponse := md5Hex(
			md5Hex("alice:atlasnote:secret") + ":nonce-1:" + parameters["nc"] + ":" + parameters["cnonce"] + ":auth:" +
				md5Hex(request.Method+":"+parameters["uri"]),
		)
		if parameters["response"] != expectedResponse {
			failure = "Digest Authorization response hash was invalid"
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL+"/dav", "/atlasnote", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	if _, err := client.Propfind(context.Background(), "", "0"); err != nil {
		t.Fatalf("first Digest PROPFIND: %v", err)
	}
	if _, err := client.Propfind(context.Background(), "", "0"); err != nil {
		t.Fatalf("cached Digest PROPFIND: %v", err)
	}
	if failure != "" {
		t.Fatal(failure)
	}
	if requestCount != 3 {
		t.Fatalf("Digest authentication request count = %d, want 3", requestCount)
	}
}

func TestHTTPClientReportsSafeAuthenticationDiagnostics(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("WWW-Authenticate", `Digest realm="private", nonce="nonce", algorithm=MD5, qop="auth"`)
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, "/", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	_, err = client.Propfind(context.Background(), "", "0")
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("PROPFIND error = %v", err)
	}
	if !statusErr.DigestCredentialsPresent || statusErr.BasicCredentialsPresent || statusErr.AuthScheme != "Digest" {
		t.Fatalf("authentication diagnostics = %#v", statusErr)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
		t.Fatalf("authentication diagnostics leaked a secret or realm: %v", err)
	}
}

func TestHTTPClientUsesStrongETagFromPropfindWhenGetOmitsHeader(t *testing.T) {
	t.Parallel()
	requestMethods := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestMethods = append(requestMethods, request.Method)
		switch request.Method {
		case http.MethodGet:
			if request.URL.Path != "/.atlasnote/head.json" {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = writer.Write([]byte(`{"formatVersion":1}`))
		case "PROPFIND":
			if request.Header.Get("Depth") != "0" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), "<d:getetag/>") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Content-Type", "application/xml; charset=utf-8")
			writer.WriteHeader(http.StatusMultiStatus)
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:propstat>
      <d:prop><d:getetag>"head-strong"</d:getetag></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`))
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, "/", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	response, err := client.GetWithStrongETag(context.Background(), headPath)
	if err != nil {
		t.Fatalf("read head with PROPFIND ETag fallback: %v", err)
	}
	if response.ETag != `"head-strong"` {
		t.Fatalf("fallback ETag = %q", response.ETag)
	}
	if got := strings.Join(requestMethods, ","); got != "GET,PROPFIND" {
		t.Fatalf("request methods = %s", got)
	}
}

func TestHTTPClientDoesNotUseWeakETagFromPropfind(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			_, _ = writer.Write([]byte(`{"formatVersion":1}`))
		case "PROPFIND":
			writer.WriteHeader(http.StatusMultiStatus)
			_, _ = writer.Write([]byte(`<?xml version="1.0"?>
<d:multistatus xmlns:d="DAV:">
  <d:response><d:propstat><d:prop><d:getetag>W/"weak-head"</d:getetag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`))
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, "/", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	response, err := client.GetWithStrongETag(context.Background(), headPath)
	if err != nil {
		t.Fatalf("read head with weak ETag: %v", err)
	}
	if response.ETag != "" || !errors.Is(requireStrongETag(response.ETag), ErrMissingStrongETag) {
		t.Fatalf("weak PROPFIND ETag was accepted: %q", response.ETag)
	}
}

func TestHTTPClientTLSOptionsAreExplicit(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusMultiStatus)
	}))
	defer server.Close()

	defaultClient, err := NewHTTPClientWithConfig(HTTPClientConfig{
		Endpoint: server.URL, RemoteRoot: "/", Username: "alice", Password: "secret",
		ProxyTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("create default TLS client: %v", err)
	}
	if _, err := defaultClient.Propfind(context.Background(), "", "0"); err == nil {
		t.Fatal("untrusted TLS certificate was accepted without an explicit setting")
	}

	insecureClient, err := NewHTTPClientWithConfig(HTTPClientConfig{
		Endpoint: server.URL, RemoteRoot: "/", Username: "alice", Password: "secret",
		IgnoreTLSErrors: true, ProxyTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("create explicit TLS bypass client: %v", err)
	}
	if _, err := insecureClient.Propfind(context.Background(), "", "0"); err != nil {
		t.Fatalf("explicit TLS bypass: %v", err)
	}

	certificatePath := filepath.Join(t.TempDir(), "server.pem")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(certificatePath, certificatePEM, 0o600); err != nil {
		t.Fatalf("write test certificate: %v", err)
	}
	customCAClient, err := NewHTTPClientWithConfig(HTTPClientConfig{
		Endpoint: server.URL, RemoteRoot: "/", Username: "alice", Password: "secret",
		CustomTLSCertificates: certificatePath, ProxyTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("create custom CA client: %v", err)
	}
	if _, err := customCAClient.Propfind(context.Background(), "", "0"); err != nil {
		t.Fatalf("custom CA PROPFIND: %v", err)
	}
}

func TestHTTPClientRejectsUnsafeProxyAndRedirect(t *testing.T) {
	t.Parallel()
	for _, proxyURL := range []string{
		"socks5://proxy.example.test:1080",
		"http://alice:secret@proxy.example.test",
		"http://proxy.example.test/path",
	} {
		_, err := NewHTTPClientWithConfig(HTTPClientConfig{
			Endpoint: "https://dav.example.test", RemoteRoot: "/", Username: "alice", Password: "secret",
			ProxyEnabled: true, ProxyURL: proxyURL, ProxyTimeoutSeconds: 1,
		})
		if !errors.Is(err, ErrInvalidProxySettings) {
			t.Fatalf("proxy %q error = %v", proxyURL, err)
		}
	}

	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusMultiStatus)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	client, err := NewHTTPClient(redirect.URL, "/", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create redirect client: %v", err)
	}
	if _, err := client.Propfind(context.Background(), "", "0"); !errors.Is(err, ErrRedirectRejected) {
		t.Fatalf("redirect error = %v", err)
	}
}

func TestJoinWebDAVURLPreservesExistingEndpointPath(t *testing.T) {
	t.Parallel()
	if got := JoinWebDAVURL("https://dav.example.test/base/", "/atlasnote"); got != "https://dav.example.test/base/atlasnote" {
		t.Fatalf("joined WebDAV URL = %q", got)
	}
}

func TestHTTPClientMkcolDoesNotIgnoreMissingParentConflict(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "/", "alice", "secret", true)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	err = client.Mkcol(context.Background(), ".atlasnote/objects")
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusConflict {
		t.Fatalf("MKCOL missing-parent conflict = %v", err)
	}
}
