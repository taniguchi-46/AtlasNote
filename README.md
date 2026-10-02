# Atlas Note

> AIを前提としたローカルファーストの知識管理・Second Brainアプリ

Atlas Noteは、Markdownをノート本文の正本として、SQLiteでメタデータや検索索引を管理するデスクトップアプリです。現在はCLI / MCP / Terminalの再設計を完了し、Multi TerminalとSplitを次の開発テーマとしています。

## 主な特徴

- Markdown本文とSQLiteメタデータによるローカルファースト設計
- ノート整理、検索、タグ、WebDAV同期、バックアップ
- Go backendとVue frontendを接続するWailsデスクトップアプリ
- 外部CLI / MCP連携と統合Terminal

## 技術スタック

Wails v2、Go、TypeScript、Vue 3、Vite、UnoCSS、SQLiteを使用します。詳細は[技術スタック](docs/development/guides/tech-stack.md)を参照してください。

## 開発を始める

- [セットアップ手順](docs/development/guides/setup.md)
- [開発者向けガイド](docs/development/guides/beginner-guide.md)
- [利用者向けガイド](docs/user-readme.md)

## ドキュメント

- [現在状況と次の作業](docs/status.md)
- [ドキュメント入口・正本一覧](docs/README.md)
- [機能仕様・開発資料](docs/development/README.md)
- [恒久ルール](docs/rules/ai.md)
- [完了済み記録](docs/archive/README.md)

開発作業では、まず `AGENTS.md`、`docs/status.md`、対象領域のルールと仕様を確認してください。