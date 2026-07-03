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
| `.agents/skills/skill.md` | 汎用開発 skill |
| `.codex/AGENTS.md` | Codex 固有の作業指針 |
