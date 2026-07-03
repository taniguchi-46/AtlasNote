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

## 確認コマンド

```powershell
wails doctor
go test ./...
npm run frontend:build
npm --prefix frontend audit --audit-level=moderate
```

確認済み:

- `wails doctor`: 成功
- `go test ./...`: 成功
- `npm run frontend:build`: 成功
- `npm --prefix frontend audit --audit-level=moderate`: 脆弱性 0 件
- `wails dev`: 成功
- `wails build`: 成功

## 注意点

- `.tools/`、`build/`、`frontend/dist/`、`frontend/node_modules/` は Git 管理対象外。
- 既に開いている PowerShell には、ユーザー PATH の変更が自動反映されない場合がある。
- `go --version` ではなく `go version` を使う。
