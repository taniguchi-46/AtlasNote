package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	FormatVersion = 1
	ServiceName   = "atlasnote-webdav"

	DefaultProxyTimeoutSeconds = 1
	DefaultSyncIntervalSeconds = 0
)

var allowedSyncIntervals = map[int]struct{}{
	0: {}, 300: {}, 600: {}, 1800: {}, 3600: {}, 43200: {}, 86400: {},
}

type Status string

const (
	StatusDisabled     Status = "disabled"
	StatusIdle         Status = "idle"
	StatusPending      Status = "pending"
	StatusSyncing      Status = "syncing"
	StatusSynced       Status = "synced"
	StatusOffline      Status = "offline"
	StatusFailed       Status = "failed"
	StatusConflict     Status = "conflict"
	StatusAuthRequired Status = "auth-required"
)

type FailedClass string

const (
	FailedClassNone       FailedClass = ""
	FailedClassNetwork    FailedClass = "network"
	FailedClassTimeout    FailedClass = "timeout"
	FailedClassServer     FailedClass = "server"
	FailedClassRateLimit  FailedClass = "rate-limit"
	FailedClassAuth       FailedClass = "auth"
	FailedClassPermission FailedClass = "permission"
	FailedClassFormat     FailedClass = "format"
	FailedClassConflict   FailedClass = "conflict"
)

type Connection struct {
	Endpoint              string    `json:"endpoint"`
	RemoteRoot            string    `json:"remoteRoot"`
	Username              string    `json:"username"`
	VaultID               string    `json:"vaultId"`
	HeadManifestHash      string    `json:"headManifestHash"`
	HeadETag              string    `json:"headETag"`
	LastSyncAt            time.Time `json:"lastSyncAt"`
	HasLastSync           bool      `json:"hasLastSync"`
	Status                Status    `json:"status"`
	AutoSync              bool      `json:"autoSync"`
	SyncIntervalSeconds   int       `json:"syncIntervalSeconds"`
	AllowInsecureHTTP     bool      `json:"allowInsecureHTTP"`
	CustomTLSCertificates string    `json:"customTLSCertificates"`
	IgnoreTLSErrors       bool      `json:"ignoreTLSErrors"`
	ProxyEnabled          bool      `json:"proxyEnabled"`
	ProxyURL              string    `json:"proxyURL"`
	ProxyTimeoutSeconds   int       `json:"proxyTimeoutSeconds"`
	FailSafe              bool      `json:"failSafe"`
	CredentialRef         string    `json:"credentialRef"`
}

// ConnectionSettings is the non-secret, UI-facing view of a sync connection.
// Internal vault/head and credential-store identifiers intentionally never cross
// the Wails boundary.
type ConnectionSettings struct {
	WebDAVURL             string    `json:"webDAVURL"`
	Username              string    `json:"username"`
	LastSyncAt            time.Time `json:"lastSyncAt"`
	HasLastSync           bool      `json:"hasLastSync"`
	Status                Status    `json:"status"`
	SyncIntervalSeconds   int       `json:"syncIntervalSeconds"`
	AllowInsecureHTTP     bool      `json:"allowInsecureHTTP"`
	CustomTLSCertificates string    `json:"customTLSCertificates"`
	IgnoreTLSErrors       bool      `json:"ignoreTLSErrors"`
	ProxyEnabled          bool      `json:"proxyEnabled"`
	ProxyURL              string    `json:"proxyURL"`
	ProxyTimeoutSeconds   int       `json:"proxyTimeoutSeconds"`
	FailSafe              bool      `json:"failSafe"`
}

type ConnectionInput struct {
	WebDAVURL             string `json:"webDAVURL"`
	Endpoint              string `json:"endpoint"`
	RemoteRoot            string `json:"remoteRoot"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	AutoSync              bool   `json:"autoSync"`
	SyncIntervalSeconds   int    `json:"syncIntervalSeconds"`
	AllowInsecureHTTP     bool   `json:"allowInsecureHTTP"`
	CustomTLSCertificates string `json:"customTLSCertificates"`
	IgnoreTLSErrors       bool   `json:"ignoreTLSErrors"`
	ProxyEnabled          bool   `json:"proxyEnabled"`
	ProxyURL              string `json:"proxyURL"`
	ProxyTimeoutSeconds   int    `json:"proxyTimeoutSeconds"`
	FailSafe              *bool  `json:"failSafe"`
	SetupMode             string `json:"setupMode"`
	InitializeRemote      bool   `json:"initializeRemote"`
}

type StatusResult struct {
	Connection    *ConnectionSettings `json:"connection,omitempty"`
	Status        Status              `json:"status"`
	OutboxCount   int                 `json:"outboxCount"`
	ConflictCount int                 `json:"conflictCount"`
	Message       string              `json:"message,omitempty"`
}

type ConfigurationTestResult struct {
	Success           bool   `json:"success"`
	RemoteInitialized bool   `json:"remoteInitialized"`
	Message           string `json:"message"`
}

type SyncResult struct {
	Status     Status `json:"status"`
	Uploaded   int    `json:"uploaded"`
	Downloaded int    `json:"downloaded"`
	Conflicts  int    `json:"conflicts"`
	Remaining  int    `json:"remaining"`
	Message    string `json:"message,omitempty"`
}

type Conflict struct {
	ID               string    `json:"id"`
	EntityKey        string    `json:"entityKey"`
	EntityType       string    `json:"entityType"`
	LocalObjectHash  string    `json:"localObjectHash"`
	BaseObjectHash   string    `json:"baseObjectHash"`
	RemoteObjectHash string    `json:"remoteObjectHash"`
	LocalSnapshot    string    `json:"localSnapshot"`
	BaseSnapshot     string    `json:"baseSnapshot"`
	RemoteSnapshot   string    `json:"remoteSnapshot"`
	ConflictType     string    `json:"conflictType"`
	ResolutionStatus string    `json:"resolutionStatus"`
	CreatedAt        time.Time `json:"createdAt"`
	ResolvedAt       time.Time `json:"resolvedAt"`
	HasResolvedAt    bool      `json:"hasResolvedAt"`
}

// ConflictSummary is the UI-facing conflict shape. Full note snapshots stay in
// SQLite and are loaded by ID only when the user chooses a resolution.
type ConflictSummary struct {
	ID           string    `json:"id"`
	EntityKey    string    `json:"entityKey"`
	EntityType   string    `json:"entityType"`
	ConflictType string    `json:"conflictType"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (c Conflict) Summary() ConflictSummary {
	return ConflictSummary{
		ID: c.ID, EntityKey: c.EntityKey, EntityType: c.EntityType,
		ConflictType: c.ConflictType, CreatedAt: c.CreatedAt,
	}
}

type ConflictResolutionInput struct {
	ConflictID string `json:"conflictId"`
	Choice     string `json:"choice"`
}

type FormatDocument struct {
	FormatVersion int    `json:"formatVersion"`
	VaultID       string `json:"vaultId"`
}

type HeadDocument struct {
	FormatVersion int    `json:"formatVersion"`
	VaultID       string `json:"vaultId"`
	Generation    int64  `json:"generation"`
	ManifestHash  string `json:"manifestHash"`
}

type ManifestDocument struct {
	FormatVersion int             `json:"formatVersion"`
	VaultID       string          `json:"vaultId"`
	Generation    int64           `json:"generation"`
	Entries       []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	EntityKey  string `json:"entityKey"`
	EntityType string `json:"entityType"`
	ObjectHash string `json:"objectHash"`
}

type ObjectDocument struct {
	FormatVersion int             `json:"formatVersion"`
	EntityKey     string          `json:"entityKey"`
	EntityType    string          `json:"entityType"`
	Deleted       bool            `json:"deleted"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

type OutboxItem struct {
	Sequence         int64     `json:"sequence"`
	ChangeSetID      string    `json:"changeSetId"`
	EntityKey        string    `json:"entityKey"`
	EntityType       string    `json:"entityType"`
	ObjectHash       string    `json:"objectHash"`
	BaseManifestHash string    `json:"baseManifestHash"`
	BaseHeadETag     string    `json:"baseHeadETag"`
	ObjectJSON       string    `json:"objectJson"`
	Deleted          bool      `json:"deleted"`
	AttemptCount     int       `json:"attemptCount"`
	NextRetryAt      time.Time `json:"nextRetryAt"`
	FailedClass      string    `json:"failedClass"`
	CreatedAt        time.Time `json:"createdAt"`
}

type ItemState struct {
	EntityKey        string    `json:"entityKey"`
	EntityType       string    `json:"entityType"`
	LocalObjectHash  string    `json:"localObjectHash"`
	BaseObjectHash   string    `json:"baseObjectHash"`
	RemoteObjectHash string    `json:"remoteObjectHash"`
	BodyHash         string    `json:"bodyHash"`
	MetadataHash     string    `json:"metadataHash"`
	ResolutionState  string    `json:"resolutionState"`
	SnapshotJSON     string    `json:"snapshotJson"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

var (
	ErrInvalidEndpoint          = errors.New("sync endpoint must use HTTPS unless HTTP is explicitly allowed")
	ErrInvalidRemoteRoot        = errors.New("sync remote root is invalid")
	ErrRemoteNotInitialized     = errors.New("remote vault is not initialized")
	ErrRemoteImportNotEmpty     = errors.New("remote import requires an empty local vault")
	ErrRetryLimitReached        = errors.New("sync retry limit reached")
	ErrVaultMismatch            = errors.New("remote vault id does not match local vault")
	ErrInvalidRemoteFormat      = errors.New("remote vault format is invalid")
	ErrHeadPrecondition         = errors.New("remote head precondition failed")
	ErrCredentialsUnavailable   = errors.New("sync credentials are unavailable")
	ErrInvalidSyncInterval      = errors.New("sync interval is invalid")
	ErrInvalidProxySettings     = errors.New("sync proxy settings are invalid")
	ErrFailSafeTriggered        = errors.New("sync fail-safe stopped a potentially destructive operation")
	ErrSetupModeRequired        = errors.New("sync setup mode is required when changing the WebDAV target")
	ErrRemoteAlreadyInitialized = errors.New("remote vault is already initialized")
)

func ValidateSyncInterval(seconds int) error {
	if _, ok := allowedSyncIntervals[seconds]; !ok {
		return ErrInvalidSyncInterval
	}
	return nil
}

func (c Connection) Settings() ConnectionSettings {
	interval := c.SyncIntervalSeconds
	if interval == 0 && c.AutoSync {
		interval = 300
	}
	proxyTimeout := c.ProxyTimeoutSeconds
	if proxyTimeout == 0 {
		proxyTimeout = DefaultProxyTimeoutSeconds
	}
	return ConnectionSettings{
		WebDAVURL:             JoinWebDAVURL(c.Endpoint, c.RemoteRoot),
		Username:              c.Username,
		LastSyncAt:            c.LastSyncAt,
		HasLastSync:           c.HasLastSync,
		Status:                c.Status,
		SyncIntervalSeconds:   interval,
		AllowInsecureHTTP:     c.AllowInsecureHTTP,
		CustomTLSCertificates: c.CustomTLSCertificates,
		IgnoreTLSErrors:       c.IgnoreTLSErrors,
		ProxyEnabled:          c.ProxyEnabled,
		ProxyURL:              c.ProxyURL,
		ProxyTimeoutSeconds:   proxyTimeout,
		FailSafe:              c.FailSafe,
	}
}

func JoinWebDAVURL(endpoint string, remoteRoot string) string {
	validatedEndpoint, err := ValidateEndpoint(endpoint, true)
	if err != nil {
		return strings.TrimSpace(endpoint)
	}
	validatedRoot, err := NormalizeRemoteRoot(remoteRoot)
	if err != nil || validatedRoot == "/" {
		return validatedEndpoint
	}
	parsed, err := url.Parse(validatedEndpoint)
	if err != nil {
		return validatedEndpoint
	}
	parsed.Path = path.Join(parsed.Path, validatedRoot)
	return strings.TrimRight(parsed.String(), "/")
}

func NormalizeWebDAVURL(value string, allowInsecureHTTP bool) (string, error) {
	return ValidateEndpoint(value, allowInsecureHTTP)
}

func ValidateEndpoint(value string, allowInsecureHTTP bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil {
		return "", ErrInvalidEndpoint
	}
	scheme := strings.ToLower(parsed.Scheme)
	if parsed.Host == "" || parsed.User != nil || (scheme != "https" && !(allowInsecureHTTP && scheme == "http")) {
		return "", ErrInvalidEndpoint
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidEndpoint
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func NormalizeRemoteRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/", nil
	}
	if strings.ContainsAny(value, "\\\x00\r\n") {
		return "", ErrInvalidRemoteRoot
	}
	if !strings.HasPrefix(value, "/") {
		return "", ErrInvalidRemoteRoot
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.Contains(cleaned, "..") {
		return "", ErrInvalidRemoteRoot
	}
	return "/" + strings.Trim(cleaned, "/"), nil
}

func NewVaultID() (string, error) {
	return randomHex(16)
}

func randomHex(size int) (string, error) {
	return secureRandomHex(make([]byte, size))
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}

func objectDocument(changeType string, key string, payload []byte, deleted bool) ([]byte, error) {
	if deleted {
		payload = nil
	}
	return canonicalJSON(ObjectDocument{
		FormatVersion: FormatVersion,
		EntityKey:     key,
		EntityType:    changeType,
		Deleted:       deleted,
		Payload:       payload,
	})
}

func manifestFor(vaultID string, generation int64, objects map[string]ManifestEntry) ManifestDocument {
	entries := make([]ManifestEntry, 0, len(objects))
	for _, entry := range objects {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].EntityKey != entries[j].EntityKey {
			return entries[i].EntityKey < entries[j].EntityKey
		}
		return entries[i].ObjectHash < entries[j].ObjectHash
	})
	return ManifestDocument{
		FormatVersion: FormatVersion,
		VaultID:       vaultID,
		Generation:    generation,
		Entries:       entries,
	}
}

func validateFormat(format FormatDocument) error {
	if format.FormatVersion != FormatVersion || len(format.VaultID) != 32 {
		return ErrInvalidRemoteFormat
	}
	if _, err := hex.DecodeString(format.VaultID); err != nil || format.VaultID != strings.ToLower(format.VaultID) {
		return ErrInvalidRemoteFormat
	}
	return nil
}

func validateHead(head HeadDocument, format FormatDocument) error {
	if head.FormatVersion != FormatVersion || head.VaultID != format.VaultID || !isSHA256Hex(head.ManifestHash) || head.Generation < 0 {
		return ErrInvalidRemoteFormat
	}
	return nil
}

func validateManifest(manifest ManifestDocument, format FormatDocument, expectedHash string) error {
	if manifest.FormatVersion != FormatVersion || manifest.VaultID != format.VaultID || manifest.Generation < 0 {
		return ErrInvalidRemoteFormat
	}
	keys := make([]string, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if _, ok := entityIDFromKey(entry.EntityType, entry.EntityKey); !ok || !isSHA256Hex(entry.ObjectHash) {
			return ErrInvalidRemoteFormat
		}
		keys = append(keys, entry.EntityKey)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			return ErrInvalidRemoteFormat
		}
	}
	encoded, err := canonicalJSON(manifest)
	if err != nil || hashBytes(encoded) != expectedHash {
		return ErrInvalidRemoteFormat
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isEntityID(value string) bool {
	if len(value) != 32 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func entityIDFromKey(entityType string, entityKey string) (string, bool) {
	switch entityType {
	case "note", "notebook", "tag", "note-tags":
	default:
		return "", false
	}
	prefix := entityType + ":"
	if !strings.HasPrefix(entityKey, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(entityKey, prefix)
	return id, isEntityID(id)
}

func ensureObjectHash(document []byte, expected string) error {
	if hashBytes(document) != expected {
		return fmt.Errorf("%w: object hash mismatch", ErrInvalidRemoteFormat)
	}
	return nil
}
