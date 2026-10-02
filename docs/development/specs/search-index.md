# Markdown全文検索索引設計

最終更新: 2026-07-13

## 結論

Markdown本文の全文検索には、SQLite FTS5のcontentfulな仮想テーブルを再構築可能な派生索引として使用する。tokenizerは日本語の部分一致を優先し、`trigram` を採用する。

- Markdownファイルを本文の正本とする。
- `notes` テーブルへ本文カラムは追加しない。
- FTS5内のタイトル・本文は、破棄・再構築できる派生データとする。
- 現在の `modernc.org/sqlite v1.53.0` はSQLite 3.53.2を含み、FTS5とtrigram tokenizerが有効であることを依存ソースで確認済みである。

SQLite FTS5とtokenizerの仕様は[SQLite FTS5 Extension](https://www.sqlite.org/fts5.html)を正とする。

## 現状

- SQLiteの `notes` はメタデータと `content_path` だけを保持する。
- Markdown本文は `notes/<note-id>.md` に保存される。
- Repository / Service / Wails API、検索Store/UIまで接続済みである。
- 検索語や本文をログへ出力しない。

## 比較

| 方式 | 長所 | 短所 | 判定 |
| --- | --- | --- | --- |
| FTS5 contentful索引 | SQLite内で完結し、BM25、column weight、highlight / snippetを利用できる。trigramで日本語部分一致に対応できる | FTSの内部content tableにタイトル・本文の派生コピーを持つ | 採用 |
| 独自の再構築可能索引 | 日本語のN-gramやスコアを完全に制御できる | token生成、位置、圧縮、ランキング、差分更新を自前実装する必要がある | MVPでは不採用 |
| FTS5 external-content | `notes` を外部content tableにできる | 本文はSQLiteではなくMarkdownにあり、FTS5が必要時に本文を参照できない | 不採用 |
| FTS5 contentless / contentless-delete | 本文コピーを保持せず索引サイズを抑えられる | column値を取得できず、snippet生成に別途Markdown読み込みが必要。trigramは3文字未満のフルテキストクエリに使えるtokenを生成しない | 現時点では不採用 |
| SQLite外部の検索エンジン | 高度な日本語解析や大規模検索を実現できる | 配布物、プロセス、障害復旧、データディレクトリの管理が増える | 大規模ノート計測後に再検討 |

external-contentとcontentlessの制約は[FTS5 External Content and Contentless Tables](https://www.sqlite.org/fts5.html#external_content_and_contentless_tables)を参照する。

## 実装スキーマ

schema version 4 migrationで専用仮想テーブルと索引状態テーブルを追加済みである。schema version 5 migrationでは索引状態へ `content_mtime_ns` を追加済みである。

```sql
CREATE VIRTUAL TABLE note_search USING fts5(
    note_id UNINDEXED,
    title,
    body,
    tokenize = 'trigram'
);
```

- `note_id` は `notes.id` と結合する識別子で、検索tokenにしない。
- `title` はSQLiteの `notes.title` から複製する派生値である。
- `body` はMarkdown正本から複製する派生値である。
- タイトルと本文のランキング重みは検索APIの実装仕様に従う。
- FTS5の内部shadow tableをRepositoryから直接操作しない。

## タイトルと本文の責務境界

- 通常の検索ボックスはFTS5の `title` と `body` を対象にする。
- 「タイトルのみ」の明示フィルターは `notes.title` のparameterized `LIKE` を使用し、Markdownを読まない。
- 全文検索の候補note IDは `notes` にjoinし、trashとnotebookのフィルターを通常テーブル側で適用する。タグと日付の条件は全文検索では扱わず、通常一覧APIで適用する。
- 本文は保存済みMarkdown文字列をそのまま索引化する。初期実装でMarkdown ASTやHTMLへ変換せず、code fence、URL、タスク文字列も検索対象に保つ。
- 将来、表示テキストだけの検索が必要にった場合は、索引バージョンを上げて全件再構築する。

## 短い検索語

trigram tokenizerのフルテキストクエリは、3 Unicode文字以上を基本とする。1〜2文字の検索は、contentful FTSテーブルのparameterized `LIKE` による全件走査を許容する。

- 1〜2文字検索はノート数が少ないMVP用のフォールバックとする。
- 性能計測で不足する場合は、最小検索長または独自bigram索引を再検討する。
- ユーザー入力をSQL文やFTS5クエリ文字列へ連結せず、必ずparameter bindingを使用する。

trigramのクエリ特性は[FTS5 Trigram Tokenizer](https://www.sqlite.org/fts5.html#the_trigram_tokenizer)を参照する。

## 更新タイミング

FTS索引はMarkdown正本とSQLiteメタデータが確定した後に更新する。FTS更新を先に確定しない。

| 操作 | 索引処理 |
| --- | --- |
| Create | Markdown commitとNote record保存成功後にinsert |
| タイトル・本文更新 | CASとMarkdown commit成功後にreplace |
| お気に入り・pin・trash・ノートブック移動 | タイトル・本文が不変なら索引本体は更新しない |
| 完全削除 | Note recordとMarkdown削除成功後にdelete |
| 復旧処理 | 復旧完了後に対象noteをreplace。判定できない場合は再構築対象 |
| 外部Markdown変更 | reconciliationでhash差分を検出し、revision更新後にreplace |

## 整合性と失敗時の扱い

- ノート保存の成功を、派生索引の更新失敗でrollbackしない。
- 索引更新失敗は本文を含めず、operation ID、note ID、処理段階、エラー分類だけを記録する。
- 検索は索引の不整合を検知した場合、不完全な結果を正常結果として扱わず、共通エラーと再構築導線へ接続する。
- FTS索引の整合性判定用に、note ID、indexed revision、content hash、Markdownのmtimeを持つ状態テーブルを検索migrationで追加済みである。mtime一致時は本文読み込みと再構築を省略し、mtime差分時にhashを正本との比較へ使用する。

## 再構築

- 再構築は `notes` とMarkdown正本から全FTS行を作り直す。
- ノート数と本文量に応じてbatch化し、UI threadをblockしない。
- 再構築中の検索可否、中断・再開、進捗表示は実装計画で確定する。
- FTS5の `integrity-check` を索引の自己検査に使用する。`optimize` は定期実行せず、大量更新後または明示メンテナンスに限定する。

FTS5の特別コマンドは[FTS5 Special INSERT Commands](https://www.sqlite.org/fts5.html#special_insert_commands)を参照する。

## migrationとrollback

- schema version 4 migrationでFTS5仮想テーブルと索引状態テーブルを追加済みである。
- schema version 5 migrationで既存の索引状態へ `content_mtime_ns = 0` を追加済みである。
- 初回復旧でhash照合とmtime保存を行う。
- migrationで既存のMarkdown、`notes`行、revision、日時を変更しない。
- migration失敗時はトランザクションをrollbackし、`PRAGMA user_version` を進めない。
- 現在はdown migration基盤がないため、旧アプリへ戻す場合はmigration前DBバックアップの復元をrollback手順とする。

## 実装・回帰テスト

- FTS5とtrigram tokenizerをアプリのSQLite driverで作成できる。
- 日本語、ASCII、絵文字、記号、改行、Markdown記法を検索できる。
- 1〜2文字のparameterized `LIKE` フォールバックを確認する。
- Create / Update / Delete / recovery / reconciliationで索引が一貫する。
- 索引更新失敗時もMarkdown正本とNote recordが失われない。
- 索引破損・欠落・過剰行を検出し、全件再構築で復旧できる。
- 検索語をSQLへ連結せず、FTS5構文記号とSQLインジェクション入力を安全に扱う。

## 再検討条件

- 大量ノートでFTS内部contentのディスク使用量が問題になる。
- 1〜2文字検索の全件走査が許容時間を超える。
- 日本語の形態素検索、同義語、かな・漢字正規化が必要になる。
- これらを計測後、contentless FTS5、独自bigram索引、SQLite外部エンジンを再比較する。

## 外部Markdown reconciliation

- Markdown本文のSHA-256 hashを正本比較に使う。mtimeはOS差・コピー処理による変動があるため、永続的な判定値にはしない。
- `note_search_state.indexed_revision` が現在のノートrevisionと同じで、`content_hash`だけが異なる場合は外部編集と判定する。Markdownを正本として受け入れ、ノートrevisionをCASで1つ進め、更新日時を更新してから索引を再構築する。
- 索引stateのrevisionが現在のrevisionより古い場合、外部編集とは断定せず、revisionを進めずに現在のMarkdownから索引を再構築する。これは索引更新失敗からの復旧を誤って競合へ変換しないためである。
- stateが存在しない場合もrevisionを変更せず、全件再構築で補完する。
- `<note-id>.md` がない場合は自動削除せず `RecoveryReport.MissingNotes` へ報告する。明示的なDeleteMissing操作でのみDBレコードを削除する。
- DBにないMarkdown（rename後のファイルを含む）は自動的に別ノートへ紐付けず、`recovery/`へ隔離する。内容の推測による誤結合を避けるためである。
