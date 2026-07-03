# TODO

最終更新: 2026-07-03

## 前提

以下の改善を先に行う。

- Codex 側で UTF-8 の Markdown / Vue ファイルを正しく読めるようにする。
- Codex 側の実行環境で `go` / `wails` の PATH を正しく扱えるようにする。

## 1, 2 の改善後に行うこと

### 1. sandbox 環境で Node build が `EPERM` になる件

- Codex の sandbox 実行時に `npm run frontend:build` が `C:\Users\mt252` の `lstat` で失敗する原因を整理する。
- 通常実行、権限付き実行、Wails build 内部実行の違いを確認する。
- 開発手順に、Codex で確認する場合の実行方法を明記する。

### 2. `startupErr` がまだ UI / API に出ていない件

- 起動時の DB 初期化、保存先作成、Markdown Store 初期化に失敗した場合の扱いを決める。
- Wails API から初期化エラーを返せるようにする。
- フロントエンド側で初期化失敗時の表示方針を決める。

### 3. DB マイグレーションが最小構成の件

- 現在の `CREATE TABLE IF NOT EXISTS` 方式を継続するか、マイグレーション履歴テーブルを導入するか決める。
- スキーマ変更時のロールバック方針を整理する。
- 本格的なノート機能追加前に、最低限のマイグレーション管理方針を決める。

### 4. SQLite と Markdown の整合性が簡易対応の件

- DB 更新と Markdown ファイル更新の片方だけが成功した場合の扱いを決める。
- 作成、更新、削除それぞれの失敗時 cleanup 方針を整理する。
- 必要に応じて一時ファイル、atomic rename、再試行、整合性チェック処理を検討する。
