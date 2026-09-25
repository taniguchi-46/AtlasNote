package localipc

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"atlasnote/internal/readapi"
)

type handlerFunc func(context.Context, readapi.Principal, readapi.Request) readapi.Response

func (handler handlerFunc) Execute(ctx context.Context, principal readapi.Principal, request readapi.Request) readapi.Response {
	return handler(ctx, principal, request)
}

func TestMCPSessionExpiresAndIsReclaimed(t *testing.T) {
	start := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	var nowNanos atomic.Int64
	nowNanos.Store(start.UnixNano())
	server := startTestServer(t, ServerConfig{
		ManagementRoot: t.TempDir(),
		StorageSpaceID: "test-space",
		MCPSessionTTL:  30 * time.Minute,
		Now: func() time.Time {
			return time.Unix(0, nowNanos.Load()).UTC()
		},
	})
	rootClient, err := Connect(filepath.Dir(server.descriptorPath))
	if err != nil {
		t.Fatal(err)
	}
	var expiredClient *Client
	for index := 0; index < 64; index++ {
		session, createErr := rootClient.CreateMCPSession(t.Context(), MCPSessionScope{})
		if createErr != nil {
			t.Fatalf("create session %d: %v", index+1, createErr)
		}
		if index == 0 {
			expiredClient = session
		}
	}
	if _, err := rootClient.CreateMCPSession(t.Context(), MCPSessionScope{}); !errors.Is(err, ErrConnection) {
		t.Fatalf("65th live session error = %v", err)
	}

	nowNanos.Store(start.Add(31 * time.Minute).UnixNano())
	requestID, _ := NewRequestID()
	if _, err := expiredClient.Call(t.Context(), readapi.OperationNotesList, readapi.NoteListInput{}, requestID); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("expired session authentication error = %v", err)
	}
	replacement, err := rootClient.CreateMCPSession(t.Context(), MCPSessionScope{})
	if err != nil {
		t.Fatalf("expired sessions were not reclaimed: %v", err)
	}
	requestID, _ = NewRequestID()
	if _, err := replacement.Call(t.Context(), readapi.OperationNotesList, readapi.NoteListInput{}, requestID); err != nil {
		t.Fatalf("replacement session call: %v", err)
	}
}

func TestMCPSessionRevokeInvalidatesToken(t *testing.T) {
	server := startTestServer(t, ServerConfig{ManagementRoot: t.TempDir(), StorageSpaceID: "test-space"})
	rootClient, err := Connect(filepath.Dir(server.descriptorPath))
	if err != nil {
		t.Fatal(err)
	}
	session, err := rootClient.CreateMCPSession(t.Context(), MCPSessionScope{})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RevokeMCPSession(t.Context()); err != nil {
		t.Fatal(err)
	}
	requestID, _ := NewRequestID()
	if _, err := session.Call(t.Context(), readapi.OperationNotesList, readapi.NoteListInput{}, requestID); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("revoked session authentication error = %v", err)
	}
}

func TestMCPSessionPermissionsAreLimitedByParent(t *testing.T) {
	principalReceived := make(chan readapi.Principal, 1)
	server := startTestServer(t, ServerConfig{
		ManagementRoot: t.TempDir(),
		StorageSpaceID: "test-space",
		Permissions:    []string{readapi.PermissionMetadata},
		Handler: handlerFunc(func(_ context.Context, principal readapi.Principal, request readapi.Request) readapi.Response {
			principalReceived <- principal
			return readapi.Response{
				APIVersion: readapi.APIVersion,
				RequestID:  request.RequestID,
				Status:     readapi.StatusOK,
				Data:       map[string]any{},
			}
		}),
	})
	rootClient, err := Connect(filepath.Dir(server.descriptorPath))
	if err != nil {
		t.Fatal(err)
	}
	noteID := "0123456789abcdef0123456789abcdef"
	session, err := rootClient.CreateMCPSession(t.Context(), MCPSessionScope{NoteIDs: []string{noteID}})
	if err != nil {
		t.Fatal(err)
	}
	requestID, _ := NewRequestID()
	if _, err := session.Call(t.Context(), readapi.OperationNotesGet, readapi.NoteGetInput{NoteID: noteID}, requestID); err != nil {
		t.Fatal(err)
	}
	principal := <-principalReceived
	if !principal.Permissions[readapi.PermissionMetadata] || principal.Permissions[readapi.PermissionContent] {
		t.Fatalf("MCP permissions were not intersected with parent: %+v", principal.Permissions)
	}
	if !principal.ScopeRestricted || !principal.AllowedNoteIDs[noteID] {
		t.Fatalf("MCP publication scope was not preserved: %+v", principal)
	}
}

func startTestServer(t *testing.T, config ServerConfig) *Server {
	t.Helper()
	if config.Handler == nil {
		config.Handler = handlerFunc(func(_ context.Context, _ readapi.Principal, request readapi.Request) readapi.Response {
			return readapi.Response{
				APIVersion: readapi.APIVersion,
				RequestID:  request.RequestID,
				Status:     readapi.StatusOK,
				Data:       map[string]any{},
			}
		})
	}
	server, err := Start(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Errorf("stop IPC server: %v", err)
		}
	})
	return server
}
