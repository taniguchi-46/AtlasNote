# プロジェクト状況

最終更新: 2026-07-11

## 概要

| 項目 | 内容 |
| --- | --- |
| プロジェクト名 | `Atlas Note` |
| 種別 | ローカルファーストのデスクトップ知識管理 / Second Brain アプリ |
| フレームワーク | Wails + Vue 3 + Vite |
| 言語 | Go + TypeScript |
| スタイル | UnoCSS + Reka UI |
| エディタ | Markdown textarea + Tiptap Rich / Preview |
| データ保存 | Markdown 本文 + SQLite メタデータ |
| 実行環境 | Wails デスクトップアプリ、開発時は Go / Node.js / Vite |
| 配信 / デプロイ | デスクトップアプリとして配布予定。具体的な配布方式は未確定 |

## 主要コマンド

```bash
npm run dev
npm run build
npm run frontend:build
npm run frontend:typecheck
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:serializer
go test ./...
wails dev
wails build
```

補足:

- `frontend/wailsjs/` は Git 管理対象外。クリーン checkout 直後にフロントエンド単体ビルドが失敗する場合は、先に `wails build` で Wails bindings を生成する。
- sandbox 環境では Node.js 実行時に `EPERM: operation not permitted, lstat 'C:\Users\mt252'` が出る場合がある。その場合は権限付きで再実行して確認する。

## 完了済み

- Wails v2 想定の最小バックエンドを作成。
  - `main.go`
  - `app.go`
  - `go.mod`
  - `wails.json`
- Vue 3 + TypeScript + Vite のフロントエンドを作成。
  - `frontend/package.json`
  - `frontend/package-lock.json`
  - `frontend/index.html`
  - `frontend/vite.config.ts`
  - `frontend/tsconfig.json`
  - `frontend/src/`
- Go 側のローカル保存基盤を実装。
  - SQLite 初期化
  - Markdown Storage
  - Note Repository / Service
  - Notebook Repository / Service
  - SQLite / Markdown 操作ジャーナル、補償処理、起動時復旧
- Wails API を実装。
  - ノート作成・一覧取得・単体取得・更新・削除
  - ノートブック作成・一覧取得・更新・削除
  - 常に最前面切り替え
  - 起動状態取得
- フロントエンド UI を実装。
  - 3ペインレイアウト
  - 上部バー
  - サイドバー
  - ノート一覧
  - ノート右クリックのコンテキストメニュー
  - ノートブックツリー
  - 設定モーダル（Reka UI Dialog を試験導入）
- エディタ基盤を実装。
  - Markdown モード
  - Rich / Preview モード
  - Markdown -> Rich -> Markdown の往復
  - Rich 側の見出し・太字・リスト編集
  - Tiptap JSON から Markdown へ戻す serializer
  - raw HTML を Rich 側で実行しないためのエスケープ
  - serializer の基本テスト
- AI Agent 向けの共通ドキュメントを `docs/` に配置。
- Codex 固有の作業指針を `.agents/AGENTS.md` に配置。
- 汎用開発 skill を `.agents/skills/skill.md` に配置。

## 現在の設計方針

- Markdown を保存データの正とする。
- SQLite と Markdown の更新は操作ジャーナルで追跡し、通常失敗時は補償、異常終了時は次回起動時に復旧する。
- 孤立Markdownは `notes/recovery/` へ退避し、DBレコードに対応する本文が欠損した場合はデータを自動削除せず起動エラーにする。
- Markdown モードは Joplin 風に、Markdown 原文をそのまま編集する。
- Rich / Preview モードは UpNote 風に、Markdown へ戻せる範囲の編集ビューとして扱う。
- Rich 側では見出し記号の `##` は表示せず、Markdown モードでは `## title` のような記法を表示する。
- raw HTML は Markdown モードで原文編集を許可するが、Rich 側では HTML として実行しない。
- 画像貼り付け、ドラッグ＆ドロップ、添付ファイル、バックアップ、グローバルショートカットは MVP 外とする。
- テーブルコピーは Phase 2 へ移し、文字寄せは現行の table 仕様に含めない。

## 次にやること

1. Markdown / Rich エディタ仕様の明文化
   - Markdown を保存責務の中心にする方針を設計ドキュメントへ反映する。
   - Rich は Markdown への変換可能範囲に限定する方針を明文化する。
   - raw HTML の扱いを仕様として明文化する。
2. 保存失敗時の表示とデータ整合性の確認
   - 保存失敗時に「保存済み」を表示しないようにする。
   - SQLite と Markdown の更新失敗時の整合性を確認する。

詳細: `docs/todo/todo-mvp.md`

## 保留事項

- デスクトップアプリの対応 OS と配布方式。
- 添付ファイルの保存ディレクトリ構成。
- WebDAV 同期時の競合解決方針。
- ユーザー API Key の保存方式と暗号化方針。
- AI 機能の対象プロバイダ、モデル選択、課金表示の扱い。
- 高度な全文検索の方式。

## 関連ファイル

| ファイル | 役割 |
| --- | --- |
| `README.md` | プロジェクト概要、技術スタック、概略アーキテクチャ |
| `docs/todo/todo-mvp.md` | MVP の現行タスク台帳 |
| `docs/development/implementation-plan.md` | 実装計画 |
| `docs/development/tech-stack.md` | 採用技術 |
| `docs/development/setup.md` | 開発環境セットアップと起動方法 |
| `docs/rules/ai.md` | AI Agent 共通ルール |
| `docs/rules/architecture.md` | 設計情報 |
| `docs/rules/conventions.md` | 実装規約 |
| `docs/rules/glossary.md` | 用語集 |
| `docs/rules/BRANCHING.md` | ブランチ、コミット、PR の運用ルール |
| `.env.example` | ローカル環境変数のサンプル。実値は含めない |
| `.agents/skills/skill.md` | 汎用開発 skill |
| `.agents/AGENTS.md` | Codex 固有の作業指針 |

## 確認済みメモ

### 2026-07-03: Wails プロジェクト本体の初期作成

- Wails v2 想定の最小バックエンドを追加。
- Vue 3 + TypeScript + Vite の最小フロントエンドを追加。
- ルートの npm 補助スクリプトを `package.json` に追加。
- `.gitignore` を追加し、`node_modules/`、`frontend/dist/`、`.env` などを除外。
- `wails dev` 起動成功。開発用 URL は `http://localhost:34115`。
- `wails build` 成功。`build/bin/AtlasNote.exe` を生成。

### 2026-07-03: Go の導入

- 公式 `go1.26.4.windows-amd64.zip` を `.tools/go` に展開。
- SHA256 を公式値と照合済み。
- ユーザー PATH に `.tools/go/bin` を追加。
- `go mod tidy` で `go.sum` を作成。

確認済み:

```bash
go version
go test ./...
```

### 2026-07-03: Wails CLI の導入

- Wails CLI `v2.10.1` を `.tools/go-bin` にインストール。
- Go モジュール側の Wails 依存 `github.com/wailsapp/wails/v2 v2.10.1` と CLI バージョンを合わせた。
- ユーザー PATH に `.tools/go-bin` を追加。

確認済み:

```bash
wails version
wails doctor
```

### 2026-07-06: MVP UI タスクの実装

- メイン画面上部バーを追加。
- エディタ側のノートブック選択プルダウンを削除。
- 設定モーダルとフォント設定などの UI を追加。
- 表操作用 BubbleMenu、`Ctrl + Enter` による行追加、ネスト table の制約を試験実装。
- ノートブックアイコンピッカー機能を実装。

### 2026-07-08: Markdown / Rich エディタ整理

- Markdown モードを保存責務の中心に整理。
- Rich / Preview モードは Markdown へ戻せる範囲の編集ビューとして整理。
- `editor.storage.markdown.getMarkdown()` 依存を避け、Tiptap JSON から Markdown へ戻す serializer を追加。
- raw HTML は Rich 側で実行しないように、Rich 読み込み前にタグ文字列をエスケープ。
- 実機で以下を確認済み。
  - Markdown モードで `## chatgpt できる` が表示される。
  - Preview 切り替え後に Markdown へ戻っても `##` が保持される。
  - Rich 側で見出し2・太字・リストを作成し、Markdown に正しく保存される。
  - `## aa -> Rich -> Markdown` が `## aa` に戻る。
  - `<div onclick="alert(1)">test</div>` が Rich 側で実行されない。

### 2026-07-08: table 編集 UI 仕様の確定

- Markdown を保存データの正とし、Rich table は Markdown table へ戻せる範囲の編集 UI として扱う方針にした。
- MVP では GitHub Flavored Markdown 互換の基本 table を対象にする。
- Rich 側では table 挿入、行追加、列追加、行削除、列削除、表削除までを提供する方針にした。
- セル結合、セル分割、複数ヘッダー行、セル単位の装飾、列幅の Markdown 永続化、table 内 table は MVP では扱わない。
- Rich 側の table 挿入は実機表示確認済み。
- table 選択中の行追加、列追加、行削除、列削除、表削除 UI を format bar に追加。
- 旧 table 操作用 CSS は `frontend/src/style.css` から整理済み。
- table 選択中の行追加、列追加、行削除、列削除、表削除は実機動作確認済み。
- table 作成済みノートで Markdown / Preview 往復しても内容が保持されることを実機確認済み。
- 別ノートから table 作成済みノートを開いても内容が消えないことを実機確認済み。

### 2026-07-10: 主要インタラクションへ Reka UI を適用

- 設定モーダルを Dialog、設定カテゴリーを Tabs へ変更。
- ノートブック作成を Dialog、削除確認を AlertDialog へ変更。
- ノートブックアイコン一覧を RadioGroup、ツリー上のアイコン選択を Popover へ変更。
- ノート右クリック操作を ContextMenu へ変更。ノートブック操作は従来の3アイコンボタンを維持。
- 独自の右クリック座標管理、外側クリック監視、hover 式サブメニューを Reka UI 側へ移管。
- 設定モーダルの試験導入は Wails 実機で問題がないことを確認済み。
- 型検査、serializer テスト、フロントエンドビルド、Go テスト、Wails 本番ビルドは成功。
- 今回追加した作成・削除・Popover・ContextMenu は Wails 実機確認が必要。

### 2026-07-10: MVP スコープの確定

- 画像貼り付け、ドラッグ＆ドロップ、添付ファイル保存を MVP 外へ移動。
- 自動バックアップ、バックアップ復元、グローバルショートカットを MVP 外へ移動し、設定画面の無効な UI を削除。
- テーブルコピーを Phase 2 へ移動。
- テキスト整列を現行の table 仕様から削除。
- 行・列操作は、現在実装済みの行追加、列追加、行削除、列削除、表削除へ統一。

### 2026-07-10: serializer TODO の整理

- heading、inline mark、list、task list、blockquote、code block、horizontal rule、hard break、table、image の基本変換テストを確認済み。
- footnote、frontmatter、reference link、Markdown コメント、高度な HTML block 保持は MVP 外の未対応範囲として明文化。
- 今後変換範囲を増やす場合は、serializer テストを先に追加する方針へ統一。

### 2026-07-10: 常に最前面機能の実機確認

- 上部バーの常に最前面ボタンから、Wails 実機で前面状態を切り替えられることを確認済み。

### 2026-07-10: SQLite / Markdown 整合性対策

- `note_storage_operations` を追加し、本文を伴う作成・更新と完全削除の途中状態をSQLiteへ記録。
- Markdown確定失敗時のSQLite補償処理と、操作ID付き一時ファイル・削除退避ファイルを追加。
- 起動時に未完了操作を再開し、本文ハッシュ、`content_path`、Markdownファイルの存在を検証する処理を追加。
- 孤立Markdownと未追跡の一時ファイルを `notes/recovery/` へ退避する処理を追加。
- Serviceの操作を直列化し、重複する自動保存によるタイトルと本文の世代ずれを防止。
- 更新・削除の異常終了復旧、本文欠損、孤立ファイル、同時更新、DBマイグレーションのテストを追加。

### 2026-07-11: 未保存状態と終了・保存失敗への対応

- ノート単位でタイトル、Markdown本文、revision、保存状態をdirty draftとしてPiniaストアへ保持するようにした。
- debounce中の保存と進行中の保存をflushし、ノート切替やエディタ破棄後も保存処理を継続できるようにした。
- 保存失敗後もdraftを保持し、元のノートを開いたときに未保存内容を復元するようにした。
- 保存失敗表示に再試行と確認付き破棄を追加し、最新revisionの保存成功時だけdirtyを解除するようにした。
- Wailsの終了要求を一度保留し、全dirtyノートの保存成功、または確認済みの全破棄後に終了する処理を追加した。
- 保存失敗後の切替・再試行、古いrevision、debounce直後のflush、複数の進行中保存を待機するテストを追加した。

### 2026-07-11: ノート選択の非同期応答逆転への対応

- ノート選択要求に世代判定を追加し、最新要求だけがactive note、エラー、読み込み状態を更新するようにした。
- A→Bの取得応答をB→Aの順で完了させ、Bが維持されるテストを追加した。
- 古い選択要求の失敗が最新ノートの表示にエラーを反映しないこともテストした。

### 2026-07-11: データディレクトリ単位の単一writer保証

- SQLiteとMarkdownを初期化する前に、データディレクトリ直下の `atlasnote.lock` をOSレベルで排他取得するようにした。
- 同じデータディレクトリを使用する2つ目のアプリは起動エラーとし、DB・Note Service・Markdown更新処理へ到達させないようにした。
- 正常終了時と初期化失敗時にロックを解放し、終了後は同じデータディレクトリを再取得できるようにした。
- 同じデータディレクトリの二重取得拒否、別ディレクトリの同時取得、解放後の再取得、拒否された起動で既存Markdownが変化しないことをテストした。
- クラウド同期・履歴機能に必要なrevision/CASは今回の対象外とし、着手前の必須タスクとして維持する。

### 2026-07-11: Notebook階層の循環禁止

- Notebook更新時に再帰CTEで子孫関係を検査し、自己・子・孫を親に指定する更新をvalidation errorとして拒否するようにした。
- フロントエンドのNotebook Storeでも同じ階層判定を行い、循環する移動はWails API呼び出し前に拒否するようにした。
- 自己・子・孫への移動拒否、正常な別ツリーとルートへの移動、孫を含む再帰削除をテストした。
- Go全体テスト、Notebook階層テスト、フロントエンド型検査、フロントエンド本番ビルドは成功。

### 2026-07-11: migration境界とSQLite接続設定の保証

- 現行コードより新しい `user_version` のDBを、WAL設定や互換化DDLを実行する前に明示的に拒否するようにした。
- migration失敗時に、途中のDDLと `user_version` が同一トランザクションでrollbackされることをテストした。
- `foreign_keys = ON` と `busy_timeout = 5000` をmodernc SQLiteのDSNへ設定し、接続プールが生成する全接続へ適用するようにした。
- WALはDB初期化時に設定し、`journal_mode = wal` が返ることを検証するようにした。
- 2つの並行接続とDB再接続後の接続で、PRAGMA値と外部キー違反の拒否をテストした。
- databaseパッケージの対象テストとGo全体テストは成功。race detectorは実行環境が `CGO_ENABLED=0` のため未実行。

### 2026-07-11: Critical/High項目のCI設定

- GitHub ActionsのWindowsクリーン環境で、Go全体テスト、Wails本番ビルド、フロントエンド型検査、serializerテスト、自動保存・ノート選択・削除・Notebook階層の回帰テストを実行するworkflowを追加した。
- 同一ブランチの古いCI実行をキャンセルし、リポジトリ内容の読み取り権限だけで動作する構成にした。
- `frontend`からリポジトリルートを参照していた未使用のローカル依存を削除し、`npm ci`直後のWails bindings生成が`node_modules`をGoパッケージとして走査する問題を解消した。
- ローカルでは`npm ci`、Go全体テスト、Wailsクリーンビルド、フロントエンド型検査、全対象テストの成功を確認した。
- GitHub Actions初回実行では、Wailsビルド前のGoテストが未生成の`frontend/dist`をembedできず失敗した。Wailsビルドを先に実行する順序へ修正した。
- 修正後のGitHub ActionsでWails本番ビルド、Go全体テスト、フロントエンド型検査、全対象テストの成功を確認し、`docs/todo/todo-mvp.md`の対象項目を完了へ更新した。
