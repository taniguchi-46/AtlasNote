# WebDAV同期設計

最終更新: 2026-07-15

## ステータス

Phase 3の設計レビュー済み・実装前です。本書の同期対象、リモート形式、認証、outbox、競合、自動同期の契約を確定しました。WebDAV通信、同期用migration、認証情報の保存処理はまだ実装しません。

本書はPhase 3同期契約の正本です。実装順序は [`implementation-plan.md`](implementation-plan.md)、進捗は [`todo-phese3.md`](../todo/todo-phese3.md) を参照します。

## 1. 目的と前提

Phase 3では、複数端末のAtlas Note間でノートと管理情報をWebDAV経由で同期する。ローカルファーストを維持し、WebDAVが利用できない場合もローカルの作成・編集・検索を継続できることを前提とする。

既存の次の仕様を正とする。

- Markdown本文はローカルノートの正本であり、SQLiteのFTS5索引とノートリンク索引は再構築可能な派生データである。
- `revision` と `expectedRevision` は同一端末内のCAS専用であり、端末間の新旧比較には使用しない。
- MarkdownとSQLiteの保存は `note_storage_operations` と一時ファイルを使う既存の復旧手順を維持する。
- フロントエンドのautosave・ノート操作laneと、同期用durable outboxは分離する。
- 競合した本文を自動mergeしたり、ユーザーの確認なしに強制上書きしたりしない。

関連する確定仕様は `docs/development/note-concurrency.md`、`docs/development/search-index.md`、`docs/development/tag-design.md` とする。

## 2. 同期対象と正本

Markdownだけを送受信すると、SQLiteに保存しているタイトル、ノートブック、タグ、お気に入り、ピン、ゴミ箱状態が失われる。そのため、同期対象をentityのpayloadとして扱い、検索・リンクなどの派生データは送受信しない。

| entity | ローカルの正本 | 同期対象 | 競合単位 |
| --- | --- | --- | --- |
| note | Markdown本文 + `notes` | 本文、title、notebook、favorite、pinned、trashed、作成・更新日時 | 本文とノート属性を含むノートaggregate全体 |
| notebook | `notebooks` | ID、親ID、名前、アイコン、作成・更新日時 | ノートブック1件 |
| tag | `tags` | ID、表示名、正規化名、作成・更新日時 | タグ1件 |
| note-tags | `note_tags` | ノートごとのtag ID集合 | ノート1件の関係集合 |
| FTS5検索索引 | `note_search` | 対象外 | 受信後に再構築 |
| ノートリンク索引 | `note_links` | 対象外 | 受信後に再抽出 |
| `notes.revision` | SQLite | 対象外 | 受信適用時にローカルCASとして生成 |
| `note_storage_operations` | SQLite | 対象外 | 端末内の保存復旧用であり、リモートへ送らない |

ノートブック削除など複数entityに影響する操作は、削除コマンドとして送らない。ローカル操作後の結果として変更された全entityをひとつの同期change setへ記録する。これにより、子ノートブックの削除、ノートのtrash・切り離しなどの副作用を別端末でも同じ結果として適用できる。

同期対象に添付ファイル、履歴、関連メモ、AI生成結果は含めない。これらは別Phaseまたは別設計とする。

## 3. リモート配置と識別子

WebDAVのルート配下にAtlas Note専用ディレクトリを作り、SQLiteファイルやWALファイルをそのまま配置しない。v1の論理配置は次のとおりとする。

```text
.atlasnote/
  format.json
  head.json
  manifests/<manifest-sha256>.json
  objects/<object-sha256>.json
```

- `format.json` は `formatVersion` とvault識別子を持つ不変リソースである。vault識別子は既存のID形式に合わせた`crypto/rand`由来の128-bit lowercase hexとする。
- `head.json` は現在のmanifest hashと世代を指す唯一の可変リソースであり、強いETagを使った`If-Match`で更新する。
- manifestはentity keyとobject hashの一覧を持つ不変JSONで、entity keyと一覧順を正規化してhashを計算する。
- objectはentityのactive payloadまたはdeleted tombstoneを持つ不変JSONで、ノート本文とノート属性は同じnote objectに含める。本文と属性を別PUTにして中間状態を公開しない。
- objectとmanifestの作成は`If-None-Match: *`で行う。既存hashが返った場合は取得してhashと内容を検証し、再利用する。
- パスは安定IDまたはSHA-256から生成し、ユーザー入力やタイトルを直接使用しない。
- permanent deleteは同じentity keyのdeleted objectをmanifestから参照するtombstoneで表現する。trashはnoteの通常属性であり、tombstoneではない。
- tombstoneと過去object・manifestはv1では自動削除しない。これによりオフライン端末の復帰時に削除が更新を復活させることを防ぐ。

リモートの可視状態は`head.json`から辿れるmanifestとobjectだけである。object・manifestのアップロード途中にクラッシュしても、`head.json`が更新されるまで他端末から見えない。

初回同期の選択は次のとおりとする。

1. `.atlasnote/format.json`がない場合は、ユーザーが「新規リモートを初期化」を明示したときだけvault IDを生成し、空manifestとheadを条件付きで作成する。
2. 既存リモートへ接続する場合、既存のsync connectionがあればvault IDが一致する場合だけ再接続する。
3. sync connectionがないローカルから既存リモートを取り込めるのは、同期対象のローカルデータと未送信outboxが空の場合だけとする。ローカルデータとの自動mergeや自動破棄は行わない。
4. 未知のformat version、欠落・不正なformat、別vaultはエラーとして停止し、上書きや自動修復を行わない。

## 4. 同期状態とdurable outbox

### 4.1 同期状態

端末ごとに次の情報をSQLiteへ保存する。具体的な列型はschema version 8のmigration設計で確定する。

| 状態 | 役割 |
| --- | --- |
| connection | endpoint、remote root、vault ID、head ETag、最後の同期時刻、接続状態、自動同期設定、CredentialStore参照ID |
| item state | entity key、ローカルobject hash、last-synced baseのobject hash、現在remote object hash、本文hash・メタデータhash、解決状態 |
| outbox | change set ID、entity object hash、base manifest hash、base head ETag、操作順序、試行回数、次回試行時刻、失敗分類 |
| conflict | entity key、local/base/remoteのobject hashとスナップショット参照、競合種別、解決状態 |

`last-synced base`は端末間比較専用のmanifest/object情報であり、ローカル`revision`と混在させない。本文hashとメタデータhashは診断用に分けて保持するが、noteの競合判定はaggregate単位で行う。

### 4.2 Outboxのライフサイクル

1. ローカル変更を既存のMarkdown/SQLite保存手順で確定する。
2. 同じSQLiteトランザクションで、変更された全entityのpayload hashとchange set IDをoutboxへ記録する。ノートブック削除などの副作用も同じchange setに含める。
3. Sync Serviceはpending outboxをまとめ、immutable objectとmanifestを`If-None-Match: *`で作成してから、headを現在の強いETagに対する`If-Match`で更新する。
4. headを再取得し、manifest hashとobject内容を検証できた場合だけoutboxを完了にする。応答を失った場合も再取得で送信済みを判定できる。
5. head更新が`412 Precondition Failed`になった場合は同じPUTをblind retryせず、remote headを取得してentity単位の3-way比較を行う。
6. 通信断、timeout、5xx、429は15秒、60秒、5分の最大3回まで再試行し、その後はfailedで停止する。認証失敗、形式不一致、権限不足、競合は自動retryしない。

同一entityのpending outboxはFIFOを維持し、同一プロセスでhead更新を並列化しない。未完了のobjectやmanifestは孤児として残っても次回のhash検証で再利用でき、データ正本には影響しない。送信成功をローカルの`isSaving`と同じ状態で表現しない。

## 5. 同期フローと状態遷移

同期開始前にフロントエンドが対象draftをflushし、失敗または競合しているdraftを暗黙に破棄しない。アプリ起動時は既存の復旧処理を完了してから同期を開始する。

```text
disabled ──設定完了──▶ idle
idle ──ローカル変更──▶ pending
pending ──開始──▶ syncing
syncing ──全件成功──▶ synced
syncing ──通信断──▶ offline
syncing ──再試行上限──▶ failed
syncing ──双方変更──▶ conflict
offline / failed ──手動再試行──▶ pending
auth-required ──再認証──▶ idle
```

- 自動同期は初期値OFFとし、設定後に有効化する。
- 保存成功後5秒のdebounceで自動同期を開始し、アプリ実行中は5分間隔でremote headをpollする。
- 起動時の自動同期は、復旧完了・自動同期ON・CredentialStoreから認証情報を取得できる場合だけ一度実行する。セッション限定資格情報の場合は`auth-required`として再入力を求める。
- 同期は常に1件だけ実行し、実行中の追加変更は完了後の1回にまとめる。UIをブロックしない。
- 手動同期はbackoffをリセットして実行し、現在の状態、成功件数、失敗件数、競合件数を表示する。
- 部分成功時はheadに反映済みのentityを再送せず、未完了・競合entityだけを次回対象にする。
- アプリ終了時はネットワーク同期を無期限に待たず、ローカル終了前flushとoutboxの永続化を優先する。

## 6. 競合検出と解決

last-synced base、ローカルobject、remote objectをentity keyごとに比較する。

| 判定 | 動作 |
| --- | --- |
| local = base、remote変更 | remote objectを既存Service経由で取り込む |
| remote = base、local変更 | local objectを含むmanifestを条件付きhead更新する |
| local = remote | 同期済みとしてoutboxを完了する |
| local・remoteがともにbaseから変更 | conflictとして保存し、自動上書きしない |
| activeとdeletedが双方で変更 | delete勝ち・更新勝ちを自動決定せずconflictとする |

競合単位は次のとおりとする。

- noteは本文、title、notebook、favorite、pinned、trashedを含むaggregate全体で競合させる。本文だけ・属性だけの自動mergeは行わない。
- notebookとtagは各entity単位で競合させる。
- note-tagsはノートごとのtag ID集合を1 entityとし、集合の自動unionは行わない。
- tagの正規化名衝突、notebookの循環・不正な親参照、依存entityの欠落は受信競合または形式エラーとして保存し、適用を止める。
- notebook削除のローカル副作用はchange setに含まれる全entityで適用し、受信側でローカルの削除モードを再実行しない。

競合時はlocal、remote、baseのスナップショットを失わない。初期UIの解決操作は、ローカル採用、リモート採用、ノートに限った両方保持（反対側を新規IDへ退避）とし、明示確認を要求する。「最新版」のような時刻だけの自動判定、自動merge、無確認の強制上書き、revisionの自動rebaseは提供しない。

受信したchange setはSync Serviceから既存のService / Repository / Markdown Storageを通して適用する。直接SQLiteやMarkdownを上書きせず、既存のCAS、操作journal、補償処理、単一writerを維持する。ローカル適用が完了してからbaseとsync stateを更新し、適用後にFTS5とノートリンク索引を再構築する。

## 7. 認証・入力検証・ログ

- 設定画面に独立した「同期」タブを追加する。トップバーの同期ボタンは未設定時にこのタブを開き、設定済みなら手動同期を開始する。
- `CredentialStore`境界を設け、初期実装は`github.com/zalando/go-keyring`のadapterを使用する。WindowsはCredential Manager、macOSはKeychain、LinuxはSecret Serviceを使用し、OS側の安全な保存先が利用できない場合はセッション限定とする。
- パスワードの保存はユーザーの明示同意時だけ行う。endpoint、remote root、ユーザー名、自動同期設定はsync stateへ保存するが、パスワードをSQLite、`localStorage`、`.env`へ保存しない。
- endpointはHTTPS URLとして検証し、URL userinfo、認証情報、危険な相対pathを受け付けない。証明書検証を無効化せず、別originへのredirectへAuthorizationを転送しない。
- Phase 3 v1の認証はHTTPS上のHTTP Basicのみとし、Digest、OAuth、クライアント証明書は対象外とする。
- WebDAVクライアントは`PROPFIND`のDepth 0/1、`MKCOL`、`GET`、`PUT`、強いETag、`If-Match`、`If-None-Match`を必須能力とする。`LOCK`、`MOVE`、remote側の物理`DELETE`、無限depthは使用しない。ETagがない、弱い、条件付き更新を検証できないサーバーは接続を拒否する。
- endpoint、ユーザー名、パスワード、Authorizationヘッダー、本文、JSON payloadはログへ出さない。
- 利用者向けエラーは認証失敗、権限不足、timeout、通信断、競合、形式エラー、secure store unavailableを区別するが、秘密情報や内部パスを含めない。
- リクエスト単位のtimeoutを設定し、無制限の接続待ち・retry loopを作らない。既存の`.env.example`にある`WEBDAV_*`は実行時設定契約ではない。

条件付き更新、PROPFIND、ETag、Basic over TLSの根拠は [RFC 9110 §13.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1)、[RFC 4918 §7.2・§8.6・§9.1](https://www.rfc-editor.org/rfc/rfc4918.html#section-7.2)、[RFC 7617 §1・§4](https://www.rfc-editor.org/rfc/rfc7617.html#section-4)を参照する。

## 8. レイヤーごとの責務

| 層 | Phase 3での責務 |
| --- | --- |
| Vue Component | 同期設定、資格情報保存同意、手動同期、状態、部分成功、競合解決の表示。Wails APIを直接呼ばない |
| Pinia / Composable | draft flush、同期状態、重複開始防止、5秒debounce、通知、競合操作 |
| API client / Wails API | 型付き入力、状態、結果、構造化エラーの受け渡し |
| Sync Service | change set作成、pull・3-way比較・manifest commit・retry・競合判定・受信適用の制御 |
| WebDAV client | HTTPS/Basic、PROPFIND/MKCOL/GET/PUT、ETag、条件付き更新、timeout、レスポンス検証 |
| CredentialStore | OS secure storeとの読み書き、セッション限定fallback、資格情報の消去 |
| Repository | sync state、outbox、conflict、snapshotのSQLite操作。SQLはSquirrelとparameter bindingを使用 |
| Note Service / Markdown Storage | 既存のCAS、2フェーズ保存、操作journal、recovery、派生索引更新 |

ローカル保存成功とクラウド送信成功を同じ保存状態や通知へ混ぜない。単一writer保証はプロセス内のデータディレクトリを守り、ETag・manifest・base・outboxは端末間同期を守る別の境界とする。

## 9. migration、既存データ、rollback

- 同期状態・outbox・conflict・snapshot用テーブルは、現在のschema version 7とは別のschema version 8 migrationとして追加する。
- migrationでは既存のnotes、notebooks、tags、Markdown本文、revision、検索・リンク索引を変更しない。新規テーブルは空で作成し、パスワード列を持たせない。
- migrationは既存のトランザクション方式と`PRAGMA user_version`の更新規則を維持する。失敗時は新規テーブルとversion更新の両方をrollbackする。
- down migrationは追加せず、旧アプリへ戻す場合はアプリ停止中に取得したデータディレクトリ全体のバックアップを復元する。
- 既存データを実環境へ適用せず、破棄可能なfixture DBで既存行不変、制約、index、rollback、旧アプリの新version拒否を検証する。

## 10. 実装前の受け入れ条件

- `format`、`head`、manifest、object、tombstone、vault ID、初回同期の選択が文書化されている。
- note aggregate、notebook、tag、note-tagsの競合単位と、複合操作のchange set境界が決まっている。
- strong ETag、`If-Match`/`If-None-Match`、base manifest、outbox順序、最大3回retry、孤児objectの扱いが決まっている。
- CredentialStoreのOS別挙動、セッション限定fallback、HTTPS/Basic、endpoint検証、秘密情報非露出が決まっている。
- ローカル保存lane、操作journal、CAS、単一writer、派生索引との責務境界が壊れていない。
- 正常系、初期化競合、認証失敗、secure store unavailable、timeout、オフライン、部分成功、412競合、削除競合、循環・正規化制約、クラッシュ復旧のテストケースがある。
- 手動同期、自動同期、auth-required、空状態、失敗状態、再試行状態、競合解決のUI契約がある。

実装後は次を実行する。

```bash
go test ./...
npm run frontend:typecheck
npm run frontend:lint
npm run frontend:build
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-operation-queue
wails build
```

## 11. 対象外

- Google Drive、OneDrive、Dropbox、Git Repositoryとの同期
- 添付ファイル、履歴、関連メモ、AI生成結果の同期
- SQLite DB、FTS5内部shadow table、ノートリンク索引の直接同期
- 自動merge、無確認の強制上書き、無期限のバックグラウンドretry
- WebDAV認証情報を平文ログ、`localStorage`、SQLite、`.env`へ保存すること
- Digest、OAuth、クライアント証明書、WebDAV LOCK、remote物理DELETE、remote compact

## 12. 決定事項

設計レビューで次を決定し、Phase 3実装の前提とする。

1. 認証情報は独立した「同期」設定タブから設定し、`CredentialStore`（Windows Credential Manager、macOS Keychain、Linux Secret Service）へ明示同意時だけ保存する。secure storeが使えない場合はセッション限定とし、平文fallbackを作らない。
2. 初回同期は「新規リモートを初期化」「既存リモートを空のローカルへ取り込む」「既存connectionへ再接続」を明示操作に分ける。vault IDは128-bit random hexで生成し、別vault・未知形式・非空ローカルとの自動mergeを拒否する。
3. tombstoneと過去object・manifestはv1で無期限保持し、自動compact・自動GCは提供しない。ゴミ箱状態と完全削除を分離する。
4. 競合単位はnote aggregate、notebook、tag、note-tags集合とし、自動merge・自動union・時刻による最新版判定を行わない。解決はローカル採用、リモート採用、ノートの両方保持に限定する。
5. 自動同期は初期値OFF、保存後5秒debounce、5分poll、最大3回retryとする。起動時は復旧後かつ資格情報取得済みの場合だけ実行し、1プロセス1同期を守る。
6. リモートの途中状態を防ぐため、entity objectとmanifestを不変にし、強いETagを持つ`head.json`だけを条件付き更新する。WebDAV LOCKには依存せず、HTTPS、Basic、PROPFIND Depth 0/1、MKCOL、GET、PUT、ETag条件付き更新をv1の対応範囲とする。
