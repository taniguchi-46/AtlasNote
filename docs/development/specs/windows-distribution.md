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

Wails標準NSISインストーラを使用し、Program Files、Windowsのアンインストール登録を生成する。インストール先の選択後に、デスクトップとスタートメニューのショートカットを個別に選択でき、初期状態はどちらもONである。チェックを外したショートカットは新規作成せず、更新時に既存のショートカットを削除しない。インストーラはWebView2 Runtimeが必要な場合に標準の導入処理を行う。

完了ページにはAtlas Noteの起動チェックボックスを表示するが、初期状態はOFFである。ONにした場合は、昇格されたインストーラから既存のExplorerシェルへ委譲して通常のデスクトップユーザーとして起動する。UAC、既定シェル、セキュリティ製品による起動制限は実機環境に依存するため、リリース前に確認する。

アンインストールでは、まずアプリ本体の削除可否を確認し、ショートカット、関連付け、実行中のuninstallerのバックアップ、uninstall.exe、64-bit viewのアンインストール登録を順に処理する。途中で失敗した場合は、元の名前のuninstall.exeと登録情報を可能な範囲で復元し、復元不能なら再インストールが必要であることを表示する。`%AppData%\AtlasNote` の設定ファイル、Documents配下のノート、バックアップ、保存空間、復旧workspace、ユーザーが追加したインストール先の内容は削除しない。

通常の対話アンインストールには追加削除の選択ページを表示し、「端末の表示設定・キャッシュを削除」「この利用者のAtlas Note用認証情報を削除」は既定OFFとする。選択時も、Wailsの既知のWebView表示設定・キャッシュ領域と、現在ユーザーのAtlas Note名前空間に紐づくCredential Store参照だけを対象にする。別プロセスの通常起動・DB migrationを呼び出さない保守コマンドで、現在ユーザーのSID／プロファイル／AppData、アプリと保存空間のlock、リンク／junction／reparse point、保留中の移行・復旧を検証する。確認できない場合、アクセス拒否、使用中、残存項目がある場合は追加削除と通常アンインストールを中止し、終了後の再試行を案内する。silent uninstallと更新経路では追加削除を実行しない。

アンインストールの回帰ハーネスは、実際のDesktop／Program Files／HKLMを使わず、一時フォルダと専用HKCUキーだけで、アプリまたはuninstallerのロック、ユーザーファイル保持、カスタムインストール先、登録削除拒否、uninstaller復元失敗、再インストール復旧を確認する。追加削除の選択は、既定OFF、対象ユーザー不一致、保留中の移行・復旧、表示設定のリンク拒否、Credential Storeの参照限定、ノート・バックアップ・保存場所設定の保持をfixtureとmockで確認する。

```powershell
powershell -NoProfile -File .\build\windows\installer\tests\test-uninstall.ps1
```

追加削除の選択配線は、NSISコンパイル、共通マクロからfixture EXEへの実引数・終了コード・起動失敗の伝播、GoのSID記録→読み込み→保守リクエスト→削除／保持の回帰を次で実行する。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build\windows\installer\tests\test-uninstall-options.ps1
```

ショートカット選択・完了起動のオプションハーネスは、実際のDesktop／スタートメニュー／Program Filesを使わず、一時フォルダのfixtureへ同じNSISオプションマクロを適用する。4通りのチェック組み合わせ、同じ設定での更新、サイレント時の既定値、カスタムインストール先、UIキャンセル、完了起動コールバックを確認する。

```powershell
powershell -NoProfile -File .\build\windows\installer\tests\test-installer-options.ps1
```

このハーネスはインストール済みWindowsアプリの実環境や「インストールされているアプリ」画面経由の操作を代替しない。リリース前には実機での確認も行う。レジストリ削除拒否のテストでは、アクセス制御変更に必要な最小権限だけを要求し、削除権限そのものは要求しない。

## リリース前確認

- `Get-AuthenticodeSignature` が署名済みを示すこと（証明書の導入後）。
- installerとEXEのSHA-256を保存すること。
- Go／Node.js／Wails未導入のクリーンWindows環境で、インストール、初回起動、再起動、更新、アンインストール、再インストールを確認すること。
- OneDriveのDocumentsリダイレクト、Files On-Demand、Controlled Folder Access、日本語ユーザー名を確認すること。
- アンインストール後も利用者データが残ることを確認すること。

インストーラはWindowsアプリとしての認識を改善するが、保存場所の検証エラー自体はアプリ側の起動回復処理で解決する。

### 追加削除の安全境界

- インストール実行者のユーザー名は削除対象の根拠にしない。通常権限でのアプリ起動時だけ、OSトークンとWindowsセッションのSID一致を検証し、HKCUの`Software\AtlasNote\Installations\<実行ファイルパスのSHA-256>`へ`UserSID`を記録する。この記録は秘密情報を含まず、追加削除でも保持する。
- NSISは`--atlasnote-maintenance --registered-user`と選択した削除フラグだけを渡す。保守側はWindowsセッションから確定したSID、実行トークンのSID、同じインストール先の利用記録を照合する。別管理者を指定したUAC、同名別SID、記録欠落、セッション判定不能は削除前に拒否する。Explorer経由の起動自体は本人確認の根拠としない。
- 旧インストールや記録欠落時は、本人のセッションで通常権限のアプリを一度起動して終了し、再試行する。別管理者のUACが必要な利用者は、アンインストール前に本人の通常権限のPowerShellから `& '<インストール先>\AtlasNote.exe' --atlasnote-maintenance --registered-user --delete-display-settings --delete-credentials` を実行する（必要な削除フラグだけ指定）。その後、追加削除を選択せずアンインストールする。
- WebViewルート全体を再帰削除しない。`%AppData%\AtlasNote.exe\EBWebView\Default`の`Local Storage\leveldb`、`Cache\Cache_Data`、`Code Cache\js`／`wasm`、`GPUCache`の既知ファイルだけを削除する。それ以外のWebView項目や未知の旧配置は保持する。対象領域内の未知ファイル、ハードリンク、読取専用、使用中、祖先を含むreparse pointは事前拒否する。
- 保存先／バックアップ先／旧標準配置／保存場所管理ディレクトリとの同一・包含関係を、存在する祖先の正規パスを含めて検証する。上書き環境変数や既定保存先の判定不能時は削除を拒否する。検証済みWindowsハンドルを削除完了まで保持し、パスを再解決せず同じ対象を削除する。後から追加された未知ファイルは削除せず、ディレクトリが空にならない場合は失敗を報告する。
- 通常アプリ同士は共有の活動マーカーを保持でき、別保存空間の同時利用を妨げない。保守削除だけが利用者SID単位で排他になる。再起動時はサービス終了、DB close、writer lock解放、活動マーカー解放の後に子を開始する。終了／解放失敗時は自動再起動を行わない。
- 実機の新規インストール→通常利用→追加削除、別管理者UAC、WebView2のバージョン別ファイル配置は手動受け入れ未確認。未知の配置は保持するため、すべてのWebView生成物の削除は保証しない。
全Go回帰では、旧配置の探索が実プロファイルへ到達しないよう、プロセスのAPPDATAを一時fixtureへ隔離する（このPowerShellプロセス内だけに適用）。

```powershell
$env:GOCACHE = Join-Path $env:TEMP 'atlasnote-review-gocache'
$fixtureProfile = Join-Path $env:TEMP ('AtlasNote-go-profile-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixtureProfile | Out-Null
$env:APPDATA = $fixtureProfile
go test ./... -count=1
```

SIDのセッション情報取得は[WTSQuerySessionInformationWの公式仕様](https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/nf-wtsapi32-wtsquerysessioninformationw)、削除ハンドルと共有制約は[CreateFileの公式仕様](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew)を参照。2026-09-09の検証では、既存installer-optionsハーネスの実行がWindowsセキュリティ検出で遮断されたため、コンパイル以降の受け入れは未完了である。