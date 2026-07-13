# 開発環境方針

Atlas Note の開発環境方針をまとめる。

アプリ本体の `package.json`、`go.mod`、Wails設定ファイルは配置済みです。具体的なセットアップ手順と確認済みバージョンは `docs/development/setup.md` を参照してください。

## 基本方針

- Wails / Go / TypeScript / Vue 3 / Vite の構成を前提にする。
- メインの開発環境はローカル環境とする。
- Docker は開発体験の主軸ではなく、検証・CI・ビルド補助に使う。
- Wasm は初期段階では採用しない。必要な処理が明確になった時点で再検討する。
- バージョン差異を減らすため、Go、Node.js、Wails CLI、パッケージマネージャーのバージョンを明示する。
- `.env`、API キー、トークン、パスワードなどの秘密情報はリポジトリに含めない。

## 想定スタック

| 項目 | 方針 |
| --- | --- |
| Desktop | Wails |
| Backend | Go |
| Frontend | Vue 3 + TypeScript |
| Build | Vite |
| Styling | UnoCSS |
| UI | Reka UI |
| State | Composables + Pinia |
| Database | SQLite |
| Editor | Markdown textarea + Tiptap |
| Storage | Markdown |
| Data Access | Repository + Squirrel |

## ローカル開発環境

ローカル環境では、Wails アプリをそのまま起動して開発する。

主な理由は、Wails が OS の WebView、ファイルシステム、将来的な Keychain 連携などのネイティブ機能に依存するため。Docker 内だけで GUI アプリの起動確認まで完結させると、環境構築が複雑になりやすい。

環境更新時に確認する項目:

- Go のバージョン
- Node.js のバージョン
- npm / pnpm / yarn のどれを使うか
- Wails CLI のバージョン
- SQLite 関連ライブラリ
- Lint / Format / Typecheck / Test のコマンド

2026-07-13 時点の確認結果:

| 項目 | 状態 |
| --- | --- |
| Node.js | `v20.19.5` |
| npm | `10.8.2` |
| Go | `go1.26.4 windows/amd64` |
| Wails CLI | `v2.10.1` |
| Frontend build | `npm run frontend:build` 成功 |
| npm audit | 脆弱性 0 件 |
| Go test | `go test ./...` 成功 |
| Wails doctor | 成功 |
| Wails dev | 成功、`http://localhost:34115` で起動 |
| Wails build | 成功、`build/bin/AtlasNote.exe` を生成 |

想定確認コマンド:

```bash
go version
node -v
npm -v
wails version
```

現在の主要コマンド:

```bash
wails dev
wails build
npm run frontend:build
npm run frontend:typecheck
npm run frontend:lint
go test ./...
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:serializer
npm --prefix frontend run test:notifications
npm --prefix frontend run test:operation-logger
```

Frontendの`lint`は`vue-tsc --noEmit`を実行する。専用formatter scriptは追加せず、Go変更時は`gofmt`、Markdown変更時は`git diff --check`を確認する。

## Docker の扱い

Docker は採用候補とする。ただし、初期段階では Wails GUI 開発の主環境にはしない。

主な用途:

- CI と同じ条件でのビルド確認
- Go / Node.js の依存関係確認
- `npm run build` や `go test ./...` の再現性確認
- 将来的なリリースビルド補助

初期段階で避けること:

- Docker 内で Wails GUI 開発を完結させること
- OS 固有の WebView / Keychain / ファイルアクセスまで Docker 前提で設計すること
- まだ未確定の構成に対して複雑な Dockerfile を先に作ること

必要になった場合の最小構成案:

- `Dockerfile.dev`
  - Go と Node.js の検証用
  - フロントエンドビルドと Go テストを実行できる構成
- `docker-compose.yml`
  - 複数サービスが必要になった場合のみ追加
  - SQLite 単体利用の間は必須ではない

## Wasm の扱い

Wasm は初期段階では採用しない。

現時点の構成では、Go 側のアプリケーションサービスと Vue 側の UI で主要機能を実装できる見込みがある。Wasm を早期に導入すると、ビルド、デバッグ、型連携、配布の複雑さが増える。

再検討する条件:

- Markdown 解析、検索、差分計算などで重い処理が発生した場合
- 外部ライブラリを安全に隔離して実行したい場合
- 将来的にブラウザ版と処理を共有したい場合
- Go / TypeScript の通常実装では性能要件を満たせない場合

再検討時の確認項目:

- Wails との連携方法
- ビルド成果物の配置方法
- TypeScript からの呼び出し方法
- テスト方法
- パフォーマンス測定方法
- 配布サイズへの影響

## バージョン管理方針

開発環境の差異を減らすため、以下を明示的に管理する。

- Go バージョン
- Node.js バージョン
- Wails CLI バージョン
- パッケージマネージャー
- 主要依存パッケージ
- 確認コマンド

管理方法:

- README または `docs/development/environment.md` に明記する
- `.node-version` または `.nvmrc` を置く
- Go は `go.mod` の `go` ディレクティブで管理する
- パッケージマネージャーは `package.json` の `packageManager` で固定する
- Docker は検証用の固定環境として使う

## 秘密情報の扱い

以下はリポジトリに含めない。

- `.env`
- API キー
- パスワード
- トークン
- WebDAV 認証情報
- AI API 認証情報

キー名だけを記載した `.env.example` を使用する。

例:

```bash
AI_PROVIDER=
AI_API_KEY=
AI_MODEL=
WEBDAV_ENDPOINT=
WEBDAV_USERNAME=
WEBDAV_PASSWORD=
ATLAS_NOTE_DATA_DIR=
```

実値はローカル環境または OS の安全な保存先で管理する。AI API Key の保存方法は、将来的に OS Keychain の利用可否を確認して決める。

## 今後決めること

- Dockerfile を作るタイミング
- Wasm を再検討する条件の詳細
- デスクトップアプリの配布対象OSとビルド手順
