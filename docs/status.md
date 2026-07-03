# プロジェクト状況

最終更新: 2026-07-02

## 概要

| 項目 | 内容 |
| --- | --- |
| プロジェクト名 | `Atlas Note` |
| 種別 | ローカルファーストのデスクトップ知識管理 / Second Brain アプリ |
| フレームワーク | Wails + Vue 3 + Vite |
| 言語 | Go + TypeScript |
| スタイル | UnoCSS + Reka UI |
| 実行環境 | Wails デスクトップアプリ、開発時は Go / Node.js / Vite |
| 配信 / デプロイ | デスクトップアプリとして配布予定。具体的な配布方式は未確定 |

## 主要コマンド

現時点ではアプリ本体の `package.json`、`go.mod`、Wails 設定ファイルは未配置です。実装開始後に実際の構成に合わせて更新してください。

想定コマンド:

```bash
wails dev
wails build
npm install
npm run build
go test ./...
```

## 完了済み

- `README.md` にプロジェクトコンセプト、AI Documents、技術スタック、概略アーキテクチャを記載。
- AI Agent 向けの共通ドキュメントを `docs/` に配置。
- Codex 固有の作業指針を `AGENTS.md` と `.codex/AGENTS.md` に配置。
- 汎用開発 skill を `.agents/skills/skill.md` に配置。

## 次にやること

- Wails プロジェクト本体を作成し、Go / TypeScript / Vue 3 / Vite の実ファイル構成を確定する。
- SQLite、Markdown Storage、Repository + Squirrel の責務境界をコードに落とし込む。
- Tiptap / CodeMirror を使う編集体験の最小構成を設計する。
- WebDAV 同期と AI API Key 管理の仕様を具体化する。
- 実際の確認コマンドを `docs/conventions.md` とこのファイルに反映する。

## 保留事項

- デスクトップアプリの対応 OS と配布方式。
- Markdown ファイル、SQLite メタデータ、添付ファイルの保存ディレクトリ構成。
- WebDAV 同期時の競合解決方針。
- ユーザー API Key の保存方式と暗号化方針。
- AI 機能の対象プロバイダ、モデル選択、課金表示の扱い。

## 関連ファイル

| ファイル | 役割 |
| --- | --- |
| `README.md` | プロジェクト概要、技術スタック、概略アーキテクチャ |
| `docs/ai.md` | AI Agent 共通ルール |
| `docs/architecture.md` | 設計情報 |
| `docs/conventions.md` | 実装規約 |
| `docs/loop.md` | AI 作業フロー |
| `docs/glossary.md` | 用語集 |
| `docs/BRANCHING.md` | ブランチ、コミット、PR の運用ルール |
| `docs/development/tech-stack.md` | 採用予定技術と未確定の開発環境情報 |
| `docs/development/setup.md` | 開発環境セットアップと起動方法 |
| `.env.example` | ローカル環境変数のサンプル。実値は含めない |
| `.agents/skills/skill.md` | 汎用開発 skill |
| `.codex/AGENTS.md` | Codex 固有の作業指針 |

## 2026-07-03 追記: Wails プロジェクト本体の初期作成

- Wails v2 想定の最小バックエンドを追加。
  - `main.go`
  - `app.go`
  - `go.mod`
  - `wails.json`
- Vue 3 + TypeScript + Vite の最小フロントエンドを追加。
  - `frontend/package.json`
  - `frontend/package-lock.json`
  - `frontend/index.html`
  - `frontend/vite.config.ts`
  - `frontend/tsconfig.json`
  - `frontend/src/`
- ルートの npm 補助スクリプトを `package.json` に追加。
- `.gitignore` を追加し、`node_modules/`、`frontend/dist/`、`.env` などを除外。

確認済み:

```bash
npm run build
npm audit --audit-level=moderate
```

確認済み:

- `wails dev` 起動成功。開発用 URL は `http://localhost:34115`。
- `wails build` 成功。`build/bin/AtlasNote.exe` を生成。

## 2026-07-03 追記: Go の導入

- Chocolatey での `golang` インストールは、管理者権限不足と Chocolatey の lockfile 権限問題により失敗。
- 代替として公式 `go1.26.4.windows-amd64.zip` を `.tools/go` に展開。
- SHA256 を公式値 `3ca8fb4630b07c419cbdd51f754e31363cfcfb83b3a5354d9e895c90be2cc345` と照合済み。
- ユーザー PATH に `.tools/go/bin` を追加。
- `go mod tidy` で `go.sum` を作成。

確認済み:

```bash
go version
go test ./...
```

結果:

- `go version go1.26.4 windows/amd64`
- `go test ./...` 成功

## 2026-07-03 追記: Wails CLI の導入

- Wails CLI `v2.10.1` を `.tools/go-bin` にインストール。
- Go モジュール側の Wails 依存 `github.com/wailsapp/wails/v2 v2.10.1` と CLI バージョンを合わせた。
- ユーザー PATH に `.tools/go-bin` を追加。

確認済み:

```bash
wails version
wails doctor
```

結果:

- `wails version`: `v2.10.1`
- `wails doctor`: `Your system is ready for Wails development!`

## 2026-07-03 追記: Wails dev / build の確認

確認済み:

```bash
wails dev
wails build
```

結果:

- `wails dev`: 起動成功。開発用 URL は `http://localhost:34115`。
- `wails build`: 成功。`build/bin/AtlasNote.exe` を生成。
