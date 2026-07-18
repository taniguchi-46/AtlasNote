package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	stdsync "sync"
	"time"

	"atlasnote/internal/note"
)

const (
	formatPath  = ".atlasnote/format.json"
	headPath    = ".atlasnote/head.json"
	manifestDir = ".atlasnote/manifests"
	objectDir   = ".atlasnote/objects"
)

type Service struct {
	repository      *Repository
	notes           *note.Service
	credentials     *CredentialManager
	clientFactory   func(Connection, string) (RemoteClient, error)
	recoveryDataDir string
	recoveryTokens  map[string]recoveryAuthorization
	mu              stdsync.Mutex
}

func NewService(repository *Repository, notes *note.Service, credentials *CredentialManager) *Service {
	service := &Service{
		repository:     repository,
		notes:          notes,
		credentials:    credentials,
		recoveryTokens: make(map[string]recoveryAuthorization),
	}
	service.clientFactory = func(connection Connection, secret string) (RemoteClient, error) {
		return NewHTTPClientWithConfig(HTTPClientConfig{
			Endpoint: connection.Endpoint, RemoteRoot: connection.RemoteRoot,
			Username: connection.Username, Password: secret,
			AllowInsecureHTTP:     connection.AllowInsecureHTTP,
			CustomTLSCertificates: connection.CustomTLSCertificates,
			IgnoreTLSErrors:       connection.IgnoreTLSErrors,
			ProxyEnabled:          connection.ProxyEnabled, ProxyURL: connection.ProxyURL,
			ProxyTimeoutSeconds: connection.ProxyTimeoutSeconds,
		})
	}
	return service
}

func (s *Service) SetRecoveryDataDir(dataDir string) {
	s.recoveryDataDir = strings.TrimSpace(dataDir)
}

func (s *Service) SetClientFactory(factory func(Connection, string) (RemoteClient, error)) {
	s.clientFactory = factory
}

func (s *Service) lockNotesForSync(ctx context.Context) (context.Context, func()) {
	if s.notes == nil {
		return ctx, func() {}
	}
	return s.notes.BeginSyncExclusive(ctx)
}

func (s *Service) GetStatus(ctx context.Context) (StatusResult, error) {
	connection, err := s.repository.GetConnection(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	outboxCount, err := s.repository.CountOutbox(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	conflictCount, err := s.repository.CountConflicts(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	status := StatusDisabled
	if connection != nil {
		status = connection.Status
		if conflictCount == 0 && status == StatusConflict {
			status = StatusSynced
		}
		if outboxCount > 0 && (status == StatusIdle || status == StatusSynced) {
			status = StatusPending
		}
		if conflictCount > 0 {
			status = StatusConflict
		}
	}
	var settings *ConnectionSettings
	if connection != nil {
		value := connection.Settings()
		value.Status = status
		settings = &value
	}
	return StatusResult{
		Connection:    settings,
		Status:        status,
		OutboxCount:   outboxCount,
		ConflictCount: conflictCount,
	}, nil
}

func (s *Service) Configure(ctx context.Context, input ConnectionInput) (StatusResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, unlockNotes := s.lockNotesForSync(ctx)
	defer unlockNotes()

	connection, err := normalizeConnectionInput(input)
	if err != nil {
		return StatusResult{}, err
	}
	existing, err := s.repository.GetConnection(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	sameTarget := existing != nil && connectionTargetsMatch(*existing, connection)
	mode := strings.TrimSpace(input.SetupMode)
	if mode == "" && input.InitializeRemote {
		mode = "initialize"
	}
	if mode == "" && sameTarget {
		mode = "update"
	}
	if mode == "" {
		return StatusResult{}, ErrSetupModeRequired
	}

	secret := input.Password
	if secret == "" {
		if !sameTarget || existing == nil {
			return StatusResult{}, errors.New("sync password is required for a new WebDAV target")
		}
		secret, err = s.credentials.Get(existing.CredentialRef)
		if err != nil {
			return StatusResult{}, ErrCredentialsUnavailable
		}
	}

	connection.Status = StatusIdle
	resetTracking := existing != nil && !sameTarget
	needsInitialSnapshot := existing == nil || resetTracking
	switch mode {
	case "update":
		if !sameTarget || existing == nil {
			return StatusResult{}, ErrSetupModeRequired
		}
		copyConnectionIdentity(&connection, *existing)
		needsInitialSnapshot = false
	case "initialize":
		inspection, inspectErr := s.inspectRemote(ctx, connection, secret)
		if inspectErr != nil {
			return StatusResult{}, inspectErr
		}
		if inspection.initialized {
			return StatusResult{}, ErrRemoteAlreadyInitialized
		}
		resetTracking = existing != nil
		needsInitialSnapshot = true
		connection.VaultID, err = NewVaultID()
		if err != nil {
			return StatusResult{}, err
		}
	case "import":
		inspection, inspectErr := s.inspectRemote(ctx, connection, secret)
		if inspectErr != nil {
			return StatusResult{}, inspectErr
		}
		if !inspection.initialized {
			return StatusResult{}, ErrRemoteNotInitialized
		}
		if err := s.requireEmptyLocalVault(ctx); err != nil {
			return StatusResult{}, err
		}
		connection.VaultID = inspection.state.format.VaultID
		connection.HeadManifestHash = inspection.state.head.ManifestHash
		connection.HeadETag = inspection.state.headETag
		resetTracking = existing != nil
		needsInitialSnapshot = false
	case "reconnect":
		if existing == nil {
			return StatusResult{}, ErrSetupModeRequired
		}
		inspection, inspectErr := s.inspectRemote(ctx, connection, secret)
		if inspectErr != nil {
			return StatusResult{}, inspectErr
		}
		if !inspection.initialized || inspection.state.format.VaultID != existing.VaultID {
			return StatusResult{}, ErrVaultMismatch
		}
		copyConnectionIdentity(&connection, *existing)
		resetTracking = false
		needsInitialSnapshot = false
	default:
		return StatusResult{}, ErrSetupModeRequired
	}
	var initialChanges []note.SyncChange
	if needsInitialSnapshot && s.notes != nil {
		initialChanges, err = s.notes.ExportSyncChanges(ctx)
		if err != nil {
			return StatusResult{}, err
		}
	}

	credentialRef := ""
	credentialPersisted := true
	credentialCreated := false
	if input.Password == "" && existing != nil {
		credentialRef = existing.CredentialRef
	} else {
		credentialRef, err = randomHex(16)
		if err != nil {
			return StatusResult{}, err
		}
		credentialPersisted, err = s.credentials.Save(credentialRef, secret, true)
		if err != nil {
			return StatusResult{}, err
		}
		credentialCreated = true
	}
	connection.CredentialRef = credentialRef
	if err := s.repository.ConfigureConnection(ctx, connection, resetTracking, initialChanges); err != nil {
		if credentialCreated {
			_ = s.credentials.Delete(credentialRef)
		}
		return StatusResult{}, err
	}
	if credentialCreated && existing != nil && existing.CredentialRef != credentialRef {
		_ = s.credentials.Delete(existing.CredentialRef)
	}
	result, err := s.GetStatus(ctx)
	if err == nil && !credentialPersisted {
		result.Message = "secure credential store unavailable; password is available for this session only"
	}
	return result, err
}

type remoteInspection struct {
	initialized bool
	state       remoteState
}

func (s *Service) TestConfiguration(ctx context.Context, input ConnectionInput) (ConfigurationTestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	connection, err := normalizeConnectionInput(input)
	if err != nil {
		return ConfigurationTestResult{}, err
	}
	existing, err := s.repository.GetConnection(ctx)
	if err != nil {
		return ConfigurationTestResult{}, err
	}
	secret := input.Password
	if secret == "" {
		if existing == nil || !connectionTargetsMatch(*existing, connection) {
			return ConfigurationTestResult{}, errors.New("sync password is required for a new WebDAV target")
		}
		secret, err = s.credentials.Get(existing.CredentialRef)
		if err != nil {
			return ConfigurationTestResult{}, ErrCredentialsUnavailable
		}
	}
	inspection, err := s.inspectRemote(ctx, connection, secret)
	if err != nil {
		return ConfigurationTestResult{}, err
	}
	message := "WebDAV connection succeeded"
	if !inspection.initialized {
		message = "WebDAV connection succeeded; Atlas Note vault is not initialized"
	}
	return ConfigurationTestResult{Success: true, RemoteInitialized: inspection.initialized, Message: message}, nil
}

func normalizeConnectionInput(input ConnectionInput) (Connection, error) {
	endpointValue, remoteRootValue := input.Endpoint, input.RemoteRoot
	if strings.TrimSpace(input.WebDAVURL) != "" {
		endpointValue, remoteRootValue = input.WebDAVURL, "/"
	}
	endpoint, err := ValidateEndpoint(endpointValue, input.AllowInsecureHTTP)
	if err != nil {
		return Connection{}, err
	}
	remoteRoot, err := NormalizeRemoteRoot(remoteRootValue)
	if err != nil {
		return Connection{}, err
	}
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return Connection{}, errors.New("sync username is required")
	}
	interval := input.SyncIntervalSeconds
	if interval == 0 && input.AutoSync {
		interval = 300
	}
	if err := ValidateSyncInterval(interval); err != nil {
		return Connection{}, err
	}
	proxyTimeout := input.ProxyTimeoutSeconds
	if proxyTimeout == 0 {
		proxyTimeout = DefaultProxyTimeoutSeconds
	}
	if proxyTimeout < 1 || proxyTimeout > 60 {
		return Connection{}, ErrInvalidProxySettings
	}
	if input.ProxyEnabled {
		if _, err := validateProxyURL(input.ProxyURL); err != nil {
			return Connection{}, err
		}
	}
	failSafe := true
	if input.FailSafe != nil {
		failSafe = *input.FailSafe
	}
	connection := Connection{
		Endpoint: endpoint, RemoteRoot: remoteRoot, Username: username,
		Status: StatusIdle, AutoSync: interval > 0, SyncIntervalSeconds: interval,
		AllowInsecureHTTP:     input.AllowInsecureHTTP,
		CustomTLSCertificates: strings.TrimSpace(input.CustomTLSCertificates),
		IgnoreTLSErrors:       input.IgnoreTLSErrors,
		ProxyEnabled:          input.ProxyEnabled, ProxyURL: strings.TrimSpace(input.ProxyURL),
		ProxyTimeoutSeconds: proxyTimeout, FailSafe: failSafe,
	}
	if _, err := buildTransport(HTTPClientConfig{
		CustomTLSCertificates: connection.CustomTLSCertificates,
		IgnoreTLSErrors:       connection.IgnoreTLSErrors,
		ProxyEnabled:          connection.ProxyEnabled, ProxyURL: connection.ProxyURL,
		ProxyTimeoutSeconds: connection.ProxyTimeoutSeconds,
	}); err != nil {
		return Connection{}, err
	}
	return connection, nil
}

func connectionTargetsMatch(left Connection, right Connection) bool {
	return left.Username == right.Username && JoinWebDAVURL(left.Endpoint, left.RemoteRoot) == JoinWebDAVURL(right.Endpoint, right.RemoteRoot)
}

func copyConnectionIdentity(target *Connection, source Connection) {
	target.VaultID = source.VaultID
	target.HeadManifestHash = source.HeadManifestHash
	target.HeadETag = source.HeadETag
	target.LastSyncAt = source.LastSyncAt
	target.HasLastSync = source.HasLastSync
}

func (s *Service) inspectRemote(ctx context.Context, connection Connection, secret string) (remoteInspection, error) {
	client, err := s.clientFactory(connection, secret)
	if err != nil {
		return remoteInspection{}, err
	}
	rootResponse, err := client.Propfind(ctx, "", "0")
	if err != nil {
		return remoteInspection{}, classifyRemoteError(err)
	}
	if rootResponse.StatusCode != http.StatusMultiStatus {
		return remoteInspection{}, ErrInvalidRemoteFormat
	}
	formatResponse, err := client.Get(ctx, formatPath)
	if err != nil {
		if formatResponse.StatusCode == http.StatusNotFound {
			return remoteInspection{}, nil
		}
		return remoteInspection{}, classifyRemoteError(err)
	}
	var format FormatDocument
	if err := json.Unmarshal(formatResponse.Body, &format); err != nil || validateFormat(format) != nil {
		return remoteInspection{}, ErrInvalidRemoteFormat
	}
	connection.VaultID = format.VaultID
	state, err := s.loadRemoteState(ctx, client, connection, false)
	if err != nil {
		return remoteInspection{}, err
	}
	return remoteInspection{initialized: true, state: state}, nil
}

func (s *Service) requireEmptyLocalVault(ctx context.Context) error {
	count, err := s.repository.CountOutbox(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRemoteImportNotEmpty
	}
	if s.notes == nil {
		return nil
	}
	changes, err := s.notes.ExportSyncChanges(ctx)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return ErrRemoteImportNotEmpty
	}
	return nil
}

func (s *Service) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	connection, err := s.repository.GetConnection(ctx)
	if err != nil {
		return err
	}
	if connection != nil && connection.CredentialRef != "" {
		if err := s.credentials.Delete(connection.CredentialRef); err != nil {
			return err
		}
	}
	return s.repository.DeleteConnection(ctx)
}

type SyncNowInput struct {
	InitializeRemote bool `json:"initializeRemote"`
	ImportRemote     bool `json:"importRemote"`
	ForceRetry       bool `json:"forceRetry"`
}

func (s *Service) SyncNow(ctx context.Context, input SyncNowInput) (SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, unlockNotes := s.lockNotesForSync(ctx)
	defer unlockNotes()

	connection, err := s.repository.GetConnection(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	if connection == nil {
		return SyncResult{Status: StatusDisabled, Message: "sync is not configured"}, nil
	}
	if err := s.repository.UpdateConnectionStatus(ctx, StatusSyncing, ""); err != nil {
		return SyncResult{}, err
	}
	secret, err := s.credentials.Get(connection.CredentialRef)
	if err != nil {
		_ = s.repository.UpdateConnectionStatus(context.Background(), StatusAuthRequired, "")
		return SyncResult{Status: StatusAuthRequired, Message: "sync credentials are unavailable"}, ErrCredentialsUnavailable
	}
	client, err := s.clientFactory(*connection, secret)
	if err != nil {
		_ = s.repository.UpdateConnectionStatus(context.Background(), StatusFailed, "")
		return SyncResult{Status: StatusFailed, Message: "sync endpoint is invalid"}, err
	}
	if input.ForceRetry {
		if err := s.repository.ResetRetryableOutbox(ctx); err != nil {
			_ = s.repository.UpdateConnectionStatus(context.Background(), StatusFailed, "")
			return SyncResult{Status: StatusFailed, Message: "sync retry state could not be reset"}, err
		}
	}

	initializeRemote := input.InitializeRemote || (!connection.HasLastSync && connection.HeadManifestHash == "")
	remote, err := s.loadRemoteState(ctx, client, *connection, initializeRemote)
	if err != nil {
		status := statusForError(err)
		_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
		return SyncResult{Status: status, Message: userMessage(err)}, err
	}

	uploaded, downloaded, conflicts, err := s.syncOutbox(ctx, client, *connection, remote)
	if err != nil {
		if !errors.Is(err, ErrHeadPrecondition) {
			status := statusForError(err)
			_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
			return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Message: userMessage(err)}, err
		}
		// Re-read the head before reconciling a 412. The second pass performs
		// entity-level comparison and is not a blind repeat of the failed PUT.
		reconciledRemote, reconcileLoadErr := s.loadRemoteState(ctx, client, *connection, false)
		if reconcileLoadErr != nil {
			status := statusForError(reconcileLoadErr)
			_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
			return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Message: userMessage(reconcileLoadErr)}, reconcileLoadErr
		}
		extraUploaded, extraDownloaded, extraConflicts, reconcileErr := s.syncOutbox(ctx, client, *connection, reconciledRemote)
		uploaded += extraUploaded
		downloaded += extraDownloaded
		conflicts += extraConflicts
		if reconcileErr != nil {
			status := statusForError(reconcileErr)
			_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
			return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Message: userMessage(reconcileErr)}, reconcileErr
		}
	}

	remote, err = s.loadRemoteState(ctx, client, *connection, false)
	if err != nil {
		status := statusForError(err)
		_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
		return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Message: userMessage(err)}, err
	}
	if err := s.checkEmptyRemoteFailSafe(ctx, *connection, remote, input.ImportRemote); err != nil {
		status := statusForError(err)
		_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
		return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Message: userMessage(err)}, err
	}
	remoteDownloaded, remoteConflicts, err := s.pullRemote(ctx, client, remote, input.ImportRemote, connection.HeadManifestHash == "")
	if err != nil {
		status := statusForError(err)
		_ = s.repository.UpdateConnectionStatus(context.Background(), status, "")
		return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded + remoteDownloaded, Conflicts: conflicts + remoteConflicts, Message: userMessage(err)}, err
	}
	downloaded += remoteDownloaded
	conflicts += remoteConflicts
	if err := s.repository.UpdateConnectionHead(ctx, remote.head.ManifestHash, remote.headETag, resultStatus(conflicts)); err != nil {
		return SyncResult{}, err
	}
	remaining, err := s.repository.CountOutbox(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	status := resultStatus(conflicts)
	if remaining > 0 && status == StatusSynced {
		status = StatusPending
	}
	return SyncResult{Status: status, Uploaded: uploaded, Downloaded: downloaded, Conflicts: conflicts, Remaining: remaining}, nil
}

func (s *Service) checkEmptyRemoteFailSafe(ctx context.Context, connection Connection, remote remoteState, remoteAuthoritative bool) error {
	if !connection.FailSafe || len(remote.manifest.Entries) > 0 {
		return nil
	}
	if !connection.HasLastSync && connection.HeadManifestHash == "" && !remoteAuthoritative {
		return nil
	}
	if s.notes == nil {
		return nil
	}
	changes, err := s.notes.ExportSyncChanges(ctx)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return ErrFailSafeTriggered
	}
	return nil
}

func (s *Service) ResolveConflict(ctx context.Context, input ConflictResolutionInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, unlockNotes := s.lockNotesForSync(ctx)
	defer unlockNotes()

	conflict, err := s.repository.GetConflict(ctx, input.ConflictID)
	if err != nil {
		return err
	}
	if conflict == nil || conflict.ResolutionStatus != "open" {
		return errors.New("sync conflict is not open")
	}
	switch input.Choice {
	case "remote":
		if err := s.applyRemoteObject(ctx, []byte(conflict.RemoteSnapshot)); err != nil {
			return err
		}
		if err := s.repository.DeleteOutboxEntity(ctx, conflict.EntityKey); err != nil {
			return err
		}
		if err := s.repository.MarkItemRemote(ctx, conflict.EntityKey, conflict.EntityType, conflict.RemoteObjectHash, conflict.RemoteSnapshot); err != nil {
			return err
		}
	case "local":
		if err := s.repository.RequeueConflictLocal(ctx, *conflict); err != nil {
			return err
		}
	case "both":
		if conflict.EntityType != note.SyncEntityNote {
			return errors.New("keeping both sides is supported for note conflicts only")
		}
		if err := s.repository.RequeueConflictLocal(ctx, *conflict); err != nil {
			return err
		}
		if err := s.keepBothNoteVersions(ctx, *conflict); err != nil {
			return err
		}
	default:
		return errors.New("sync conflict choice must be local, remote, or both")
	}
	return s.repository.ResolveConflict(ctx, conflict.ID)
}

func (s *Service) keepBothNoteVersions(ctx context.Context, conflict Conflict) error {
	object, err := decodeObject([]byte(conflict.RemoteSnapshot))
	if err != nil || object.Deleted {
		return ErrInvalidRemoteFormat
	}
	var payload note.SyncNotePayload
	if err := json.Unmarshal(object.Payload, &payload); err != nil {
		return ErrInvalidRemoteFormat
	}
	// A retry after the note was created but before the conflict row was marked
	// resolved must update the same copy instead of creating duplicates.
	id := hashBytes([]byte("conflict-remote-copy:" + conflict.ID))[:32]
	payload.ID = id
	if err := s.notes.ApplySyncNote(ctx, payload); err != nil {
		return err
	}
	createdAt := parseTimestamp(payload.CreatedAt)
	updatedAt := parseTimestamp(payload.UpdatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	change, err := note.NewNoteSyncChange(conflict.ID+"-remote", note.Record{
		ID:         id,
		NotebookID: payload.NotebookID,
		Title:      payload.Title,
		IsFavorite: payload.IsFavorite,
		IsPinned:   payload.IsPinned,
		IsTrashed:  payload.IsTrashed,
		Revision:   1,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, payload.Content)
	if err != nil {
		return err
	}
	return s.repository.EnqueueChanges(ctx, []note.SyncChange{change})
}

func (s *Service) ListConflicts(ctx context.Context) ([]ConflictSummary, error) {
	conflicts, err := s.repository.ListConflicts(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]ConflictSummary, 0, len(conflicts))
	for _, conflict := range conflicts {
		summaries = append(summaries, conflict.Summary())
	}
	return summaries, nil
}

type remoteState struct {
	format   FormatDocument
	head     HeadDocument
	headETag string
	manifest ManifestDocument
	entries  map[string]ManifestEntry
}

func (s *Service) loadRemoteState(ctx context.Context, client RemoteClient, connection Connection, initialize bool) (remoteState, error) {
	formatResponse, formatErr := client.Get(ctx, formatPath)
	if formatErr != nil && formatResponse.StatusCode != http.StatusNotFound {
		return remoteState{}, classifyRemoteError(formatErr)
	}
	if formatResponse.StatusCode == http.StatusNotFound {
		if !initialize {
			return remoteState{}, ErrRemoteNotInitialized
		}
		if err := initializeRemote(ctx, client, connection.VaultID); err != nil {
			return remoteState{}, err
		}
		formatResponse, formatErr = client.Get(ctx, formatPath)
		if formatErr != nil {
			return remoteState{}, classifyRemoteError(formatErr)
		}
	}
	var format FormatDocument
	if err := json.Unmarshal(formatResponse.Body, &format); err != nil {
		return remoteState{}, ErrInvalidRemoteFormat
	}
	if err := validateFormat(format); err != nil {
		return remoteState{}, err
	}
	if connection.VaultID != format.VaultID {
		return remoteState{}, ErrVaultMismatch
	}

	headResponse, headErr := getResponseWithStrongETag(ctx, client, headPath)
	if headErr != nil {
		return remoteState{}, classifyRemoteError(headErr)
	}
	if err := requireStrongETag(headResponse.ETag); err != nil {
		return remoteState{}, err
	}
	var head HeadDocument
	if err := json.Unmarshal(headResponse.Body, &head); err != nil || validateHead(head, format) != nil {
		return remoteState{}, ErrInvalidRemoteFormat
	}
	manifestResponse, manifestErr := client.Get(ctx, manifestPath(head.ManifestHash))
	if manifestErr != nil {
		return remoteState{}, classifyRemoteError(manifestErr)
	}
	var manifest ManifestDocument
	if err := json.Unmarshal(manifestResponse.Body, &manifest); err != nil {
		return remoteState{}, ErrInvalidRemoteFormat
	}
	if err := validateManifest(manifest, format, head.ManifestHash); err != nil {
		return remoteState{}, err
	}
	if manifest.Generation != head.Generation {
		return remoteState{}, ErrInvalidRemoteFormat
	}
	entries := make(map[string]ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entries[entry.EntityKey] = entry
	}
	return remoteState{format: format, head: head, headETag: headResponse.ETag, manifest: manifest, entries: entries}, nil
}

func initializeRemote(ctx context.Context, client RemoteClient, vaultID string) error {
	if err := validateFormat(FormatDocument{FormatVersion: FormatVersion, VaultID: vaultID}); err != nil {
		return err
	}
	if err := client.Mkcol(ctx, ".atlasnote"); err != nil {
		return err
	}
	if err := client.Mkcol(ctx, manifestDir); err != nil {
		return err
	}
	if err := client.Mkcol(ctx, objectDir); err != nil {
		return err
	}
	format, err := canonicalJSON(FormatDocument{FormatVersion: FormatVersion, VaultID: vaultID})
	if err != nil {
		return err
	}
	if _, err := client.Put(ctx, formatPath, format, "", "*"); err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return err
		}
	}
	emptyManifest, err := canonicalJSON(manifestFor(vaultID, 0, map[string]ManifestEntry{}))
	if err != nil {
		return err
	}
	manifestHash := hashBytes(emptyManifest)
	if _, err := client.Put(ctx, manifestPath(manifestHash), emptyManifest, "", "*"); err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return err
		}
	}
	head, err := canonicalJSON(HeadDocument{FormatVersion: FormatVersion, VaultID: vaultID, Generation: 0, ManifestHash: manifestHash})
	if err != nil {
		return err
	}
	_, err = client.Put(ctx, headPath, head, "", "*")
	if err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return err
		}
		return nil
	}
	return nil
}

func (s *Service) syncOutbox(ctx context.Context, client RemoteClient, connection Connection, remote remoteState) (int, int, int, error) {
	items, err := s.repository.ListOutbox(ctx, 1000)
	if err != nil {
		return 0, 0, 0, err
	}
	uploaded, downloaded, conflicts := 0, 0, 0
	for _, item := range items {
		current, err := s.loadRemoteState(ctx, client, connection, false)
		if err != nil {
			return uploaded, downloaded, conflicts, err
		}
		remoteHash := ""
		if entry, ok := current.entries[item.EntityKey]; ok {
			remoteHash = entry.ObjectHash
		}
		state, err := s.repository.GetItemState(ctx, item.EntityKey)
		if err != nil {
			return uploaded, downloaded, conflicts, err
		}
		if state != nil && state.LocalObjectHash != state.BaseObjectHash && remoteHash != "" && remoteHash != state.BaseObjectHash && remoteHash != state.LocalObjectHash {
			remoteObject, objectErr := s.fetchRemoteObject(ctx, client, current.entries[item.EntityKey])
			if objectErr != nil {
				return uploaded, downloaded, conflicts, objectErr
			}
			baseSnapshot, snapshotErr := baseSnapshotForState(ctx, s.repository, *state)
			if snapshotErr != nil {
				return uploaded, downloaded, conflicts, snapshotErr
			}
			conflict := Conflict{
				EntityKey:        item.EntityKey,
				EntityType:       item.EntityType,
				LocalObjectHash:  item.ObjectHash,
				BaseObjectHash:   state.BaseObjectHash,
				RemoteObjectHash: remoteHash,
				LocalSnapshot:    item.ObjectJSON,
				BaseSnapshot:     baseSnapshot,
				RemoteSnapshot:   string(remoteObject),
				ConflictType:     "both-changed",
				ResolutionStatus: "open",
			}
			if err := s.repository.CreateConflict(ctx, conflict); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			if err := s.repository.MarkOutboxRetry(ctx, item.Sequence, item.AttemptCount, time.Now().UTC().Add(24*time.Hour), FailedClassConflict); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			conflicts++
			continue
		}
		if state != nil && state.LocalObjectHash == state.BaseObjectHash && remoteHash != "" && remoteHash != state.BaseObjectHash {
			object, objectErr := s.fetchRemoteObject(ctx, client, current.entries[item.EntityKey])
			if objectErr != nil {
				return uploaded, downloaded, conflicts, objectErr
			}
			if err := s.applyRemoteObject(ctx, object); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			if err := s.repository.DeleteOutbox(ctx, item.Sequence); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			if err := s.repository.MarkItemRemote(ctx, item.EntityKey, item.EntityType, remoteHash, string(object)); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			downloaded++
			continue
		}
		if remoteHash == item.ObjectHash {
			if err := s.repository.DeleteOutbox(ctx, item.Sequence); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			if err := s.repository.MarkItemSynced(ctx, item.EntityKey, item.EntityType, item.ObjectHash, item.ObjectJSON); err != nil {
				return uploaded, downloaded, conflicts, err
			}
			continue
		}
		if err := s.uploadOne(ctx, client, current, item); err != nil {
			if errors.Is(err, ErrHeadPrecondition) {
				return uploaded, downloaded, conflicts, err
			}
			if shouldRetry(err) {
				if item.AttemptCount < 3 {
					if markErr := s.repository.MarkOutboxRetry(ctx, item.Sequence, item.AttemptCount+1, time.Now().UTC().Add(retryDelay(item.AttemptCount+1)), retryClass(err)); markErr != nil {
						return uploaded, downloaded, conflicts, markErr
					}
					continue
				}
				if markErr := s.repository.MarkOutboxFailed(ctx, item.Sequence, retryClass(err)); markErr != nil {
					return uploaded, downloaded, conflicts, markErr
				}
				return uploaded, downloaded, conflicts, ErrRetryLimitReached
			}
			if markErr := s.repository.MarkOutboxFailed(ctx, item.Sequence, failedClassForError(err)); markErr != nil {
				return uploaded, downloaded, conflicts, markErr
			}
			return uploaded, downloaded, conflicts, err
		}
		if err := s.repository.DeleteOutbox(ctx, item.Sequence); err != nil {
			return uploaded, downloaded, conflicts, err
		}
		if err := s.repository.MarkItemSynced(ctx, item.EntityKey, item.EntityType, item.ObjectHash, item.ObjectJSON); err != nil {
			return uploaded, downloaded, conflicts, err
		}
		uploaded++
	}
	return uploaded, downloaded, conflicts, nil
}

func (s *Service) uploadOne(ctx context.Context, client RemoteClient, remote remoteState, item OutboxItem) error {
	objectBytes := []byte(item.ObjectJSON)
	if err := ensureObjectHash(objectBytes, item.ObjectHash); err != nil {
		return err
	}
	response, err := client.Put(ctx, objectPath(item.ObjectHash), objectBytes, "", "*")
	if err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return classifyRemoteError(err)
		}
		existing, getErr := client.Get(ctx, objectPath(item.ObjectHash))
		if getErr != nil || hashBytes(existing.Body) != item.ObjectHash {
			return ErrInvalidRemoteFormat
		}
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	verifiedObject, err := client.Get(ctx, objectPath(item.ObjectHash))
	if err != nil {
		return classifyRemoteError(err)
	}
	if err := ensureObjectHash(verifiedObject.Body, item.ObjectHash); err != nil {
		return err
	}

	entries := make(map[string]ManifestEntry, len(remote.entries)+1)
	for key, entry := range remote.entries {
		entries[key] = entry
	}
	entries[item.EntityKey] = ManifestEntry{EntityKey: item.EntityKey, EntityType: item.EntityType, ObjectHash: item.ObjectHash}
	manifest := manifestFor(remote.format.VaultID, remote.head.Generation+1, entries)
	manifestBytes, err := canonicalJSON(manifest)
	if err != nil {
		return err
	}
	manifestHash := hashBytes(manifestBytes)
	if _, err := client.Put(ctx, manifestPath(manifestHash), manifestBytes, "", "*"); err != nil {
		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusPreconditionFailed {
			return classifyRemoteError(err)
		}
	}
	verifiedManifest, err := client.Get(ctx, manifestPath(manifestHash))
	if err != nil {
		return classifyRemoteError(err)
	}
	var verifiedManifestDocument ManifestDocument
	if err := json.Unmarshal(verifiedManifest.Body, &verifiedManifestDocument); err != nil ||
		validateManifest(verifiedManifestDocument, remote.format, manifestHash) != nil {
		return ErrInvalidRemoteFormat
	}
	head := HeadDocument{FormatVersion: FormatVersion, VaultID: remote.format.VaultID, Generation: manifest.Generation, ManifestHash: manifestHash}
	headBytes, err := canonicalJSON(head)
	if err != nil {
		return err
	}
	response, err = client.Put(ctx, headPath, headBytes, remote.headETag, "")
	if err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusPreconditionFailed {
			return ErrHeadPrecondition
		}
		return classifyRemoteError(err)
	}
	verified, err := getResponseWithStrongETag(ctx, client, headPath)
	if err != nil {
		return classifyRemoteError(err)
	}
	var verifiedHead HeadDocument
	if err := json.Unmarshal(verified.Body, &verifiedHead); err != nil || validateHead(verifiedHead, remote.format) != nil {
		return ErrInvalidRemoteFormat
	}
	if err := requireStrongETag(verified.ETag); err != nil {
		return err
	}
	if verifiedHead != head {
		return ErrHeadPrecondition
	}
	return nil
}

func (s *Service) pullRemote(ctx context.Context, client RemoteClient, remote remoteState, importRemote bool, initialImport bool) (int, int, error) {
	states, err := s.repository.ListItemStates(ctx)
	if err != nil {
		return 0, 0, err
	}
	stateByKey := make(map[string]ItemState, len(states))
	for _, state := range states {
		stateByKey[state.EntityKey] = state
	}
	orderedEntries, prefetchedObjects, err := s.orderRemoteEntries(ctx, client, remote)
	if err != nil {
		return 0, 0, err
	}
	fetchObject := func(entry ManifestEntry) ([]byte, error) {
		if object, ok := prefetchedObjects[entry.EntityKey]; ok {
			return object, nil
		}
		return s.fetchRemoteObject(ctx, client, entry)
	}
	downloaded, conflicts := 0, 0
	for _, entry := range orderedEntries {
		key := entry.EntityKey
		state, exists := stateByKey[key]
		if !exists && initialImport && !importRemote {
			return downloaded, conflicts, errors.New("remote import requires explicit confirmation")
		}
		if exists && state.RemoteObjectHash == entry.ObjectHash && state.BaseObjectHash == entry.ObjectHash {
			continue
		}
		if exists && state.LocalObjectHash != state.BaseObjectHash && state.LocalObjectHash != entry.ObjectHash && state.BaseObjectHash != entry.ObjectHash {
			object, err := fetchObject(entry)
			if err != nil {
				return downloaded, conflicts, err
			}
			baseSnapshot, snapshotErr := baseSnapshotForState(ctx, s.repository, state)
			if snapshotErr != nil {
				return downloaded, conflicts, snapshotErr
			}
			if err := s.repository.CreateConflict(ctx, Conflict{
				EntityKey: key, EntityType: entry.EntityType,
				LocalObjectHash: state.LocalObjectHash, BaseObjectHash: state.BaseObjectHash,
				RemoteObjectHash: entry.ObjectHash, LocalSnapshot: state.SnapshotJSON,
				BaseSnapshot: baseSnapshot, RemoteSnapshot: string(object),
				ConflictType: "both-changed", ResolutionStatus: "open",
			}); err != nil {
				return downloaded, conflicts, err
			}
			conflicts++
			continue
		}
		object, err := fetchObject(entry)
		if err != nil {
			return downloaded, conflicts, err
		}
		if err := s.applyRemoteObject(ctx, object); err != nil {
			return downloaded, conflicts, err
		}
		if err := s.repository.MarkItemRemote(ctx, key, entry.EntityType, entry.ObjectHash, string(object)); err != nil {
			return downloaded, conflicts, err
		}
		downloaded++
	}
	return downloaded, conflicts, nil
}

func (s *Service) orderRemoteEntries(ctx context.Context, client RemoteClient, remote remoteState) ([]ManifestEntry, map[string][]byte, error) {
	entries := make([]ManifestEntry, 0, len(remote.entries))
	prefetched := make(map[string][]byte)
	parents := make(map[string]string)
	for key, entry := range remote.entries {
		entries = append(entries, entry)
		if entry.EntityType != note.SyncEntityNotebook {
			continue
		}
		object, err := s.fetchRemoteObject(ctx, client, entry)
		if err != nil {
			return nil, nil, err
		}
		prefetched[key] = object
		decoded, err := decodeObject(object)
		if err != nil {
			return nil, nil, err
		}
		if decoded.Deleted {
			continue
		}
		var payload note.SyncNotebookPayload
		if err := json.Unmarshal(decoded.Payload, &payload); err != nil || !isEntityID(payload.ID) {
			return nil, nil, ErrInvalidRemoteFormat
		}
		if payload.ParentID != nil && *payload.ParentID != "" {
			if !isEntityID(*payload.ParentID) || *payload.ParentID == payload.ID {
				return nil, nil, ErrInvalidRemoteFormat
			}
			parents[key] = note.SyncEntityKey(note.SyncEntityNotebook, *payload.ParentID)
		}
	}

	var depth func(string, map[string]bool) int
	depth = func(key string, visiting map[string]bool) int {
		if visiting[key] {
			return 0
		}
		parent, ok := parents[key]
		if !ok {
			return 0
		}
		visiting[key] = true
		value := depth(parent, visiting) + 1
		delete(visiting, key)
		return value
	}
	sort.SliceStable(entries, func(i, j int) bool {
		leftRank := syncEntityRank(entries[i].EntityType)
		rightRank := syncEntityRank(entries[j].EntityType)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if entries[i].EntityType == note.SyncEntityNotebook {
			leftDepth := depth(entries[i].EntityKey, make(map[string]bool))
			rightDepth := depth(entries[j].EntityKey, make(map[string]bool))
			if leftDepth != rightDepth {
				return leftDepth < rightDepth
			}
		}
		return entries[i].EntityKey < entries[j].EntityKey
	})
	return entries, prefetched, nil
}

func syncEntityRank(entityType string) int {
	switch entityType {
	case note.SyncEntityNotebook:
		return 0
	case note.SyncEntityTag:
		return 1
	case note.SyncEntityNote:
		return 2
	case note.SyncEntityNoteTags:
		return 3
	default:
		return 4
	}
}

func (s *Service) fetchRemoteObject(ctx context.Context, client RemoteClient, entry ManifestEntry) ([]byte, error) {
	response, err := client.Get(ctx, objectPath(entry.ObjectHash))
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	if err := ensureObjectHash(response.Body, entry.ObjectHash); err != nil {
		return nil, err
	}
	object, err := decodeObject(response.Body)
	if err != nil || object.EntityKey != entry.EntityKey || object.EntityType != entry.EntityType {
		return nil, ErrInvalidRemoteFormat
	}
	return response.Body, nil
}

func (s *Service) applyRemoteObject(ctx context.Context, raw []byte) error {
	return applyRemoteObjectTo(ctx, s.notes, raw)
}

func applyRemoteObjectTo(ctx context.Context, notes *note.Service, raw []byte) error {
	if notes == nil {
		return errors.New("note service is unavailable")
	}
	object, err := decodeObject(raw)
	if err != nil {
		return err
	}
	if object.Deleted {
		switch object.EntityType {
		case note.SyncEntityNote:
			id, err := syncEntityID(object, note.SyncEntityNote)
			if err != nil {
				return err
			}
			return notes.DeleteSyncNote(ctx, id)
		case note.SyncEntityNotebook:
			id, err := syncEntityID(object, note.SyncEntityNotebook)
			if err != nil {
				return err
			}
			return notes.DeleteSyncNotebook(ctx, id)
		case note.SyncEntityTag:
			id, err := syncEntityID(object, note.SyncEntityTag)
			if err != nil {
				return err
			}
			return notes.DeleteSyncTag(ctx, id)
		case note.SyncEntityNoteTags:
			id, err := syncEntityID(object, note.SyncEntityNoteTags)
			if err != nil {
				return err
			}
			return notes.DeleteSyncNoteTags(ctx, id)
		default:
			return ErrInvalidRemoteFormat
		}
	}
	switch object.EntityType {
	case note.SyncEntityNote:
		var payload note.SyncNotePayload
		if err := json.Unmarshal(object.Payload, &payload); err != nil {
			return ErrInvalidRemoteFormat
		}
		if !isEntityID(payload.ID) || object.EntityKey != note.SyncEntityKey(note.SyncEntityNote, payload.ID) ||
			(payload.NotebookID != nil && !isEntityID(*payload.NotebookID)) {
			return ErrInvalidRemoteFormat
		}
		return notes.ApplySyncNote(ctx, payload)
	case note.SyncEntityNotebook:
		var payload note.SyncNotebookPayload
		if err := json.Unmarshal(object.Payload, &payload); err != nil {
			return ErrInvalidRemoteFormat
		}
		if !isEntityID(payload.ID) || object.EntityKey != note.SyncEntityKey(note.SyncEntityNotebook, payload.ID) ||
			(payload.ParentID != nil && (!isEntityID(*payload.ParentID) || *payload.ParentID == payload.ID)) {
			return ErrInvalidRemoteFormat
		}
		return notes.ApplySyncNotebook(ctx, payload)
	case note.SyncEntityTag:
		var payload note.SyncTagPayload
		if err := json.Unmarshal(object.Payload, &payload); err != nil {
			return ErrInvalidRemoteFormat
		}
		if !isEntityID(payload.ID) || object.EntityKey != note.SyncEntityKey(note.SyncEntityTag, payload.ID) {
			return ErrInvalidRemoteFormat
		}
		return notes.ApplySyncTag(ctx, payload)
	case note.SyncEntityNoteTags:
		var payload note.SyncNoteTagsPayload
		if err := json.Unmarshal(object.Payload, &payload); err != nil {
			return ErrInvalidRemoteFormat
		}
		if !isEntityID(payload.NoteID) || object.EntityKey != note.SyncEntityKey(note.SyncEntityNoteTags, payload.NoteID) {
			return ErrInvalidRemoteFormat
		}
		for index, tagID := range payload.TagIDs {
			if !isEntityID(tagID) || (index > 0 && payload.TagIDs[index-1] >= tagID) {
				return ErrInvalidRemoteFormat
			}
		}
		return notes.ApplySyncNoteTags(ctx, payload)
	default:
		return ErrInvalidRemoteFormat
	}
}

func syncEntityID(object ObjectDocument, entityType string) (string, error) {
	if object.EntityType != entityType {
		return "", ErrInvalidRemoteFormat
	}
	id, ok := entityIDFromKey(entityType, object.EntityKey)
	if !ok {
		return "", ErrInvalidRemoteFormat
	}
	return id, nil
}

func decodeObject(raw []byte) (ObjectDocument, error) {
	var object ObjectDocument
	if err := json.Unmarshal(raw, &object); err != nil {
		return ObjectDocument{}, ErrInvalidRemoteFormat
	}
	if object.FormatVersion != FormatVersion {
		return ObjectDocument{}, ErrInvalidRemoteFormat
	}
	if _, ok := entityIDFromKey(object.EntityType, object.EntityKey); !ok {
		return ObjectDocument{}, ErrInvalidRemoteFormat
	}
	if (!object.Deleted && len(object.Payload) == 0) || (object.Deleted && len(object.Payload) != 0) {
		return ObjectDocument{}, ErrInvalidRemoteFormat
	}
	return object, nil
}

func manifestPath(hash string) string { return manifestDir + "/" + hash + ".json" }
func objectPath(hash string) string   { return objectDir + "/" + hash + ".json" }

func requireStrongETag(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(strings.ToLower(value), "w/") || !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") {
		return ErrMissingStrongETag
	}
	return nil
}

func statusForError(err error) Status {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusUnauthorized {
		return StatusAuthRequired
	}
	switch {
	case errors.Is(err, ErrCredentialsUnavailable):
		return StatusAuthRequired
	case errors.Is(err, ErrRetryLimitReached):
		return StatusFailed
	case errors.Is(err, ErrHeadPrecondition):
		return StatusConflict
	case errors.Is(err, ErrRemoteNotInitialized), errors.Is(err, ErrInvalidRemoteFormat), errors.Is(err, ErrVaultMismatch):
		return StatusFailed
	case shouldRetry(err):
		return StatusOffline
	default:
		return StatusFailed
	}
}

func resultStatus(conflicts int) Status {
	if conflicts > 0 {
		return StatusConflict
	}
	return StatusSynced
}

func classifyRemoteError(err error) error {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("sync authentication failed: %w", err)
		case http.StatusForbidden:
			return fmt.Errorf("sync permission denied: %w", err)
		case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return fmt.Errorf("sync remote service is temporarily unavailable: %w", err)
		}
	}
	return err
}

func shouldRetry(err error) bool {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusRequestTimeout || statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) == false && strings.Contains(strings.ToLower(err.Error()), "webdav request")
}

func retryClass(err error) FailedClass {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusTooManyRequests:
			return FailedClassRateLimit
		case http.StatusRequestTimeout:
			return FailedClassTimeout
		default:
			if statusErr.StatusCode >= 500 {
				return FailedClassServer
			}
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailedClassTimeout
	}
	return FailedClassNetwork
}

func failedClassForError(err error) FailedClass {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized:
			return FailedClassAuth
		case http.StatusForbidden:
			return FailedClassPermission
		case http.StatusTooManyRequests:
			return FailedClassRateLimit
		case http.StatusRequestTimeout:
			return FailedClassTimeout
		default:
			if statusErr.StatusCode >= 500 {
				return FailedClassServer
			}
		}
	}
	if errors.Is(err, ErrInvalidRemoteFormat) {
		return FailedClassFormat
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailedClassTimeout
	}
	if errors.Is(err, ErrHeadPrecondition) {
		return FailedClassConflict
	}
	return FailedClassNetwork
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 15 * time.Second
	case 2:
		return time.Minute
	default:
		return 5 * time.Minute
	}
}

func userMessage(err error) string {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized:
			return "sync authentication failed; re-enter the WebDAV password"
		case http.StatusForbidden:
			return "sync permission was denied by the WebDAV server"
		}
	}
	switch {
	case errors.Is(err, ErrCredentialsUnavailable):
		return "sync credentials are unavailable"
	case errors.Is(err, ErrRemoteNotInitialized):
		return "remote vault is not initialized"
	case errors.Is(err, ErrRemoteImportNotEmpty):
		return "remote import requires an empty local vault"
	case errors.Is(err, ErrRetryLimitReached):
		return "sync retry limit reached; use manual sync to retry"
	case errors.Is(err, ErrVaultMismatch):
		return "remote vault does not match this local vault"
	case errors.Is(err, ErrInvalidRemoteFormat):
		return "remote vault format is invalid"
	case errors.Is(err, ErrHeadPrecondition):
		return "remote vault changed; sync was stopped for reconciliation"
	case errors.Is(err, ErrFailSafeTriggered):
		return "sync stopped because the remote target is empty while local data exists"
	default:
		return "sync failed"
	}
}

func parseTimestamp(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func baseSnapshotForState(ctx context.Context, repository *Repository, state ItemState) (string, error) {
	if state.BaseObjectHash == "" {
		return "", nil
	}
	snapshot, err := repository.GetSnapshot(ctx, state.EntityKey, state.BaseObjectHash)
	if err != nil {
		return "", err
	}
	if snapshot == "" && state.BaseObjectHash == state.LocalObjectHash {
		snapshot = state.SnapshotJSON
	}
	return snapshot, nil
}
