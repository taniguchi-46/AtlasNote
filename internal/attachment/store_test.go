package attachment

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"atlasnote/internal/contentformat"
)

func TestStoreSavesReadsRecoversAndExportsImageAttachment(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "notes"), nil)
	if err != nil {
		t.Fatalf("new attachment store: %v", err)
	}
	content := testPNG(t, 3, 2)
	result, err := store.Save(context.Background(), SaveInput{
		NoteID: "note-1", Kind: "image", MIMEType: "image/png", Name: "paste.png", Data: content,
	})
	if err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if result.Metadata.ID == "" || result.Metadata.Size != int64(len(content)) || result.Metadata.Width != 3 || result.Metadata.Height != 2 {
		t.Fatalf("saved attachment metadata = %#v", result.Metadata)
	}
	if result.Reference != Reference("note-1", result.Metadata.ID) {
		t.Fatalf("attachment reference = %q", result.Reference)
	}
	noteID, attachmentID, ok := ParseReference(result.Reference)
	if !ok || noteID != "note-1" || attachmentID != result.Metadata.ID {
		t.Fatalf("parsed attachment reference = %q, %q, %v", noteID, attachmentID, ok)
	}

	items, err := store.List(context.Background(), "note-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("list attachments = %#v, %v", items, err)
	}
	readMetadata, readContent, err := store.Read(context.Background(), "note-1", result.Metadata.ID)
	if err != nil || readMetadata.SHA256 != result.Metadata.SHA256 || !bytes.Equal(readContent, content) {
		t.Fatalf("read attachment = %#v, %v, %d bytes", readMetadata, err, len(readContent))
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recover attachments: %v", err)
	}

	archiveBytes, err := store.ExportZip(context.Background(), "note-1")
	if err != nil {
		t.Fatalf("export attachments: %v", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("read attachment archive: %v", err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("archive entries = %d, want manifest and image", len(archive.File))
	}
	var foundImage bool
	for _, file := range archive.File {
		reader, openErr := file.Open()
		if openErr != nil {
			t.Fatalf("open archive entry %q: %v", file.Name, openErr)
		}
		entry, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			t.Fatalf("read archive entry %q: %v", file.Name, readErr)
		}
		if file.Name != "manifest.json" && bytes.Equal(entry, content) {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatal("archive did not contain the original image bytes")
	}
}

func TestStoreRejectsInvalidImageAndKeepsLimitsBounded(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "notes"), nil)
	if err != nil {
		t.Fatalf("new attachment store: %v", err)
	}
	if _, err := store.Save(context.Background(), SaveInput{
		NoteID: "note-1", Kind: "image", MIMEType: "image/png", Name: "bad.png", Data: []byte("not an image"),
	}); err != ErrInvalidImage {
		t.Fatalf("invalid image error = %v, want %v", err, ErrInvalidImage)
	}
	if _, _, ok := ParseReference("atlasnote-attachment://note-1/../secret"); ok {
		t.Fatal("unsafe attachment reference was accepted")
	}
	if _, err := store.ExportZip(context.Background(), "note-1"); err != ErrNoAttachments {
		t.Fatalf("empty attachment export error = %v, want %v", err, ErrNoAttachments)
	}
	if _, err := store.List(context.Background(), "bad/note"); err != ErrInvalidInput {
		t.Fatalf("unsafe note ID error = %v, want %v", err, ErrInvalidInput)
	}
}

func TestStoreAcceptsMaximumEncodedProtectedAttachmentAndRecovers(t *testing.T) {
	protector := boundaryProtector{}
	store, err := NewStore(filepath.Join(t.TempDir(), "notes"), protector)
	if err != nil {
		t.Fatalf("new attachment store: %v", err)
	}
	content := append(testPNG(t, 1, 1), bytes.Repeat([]byte{0x7f}, MaxAttachmentBytes-len(testPNG(t, 1, 1)))...)
	result, err := store.Save(context.Background(), SaveInput{
		NoteID: "note-1", Kind: "image", MIMEType: "image/png", Name: "large.png", Data: content,
	})
	if err != nil {
		t.Fatalf("save maximum protected attachment: %v", err)
	}
	storedPath := filepath.Join(store.rootDir, "note-1", result.Metadata.ID+".bin")
	info, err := os.Stat(storedPath)
	if err != nil {
		t.Fatalf("stat protected attachment: %v", err)
	}
	if info.Size() != int64(contentformat.MaxProtectedContentBytes(MaxAttachmentBytes)) {
		t.Fatalf("stored size = %d, want %d", info.Size(), contentformat.MaxProtectedContentBytes(MaxAttachmentBytes))
	}
	if _, _, err := store.Read(context.Background(), "note-1", result.Metadata.ID); err != nil {
		t.Fatalf("read maximum protected attachment: %v", err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recover maximum protected attachment: %v", err)
	}
}

func TestStoreStagesAndCommitsNoteAttachmentDelete(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "notes"), nil)
	if err != nil {
		t.Fatalf("new attachment store: %v", err)
	}
	if _, err := store.Save(context.Background(), SaveInput{
		NoteID: "note-1", Kind: "image", MIMEType: "image/png", Name: "paste.png", Data: testPNG(t, 1, 1),
	}); err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	const operationID = "operation-1"
	if err := store.StageDelete(context.Background(), "note-1", operationID); err != nil {
		t.Fatalf("stage attachment delete: %v", err)
	}
	if exists, err := store.Exists(context.Background(), "note-1"); err != nil || exists {
		t.Fatalf("attachment source exists after stage = %v, %v", exists, err)
	}
	if staged, err := store.DeleteStagedExists(context.Background(), "note-1", operationID); err != nil || !staged {
		t.Fatalf("attachment staged state = %v, %v", staged, err)
	}
	if hasAny, err := store.HasAny(context.Background()); err != nil || hasAny {
		t.Fatalf("staged attachment counted as active = %v, %v", hasAny, err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recover staged attachment: %v", err)
	}
	if err := store.RestoreDelete(context.Background(), "note-1", operationID); err != nil {
		t.Fatalf("restore attachment delete: %v", err)
	}
	if hasAny, err := store.HasAny(context.Background()); err != nil || !hasAny {
		t.Fatalf("restored attachment active state = %v, %v", hasAny, err)
	}
	if err := store.StageDelete(context.Background(), "note-1", operationID); err != nil {
		t.Fatalf("restage attachment delete: %v", err)
	}
	if err := store.CommitDelete(context.Background(), "note-1", operationID); err != nil {
		t.Fatalf("commit attachment delete: %v", err)
	}
	if hasAny, err := store.HasAny(context.Background()); err != nil || hasAny {
		t.Fatalf("deleted attachment active state = %v, %v", hasAny, err)
	}
}

func TestStoreStagesAndCommitsSyncCopyWithProtector(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "notes"), boundaryProtector{})
	if err != nil {
		t.Fatalf("new protected attachment store: %v", err)
	}
	content := testPNG(t, 2, 2)
	source, err := store.Save(context.Background(), SaveInput{
		NoteID: "source-note", Kind: "image", MIMEType: "image/png", Name: "source.png", Data: content,
	})
	if err != nil {
		t.Fatalf("save protected source attachment: %v", err)
	}
	copied := source.Metadata
	copied.ID = "copied-image"
	copied.NoteID = "copied-note"
	if err := store.StageSyncAttachment(context.Background(), copied.NoteID, "copy-operation", copied, content); err != nil {
		t.Fatalf("stage protected sync copy: %v", err)
	}
	if items, err := store.ListAll(context.Background()); err != nil || len(items) != 1 || items[0].NoteID != source.Metadata.NoteID {
		t.Fatalf("staged protected copy appeared active = %#v, err=%v", items, err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recover protected sync copy: %v", err)
	}
	if err := store.CommitSyncAttachments(context.Background(), copied.NoteID, "copy-operation"); err != nil {
		t.Fatalf("commit protected sync copy: %v", err)
	}
	_, readContent, err := store.Read(context.Background(), copied.NoteID, copied.ID)
	if err != nil || !bytes.Equal(readContent, content) {
		t.Fatalf("read committed protected sync copy = %v", err)
	}
	stored, err := os.ReadFile(store.dataPath(copied.NoteID, copied.ID))
	if err != nil {
		t.Fatalf("read stored protected sync copy: %v", err)
	}
	if bytes.Equal(stored, content) {
		t.Fatal("committed protected sync copy was written as plaintext")
	}
}

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	imageValue := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageValue.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageValue); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	if !strings.HasPrefix(string(buffer.Bytes()), "\x89PNG") {
		t.Fatal("test PNG header missing")
	}
	return buffer.Bytes()
}

type boundaryProtector struct{}

func (boundaryProtector) EncodeAttachment(_ context.Context, _ string, _ string, plain []byte) ([]byte, error) {
	stored := make([]byte, contentformat.MaxProtectedContentBytes(len(plain)))
	binary.LittleEndian.PutUint64(stored[:8], uint64(len(plain)))
	copy(stored[8:], plain)
	return stored, nil
}

func (boundaryProtector) DecodeAttachment(_ context.Context, _ string, _ string, stored []byte) ([]byte, error) {
	plainSize := int(binary.LittleEndian.Uint64(stored[:8]))
	return append([]byte(nil), stored[8:8+plainSize]...), nil
}
