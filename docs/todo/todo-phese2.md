# Phase 2 TODO

## TODOの目的

Phase 2「整理・検索」を、安全な最小差分で段階的に実装する。

機能要件は `docs/development/scopes/scope-phese2.md`、実装順序は `docs/development/implementation-plan.md` を正とする。

## 現状・前提

- MVPの移行前必須項目とCI確認は完了済み。
- 検索入力UIは実装済みだが、現在は入力値をログへ出すだけで実検索処理は未実装。
- ノートブック、お気に入り、ピン留め、ゴミ箱は実装済み。
- タグ、全文検索索引、バックリンク、関連メモのデータ構造は未実装。
- ノート本文はMarkdown、メタデータはSQLiteを正として扱う。
- DBスキーマ、migration、API、UIを変更する前に、対象機能の設計を確定する。

## 絶対遵守事項

- 既存のRepository / Service / Wails API / Piniaの責務境界を維持する。
- Markdown本文を正本とし、索引や関連情報は再構築可能にする。
- ユーザー入力をSQL文字列へ直接連結しない。
- 検索語、ノート本文、API Keyなどをログへ不用意に出さない。
- DB変更時は既存データへの影響、migration、rollback方法を先に記載する。
- 不要なライブラリ追加、大規模リファクタリング、関係ないUI変更を行わない。
- 各タスクは実装、テスト、ドキュメント更新まで完了してからチェックする。

## 0. Phase 2実装前チェック

- [ ] 現在のドキュメント整理差分をレビューし、削除・更新内容に問題がないことを確認する。
- [ ] `docs/development/scopes/scope-phese2.md`の対象機能と対象外機能を確認する。
- [ ] `docs/development/implementation-plan.md`の実装順序を確認し、最初に着手する機能を決める。
- [ ] 最初の実装対象をタイトル検索に限定するか判断する。
- [ ] 1機能ごとの変更範囲をRepository、Service、Wails API、Store、UIに分けて整理する。
- [ ] 検索方式、API、DB変更、テスト方針が未確定のまま機能実装へ進まないことを確認する。
- [ ] 現在の作業ツリーにPhase 2実装と無関係な変更がないことを確認する。

### 実装開始条件

- [ ] 最初に実装する機能と完了条件が決まっている。
- [ ] 検索方式の採用案、代替案、影響範囲、リスクを比較できている。
- [ ] APIの入力、出力、エラー形式が決まっている。
- [ ] DB変更がある場合、既存データへの影響、migration、rollback方法が決まっている。
- [ ] 正常系、異常系、境界値、セキュリティのテストケースが整理されている。

## 次フェーズと並行して対応する項目

以前のMVPレビューで定めた継続項目を、対応時期と判断理由を含めて引き継ぐ。

| 項目 | 対応時期 | 理由 | 放置リスク |
| --- | --- | --- | --- |
| 全文検索索引方式 | 全文検索着手時の最初 | 本文を`notes`へ重複保存する必要は確定していない | 毎回全Markdownを読む低速実装になる |
| 完了済みMarkdownのhash / mtime管理 | 検索・同期・履歴より前 | 現MVPの単体利用には必須でない | 外部変更を索引や同期へ反映できない |
| revision / CAS | 履歴・同期・AIストリーミング前 | 単一起動保証後のMVPでは延期可能 | stale updateを検出できない |
| raw HTML sanitization | インポート・同期より前 | 現在は自己入力中心 | 外部MarkdownがXSS経路になる |
| 共通エラー通知 | Phase 2初期 | 保存失敗以外のstore errorがUIへ未接続 | 部分成功・失敗が不明になる |
| `isSaving` request counter | 自動保存修正時または直後 | 現在のbooleanは並行要求に弱い | 誤った保存完了表示になる |
| serializer round-tripテスト | Rich機能追加と同時 | MVP外記法は明文化済み | 対応済み記法でも内容が変化する |
| 起動時全件`Stat`・一覧ページング | 大量ノート対応時 | MVP規模では許容できる | ノート増加時に起動が遅延する |
| ログ・可観測性 | 配布・同期開始前 | 現在はローカル開発中心 | ユーザー環境の障害を追跡できない |
| lint・文書更新 | Phase 2初期 | 現時点では実行時障害ではない | 未処理Promiseや環境差を見逃す |

対応時期がPhase 2より後の項目も削除せず、後続機能の着手条件として維持する。

### High（対象機能の着手前）

- [x] インポート・クラウド同期を開始する前に、raw HTMLをregex依存で処理しない安全な変換・サニタイズ方針を決める。
  - [x] 複数行HTMLをテストする。
  - [x] `onclick`等のイベント属性をテストする。
  - [x] `javascript:`等の危険なURLをテストする。
- [x] クラウド同期・履歴・AIストリーミングを開始する前に、revision、競合検出、保存キューの仕様を確定する。
  - [x] 永続revision、フロントのdraft version、operation ID、同期世代の責務を分離する。
  - [x] CAS対象、競合時のdraft保持、保存キューの順序とflushを定義する。
  - [x] ローカル保存キューと将来の同期用durable outboxを分離する。
  - [x] 詳細仕様を `docs/development/note-concurrency.md` に記録する。

### Medium

- [ ] 全文検索を開始する前に、Markdown本文の索引方式を決める。
  - [ ] SQLite FTS5、再構築可能な専用索引、外部contentless indexを比較する。
  - [ ] Markdown外部変更時の索引更新・再構築を定義する。
- [ ] 完了済みMarkdownのhashまたはmtimeを検出し、外部編集・rename・deleteのreconciliation方針を決める。
- [ ] store / APIのエラーを共通通知へ接続し、batch操作の部分成功と未処理Promiseを整理する。
- [x] `isSaving`を要求数または保存キューで管理し、並行保存中の表示を正確にする。
- [ ] Markdown / Rich変換の空段落、code fence、URL、多重markを追加テストする。
  - [ ] MVP外のfootnote、frontmatter、reference link、Markdownコメント、高度なHTML blockは未対応仕様を維持する。

### Low

- [ ] autosave coordinatorを分離し、`NoteEditor`の責務を段階的に縮小する。全面分割は行わない。
- [ ] lint、formatter、構築手順、環境文書と実装の差分を整理する。
- [ ] 本文をログへ出さず、operation ID、note ID、処理段階、エラー分類だけを記録する。
- [ ] 大量ノート対応時に起動時全件読み込み、全件`Stat`、一覧ページングを見直す。

## 1. 検索基盤の設計

- [ ] 現在のNote Repository / Service / Wails API / Storeの検索関連実装を確認する。
- [ ] SQLite FTS5、再構築可能な専用索引、外部contentless indexを比較する。
- [ ] タイトル検索と本文全文検索の責務境界を決める。
- [ ] Markdown外部変更時の索引更新・再構築方針を決める。
- [ ] 検索結果のページング、最大件数、並び順を決める。
- [ ] 検索APIのリクエスト、レスポンス、エラー形式を決める。
- [ ] 検索文字列の最大長、空白、特殊文字、Unicodeの扱いを決める。
- [ ] 採用方式、代替案、影響範囲、リスクを設計文書へ反映する。

### 完了条件

- [ ] DB/API変更前に設計レビュー可能な状態になっている。
- [ ] 索引が壊れた場合の再構築方法が定義されている。
- [ ] SQLインジェクション対策とログ方針が明文化されている。

## 2. タグ設計

- [ ] タグ名の正規化、最大長、空文字、大文字小文字の扱いを決める。
- [ ] 同名タグのUNIQUE制約を決める。
- [ ] タグ、ノート関連テーブルの主キー、外部キー、INDEXを設計する。
- [ ] タグ削除、ノート削除時の関連解除方法を決める。
- [ ] 既存データへの影響を確認する。
- [ ] migrationとrollback方法を設計する。
- [ ] Repository / Service / Wails APIの責務を決める。

### 完了条件

- [ ] DB制約と入力検証の責務が明確になっている。
- [ ] migration失敗時に既存DBを維持できる設計になっている。

## 3. 検索実装

- [ ] タイトル検索を実装する。
- [ ] 本文全文検索を実装する。
- [ ] 検索APIを追加する。
- [ ] 既存の検索UIを実検索処理へ接続する。
- [ ] 検索中、0件、失敗時の状態を表示する。
- [ ] 最新の検索要求だけを画面へ反映する。
- [ ] 空文字、長すぎる入力、特殊文字、Unicodeをテストする。
- [ ] SQLインジェクションを狙った入力を安全に処理できることをテストする。
- [ ] 索引の作成、更新、削除、再構築をテストする。

## 4. タグ実装

- [ ] タグの追加を実装する。
- [ ] タグの編集を実装する。
- [ ] タグの削除を実装する。
- [ ] ノートへのタグ付与を実装する。
- [ ] ノートからのタグ解除を実装する。
- [ ] タグ検索を実装する。
- [ ] 同名、空文字、最大長超過の入力検証を実装する。
- [ ] 権限不要なローカル機能であることを維持し、外部通信を追加しない。
- [ ] migration、外部キー、削除時の参照整合性をテストする。

## 5. フィルター実装

- [ ] ノートブックフィルターを実装する。
- [ ] タグフィルターを実装する。
- [ ] 作成日フィルターを実装する。
- [ ] 更新日フィルターを実装する。
- [ ] 検索条件とフィルターを併用できるようにする。
- [ ] フィルター解除を実装する。
- [ ] 不正な日付範囲を拒否する。
- [ ] 条件の組み合わせと0件表示をテストする。

## 6. 並び替え・最近開いたメモ

- [ ] 対応する並び替え項目と方向を決める。
- [ ] 並び替え項目を許可リストで制限する。
- [ ] 並び替えを実装する。
- [ ] 「最近開いた」の記録タイミングと保持件数を決める。
- [ ] 最近開いたメモを実装する。
- [ ] 削除済み・ゴミ箱・復元済みノートの扱いをテストする。
- [ ] ドラッグドロップでノーブック移動を実装する。

## 7. バックリンク・関連メモ

- [ ] ノートリンクの記法を決める。
- [ ] バックリンクの抽出規則を決める。
- [ ] 存在しないリンク先、タイトル変更、ノート削除時の扱いを決める。
- [ ] バックリンクを実装する。
- [ ] 関連メモの判定基準を決める。
- [ ] 関連メモを実装する。
- [ ] 循環リンク、大量リンク、不正なリンク記法をテストする。

## 8. テーブルコピー

- [ ] コピー対象をセル、行、列、表全体のどこまでにするか決める。
- [ ] Markdown形式とプレーンテキスト形式の出力方針を決める。
- [ ] Markdownを壊さないテーブルコピーを実装する。
- [ ] Markdown / Rich往復後も表の内容が保持されることをテストする。
- [ ] 空セル、改行、インライン装飾、特殊文字をテストする。

## 9. 継続する品質課題

- [ ] Markdownのhashまたはmtimeを使った外部編集・rename・deleteの検知方針を決める。
- [ ] store / APIエラーを共通通知へ接続する。
- [ ] batch操作の部分成功と未処理Promiseを整理する。
- [x] `isSaving`を要求数または保存キューで管理する。
- [ ] Markdown / Rich変換の空段落、code fence、URL、多重markをテストする。
- [ ] operation ID、note ID、処理段階、エラー分類を使ったログへ整理する。
- [ ] 大量ノート時の起動、検索、一覧表示を計測する。
- [ ] 履歴・同期・AIストリーミング着手前にrevision / CASと競合検出を実装する。
  - [x] schema version 3で `notes.revision` を追加し、既存行をrevision `1`へbackfillする。
  - [x] Note / Summary / Recordと既存Repository・Serviceの入出力へrevisionを追加する。
  - [x] `expectedRevision`、構造化競合結果モデルを追加する。
  - [x] Repositoryへ原子的な更新・削除CASを追加する。
  - [x] Serviceの通常更新・完全削除をCAS経路へ切り替える。
  - [x] ノートブック削除に伴うノート更新でrevisionを増加する。
  - [x] Wails APIとStoreから `expectedRevision` を受け渡す。
  - [x] Wails APIへ構造化競合応答を接続する。
  - [x] ノート単位保存キューと競合時のdraft保持を実装する。
    - [x] Storeに `conflicted` 状態と競合情報を保持する。
    - [x] 競合後のローカル下書きを破棄せず、自動再試行を停止する。
    - [x] `NoteEditor` に競合と下書き保持中の状態を表示する。
    - [x] 競合draftを破棄してサーバー最新版を再読み込む操作を追加する。
    - [x] 競合draftを新規ノートへコピー保存する操作を追加する。
    - [x] ノート単位の保存キューを実装する。
    - [x] autosave・メタデータ更新・削除を同じノート操作laneへ統合する。
    - [x] ノート操作laneと `isSaving` 要求カウンターの専用テストを追加する。
- [x] インポート・同期着手前にraw HTMLをregexだけに依存せず処理するsanitization方針を決める。
- [ ] Rich機能追加時にserializer round-tripテストを追加する。
- [ ] 大量ノート対応時に起動時全件`Stat`と一覧ページングを見直す。
- [ ] Phase 2初期にlint対象、未処理Promise、環境文書と実装の差分を整理する。
- [ ] autosave coordinatorを段階的に分離し、`NoteEditor`の責務を縮小する。

## 10. ドキュメント・最終確認

- [ ] `docs/status.md`を実装状態へ更新する。
- [ ] `docs/rules/architecture.md`へ確定した検索・タグ設計を反映する。
- [ ] `docs/development/scopes/scope-phese2.md`と実装の差分を確認する。
- [ ] API、DB、migration、テストの変更内容を記録する。
- [ ] 不要なTODO、古い前提、リンク切れがないことを確認する。

## Phase 2完了条件

- [ ] 対象機能の正常系、異常系、境界値テストが成功する。
- [ ] 検索、フィルター、並び替えの併用結果が一貫する。
- [ ] SQLインジェクションやログからの情報漏えいがない。
- [ ] 既存の保存、編集、ノートブック機能を壊していない。
- [ ] migrationとrollbackを確認できる。
- [ ] 関連ドキュメントが実装状態と一致する。
- [ ] CIの全チェックが成功する。

## 確認コマンド

```bash
go test ./...
npm run frontend:typecheck
npm run frontend:lint
npm run frontend:build
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:serializer
wails build
```

Phase 2の機能ごとに、Repository、Service、Wails API、Store、UIの対象テストを追加する。
