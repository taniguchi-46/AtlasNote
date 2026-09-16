package app

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"

	attachmentstore "atlasnote/internal/attachment"
	"atlasnote/internal/contentlock"
	"atlasnote/internal/noteexport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var errProtectedAttachmentExportConsent = errors.New("保護された添付ファイルの平文ZIP保存には明示的な確認が必要です。")
var errAttachmentExportStateChanged = errors.New("添付ファイルまたはノートの保存状態が変わったため、ZIP保存を中止しました。")

type NoteAttachment struct {
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
	Reference string `json:"reference"`
}

type SaveNoteAttachmentInput struct {
	NoteID   string `json:"noteId"`
	Kind     string `json:"kind"`
	MIMEType string `json:"mimeType"`
	Name     string `json:"name"`
	Data     string `json:"data"`
}

type GetNoteAttachmentInput struct {
	NoteID       string `json:"noteId"`
	AttachmentID string `json:"attachmentId"`
}

type NoteAttachmentData struct {
	Attachment NoteAttachment `json:"attachment"`
	Data       string         `json:"data"`
}

type SaveNoteAttachmentsResult struct {
	Saved     bool   `json:"saved"`
	Cancelled bool   `json:"cancelled"`
	SavedName string `json:"savedName,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *App) SaveNoteAttachment(input SaveNoteAttachmentInput) (NoteAttachment, error) {
	if a.attachments == nil {
		return NoteAttachment{}, errors.New("添付ファイル機能を利用できません。")
	}
	if err := a.ensureNoteAttachmentAccess(input.NoteID); err != nil {
		return NoteAttachment{}, attachmentAPIError(err)
	}
	encoded := strings.TrimSpace(input.Data)
	maxEncodedBytes := ((attachmentstore.MaxAttachmentBytes + 2) / 3) * 4
	if len(encoded) > maxEncodedBytes {
		return NoteAttachment{}, attachmentAPIError(attachmentstore.ErrAttachmentTooLarge)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return NoteAttachment{}, attachmentAPIError(attachmentstore.ErrInvalidImage)
	}
	operationContext, release := a.notes.BeginSyncExclusive(a.operationContext())
	defer release()
	if err := a.ensureNoteAttachmentAccessContext(operationContext, input.NoteID); err != nil {
		return NoteAttachment{}, attachmentAPIError(err)
	}
	releaseContent := func() {}
	if a.contentLocks != nil {
		releaseContent = a.contentLocks.BeginContentAccess(operationContext)
	}
	defer releaseContent()
	if a.contentLocks != nil {
		if err := a.contentLocks.AssertNoteAccess(operationContext, input.NoteID); err != nil {
			return NoteAttachment{}, attachmentAPIError(err)
		}
	}
	result, err := a.attachments.Save(operationContext, attachmentstore.SaveInput{
		NoteID: input.NoteID, Kind: input.Kind, MIMEType: input.MIMEType, Name: input.Name, Data: data,
	})
	if err != nil {
		return NoteAttachment{}, attachmentAPIError(err)
	}
	return noteAttachmentFromMetadata(result.Metadata, result.Reference), nil
}

func (a *App) ListNoteAttachments(noteID string) ([]NoteAttachment, error) {
	if a.attachments == nil {
		return []NoteAttachment{}, errors.New("添付ファイル機能を利用できません。")
	}
	operationContext, release := a.notes.BeginStorageSnapshot(a.operationContext())
	defer release()
	if err := a.ensureNoteAttachmentAccessContext(operationContext, noteID); err != nil {
		return []NoteAttachment{}, attachmentAPIError(err)
	}
	releaseContent := func() {}
	if a.contentLocks != nil {
		releaseContent = a.contentLocks.BeginContentAccess(operationContext)
	}
	defer releaseContent()
	if a.contentLocks != nil {
		if err := a.contentLocks.AssertNoteAccess(operationContext, noteID); err != nil {
			return []NoteAttachment{}, attachmentAPIError(err)
		}
	}
	items, err := a.attachments.List(operationContext, noteID)
	if err != nil {
		return []NoteAttachment{}, attachmentAPIError(err)
	}
	result := make([]NoteAttachment, 0, len(items))
	for _, item := range items {
		result = append(result, noteAttachmentFromMetadata(item, attachmentstore.Reference(item.NoteID, item.ID)))
	}
	return result, nil
}

func (a *App) GetNoteAttachment(input GetNoteAttachmentInput) (NoteAttachmentData, error) {
	if a.attachments == nil {
		return NoteAttachmentData{}, errors.New("添付ファイル機能を利用できません。")
	}
	operationContext, release := a.notes.BeginStorageSnapshot(a.operationContext())
	defer release()
	if err := a.ensureNoteAttachmentAccessContext(operationContext, input.NoteID); err != nil {
		return NoteAttachmentData{}, attachmentAPIError(err)
	}
	releaseContent := func() {}
	if a.contentLocks != nil {
		releaseContent = a.contentLocks.BeginContentAccess(operationContext)
	}
	defer releaseContent()
	if a.contentLocks != nil {
		if err := a.contentLocks.AssertNoteAccess(operationContext, input.NoteID); err != nil {
			return NoteAttachmentData{}, attachmentAPIError(err)
		}
	}
	metadata, data, err := a.attachments.Read(operationContext, input.NoteID, input.AttachmentID)
	if err != nil {
		return NoteAttachmentData{}, attachmentAPIError(err)
	}
	return NoteAttachmentData{
		Attachment: noteAttachmentFromMetadata(metadata, attachmentstore.Reference(metadata.NoteID, metadata.ID)),
		Data:       base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (a *App) SaveNoteAttachments(noteID string, title string, expectedRevision int64, allowPlaintextProtected bool) SaveNoteAttachmentsResult {
	if a.attachments == nil {
		return SaveNoteAttachmentsResult{Error: "添付ファイル機能を利用できません。"}
	}
	if expectedRevision < 1 {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(errAttachmentExportStateChanged)}
	}
	if err := a.ensureNoteAttachmentAccess(noteID); err != nil {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
	}
	if !a.exportMu.TryLock() {
		return SaveNoteAttachmentsResult{Error: "別のファイル保存処理が実行中です。"}
	}
	defer a.exportMu.Unlock()

	initialContext, releaseInitial := a.notes.BeginStorageSnapshot(a.operationContext())
	items, err := a.attachments.List(initialContext, noteID)
	releaseInitial()
	if err != nil {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
	}
	if len(items) == 0 {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(attachmentstore.ErrNoAttachments)}
	}
	baseName := noteexport.SuggestedFilename(title, noteexport.FormatTXT)
	baseName = strings.TrimSuffix(baseName, ".txt") + "-attachments.zip"
	options := runtime.SaveDialogOptions{
		Title:                "添付ファイルを保存",
		DefaultFilename:      baseName,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{{
			DisplayName: "ZIPファイル (*.zip)", Pattern: "*.zip",
		}},
	}
	selectFile := a.saveAttachmentsFile
	if selectFile == nil {
		selectFile = runtime.SaveFileDialog
	}
	path, err := selectFile(a.operationContext(), options)
	if err != nil {
		return SaveNoteAttachmentsResult{Error: "添付ファイルの保存先を選択できませんでした。"}
	}
	if strings.TrimSpace(path) == "" {
		return SaveNoteAttachmentsResult{Cancelled: true}
	}
	operationContext, releaseSnapshot := a.notes.BeginStorageSnapshot(a.operationContext())
	defer releaseSnapshot()
	current, err := a.notes.Get(operationContext, noteID)
	if err != nil {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
	}
	releaseExportAccess := func() {}
	if a.contentLocks != nil {
		releaseExportAccess = a.contentLocks.BeginExportContentAccess(operationContext)
	}
	defer releaseExportAccess()
	if a.contentLocks != nil {
		if err := a.contentLocks.AssertNoteAccess(operationContext, noteID); err != nil {
			return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
		}
		protected, locked, _, statusErr := a.contentLocks.NoteLockStatus(operationContext, noteID)
		if statusErr != nil {
			return SaveNoteAttachmentsResult{Error: attachmentUserMessage(statusErr)}
		}
		current.Protected = protected
		current.Locked = locked
	}
	if current.Revision != expectedRevision {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(errAttachmentExportStateChanged)}
	}
	if current.Protected && !allowPlaintextProtected {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(errProtectedAttachmentExportConsent)}
	}
	currentItems, err := a.attachments.List(operationContext, noteID)
	if err != nil {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
	}
	if !sameAttachmentManifest(items, currentItems) {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(errAttachmentExportStateChanged)}
	}
	archive, err := a.attachments.ExportZip(operationContext, noteID)
	if err != nil {
		return SaveNoteAttachmentsResult{Error: attachmentUserMessage(err)}
	}
	if err := noteexport.WriteFileAtomic(path, archive); err != nil {
		return SaveNoteAttachmentsResult{Error: "添付ファイルを保存できませんでした。"}
	}
	return SaveNoteAttachmentsResult{Saved: true, SavedName: filepath.Base(path)}
}

func sameAttachmentManifest(before []attachmentstore.Metadata, after []attachmentstore.Metadata) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range before {
		if before[index] != after[index] {
			return false
		}
	}
	return true
}

func (a *App) ensureNoteAttachmentAccess(noteID string) error {
	return a.ensureNoteAttachmentAccessContext(a.operationContext(), noteID)
}

func (a *App) ensureNoteAttachmentAccessContext(ctx context.Context, noteID string) error {
	if a.notes == nil {
		return errors.New("note service is not initialized")
	}
	_, err := a.notes.Get(ctx, noteID)
	return err
}

func noteAttachmentFromMetadata(metadata attachmentstore.Metadata, reference string) NoteAttachment {
	return NoteAttachment{
		ID: metadata.ID, NoteID: metadata.NoteID, Kind: metadata.Kind, MIMEType: metadata.MIMEType,
		Name: metadata.Name, Size: metadata.Size, SHA256: metadata.SHA256, Width: metadata.Width,
		Height: metadata.Height, CreatedAt: metadata.CreatedAt, Reference: reference,
	}
}

func attachmentAPIError(err error) error {
	return errors.New(attachmentUserMessage(err))
}

func attachmentUserMessage(err error) string {
	switch {
	case errors.Is(err, errProtectedAttachmentExportConsent):
		return errProtectedAttachmentExportConsent.Error()
	case errors.Is(err, errAttachmentExportStateChanged):
		return errAttachmentExportStateChanged.Error()
	case errors.Is(err, attachmentstore.ErrInvalidInput), errors.Is(err, attachmentstore.ErrInvalidImage):
		return "PNGまたはJPEG画像の入力を確認してください。"
	case errors.Is(err, attachmentstore.ErrAttachmentTooLarge):
		return "画像が大きすぎます。1画像10 MiB以内で指定してください。"
	case errors.Is(err, attachmentstore.ErrAttachmentDimensions):
		return "画像の寸法または画素数が上限を超えています。"
	case errors.Is(err, attachmentstore.ErrTooManyAttachments):
		return "このノートの添付数が上限に達しています。"
	case errors.Is(err, attachmentstore.ErrNoteAttachmentsLarge):
		return "このノートの添付容量が上限に達しています。"
	case errors.Is(err, attachmentstore.ErrAttachmentNotFound):
		return "添付ファイルが見つかりません。"
	case errors.Is(err, attachmentstore.ErrNoAttachments):
		return "保存する添付ファイルがありません。"
	case errors.Is(err, contentlock.ErrLocked):
		return "本文ロックを解除してから添付ファイルを操作してください。"
	default:
		return "添付ファイルを処理できませんでした。"
	}
}
