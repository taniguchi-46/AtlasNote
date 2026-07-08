# 実装計画

最終更新: 2026-07-08

## 目的

Atlas Note を、ローカルでノートを保存・表示・編集できる MVP から、Markdown を正とした安定した編集体験へ段階的に育てる。

現在は初期の「保存基盤を作る」段階を越えているため、この計画は完了済みフェーズと今後の継続フェーズを分けて管理する。

## 現状

- Wails v2、Go、Vue 3、TypeScript、Vite のプロジェクトは配置済み。
- ルート `package.json` には Wails / frontend 用の基本スクリプトがある。
- Go 側には SQLite / Markdown Storage / Repository / Service / Wails API の基本構成がある。
- フロントエンドには Vue 3、Pinia、UnoCSS、Reka UI、Tiptap、tiptap-markdown が導入済み。
- ノート本文は Markdown、メタデータは SQLite に保存する。
- エディタは Markdown モードを保存責務の中心とし、Rich / Preview モードを Markdown へ戻せる範囲の編集ビューとして扱う。
- `.env.example` には AI、WebDAV、ローカル保存先の環境変数サンプルがある。

## 開発方針

- Markdown を保存データの正とする。
- Rich 側で独自表現を増やす場合は、Markdown へ戻せることを先に確認する。
- DB / API / ファイル構成変更は、必要性・影響範囲・ロールバック方法を確認してから行う。
- UI は既存の3ペイン構成と UpNote 風 Rich 編集の方向性を尊重する。
- 画像貼り付け・添付ファイル設計は現在のエディタ整理スコープから外す。

## 確認済み

```bash
go test ./...
npm --prefix frontend run test:serializer
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

補足:

- sandbox 環境では Node.js が `C:\Users\mt252` を参照できず `EPERM` になる場合があるため、必要に応じて権限付きで再確認する。
- `frontend/wailsjs/` が未生成のクリーン環境では、先に `wails build` で Wails bindings を生成する。

## 完了済みフェーズ

### フェーズ 1: ローカルデータ基盤

目的:

- ノートをローカルに保存できる最低限の土台を作る。

完了内容:

- SQLite 初期化処理を追加。
- Markdown 本文の読み書きを担当する `MarkdownStore` を追加。
- ノートのメタデータを SQLite に保存。
- ノート本文を Markdown ファイルとして保存。
- ユーザー入力をファイルパスへ直接使わない ID ベースの保存にした。

### フェーズ 2: Repository と Service

目的:

- SQLite と Markdown Storage の詳細を UI / Wails API から隠す。

完了内容:

- `NoteRepository` で SQLite の読み書きを担当。
- `MarkdownStore` で本文ファイルの読み書きを担当。
- `NoteService` で作成、取得、更新、削除のユースケースを集約。
- Notebook Repository / Service を追加。
- ノートブック紐付けの解除を含む更新テストを追加。

### フェーズ 3: Wails API

目的:

- フロントエンドからノート操作を呼べるようにする。

完了内容:

- `App` にノート操作メソッドを追加。
- `App` にノートブック操作メソッドを追加。
- Wails の型生成結果を frontend から利用。
- 画面用 DTO を返し、SQL や内部ファイルパスを UI に漏らさない構成にした。

主な API:

- `ListNotes()`
- `GetNote(id string)`
- `CreateNote(input CreateInput)`
- `UpdateNote(id string, input UpdateInput)`
- `DeleteNote(id string)`
- `ListNotebooks()`
- `CreateNotebook(input NotebookCreateInput)`
- `UpdateNotebook(id string, input NotebookUpdateInput)`
- `DeleteNotebook(id string)`
- `ToggleAlwaysOnTop(b bool)`

### フェーズ 4: フロントエンド最小 UI

目的:

- ノート一覧、選択、編集、保存の基本操作を画面で確認できる状態にする。

完了内容:

- 3ペインレイアウトを追加。
- 上部バー、サイドバー、ノート一覧、エディタ、設定モーダルを追加。
- Pinia store 経由で Wails API を呼び出す構成にした。
- お気に入り、ピン留め、ゴミ箱の表示・操作を追加。
- ノート一覧の右クリックコンテキストメニューを追加。
- ノートブックツリーとノートブック選択を追加。

### フェーズ 5: エディタ拡張

目的:

- Markdown を中心に、Rich / Preview と往復できる編集体験を作る。

完了内容:

- Markdown モードを追加。
- Rich / Preview モードを追加。
- Markdown -> Rich -> Markdown の往復を実装。
- Rich 側の見出し・太字・リスト編集を実装。
- `editor.storage.markdown.getMarkdown()` 依存を避け、Tiptap JSON から Markdown へ戻す serializer を追加。
- raw HTML は Rich 側で実行しないように、Rich 読み込み前にタグ文字列をエスケープ。
- serializer の基本テストを追加。

実機確認済み:

- Markdown モードで `## chatgpt できる` が表示される。
- Preview 切り替え後に Markdown へ戻っても `##` が保持される。
- Rich 側で見出し2・太字・リストを作成し、Markdown に正しく保存される。
- `## aa -> Rich -> Markdown` が `## aa` に戻る。
- `<div onclick="alert(1)">test</div>` が Rich 側で実行されない。

## 継続フェーズ

### フェーズ 6: Markdown / Rich 仕様の明文化

目的:

- 今後の後戻りを減らすため、エディタの保存責務と対応範囲を明文化する。

実装候補:

- Markdown を保存責務の中心にする方針を `docs/rules/architecture.md` へ反映する。
- Rich は Markdown への変換可能範囲に限定する方針を明文化する。
- raw HTML の扱いを仕様として明文化する。
- serializer の対応範囲と未対応範囲を記録する。

確認すること:

- Rich 側で HTML が実行されないこと。
- Markdown 原文を壊さないこと。
- Rich 編集後に Markdown として保存されること。

### フェーズ 7: table 編集 UI の再設計

目的:

- table を Markdown 中心設計と矛盾しない範囲で扱えるようにする。

確定仕様:

- Markdown を保存データの正とし、Rich table は Markdown table へ戻せる範囲の編集 UI として扱う。
- MVP では GitHub Flavored Markdown 互換の基本 table を対象にする。
- 対応する操作は table 挿入、行追加、列追加、行削除、列削除、表削除までにする。
- セル内の複数段落やリストは Markdown 保存時に `<br>` を含む単一セル表現へ正規化される。
- セル結合、セル分割、複数ヘッダー行、セル単位の背景色、文字寄せ、列幅の Markdown 永続化、table 内 table は MVP では扱わない。
- 旧 table 操作用 CSS は整理済み。新しい table 操作 UI は Rich モードの format bar に集約する。

注意点:

- 先に Markdown table へ落とせる範囲を決める。
- Tiptap Table の標準コマンドを優先し、不要なライブラリは追加しない。
- Rich 側だけで表現できる機能を増やしすぎない。
- `TableCell` / `TableHeader` のカスタムスキーマで table 内 table を作れない制約を維持する。

### フェーズ 8: serializer の拡張とテスト

目的:

- Rich 編集で扱える Markdown 変換範囲を安全に広げる。

現対応:

- heading
- bold / italic / strike / inline code / link
- bullet list / ordered list / task list
- blockquote
- code block
- horizontal rule
- table
- image

未対応:

- footnote
- frontmatter
- reference link
- Markdown コメント
- HTML block の高度な保持

方針:

- 変換範囲を増やす場合は、先に `frontend/scripts/test-serializer.mjs` へテストを追加する。
- 実装は `frontend/src/utils/tiptapMarkdownSerializer.ts` に閉じる。
- `NoteEditor.vue` 側へ個別変換ロジックを増やしすぎない。

### フェーズ 9: WebDAV / AI 連携の設計

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

## 保留事項

- 設定モーダルの実機表示確認。
- 常に最前面ボタンの実機動作確認。
- 添付ファイル保存設計。
- デスクトップアプリの対応 OS と配布方式。
- 高度な全文検索の方式。

## 確認コマンド

開発中の基本確認:

```bash
go test ./...
npm --prefix frontend run test:serializer
npm --prefix frontend run typecheck
npm --prefix frontend run build
```

必要に応じて:

```bash
wails doctor
wails build
```

## 次の実装タスク候補

1. Markdown / Rich エディタ仕様を `docs/rules/architecture.md` に反映する。
2. table 編集 UI の仕様を決める。
3. 設定モーダルと常に最前面機能を実機で再確認する。
