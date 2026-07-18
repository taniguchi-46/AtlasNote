# 実装計画

最終更新: 2026-07-15

## 目的

Phase 2の完了記録を維持しながら、Phase 3「同期」を設計承認後に段階的に実装します。

機能要件は `docs/development/scopes/scope.md`、現在状況は `docs/status.md`、Phase 3の同期設計は `docs/development/webdav-sync.md` を正とします。

本書は承認済み設計に従った実装順序の正本です。進捗チェックは `docs/todo/todo-phese3.md`、Phase 2の実績と残課題は `docs/todo/todo-phese2.md` を参照します。

## 現在のPhase

Phase 3はschema version 10、Joplin方式の同期設定、フェイルセーフ、安全な復旧操作を含む実装とローカル自動検証まで完了しています（2026-07-15）。実WebDAVサーバー相互運用、手動UI受け入れ、CI上の最終確認を残課題として扱います。

## Phase 3実装順序

### 0. 同期設計の確定

実装状況: 設計レビュー完了、コア実装完了（2026-07-15）

- `docs/development/webdav-sync.md` で、同期対象、change set、`head`/manifest/objectのリモート配置、vault識別、初回同期、tombstone、strong ETag、hash、last-synced baseを確定済み。
- Markdown正本、SQLiteメタデータ、FTS5・ノートリンク索引、`revision`、操作journal、ローカル保存laneの責務境界を維持する。
- durable outbox、同期状態、manifest commit、競合状態、15秒・60秒・5分の最大3回retry、オフライン、部分成功、終了前flushの契約を確定済み。
- `CredentialStore`、同期設定タブ、HTTPS/Basic、明示的なHTTP許可トグル、endpoint検証、timeout、ログ・エラーの秘密情報非露出を確定済み。
- migration、rollback、既存データへの影響、Repository / Service / Wails API / フロントAPI / Store / UIの変更範囲とテストケースは、実装ステップで具体化し、受け入れ条件に沿って検証する。

### 1. 同期データモデルとmigration

- sync connection、item state、durable outbox、conflict、snapshotの最小スキーマをschema version 8、HTTP許可設定をversion 9、同期間隔・フェイルセーフ・TLS・proxy設定をversion 10として追加済み。
- version 9の`auto_sync`を300秒/0秒へbackfillし、許可間隔とproxy timeoutのCHECK制約、フェイルセーフ既定ONをmigration testで確認済み。
- 既存のschema version 7、notes、notebooks、tags、Markdown、revision、索引を変更しない。
- 既存のmigrationトランザクション、rollback、旧アプリの新version拒否を既存のdatabaseテストと合わせて検証済み。

### 2. WebDAVクライアントと認証境界

- `PROPFIND` Depth 0/1、`MKCOL`、`GET`、`PUT`、strong ETag、`If-Match`/`If-None-Match`、timeout、retry可能エラー、認証失敗、レスポンス検証を実装済み。
- 読み取り専用の設定確認、custom root CA、明示的TLS error ignore、HTTP/HTTPS proxy、redirect拒否をGo標準libraryだけで実装済み。
- endpointとremote pathを検証し、ユーザー入力や認証ヘッダーをログへ出さない。
- `go-keyring` adapterによるCredentialStore、OS secure store unavailable時のセッション限定fallback、平文設定を永続保存しない境界を実装済み。

### 3. Sync Serviceとdurable outbox

- ローカル保存成功と同一SQLiteトランザクションでchange setをoutboxへ記録し、不変object・manifest作成後のhead更新を検証できた場合だけ完了にする処理を実装済み。
- pull・3-way比較・manifest commit・retry・部分成功・tombstone・競合保存をentity単位で処理する実装を追加済み。
- 受信変更は既存Service / Repository / Markdown Storageを通し、CAS、操作journal、補償処理、単一writer、派生索引更新を維持する。

### 4. Wails API、フロントAPI、Store、UI

- 単一WebDAV URL、同期間隔、設定確認、詳細設定、フェイルセーフ、復旧操作をJoplinと同じ項目・draft方式で既存の責務境界へ接続済み。
- 「適用」は保存して継続、「OK」は保存して閉じる、「戻る」・閉じるは未保存draftを破棄する。runtime statusは`idle`へ強制せず「待機中」「同期済み」等へローカライズする。
- 同期開始前に対象draftをflushし、同期送信状態を既存の`isSaving`と混同しない。
- ComponentからWails APIを直接呼ばず、API clientとPinia / Composableを経由する。

### 5. 自動同期と受け入れ検証

- ローカル保存後5秒のdebounce、選択間隔poll、CredentialStore取得後の起動同期、最大3回retry、手動retry、オフライン状態を実装済み。アプリ終了時は既存のローカルflushと永続化を優先する。
- 空remoteフェイルセーフ、条件付き再アップロード、別領域へstageする再ダウンロード、起動時backup付きswapとrollbackを実装済み。
- 正常系、強いETag、outbox/base snapshot、TLS/proxy、資格情報境界、フェイルセーフ、412、復旧rollback、同期ストア、既存回帰テスト、別名出力のWails本番build、秘密情報非露出の自動検証を完了。実サーバー相互運用と実環境での手動復旧は受け入れ確認として残す。
- 手動同期、自動同期、空状態、失敗状態、競合解決のUIを手動確認する（残課題）。

## 開発方針

- Markdownをノート本文の正とする。
- 既存のRepository / Service / Wails API / Piniaの責務境界を維持する。
- 未確定の方式やDB構造を実装で先に固定せず、比較と影響確認を行ってから決定する。
- DB変更時は既存データへの影響、migration、rollback方法を先に明文化する。
- UIは既存の3ペイン構成を維持し、Phase 3に必要な範囲だけ変更する。
- 追加ライブラリは既存技術で実現できないことを確認してから検討する。
- ユーザー入力は検証し、SQLはパラメータ化されたRepository経由で実行する。

## Phase 2実装順序（完了記録・履歴）

以下はPhase 2の実績を残すための履歴です。現在の実装状態と未確認事項は [`status.md`](../status.md) と [`todo-phese2.md`](../todo/todo-phese2.md) を参照します。

### 0. revision・競合・保存キュー

実装状況: 完了（2026-07-12）

- 確定仕様は `docs/development/note-concurrency.md` を正とする。
- ノート単位の整数revisionと `expectedRevision` によるCASを実装する。
- stale更新ではMarkdown、SQLite、操作ジャーナル、`updated_at`を変更しない。
- autosaveとメタデータ更新を同じノート単位queueで直列化する。
- ローカル保存キューと同期用durable outboxを分離する。
- 仕様確定と実装完了は別管理とし、クラウド同期・履歴・AIストリーミング着手前に実装と競合テストまで完了する。

### 1. 検索基盤の設計

索引方式: contentful SQLite FTS5 + trigramに確定（2026-07-12）。API・ページング・入力仕様も確定し実装済み。

- タイトル検索と本文全文検索の責務境界を決める。
- SQLite FTS5、再構築可能な専用索引、外部索引を比較する。
- Markdown外部変更時の索引更新・再構築方針を実装する。
- 検索APIの入力、出力、エラー、ページング方針を実装済みの契約へ反映する。

### 2. タグ実装

実装状況: 完了（2026-07-13）。確定仕様は `docs/development/tag-design.md` を正とする。

- タグ名の正規化、長さ、Unicode case-foldによる重複防止、削除時の扱いを実装する。
- `tags` / `note_tags`の多対多関連、主キー・外部キー・UNIQUE・逆引きINDEXを実装する。
- schema version 6 migration、transaction rollback、既存version 5 DBの不変性テストを追加する。
- Repository / Service / Wails API / フロントAPI / Pinia / UIの責務境界で、タグ作成・編集・削除・付与・解除を実装する。

### 3. 検索とタグ遷移

実装状況: 検索とタグ遷移は完了（2026-07-14）。検索APIではノートブック・日付による絞り込みを対象外とし、最近更新した一覧の日付条件は通常一覧APIで扱う。

- 既存検索UIを実検索処理へ接続する。
- タイトル・本文検索を実装済み。タグ条件は通常一覧の単一タグ遷移として実装し、全文検索へは追加しない。
- 検索とタグ遷移の組み合わせをテスト済み。並び替えとの組み合わせも実装・検証済み。

### 4. メモ管理

実装状況: 並び替え、最近更新した一覧、ノートブックのドラッグ＆ドロップ移動は完了（2026-07-14）。

- 並び替え項目は `updatedAt`、`createdAt`、`title`、方向は `asc` / `desc` に限定する。未指定時は通常一覧を更新日時の新しい順、全文検索を関連度順とする。
- 「最近更新した」はノートの `updated_at` を記録タイミングとし、アプリのローカル日付で当日00:00〜翌日00:00未満に更新されたアクティブノートを表示する。別の履歴テーブルや追加migrationは使用しない。
- ノートブック行のドラッグ＆ドロップで子ノートブックまたはルートへ移動し、自己・子孫への移動は拒否する。
- ノートリンクの記法・バックリンクの抽出規則を決めて実装する（完了）。

### 5. エディタ改善

実装状況: 完了（2026-07-14）。

- コピー対象はMarkdown / Richともにカーソル位置の表全体に統一し、セル・行・列単位の独自コピーは対象外とする。
- Markdownモードは表の元ソース、Richモードは表ノードから生成したMarkdownをコピーする。
- `text/plain` には同じMarkdownを、Rich貼り付け用の`text/html`には表構造を出力する。`ClipboardItem`にはWebView2が扱える標準MIME型だけを渡す。
- RichコピーでClipboardItemの書き込みに失敗した場合は、Markdownだけへ黙ってフォールバックせず、エラー通知と本文を含まない失敗ログを残す。Markdown→Rich / Rich→Rich貼り付け時も表構造を保持する。
- Markdownセルの改行、インライン装飾、区切り文字、特殊文字をエスケープし、Markdown / Rich往復とClipboard出力をテストする。

### 6. 品質課題

- 外部編集の検知とreconciliationを実装済み。
- APIエラーの共通通知とbatch操作の部分成功・Promise処理は実装済み。
- 並行保存中の表示は実装済み。構造化ログも本文非記録で実装済み。大量ノート一覧は固定上限付きページングと追加読込を実装済み。
- 大量ノート時の検索・起動復旧（全件読み込み）はベンチマークで計測可能にし、mtime一致時の索引再利用と変更時hash照合フォールバックを実装済み。5,000件基準値を記録し、今後は継続比較する。起動復旧の存在確認は管理ファイル一覧の一括取得へ置き換え済み。

## セキュリティ・整合性

- 検索文字列、タグ名、並び替え項目を検証する。
- 動的な並び替え列や検索条件を許可リストで制限する。
- ノート本文、秘密情報、検索語をログへ不用意に出さない。
- 外部Markdownのraw HTMLはRichエディタ変換時にHTMLとして解釈せず、危険な属性・URLをDOMへ生成しない。
- migration失敗時に既存DBを利用不能にしない。
- 索引は再構築可能にし、Markdown本文の正本性を維持する。

## 確認コマンド

```bash
go test ./...
npm run frontend:typecheck
npm run frontend:lint
npm run frontend:build
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-operation-queue
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:notifications
npm --prefix frontend run test:tags
npm --prefix frontend run test:serializer
npm --prefix frontend run test:operation-logger
npm --prefix frontend run test:note-links
npm --prefix frontend run test:note-list-view
npm --prefix frontend run test:table-copy
npm --prefix frontend run test:markdown-safety
wails build
```

機能追加時は、対象機能の異常系・境界値・競合テストを追加します。

## Phase 3の確認方針

設計文書の更新時は、リンク先、用語、対象外、未確定事項、migration・rollback記述の整合性を確認し、`git diff --check`を実行します。実装開始後は、実際のpackage scriptsとGoテストを基に、次の確認を行います。

```bash
go test ./...
npm run frontend:typecheck
npm run frontend:lint
npm run frontend:build
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-operation-queue
npm --prefix frontend run test:sync
wails build
```
