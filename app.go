package main

import (
	"context"
	"database/sql"
	"errors"

	"atlasnote/internal/config"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	db         *sql.DB
	notes      *note.Service
	dataDir    string
	startupErr error
}

type StartupStatus struct {
	Ready   bool   `json:"ready"`
	Message string `json:"message,omitempty"`
	DataDir string `json:"dataDir,omitempty"`
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
	}
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
	if a.startupErr != nil {
		return StartupStatus{
			Ready:   false,
			Message: a.startupErr.Error(),
			DataDir: a.dataDir,
		}
	}

	return StartupStatus{
		Ready:   true,
		DataDir: a.dataDir,
	}
}

func (a *App) initialize(ctx context.Context) {
	paths, err := config.LoadPaths()
	if err != nil {
		a.startupErr = err
		return
	}
	a.dataDir = paths.DataDir

	db, err := database.Open(ctx, paths.DatabasePath)
	if err != nil {
		a.startupErr = err
		return
	}

	store, err := storage.NewMarkdownStore(paths.NotesDir)
	if err != nil {
		_ = db.Close()
		a.startupErr = err
		return
	}

	service := note.NewService(note.NewRepository(db), store)
	if err := service.Recover(ctx); err != nil {
		_ = db.Close()
		a.startupErr = err
		return
	}

	a.db = db
	a.notes = service
}

func (a *App) ToggleAlwaysOnTop(b bool) {
	if a.ctx != nil {
		runtime.WindowSetAlwaysOnTop(a.ctx, b)
	}
}
