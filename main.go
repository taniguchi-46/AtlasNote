package main

import (
	"embed"
	"encoding/json"
	"os"
	"strings"

	backendapp "atlasnote/internal/app"
	"atlasnote/internal/appcleanup"
	"atlasnote/internal/externalcmd"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed docs/user-readme.md
var userReadme []byte

//go:embed wails.json
var wailsConfigBytes []byte

type wailsConfig struct {
	Info struct {
		ProductVersion string `json:"productVersion"`
	} `json:"info"`
}

func applicationProductVersion() string {
	var config wailsConfig
	if err := json.Unmarshal(wailsConfigBytes, &config); err != nil || strings.TrimSpace(config.Info.ProductVersion) == "" {
		return "unknown"
	}
	return config.Info.ProductVersion
}

func main() {
	if handled, exitCode := externalcmd.Run(os.Args[1:], os.Stdout, os.Stderr); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}
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
	app := backendapp.NewWithUserGuide(applicationProductVersion(), string(userReadme))
	app.SetRecordApplicationUser(appcleanup.RecordApplicationUser)

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
		OnStartup:        app.Startup,
		// OnBeforeCloseをフックすることで、ユーザーが「×」ボタンでウィンドウを閉じようとした際に、
		// 未保存の入力データをDBやファイルに保存し終わるまでアプリの終了を待機させる。
		OnBeforeClose: app.BeforeClose,
		OnShutdown:    app.Shutdown,
		// フロントエンド（JS/TS）からGoのメソッドを呼び出せるようにバインディングを登録する。
		Bind: []interface{}{
			app,
		},
	})
	if err := backendapp.FinishApplication(app, err, applicationLock.Release); err != nil {
		reportMaintenanceFailure("Atlas Noteの終了または自動再起動に失敗しました。手動で起動し直してください。")
	}
}
