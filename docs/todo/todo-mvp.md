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
- 画像貼り付け、ドラッグ＆ドロップ、添付ファイル、バックアップ、グローバルショートカットは MVP 外とする。
- テーブルコピーは Phase 2 へ移し、文字寄せは現行の table 仕様に含めない。

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

## serializer の確認結果と継続方針

- [x] heading / mark / list / task list / blockquote / code block / horizontal rule / hard break / table / image の基本変換テストを追加済み。
- [x] footnote / frontmatter / reference link / Markdown コメント / 高度な HTML block 保持は未対応範囲として明文化済み。
- 変換範囲を増やす場合は、先に `frontend/scripts/test-serializer.mjs` へテストを追加する。


## MVP 外 / 別管理

- 画像貼り付け・ドラッグ&ドロップ・添付ファイル保存設計
- 自動バックアップ・バックアップ復元
- グローバルショートカット
- テーブルコピー（Phase 2）
- serializer の footnote / frontmatter / reference link / Markdown コメント / 高度な HTML block 保持
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

## 移行前に必ず対応する項目

優先度の高い順に対応する。各項目は実装だけでなく、記載した競合・異常系テストが成功した時点で完了とする。

### Critical

- [x] 自動保存の対象ノートを固定する。
  - `NoteEditor` のdebounce登録時に note ID、本文、タイトル、revision（または同等の世代情報）を一組で保持する。
  - ノートAを編集してから1秒以内にノートBへ切り替えても、Aの本文がBへ保存されないことをテストする。
  - ノート切替時の旧タイマーをflushまたはcancelし、古い保存結果を現在のエディタへ反映しない。

### High

- [x] 未保存状態を保持し、終了・保存失敗時の入力消失を防止する。
  - [x] アプリ終了・エディタ破棄前にdebounce中の編集をflushする。
  - [x] 保存失敗後もdirty内容を保持し、再試行または明示的な破棄を可能にする。
  - [x] 保存失敗後に別ノートへ切り替えても、未保存内容が回収不能にならないことをテストする。

- [ ] ノート選択の非同期応答を最新要求だけ反映する。
  - A→Bの取得応答が逆順で完了しても、最後に選択したBが表示されることをテストする。

- [ ] 同一データディレクトリへの複数writerを防止する。
  - MVPでは単一起動またはデータディレクトリ単位のプロセス間排他を採用する。
  - 同時起動時に古いMarkdownが新しい内容を上書きしないことを確認する。
  - 同期・履歴機能の開始前にrevisionまたはexpectedUpdatedAtによるCASを追加する。

- [ ] Markdown削除確定失敗を成功扱いしない。
  - `CommitDelete`失敗時はエラーを返し、操作ジャーナルを次回復旧または再試行へ残す。
  - ファイル削除失敗、再起動後の復旧、UIの失敗表示をテストする。

- [ ] Notebook階層の循環を拒否する。
  - 自己参照だけでなく、子孫を親に指定する更新を拒否する。
  - 子・孫への移動、正常な別ツリーへの移動、再帰削除をテストする。

- [ ] Markdown本文欠落時にアプリ全体を利用不能にしない。
  - 欠落ノートを自動削除せず、正常なノートは利用できるdegraded状態を用意する。
  - 欠落ファイルの場所、復元、明示削除などの回復経路を提示する。
  - 1件欠落・複数件正常、復元後の再検査をテストする。

### Medium（次フェーズでDB変更を行う前に完了）

- [ ] migrationのバージョン境界を安全にする。
  - 現行コードより新しい`user_version`を明示的に拒否する。
  - migration失敗時のrollbackと、将来版DBの拒否をテストする。

- [ ] SQLite接続設定を全接続で保証する。
  - `foreign_keys`、`busy_timeout`、WALの適用方式を決める。
  - 再接続・並行接続時にも外部キー制約が有効であることをテストする。

- [ ] Critical/High項目をCIで自動検証する。
  - Goテスト、frontend typecheck、serializerテスト、競合テスト、Wailsビルドをクリーン環境で実行する。

## 次フェーズと並行して対応する項目

優先度の高い順に、対象機能の着手前または同時に対応する。

### High（対象機能の着手前）

- [ ] インポート・クラウド同期を開始する前に、raw HTMLをregex依存で処理しない安全な変換・サニタイズ方針を決める。
  - 複数行HTML、イベント属性、`javascript:`等の入力をテストする。

- [ ] クラウド同期・履歴・AIストリーミングを開始する前に、revision、競合検出、保存キューの仕様を確定する。

### Medium

- [ ] 全文検索を開始する前に、Markdown本文の索引方式を決める。
  - SQLite FTS5、再構築可能な専用索引、外部contentless indexを比較する。
  - Markdown外部変更時の索引更新・再構築を定義する。

- [ ] 完了済みMarkdownのhashまたはmtimeを検出し、外部編集・rename・deleteのreconciliation方針を決める。

- [ ] store/APIのエラーを共通通知へ接続し、batch操作の部分成功と未処理Promiseを整理する。

- [ ] `isSaving`を要求数または保存キューで管理し、並行保存中の表示を正確にする。

- [ ] Markdown↔Rich変換の空段落、code fence、URL、多重markを追加テストする。MVP外のfootnote等は仕様を維持する。

### Low

- [ ] autosave coordinatorを分離し、`NoteEditor`の責務を段階的に縮小する。全面分割は行わない。
- [ ] lint、formatter、構築手順、環境文書の実装との差分を整理する。
- [ ] 本文をログへ出さず、operation ID、note ID、処理段階、エラー分類だけを記録する。
- [ ] 大量ノート対応時に起動時全件読み込み、全件`Stat`、一覧ページングを見直す。
