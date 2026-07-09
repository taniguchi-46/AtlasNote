# TODO

## todoの内容

MVP (v0.1) 機能の実装。ローカルで動作する Markdown / Rich 対応の3ペイン型ノートアプリを構築する。

このファイルは、現在の実装状態に合わせた MVP の作業台帳として扱う。古い実装計画の詳細は `docs/development/implementation-plan.md`、設計ルールは `docs/rules/architecture.md` を参照する。

## 現状/前提

- Wails + Go + Vue 3 + Vite のプロジェクトは構築済み。
- ノート本文は Markdown、メタデータは SQLite に保存する方針で実装済み。
- Go 側には SQLite / Markdown Storage / Repository / Service / Wails API の基本構成がある。
- フロントエンドは3ペイン UI、上部バー、サイドバー、ノート一覧、エディタ、設定画面の基本構成がある。
- エディタは Markdown モードを保存責務の中心とし、Rich モードは UpNote 風の編集ビューとして扱う。
- Markdown / Rich 往復では、Markdown を正とし、Rich 側の内容は `frontend/src/utils/tiptapMarkdownSerializer.ts` で Markdown へ戻す。
- 画像貼り付け・添付ファイル設計は今回のエディタ整理スコープから外す。

## 実装済み

- データ層
  - [x] SQLite 接続・初期化
  - [x] Markdown ファイル読み書き
  - [x] ノート Repository / Service
  - [x] ノートブック Repository / Service
- Wails API
  - [x] ノート一覧取得
  - [x] ノート作成・更新・削除
  - [x] お気に入り・ピン留め・ゴミ箱
  - [x] ノートブック一覧取得・作成・更新・削除
  - [x] 常に最前面 API 呼び出し
- フロントエンド UI
  - [x] 3ペインレイアウト
  - [x] 上部バー
  - [x] サイドバー
  - [x] ノート一覧
  - [x] ノート右クリックのコンテキストメニュー
  - [x] ノートブックツリー
  - [x] 設定モーダル
- エディタ
  - [x] Markdown モード
  - [x] Rich / Preview モード
  - [x] Markdown -> Rich -> Markdown の往復
  - [x] Rich 側の見出し・太字・リスト編集
  - [x] Rich 更新内容の Markdown 保存
  - [x] raw HTML を Rich 側で実行しないためのエスケープ
  - [x] serializer の基本テスト

## 現在のエディタ仕様

- Markdown モード
  - Markdown 原文をそのまま編集する。
  - 見出しは `## title` のように Markdown 記法で表示する。
  - 保存対象は `localMarkdown` の内容。
- Rich / Preview モード
  - Tiptap の `EditorContent` で編集する。
  - 見出しは見た目として表示し、`##` は表示しない。
  - Rich 側の変更は Tiptap JSON から Markdown へ serialize して保存する。
- raw HTML
  - Markdown モードでは原文編集を許可する。
  - Rich モードでは HTML として実行しない。
  - Rich へ読み込む前にタグ文字列をエスケープする。
  - Rich 編集後はエスケープ済みテキストとして正規化される可能性がある。

## table 編集 UI 仕様

- 基本方針
  - Markdown を保存データの正とし、Rich table は Markdown table へ戻せる範囲の編集 UI として扱う。
  - MVP では GitHub Flavored Markdown 互換の基本 table を対象にする。
  - 独自の表属性や Markdown へ安定変換できない表現は MVP では扱わない。
- Markdown table と Rich table の対応範囲
  - 対応する: 見出し行、通常セル、行追加、列追加、行削除、列削除、表削除、列幅リサイズ、セル内の基本インライン装飾。
  - 制限付きで対応する: セル内の複数段落やリストは Markdown 保存時に `<br>` を含む単一セル表現へ正規化される。
  - 対応しない: セル結合、セル分割、複数ヘッダー行、セル単位の背景色、文字寄せ、列幅の Markdown 永続化、table 内 table。
- Rich 側で提供する操作
  - ツールバーから `3 x 3` の table を挿入できるようにする。
  - table 選択中のみ、行追加、列追加、行削除、列削除、表削除を提供する。
  - 操作は Tiptap Table の標準コマンドを優先し、追加ライブラリは入れない。
  - セル結合、セル分割、ヘッダー切り替えは Markdown 変換仕様が固まるまで UI に出さない。
- 旧 UI / CSS の扱い
  - `frontend/src/style.css` に残っていた `.table-action-*` 系の旧 table 操作用 CSS は整理済み。
  - `frontend/src/components/NoteEditor.vue` には対応する旧 table 操作 UI 本体は残っていない。
  - 新しい table 操作 UI は Rich モードの format bar に集約する。
- 制約
  - `TableCell` / `TableHeader` のカスタムスキーマで table 内 table を作れない制約を維持する。
  - serializer の table 出力仕様を変更する場合は、先に `frontend/scripts/test-serializer.mjs` へテストを追加する。

## 直近の残タスク

- serializer の継続確認
  - [ ] heading / mark / list / task list / blockquote / code block / table / image の基本変換を維持する。
  - [ ] footnote / frontmatter / reference link / Markdown コメントは未対応として扱う。
  - [ ] 変換範囲を増やす場合は serializer テストを先に追加する。


## MVP 外 / 別管理

- 画像貼り付け・ドラッグ&ドロップ・添付ファイル保存設計
- WebDAV / 外部同期
- AI 連携
- 高度な全文検索
- モバイル対応

## 注意事項

- 既存の Wails / Go / TypeScript / Vue 3 / UnoCSS の設計に合わせる。
- Markdown を保存データの正とし、Rich 側は Markdown へ戻せる範囲から広げる。
- 大規模リファクタリング、不要なライブラリ追加、DB / API / ファイル構成変更は避ける。
- 不明点は推測で固定せず、保留事項に残すか確認する。
- 実装後は可能な範囲で確認コマンドを実行する。

## 確認コマンド

```bash
npm --prefix frontend run test:serializer
npm --prefix frontend run typecheck
npm --prefix frontend run build
go test ./...
```
