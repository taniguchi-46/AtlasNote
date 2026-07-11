package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Atlas Note",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 1},
		OnStartup:        app.startup,
		// OnBeforeCloseをフックすることで、ユーザーが「×」ボタンでウィンドウを閉じようとした際に、
		// 未保存の入力データをDBやファイルに保存し終わるまでアプリの終了を待機させる。
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		// フロントエンド（JS/TS）からGoのメソッドを呼び出せるようにバインディングを登録する。
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
