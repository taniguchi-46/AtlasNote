package app

import (
	"testing"

	"atlasnote/internal/contentlock"
	"atlasnote/internal/note"
)

func TestRelatedNotesExcludesProtectedNoteEvenWhileUnlocked(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("ATLAS_NOTE_DATA_DIR", dataDir)
	app := NewApp()
	app.startup(t.Context())
	t.Cleanup(func() { app.shutdown(t.Context()) })
	candidate, err := app.CreateNote(note.CreateInput{Title: "候補", Content: "本文"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := app.CreateNote(note.CreateInput{Title: "元", Content: "[候補](atlasnote://note/" + candidate.ID + ")"})
	if err != nil {
		t.Fatal(err)
	}
	input := note.RelatedNoteInput{NoteID: source.ID}
	before, err := app.RelatedNotes(input)
	if err != nil || len(before.Items) != 1 {
		t.Fatalf("before protection = %#v, %v", before, err)
	}
	protected := app.EnableContentLock(contentlock.EnableInput{TargetType: contentlock.TargetNote, TargetID: candidate.ID, Passphrase: "correct horse battery staple"})
	if protected.Error != nil || !protected.Unlocked {
		t.Fatalf("protect candidate = %#v", protected)
	}
	after, err := app.RelatedNotes(input)
	if err != nil || len(after.Items) != 0 {
		t.Fatalf("protected candidate = %#v, %v", after, err)
	}
	protected = app.EnableContentLock(contentlock.EnableInput{TargetType: contentlock.TargetNote, TargetID: source.ID, Passphrase: "correct horse battery staple"})
	if protected.Error != nil {
		t.Fatalf("protect source = %#v", protected)
	}
	if _, err := app.RelatedNotes(input); err == nil {
		t.Fatal("protected source was accepted")
	}
}
