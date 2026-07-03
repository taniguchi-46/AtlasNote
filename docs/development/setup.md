# 開発環境セットアップ

Atlas Note の開発環境セットアップと起動方法をまとめる。

## 前提

現時点の開発環境は、Wails / Go / Vue 3 / TypeScript / Vite を前提にする。

確認済みのバージョン:

| 項目 | バージョン |
| --- | --- |
| Node.js | `v20.19.5` |
| npm | `10.8.2` |
| Go | `go1.26.4 windows/amd64` |
| Wails CLI | `v2.10.1` |

このリポジトリでは、管理者権限に依存しないために Go と Wails CLI を `.tools/` 配下へ配置している。`.tools/` は Git 管理対象外。

## PATH 設定

新しく開いた PowerShell で `go` または `wails` が見つからない場合は、次を実行する。

```powershell
$env:Path = "C:\Users\mt252\Desktop\MerianProjects\AtlasNote\.tools\go-bin;C:\Users\mt252\Desktop\MerianProjects\AtlasNote\.tools\go\bin;$env:Path"
```

確認:

```powershell
go version
wails version
```

期待する結果:

```text
go version go1.26.4 windows/amd64
v2.10.1
```

## 依存関係のインストール

```powershell
npm --prefix frontend install
go mod tidy
```

## 環境変数

ローカル設定が必要な場合は `.env.example` を `.env` にコピーし、実値は `.env` にだけ記載する。

```powershell
Copy-Item .env.example .env
```

`.env` には API キー、パスワード、トークンなどの秘密情報を入れる可能性があるため、Git 管理しない。

## 開発サーバーの起動

```powershell
wails dev
```

ブラウザで確認する場合:

```text
http://localhost:34115
```

`wails dev` は監視プロセスとして動き続ける。終了する場合はターミナルで `Ctrl + C` を押す。

## ビルド

```powershell
wails build
```

生成物:

```text
build/bin/AtlasNote.exe
```

## Codex sandbox での確認

Codex の通常 sandbox 実行では、Node.js と Go がユーザープロファイル配下へアクセスするため、ローカル PowerShell で成功するコマンドでも権限エラーになる場合がある。

確認済みの挙動:

| 実行方法 | コマンド | 結果 |
| --- | --- | --- |
| Codex sandbox 通常実行 | `npm run frontend:build` | `C:\Users\mt252` の `lstat` で `EPERM` |
| Codex 権限付き実行 | `npm run frontend:build` | 成功 |
| Codex sandbox 通常実行 | `.\.tools\go-bin\wails.exe build` | Go build cache `C:\Users\mt252\AppData\Local\go-build` へのアクセスで失敗 |
| Codex 権限付き実行 | `.\.tools\go-bin\wails.exe build` | 成功 |

Codex でビルド確認を行う場合は、通常 sandbox ではなく権限付き実行で確認する。`wails` が PATH で見つからない場合は、次のようにリポジトリ内の Wails CLI を直接指定する。

```powershell
.\.tools\go-bin\wails.exe build
```

このエラーはアプリ本体のビルドエラーではなく、Codex sandbox の権限境界によるものとして扱う。

## 確認コマンド

```powershell
wails doctor
go test ./...
wails build
npm --prefix frontend audit --audit-level=moderate
```

`npm run frontend:build` はフロントエンド単体の確認に使える。ただし `frontend/wailsjs/` は Git 管理対象外で、Wails API の TypeScript bindings が未生成のクリーン環境では失敗する。クリーン checkout 直後は先に `wails build` を実行して bindings を生成してから使う。

確認済み:

- `wails doctor`: 成功
- `go test ./...`: 成功
- `wails build`: 通常 PowerShell / Codex 権限付き実行で成功。Codex sandbox 通常実行では Go build cache へのアクセスで失敗。
- `npm run frontend:build`: Wails bindings 生成後の通常 PowerShell / Codex 権限付き実行で成功。Codex sandbox 通常実行では `EPERM`。
- `npm --prefix frontend audit --audit-level=moderate`: 脆弱性 0 件
- `wails dev`: 成功

## 注意点

- `.tools/`、`build/`、`frontend/dist/`、`frontend/node_modules/` は Git 管理対象外。
- `frontend/wailsjs/` は Git 管理対象外。クリーン環境でフロントエンド単体ビルドを行う前に、`wails build` で Wails API bindings を生成する。
- 既に開いている PowerShell には、ユーザー PATH の変更が自動反映されない場合がある。
- `go --version` ではなく `go version` を使う。
- Codex sandbox 通常実行での `EPERM` は、権限付き実行で再確認する。
