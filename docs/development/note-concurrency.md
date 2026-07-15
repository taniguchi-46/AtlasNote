# ノートrevision・競合検出・保存キュー仕様

最終更新: 2026-07-13

## 目的

クラウド同期、履歴、AIストリーミングを開始する前に、staleな更新が新しい内容を上書きしないためのrevision、競合検出、ローカル保存キューの契約を確定する。

この文書はrevision / CAS、競合UI、ローカル保存キューの実装契約を定義する。将来の同期outboxは未実装である。

## 用語と責務

| 用語 | 意味 | 永続化 |
| --- | --- | --- |
| 永続revision | SQLiteに保存するノート単位のCASトークン | SQLite |
| draft version | フロントエンドで入力snapshotの新旧を判定する世代番号 | メモリのみ |
| operation ID | SQLite / Markdown操作ジャーナルとログを関連付ける識別子 | 操作完了までSQLite |
| sync version | WebDAVのETag、同期元hash、last-synced baseなど端末間同期に使う情報 | Phase 3設計で定義・実装前 |

- フロントエンドの入力snapshot世代は `draftVersion` として扱い、永続revisionとは分離する。
- 永続revisionは端末内で更新順序を判定する値であり、端末間の新旧比較には使用しない。
- operation IDは監査・復旧用であり、CASトークンには使用しない。

## 永続revision

### データモデル

- schema version 3 migrationで `notes` に `revision INTEGER NOT NULL DEFAULT 1` を追加済み。
- 新規ノートのrevisionは `1` とする。
- 既存ノートはmigration時にrevision `1`として扱う。
- `Note`、`Summary`、内部Record、更新・削除APIでrevisionを受け渡す。
- `id`は既に主キーであるため、revision専用INDEXは追加しない。

### revisionを増加させる操作

次の操作が成功したとき、対象ノートのrevisionを1増加させる。

- タイトル変更
- 本文変更
- ノートブック変更
- お気に入り変更
- ピン留め変更
- ゴミ箱への移動と復元
- ノートブック削除に伴うノートの移動またはゴミ箱への移動
- 外部Markdown変更を正本として受け入れるreconciliation

### 外部Markdown reconciliationの判定

- SHA-256 hashをMarkdown正本との比較値とする。mtimeは永続判定値にしない。
- `note_search_state.indexed_revision` が現行revisionと同じでhashだけ異なる場合だけ外部編集と判定し、CASでrevisionを1つ進める。
- 索引stateのrevisionが古い場合は索引だけを再構築し、誤ってrevisionを進めない。
- `<note-id>.md` の欠落は `MissingNotes` へ報告し、自動削除しない。DBにないMarkdownは `recovery/` へ隔離し、自動renameしない。
- 判定は起動時復旧と「再検査」操作で行い、復旧後に検索索引を再構築する。
- 将来の同期、履歴復元、AI出力の確定保存も、ローカル保存成功時はrevisionを増加させる対象とする。

同じ保存要求内でタイトルと本文を同時に変更しても、revisionの増加は1回とする。起動時の未完了操作復旧では、既に確定したrevisionを再度増加させない。

### CAS

- 更新と完全削除は `expectedRevision` を必須とする。
- Repositoryは `id` と `revision` を同じSQL条件で検査する。
- 更新成功時は同じSQLでrevisionを1増加させる。
- 更新件数が0件の場合は存在確認を行い、not foundとrevision conflictを区別する。
- stale要求ではSQLite、Markdown、一時ファイル、操作ジャーナル、`updated_at`を変更しない。
- Markdown確定失敗時の補償処理は、本文、メタデータ、`updated_at`、revisionを更新前の状態へ戻す。

概念上の更新条件は次のとおりとする。

```sql
UPDATE notes
SET ..., revision = revision + 1
WHERE id = ? AND revision = ?;
```

## 競合検出と応答

### 競合条件

次の場合をrevision conflictとする。

- APIの `expectedRevision` とSQLiteの現在revisionが一致しない。
- 編集開始後に外部Markdown変更が検出され、reconciliationでrevisionが進んだ。
- 編集開始後に同期、履歴復元、AI確定保存など別の更新元がrevisionを進めた。

### 競合時の原則

- staleな本文やメタデータを自動上書きしない。
- 本文競合を自動mergeしない。
- 同じ要求を新しいrevisionへ自動で付け替えて再試行しない。
- ローカルdraftを `conflicted` 状態で保持し、ユーザー操作なしに破棄しない。
- サーバー側の最新版は競合後に改めて取得し、ローカルdraftと分離して扱う。

初期UIで提供する解決手段は次の範囲に限定する。

- 最新版を読み直してローカルdraftを破棄する。
- ローカル内容をクリップボード等へ退避してから最新版を読み直す。
- ローカルdraftを保持したまま編集画面へ戻る。

自動merge、差分UI、「強制上書き」は同期機能の競合要件を確定するまで追加しない。

### APIエラー契約

- 競合には安定したエラーコード `NOTE_REVISION_CONFLICT` を使用する。
- 競合情報は `noteId`、`expectedRevision`、`actualRevision` を持つ。
- 競合応答へ本文、タイトル、ファイルパス、内部スタックを含めない。
- Wails v2のPromise rejectionはGo errorの文字列を返すため、エラーメッセージの部分一致を恒久契約にしない。
- revision conflictはGo errorへ変換せず、更新APIでは `UpdateNoteResult`、削除APIでは `DeleteNoteResult` の構造化結果として返す。
- 構造化結果は成功データまたは `RevisionConflict` のどちらか一方だけを持つ。DB障害、ファイルI/O失敗、入力不正、not foundは従来どおりGo errorとしてPromiseをrejectする。
- フロントエンドAPI clientは構造化結果を共通のdomain errorへ変換し、StoreやComponentがWails固有形式へ依存しないようにする。

診断ログは本文・タイトル・検索語・元のErrorオブジェクトを記録せず、operation ID（取得できる場合）、note ID、処理段階、エラー分類だけを記録する。

## ローカル保存キュー

### 基本方針

- 保存キューはノート単位のlaneとして管理する。
- 同じノートでは同時に1要求だけをin-flightとし、FIFOで処理する。
- 未開始の本文・タイトルautosaveは、最新snapshotだけを残すlatest-wins方式で集約する。
- 既にin-flightの要求は中断せず、完了後に最新snapshotを保存する。
- 別ノートのpending snapshotを上書きしない。

### 同じlaneを通す操作

次の操作は同じノートの保存laneを通し、アプリ自身の操作同士でrevision conflictを発生させない。

- 本文・タイトルのautosave
- お気に入り、ピン留め
- ゴミ箱への移動、復元
- ノートブック移動
- 完全削除

キュー内で先に成功したローカル操作のrevisionは、後続操作の既知revisionへ引き継ぐ。外部変更や同期によって進んだrevisionへdraftを自動rebaseしてはならない。

### キュー状態

状態は最低限、次のように分離する。

- `dirty`: debounce待ちで未投入
- `queued`: 保存待ち
- `saving`: in-flight
- `failed`: 通常の保存失敗
- `conflicted`: revision conflict

`isSaving`はbooleanを直接代入せず、in-flight要求数またはキュー状態から算出する。エディタの保存表示は全体状態ではなく、アクティブノートの状態を使用する。

### 失敗、再試行、終了処理

- 通常失敗または競合が発生したlaneは停止し、最新draftを保持する。
- 通常失敗の手動再試行は同じsnapshotを対象にできるが、競合は解決方法を選ぶまで再試行しない。
- `flush(noteId)` は対象ノートのin-flightとqueued snapshotを待つ。
- `flushAll()` は全ノートのin-flightとqueued snapshotを待つ。
- 完全削除前は対象draftをflushする。flush失敗時は、明示確認なしにdraftを破棄して削除しない。
- アプリ終了時は現在の終了前flush、再試行、破棄確認を維持する。
- fire-and-forgetで開始する保存Promiseにも必ず失敗処理を接続し、未処理Promiseを発生させない。

### 将来の同期キューとの分離

- ローカル保存キューはメモリ上でよい。
- WebDAVへ送信する変更は、クラッシュ後も再開できるSQLite上のdurable outboxとして別途設計する。
- ローカル保存成功とクラウド送信成功を同じ `isSaving` で表現しない。
- 同期状態は `pending / syncing / synced / conflict / failed` など別の状態として管理する。

## 履歴とAIストリーミング

### 履歴

- CAS revisionは保存のたびに増加させる。
- 履歴の保存単位や表示上の集約は別ポリシーとし、autosaveごとに履歴を表示することをこの仕様では要求しない。
- 履歴復元は現在revisionを `expectedRevision` として検査し、成功時に新しいrevisionを作る。

### AIストリーミング

- AI処理開始時のrevisionをbase revisionとして保持する。
- ストリーミング途中のchunkをノート正本へ逐次保存しない。
- 生成結果はdraftまたは一時状態へ蓄積し、確定時に1回のCAS保存を行う。
- 生成中にユーザー編集等でrevisionが進んだ場合は競合とし、自動上書きしない。

## 層ごとの責務

| 層 | 責務 |
| --- | --- |
| Vue Component | 保存・競合状態の表示、ユーザーによる解決操作 |
| Pinia / autosave coordinator | draft version、ノート単位queue、flush、状態遷移 |
| API client / Wails API | `expectedRevision`と競合情報の受け渡し |
| Service | 入力検証、外部変更確認、保存手順、補償処理の制御 |
| Repository | revisionを含む原子的CAS、not foundと競合の区別 |
| Markdown Storage | 一時ファイル、確定、rollback。revisionの判定は行わない |
| 将来のSync Service | ETag、last-synced base、durable outbox、端末間競合 |

## migrationとrollback

- revision追加はschema version 3 migrationとして実装済みである。
- migrationは既存ノートの本文、タイトル、日時、`content_path`を変更しない。
- 既存行はrevision `1`でbackfillする。
- migration失敗時はトランザクションをrollbackし、`PRAGMA user_version`を進めない。
- 現在はdown migration基盤がないため、旧アプリへ戻す場合はmigration前DBバックアップの復元をrollback手順とする。
- migrationでは既存データ保持、失敗rollback、新しいschema versionを旧アプリが拒否することをテストする。

## 実装時の受け入れ条件

- Create、Get、Listが永続revisionを返す。
- revision `1`の更新成功後にrevision `2`が返る。
- 同じexpected revisionによる2つの更新は、1件だけ成功し、もう1件は競合する。
- staleな本文更新でMarkdown、SQLite、操作ジャーナル、`updated_at`が変化しない。
- Markdown確定失敗時に以前のrevisionまで復元される。
- 起動時復旧でrevisionが二重加算されない。
- ノートブック削除に伴うノート更新でもrevisionが増加する。
- 同じノートのautosaveとメタデータ更新がqueue順に処理される。
- 別ノートのpending autosaveが失われない。
- 競合時にローカルdraftが保持され、自動再試行loopが発生しない。
- 重複要求の一方が完了しただけで `isSaving` がfalseにならない。
- `flushAll()` が全in-flightとqueued snapshotを待つ。
- 保存Promise rejectionが未処理にならない。

## 対象外

- WebDAV通信と認証
- 端末間の自動mergeアルゴリズム
- 履歴データモデルと保持期間
- AI APIのprovider、model、課金、ストリーム形式
- 同期用durable outboxのテーブル設計
- 全面的な `NoteEditor` 分割

## 実装順序（完了記録）

以下はPhase 2で実装した順序の記録です。Phase 3の同期outboxは [`webdav-sync.md`](webdav-sync.md) と [`implementation-plan.md`](implementation-plan.md) に従って別途実装します。

1. revision migration、モデル、RepositoryのCASを実装済み。
2. Serviceの更新・削除・補償・復旧へrevisionを接続済み。
3. Wails APIとフロントエンド型へ `expectedRevision` と競合情報を接続済み。
4. フロントエンドの入力snapshot世代を `draftVersion` として永続revisionから分離済み。
5. autosaveをノート単位queueへ拡張し、メタデータ更新も同じlaneへ接続済み。
6. `isSaving`をqueue状態または要求数から算出済み。
7. 最小の競合表示とdraft保持・破棄導線を追加済み。
8. 正常系、競合、補償、復旧、並行保存テストを追加済み。
9. 実装状態を `docs/status.md` と `docs/todo/todo-phese2.md` へ反映済み。
