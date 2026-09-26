package localipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"atlasnote/internal/readapi"
)

type Client struct {
	descriptor Descriptor
	httpClient *http.Client
}

var (
	ErrAuthentication = errors.New("IPC authentication failed")
	ErrConnection     = errors.New("IPC connection failed")
)

func NewClient(descriptor Descriptor) (*Client, error) {
	if err := validateDescriptor(descriptor); err != nil {
		return nil, err
	}
	return &Client{
		descriptor: descriptor,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func Connect(managementRoot string) (*Client, error) {
	descriptor, err := LoadDescriptor(managementRoot)
	if err != nil {
		return nil, err
	}
	return NewClient(descriptor)
}

func (c *Client) CreateMCPSession(ctx context.Context, scope MCPSessionScope) (*Client, error) {
	noteIDs, err := normalizePublishedIDs(scope.NoteIDs)
	if err != nil {
		return nil, err
	}
	notebookIDs, err := normalizePublishedIDs(scope.NotebookIDs)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(c.descriptor.Endpoint)
	if err != nil {
		return nil, ErrConnection
	}
	endpoint.Path = "/v1/session"
	requestBody, err := json.Marshal(mcpSessionRequest{
		Kind:  "mcp",
		Scope: MCPSessionScope{NoteIDs: noteIDs, NotebookIDs: notebookIDs},
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.descriptor.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, ErrConnection
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return nil, ErrAuthentication
	}
	if response.StatusCode != http.StatusOK {
		return nil, ErrConnection
	}
	limited := io.LimitReader(response.Body, 16*1024+1)
	encoded, err := io.ReadAll(limited)
	if err != nil || len(encoded) > 16*1024 {
		return nil, ErrConnection
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var session mcpSessionResponse
	if err := decoder.Decode(&session); err != nil {
		return nil, ErrConnection
	}
	return NewClient(session.Descriptor)
}

func (c *Client) RevokeMCPSession(ctx context.Context) error {
	endpoint, err := url.Parse(c.descriptor.Endpoint)
	if err != nil {
		return ErrConnection
	}
	endpoint.Path = "/v1/session"
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.descriptor.Token)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return ErrConnection
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusUnauthorized {
		return nil
	}
	return ErrConnection
}

func (c *Client) Call(ctx context.Context, operation string, params any, requestID string) (readapi.Response, error) {
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return readapi.Response{}, err
	}
	request := readapi.Request{
		APIVersion: readapi.APIVersion, RequestID: requestID, ClientID: c.descriptor.ClientID,
		Scope: readapi.Scope{StorageSpaceID: c.descriptor.StorageSpaceID}, Operation: operation, Params: encodedParams,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return readapi.Response{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.descriptor.Endpoint, bytes.NewReader(body))
	if err != nil {
		return readapi.Response{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.descriptor.Token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return readapi.Response{}, ErrConnection
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusUnauthorized {
		return readapi.Response{}, ErrAuthentication
	}
	if httpResponse.StatusCode != http.StatusOK {
		return readapi.Response{}, ErrConnection
	}
	const maxResponseBytes = 3 << 20
	limited := io.LimitReader(httpResponse.Body, maxResponseBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil || len(encoded) > maxResponseBytes {
		return readapi.Response{}, errors.New("Atlas Noteの応答を読み取れませんでした")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var response readapi.Response
	if err := decoder.Decode(&response); err != nil {
		return readapi.Response{}, errors.New("Atlas Noteの応答を検証できませんでした")
	}
	if response.APIVersion != readapi.APIVersion || response.RequestID != requestID {
		return readapi.Response{}, errors.New("Atlas Noteの応答を検証できませんでした")
	}
	return response, nil
}

func NewRequestID() (string, error) {
	return randomSecret(16)
}
