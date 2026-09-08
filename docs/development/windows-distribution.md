# Windows配布

## 正式な成果物

Atlas Noteはリポジトリ全体をzipで配布せず、Wailsが生成するx64インストーラを配布する。

```powershell
npm ci --prefix frontend
wails build -clean -platform windows/amd64 -nsis
```

`-nsis`にはNSISの`makensis`が必要で、未導入の環境ではWailsがEXEだけを生成してインストーラを作成せず警告を表示する。この検証環境ではNSIS 3.12の`makensis`によるプロジェクトコンパイルまで確認済みで、リリース環境ではinstallerの存在を確認する。

生成物は通常 `build/bin/AtlasNote-amd64-installer.exe` となる。配布時は、バージョンを含む名前へコピーしたインストーラとSHA-256チェックサムだけを公開する。

```powershell
Get-FileHash .\build\bin\AtlasNote-amd64-installer.exe -Algorithm SHA256
```

ソース、`.git`、`node_modules`、開発用の実行ファイル、`.env` を含む作業フォルダを配布物にしない。

## インストールとアンインストール

Wails標準NSISインストーラを使用し、Program Files、Start Menu、デスクトップショートカット、Windowsのアンインストール登録を生成する。インストーラはWebView2 Runtimeが必要な場合に標準の導入処理を行う。

アンインストールでは、まずアプリ本体の削除可否を確認し、ショートカット、関連付け、実行中のuninstallerのバックアップ、uninstall.exe、64-bit viewのアンインストール登録を順に処理する。途中で失敗した場合は、元の名前のuninstall.exeと登録情報を可能な範囲で復元し、復元不能なら再インストールが必要であることを表示する。`%AppData%\AtlasNote` の設定ファイル、Documents配下のノート、バックアップ、ユーザーが追加したインストール先の内容は削除しない。データ削除は別途明示確認付きの機能として設計する。

アンインストールの回帰ハーネスは、実際のDesktop／Program Files／HKLMを使わず、一時フォルダと専用HKCUキーだけで、アプリまたはuninstallerのロック、ユーザーファイル保持、カスタムインストール先、登録削除拒否、uninstaller復元失敗、再インストール復旧を確認する。

```powershell
powershell -NoProfile -File .\build\windows\installer\tests\test-uninstall.ps1
```

このハーネスはインストール済みWindowsアプリの実環境や「インストールされているアプリ」画面経由の操作を代替しない。リリース前には実機での確認も行う。レジストリ削除拒否のテストでは、アクセス制御変更に必要な最小権限だけを要求し、削除権限そのものは要求しない。

## リリース前確認

- `Get-AuthenticodeSignature` が署名済みを示すこと（証明書の導入後）。
- installerとEXEのSHA-256を保存すること。
- Go／Node.js／Wails未導入のクリーンWindows環境で、インストール、初回起動、再起動、更新、アンインストール、再インストールを確認すること。
- OneDriveのDocumentsリダイレクト、Files On-Demand、Controlled Folder Access、日本語ユーザー名を確認すること。
- アンインストール後も利用者データが残ることを確認すること。

インストーラはWindowsアプリとしての認識を改善するが、保存場所の検証エラー自体はアプリ側の起動回復処理で解決する。
