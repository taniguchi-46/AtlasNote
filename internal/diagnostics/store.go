package diagnostics

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"atlasnote/internal/datalock"
)

const (
	SchemaVersion = 1
	MaxEvents     = 100
	MaxBytes      = 256 * 1024

	diagnosticsFile = "events.json"
	diagnosticsLock = "events.lock"
)

// Metadata contains build information that is safe to include in a local
// diagnostic record. It intentionally does not contain a path or environment
// value.
type Metadata struct {
	AppVersion  string
	VCSRevision string
}

// Event is the complete allowlist for persisted diagnostics. Do not add raw
// paths, filenames, usernames, note content, environment values, or errors to
// this type.
type Event struct {
	Schema        int    `json:"schema"`
	Timestamp     string `json:"timestamp"`
	DiagnosticID  string `json:"diagnosticId"`
	Operation     string `json:"operation"`
	Phase         string `json:"phase,omitempty"`
	Role          string `json:"role,omitempty"`
	Code          string `json:"code"`
	Reason        string `json:"reason,omitempty"`
	Stage         string `json:"stage,omitempty"`
	OSErrorNumber int    `json:"osErrorNumber,omitempty"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	AppVersion    string `json:"appVersion"`
	VCSRevision   string `json:"vcsRevision"`
}

type envelope struct {
	Schema int     `json:"schema"`
	Events []Event `json:"events"`
}

// Store is deliberately best-effort. A diagnostics failure must never become
// a second application failure, so Record only returns the generated event
// and keeps the bounded in-memory copy when disk persistence is unavailable.
type Store struct {
	mu          sync.Mutex
	root        string
	eventsPath  string
	lockPath    string
	metadata    Metadata
	events      []Event
	once        map[string]Event
	diskEnabled bool
}

func NewStore(root string, metadata Metadata) *Store {
	store := &Store{
		root:     cleanRoot(root),
		metadata: normalizeMetadata(metadata),
		events:   make([]Event, 0, MaxEvents),
		once:     make(map[string]Event),
	}
	if store.root == "" {
		return store
	}
	store.eventsPath = filepath.Join(store.root, diagnosticsFile)
	store.lockPath = filepath.Join(store.root, diagnosticsLock)
	if loaded, err := store.load(); err == nil {
		store.events = loaded
		store.diskEnabled = true
	}
	return store
}

func NewDefaultStore(metadata Metadata) *Store {
	cacheRoot, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheRoot) == "" {
		return NewStore("", metadata)
	}
	return NewStore(filepath.Join(cacheRoot, "AtlasNote", "diagnostics"), metadata)
}

func cleanRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return ""
	}
	return filepath.Clean(absolute)
}

func (s *Store) Record(event Event) Event {
	return s.record(event, "")
}

// RecordOnce records one event for a process-local boundary key and returns
// the original event for subsequent calls. This prevents status polling from
// producing duplicate records while retaining the same diagnostic ID for UI.
func (s *Store) RecordOnce(key string, event Event) Event {
	if strings.TrimSpace(key) == "" {
		return s.Record(event)
	}
	return s.record(event, key)
}

func (s *Store) record(event Event, onceKey string) Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if onceKey != "" {
		if existing, ok := s.once[onceKey]; ok {
			return existing
		}
	}

	normalized := s.normalizeEvent(event)
	s.events = append(s.events, normalized)
	s.trimEvents()
	if onceKey != "" {
		s.once[onceKey] = normalized
	}
	if s.diskEnabled {
		if err := s.persist([]Event{normalized}); err != nil {
			s.diskEnabled = false
		}
	}
	return normalized
}

func (s *Store) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}

func FormatReport(events []Event) string {
	safeEvents := make([]Event, 0, len(events))
	for _, event := range events {
		safeEvents = append(safeEvents, normalizeLoadedEvent(event))
	}
	encoded, err := json.MarshalIndent(envelope{Schema: SchemaVersion, Events: safeEvents}, "", "  ")
	if err != nil {
		return "{\"schema\":1,\"events\":[]}"
	}
	return string(append(encoded, '\n'))
}

func (s *Store) normalizeEvent(event Event) Event {
	event.Schema = SchemaVersion
	event.Timestamp = safeValue(event.Timestamp, time.Now().UTC().Format(time.RFC3339Nano))
	event.DiagnosticID = safeValue(event.DiagnosticID, newDiagnosticID())
	event.Operation = safeValue(event.Operation, "unknown-operation")
	event.Phase = safeValue(event.Phase, "")
	event.Role = safeValue(event.Role, "unknown")
	event.Code = safeValue(event.Code, "UNKNOWN")
	event.Reason = safeValue(event.Reason, "")
	event.Stage = safeValue(event.Stage, "")
	if event.OSErrorNumber < 0 {
		event.OSErrorNumber = 0
	}
	if event.OS == "" {
		event.OS = runtime.GOOS
	}
	if event.Arch == "" {
		event.Arch = runtime.GOARCH
	}
	event.OS = safeValue(event.OS, "unknown")
	event.Arch = safeValue(event.Arch, "unknown")
	event.AppVersion = safeValue(s.metadata.AppVersion, "unknown")
	event.VCSRevision = safeValue(s.metadata.VCSRevision, "unknown")
	return event
}

func (s *Store) trimEvents() {
	if len(s.events) > MaxEvents {
		s.events = append([]Event(nil), s.events[len(s.events)-MaxEvents:]...)
	}
	for len(s.events) > 0 {
		encoded, err := json.MarshalIndent(envelope{Schema: SchemaVersion, Events: s.events}, "", "  ")
		if err == nil && len(encoded)+1 <= MaxBytes {
			return
		}
		s.events = s.events[1:]
	}
}

func (s *Store) load() ([]Event, error) {
	if err := ensureSafeDirectory(s.root, false); err != nil {
		return nil, err
	}
	if err := checkOptionalSafeFile(s.lockPath); err != nil {
		return nil, err
	}
	// A missing directory has no history and cannot yet contain a lock.
	if _, err := os.Lstat(s.root); errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	lock, err := datalock.Acquire(s.lockPath)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	return s.loadLocked()
}

// loadLocked is called only while holding the cross-instance file lock.
func (s *Store) loadLocked() ([]Event, error) {
	info, err := os.Lstat(s.eventsPath)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil || !info.Mode().IsRegular() || isUnsafePath(s.eventsPath, info) || info.Size() > MaxBytes {
		return nil, errors.New("diagnostics file is not safe")
	}
	encoded, err := os.ReadFile(s.eventsPath)
	if err != nil {
		return nil, err
	}
	var stored envelope
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil || stored.Schema != SchemaVersion {
		return nil, errors.New("diagnostics file is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("diagnostics file is invalid")
	}
	if len(stored.Events) > MaxEvents {
		stored.Events = stored.Events[len(stored.Events)-MaxEvents:]
	}
	for index := range stored.Events {
		stored.Events[index] = normalizeLoadedEvent(stored.Events[index])
	}
	return stored.Events, nil
}

func (s *Store) persist(events []Event) error {
	if err := ensureSafeDirectory(s.root, true); err != nil {
		return err
	}
	if err := checkOptionalSafeFile(s.lockPath); err != nil {
		return err
	}
	lock, err := datalock.Acquire(s.lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()
	if info, err := os.Lstat(s.lockPath); err != nil || isUnsafePath(s.lockPath, info) || !info.Mode().IsRegular() {
		return errors.New("diagnostics lock is not safe")
	}
	latest, err := s.loadLocked()
	if err != nil {
		return err
	}
	// Merge only new records, never stale cached history: an old Store must
	// not resurrect records already removed by retention in another Store.
	seen := make(map[string]bool, len(latest)+len(events))
	merged := make([]Event, 0, len(latest)+len(events))
	for _, event := range append(latest, events...) {
		if !seen[event.DiagnosticID] {
			seen[event.DiagnosticID] = true
			merged = append(merged, event)
		}
	}
	bounded := &Store{events: merged}
	bounded.trimEvents()
	events = bounded.events
	encoded, err := json.MarshalIndent(envelope{Schema: SchemaVersion, Events: events}, "", "  ")
	if err != nil || len(encoded) > MaxBytes {
		return errors.New("diagnostics payload is too large")
	}
	encoded = append(encoded, '\n')
	if len(encoded) > MaxBytes {
		return errors.New("diagnostics payload is too large")
	}
	if err := writeAtomic(s.root, s.eventsPath, encoded); err != nil {
		return err
	}
	s.events = events
	return nil
}

func writeAtomic(root string, target string, contents []byte) error {
	temporary, err := os.CreateTemp(root, ".events-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(contents); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := renameAtomic(temporaryPath, target); err != nil {
		return err
	}
	committed = true
	return nil
}

func ensureSafeDirectory(root string, create bool) error {
	if strings.TrimSpace(root) == "" {
		return errors.New("diagnostics root is unavailable")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absolute = filepath.Clean(absolute)
	current := absolute
	missing := make([]string, 0)
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if isUnsafePath(current, info) || !info.IsDir() {
				return errors.New("diagnostics root is not a safe directory")
			}
			break
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			return errors.New("diagnostics root has no safe parent")
		}
		current = parent
	}
	if !create && len(missing) > 0 {
		return nil
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := os.Mkdir(missing[index], 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(missing[index])
		if err != nil || isUnsafePath(missing[index], info) || !info.IsDir() {
			return errors.New("diagnostics root is not a safe directory")
		}
	}
	return nil
}

func checkOptionalSafeFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || isUnsafePath(path, info) || !info.Mode().IsRegular() {
		return errors.New("diagnostics file is not safe")
	}
	return nil
}

func normalizeMetadata(metadata Metadata) Metadata {
	metadata.AppVersion = safeValue(metadata.AppVersion, "unknown")
	metadata.VCSRevision = safeValue(metadata.VCSRevision, "unknown")
	return metadata
}

func normalizeLoadedEvent(event Event) Event {
	event.Schema = SchemaVersion
	event.DiagnosticID = safeValue(event.DiagnosticID, newDiagnosticID())
	event.Timestamp = safeValue(event.Timestamp, "unknown")
	event.Operation = safeValue(event.Operation, "unknown-operation")
	event.Phase = safeValue(event.Phase, "")
	event.Role = safeValue(event.Role, "unknown")
	event.Code = safeValue(event.Code, "UNKNOWN")
	event.Reason = safeValue(event.Reason, "")
	event.Stage = safeValue(event.Stage, "")
	event.OS = safeValue(event.OS, "unknown")
	event.Arch = safeValue(event.Arch, "unknown")
	event.AppVersion = safeValue(event.AppVersion, "unknown")
	event.VCSRevision = safeValue(event.VCSRevision, "unknown")
	if event.OSErrorNumber < 0 {
		event.OSErrorNumber = 0
	}
	return event
}

func safeValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if len(value) > 128 {
		return value[:128]
	}
	for _, char := range value {
		if char < 0x20 || char == '\\' || char == '/' {
			return fallback
		}
	}
	return value
}

func newDiagnosticID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "diag-" + hex.EncodeToString(bytes[:])
	}
	return "diag-" + hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}
