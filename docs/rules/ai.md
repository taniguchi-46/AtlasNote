# AI 共通ガイド

このドキュメントは、`Atlas Note` で AI Agent が作業するときの共通ルールをまとめます。

## 参照順

1. `README.md` でプロジェクト概要、思想、技術スタックを確認する。
2. `docs/status.md` で現在の状態、完了済みタスク、残タスク、保留事項を確認する。
3. `docs/rules/architecture.md` で設計、責務境界、データ構造を確認する。
4. `docs/rules/conventions.md` で命名、実装、確認方法を確認する。
5. 必要に応じて `docs/rules/loop.md` と `docs/rules/glossary.md` を参照する。

## 基本方針

- Atlas Note はローカルファーストの知識管理アプリとして扱う。
- 既存コード、既存設計、既存の命名規則を優先する。
- 変更は依頼範囲に絞る。
- README と `docs/status.md` にない仕様は断定せず、TODO または保留事項に残す。
- 実装後は可能な範囲で確認コマンドを実行する。
- テンプレート化できる知識は `.codex/templates/` に汎用化して残す。

## 文字コード

- Markdown、Vue、TypeScript、Go などのテキストファイルは UTF-8 として扱う。
- Codex が PowerShell で日本語を含むファイルを読む場合は、文字化けを避けるため `Get-Content -Encoding UTF8` を使う。
- 文字化けして見える場合でも、すぐにファイル破損と判断しない。まず UTF-8 明示読み取り、VSCode 表示、必要に応じてバイト列を確認してから判断する。
- 文字化け調査を目的としない限り、表示上の文字化けだけを理由に本文を書き換えない。

## 技術前提

| 項目 | 方針 |
| --- | --- |
| Desktop | Wails |
| Backend | Go |
| Frontend | Vue 3 + TypeScript + Vite |
| Styling | UnoCSS |
| UI | Reka UI |
| State | Composables + Pinia |
| Database | SQLite |
| Editor | Tiptap + CodeMirror |
| Storage | Markdown |
| Data Access | Repository + Squirrel |

## ドキュメント配置

| 種類 | 配置 | 内容 |
| --- | --- | --- |
| AI 共通ルール | `docs/rules/ai.md` | Agent 共通の作業方針 |
| 現在状況 | `docs/status.md` | 作業状況、完了事項、次タスク、保留事項 |
| 作業フロー | `docs/rules/loop.md` | 調査から完了報告までの流れ |
| 設計情報 | `docs/rules/architecture.md` | 構成、データ、外部連携 |
| 実装規約 | `docs/rules/conventions.md` | 命名、実装、確認方法 |
| 用語集 | `docs/rules/glossary.md` | プロジェクト内の用語 |
| Agent skill | `.agents/skills/skill.md` | 作業時に参照する汎用手順 |
