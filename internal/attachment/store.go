package attachment

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"atlasnote/internal/contentformat"
	"atlasnote/internal/fileatomic"
)

const (
	ManifestVersion        = 1
	MaxAttachmentBytes     = 10 * 1024 * 1024
	MaxNoteAttachmentBytes = 64 * 1024 * 1024
	MaxAttachmentsPerNote  = 100
	MaxImageWidth          = 8192
	MaxImageHeight         = 8192
	MaxImagePixels         = int64(40_000_000)
	maxManifestBytes       = 256 * 1024
	syncCopyStageVersion   = 1
	syncCopyStageSuffix    = ".sync-copy"
)

var maxStoredAttachmentBytes = contentformat.MaxProtectedContentBytes(MaxAttachmentBytes)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	ErrInvalidInput         = errors.New("attachment input is invalid")
	ErrInvalidImage         = errors.New("attachment image is invalid")
	ErrAttachmentTooLarge   = errors.New("attachment image is too large")
	ErrAttachmentDimensions = errors.New("attachment image dimensions are invalid")
	ErrTooManyAttachments   = errors.New("too many note attachments")
	ErrNoteAttachmentsLarge = errors.New("note attachments are too large")
	ErrAttachmentNotFound   = errors.New("note attachment was not found")
	ErrManifestInvalid      = errors.New("note attachment manifest is invalid")
	ErrNoAttachments        = errors.New("note has no attachments")
)

// Protector is implemented by the content-lock manager. Attachments use a
// purpose-specific authenticated-encryption context while reusing the note's
// lock materials.
type Protector interface {
	EncodeAttachment(context.Context, string, string, []byte) ([]byte, error)
	DecodeAttachment(context.Context, string, string, []byte) ([]byte, error)
}

type Metadata struct {
	ID        string `json:"id"`
	NoteID    string `json:"noteId"`
	Kind      string `json:"kind"`
	MIMEType  string `json:"mimeType"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CreatedAt string `json:"createdAt"`
}

type SaveInput struct {
	NoteID   string
	Kind     string
	MIMEType string
	Name     string
	Data     []byte
}

type SaveResult struct {
	Metadata  Metadata
	Reference string
}

type manifest struct {
	Version     int        `json:"version"`
	NoteID      string     `json:"noteId"`
	Attachments []Metadata `json:"attachments"`
}

type syncCopyManifest struct {
	Version     int        `json:"version"`
	NoteID      string     `json:"noteId"`
	OperationID string     `json:"operationId"`
	Attachments []Metadata `json:"attachments"`
}

type Store struct {
	rootDir     string
	mu          sync.Mutex
	protectorMu sync.RWMutex
	protector   Protector
}

func NewStore(notesDir string, protector Protector) (*Store, error) {
	rootDir := filepath.Join(filepath.Clean(notesDir), "attachments")
	if err := ensureDirectory(rootDir); err != nil {
		return nil, fmt.Errorf("create attachment directory: %w", err)
	}
	return &Store{rootDir: rootDir, protector: protector}, nil
}

func (s *Store) SetProtector(protector Protector) {
	s.protectorMu.Lock()
	s.protector = protector
	s.protectorMu.Unlock()
}

func (s *Store) protectorValue() Protector {
	s.protectorMu.RLock()
	protector := s.protector
	s.protectorMu.RUnlock()
	return protector
}

func (s *Store) Save(ctx context.Context, input SaveInput) (SaveResult, error) {
	if err := ctx.Err(); err != nil {
		return SaveResult{}, err
	}
	noteID, err := validateID(input.NoteID)
	if err != nil {
		return SaveResult{}, err
	}
	if strings.TrimSpace(input.Kind) != "image" {
		return SaveResult{}, ErrInvalidInput
	}
	width, height, mimeType, err := validateImage(input.MIMEType, input.Data)
	if err != nil {
		return SaveResult{}, err
	}
	name := safeAttachmentName(input.Name, mimeType, "")

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return SaveResult{}, err
	}
	if len(current.Attachments) >= MaxAttachmentsPerNote {
		return SaveResult{}, ErrTooManyAttachments
	}
	var total int64
	for _, item := range current.Attachments {
		total += item.Size
	}
	if total+int64(len(input.Data)) > MaxNoteAttachmentBytes {
		return SaveResult{}, ErrNoteAttachmentsLarge
	}

	id, err := newAttachmentID()
	if err != nil {
		return SaveResult{}, err
	}
	name = safeAttachmentName(input.Name, mimeType, id)
	stored := append([]byte(nil), input.Data...)
	if protector := s.protectorValue(); protector != nil {
		stored, err = protector.EncodeAttachment(ctx, noteID, id, stored)
		if err != nil {
			return SaveResult{}, err
		}
	}
	if len(stored) > maxStoredAttachmentBytes {
		return SaveResult{}, ErrAttachmentTooLarge
	}
	if err := ensureDirectory(s.noteDir(noteID)); err != nil {
		return SaveResult{}, fmt.Errorf("create attachment note directory: %w", err)
	}
	if err := writeAtomic(s.dataPath(noteID, id), stored); err != nil {
		return SaveResult{}, fmt.Errorf("write attachment: %w", err)
	}

	metadata := Metadata{
		ID: id, NoteID: noteID, Kind: "image", MIMEType: mimeType, Name: name,
		Size: int64(len(input.Data)), SHA256: hashBytes(input.Data), Width: width,
		Height: height, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	current.Attachments = append(current.Attachments, metadata)
	if err := s.writeManifestLocked(noteID, current); err != nil {
		// Keep the bytes. A failed metadata commit must not destroy the pasted
		// image; startup validation can preserve or recover the orphan safely.
		return SaveResult{}, fmt.Errorf("write attachment manifest: %w", err)
	}
	return SaveResult{Metadata: metadata, Reference: Reference(noteID, id)}, nil
}

func (s *Store) List(ctx context.Context, noteID string) ([]Metadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return nil, err
	}
	return append([]Metadata(nil), current.Attachments...), nil
}

// ListAll returns the active attachment metadata across the vault. Delete
// journals are intentionally excluded; note deletion owns their lifecycle.
func (s *Store) ListAll(ctx context.Context) ([]Metadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.rootDir)
	if errors.Is(err, os.ErrNotExist) {
		return []Metadata{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Metadata, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if isDeleteStageName(entry.Name()) || isSyncCopyStageName(entry.Name()) {
			continue
		}
		noteID, err := validateID(entry.Name())
		if err != nil {
			return nil, ErrManifestInvalid
		}
		current, err := s.loadManifestLocked(noteID)
		if err != nil {
			return nil, err
		}
		items = append(items, current.Attachments...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].NoteID != items[j].NoteID {
			return items[i].NoteID < items[j].NoteID
		}
		if items[i].CreatedAt != items[j].CreatedAt {
			return items[i].CreatedAt < items[j].CreatedAt
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *Store) HasAny(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.rootDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if isDeleteStageName(entry.Name()) || isSyncCopyStageName(entry.Name()) {
			continue
		}
		if _, err := validateID(entry.Name()); err != nil {
			return false, ErrManifestInvalid
		}
		current, err := s.loadManifestLocked(entry.Name())
		if err != nil {
			return false, err
		}
		if len(current.Attachments) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) Read(ctx context.Context, noteID string, attachmentID string) (Metadata, []byte, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, nil, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return Metadata{}, nil, err
	}
	attachmentID, err = validateID(attachmentID)
	if err != nil {
		return Metadata{}, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return Metadata{}, nil, err
	}
	for _, metadata := range current.Attachments {
		if metadata.ID != attachmentID {
			continue
		}
		content, err := s.readAttachmentLocked(ctx, metadata)
		if err != nil {
			return Metadata{}, nil, err
		}
		return metadata, content, nil
	}
	return Metadata{}, nil, ErrAttachmentNotFound
}

// ImportSyncAttachment validates and atomically records one attachment
// received from the sync protocol. Sync payloads carry plaintext bytes; the
// local Protector, when present, is still responsible for the local at-rest
// representation.
func (s *Store) ImportSyncAttachment(ctx context.Context, metadata Metadata, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(metadata.NoteID)
	if err != nil {
		return err
	}
	attachmentID, err := validateID(metadata.ID)
	if err != nil {
		return err
	}
	width, height, mimeType, err := validateImage(metadata.MIMEType, data)
	if err != nil {
		return err
	}
	if metadata.Kind != "image" || metadata.Size != int64(len(data)) || metadata.SHA256 != hashBytes(data) ||
		metadata.Width != width || metadata.Height != height || metadata.MIMEType != mimeType {
		return ErrManifestInvalid
	}
	metadata.NoteID = noteID
	metadata.ID = attachmentID
	metadata.Name = safeAttachmentName(metadata.Name, mimeType, attachmentID)

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return err
	}
	existingIndex := -1
	for index, existing := range current.Attachments {
		if existing.ID != attachmentID {
			continue
		}
		if existing == metadata {
			if _, readErr := s.readAttachmentLocked(ctx, existing); readErr == nil {
				return nil
			}
			existingIndex = index
			break
		}
		// Attachment IDs are immutable. A different body under an existing ID
		// must be resolved as a sync conflict instead of replacing bytes under
		// a manifest that may still be visible to another reader.
		return ErrManifestInvalid
	}
	if existingIndex < 0 && len(current.Attachments) >= MaxAttachmentsPerNote {
		return ErrTooManyAttachments
	}
	var total int64
	for index, item := range current.Attachments {
		if index == existingIndex {
			continue
		}
		total += item.Size
	}
	if total+metadata.Size > MaxNoteAttachmentBytes {
		return ErrNoteAttachmentsLarge
	}

	stored := append([]byte(nil), data...)
	if protector := s.protectorValue(); protector != nil {
		stored, err = protector.EncodeAttachment(ctx, noteID, attachmentID, stored)
		if err != nil {
			return err
		}
	}
	if len(stored) > maxStoredAttachmentBytes {
		return ErrAttachmentTooLarge
	}
	if err := ensureDirectory(s.noteDir(noteID)); err != nil {
		return err
	}
	if err := writeAtomic(s.dataPath(noteID, attachmentID), stored); err != nil {
		return err
	}
	if existingIndex >= 0 {
		current.Attachments[existingIndex] = metadata
	} else {
		current.Attachments = append(current.Attachments, metadata)
	}
	return s.writeManifestLocked(noteID, current)
}

// StageSyncAttachment stores one attachment for a sync-side note copy outside
// the active note directories. Repeating the same operation and attachment ID
// replaces the same staged bytes and metadata, so a retry cannot create a
// second active attachment.
func (s *Store) StageSyncAttachment(ctx context.Context, noteID string, operationID string, metadata Metadata, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	attachmentID, err := validateID(metadata.ID)
	if err != nil {
		return err
	}
	width, height, mimeType, err := validateImage(metadata.MIMEType, data)
	if err != nil {
		return err
	}
	if metadata.NoteID != noteID || metadata.Kind != "image" || metadata.Size != int64(len(data)) ||
		metadata.SHA256 != hashBytes(data) || metadata.Width != width || metadata.Height != height || metadata.MIMEType != mimeType {
		return ErrManifestInvalid
	}
	metadata.NoteID = noteID
	metadata.ID = attachmentID
	metadata.Name = safeAttachmentName(metadata.Name, mimeType, attachmentID)

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadSyncCopyManifestLocked(noteID, operationID)
	if err != nil {
		return err
	}
	existingIndex := -1
	for index, existing := range current.Attachments {
		if existing.ID == attachmentID {
			existingIndex = index
			if existing != metadata {
				return ErrManifestInvalid
			}
			break
		}
	}
	if existingIndex < 0 && len(current.Attachments) >= MaxAttachmentsPerNote {
		return ErrTooManyAttachments
	}
	var total int64
	for index, item := range current.Attachments {
		if index != existingIndex {
			total += item.Size
		}
	}
	if existingIndex < 0 {
		total += metadata.Size
	}
	if total > MaxNoteAttachmentBytes {
		return ErrNoteAttachmentsLarge
	}

	stored := append([]byte(nil), data...)
	if protector := s.protectorValue(); protector != nil {
		stored, err = protector.EncodeAttachment(ctx, noteID, attachmentID, stored)
		if err != nil {
			return err
		}
	}
	if len(stored) > maxStoredAttachmentBytes {
		return ErrAttachmentTooLarge
	}
	if err := ensureDirectory(s.syncCopyDir(noteID, operationID)); err != nil {
		return err
	}
	if err := writeAtomic(s.syncCopyDataPath(noteID, operationID, attachmentID), stored); err != nil {
		return err
	}
	if existingIndex >= 0 {
		current.Attachments[existingIndex] = metadata
	} else {
		current.Attachments = append(current.Attachments, metadata)
	}
	return s.writeSyncCopyManifestLocked(noteID, operationID, current)
}

// CommitSyncAttachments makes a completed sync-side note copy visible in the
// active attachment manifest. The staged directory is retained if any write
// fails, allowing the same deterministic operation to resume later.
func (s *Store) CommitSyncAttachments(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stageDir := s.syncCopyDir(noteID, operationID)
	info, err := os.Lstat(stageDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrManifestInvalid
	}
	if _, err := os.Lstat(s.syncCopyManifestPath(noteID, operationID)); errors.Is(err, os.ErrNotExist) {
		return ErrManifestInvalid
	} else if err != nil {
		return err
	}
	staged, err := s.loadSyncCopyManifestLocked(noteID, operationID)
	if err != nil {
		return err
	}
	if staged.OperationID != operationID || len(staged.Attachments) == 0 {
		return ErrManifestInvalid
	}

	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return err
	}
	activeByID := make(map[string]int, len(current.Attachments))
	for index, item := range current.Attachments {
		activeByID[item.ID] = index
	}
	var total int64
	for _, item := range current.Attachments {
		total += item.Size
	}
	storedByID := make(map[string][]byte, len(staged.Attachments))
	for _, metadata := range staged.Attachments {
		if index, ok := activeByID[metadata.ID]; ok {
			if current.Attachments[index] != metadata {
				return ErrManifestInvalid
			}
		} else {
			if len(current.Attachments) >= MaxAttachmentsPerNote {
				return ErrTooManyAttachments
			}
			current.Attachments = append(current.Attachments, metadata)
			activeByID[metadata.ID] = len(current.Attachments) - 1
			total += metadata.Size
			if total > MaxNoteAttachmentBytes {
				return ErrNoteAttachmentsLarge
			}
		}
		stored, _, readErr := s.readStoredDataLocked(ctx, metadata, s.syncCopyDataPath(noteID, operationID, metadata.ID))
		if readErr != nil {
			return readErr
		}
		storedByID[metadata.ID] = stored
	}

	for _, metadata := range staged.Attachments {
		if err := writeAtomic(s.dataPath(noteID, metadata.ID), storedByID[metadata.ID]); err != nil {
			return err
		}
	}
	sort.SliceStable(current.Attachments, func(i, j int) bool {
		if current.Attachments[i].CreatedAt != current.Attachments[j].CreatedAt {
			return current.Attachments[i].CreatedAt < current.Attachments[j].CreatedAt
		}
		return current.Attachments[i].ID < current.Attachments[j].ID
	})
	if err := s.writeManifestLocked(noteID, current); err != nil {
		return err
	}
	return removeAttachmentPath(stageDir)
}

// RecoverSyncCopy resumes completed sync-copy stages for a note that is
// already canonical locally. A stage without its manifest is an incomplete
// operation and remains isolated for an explicit retry; its parent note must
// not be registered for upload while its attachment set is incomplete.
func (s *Store) RecoverSyncCopy(ctx context.Context, noteID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(s.rootDir)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	prefix := noteID + "."
	ready := true
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !isSyncCopyStageName(name) {
			continue
		}
		stagePath := filepath.Join(s.rootDir, name)
		info, err := os.Lstat(stagePath)
		if err != nil {
			return false, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false, ErrManifestInvalid
		}
		operationID := strings.TrimSuffix(strings.TrimPrefix(name, prefix), syncCopyStageSuffix)
		if _, err := validateID(operationID); err != nil {
			return false, ErrManifestInvalid
		}
		manifestPath := filepath.Join(stagePath, "manifest.json")
		manifestInfo, err := os.Lstat(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			ready = false
			continue
		}
		if err != nil {
			return false, err
		}
		if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
			return false, ErrManifestInvalid
		}
		if err := s.CommitSyncAttachments(ctx, noteID, operationID); err != nil {
			return false, err
		}
	}
	return ready, nil
}

func (s *Store) loadSyncCopyManifestLocked(noteID string, operationID string) (syncCopyManifest, error) {
	stageDir := s.syncCopyDir(noteID, operationID)
	stageInfo, stageErr := os.Lstat(stageDir)
	if stageErr == nil && (!stageInfo.IsDir() || stageInfo.Mode()&os.ModeSymlink != 0) {
		return syncCopyManifest{}, ErrManifestInvalid
	}
	if stageErr != nil && !errors.Is(stageErr, os.ErrNotExist) {
		return syncCopyManifest{}, stageErr
	}
	path := s.syncCopyManifestPath(noteID, operationID)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return syncCopyManifest{
			Version: syncCopyStageVersion, NoteID: noteID, OperationID: operationID,
			Attachments: []Metadata{},
		}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxManifestBytes {
		return syncCopyManifest{}, ErrManifestInvalid
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return syncCopyManifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var current syncCopyManifest
	if err := decoder.Decode(&current); err != nil || current.Version != syncCopyStageVersion ||
		current.NoteID != noteID || current.OperationID != operationID {
		return syncCopyManifest{}, ErrManifestInvalid
	}
	if err := validateAttachmentManifestItems(current.Attachments, noteID); err != nil {
		return syncCopyManifest{}, err
	}
	return current, nil
}

func (s *Store) writeSyncCopyManifestLocked(noteID string, operationID string, current syncCopyManifest) error {
	current.Version = syncCopyStageVersion
	current.NoteID = noteID
	current.OperationID = operationID
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxManifestBytes {
		return ErrManifestInvalid
	}
	return writeAtomic(s.syncCopyManifestPath(noteID, operationID), append(encoded, '\n'))
}

func (s *Store) validateSyncCopyStageLocked(ctx context.Context, stageName string) error {
	stageDir := filepath.Join(s.rootDir, stageName)
	info, err := os.Lstat(stageDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrManifestInvalid
	}
	manifestPath := filepath.Join(stageDir, "manifest.json")
	if _, err := os.Lstat(manifestPath); errors.Is(err, os.ErrNotExist) {
		// A crash between the first staged body and its manifest is an
		// incomplete but isolated operation. Keep it for the next retry.
		return nil
	} else if err != nil {
		return err
	}
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var staged syncCopyManifest
	if err := decoder.Decode(&staged); err != nil || staged.Version != syncCopyStageVersion || staged.OperationID == "" {
		return ErrManifestInvalid
	}
	if _, err := validateID(staged.NoteID); err != nil {
		return ErrManifestInvalid
	}
	if _, err := validateID(staged.OperationID); err != nil || stageName != syncCopyStageName(staged.NoteID, staged.OperationID) {
		return ErrManifestInvalid
	}
	if err := validateAttachmentManifestItems(staged.Attachments, staged.NoteID); err != nil {
		return err
	}
	for _, metadata := range staged.Attachments {
		if _, err := s.readStoredAttachmentLocked(ctx, metadata, filepath.Join(stageDir, metadata.ID+".bin")); err != nil {
			return err
		}
	}
	return nil
}

func validateAttachmentManifestItems(items []Metadata, noteID string) error {
	if len(items) > MaxAttachmentsPerNote {
		return ErrTooManyAttachments
	}
	seen := make(map[string]struct{}, len(items))
	var total int64
	for _, item := range items {
		if _, err := validateID(item.ID); err != nil || item.NoteID != noteID || item.Kind != "image" ||
			item.Size < 1 || item.Size > MaxAttachmentBytes || !isSHA256(item.SHA256) ||
			(item.MIMEType != "image/png" && item.MIMEType != "image/jpeg") || item.Width < 1 ||
			item.Width > MaxImageWidth || item.Height < 1 || item.Height > MaxImageHeight ||
			int64(item.Width)*int64(item.Height) > MaxImagePixels {
			return ErrManifestInvalid
		}
		if _, ok := seen[item.ID]; ok {
			return ErrManifestInvalid
		}
		seen[item.ID] = struct{}{}
		total += item.Size
	}
	if total > MaxNoteAttachmentBytes {
		return ErrNoteAttachmentsLarge
	}
	return nil
}

// DeleteAttachment removes one active attachment from its manifest. A body
// left behind after a later filesystem failure is treated as an orphan and is
// deliberately preserved for recovery inspection.
func (s *Store) DeleteAttachment(ctx context.Context, noteID string, attachmentID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	attachmentID, err = validateID(attachmentID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return err
	}
	index := -1
	for i, item := range current.Attachments {
		if item.ID == attachmentID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}
	current.Attachments = append(current.Attachments[:index], current.Attachments[index+1:]...)
	if err := s.writeManifestLocked(noteID, current); err != nil {
		return err
	}
	if err := os.Remove(s.dataPath(noteID, attachmentID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) ExportZip(ctx context.Context, noteID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return nil, err
	}
	if len(current.Attachments) == 0 {
		return nil, ErrNoAttachments
	}

	buffer := bytes.NewBuffer(nil)
	archive := zip.NewWriter(buffer)
	manifestBytes, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		_ = archive.Close()
		return nil, err
	}
	manifestFile, err := archive.Create("manifest.json")
	if err != nil {
		_ = archive.Close()
		return nil, err
	}
	if _, err := manifestFile.Write(append(manifestBytes, '\n')); err != nil {
		_ = archive.Close()
		return nil, err
	}

	usedNames := make(map[string]struct{}, len(current.Attachments))
	for _, metadata := range current.Attachments {
		content, err := s.readAttachmentLocked(ctx, metadata)
		if err != nil {
			_ = archive.Close()
			return nil, err
		}
		name := safeAttachmentName(metadata.Name, metadata.MIMEType, metadata.ID)
		if _, exists := usedNames[name]; exists {
			name = metadata.ID + "-" + name
		}
		usedNames[name] = struct{}{}
		file, err := archive.Create(name)
		if err != nil {
			_ = archive.Close()
			return nil, err
		}
		if _, err := file.Write(content); err != nil {
			_ = archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// StageDelete moves all attachments for a note aside while the note storage
// operation is journaled. An absent directory is a valid no-attachment case.
func (s *Store) StageDelete(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.noteDir(noteID)
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrManifestInvalid
	}
	staged := s.deletePath(noteID, operationID)
	if _, err := os.Lstat(staged); err == nil {
		return ErrManifestInvalid
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(source, staged); err != nil {
		return fmt.Errorf("stage attachment delete: %w", err)
	}
	return nil
}

func (s *Store) RestoreDelete(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	staged := s.deletePath(noteID, operationID)
	if _, err := os.Lstat(staged); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	source := s.noteDir(noteID)
	if _, err := os.Lstat(source); err == nil {
		return ErrManifestInvalid
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(staged, source); err != nil {
		return fmt.Errorf("restore staged attachment delete: %w", err)
	}
	return nil
}

func (s *Store) CommitDelete(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return removeAttachmentPath(s.deletePath(noteID, operationID))
}

func (s *Store) DeleteStagedExists(ctx context.Context, noteID string, operationID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return false, err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Lstat(s.deletePath(noteID, operationID))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, ErrManifestInvalid
	}
	return true, nil
}

func (s *Store) Exists(ctx context.Context, noteID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Lstat(s.noteDir(noteID))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, ErrManifestInvalid
	}
	return true, nil
}

func (s *Store) Delete(ctx context.Context, noteID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return removeAttachmentPath(s.noteDir(noteID))
}

// Recover validates attachment manifests and referenced files without
// deleting anything. Unknown or orphaned files remain recoverable.
func (s *Store) Recover(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.rootDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if isDeleteStageName(entry.Name()) {
			continue
		}
		if isSyncCopyStageName(entry.Name()) {
			if err := s.validateSyncCopyStageLocked(ctx, entry.Name()); err != nil {
				return err
			}
			continue
		}
		if _, err := validateID(entry.Name()); err != nil {
			return ErrManifestInvalid
		}
		current, err := s.loadManifestLocked(entry.Name())
		if err != nil {
			return err
		}
		for _, metadata := range current.Attachments {
			if err := s.validateStoredAttachmentLocked(metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

// StageReencode prepares attachment ciphertext alongside the Markdown lock
// transaction. The manager supplies pending lock state through Protector.
func (s *Store) StageReencode(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return err
	}
	protector := s.protectorValue()
	for _, metadata := range current.Attachments {
		plain, err := s.readAttachmentLocked(ctx, metadata)
		if err != nil {
			return err
		}
		stored := plain
		if protector != nil {
			stored, err = protector.EncodeAttachment(ctx, noteID, metadata.ID, plain)
			if err != nil {
				return err
			}
		}
		if len(stored) > maxStoredAttachmentBytes {
			return ErrAttachmentTooLarge
		}
		if err := writeAtomic(s.stagePath(noteID, metadata.ID, operationID), stored); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CommitReencode(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadManifestLocked(noteID)
	if err != nil {
		return err
	}
	for _, metadata := range current.Attachments {
		stagedPath := s.stagePath(noteID, metadata.ID, operationID)
		stored, readErr := os.ReadFile(stagedPath)
		if errors.Is(readErr, os.ErrNotExist) {
			if _, statErr := os.Stat(s.dataPath(noteID, metadata.ID)); statErr != nil {
				return ErrManifestInvalid
			}
			continue
		}
		if readErr != nil {
			return readErr
		}
		if len(stored) > maxStoredAttachmentBytes {
			return ErrAttachmentTooLarge
		}
		if err := writeAtomic(s.dataPath(noteID, metadata.ID), stored); err != nil {
			return err
		}
		if err := os.Remove(stagedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) RollbackReencode(ctx context.Context, noteID string, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	noteID, err := validateID(noteID)
	if err != nil {
		return err
	}
	operationID, err = validateID(operationID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.noteDir(noteID))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	suffix := "." + operationID + ".tmp"
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		if err := os.Remove(filepath.Join(s.noteDir(noteID), entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) loadManifestLocked(noteID string) (manifest, error) {
	path := s.manifestPath(noteID)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return manifest{Version: ManifestVersion, NoteID: noteID, Attachments: []Metadata{}}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxManifestBytes {
		return manifest{}, ErrManifestInvalid
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var current manifest
	if err := decoder.Decode(&current); err != nil || current.Version != ManifestVersion || current.NoteID != noteID {
		return manifest{}, ErrManifestInvalid
	}
	if len(current.Attachments) > MaxAttachmentsPerNote {
		return manifest{}, ErrTooManyAttachments
	}
	seen := make(map[string]struct{}, len(current.Attachments))
	var total int64
	for index := range current.Attachments {
		item := &current.Attachments[index]
		if _, err := validateID(item.ID); err != nil || item.NoteID != noteID || item.Kind != "image" || item.Size < 1 || item.Size > MaxAttachmentBytes || !isSHA256(item.SHA256) {
			return manifest{}, ErrManifestInvalid
		}
		if _, ok := seen[item.ID]; ok {
			return manifest{}, ErrManifestInvalid
		}
		seen[item.ID] = struct{}{}
		if item.MIMEType != "image/png" && item.MIMEType != "image/jpeg" {
			return manifest{}, ErrManifestInvalid
		}
		if item.Width < 1 || item.Width > MaxImageWidth || item.Height < 1 || item.Height > MaxImageHeight || int64(item.Width)*int64(item.Height) > MaxImagePixels {
			return manifest{}, ErrManifestInvalid
		}
		item.Name = safeAttachmentName(item.Name, item.MIMEType, item.ID)
		total += item.Size
	}
	if total > MaxNoteAttachmentBytes {
		return manifest{}, ErrNoteAttachmentsLarge
	}
	sort.SliceStable(current.Attachments, func(i, j int) bool {
		if current.Attachments[i].CreatedAt != current.Attachments[j].CreatedAt {
			return current.Attachments[i].CreatedAt < current.Attachments[j].CreatedAt
		}
		return current.Attachments[i].ID < current.Attachments[j].ID
	})
	return current, nil
}

func (s *Store) writeManifestLocked(noteID string, current manifest) error {
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded)+1 > maxManifestBytes {
		return ErrManifestInvalid
	}
	return writeAtomic(s.manifestPath(noteID), append(encoded, '\n'))
}

func (s *Store) readAttachmentLocked(ctx context.Context, metadata Metadata) ([]byte, error) {
	_, plain, err := s.readStoredDataLocked(ctx, metadata, s.dataPath(metadata.NoteID, metadata.ID))
	return plain, err
}

func (s *Store) readStoredAttachmentLocked(ctx context.Context, metadata Metadata, storedPath string) ([]byte, error) {
	_, plain, err := s.readStoredDataLocked(ctx, metadata, storedPath)
	return plain, err
}

func (s *Store) readStoredDataLocked(ctx context.Context, metadata Metadata, storedPath string) ([]byte, []byte, error) {
	if err := s.validateStoredPathLocked(storedPath); err != nil {
		return nil, nil, err
	}
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		return nil, nil, err
	}
	plain := stored
	if protector := s.protectorValue(); protector != nil {
		plain, err = protector.DecodeAttachment(ctx, metadata.NoteID, metadata.ID, stored)
		if err != nil {
			return nil, nil, err
		}
	}
	if int64(len(plain)) != metadata.Size || hashBytes(plain) != metadata.SHA256 {
		return nil, nil, ErrManifestInvalid
	}
	if _, _, _, err := validateImage(metadata.MIMEType, plain); err != nil {
		return nil, nil, ErrManifestInvalid
	}
	return stored, plain, nil
}

func (s *Store) validateStoredAttachmentLocked(metadata Metadata) error {
	return s.validateStoredPathLocked(s.dataPath(metadata.NoteID, metadata.ID))
}

func (s *Store) validateStoredPathLocked(storedPath string) error {
	info, err := os.Lstat(storedPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > int64(maxStoredAttachmentBytes) {
		return ErrManifestInvalid
	}
	return nil
}

func (s *Store) noteDir(noteID string) string {
	return filepath.Join(s.rootDir, noteID)
}

func (s *Store) manifestPath(noteID string) string {
	return filepath.Join(s.noteDir(noteID), "manifest.json")
}

func (s *Store) dataPath(noteID string, attachmentID string) string {
	return filepath.Join(s.noteDir(noteID), attachmentID+".bin")
}

func (s *Store) stagePath(noteID string, attachmentID string, operationID string) string {
	return filepath.Join(s.noteDir(noteID), attachmentID+"."+operationID+".tmp")
}

func (s *Store) syncCopyDir(noteID string, operationID string) string {
	return filepath.Join(s.rootDir, syncCopyStageName(noteID, operationID))
}

func (s *Store) syncCopyManifestPath(noteID string, operationID string) string {
	return filepath.Join(s.syncCopyDir(noteID, operationID), "manifest.json")
}

func (s *Store) syncCopyDataPath(noteID string, operationID string, attachmentID string) string {
	return filepath.Join(s.syncCopyDir(noteID, operationID), attachmentID+".bin")
}

func (s *Store) deletePath(noteID string, operationID string) string {
	return filepath.Join(s.rootDir, noteID+"."+operationID+".delete")
}

func validateID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !safeIDPattern.MatchString(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func validateImage(inputMIME string, content []byte) (int, int, string, error) {
	if len(content) == 0 {
		return 0, 0, "", ErrInvalidImage
	}
	if len(content) > MaxAttachmentBytes {
		return 0, 0, "", ErrAttachmentTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return 0, 0, "", ErrInvalidImage
	}
	var mimeType string
	switch format {
	case "png":
		mimeType = "image/png"
	case "jpeg":
		mimeType = "image/jpeg"
	default:
		return 0, 0, "", ErrInvalidImage
	}
	if strings.TrimSpace(inputMIME) != mimeType {
		return 0, 0, "", ErrInvalidImage
	}
	if config.Width < 1 || config.Width > MaxImageWidth || config.Height < 1 || config.Height > MaxImageHeight || int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return 0, 0, "", ErrAttachmentDimensions
	}
	return config.Width, config.Height, mimeType, nil
}

func safeAttachmentName(name string, mimeType string, id string) string {
	value := strings.TrimSpace(name)
	value = filepath.Base(value)
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f || character == '/' || character == '\\' {
			return '_'
		}
		return character
	}, value)
	value = strings.Trim(value, " .")
	extension := ".png"
	if mimeType == "image/jpeg" {
		extension = ".jpg"
	}
	if value == "" || value == "." || value == ".." {
		value = "image"
	}
	currentExtension := strings.ToLower(filepath.Ext(value))
	if currentExtension != ".png" && currentExtension != ".jpg" && currentExtension != ".jpeg" {
		value += extension
	}
	if id != "" && value == "image"+extension {
		value = "image-" + id + extension
	}
	if len(value) > 120 {
		value = value[:120]
		for len(value) > 0 && !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func Reference(noteID string, attachmentID string) string {
	return "atlasnote-attachment://" + noteID + "/" + attachmentID
}

func ParseReference(value string) (string, string, bool) {
	prefix := "atlasnote-attachment://"
	if !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(value, prefix), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	noteID, noteErr := validateID(parts[0])
	attachmentID, attachmentErr := validateID(parts[1])
	return noteID, attachmentID, noteErr == nil && attachmentErr == nil
}

func hashBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newAttachmentID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func ensureDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrManifestInvalid
	}
	return nil
}

func isDeleteStageName(name string) bool {
	return strings.HasSuffix(name, ".delete")
}

func syncCopyStageName(noteID string, operationID string) string {
	return noteID + "." + operationID + syncCopyStageSuffix
}

func isSyncCopyStageName(name string) bool {
	return strings.HasSuffix(name, syncCopyStageSuffix)
}

func removeAttachmentPath(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrManifestInvalid
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove attachment directory: %w", err)
	}
	return nil
}

func writeAtomic(path string, content []byte) error {
	directory := filepath.Dir(path)
	if err := ensureDirectory(directory); err != nil {
		return err
	}
	return fileatomic.Write(path, content)
}
