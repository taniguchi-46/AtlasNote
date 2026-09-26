package localipc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/readapi"
)

const (
	maxRequestBytes      = 1 << 20
	defaultMCPSessionTTL = 30 * time.Minute
)

type Handler interface {
	Execute(context.Context, readapi.Principal, readapi.Request) readapi.Response
}

type ServerConfig struct {
	ManagementRoot string
	StorageSpaceID string
	Handler        Handler
	Permissions    []string
	MCPSessionTTL  time.Duration
	Now            func() time.Time
}

type mcpSession struct {
	principal readapi.Principal
	expiresAt time.Time
}

type Server struct {
	httpServer     *http.Server
	listener       net.Listener
	descriptorPath string
	descriptor     Descriptor
	done           chan struct{}
	stopOnce       sync.Once
	sessionMu      sync.Mutex
	sessions       map[[sha256.Size]byte]mcpSession
	sessionTTL     time.Duration
	now            func() time.Time
}

func Start(config ServerConfig) (*Server, error) {
	if config.Handler == nil || strings.TrimSpace(config.ManagementRoot) == "" || strings.TrimSpace(config.StorageSpaceID) == "" {
		return nil, errors.New("invalid IPC server configuration")
	}
	permissions := append([]string(nil), config.Permissions...)
	if len(permissions) == 0 {
		permissions = []string{readapi.PermissionMetadata, readapi.PermissionContent, readapi.PermissionProposal}
	}
	sessionTTL := config.MCPSessionTTL
	if sessionTTL <= 0 {
		sessionTTL = defaultMCPSessionTTL
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	token, err := randomSecret(32)
	if err != nil {
		return nil, err
	}
	clientID, err := randomSecret(16)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("start loopback IPC listener")
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.IP == nil || !address.IP.IsLoopback() {
		_ = listener.Close()
		return nil, errors.New("IPC listener is not loopback-only")
	}
	descriptor := Descriptor{
		Version: 1, Endpoint: "http://" + listener.Addr().String() + "/v1/request",
		Token: token, ClientID: clientID, StorageSpaceID: config.StorageSpaceID,
		Permissions: permissions, PID: os.Getpid(),
	}
	if err := validateDescriptor(descriptor); err != nil {
		_ = listener.Close()
		return nil, err
	}
	principal := readapi.Principal{
		ClientID: clientID, Kind: "cli", StorageSpaceID: config.StorageSpaceID,
		Permissions: make(map[string]bool, len(permissions)),
	}
	for _, permission := range permissions {
		principal.Permissions[permission] = true
	}
	server := &Server{
		listener: listener, descriptorPath: DescriptorPath(config.ManagementRoot),
		descriptor: descriptor, done: make(chan struct{}), sessions: make(map[[sha256.Size]byte]mcpSession),
		sessionTTL: sessionTTL, now: now,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/request", server.handle(config.Handler, principal))
	mux.HandleFunc("/v1/session", server.handleMCPSession(principal))
	server.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       20 * time.Second,
	}
	if err := writeDescriptor(server.descriptorPath, descriptor); err != nil {
		_ = listener.Close()
		return nil, errors.New("publish IPC descriptor")
	}
	go func() {
		defer close(server.done)
		_ = server.httpServer.Serve(listener)
	}()
	return server, nil
}

func (s *Server) handle(handler Handler, principal readapi.Principal) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		requestPrincipal, ok := s.principalForAuthorization(request.Header.Get("Authorization"), principal)
		if !ok {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Content-Type") != "application/json" {
			writer.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		body := http.MaxBytesReader(writer, request.Body, maxRequestBytes)
		defer body.Close()
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		var input readapi.Request
		if err := decoder.Decode(&input); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		response := handler.Execute(request.Context(), requestPrincipal, input)
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(response)
	}
}

func (s *Server) handleMCPSession(rootPrincipal readapi.Principal) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if request.Method == http.MethodDelete {
			if !s.revokeSession(request.Header.Get("Authorization")) {
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !authorized(request.Header.Get("Authorization"), s.descriptor.Token) {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Content-Type") != "application/json" {
			writer.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		body := http.MaxBytesReader(writer, request.Body, 64*1024)
		defer body.Close()
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		var input mcpSessionRequest
		if err := decoder.Decode(&input); err != nil || input.Kind != "mcp" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		noteIDs, err := normalizePublishedIDs(input.Scope.NoteIDs)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		notebookIDs, err := normalizePublishedIDs(input.Scope.NotebookIDs)
		if err != nil || len(noteIDs)+len(notebookIDs) > maxPublishedTargets {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		token, err := randomSecret(32)
		if err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		clientID, err := randomSecret(16)
		if err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		publishedPermissions := map[string]bool{readapi.PermissionMetadata: true}
		if len(noteIDs)+len(notebookIDs) > 0 {
			publishedPermissions[readapi.PermissionContent] = true
			publishedPermissions[readapi.PermissionProposal] = true
		}
		permissions := make([]string, 0, 3)
		for _, permission := range []string{readapi.PermissionMetadata, readapi.PermissionContent, readapi.PermissionProposal} {
			if rootPrincipal.Permissions[permission] && publishedPermissions[permission] {
				permissions = append(permissions, permission)
			}
		}
		principal := readapi.Principal{
			ClientID: clientID, Kind: "mcp", StorageSpaceID: rootPrincipal.StorageSpaceID,
			Permissions: make(map[string]bool, len(permissions)), ScopeRestricted: true,
			AllowedNoteIDs: make(map[string]bool, len(noteIDs)), AllowedNotebookIDs: make(map[string]bool, len(notebookIDs)),
		}
		for _, permission := range permissions {
			principal.Permissions[permission] = true
		}
		for _, id := range noteIDs {
			principal.AllowedNoteIDs[id] = true
		}
		for _, id := range notebookIDs {
			principal.AllowedNotebookIDs[id] = true
		}
		now := s.now()
		s.sessionMu.Lock()
		s.removeExpiredSessionsLocked(now)
		if len(s.sessions) >= 64 {
			s.sessionMu.Unlock()
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		s.sessions[sha256.Sum256([]byte(token))] = mcpSession{principal: principal, expiresAt: now.Add(s.sessionTTL)}
		s.sessionMu.Unlock()
		descriptor := Descriptor{
			Version: 1, Endpoint: s.descriptor.Endpoint, Token: token, ClientID: clientID,
			StorageSpaceID: rootPrincipal.StorageSpaceID, Permissions: permissions, PID: os.Getpid(),
		}
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(mcpSessionResponse{Descriptor: descriptor})
	}
}

func (s *Server) principalForAuthorization(header string, root readapi.Principal) (readapi.Principal, bool) {
	token, ok := bearerToken(header)
	if !ok {
		return readapi.Principal{}, false
	}
	if len(token) == len(s.descriptor.Token) && subtle.ConstantTimeCompare([]byte(token), []byte(s.descriptor.Token)) == 1 {
		return root, true
	}
	tokenHash := sha256.Sum256([]byte(token))
	s.sessionMu.Lock()
	session, exists := s.sessions[tokenHash]
	if exists && !s.now().Before(session.expiresAt) {
		delete(s.sessions, tokenHash)
		exists = false
	}
	s.sessionMu.Unlock()
	return session.principal, exists
}

func (s *Server) revokeSession(header string) bool {
	token, ok := bearerToken(header)
	if !ok {
		return false
	}
	tokenHash := sha256.Sum256([]byte(token))
	s.sessionMu.Lock()
	_, exists := s.sessions[tokenHash]
	if exists {
		delete(s.sessions, tokenHash)
	}
	s.sessionMu.Unlock()
	return exists
}

func (s *Server) removeExpiredSessionsLocked(now time.Time) {
	for tokenHash, session := range s.sessions {
		if !now.Before(session.expiresAt) {
			delete(s.sessions, tokenHash)
		}
	}
}

func (s *Server) Stop(ctx context.Context) error {
	var stopErr error
	s.stopOnce.Do(func() {
		stopErr = s.httpServer.Shutdown(ctx)
		if stopErr != nil {
			stopErr = errors.Join(stopErr, s.httpServer.Close())
		}
		select {
		case <-s.done:
		case <-ctx.Done():
			stopErr = errors.Join(stopErr, ctx.Err())
		}
		if descriptor, err := loadDescriptorPath(s.descriptorPath); err == nil && descriptor.Token == s.descriptor.Token {
			if removeErr := os.Remove(s.descriptorPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				stopErr = errors.Join(stopErr, removeErr)
			}
		}
	})
	return stopErr
}

func loadDescriptorPath(path string) (Descriptor, error) {
	return LoadDescriptor(filepath.Dir(path))
}

func randomSecret(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func authorized(header, token string) bool {
	provided, ok := bearerToken(header)
	if !ok {
		return false
	}
	if len(provided) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	return token, token != ""
}
