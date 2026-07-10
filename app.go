package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"

	"atlasnote/internal/config"
	"atlasnote/internal/database"
	"atlasnote/internal/datalock"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx            context.Context
	db             *sql.DB
	dataLock       *datalock.Lock
	notes          *note.Service
	dataDir        string
	startupErr     error
	recoveryReport note.RecoveryReport
	statusMu       sync.RWMutex
	closeMu        sync.Mutex
	closeRequested bool
	allowClose     bool
}

type StartupStatus struct {
	Ready        bool                    `json:"ready"`
	Degraded     bool                    `json:"degraded"`
	Message      string                  `json:"message,omitempty"`
	DataDir      string                  `json:"dataDir,omitempty"`
	MissingNotes []MissingNoteDiagnostic `json:"missingNotes"`
}

type MissingNoteDiagnostic struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FilePath string `json:"filePath"`
}

func NewApp() *App {
	app := &App{}
	app.initialize(context.Background())
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	if a.dataLock != nil {
		_ = a.dataLock.Release()
		a.dataLock = nil
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.closeMu.Lock()
	if a.allowClose {
		a.closeMu.Unlock()
		return false
	}
	if a.closeRequested {
		a.closeMu.Unlock()
		return true
	}
	a.closeRequested = true
	a.closeMu.Unlock()

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

func (a *App) ListNotes() ([]note.Summary, error) {
	if a.notes == nil {
		return nil, errors.New("note service is not initialized")
	}
	return a.notes.List(a.ctx)
}

func (a *App) GetNote(id string) (note.Note, error) {
	if a.notes == nil {
		return note.Note{}, errors.New("note service is not initialized")
	}
	return a.notes.Get(a.ctx, id)
}

func (a *App) UpdateNote(id string, input note.UpdateInput) (note.Note, error) {
	if a.notes == nil {
		return note.Note{}, errors.New("note service is not initialized")
	}
	return a.notes.Update(a.ctx, id, input)
}

func (a *App) DeleteNote(id string) error {
	if a.notes == nil {
		return errors.New("note service is not initialized")
	}
	return a.notes.Delete(a.ctx, id)
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

func (a *App) GetStartupStatus() StartupStatus {
	a.statusMu.RLock()
	defer a.statusMu.RUnlock()

	if a.startupErr != nil {
		return StartupStatus{
			Ready:        false,
			Message:      a.startupErr.Error(),
			DataDir:      a.dataDir,
			MissingNotes: []MissingNoteDiagnostic{},
		}
	}

	missingNotes := make([]MissingNoteDiagnostic, 0, len(a.recoveryReport.MissingNotes))
	for _, missing := range a.recoveryReport.MissingNotes {
		missingNotes = append(missingNotes, MissingNoteDiagnostic{
			ID:       missing.ID,
			Title:    missing.Title,
			FilePath: filepath.Join(a.dataDir, "notes", missing.ContentPath),
		})
	}
	return StartupStatus{
		Ready:        true,
		Degraded:     len(missingNotes) > 0,
		DataDir:      a.dataDir,
		MissingNotes: missingNotes,
	}
}

func (a *App) initialize(ctx context.Context) {
	paths, err := config.LoadPaths()
	if err != nil {
		a.startupErr = err
		return
	}
	a.dataDir = paths.DataDir
	dataLock, err := datalock.Acquire(paths.LockPath)
	if err != nil {
		a.startupErr = err
		return
	}
	a.dataLock = dataLock

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

	service := note.NewService(note.NewRepository(db), store)
	recoveryReport, err := service.Recover(ctx)
	if err != nil {
		_ = db.Close()
		_ = a.dataLock.Release()
		a.dataLock = nil
		a.startupErr = err
		return
	}

	a.db = db
	a.notes = service
	a.recoveryReport = recoveryReport
}

func (a *App) ToggleAlwaysOnTop(b bool) {
	if a.ctx != nil {
		runtime.WindowSetAlwaysOnTop(a.ctx, b)
	}
}
