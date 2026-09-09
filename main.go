package main

import (
	"context"
	"embed"
	"errors"
	"os"

	"atlasnote/internal/appcleanup"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if handled, exitCode := runMaintenanceCommand(os.Args[1:]); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}
	applicationLock, err := appcleanup.AcquireApplicationLock()
	if err != nil {
		reportMaintenanceFailure("Atlas Noteの保守処理中、または起動用の排他を取得できないため起動できません。")
		return
	}
	defer applicationLock.Release()
	app := NewApp()
	app.recordApplicationUser = appcleanup.RecordApplicationUser

	err = wails.Run(&options.App{
		Title:            "Atlas Note",
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 255},
		OnStartup:        app.startup,
		// OnBeforeCloseをフックすることで、ユーザーが「×」ボタンでウィンドウを閉じようとした際に、
		// 未保存の入力データをDBやファイルに保存し終わるまでアプリの終了を待機させる。
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		// フロントエンド（JS/TS）からGoのメソッドを呼び出せるようにバインディングを登録する。
		Bind: []interface{}{
			app,
		},
	})
	if err := finishApplication(app, err, applicationLock.Release); err != nil {
		reportMaintenanceFailure("Atlas Noteの終了または自動再起動に失敗しました。手動で起動し直してください。")
	}
}

// Also closes resources if Wails fails before its OnShutdown callback.
func finishApplication(app *App, runErr error, release func() error) error {
	app.shutdown(context.Background())
	err := errors.Join(runErr, app.shutdownErr, release())
	if err != nil {
		return err
	}
	return app.launchRestartIfRequested()
}
