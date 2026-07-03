# TODO

最終更新: 2026-07-03

### 1. sandbox 環境で Node build が `EPERM` になる件（完了）

- Codex の sandbox 実行時に `npm run frontend:build` が `C:\Users\mt252` の `lstat` で失敗する原因を整理済み。
- 通常 sandbox 実行、権限付き実行、Wails build 内部実行の違いを確認済み。
- 開発手順に、Codex で確認する場合の実行方法を明記済み。
- 詳細は `docs/development/setup.md` の「Codex sandbox での確認」を参照。

### 2. `startupErr` がまだ UI / API に出ていない件（完了）

- 起動時の DB 初期化、保存先作成、Markdown Store 初期化に失敗した場合は、アプリを起動したまま保存機能を利用不可として UI に表示する方針に決定。
- Wails API の `GetStartupStatus` から初期化状態、エラーメッセージ、保存先を返すように実装済み。
- フロントエンド側で起動状態を確認し、初期化失敗時に警告を表示するように実装済み。

### 3. DB マイグレーションが最小構成の件

- 現在の `CREATE TABLE IF NOT EXISTS` 方式を継続するか、マイグレーション履歴テーブルを導入するか決める。
- スキーマ変更時のロールバック方針を整理する。
- 本格的なノート機能追加前に、最低限のマイグレーション管理方針を決める。

### 4. SQLite と Markdown の整合性が簡易対応の件

- DB 更新と Markdown ファイル更新の片方だけが成功した場合の扱いを決める。
- 作成、更新、削除それぞれの失敗時 cleanup 方針を整理する。
- 必要に応じて一時ファイル、atomic rename、再試行、整合性チェック処理を検討する。
