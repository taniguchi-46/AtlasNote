package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	backupservice "atlasnote/internal/backup"
	"atlasnote/internal/config"
	"atlasnote/internal/note"
)

func TestAppBackupRestoreThenTrashAndDeleteUsesRestoredRevision(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		separateArchive bool
	}{
		{name: "same-archive", separateArchive: false},
		{name: "separate-archive", separateArchive: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			runAppBackupRestoreThenTrashAndDeleteTest(t, testCase.separateArchive)
		})
	}
}

func runAppBackupRestoreThenTrashAndDeleteTest(t *testing.T, separateArchive bool) {
	t.Helper()
	configFile := filepath.Join(t.TempDir(), "bootstrap", "storage-locations.json")
	dataRoot := filepath.Join(t.TempDir(), "data")
	backupRoot := dataRoot
	if separateArchive {
		backupRoot = filepath.Join(t.TempDir(), "archive")
	}
	t.Setenv("ATLAS_NOTE_DATA_DIR", "")
	t.Setenv("ATLAS_NOTE_DEFAULT_DATA_ROOT", dataRoot)
	t.Setenv("ATLAS_NOTE_STORAGE_LOCATIONS_FILE", configFile)
	if err := config.SaveStorageLocationsTo(configFile, config.StorageLocations{
		Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot,
	}); err != nil {
		t.Fatalf("save test storage locations: %v", err)
	}

	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	if status := app.GetStartupStatus(); !status.Ready {
		t.Fatalf("initial app is not ready: %#v", status)
	}
	target, err := app.CreateNote(note.CreateInput{Title: "復元対象", Content: "復元前の本文"})
	if err != nil {
		t.Fatalf("create target note: %v", err)
	}
	other, err := app.CreateNote(note.CreateInput{Title: "保持するノート", Content: "残る本文"})
	if err != nil {
		t.Fatalf("create other note: %v", err)
	}

	automatic := app.CreateAutomaticBackup()
	if automatic.Error != nil || !automatic.Created || automatic.Backup == nil {
		t.Fatalf("create automatic backup: %#v", automatic)
	}
	spaceID := app.activeSpace.ID
	archiveSpaceRoot, err := backupservice.RootFor(backupRoot, spaceID)
	if err != nil {
		t.Fatalf("resolve backup root: %v", err)
	}
	automaticGeneration := filepath.Join(archiveSpaceRoot, "generations", automatic.Backup.ID)
	automaticSnapshot := snapshotBackupGeneration(t, automaticGeneration)

	changedTitle := "復元後に置き換わるタイトル"
	changedContent := "復元後に置き換わる本文"
	expectedRevision := target.Revision
	changed, err := app.UpdateNote(target.ID, note.UpdateInput{
		Title: &changedTitle, Content: &changedContent, ExpectedRevision: &expectedRevision,
	})
	if err != nil || changed.Note == nil || changed.Conflict != nil {
		t.Fatalf("change current target note: %#v, %v", changed, err)
	}
	if changed.Note.Revision != target.Revision+1 {
		t.Fatalf("changed target revision = %d, want %d", changed.Note.Revision, target.Revision+1)
	}

	preview := app.PreviewBackupRestore(automatic.Backup.ID)
	if preview.Error != nil || preview.Preview == nil || preview.Preview.Token == "" {
		t.Fatalf("preview backup restore: %#v", preview)
	}
	staged := app.ExecuteBackupRestore(backupservice.RestoreExecutionInput{Token: preview.Preview.Token})
	if staged.Error != nil || !staged.RestartRequired || staged.BackupID != automatic.Backup.ID {
		t.Fatalf("stage backup restore: %#v", staged)
	}
	app.shutdown(t.Context())

	restarted := NewApp()
	restarted.startup(t.Context())
	t.Cleanup(func() { restarted.shutdown(t.Context()) })
	status := restarted.GetStartupStatus()
	if !status.Ready || status.BackupRestoreSafetyBackupID == "" {
		t.Fatalf("restored app status = %#v", status)
	}
	restored, err := restarted.GetNote(target.ID)
	if err != nil {
		t.Fatalf("get restored target note: %v", err)
	}
	if restored.Title != target.Title || restored.Content != target.Content || restored.Revision != target.Revision || restored.IsTrashed {
		t.Fatalf("restored target note = %#v, want original revision and content", restored)
	}
	if _, err := restarted.GetNote(other.ID); err != nil {
		t.Fatalf("other note was not restored: %v", err)
	}

	safetyGeneration := filepath.Join(archiveSpaceRoot, "generations", status.BackupRestoreSafetyBackupID)
	safetySnapshot := snapshotBackupGeneration(t, safetyGeneration)
	backups := restarted.ListBackups()
	if backups.Error != nil {
		t.Fatalf("list backups after restore: %#v", backups)
	}
	foundSafety := false
	for _, summary := range backups.Backups {
		if summary.ID == status.BackupRestoreSafetyBackupID {
			foundSafety = summary.Kind == backupservice.KindRestoreSafety && summary.Restorable
			break
		}
	}
	if !foundSafety {
		t.Fatalf("restore safety backup was not listed as restorable: %#v", backups.Backups)
	}

	trash := true
	trashResult, err := restarted.UpdateNote(target.ID, note.UpdateInput{
		IsTrashed: &trash, ExpectedRevision: &restored.Revision,
	})
	if err != nil || trashResult.Note == nil || trashResult.Conflict != nil {
		t.Fatalf("trash restored target note: %#v, %v", trashResult, err)
	}
	if !trashResult.Note.IsTrashed || trashResult.Note.Revision != restored.Revision+1 {
		t.Fatalf("trashed target note = %#v", trashResult.Note)
	}
	listed, err := restarted.ListNotes()
	if err != nil {
		t.Fatalf("list notes after trash: %v", err)
	}
	for _, summary := range listed {
		if summary.ID == target.ID && (!summary.IsTrashed || summary.Revision != trashResult.Note.Revision) {
			t.Fatalf("trashed summary = %#v", summary)
		}
	}

	deleted, err := restarted.DeleteNote(target.ID, note.DeleteInput{ExpectedRevision: trashResult.Note.Revision})
	if err != nil || !deleted.Deleted || deleted.Conflict != nil {
		t.Fatalf("delete trashed target note: %#v, %v", deleted, err)
	}
	if _, err := restarted.GetNote(target.ID); !errors.Is(err, note.ErrNotFound) {
		t.Fatalf("deleted target lookup error = %v, want note.ErrNotFound", err)
	}
	if _, err := restarted.GetNote(other.ID); err != nil {
		t.Fatalf("other note was deleted with target: %v", err)
	}

	assertBackupGenerationSnapshot(t, automaticGeneration, automaticSnapshot)
	assertBackupGenerationSnapshot(t, safetyGeneration, safetySnapshot)
	if pending, err := backupservice.PendingRestoreExists(archiveSpaceRoot); err != nil || pending {
		t.Fatalf("archive pending restore = %v, %v", pending, err)
	}
	localPending := filepath.Join(dataRoot, ".atlasnote-restore", spaceID, "pending.json")
	if _, err := os.Stat(localPending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local pending restore remains: %v", err)
	}
	if backupStatus := restarted.GetBackupStatus(); backupStatus.Error != nil || backupStatus.PendingRestore {
		t.Fatalf("backup status after restore = %#v", backupStatus)
	}
}

func snapshotBackupGeneration(t *testing.T, generationRoot string) map[string][]byte {
	t.Helper()
	snapshot := make(map[string][]byte)
	if err := filepath.WalkDir(generationRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("backup generation contains a non-regular file")
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(generationRoot, path)
		if err != nil {
			return err
		}
		snapshot[relative] = contents
		return nil
	}); err != nil {
		t.Fatalf("snapshot backup generation %q: %v", generationRoot, err)
	}
	return snapshot
}

func assertBackupGenerationSnapshot(t *testing.T, generationRoot string, want map[string][]byte) {
	t.Helper()
	got := snapshotBackupGeneration(t, generationRoot)
	if len(got) != len(want) {
		t.Fatalf("backup generation file count = %d, want %d", len(got), len(want))
	}
	for relative, expected := range want {
		actual, ok := got[relative]
		if !ok || !bytes.Equal(actual, expected) {
			t.Fatalf("backup generation file %q changed", relative)
		}
	}
}
