package main

import (
	"atlasnote/internal/database"
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestFinishApplicationReleasesBeforeImmediateChild(t *testing.T) {
	for _, scenario := range []string{"restart", "normal", "run-failure", "shutdown-failure", "release-failure", "child-failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlasnote.db"))
			if err != nil {
				t.Fatal(err)
			}
			app := &App{db: db}
			released := false
			started := false
			sentinel := errors.New("fixture failure")
			if scenario != "normal" {
				app.restartExecutable = "fixture.exe"
			}
			if scenario == "shutdown-failure" {
				app.shutdownErr = sentinel
			}
			app.startProcess = func(string) error {
				started = true
				if !released {
					t.Fatal("child attempted lock acquisition before release")
				}
				if err := db.Ping(); err == nil {
					t.Fatal("DB is still open")
				}
				if scenario == "child-failure" {
					return sentinel
				}
				return nil
			}
			var runErr error
			if scenario == "run-failure" {
				runErr = sentinel
			}
			err = finishApplication(app, runErr, func() error {
				if app.db != nil || app.dataLock != nil {
					t.Fatal("resources still held at release")
				}
				released = true
				if scenario == "release-failure" {
					return sentinel
				}
				return nil
			})
			if !released {
				t.Fatal("lock leaked")
			}
			wantStart := scenario == "restart" || scenario == "child-failure"
			if started != wantStart {
				t.Fatalf("started=%v", started)
			}
			wantError := scenario != "restart" && scenario != "normal"
			if (err != nil) != wantError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
