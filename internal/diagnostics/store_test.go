package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"atlasnote/internal/datalock"
)

func TestStorePersistsLoadsAndBoundsEvents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "diagnostics")
	store := NewStore(root, Metadata{AppVersion: "0.1.0", VCSRevision: "revision"})
	for index := 0; index < MaxEvents+7; index++ {
		store.Record(Event{Operation: "storage-location.select", Code: "STORAGE_LOCATION_UNWRITABLE", Reason: "書き込み不可"})
	}
	if got := len(store.Events()); got != MaxEvents {
		t.Fatalf("events in memory = %d, want %d", got, MaxEvents)
	}
	encoded, err := os.ReadFile(filepath.Join(root, diagnosticsFile))
	if err != nil {
		t.Fatalf("read persisted events: %v", err)
	}
	if len(encoded) > MaxBytes {
		t.Fatalf("persisted events size = %d, want <= %d", len(encoded), MaxBytes)
	}
	var stored envelope
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatalf("decode persisted events: %v", err)
	}
	if len(stored.Events) != MaxEvents || stored.Schema != SchemaVersion {
		t.Fatalf("persisted envelope = %#v", stored)
	}
	loaded := NewStore(root, Metadata{AppVersion: "0.1.0", VCSRevision: "revision"})
	if len(loaded.Events()) != MaxEvents {
		t.Fatalf("reloaded events = %d, want %d", len(loaded.Events()), MaxEvents)
	}
	if loaded.Events()[0].DiagnosticID == "" || loaded.Events()[0].AppVersion != "0.1.0" {
		t.Fatalf("reloaded event metadata = %#v", loaded.Events()[0])
	}
}

func TestStoreRecordOnceReusesDiagnosticID(t *testing.T) {
	store := NewStore("", Metadata{AppVersion: "0.1.0"})
	first := store.RecordOnce("status|code|stage|role", Event{Operation: "storage-location.status", Code: "CODE"})
	second := store.RecordOnce("status|code|stage|role", Event{Operation: "storage-location.status", Code: "DIFFERENT"})
	if first.DiagnosticID == "" || second.DiagnosticID != first.DiagnosticID {
		t.Fatalf("record once IDs = %q, %q", first.DiagnosticID, second.DiagnosticID)
	}
	if len(store.Events()) != 1 || store.Events()[0].Code != "CODE" {
		t.Fatalf("record once events = %#v", store.Events())
	}
}

func TestStoreFallsBackToMemoryForCorruptOrLockedStorage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, diagnosticsFile), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt diagnostics: %v", err)
	}
	store := NewStore(root, Metadata{AppVersion: "0.1.0"})
	store.Record(Event{Operation: "storage-location.apply", Code: "CODE"})
	if len(store.Events()) != 1 {
		t.Fatalf("corrupt-file fallback events = %#v", store.Events())
	}
	if got, err := os.ReadFile(filepath.Join(root, diagnosticsFile)); err != nil || string(got) != "not-json" {
		t.Fatalf("corrupt file was rewritten: %q, %v", string(got), err)
	}

	lockedRoot := filepath.Join(t.TempDir(), "locked")
	lockedStore := NewStore(lockedRoot, Metadata{AppVersion: "0.1.0"})
	lock, err := datalock.Acquire(filepath.Join(lockedRoot, diagnosticsLock))
	if err != nil {
		t.Fatalf("acquire diagnostics lock: %v", err)
	}
	lockedStore.Record(Event{Operation: "storage-location.retry", Code: "LOCKED"})
	if err := lock.Release(); err != nil {
		t.Fatalf("release diagnostics lock: %v", err)
	}
	if len(lockedStore.Events()) != 1 {
		t.Fatalf("locked fallback events = %#v", lockedStore.Events())
	}
}

func TestStoreSanitizesReportAndSupportsConcurrentRecords(t *testing.T) {
	secretPath := filepath.Join("C:\\Users", "person", "secret.txt")
	store := NewStore("", Metadata{AppVersion: "0.1.0"})
	var group sync.WaitGroup
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			store.Record(Event{Operation: "storage-location.select", Code: "CODE", Reason: secretPath})
		}()
	}
	group.Wait()
	store.Record(Event{
		Schema:       99,
		Timestamp:    secretPath,
		DiagnosticID: secretPath,
		Operation:    "storage-location.select",
		Code:         "CODE",
	})
	report := FormatReport(store.Events())
	if strings.Contains(report, secretPath) || strings.Contains(report, "secret.txt") {
		t.Fatalf("report leaked path: %s", report)
	}
	if !strings.Contains(report, `"schema": 1`) {
		t.Fatalf("report missing schema: %s", report)
	}
	if len(store.Events()) != 17 || store.Events()[16].Schema != SchemaVersion {
		t.Fatalf("normalized records = %#v", store.Events()[16])
	}
}

func TestStoreUsesMemoryWhenRootIsUnsafe(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file-root")
	if err := os.WriteFile(root, []byte("root"), 0o600); err != nil {
		t.Fatalf("write file root: %v", err)
	}
	store := NewStore(root, Metadata{})
	store.Record(Event{Operation: "storage-location.status", Code: "CODE"})
	if len(store.Events()) != 1 {
		t.Fatalf("unsafe-root fallback events = %#v", store.Events())
	}
	if _, err := os.Stat(filepath.Join(root, diagnosticsFile)); err == nil {
		t.Fatalf("unexpected diagnostics file beside unsafe root: %v", err)
	}
}

func TestIndependentStoresMergeWithoutResurrectingExpiredEvents(t *testing.T) {
	root := t.TempDir()
	a, b := NewStore(root, Metadata{}), NewStore(root, Metadata{})
	ids := []string{}
	for i := 0; i < MaxEvents+5; i++ {
		s := a
		if i%2 == 1 {
			s = b
		}
		ids = append(ids, s.Record(Event{Code: "CODE"}).DiagnosticID)
		if i == 2 {
			loaded := NewStore(root, Metadata{}).Events()
			if len(loaded) != 3 {
				t.Fatalf("A-B-A lost a record: %d", len(loaded))
			}
			for index, event := range loaded {
				if event.DiagnosticID != ids[index] {
					t.Fatal("A-B-A record order changed")
				}
			}
		}
	}
	events := NewStore(root, Metadata{}).Events()
	if len(events) != MaxEvents {
		t.Fatalf("count %d", len(events))
	}
	seen := map[string]bool{}
	for i, e := range events {
		if seen[e.DiagnosticID] || e.DiagnosticID != ids[i+5] {
			t.Fatalf("lost, reordered or duplicate event at %d", i)
		}
		seen[e.DiagnosticID] = true
	}
}

func TestStoreDoesNotOverwriteFileCorruptedAfterInitialization(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root, Metadata{})
	p := filepath.Join(root, diagnosticsFile)
	if err := os.WriteFile(p, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	s.Record(Event{Code: "MEMORY"})
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "corrupt" {
		t.Fatalf("corruption overwritten: %q %v", got, err)
	}
	if len(s.Events()) != 1 {
		t.Fatal("memory fallback missing")
	}
}

func TestIndependentStoreLockConflictKeepsDiskAndMemory(t *testing.T) {
	root := t.TempDir()
	a := NewStore(root, Metadata{})
	first := a.Record(Event{Code: "FIRST"})
	b := NewStore(root, Metadata{})
	path := filepath.Join(root, diagnosticsFile)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := datalock.Acquire(filepath.Join(root, diagnosticsLock))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	second := b.Record(Event{Code: "MEMORY"})
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("locked disk changed")
	}
	events := b.Events()
	if len(events) != 2 || events[0].DiagnosticID != first.DiagnosticID || events[1].DiagnosticID != second.DiagnosticID {
		t.Fatal("fallback lost records")
	}
}

func TestStoreRejectsLinkInsertedAfterInitialization(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root, Metadata{})
	target := filepath.Join(t.TempDir(), "keep.json")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, diagnosticsFile)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	s.Record(Event{Code: "MEMORY"})
	b, err := os.ReadFile(target)
	if err != nil || string(b) != "keep" {
		t.Fatal("link target changed")
	}
	if len(s.Events()) != 1 {
		t.Fatal("memory record missing")
	}
	info, err := os.Lstat(filepath.Join(root, diagnosticsFile))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link replaced")
	}
}
