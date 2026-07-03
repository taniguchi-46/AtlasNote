package main

import (
	"context"
	"database/sql"

	"atlasnote/internal/config"
	"atlasnote/internal/database"
	"atlasnote/internal/note"
	"atlasnote/internal/storage"
)

type App struct {
	ctx        context.Context
	db         *sql.DB
	notes      *note.Service
	dataDir    string
	startupErr error
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

func (a *App) Greet(name string) string {
	return "Hello " + name + "!"
}

func (a *App) initialize(ctx context.Context) {
	paths, err := config.LoadPaths()
	if err != nil {
		a.startupErr = err
		return
	}

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

	a.db = db
	a.notes = note.NewService(note.NewRepository(db), store)
	a.dataDir = paths.DataDir
}
