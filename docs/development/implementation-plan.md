# 実装計画

最終更新: 2026-07-03

## 目的

Atlas Note をドキュメント整備と Wails 最小構成の段階から、実際にノートを保存・表示・編集できる開発段階へ移行する。

当面はローカルファーストの基盤を優先し、AI 連携、WebDAV 同期、高機能エディタは土台が安定してから段階的に追加する。

## 現状

- Wails v2、Go、Vue 3、TypeScript、Vite の最小プロジェクトは配置済み。
- ルート `package.json` には Wails / frontend 用の基本スクリプトがある。
- `go.mod` は Wails 依存のみの最小構成。
- frontend は現時点で Vue 以外のアプリ用依存をまだ追加していない。
- `.env.example` には AI、WebDAV、ローカル保存先の環境変数サンプルがある。
- 実装済みの Go API は `Greet` のみで、ノート保存・DB・Markdown Storage は未実装。
- 画面文言と一部ドキュメントで文字化けして見える箇所があるため、実装前または初期実装時に確認が必要。

## 開発に移れるか

結論として、開発には移れる。

ただし、最初に着手する範囲は「ノート保存基盤の最小実装」に絞る。SQLite、Markdown Storage、Repository、Wails API の境界を先に作り、Tiptap、CodeMirror、WebDAV、AI 連携は後続フェーズに分ける。

## 確認済み

```powershell
.\.tools\go\bin\go.exe test ./...
npm run frontend:build
```

結果:

- Go テストは成功。現時点ではテストファイルなし。
- frontend build は成功。
- 通常 PATH では `go` が見つからなかったため、`.tools/go/bin/go.exe` を直接使って確認した。
- sandbox 環境では Node.js が `C:\Users\mt252` を参照できず `EPERM` になったため、frontend build は権限付きで再確認した。

## 懸念事項

### 文字化け

`README.md`、`docs/*.md`、`frontend/src/App.vue` の表示文言に文字化けして見える箇所がある。

対応方針:

- 実際にファイル内容が壊れているか、表示環境のエンコード問題かを確認する。
- UI に出る文言は、現在の機能に合わせて正常な日本語へ修正する。
- ドキュメントの全面修正は範囲が大きいため、開発に必要な箇所から最小限で直す。

### PATH

Go と Wails CLI は `.tools/` 配下に配置されているが、現在の PowerShell セッションでは `go` が見つからなかった。

対応方針:

- 開発時の確認コマンドは、必要に応じて `.tools/go/bin/go.exe` と `.tools/go-bin/wails.exe` を直接指定する。
- 手順書には PATH 反映が新しい PowerShell で有効になる可能性があることを明記する。

### 依存追加

設計上は UnoCSS、Reka UI、Pinia、Squirrel、SQLite、Tiptap、CodeMirror が予定されているが、現時点では多くが未導入。

対応方針:

- 最初の実装で必要な依存だけ追加する。
- エディタ系や UI 系の依存は、最小 CRUD が動いてから導入する。
- 依存追加前に目的、影響範囲、代替案を確認する。

## フェーズ 1: ローカルデータ基盤

目的:

- ノートをローカルに保存できる最低限の土台を作る。

実装候補:

- Go 側に `internal/note`、`internal/storage`、`internal/repository` などの責務別パッケージを追加する。
- SQLite 初期化処理を追加する。
- Markdown 本文の保存先ディレクトリを決める。
- ノートのメタデータを SQLite に保存する。
- ノート本文を Markdown ファイルとして保存する。

最小データ項目:

- `id`
- `title`
- `content_path`
- `created_at`
- `updated_at`

注意点:

- DB スキーマ変更は小さく始める。
- Markdown ファイル名は安全な ID ベースにする。
- ユーザー入力をファイルパスへ直接使わない。
- 保存先ディレクトリは `.env` またはアプリ既定値から決める。

## フェーズ 2: Repository と Service

目的:

- SQLite と Markdown Storage の詳細を UI / Wails API から隠す。

実装候補:

- `NoteRepository` で SQLite の読み書きを担当する。
- `MarkdownStore` で本文ファイルの読み書きを担当する。
- `NoteService` で作成、取得、更新、削除のユースケースをまとめる。

確認すること:

- エラー時に秘密情報やローカル絶対パスを出しすぎない。
- 入力値の空文字、長すぎるタイトル、不正 ID を検証する。
- DB と Markdown の片方だけ成功した場合の扱いを決める。

## フェーズ 3: Wails API

目的:

- フロントエンドからノート操作を呼べるようにする。

実装候補:

- `App` にノート操作メソッドを追加する。
- Wails の型生成結果を frontend から使う。
- 直接 SQL やファイルパスを返さず、画面用 DTO を返す。

API 候補:

- `ListNotes()`
- `GetNote(id string)`
- `CreateNote(input CreateNoteInput)`
- `UpdateNote(id string, input UpdateNoteInput)`
- `DeleteNote(id string)`

## フェーズ 4: フロントエンド最小 UI

目的:

- ノート一覧、選択、編集、保存の基本操作を画面で確認できる状態にする。

実装候補:

- `frontend/src/components/` に一覧、編集領域、空状態を分ける。
- `frontend/src/composables/` に Wails API 呼び出しをまとめる。
- 最初はプレーンな textarea で編集し、Tiptap / CodeMirror は後続にする。

確認すること:

- ローディング状態
- 空データ時の表示
- 保存失敗時の表示
- 画面文言の文字化け
- XSS につながる危険な HTML 挿入がないこと

## フェーズ 5: エディタ拡張

目的:

- 最小 CRUD の安定後に、編集体験を改善する。

実装候補:

- Markdown 編集を CodeMirror で扱う。
- プレビューまたはリッチ編集が必要な場合に Tiptap の導入範囲を決める。
- キーボード操作と保存状態表示を追加する。

注意点:

- Tiptap / CodeMirror の両方を同時に入れると責務が曖昧になりやすいため、先に用途を分ける。
- 依存追加前に、最小構成で解決できるかを確認する。

## フェーズ 6: WebDAV / AI 連携の設計

目的:

- ローカル保存基盤を前提に、同期と AI 機能を安全に追加する。

WebDAV:

- 競合解決方針を決める。
- 同期対象を SQLite、Markdown、添付ファイルのどこまでにするか決める。
- 失敗時にローカルデータを壊さない方針を明文化する。

AI:

- API Key の保存方式を決める。
- ログに API Key、会話内容、個人情報を出しすぎない。
- レート制限、タイムアウト、コスト表示の扱いを決める。

## 確認コマンド

開発中の基本確認:

```powershell
.\.tools\go\bin\go.exe test ./...
npm run frontend:build
```

必要に応じて:

```powershell
.\.tools\go-bin\wails.exe doctor
.\.tools\go-bin\wails.exe build
```

## 最初の実装タスク候補

1. UI とドキュメントの文字化け箇所を確認し、開発に直接影響する箇所だけ直す。
2. SQLite / Markdown Storage / Repository の最小パッケージ構成を作る。
3. ノート CRUD を Wails API と Vue の最小 UI に接続する。
