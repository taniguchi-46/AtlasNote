package localipc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"atlasnote/internal/readapi"
)

const descriptorName = ".atlasnote-ipc.json"

const maxPublishedTargets = 256

type MCPSessionScope struct {
	NoteIDs     []string `json:"noteIds"`
	NotebookIDs []string `json:"notebookIds"`
}

type mcpSessionRequest struct {
	Kind  string          `json:"kind"`
	Scope MCPSessionScope `json:"scope"`
}

type mcpSessionResponse struct {
	Descriptor Descriptor `json:"descriptor"`
}

type Descriptor struct {
	Version        int      `json:"version"`
	Endpoint       string   `json:"endpoint"`
	Token          string   `json:"token"`
	ClientID       string   `json:"clientId"`
	StorageSpaceID string   `json:"storageSpaceId"`
	Permissions    []string `json:"permissions"`
	PID            int      `json:"pid"`
}

func DescriptorPath(managementRoot string) string {
	return filepath.Join(filepath.Clean(managementRoot), descriptorName)
}

func writeDescriptor(path string, descriptor Descriptor) error {
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return err
	}
	if err := writePrivateAtomic(path, encoded); err != nil {
		return err
	}
	if err := restrictDescriptor(path); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func LoadDescriptor(managementRoot string) (Descriptor, error) {
	path := DescriptorPath(managementRoot)
	info, err := os.Lstat(path)
	if err != nil {
		return Descriptor{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 16*1024 {
		return Descriptor{}, errors.New("invalid IPC descriptor")
	}
	if err := validateDescriptorSecurity(path, info); err != nil {
		return Descriptor{}, err
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return Descriptor{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var descriptor Descriptor
	if err := decoder.Decode(&descriptor); err != nil {
		return Descriptor{}, errors.New("invalid IPC descriptor")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Descriptor{}, errors.New("invalid IPC descriptor")
	}
	if err := validateDescriptor(descriptor); err != nil {
		return Descriptor{}, err
	}
	return descriptor, nil
}

func validateDescriptor(descriptor Descriptor) error {
	if descriptor.Version != 1 || len(descriptor.Token) < 43 || len(descriptor.Token) > 256 ||
		strings.TrimSpace(descriptor.ClientID) == "" || len(descriptor.ClientID) > 128 ||
		strings.TrimSpace(descriptor.StorageSpaceID) == "" || descriptor.PID < 1 {
		return errors.New("invalid IPC descriptor")
	}
	parsed, err := url.Parse(descriptor.Endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Path != "/v1/request" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid IPC endpoint")
	}
	host, _, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return errors.New("invalid IPC endpoint")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("IPC endpoint is not loopback-only")
	}
	allowed := map[string]bool{readapi.PermissionMetadata: true, readapi.PermissionContent: true}
	if len(descriptor.Permissions) == 0 || len(descriptor.Permissions) > len(allowed) {
		return errors.New("invalid IPC permissions")
	}
	for _, permission := range descriptor.Permissions {
		if !allowed[permission] {
			return fmt.Errorf("invalid IPC permission")
		}
	}
	return nil
}

func normalizePublishedIDs(values []string) ([]string, error) {
	if len(values) > maxPublishedTargets {
		return nil, errors.New("too many published targets")
	}
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) != 32 || value != strings.ToLower(value) {
			return nil, errors.New("invalid published target")
		}
		if _, err := hex.DecodeString(value); err != nil {
			return nil, errors.New("invalid published target")
		}
		if !seen[value] {
			seen[value] = true
			normalized = append(normalized, value)
		}
	}
	return normalized, nil
}
