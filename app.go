package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	aiservice "atlasnote/internal/ai"
	"atlasnote/internal/config"
	"atlasnote/internal/contentlock"
	"atlasnote/internal/credential"
	"atlasnote/internal/database"
	"atlasnote/internal/datalock"
	"atlasnote/internal/note"
	"atlasnote/internal/noteexport"
	"atlasnote/internal/noteimport"
	"atlasnote/internal/notespace"
	"atlasnote/internal/storage"
	syncservice "atlasnote/internal/sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                context.Context
	buildType          string
	db                 *sql.DB
	dataLock           *datalock.Lock
	markdownStore      *storage.MarkdownStore
	contentLocks       *contentlock.Manager
	notes              *note.Service
	noteExporter       *noteexport.Service
	noteImporter       *noteimport.Service
	syncService        *syncservice.Service
	aiService          *aiservice.Service
	spaceRegistry      *notespace.Registry
	activeSpace        notespace.Space
	dataDir            string
	notesDir           string
	startupErr         error
	startupLocked      bool
	recoveryReport     note.RecoveryReport
	syncRecoveryBackup string
	statusMu           sync.RWMutex
	closeMu            sync.Mutex
	closeRequested     bool
	allowClose         bool
	importMu           sync.Mutex
	openImportFiles    func(context.Context, runtime.OpenDialogOptions) ([]string, error)
	exportMu           sync.Mutex
	saveExportFile     func(context.Context, runtime.SaveDialogOptions) (string, error)
	restartExecutable  string
	startProcess       func(string) error
	quitApplication    func(context.Context)
}

var (
	errRestartUnavailable     = errors.New("automatic restart is unavailable")
	errRestartDevelopmentMode = errors.New("automatic restart is unavailable in Wails development mode")
	errProtectedContentSync   = errors.New("本文ロックが設定されている保存空間では同期できません。保護済み本文を同期する暗号化形式は未対応です")
)

type StartupStatus struct {
	Ready              bool                    `json:"ready"`
	Locked             bool                    `json:"locked"`
	Degraded           bool                    `json:"degraded"`
	Message            string                  `json:"message,omitempty"`
	DataDir            string                  `json:"dataDir,omitempty"`
	MissingNotes       []MissingNoteDiagnostic `json:"missingNotes"`
	SyncRecoveryBackup string                  `json:"syncRecoveryBackup,omitempty"`
	ActiveStorageSpace *notespace.Space        `json:"activeStorageSpace,omitempty"`
}

type MissingNoteDiagnostic struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FilePath string `json:"filePath"`
}

// StorageSpaceLockStatus is deliberately limited to lock metadata so the
// settings screen can show every storage space without opening its notes.
type StorageSpaceLockStatus struct {
	SpaceID   string                `json:"spaceId"`
	Protected bool                  `json:"protected"`
	Locked    bool                  `json:"locked"`
	Error     *contentlock.APIError `json:"error,omitempty"`
}

type StorageSpaceLockStatusResult struct {
	Statuses []StorageSpaceLockStatus `json:"statuses"`
	Error    *contentlock.APIError    `json:"error,omitempty"`
}

func NewApp() *App {
	app := &App{}
	app.initialize(context.Background())
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.buildType = runtime.Environment(ctx).BuildType
}

func (a *App) shutdown(ctx context.Context) {
	if a.aiService != nil {
		a.aiService.Shutdown()
	}
	a.noteImporter = nil
	a.noteExporter = nil
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	if a.contentLocks != nil {
		a.contentLocks.Close()
		a.contentLocks = nil
	}
	a.markdownStore = nil
	if a.dataLock != nil {
		_ = a.dataLock.Release()
		a.dataLock = nil
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.closeMu.Lock()
	// フロントエンドでの保存処理が完了し、終了が許可された場合は false を返して終了プロセスを続行する
	if a.allowClose {
		a.closeMu.Unlock()
		return false
	}
	// すでに終了リクエストをフロントエンドに送信済みの場合は、重複してイベントを送らないようにする
	if a.closeRequested {
		a.closeMu.Unlock()
		return true
	}
	a.closeRequested = true
	a.closeMu.Unlock()

	// 即座にアプリを終了させず、フロントエンドに対して終了処理のフック（app:before-close）を通知する。
	// これにより、フロントエンド側で未保存のノートの非同期保存（フラッシュ）を完了させる猶予を与える。
	// true を返すとWails側でのウィンドウ終了処理が一旦キャンセルされる。
	runtime.EventsEmit(ctx, "app:before-close")
	return true
}

func (a *App) CompleteClose() {
	a.closeMu.Lock()
	a.allowClose = true
	a.closeRequested = false
	a.closeMu.Unlock()

	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

func (a *App) RestartApp() error {
	executable, err := os.Executable()
	if err != nil {
		return errRestartUnavailable
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return errRestartUnavailable
	}

	a.closeMu.Lock()
	if a.ctx == nil {
		a.closeMu.Unlock()
		return errRestartUnavailable
	}
	// wails dev owns the frontend server and the generated *-dev executable.
	// Relaunching only that child binary leaves it without its server and prevents
	// the parent CLI from deleting the old executable on Windows.
	if a.buildType == "dev" {
		a.closeMu.Unlock()
		return errRestartDevelopmentMode
	}
	a.restartExecutable = executable
	a.allowClose = true
	a.closeRequested = false
	ctx := a.ctx
	quitApplication := a.quitApplication
	a.closeMu.Unlock()

	if quitApplication != nil {
		quitApplication(ctx)
	} else {
		runtime.Quit(ctx)
	}
	return nil
}

func (a *App) launchRestartIfRequested() error {
	a.closeMu.Lock()
	executable := a.restartExecutable
	a.restartExecutable = ""
	startProcess := a.startProcess
	a.closeMu.Unlock()
	if executable == "" {
		return nil
	}
	if startProcess != nil {
		return startProcess(executable)
	}
	return startDetachedProcess(executable)
}

func startDetachedProcess(executable string) error {
	// executable is resolved by os.Executable and never comes from user input.
	command := exec.Command(executable)
	if err := command.Start(); err != nil {
		return err
	}
	if command.Process != nil {
		_ = command.Process.Release()
	}
	return nil
}

func (a *App) CancelClose() {
	a.closeMu.Lock()
	a.closeRequested = false
	a.closeMu.Unlock()
}

func (a *App) CreateNote(input note.CreateInput) (note.Note, error) {
	if a.notes == nil {
		return note.Note{}, errors.New("note service is not initialized")
	}
	return a.notes.Create(a.ctx, input)
}

// ImportNotes opens the native file picker in the backend so source paths are
// never accepted from the frontend bridge. The note import service delegates
// every write to note.Service and therefore keeps the existing journal, index,
// sync, and content-lock guarantees intact.
func (a *App) ImportNotes(input noteimport.Input) (noteimport.Result, error) {
	if a.noteImporter == nil {
		return noteimport.NewResult(), errors.New("note import service is not initialized")
	}
	if !a.importMu.TryLock() {
		return noteimport.NewErrorResult(noteimport.ErrorCodeBusy, "別のインポート処理が実行中です。", true), nil
	}
	defer a.importMu.Unlock()

	paths, err := a.selectImportFiles(a.operationContext())
	if err != nil {
		return noteimport.NewResult(), err
	}
	if len(paths) == 0 {
		result := noteimport.NewResult()
		result.Cancelled = true
		return result, nil
	}
	return a.noteImporter.Import(a.operationContext(), paths, input), nil
}

// ExportNote uses a backend-native save dialog so the frontend never receives
// or supplies a filesystem destination. The export gate starts only after the
// dialog closes, then keeps the final note snapshot and atomic file write on
// the same side of any content-lock conversion.
func (a *App) ExportNote(input noteexport.Input) (noteexport.Result, error) {
	if a.noteExporter == nil {
		return noteexport.NewErrorResult(noteexport.ErrorCodeUnavailable, "エクスポートを開始できませんでした。", "", true), nil
	}
	if validationError := noteexport.ValidateInput(input); validationError != nil {
		return noteexport.Result{Error: validationError}, nil
	}
	if !a.exportMu.TryLock() {
		return noteexport.NewErrorResult(noteexport.ErrorCodeBusy, "別のエクスポート処理が実行中です。", "", true), nil
	}
	defer a.exportMu.Unlock()

	path, err := a.selectExportFile(a.operationContext(), input)
	if err != nil {
		return noteexport.NewErrorResult(noteexport.ErrorCodeUnavailable, "保存先を選択できませんでした。", "", true), nil
	}
	if path == "" {
		return noteexport.Result{Cancelled: true}, nil
	}

	releaseExportAccess := func() {}
	if a.contentLocks != nil {
		releaseExportAccess = a.contentLocks.BeginExportContentAccess(a.operationContext())
	}
	defer releaseExportAccess()
	return a.noteExporter.Export(a.operationContext(), path, input)
}

func (a *App) selectExportFile(ctx context.Context, input noteexport.Input) (string, error) {
	options := runtime.SaveDialogOptions{
		DefaultFilename:      noteexport.SuggestedFilename(input.Title, input.Format),
		CanCreateDirectories: true,
	}
	switch input.Format {
	case noteexport.FormatHTML:
		options.Title = "HTMLとしてエクスポート"
		options.Filters = []runtime.FileFilter{{DisplayName: "HTMLファイル (*.html)", Pattern: "*.html"}}
	case noteexport.FormatPDF:
		options.Title = "PDFとしてエクスポート"
		options.Filters = []runtime.FileFilter{{DisplayName: "PDFファイル (*.pdf)", Pattern: "*.pdf"}}
	}
	if a.saveExportFile != nil {
		return a.saveExportFile(ctx, options)
	}
	return runtime.SaveFileDialog(ctx, options)
}

func (a *App) selectImportFiles(ctx context.Context) ([]string, error) {
	options := runtime.OpenDialogOptions{
		Title: "ノートをインポート",
		Filters: []runtime.FileFilter{
			{DisplayName: "ノートファイル (*.md, *.txt, *.html, *.htm)", Pattern: "*.md;*.txt;*.html;*.htm"},
		},
	}
	if a.openImportFiles != nil {
		return a.openImportFiles(ctx, options)
	}
	return runtime.OpenMultipleFilesDialog(ctx, options)
}

func (a *App) ListNotes() ([]note.Summary, error) {
	if a.notes == nil {
		return nil, errors.New("note service is not initialized")
	}
	return a.notes.List(a.ctx)
}

func (a *App) ListNotesPage(input note.NoteListInput) (note.NoteListResult, error) {
	if a.notes == nil {
		return note.NoteListResult{Items: make([]note.Summary, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.ListPage(a.ctx, input)
}

func (a *App) SearchNotes(input note.SearchInput) (note.SearchResult, error) {
	if a.notes == nil {
		return note.SearchResult{Items: make([]note.SearchItem, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.Search(a.ctx, input)
}

func (a *App) ListBacklinks(input note.BacklinkListInput) (note.BacklinkListResult, error) {
	if a.notes == nil {
		return note.BacklinkListResult{Items: make([]note.Summary, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.ListBacklinks(a.ctx, input)
}

func (a *App) GetNote(id string) (note.Note, error) {
	if a.notes == nil {
		return note.Note{}, errors.New("note service is not initialized")
	}
	return a.notes.Get(a.ctx, id)
}

func (a *App) UpdateNote(id string, input note.UpdateInput) (note.UpdateNoteResult, error) {
	if a.notes == nil {
		return note.UpdateNoteResult{}, errors.New("note service is not initialized")
	}
	updated, err := a.notes.Update(a.ctx, id, input)
	if err != nil {
		var conflict *note.RevisionConflict
		if errors.As(err, &conflict) {
			return note.UpdateNoteResult{Conflict: conflict}, nil
		}
		return note.UpdateNoteResult{}, err
	}

	return note.UpdateNoteResult{Note: &updated}, nil
}

func (a *App) DeleteNote(id string, input note.DeleteInput) (note.DeleteNoteResult, error) {
	if a.notes == nil {
		return note.DeleteNoteResult{}, errors.New("note service is not initialized")
	}
	if err := a.notes.Delete(a.ctx, id, input); err != nil {
		var conflict *note.RevisionConflict
		if errors.As(err, &conflict) {
			return note.DeleteNoteResult{Conflict: conflict}, nil
		}
		return note.DeleteNoteResult{}, err
	}

	return note.DeleteNoteResult{Deleted: true}, nil
}

func (a *App) DeleteMissingNote(id string) (StartupStatus, error) {
	if a.notes == nil {
		return a.GetStartupStatus(), errors.New("note service is not initialized")
	}
	if err := a.notes.DeleteMissing(a.ctx, id); err != nil {
		return a.GetStartupStatus(), err
	}
	return a.ReinspectRecovery()
}

func (a *App) ReinspectRecovery() (StartupStatus, error) {
	if a.notes == nil {
		return a.GetStartupStatus(), errors.New("note service is not initialized")
	}
	report, err := a.notes.Recover(a.ctx)
	if err != nil {
		return a.GetStartupStatus(), err
	}
	a.statusMu.Lock()
	a.recoveryReport = report
	a.statusMu.Unlock()
	return a.GetStartupStatus(), nil
}

func (a *App) CreateNotebook(input note.NotebookCreateInput) (note.Notebook, error) {
	if a.notes == nil {
		return note.Notebook{}, errors.New("note service is not initialized")
	}
	return a.notes.CreateNotebook(a.ctx, input)
}

func (a *App) ListNotebooks() ([]note.Notebook, error) {
	if a.notes == nil {
		return nil, errors.New("note service is not initialized")
	}
	return a.notes.ListNotebooks(a.ctx)
}

func (a *App) UpdateNotebook(id string, input note.NotebookUpdateInput) (note.Notebook, error) {
	if a.notes == nil {
		return note.Notebook{}, errors.New("note service is not initialized")
	}
	return a.notes.UpdateNotebook(a.ctx, id, input)
}

func (a *App) DeleteNotebook(id string, input note.NotebookDeleteInput) error {
	if a.notes == nil {
		return errors.New("note service is not initialized")
	}
	return a.notes.DeleteNotebook(a.ctx, id, input)
}

func (a *App) ListTags() ([]note.Tag, error) {
	if a.notes == nil {
		return make([]note.Tag, 0), errors.New("note service is not initialized")
	}
	return a.notes.ListTags(a.ctx)
}

func (a *App) ListNoteTags(noteID string) (note.NoteTagsResult, error) {
	if a.notes == nil {
		return note.NoteTagsResult{Tags: make([]note.Tag, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.ListNoteTags(a.ctx, noteID)
}

func (a *App) CreateTag(input note.TagCreateInput) (note.TagMutationResult, error) {
	if a.notes == nil {
		return note.TagMutationResult{}, errors.New("note service is not initialized")
	}
	return a.notes.CreateTag(a.ctx, input)
}

func (a *App) UpdateTag(tagID string, input note.TagUpdateInput) (note.TagMutationResult, error) {
	if a.notes == nil {
		return note.TagMutationResult{}, errors.New("note service is not initialized")
	}
	return a.notes.UpdateTag(a.ctx, tagID, input)
}

func (a *App) DeleteTag(tagID string) (note.TagDeleteResult, error) {
	if a.notes == nil {
		return note.TagDeleteResult{}, errors.New("note service is not initialized")
	}
	return a.notes.DeleteTag(a.ctx, tagID)
}

func (a *App) SetNoteTags(noteID string, input note.SetNoteTagsInput) (note.NoteTagsResult, error) {
	if a.notes == nil {
		return note.NoteTagsResult{Tags: make([]note.Tag, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.SetNoteTags(a.ctx, noteID, input)
}

func (a *App) SetNoteTagsWithExpectedRevision(noteID string, input note.SetNoteTagsWithExpectedRevisionInput) (note.NoteTagsResult, error) {
	if a.notes == nil {
		return note.NoteTagsResult{Tags: make([]note.Tag, 0)}, errors.New("note service is not initialized")
	}
	return a.notes.SetNoteTagsWithExpectedRevision(a.ctx, noteID, input)
}

func (a *App) GetSyncStatus() (syncservice.StatusResult, error) {
	if a.syncService == nil {
		return syncservice.StatusResult{Status: syncservice.StatusDisabled}, errors.New("sync service is not initialized")
	}
	return a.syncService.GetStatus(a.ctx)
}

func (a *App) ConfigureSync(input syncservice.ConnectionInput) (syncservice.StatusResult, error) {
	if a.syncService == nil {
		return syncservice.StatusResult{Status: syncservice.StatusDisabled}, errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return syncservice.StatusResult{Status: syncservice.StatusDisabled}, err
	}
	return a.syncService.Configure(a.ctx, input)
}

func (a *App) TestSyncConfiguration(input syncservice.ConnectionInput) (syncservice.ConfigurationTestResult, error) {
	if a.syncService == nil {
		return syncservice.ConfigurationTestResult{}, errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return syncservice.ConfigurationTestResult{}, err
	}
	return a.syncService.TestConfiguration(a.ctx, input)
}

func (a *App) SyncNow(input syncservice.SyncNowInput) (syncservice.SyncResult, error) {
	if a.syncService == nil {
		return syncservice.SyncResult{Status: syncservice.StatusDisabled}, errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return syncservice.SyncResult{Status: syncservice.StatusDisabled, Message: err.Error()}, err
	}
	return a.syncService.SyncNow(a.ctx, input)
}

func (a *App) ensureSyncAllowed() error {
	if a.contentLocks == nil {
		return nil
	}
	hasLocks, err := a.contentLocks.HasContentLocks(a.operationContext())
	if err != nil {
		return err
	}
	if hasLocks {
		return errProtectedContentSync
	}
	return nil
}

func (a *App) ResolveSyncConflict(input syncservice.ConflictResolutionInput) error {
	if a.syncService == nil {
		return errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return err
	}
	return a.syncService.ResolveConflict(a.ctx, input)
}

func (a *App) ListSyncConflicts() ([]syncservice.ConflictSummary, error) {
	if a.syncService == nil {
		return []syncservice.ConflictSummary{}, errors.New("sync service is not initialized")
	}
	return a.syncService.ListConflicts(a.ctx)
}

func (a *App) DisconnectSync() error {
	if a.syncService == nil {
		return errors.New("sync service is not initialized")
	}
	return a.syncService.Disconnect(a.ctx)
}

func (a *App) GetAISettings() ([]aiservice.ProviderSettings, error) {
	if a.aiService == nil {
		return []aiservice.ProviderSettings{}, errors.New("AI service is not initialized")
	}
	return a.aiService.GetSettings(a.ctx)
}

func (a *App) ConfigureAIProvider(input aiservice.ConfigureProviderInput) ([]aiservice.ProviderSettings, error) {
	if a.aiService == nil {
		return []aiservice.ProviderSettings{}, errors.New("AI service is not initialized")
	}
	return a.aiService.Configure(a.ctx, input)
}

func (a *App) TestAIConnection(input aiservice.TestConnectionInput) (aiservice.ConnectionTestResult, error) {
	if a.aiService == nil {
		return aiservice.ConnectionTestResult{}, errors.New("AI service is not initialized")
	}
	return a.aiService.TestConnection(a.ctx, input)
}

// TestAIGeneration sends only the service's fixed probe content to the
// selected model and discards the output. It is intentionally separate from
// authentication/model-list checks so the UI can verify actual generation.
func (a *App) TestAIGeneration(input aiservice.TestGenerationInput) (aiservice.ConnectionTestResult, error) {
	if a.aiService == nil {
		return aiservice.ConnectionTestResult{}, errors.New("AI service is not initialized")
	}
	return a.aiService.TestGeneration(a.ctx, input)
}

func (a *App) UpdateAIProviderModel(input aiservice.UpdateProviderModelInput) ([]aiservice.ProviderSettings, error) {
	if a.aiService == nil {
		return []aiservice.ProviderSettings{}, errors.New("AI service is not initialized")
	}
	return a.aiService.UpdateProviderModel(a.ctx, input)
}

// ListAIModels returns only normalized model metadata and a typed safe error.
// The draft API key is consumed by the Go service and never appears in this
// response or in a Wails error string.
func (a *App) ListAIModels(input aiservice.ListModelsInput) aiservice.ModelListResponse {
	if a.aiService == nil {
		return aiservice.ModelListResponse{
			Models: []aiservice.ModelInfo{},
			Error:  aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable),
		}
	}
	result, err := a.aiService.ListModels(a.ctx, input)
	if err != nil {
		return aiservice.ModelListResponse{
			Models: []aiservice.ModelInfo{},
			Error:  aiservice.SafeErrorFrom(err),
		}
	}
	return aiservice.ModelListResponse{Models: result.Models, RetrievedAt: result.RetrievedAt}
}

// GenerateAISummary resolves the selected provider credential internally and
// returns no raw provider error to the Wails boundary.
func (a *App) GenerateAISummary(input aiservice.GenerateSummaryInput) aiservice.SummaryResponse {
	if a.aiService == nil {
		return aiservice.SummaryResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.validateAISummarySource(input); err != nil {
		return aiservice.SummaryResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	result, err := a.aiService.GenerateSummary(a.ctx, input)
	if err != nil {
		return aiservice.SummaryResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.SummaryResponse{Text: result.Text}
}

// validateAISummarySource keeps the raw-body summary endpoint tied to the
// saved note it claims to summarize. In particular, a renderer cannot submit
// a protected note body while naming an unrelated unprotected note.
func (a *App) validateAISummarySource(input aiservice.GenerateSummaryInput) error {
	if a.notes == nil {
		// Service-only callers used by non-Wails tests do not have a note
		// repository. The production application always initializes notes before
		// exposing this endpoint, where the source binding below is mandatory.
		return nil
	}
	noteID := strings.TrimSpace(input.NoteID)
	if noteID == "" {
		return aiservice.ErrInputInvalid
	}
	if a.contentLocks != nil {
		if err := a.contentLocks.AssertAIAllowed(a.operationContext(), noteID); err != nil {
			return aiservice.ErrInputInvalid
		}
	}
	item, err := a.notes.Get(a.operationContext(), noteID)
	if err != nil || item.IsTrashed || item.Content != input.Content {
		return aiservice.ErrInputInvalid
	}
	return nil
}

func (a *App) PrepareAIContext(input aiservice.AIContextInput) aiservice.AIContextResponse {
	if a.aiService == nil {
		return aiservice.AIContextResponse{Sources: []aiservice.AIContextSource{}, Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	sources, err := a.aiService.PrepareContext(a.ctx, input)
	if err != nil {
		return aiservice.AIContextResponse{Sources: []aiservice.AIContextSource{}, Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIContextResponse{Sources: sources}
}

func (a *App) RunAIAssistant(input aiservice.AssistantInput) aiservice.AssistantResponse {
	if a.aiService == nil {
		return aiservice.AssistantResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	result, err := a.aiService.RunAssistant(a.ctx, input)
	if err != nil {
		return aiservice.AssistantResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AssistantResponse{Result: &result}
}

func (a *App) CancelAIAssistant(requestID string) aiservice.AssistantCancelResponse {
	if a.aiService == nil {
		return aiservice.AssistantCancelResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	return aiservice.AssistantCancelResponse{Canceled: a.aiService.CancelAssistant(requestID)}
}

func (a *App) SaveAIHistory(input aiservice.SaveAIHistoryInput) aiservice.AIHistoryResponse {
	if a.aiService == nil {
		return aiservice.AIHistoryResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	history, err := a.aiService.SaveHistory(a.ctx, input)
	if err != nil {
		return aiservice.AIHistoryResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIHistoryResponse{History: &history}
}

func (a *App) ListAIHistories() aiservice.AIHistoryListResponse {
	if a.aiService == nil {
		return aiservice.AIHistoryListResponse{Items: []aiservice.AIHistory{}, Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	items, err := a.aiService.ListHistories(a.ctx)
	if err != nil {
		return aiservice.AIHistoryListResponse{Items: []aiservice.AIHistory{}, Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIHistoryListResponse{Items: items}
}

func (a *App) GetAIHistory(id string) aiservice.AIHistoryResponse {
	if a.aiService == nil {
		return aiservice.AIHistoryResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	history, err := a.aiService.GetHistory(a.ctx, id)
	if err != nil {
		return aiservice.AIHistoryResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIHistoryResponse{History: &history}
}

func (a *App) DeleteAIHistory(id string) aiservice.AIDeleteResponse {
	if a.aiService == nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.aiService.DeleteHistory(a.ctx, id); err != nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIDeleteResponse{Deleted: true}
}

func (a *App) DeleteAllAIHistories() aiservice.AIDeleteResponse {
	if a.aiService == nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.aiService.DeleteAllHistories(a.ctx); err != nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIDeleteResponse{Deleted: true}
}

func (a *App) RunAIWriting(input aiservice.WritingInput) aiservice.WritingResponse {
	if a.aiService == nil {
		return aiservice.WritingResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	result, err := a.aiService.RunWriting(a.ctx, input)
	if err != nil {
		return aiservice.WritingResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.WritingResponse{Result: &result}
}

func (a *App) SaveAIArtifact(input aiservice.SaveAIArtifactInput) aiservice.AIArtifactResponse {
	if a.aiService == nil {
		return aiservice.AIArtifactResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	artifact, err := a.aiService.SaveArtifact(a.ctx, input)
	if err != nil {
		return aiservice.AIArtifactResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIArtifactResponse{Artifact: &artifact}
}

func (a *App) ListAIArtifacts() aiservice.AIArtifactListResponse {
	if a.aiService == nil {
		return aiservice.AIArtifactListResponse{Items: []aiservice.AIArtifact{}, Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	items, err := a.aiService.ListArtifacts(a.ctx)
	if err != nil {
		return aiservice.AIArtifactListResponse{Items: []aiservice.AIArtifact{}, Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIArtifactListResponse{Items: items}
}

func (a *App) GetAIArtifact(id string) aiservice.AIArtifactResponse {
	if a.aiService == nil {
		return aiservice.AIArtifactResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	artifact, err := a.aiService.GetArtifact(a.ctx, id)
	if err != nil {
		return aiservice.AIArtifactResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIArtifactResponse{Artifact: &artifact}
}

func (a *App) DeleteAIArtifact(id string) aiservice.AIDeleteResponse {
	if a.aiService == nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.aiService.DeleteArtifact(a.ctx, id); err != nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIDeleteResponse{Deleted: true}
}

func (a *App) DeleteAllAIArtifacts() aiservice.AIDeleteResponse {
	if a.aiService == nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.aiService.DeleteAllArtifacts(a.ctx); err != nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIDeleteResponse{Deleted: true}
}

func (a *App) DeleteAllAIWritingArtifacts() aiservice.AIDeleteResponse {
	if a.aiService == nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	if err := a.aiService.DeleteAllWritingArtifacts(a.ctx); err != nil {
		return aiservice.AIDeleteResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return aiservice.AIDeleteResponse{Deleted: true}
}

func (a *App) StartAILibrarian(input aiservice.LibrarianInput) aiservice.LibrarianStartResponse {
	if a.aiService == nil {
		return aiservice.LibrarianStartResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	eventContext := a.ctx
	result, err := a.aiService.StartLibrarian(a.ctx, input, func(event aiservice.LibrarianEvent) {
		if eventContext != nil {
			runtime.EventsEmit(eventContext, aiservice.LibrarianEventName, event)
		}
	})
	if err != nil {
		return aiservice.LibrarianStartResponse{Error: aiservice.SafeErrorFrom(err)}
	}
	return result
}

func (a *App) CancelAILibrarian(requestID string) aiservice.LibrarianCancelResponse {
	if a.aiService == nil {
		return aiservice.LibrarianCancelResponse{Error: aiservice.SafeErrorFrom(aiservice.ErrConfigurationUnavailable)}
	}
	a.aiService.CancelLibrarian(requestID)
	return aiservice.LibrarianCancelResponse{Canceled: true}
}

func (a *App) DeleteAIProviderCredential(providerID string) ([]aiservice.ProviderSettings, error) {
	if a.aiService == nil {
		return []aiservice.ProviderSettings{}, errors.New("AI service is not initialized")
	}
	return a.aiService.DeleteProvider(a.ctx, aiservice.ProviderID(providerID))
}

func (a *App) DeleteAllAICredentials() ([]aiservice.ProviderSettings, error) {
	if a.aiService == nil {
		return []aiservice.ProviderSettings{}, errors.New("AI service is not initialized")
	}
	return a.aiService.DeleteAll(a.ctx)
}

func (a *App) PrepareSyncRecovery(action string) (syncservice.RecoveryPreview, error) {
	if a.syncService == nil {
		return syncservice.RecoveryPreview{}, errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return syncservice.RecoveryPreview{}, err
	}
	return a.syncService.PrepareRecovery(a.ctx, action)
}

func (a *App) ExecuteSyncRecovery(input syncservice.RecoveryExecutionInput) (syncservice.RecoveryResult, error) {
	if a.syncService == nil {
		return syncservice.RecoveryResult{}, errors.New("sync service is not initialized")
	}
	if err := a.ensureSyncAllowed(); err != nil {
		return syncservice.RecoveryResult{}, err
	}
	return a.syncService.ExecuteRecovery(a.ctx, input)
}

func (a *App) QuitForSyncRecovery() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

func (a *App) ListStorageSpaces() notespace.ListResult {
	if a.spaceRegistry == nil {
		err := a.startupErr
		if err == nil {
			err = notespace.ErrUnavailable
		}
		return notespace.ListResult{
			Spaces: []notespace.Space{},
			Error:  notespace.APIErrorFrom(err),
		}
	}
	result, err := a.spaceRegistry.List()
	if err != nil {
		return notespace.ListResult{
			Spaces: []notespace.Space{},
			Error:  notespace.APIErrorFrom(err),
		}
	}
	if a.activeSpace.ID != "" {
		result.ActiveSpaceID = a.activeSpace.ID
		for index := range result.Spaces {
			result.Spaces[index].Active = result.Spaces[index].ID == a.activeSpace.ID
		}
	}
	return result
}

func (a *App) CreateStorageSpace(input notespace.CreateInput) notespace.MutationResult {
	if a.spaceRegistry == nil {
		return notespace.MutationResult{Error: notespace.APIErrorFrom(notespace.ErrUnavailable)}
	}
	space, activeSpaceID, err := a.spaceRegistry.Create(a.operationContext(), input.Name, a.prepareStorageSpace)
	if err != nil {
		return notespace.MutationResult{Error: notespace.APIErrorFrom(err)}
	}
	if a.activeSpace.ID != "" {
		activeSpaceID = a.activeSpace.ID
		space.Active = space.ID == a.activeSpace.ID
	}
	return notespace.MutationResult{Space: &space, ActiveSpaceID: activeSpaceID}
}

func (a *App) SelectStorageSpace(input notespace.SelectInput) notespace.MutationResult {
	if a.spaceRegistry == nil {
		return notespace.MutationResult{Error: notespace.APIErrorFrom(notespace.ErrUnavailable)}
	}
	if input.ID == a.activeSpace.ID {
		space := a.activeSpace
		return notespace.MutationResult{Space: &space, ActiveSpaceID: space.ID}
	}
	space, restartRequired, err := a.spaceRegistry.Select(a.operationContext(), input.ID, a.prepareStorageSpace)
	if err != nil {
		return notespace.MutationResult{Error: notespace.APIErrorFrom(err)}
	}
	return notespace.MutationResult{
		Space: &space, ActiveSpaceID: space.ID,
		RestartRequired: restartRequired || space.ID != a.activeSpace.ID,
	}
}

func (a *App) ListContentLocks() contentlock.ListResult {
	if a.contentLocks == nil {
		return contentlock.ListResult{Locks: []contentlock.Lock{}, Error: contentlock.APIErrorFrom(contentlock.ErrNotFound)}
	}
	locks, err := a.contentLocks.List(a.operationContext())
	if err != nil {
		return contentlock.ListResult{Locks: []contentlock.Lock{}, Error: contentlock.APIErrorFrom(err)}
	}
	return contentlock.ListResult{Locks: locks}
}

// ListRequiredContentLocks reports the still-locked keys needed to open a
// note or notebook. The result is metadata-only and does not expose any key
// or encrypted body data.
func (a *App) ListRequiredContentLocks(target contentlock.Target) contentlock.ListResult {
	var locks []contentlock.Lock
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var listErr error
		locks, listErr = manager.ListRequiredLocks(a.operationContext(), target)
		return listErr
	})
	if err != nil {
		return contentlock.ListResult{Locks: []contentlock.Lock{}, Error: contentlock.APIErrorFrom(err)}
	}
	return contentlock.ListResult{Locks: locks}
}

// ListStorageSpaceLockStatuses inspects each internally managed space under
// its own single-writer lock. Inactive spaces are opened only for the duration
// of the metadata check and never become the active application space.
func (a *App) ListStorageSpaceLockStatuses() StorageSpaceLockStatusResult {
	result := StorageSpaceLockStatusResult{Statuses: make([]StorageSpaceLockStatus, 0)}
	if a.spaceRegistry == nil {
		result.Error = contentlock.APIErrorFrom(contentlock.ErrNotFound)
		return result
	}
	spaces, err := a.spaceRegistry.List()
	if err != nil {
		result.Error = contentlock.APIErrorFrom(err)
		return result
	}
	result.Statuses = make([]StorageSpaceLockStatus, 0, len(spaces.Spaces))
	for _, space := range spaces.Spaces {
		status := StorageSpaceLockStatus{SpaceID: space.ID}
		target := contentlock.Target{Type: contentlock.TargetSpace, ID: space.ID}
		var lockStatus contentlock.TargetStatus
		err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
			var statusErr error
			lockStatus, statusErr = manager.GetTargetStatus(a.operationContext(), target)
			return statusErr
		})
		if err != nil {
			status.Error = contentlock.APIErrorFrom(err)
		} else {
			status.Protected = lockStatus.Protected
			status.Locked = lockStatus.Locked
		}
		result.Statuses = append(result.Statuses, status)
	}
	return result
}

func (a *App) GetContentLockStatus(target contentlock.Target) (contentlock.TargetStatus, error) {
	var status contentlock.TargetStatus
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var statusErr error
		status, statusErr = manager.GetTargetStatus(a.operationContext(), target)
		return statusErr
	})
	return status, err
}

func (a *App) EnableContentLock(input contentlock.EnableInput) contentlock.MutationResult {
	target := contentlock.Target{Type: input.TargetType, ID: input.TargetID}
	var lock contentlock.Lock
	var aiRecordCount int
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var enableErr error
		lock, aiRecordCount, enableErr = manager.Enable(a.operationContext(), input)
		return enableErr
	})
	if err != nil {
		return contentlock.MutationResult{AIRecordCount: aiRecordCount, Error: contentlock.APIErrorFrom(err)}
	}
	if a.isActiveContentLockTarget(target) {
		a.refreshContentLockRecovery()
	} else {
		lock.Unlocked = false
	}
	return contentlock.MutationResult{Lock: &lock, AIRecordCount: aiRecordCount, Unlocked: lock.Unlocked}
}

func (a *App) UnlockContentLock(input contentlock.UnlockInput) contentlock.MutationResult {
	target := contentlock.Target{Type: input.TargetType, ID: input.TargetID}
	if !a.isActiveContentLockTarget(target) {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(contentlock.ErrValidation)}
	}
	var lock contentlock.Lock
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var unlockErr error
		lock, unlockErr = manager.Unlock(a.operationContext(), input)
		return unlockErr
	})
	if err != nil {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
	}
	if input.TargetType == contentlock.TargetSpace && a.startupLocked {
		if a.db == nil || a.markdownStore == nil {
			return contentlock.MutationResult{Error: contentlock.APIErrorFrom(contentlock.ErrNotFound)}
		}
		if err := a.initializeServices(a.operationContext(), a.db, a.markdownStore, config.PathsForDataDir(a.dataDir)); err != nil {
			return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
		}
	}
	a.refreshContentLockRecovery()
	return contentlock.MutationResult{Lock: &lock, Unlocked: true}
}

func (a *App) LockContentNow(target contentlock.Target) contentlock.MutationResult {
	if !a.isActiveContentLockTarget(target) {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(contentlock.ErrValidation)}
	}
	var lock contentlock.Lock
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var lockErr error
		lock, lockErr = manager.LockNow(a.operationContext(), target)
		return lockErr
	})
	if err != nil {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
	}
	if target.Type == contentlock.TargetSpace {
		a.quiesceLockedSpace()
	}
	return contentlock.MutationResult{Lock: &lock}
}

// LockContentTargetsNow is used by the fixed-time auto-lock scheduler. It
// makes all due targets unavailable in one manager operation after the
// frontend has safely flushed pending note edits.
func (a *App) LockContentTargetsNow(targets []contentlock.Target) contentlock.ListResult {
	if len(targets) == 0 {
		return contentlock.ListResult{Locks: []contentlock.Lock{}}
	}
	for _, target := range targets {
		if !a.isActiveContentLockTarget(target) {
			return contentlock.ListResult{Locks: []contentlock.Lock{}, Error: contentlock.APIErrorFrom(contentlock.ErrValidation)}
		}
	}

	var locks []contentlock.Lock
	err := a.withContentLockManager(a.operationContext(), targets[0], func(manager *contentlock.Manager) error {
		var lockErr error
		locks, lockErr = manager.LockTargetsNow(a.operationContext(), targets)
		return lockErr
	})
	if err != nil {
		return contentlock.ListResult{Locks: []contentlock.Lock{}, Error: contentlock.APIErrorFrom(err)}
	}
	for _, lock := range locks {
		if lock.TargetType == contentlock.TargetSpace {
			a.quiesceLockedSpace()
			break
		}
	}
	return contentlock.ListResult{Locks: locks}
}

func (a *App) ChangeContentLockPassphrase(input contentlock.ChangePassphraseInput) contentlock.MutationResult {
	target := contentlock.Target{Type: input.TargetType, ID: input.TargetID}
	var lock contentlock.Lock
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		var changeErr error
		lock, changeErr = manager.ChangePassphrase(a.operationContext(), input)
		return changeErr
	})
	if err != nil {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
	}
	return contentlock.MutationResult{Lock: &lock, Unlocked: true}
}

func (a *App) DisableContentLock(input contentlock.DisableInput) contentlock.MutationResult {
	target := contentlock.Target{Type: input.TargetType, ID: input.TargetID}
	err := a.withContentLockManager(a.operationContext(), target, func(manager *contentlock.Manager) error {
		return manager.Disable(a.operationContext(), input)
	})
	if err != nil {
		return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
	}
	if target.Type == contentlock.TargetSpace && a.startupLocked {
		if a.db == nil || a.markdownStore == nil {
			return contentlock.MutationResult{Error: contentlock.APIErrorFrom(contentlock.ErrNotFound)}
		}
		if err := a.initializeServices(a.operationContext(), a.db, a.markdownStore, config.PathsForDataDir(a.dataDir)); err != nil {
			return contentlock.MutationResult{Error: contentlock.APIErrorFrom(err)}
		}
	}
	if a.isActiveContentLockTarget(target) {
		a.refreshContentLockRecovery()
	}
	return contentlock.MutationResult{Removed: true}
}

func (a *App) isActiveContentLockTarget(target contentlock.Target) bool {
	return target.Type != contentlock.TargetSpace ||
		target.ID == "" ||
		target.ID == contentlock.SpaceTargetID ||
		target.ID == a.activeSpace.ID
}

func (a *App) withContentLockManager(ctx context.Context, target contentlock.Target, operation func(*contentlock.Manager) error) (returnErr error) {
	if operation == nil {
		return contentlock.ErrValidation
	}
	if a.isActiveContentLockTarget(target) {
		if a.contentLocks == nil {
			return contentlock.ErrNotFound
		}
		return operation(a.contentLocks)
	}
	if target.Type != contentlock.TargetSpace || a.spaceRegistry == nil {
		return contentlock.ErrNotFound
	}

	_, dataDir, err := a.spaceRegistry.DataDir(target.ID)
	if err != nil {
		return err
	}
	paths := config.PathsForDataDir(dataDir)
	dataLock, err := datalock.Acquire(paths.LockPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := dataLock.Release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	db, err := database.Open(ctx, paths.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()
	store, err := storage.NewMarkdownStore(paths.NotesDir)
	if err != nil {
		return err
	}
	manager := contentlock.NewManager(db, store)
	defer manager.Close()
	if err := manager.Recover(ctx); err != nil {
		return err
	}
	return operation(manager)
}

func (a *App) refreshContentLockRecovery() {
	if a.notes == nil {
		return
	}
	report, err := a.notes.Recover(a.operationContext())
	if err != nil {
		return
	}
	a.statusMu.Lock()
	a.recoveryReport = report
	a.statusMu.Unlock()
}

func (a *App) quiesceLockedSpace() {
	if a.aiService != nil {
		a.aiService.Shutdown()
	}
	a.notes = nil
	a.noteImporter = nil
	a.noteExporter = nil
	a.syncService = nil
	a.aiService = nil
	a.statusMu.Lock()
	a.recoveryReport = note.RecoveryReport{}
	a.startupLocked = true
	a.statusMu.Unlock()
}

func (a *App) operationContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) prepareStorageSpace(ctx context.Context, dataDir string) (returnErr error) {
	paths := config.PathsForDataDir(dataDir)
	dataLock, err := datalock.Acquire(paths.LockPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := dataLock.Release(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()

	db, err := database.Open(ctx, paths.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); returnErr == nil && err != nil {
			returnErr = err
		}
	}()
	store, err := storage.NewMarkdownStore(paths.NotesDir)
	if err != nil {
		return err
	}
	manager := contentlock.NewManager(db, store)
	defer manager.Close()
	return manager.Recover(ctx)
}

func (a *App) GetStartupStatus() StartupStatus {
	a.statusMu.RLock()
	defer a.statusMu.RUnlock()

	var activeStorageSpace *notespace.Space
	if a.activeSpace.ID != "" {
		active := a.activeSpace
		activeStorageSpace = &active
	}

	if a.startupErr != nil {
		return StartupStatus{
			Ready:              false,
			Message:            a.startupErr.Error(),
			DataDir:            a.dataDir,
			MissingNotes:       []MissingNoteDiagnostic{},
			ActiveStorageSpace: activeStorageSpace,
		}
	}
	if a.startupLocked {
		return StartupStatus{
			Ready:              false,
			Locked:             true,
			Message:            "この保存空間はロックされています。",
			DataDir:            a.dataDir,
			MissingNotes:       []MissingNoteDiagnostic{},
			ActiveStorageSpace: activeStorageSpace,
		}
	}

	missingNotes := make([]MissingNoteDiagnostic, 0, len(a.recoveryReport.MissingNotes))
	for _, missing := range a.recoveryReport.MissingNotes {
		missingNotes = append(missingNotes, MissingNoteDiagnostic{
			ID:       missing.ID,
			Title:    missing.Title,
			FilePath: filepath.Join(a.notesDir, missing.ContentPath),
		})
	}
	return StartupStatus{
		Ready:              true,
		Degraded:           len(missingNotes) > 0,
		DataDir:            a.dataDir,
		MissingNotes:       missingNotes,
		SyncRecoveryBackup: a.syncRecoveryBackup,
		ActiveStorageSpace: activeStorageSpace,
	}
}

func (a *App) initialize(ctx context.Context) {
	basePaths, err := config.LoadPaths()
	if err != nil {
		a.startupErr = err
		return
	}
	a.dataDir = basePaths.DataDir
	spaceRegistry, err := notespace.Open(basePaths.DataDir)
	if err != nil {
		a.startupErr = err
		return
	}
	a.spaceRegistry = spaceRegistry
	activeSpace, activeDataDir, err := spaceRegistry.Active()
	if err != nil {
		a.startupErr = err
		return
	}
	a.activeSpace = activeSpace
	a.dataDir = activeDataDir
	paths := config.PathsForDataDir(activeDataDir)
	a.notesDir = paths.NotesDir
	dataLock, err := datalock.Acquire(paths.LockPath)
	if err != nil {
		a.startupErr = err
		return
	}
	a.dataLock = dataLock
	backupPath, err := syncservice.ApplyPendingRecovery(syncservice.RecoveryPaths{
		DataDir: paths.DataDir, DatabasePath: paths.DatabasePath, NotesDir: paths.NotesDir,
	})
	if err != nil {
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}
	a.syncRecoveryBackup = backupPath

	db, err := database.Open(ctx, paths.DatabasePath)
	if err != nil {
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}

	store, err := storage.NewMarkdownStore(paths.NotesDir)
	if err != nil {
		_ = db.Close()
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}

	a.db = db
	a.markdownStore = store
	a.contentLocks = contentlock.NewManager(db, store)
	if err := a.contentLocks.Recover(ctx); err != nil {
		a.contentLocks.Close()
		a.contentLocks = nil
		a.markdownStore = nil
		_ = db.Close()
		a.db = nil
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}
	spaceStatus, err := a.contentLocks.GetTargetStatus(ctx, contentlock.Target{Type: contentlock.TargetSpace})
	if err != nil {
		a.contentLocks.Close()
		a.contentLocks = nil
		a.markdownStore = nil
		_ = db.Close()
		a.db = nil
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}
	if spaceStatus.Locked {
		a.startupLocked = true
		return
	}
	if err := a.initializeServices(ctx, db, store, paths); err != nil {
		a.contentLocks.Close()
		a.contentLocks = nil
		a.markdownStore = nil
		_ = db.Close()
		a.db = nil
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
	}
}

func (a *App) initializeServices(ctx context.Context, db *sql.DB, store *storage.MarkdownStore, paths config.Paths) error {
	noteRepository := note.NewRepository(db)
	syncRepository := syncservice.NewRepository(db)
	aiRepository := aiservice.NewRepository(db)
	noteRepository.SetSyncChangeRecorder(syncRepository)
	service := note.NewService(noteRepository, store)
	if a.contentLocks != nil {
		service.SetContentLockGuard(a.contentLocks)
	}
	credentialManager := syncservice.NewCredentialManager(syncservice.NewKeyringCredentialStore(syncservice.ServiceName))
	syncService := syncservice.NewService(syncRepository, service, credentialManager)
	aiCredentialManager := credential.NewManager(credential.NewKeyringStore(aiservice.CredentialStoreServiceName))
	aiService := aiservice.NewServiceWithAdapter(aiRepository, aiCredentialManager, aiservice.NewHTTPProviderAdapter())
	if a.contentLocks != nil {
		aiService.SetContentAccessGuard(a.contentLocks)
		aiService.SetNoteContextProvider(aiservice.NewNoteContextProviderWithAccessGuard(service, a.contentLocks))
	} else {
		aiService.SetNoteContextProvider(aiservice.NewNoteContextProvider(service))
	}
	syncService.SetRecoveryDataDir(paths.DataDir)
	recoveryReport, err := service.Recover(ctx)
	if err != nil {
		return err
	}
	a.notes = service
	a.noteImporter = noteimport.NewService(service)
	a.noteExporter = noteexport.NewService(service, a.contentLocks)
	a.syncService = syncService
	a.aiService = aiService
	a.recoveryReport = recoveryReport
	a.startupLocked = false
	return nil
}

func (a *App) ToggleAlwaysOnTop(b bool) {
	if a.ctx != nil {
		runtime.WindowSetAlwaysOnTop(a.ctx, b)
	}
}
