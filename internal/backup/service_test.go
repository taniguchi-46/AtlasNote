package backup

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

type backupTestWorkspace struct {
	root    string
	db      *sql.DB
	store   *storage.MarkdownStore
	locks   *contentlock.Manager
	notes   *note.Service
	service *Service
}

func newBackupTestWorkspace(t *testing.T) *backupTestWorkspace {
	t.Helper()
	root := t.TempDir()
	db, err := database.Open(t.Context(), filepath.Join(root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	store, err := storage.NewMarkdownStore(filepath.Join(root, "notes"))
	if err != nil {
		_ = db.Close()
		t.Fatalf("create test markdown store: %v", err)
	}
	locks := contentlock.NewManager(db, store)
	notes := note.NewService(note.NewRepository(db), store)
	notes.SetContentLockGuard(locks)
	paths := Paths{
		ManagementRoot: root,
		SpaceID:        "0123456789abcdef0123456789abcdef",
		DataDir:        root,
		DatabasePath:   filepath.Join(root, "atlasnote.db"),
		NotesDir:       filepath.Join(root, "notes"),
	}
	service := NewService(db, notes, locks, paths)
	workspace := &backupTestWorkspace{root: root, db: db, store: store, locks: locks, notes: notes, service: service}
	t.Cleanup(func() {
		service.Shutdown()
		locks.Close()
		_ = db.Close()
	})
	return workspace
}

func TestCreateAutomaticBackupAndDetectTampering(t *testing.T) {
	workspace := newBackupTestWorkspace(t)
	_, err := workspace.notes.Create(t.Context(), note.CreateInput{Title: "バックアップ", Content: "本文"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	result, err := workspace.service.CreateAutomaticBackup(t.Context())
	if err != nil {
		t.Fatalf("create automatic backup: %v", err)
	}
	if !result.Created || result.Backup == nil || result.Backup.Kind != KindAutomatic {
		t.Fatalf("automatic backup result = %#v", result)
	}
	list, err := workspace.service.List(t.Context())
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	backupRoot, err := RootFor(workspace.root, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("resolve backup root: %v", err)
	}
	if len(list.Backups) != 1 || !list.Backups[0].Restorable {
		t.Fatalf("backup list = %#v", list.Backups)
	}

	extraFile := filepath.Join(backupRoot, generationsName, result.Backup.ID, "notes", "unlisted.md")
	if err := os.WriteFile(extraFile, []byte("unlisted"), 0o600); err != nil {
		t.Fatalf("add unlisted backup file: %v", err)
	}
	list, err = workspace.service.List(t.Context())
	if err != nil {
		t.Fatalf("list backup with unlisted file: %v", err)
	}
	if len(list.Backups) != 1 || list.Backups[0].Restorable {
		t.Fatalf("unlisted backup file was accepted: %#v", list.Backups)
	}
	if err := os.Remove(extraFile); err != nil {
		t.Fatalf("remove unlisted backup file: %v", err)
	}
	backupDatabase := filepath.Join(backupRoot, generationsName, result.Backup.ID, "atlasnote.db")
	if err := os.WriteFile(backupDatabase, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper backup: %v", err)
	}
	list, err = workspace.service.List(t.Context())
	if err != nil {
		t.Fatalf("list tampered backups: %v", err)
	}
	if len(list.Backups) != 1 || list.Backups[0].Restorable || list.Backups[0].ErrorMessage == "" {
		t.Fatalf("tampered backup list = %#v", list.Backups)
	}
	status, err := workspace.service.Status(t.Context())
	if err != nil {
		t.Fatalf("status for tampered backup: %v", err)
	}
	if !status.AutomaticDue {
		t.Fatalf("tampered latest backup must be due: %#v", status)
	}
	if _, err := workspace.service.PreviewRestore(t.Context(), result.Backup.ID); !errors.Is(err, ErrTampered) {
		t.Fatalf("tampered preview error = %v", err)
	}
}

func TestBackupRootRejectsNonDirectoryInternalPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, backupDirectoryName), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create conflicting backup path: %v", err)
	}
	if _, err := RootFor(root, "0123456789abcdef0123456789abcdef"); !errors.Is(err, ErrValidation) {
		t.Fatalf("RootFor error = %v, want ErrValidation", err)
	}
	pending, err := PendingRestoreExists(filepath.Join(root, backupDirectoryName, "0123456789abcdef0123456789abcdef"))
	if !errors.Is(err, ErrRestoreApply) || pending {
		t.Fatalf("pending marker result = %v, %v; want ErrRestoreApply", pending, err)
	}
}

func TestRestoreStagesAndAppliesPreviousVaultWithSafetyBackup(t *testing.T) {
	workspace := newBackupTestWorkspace(t)
	created, err := workspace.notes.Create(t.Context(), note.CreateInput{Title: "復元前", Content: "元の本文"})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	automatic, err := workspace.service.CreateAutomaticBackup(t.Context())
	if err != nil || automatic.Backup == nil {
		t.Fatalf("create automatic backup: %#v %v", automatic, err)
	}
	newTitle := "変更後"
	newContent := "変更された本文"
	expectedRevision := created.Revision
	if _, err := workspace.notes.Update(t.Context(), created.ID, note.UpdateInput{Title: &newTitle, Content: &newContent, ExpectedRevision: &expectedRevision}); err != nil {
		t.Fatalf("change current note: %v", err)
	}

	preview, err := workspace.service.PreviewRestore(t.Context(), automatic.Backup.ID)
	if err != nil {
		t.Fatalf("preview restore: %v", err)
	}
	if preview.Token == "" {
		t.Fatal("restore preview did not issue token")
	}
	staged, err := workspace.service.ExecuteRestore(t.Context(), RestoreExecutionInput{Token: preview.Token})
	if err != nil {
		t.Fatalf("execute restore: %v", err)
	}
	if !staged.RestartRequired || staged.BackupID != automatic.Backup.ID {
		t.Fatalf("restore result = %#v", staged)
	}
	if _, err := workspace.service.ExecuteRestore(t.Context(), RestoreExecutionInput{Token: preview.Token}); !errors.Is(err, ErrRestorePending) {
		t.Fatalf("restore while pending error = %v", err)
	}
	if canceled, err := workspace.service.CancelRestore(t.Context()); err != nil || !canceled.Canceled {
		t.Fatalf("cancel staged restore: %#v %v", canceled, err)
	}
	preview, err = workspace.service.PreviewRestore(t.Context(), automatic.Backup.ID)
	if err != nil {
		t.Fatalf("preview restore after cancel: %v", err)
	}
	staged, err = workspace.service.ExecuteRestore(t.Context(), RestoreExecutionInput{Token: preview.Token})
	if err != nil {
		t.Fatalf("execute restore after cancel: %v", err)
	}

	workspace.service.Shutdown()
	workspace.locks.Close()
	if err := workspace.db.Close(); err != nil {
		t.Fatalf("close current database: %v", err)
	}
	applyResult, err := ApplyPendingRestore(t.Context(), RestorePaths{
		ManagementRoot: workspace.root,
		BackupRoot: func() string {
			root, _ := RootFor(workspace.root, "0123456789abcdef0123456789abcdef")
			return root
		}(),
		SpaceID: "0123456789abcdef0123456789abcdef",
		DataDir: workspace.root, DatabasePath: filepath.Join(workspace.root, "atlasnote.db"),
		NotesDir: filepath.Join(workspace.root, "notes"),
	})
	if err != nil {
		t.Fatalf("apply pending restore: %v", err)
	}
	if applyResult.RestoreSafetyBackupID == "" {
		t.Fatal("restore safety backup was not created")
	}
	pending, err := PendingRestoreExists(func() string {
		root, _ := RootFor(workspace.root, "0123456789abcdef0123456789abcdef")
		return root
	}())
	if err != nil || pending {
		t.Fatalf("pending restore after apply = %v err=%v", pending, err)
	}

	restoredDB, err := database.Open(t.Context(), filepath.Join(workspace.root, "atlasnote.db"))
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	defer restoredDB.Close()
	restoredStore, err := storage.NewMarkdownStore(filepath.Join(workspace.root, "notes"))
	if err != nil {
		t.Fatalf("open restored markdown store: %v", err)
	}
	restoredNotes := note.NewService(note.NewRepository(restoredDB), restoredStore)
	restored, err := restoredNotes.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("read restored note: %v", err)
	}
	if restored.Title != "復元前" || restored.Content != "元の本文" {
		t.Fatalf("restored note = %#v", restored)
	}

	safetyDir := filepath.Join(func() string {
		root, _ := RootFor(workspace.root, "0123456789abcdef0123456789abcdef")
		return root
	}(), generationsName, applyResult.RestoreSafetyBackupID)
	manifest, _, err := loadManifestAt(filepath.Dir(safetyDir), applyResult.RestoreSafetyBackupID, "0123456789abcdef0123456789abcdef")
	if err != nil || manifest.Kind != KindRestoreSafety {
		t.Fatalf("safety manifest = %#v err=%v", manifest, err)
	}
}
