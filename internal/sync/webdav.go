package sync

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 -- Digest/MD5 is required by compatible WebDAV servers.
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	stdsync "sync"
	"time"
)

const (
	requestTimeout         = 20 * time.Second
	defaultWebDAVUserAgent = "AtlasNote/1.0"
	authSchemeBasic        = "basic"
	authSchemeDigest       = "digest"
	propfindRequestBody    = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:getetag/>
    <d:getlastmodified/>
  </d:prop>
</d:propfind>`
)

type RemoteResponse struct {
	StatusCode int
	ETag       string
	Body       []byte
}

type RemoteClient interface {
	Get(context.Context, string) (RemoteResponse, error)
	Put(context.Context, string, []byte, string, string) (RemoteResponse, error)
	Mkcol(context.Context, string) error
	Propfind(context.Context, string, string) (RemoteResponse, error)
}

// strongETagReader is an optional extension for clients that can obtain an
// ETag from WebDAV properties when a successful GET omits the ETag header.
// The sync protocol still accepts only a strong, quoted ETag.
type strongETagReader interface {
	GetWithStrongETag(context.Context, string) (RemoteResponse, error)
}

func getResponseWithStrongETag(ctx context.Context, client RemoteClient, remotePath string) (RemoteResponse, error) {
	if reader, ok := client.(strongETagReader); ok {
		return reader.GetWithStrongETag(ctx, remotePath)
	}
	return client.Get(ctx, remotePath)
}

type HTTPStatusError struct {
	StatusCode               int
	ETag                     string
	Method                   string
	AuthScheme               string
	BasicCredentialsPresent  bool
	DigestCredentialsPresent bool
}

func (e *HTTPStatusError) Error() string {
	message := "webdav request"
	if e.Method != "" {
		message = fmt.Sprintf("webdav %s request", e.Method)
	}
	message = fmt.Sprintf("%s returned HTTP %d", message, e.StatusCode)
	if e.StatusCode != http.StatusUnauthorized {
		return message
	}
	details := make([]string, 0, 2)
	if e.BasicCredentialsPresent {
		details = append(details, "Basic credentials were sent")
	}
	if e.DigestCredentialsPresent {
		details = append(details, "Digest credentials were sent")
	}
	if e.AuthScheme != "" {
		details = append(details, fmt.Sprintf("server requested %s authentication", e.AuthScheme))
	}
	if len(details) == 0 {
		return message
	}
	return fmt.Sprintf("%s (%s)", message, strings.Join(details, "; "))
}

func (e *HTTPStatusError) Is(target error) bool {
	other, ok := target.(*HTTPStatusError)
	return ok && e.StatusCode == other.StatusCode
}

var (
	ErrMissingStrongETag      = errors.New("webdav response did not include a strong ETag")
	ErrRedirectRejected       = errors.New("webdav redirect was rejected")
	ErrInvalidTLSCertificates = errors.New("custom TLS certificates could not be loaded")
)

type HTTPClientConfig struct {
	Endpoint              string
	RemoteRoot            string
	Username              string
	Password              string
	AllowInsecureHTTP     bool
	CustomTLSCertificates string
	IgnoreTLSErrors       bool
	ProxyEnabled          bool
	ProxyURL              string
	ProxyTimeoutSeconds   int
}

type HTTPClient struct {
	baseURL           *url.URL
	username          string
	password          string
	client            *http.Client
	authMu            stdsync.Mutex
	authScheme        string
	digestChallenge   *digestChallenge
	digestNonceCounts map[string]uint32
}

type digestChallenge struct {
	realm     string
	nonce     string
	opaque    string
	algorithm string
	qop       string
}

func NewHTTPClient(endpoint string, remoteRoot string, username string, password string, allowInsecureHTTP bool) (*HTTPClient, error) {
	return NewHTTPClientWithConfig(HTTPClientConfig{
		Endpoint: endpoint, RemoteRoot: remoteRoot, Username: username,
		Password: password, AllowInsecureHTTP: allowInsecureHTTP,
		ProxyTimeoutSeconds: DefaultProxyTimeoutSeconds,
	})
}

func NewHTTPClientWithConfig(config HTTPClientConfig) (*HTTPClient, error) {
	validatedEndpoint, err := ValidateEndpoint(config.Endpoint, config.AllowInsecureHTTP)
	if err != nil {
		return nil, err
	}
	validatedRoot, err := NormalizeRemoteRoot(config.RemoteRoot)
	if err != nil {
		return nil, err
	}
	baseURL, err := url.Parse(validatedEndpoint)
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	baseURL.Path = joinURLPath(baseURL.Path, validatedRoot)
	transport, err := buildTransport(config)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{
		baseURL:  baseURL,
		username: config.Username,
		password: config.Password,
		client: &http.Client{
			Timeout:   requestTimeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return ErrRedirectRejected
			},
		},
		digestNonceCounts: make(map[string]uint32),
	}, nil
}

func buildTransport(config HTTPClientConfig) (*http.Transport, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport is unavailable")
	}
	clone := transport.Clone()
	timeout := config.ProxyTimeoutSeconds
	if timeout == 0 {
		timeout = DefaultProxyTimeoutSeconds
	}
	if timeout < 1 || timeout > 60 {
		return nil, ErrInvalidProxySettings
	}
	if config.ProxyEnabled {
		proxyURL, err := validateProxyURL(config.ProxyURL)
		if err != nil {
			return nil, err
		}
		clone.Proxy = http.ProxyURL(proxyURL)
		clone.DialContext = (&net.Dialer{Timeout: time.Duration(timeout) * time.Second, KeepAlive: 30 * time.Second}).DialContext
	} else {
		clone.Proxy = nil
	}

	rootCAs, err := loadCustomRootCAs(config.CustomTLSCertificates)
	if err != nil {
		return nil, err
	}
	clone.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		// This is only reachable through the explicit advanced setting and is
		// intentionally never inferred from HTTP or certificate failures.
		InsecureSkipVerify: config.IgnoreTLSErrors, // #nosec G402 -- explicit user opt-in
	}
	return clone, nil
}

func validateProxyURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil {
		return nil, ErrInvalidProxySettings
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidProxySettings
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, ErrInvalidProxySettings
	}
	return parsed, nil
}

func loadCustomRootCAs(value string) (*x509.CertPool, error) {
	entries := strings.Split(strings.TrimSpace(value), ",")
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	for _, certificatePath := range paths {
		info, err := os.Stat(certificatePath)
		if err != nil {
			return nil, ErrInvalidTLSCertificates
		}
		files := []string{certificatePath}
		if info.IsDir() {
			entries, err := os.ReadDir(certificatePath)
			if err != nil {
				return nil, ErrInvalidTLSCertificates
			}
			files = files[:0]
			for _, entry := range entries {
				if entry.Type().IsRegular() {
					files = append(files, filepath.Join(certificatePath, entry.Name()))
				}
			}
			sort.Strings(files)
		}
		for _, file := range files {
			pemData, err := os.ReadFile(file)
			if err != nil || !pool.AppendCertsFromPEM(pemData) {
				return nil, ErrInvalidTLSCertificates
			}
		}
	}
	return pool, nil
}

func (c *HTTPClient) Get(ctx context.Context, remotePath string) (RemoteResponse, error) {
	return c.do(ctx, http.MethodGet, remotePath, nil, "", "")
}

// GetWithStrongETag reads a resource normally first. Some WebDAV servers omit
// the ETag response header on GET while exposing the same value through the
// DAV:getetag property. In that case, a depth-zero PROPFIND provides a safe
// compatibility path without weakening the conditional head update contract.
func (c *HTTPClient) GetWithStrongETag(ctx context.Context, remotePath string) (RemoteResponse, error) {
	response, err := c.Get(ctx, remotePath)
	if err != nil || requireStrongETag(response.ETag) == nil {
		return response, err
	}
	properties, propertyErr := c.Propfind(ctx, remotePath, "0")
	if propertyErr != nil {
		// Preserve the successful GET result. The caller will keep rejecting a
		// missing/weak ETag rather than proceeding with an unsafe update.
		return response, nil
	}
	if etag := strongETagFromPropfind(properties.Body); etag != "" {
		response.ETag = etag
	}
	return response, nil
}

func (c *HTTPClient) Put(ctx context.Context, remotePath string, body []byte, ifMatch string, ifNoneMatch string) (RemoteResponse, error) {
	return c.do(ctx, http.MethodPut, remotePath, body, ifMatch, ifNoneMatch)
}

func (c *HTTPClient) Mkcol(ctx context.Context, remotePath string) error {
	response, err := c.do(ctx, "MKCOL", remotePath, nil, "", "")
	if err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusMethodNotAllowed {
			return nil
		}
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPStatusError{StatusCode: response.StatusCode, ETag: response.ETag}
	}
	return nil
}

func (c *HTTPClient) Propfind(ctx context.Context, remotePath string, depth string) (RemoteResponse, error) {
	request, err := c.newRequest(ctx, "PROPFIND", remotePath, []byte(propfindRequestBody))
	if err != nil {
		return RemoteResponse{}, err
	}
	request.Header.Set("Depth", depth)
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	return c.send(request)
}

type propfindMultiStatus struct {
	Responses []propfindResponse `xml:"response"`
}

type propfindResponse struct {
	PropStats []propfindPropStat `xml:"propstat"`
}

type propfindPropStat struct {
	Prop   propfindProperties `xml:"prop"`
	Status string             `xml:"status"`
}

type propfindProperties struct {
	ETag string `xml:"getetag"`
}

func strongETagFromPropfind(body []byte) string {
	var result propfindMultiStatus
	if err := xml.Unmarshal(body, &result); err != nil {
		return ""
	}
	for _, response := range result.Responses {
		for _, propStat := range response.PropStats {
			if !isSuccessfulPropfindStatus(propStat.Status) {
				continue
			}
			etag := strings.TrimSpace(propStat.Prop.ETag)
			if requireStrongETag(etag) == nil {
				return etag
			}
		}
	}
	return ""
}

func isSuccessfulPropfindStatus(value string) bool {
	for _, field := range strings.Fields(value) {
		statusCode, err := strconv.Atoi(field)
		if err == nil {
			return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
		}
	}
	return false
}

func (c *HTTPClient) do(ctx context.Context, method string, remotePath string, body []byte, ifMatch string, ifNoneMatch string) (RemoteResponse, error) {
	request, err := c.newRequest(ctx, method, remotePath, body)
	if err != nil {
		return RemoteResponse{}, err
	}
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	if method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	return c.send(request)
}

func (c *HTTPClient) newRequest(ctx context.Context, method string, remotePath string, body []byte) (*http.Request, error) {
	cleanPath, err := remoteRelativePath(remotePath)
	if err != nil {
		return nil, err
	}
	requestURL := *c.baseURL
	requestURL.Path = joinRequestURLPath(c.baseURL.Path, cleanPath)
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create webdav request: %w", err)
	}
	request.Header.Set("Cache-Control", "no-store")
	request.Header.Set("User-Agent", defaultWebDAVUserAgent)
	if err := c.applyCachedAuthentication(request); err != nil {
		return nil, fmt.Errorf("prepare webdav authentication: %w", err)
	}
	return request, nil
}

func (c *HTTPClient) send(request *http.Request) (RemoteResponse, error) {
	response, result, err := c.execute(request)
	if err != nil {
		return RemoteResponse{}, err
	}
	finalRequest := request
	challenges := response.Header.Values("WWW-Authenticate")
	if response.StatusCode == http.StatusUnauthorized && c.username != "" && c.password != "" {
		retry, shouldRetry, retryErr := c.authenticatedRetry(request, challenges)
		if retryErr != nil {
			return result, fmt.Errorf("prepare webdav authentication retry: %w", retryErr)
		}
		if shouldRetry {
			response, result, err = c.execute(retry)
			if err != nil {
				return RemoteResponse{}, err
			}
			finalRequest = retry
			challenges = response.Header.Values("WWW-Authenticate")
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, newHTTPStatusError(response, result, finalRequest, challenges)
	}
	return result, nil
}

func (c *HTTPClient) execute(request *http.Request) (*http.Response, RemoteResponse, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, RemoteResponse{}, fmt.Errorf("webdav request: %w", err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024))
	if readErr != nil {
		return nil, RemoteResponse{}, fmt.Errorf("read webdav response: %w", readErr)
	}
	result := RemoteResponse{
		StatusCode: response.StatusCode,
		ETag:       response.Header.Get("ETag"),
		Body:       body,
	}
	return response, result, nil
}

func (c *HTTPClient) applyCachedAuthentication(request *http.Request) error {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	switch c.authScheme {
	case authSchemeBasic:
		request.SetBasicAuth(c.username, c.password)
	case authSchemeDigest:
		if c.digestChallenge != nil {
			return c.applyDigestAuthenticationLocked(request, *c.digestChallenge)
		}
	}
	return nil
}

func (c *HTTPClient) authenticatedRetry(request *http.Request, challenges []string) (*http.Request, bool, error) {
	if challenge, ok := digestChallengeFromHeaders(challenges); ok {
		retry, err := cloneRequestForAuthenticationRetry(request)
		if err != nil {
			return nil, false, err
		}
		c.authMu.Lock()
		c.authScheme = authSchemeDigest
		c.digestChallenge = &challenge
		err = c.applyDigestAuthenticationLocked(retry, challenge)
		c.authMu.Unlock()
		if err != nil {
			return nil, false, err
		}
		return retry, true, nil
	}
	if !hasAuthenticationScheme(challenges, "Basic") {
		return nil, false, nil
	}
	retry, err := cloneRequestForAuthenticationRetry(request)
	if err != nil {
		return nil, false, err
	}
	retry.SetBasicAuth(c.username, c.password)
	c.authMu.Lock()
	c.authScheme = authSchemeBasic
	c.digestChallenge = nil
	c.authMu.Unlock()
	return retry, true, nil
}

func cloneRequestForAuthenticationRetry(request *http.Request) (*http.Request, error) {
	retry := request.Clone(request.Context())
	retry.Header = request.Header.Clone()
	retry.Header.Del("Authorization")
	if request.GetBody == nil {
		if request.Body == nil || request.Body == http.NoBody {
			retry.Body = http.NoBody
			return retry, nil
		}
		return nil, errors.New("webdav request body cannot be replayed for authentication")
	}
	body, err := request.GetBody()
	if err != nil {
		return nil, fmt.Errorf("reopen webdav request body: %w", err)
	}
	retry.Body = body
	return retry, nil
}

func (c *HTTPClient) applyDigestAuthenticationLocked(request *http.Request, challenge digestChallenge) error {
	cnonce, err := newDigestCNonce()
	if err != nil {
		return err
	}
	nonceKey := challenge.realm + "\x00" + challenge.nonce
	c.digestNonceCounts[nonceKey]++
	if c.digestNonceCounts[nonceKey] == 0 {
		c.digestNonceCounts[nonceKey] = 1
	}
	nonceCount := fmt.Sprintf("%08x", c.digestNonceCounts[nonceKey])
	requestURI := request.URL.RequestURI()
	if requestURI == "" {
		requestURI = "/"
	}
	ha1 := md5Hex(c.username + ":" + challenge.realm + ":" + c.password)
	ha2 := md5Hex(request.Method + ":" + requestURI)
	response := md5Hex(ha1 + ":" + challenge.nonce + ":" + nonceCount + ":" + cnonce + ":" + challenge.qop + ":" + ha2)
	parts := []string{
		"username=" + quoteDigestValue(c.username),
		"realm=" + quoteDigestValue(challenge.realm),
		"nonce=" + quoteDigestValue(challenge.nonce),
		"uri=" + quoteDigestValue(requestURI),
		"response=" + quoteDigestValue(response),
		"algorithm=" + challenge.algorithm,
		"qop=" + challenge.qop,
		"nc=" + nonceCount,
		"cnonce=" + quoteDigestValue(cnonce),
	}
	if challenge.opaque != "" {
		parts = append(parts, "opaque="+quoteDigestValue(challenge.opaque))
	}
	request.Header.Set("Authorization", "Digest "+strings.Join(parts, ", "))
	return nil
}

func newDigestCNonce() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate digest cnonce: %w", err)
	}
	return fmt.Sprintf("%x", value), nil
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func quoteDigestValue(value string) string {
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value)
	return "\"" + escaped + "\""
}

func newHTTPStatusError(response *http.Response, result RemoteResponse, request *http.Request, challenges []string) *HTTPStatusError {
	statusErr := &HTTPStatusError{StatusCode: response.StatusCode, ETag: result.ETag, Method: request.Method}
	if response.StatusCode != http.StatusUnauthorized {
		return statusErr
	}
	statusErr.AuthScheme = authenticationScheme(challenges)
	switch requestAuthenticationScheme(request) {
	case "Basic":
		username, password, hasBasicCredentials := request.BasicAuth()
		statusErr.BasicCredentialsPresent = hasBasicCredentials && username != "" && password != ""
	case "Digest":
		statusErr.DigestCredentialsPresent = true
	}
	return statusErr
}

func authenticationScheme(challenges []string) string {
	for _, scheme := range []string{"Digest", "Basic", "Bearer", "Negotiate"} {
		if hasAuthenticationScheme(challenges, scheme) {
			return scheme
		}
	}
	for _, challenge := range challenges {
		if strings.TrimSpace(challenge) != "" {
			return "another"
		}
	}
	return ""
}

func hasAuthenticationScheme(challenges []string, scheme string) bool {
	for _, challenge := range challenges {
		for _, part := range strings.Split(challenge, ",") {
			fields := strings.Fields(strings.TrimSpace(part))
			if len(fields) > 0 && strings.EqualFold(fields[0], scheme) {
				return true
			}
		}
	}
	return false
}

func requestAuthenticationScheme(request *http.Request) string {
	fields := strings.Fields(request.Header.Get("Authorization"))
	if len(fields) == 0 {
		return ""
	}
	switch strings.ToLower(fields[0]) {
	case authSchemeBasic:
		return "Basic"
	case authSchemeDigest:
		return "Digest"
	default:
		return ""
	}
}

func digestChallengeFromHeaders(challenges []string) (digestChallenge, bool) {
	for _, challenge := range challenges {
		if parsed, ok := parseDigestChallenge(challenge); ok {
			return parsed, true
		}
	}
	return digestChallenge{}, false
}

func parseDigestChallenge(value string) (digestChallenge, bool) {
	value = strings.TrimSpace(value)
	if len(value) < len("Digest") || !strings.EqualFold(value[:len("Digest")], "Digest") {
		return digestChallenge{}, false
	}
	if len(value) > len("Digest") && value[len("Digest")] != ' ' && value[len("Digest")] != '\t' {
		return digestChallenge{}, false
	}
	parameters := parseAuthenticationParameters(value[len("Digest"):])
	realm, nonce := parameters["realm"], parameters["nonce"]
	if realm == "" || nonce == "" {
		return digestChallenge{}, false
	}
	algorithm := parameters["algorithm"]
	if algorithm == "" {
		algorithm = "MD5"
	}
	if !strings.EqualFold(algorithm, "MD5") {
		return digestChallenge{}, false
	}
	qop, ok := selectDigestQOP(parameters["qop"])
	if !ok {
		return digestChallenge{}, false
	}
	return digestChallenge{
		realm: realm, nonce: nonce, opaque: parameters["opaque"], algorithm: "MD5", qop: qop,
	}, true
}

func selectDigestQOP(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	for _, candidate := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), "auth") {
			return "auth", true
		}
	}
	return "", false
}

func parseAuthenticationParameters(value string) map[string]string {
	parameters := make(map[string]string)
	for {
		value = strings.TrimLeft(value, " \t,")
		if value == "" {
			return parameters
		}
		equalsIndex := strings.IndexByte(value, '=')
		if equalsIndex < 1 {
			return parameters
		}
		name := strings.TrimSpace(value[:equalsIndex])
		if name == "" || strings.ContainsAny(name, " \t") {
			return parameters
		}
		value = strings.TrimLeft(value[equalsIndex+1:], " \t")
		if value == "" {
			return parameters
		}
		var parameter string
		if value[0] == '"' {
			var builder strings.Builder
			escaped := false
			terminated := false
			index := 1
			for ; index < len(value); index++ {
				character := value[index]
				if escaped {
					builder.WriteByte(character)
					escaped = false
					continue
				}
				if character == '\\' {
					escaped = true
					continue
				}
				if character == '"' {
					terminated = true
					index++
					break
				}
				builder.WriteByte(character)
			}
			if !terminated {
				return parameters
			}
			parameter = builder.String()
			value = value[index:]
		} else {
			commaIndex := strings.IndexByte(value, ',')
			if commaIndex == -1 {
				parameter = strings.TrimSpace(value)
				value = ""
			} else {
				parameter = strings.TrimSpace(value[:commaIndex])
				value = value[commaIndex:]
			}
		}
		parameters[strings.ToLower(name)] = parameter
	}
}

func remoteRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\\\x00\r\n") {
		return "", ErrInvalidRemoteRoot
	}
	if value == "" || value == "/" {
		return "", nil
	}
	cleaned := path.Clean(strings.TrimPrefix(value, "/"))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidRemoteRoot
	}
	return cleaned, nil
}

func joinURLPath(base string, child string) string {
	if base == "" {
		base = "/"
	}
	return "/" + strings.Trim(path.Join(base, child), "/")
}

// WebDAV collection URLs are conventionally addressed with a trailing slash.
// Joplin sends its configuration PROPFIND that way, and some servers route the
// slashless variant through a different authentication handler.
func joinRequestURLPath(base string, child string) string {
	joined := joinURLPath(base, child)
	if child == "" && joined != "/" {
		return strings.TrimRight(joined, "/") + "/"
	}
	return joined
}
