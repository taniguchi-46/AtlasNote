# タグ設計

最終更新: 2026-07-13

## ステータス

タグのデータモデル、制約、migration、Repository、Service、Wails API、ノートへのタグ付与・解除、タグ一覧クリックによるノート一覧遷移、ノート編集画面のタグ選択ポップアップを実装済みです。タグ名の候補検索とノート検索APIへのタグ条件は廃止しています。

## 目的と対象範囲

Phase 2で、タグの作成・編集・削除とノートへの付与・解除を追加する。ノート本文は引き続きMarkdownを正本とし、タグと関連はSQLiteのメタデータとして保持する。

この設計は次を対象とする。

- タグ名の正規化、入力検証、重複防止
- タグとノートの多対多関連
- タグ一覧クリックによる単一タグのノート一覧遷移に必要なINDEX
- ノートへの複数タグ付与と、既存タグを選択して追加するUI
- タグ・ノート削除時の参照整合性
- schema version 6へのmigrationとrollback
- Repository / Service / Wails APIの責務境界

Markdown内のタグ記法の解析、WebDAV同期、タグ履歴、自動タグ生成、ユーザー向けのタグ名検索、タグ条件を全文検索へ統合する検索、複数タグのAND/OR検索は対象外とする。

## タグ名

### 正規化と検証

タグ作成・更新時はServiceで次の順に処理する。

1. NULまたはUnicode制御文字を含む入力を拒否する。
2. Unicode NFCへ正規化する。
3. Unicode空白を半角スペースへ変換し、連続する空白を1つに縮約する。
4. 前後空白を除去する。
5. 空文字を拒否し、正規化後の長さをUnicode文字数で100文字以下に制限する。
6. 表示用の`name`からUnicode case-fold済みの`normalized_name`を生成する。

`name`には正規化後の表示名を保存する。大文字小文字は表示上保持するが、`normalized_name`によって比較するため、`Go`と`go`は同じタグとして扱う。日本語など大文字小文字を持たない文字はそのまま比較される。

NFKCは使わない。`①`と`1`、全角英数字と半角英数字のように、見た目が近くても意味が異なる可能性のある文字を自動的に同一視しないためである。

`#`は特別な構文として扱わず、入力された文字列の一部として保存する。Markdownタグ解析を追加する場合は、別途記法と移行方針を決める。

### UNIQUE制約

`tags.normalized_name`へ`UNIQUE`制約を付ける。SQLiteの`NOCASE`はUnicodeのcase-foldやNFCを保証しないため、比較規則をSQLite照合順序に依存させない。

- 新規作成で同じ`normalized_name`が存在する場合は`TAG_NAME_CONFLICT`を返す。
- 別タグと衝突しない限り、表示上の大文字小文字だけを変更する更新は許可する。
- `normalized_name`はAPIへ公開せず、ServiceとRepository内部の比較用データとする。

## データモデル

### テーブル

```sql
CREATE TABLE tags (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
	normalized_name TEXT NOT NULL UNIQUE CHECK(length(normalized_name) > 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE note_tags (
	note_id TEXT NOT NULL,
	tag_id TEXT NOT NULL,
	PRIMARY KEY (note_id, tag_id),
	FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
	FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX idx_note_tags_tag_id_note_id
	ON note_tags(tag_id, note_id);
```

| 対象 | 決定 | 理由 |
| --- | --- | --- |
| `tags.id` | `TEXT PRIMARY KEY` | 既存のノート・ノートブックIDと同じ方針。Serviceが既存の`newID`で生成する。 |
| `tags.name` | 表示名、`NOT NULL`、1〜100文字 | UI表示と入力上限をDBでも防御する。 |
| `tags.normalized_name` | 非公開の比較キー、`NOT NULL UNIQUE` | 大文字小文字・NFC差異を含む同名タグを防ぐ。 |
| `created_at` / `updated_at` | UTCのRFC3339Nano文字列 | 現在の`notes`・`notebooks`と統一する。 |
| `note_tags` | `(note_id, tag_id)`複合主キー | 同一ノートへの同一タグの重複を防ぎ、ノートごとのタグ取得を支える。 |
| `note_tags.created_at` | 追加しない | 現時点で関連の順序・履歴は要件外であり、最小スキーマを維持する。 |

`tags.normalized_name`のUNIQUE INDEXと`note_tags`の複合主キーINDEXにより、追加の名前INDEXや`note_id`単体INDEXは不要である。逆引きINDEXの`idx_note_tags_tag_id_note_id`は、タグクリックによるノート一覧遷移と外部キーCASCADEの走査を支える。

`note_tags`の複合主キーは同一ノートへの同一タグの重複だけを防ぐため、異なる`tag_id`を持つ関連を複数保存できる。したがって、1つのノートは0個以上のタグを持てる。

タグ一覧は`normalized_name ASC, id ASC`で安定して並べる。タグクリックによる一覧遷移では、`ListNotesPage`に単一の`tagId`を渡し、パラメータ化した`EXISTS`条件を使用する。JOINによるノート行の重複を発生させず、`SearchNotes`へタグ条件を追加しない。

## タグによるノート一覧遷移

タグは検索フィルターではなく、第2のノートブックとして扱う。サイドバーのタグ名をクリックすると、そのタグを持つノートだけを一覧表示する。

- 選択できるタグは常に1件だけとし、複数タグのAND/OR検索は提供しない。
- タグ選択とノートブック選択は同時に保持しない。ノートブック、通常一覧、全文検索へ遷移したときは、選択中のタグを解除する。
- 選択中のタグが削除されたときは、タグ選択を解除して通常一覧へ戻す。
- タグ名を検索する入力UIは提供しない。
- 選択中のタグがノートへ付与・解除されたときは、現在の一覧を再読み込みして表示内容を同期する。
- Repositoryは`ListPage`で`note_tags`を`EXISTS`する単一`tagId`条件を扱い、件数取得とページングにも同じ条件を適用する。

## ノートへの複数タグ付与とUI

ノートには複数のタグを付与できる。付与済みタグはノート編集画面の文字数表示左にある追加アイコンの右隣へチップ表示し、各チップから既存どおり解除できる。

タグ追加は、文字数表示の左に置くアイコンボタンから行う。ボタンを押すとポップアップを表示し、次の操作だけを提供する。

1. 未付与の既存タグだけを表示するドロップダウンリストから、追加するタグを1件選ぶ。
2. `キャンセル`でポップアップを閉じ、選択を破棄する。
3. `追加`で選択したタグを既存の`SetNoteTags`経由でノートへ付与する。未選択時、候補がないとき、更新中は追加できない。

このポップアップではタグ名の入力、候補検索、新規タグ作成を行わない。新規タグの作成はサイドバーの`＋`ボタンだけで行う。ゴミ箱内ノートでは、タグの追加・解除を提供しない。

## 削除と整合性

| 操作 | `note_tags`の扱い | ノート・Markdownへの影響 |
| --- | --- | --- |
| タグ削除 | `tags`削除時のCASCADEで関連を解除する | ノートは削除・更新しない。 |
| ノートの完全削除 | `notes`削除時のCASCADEで関連を解除する | 既存のMarkdown削除・操作ジャーナル手順をそのまま使う。 |
| ゴミ箱へ移動 | 関連を保持する | 復元後もタグを維持する。 |
| ゴミ箱から復元 | 関連を保持する | タグ再作成や再付与は不要。 |

SQLite接続では既に`foreign_keys(ON)`を設定しているため、外部キー違反とCASCADEをDBで保証する。

タグ付与・解除、タグ名編集、タグ削除では`notes.updated_at`と`notes.revision`を変更しない。現在の`revision`は本文と`notes`自身の属性を保護するCASトークンであり、タグの一括変更によって本文編集中のノートを競合状態にしないためである。タグ操作はMarkdown、`note_storage_operations`、FTS5索引も変更しない。

Phase 3同期では`notes.revision`を流用せず、tag entityとnote-tags集合を別の同期単位としてdurable outbox、last-synced base、競合状態を管理する。

## APIと責務

### 公開モデルとWails API

`Tag`は`id`、`name`、`createdAt`、`updatedAt`だけを公開する。`normalized_name`は公開しない。タグ一覧をノート一覧へN+1で埋め込まないため、初期実装では`Note`・`Summary`へ`tags`フィールドを追加せず、専用APIで取得する。

```go
ListTags() ([]note.Tag, error)
ListNoteTags(noteID string) (note.NoteTagsResult, error)
CreateTag(input note.TagCreateInput) (note.TagMutationResult, error)
UpdateTag(tagID string, input note.TagUpdateInput) (note.TagMutationResult, error)
DeleteTag(tagID string) (note.TagDeleteResult, error)
SetNoteTags(noteID string, input note.SetNoteTagsInput) (note.NoteTagsResult, error)
```

`SetNoteTags`は`tagIDs`全体を受け取り、ノートに付くタグの最終状態を1トランザクションで置換する。空配列は全解除を意味する。入力の重複IDは除去し、存在しないタグIDまたはノートIDは成功として扱わない。個別の付与・解除APIは公開しないため、途中状態や複数API呼び出しによる不整合を作らない。

タグ一覧遷移では既存の`ListNotesPage(input note.NoteListInput)`に任意の`tagId`を渡す。`tagId`は一覧遷移だけに使い、`SearchNotes`の入力・検索条件には追加しない。タグ追加ポップアップは既存の`ListTags`、`ListNoteTags`、`SetNoteTags`を再利用するため、新しいタグ操作APIは増やしていない。

既知の入力・重複・参照先エラーは、Wailsのエラー文字列解析に依存せず、`TagError`を含む構造化結果で返す。想定コードは次のとおりとする。

- `TAG_NAME_EMPTY`
- `TAG_NAME_TOO_LONG`
- `TAG_NAME_INVALID`
- `TAG_NAME_CONFLICT`
- `TAG_NOT_FOUND`
- `TAG_NOTE_NOT_FOUND`

DB障害、予期しないI/O障害、内部エラーはGo errorとして返す。`TagError`のメッセージにはSQL、ファイルパス、ノート本文、入力されたタグ名を含めない。

### レイヤーごとの責務

| 層 | 責務 |
| --- | --- |
| Repository | Squirrelとparameter bindingによるSQL、タグCRUD、関連の取得・置換、`ListPage`の単一`tagId`条件、外部キー前提のトランザクション。Markdown、UI文言、正規化は扱わない。 |
| Service | タグ名の正規化・検証、ID・時刻生成、同名判定、構造化エラーへの変換、`SetNoteTags`の全件存在確認と原子的置換、タグ指定一覧の入力受け渡し。既存のService mutexでノート完全削除との同一プロセス内競合を直列化する。 |
| Wails API | 初期化済みServiceへの委譲と構造化結果の受け渡し。`ListNotesPage`へ任意の`tagId`を渡すが、SQL・正規化・UI状態を持たない。 |
| フロントAPI / Pinia | Wails結果を型付きエラーへ変換し、タグ一覧・選択ノートのタグ状態・単一の選択タグIDを管理する。コンポーネントはWails APIを直接呼ばない。 |

ゴミ箱内ノートのタグは保持する。UIではゴミ箱内の編集導線を表示しないが、Serviceは存在するノートの関連を不必要に破壊しない。

## migrationとrollback

### migration

- schema version 5の後ろへ、上記2テーブルとINDEXを作成するmigration version 6を追加した。
- `migrations`と`ensureCompatibleSchema`の両方へ同じタグDDLを反映し、既存の自己修復パターンに合わせた。
- 既存の`migrate`は1トランザクション内でDDLを実行し、成功時だけ`PRAGMA user_version = 6`を確定する。この方式を維持している。
- 既存の`notes`、Markdown、`revision`、日時、FTS5索引、検索状態を更新・backfillしない。追加直後の`tags`と`note_tags`は空であることをテストで確認した。
- 今回の一覧遷移・ノート編集UIの仕様変更は、既存の`note_tags`と`idx_note_tags_tag_id_note_id`を再利用する。schema versionの更新、新しいmigration、rollback手順の追加は不要である。

### rollback

- migration途中の失敗はトランザクションrollbackにより、テーブル作成と`user_version`更新の両方を取り消す。
- 適用済みversion 6を旧アプリへ戻すためのdown migrationは追加しない。タグデータを失う`DROP TABLE`や`user_version`だけの書き換えは禁止する。
- 旧アプリへの復帰は、アプリ停止中に取得したmigration前のデータディレクトリ全体のバックアップを復元する。WAL運用のため、DBファイルだけを任意に差し替えない。
- 自動バックアップ・復元はPhase 2の対象外である。

## 既存データへの影響

ワークスペース内には実行時SQLite DBが存在しない。実データは`ATLAS_NOTE_DATA_DIR`、未指定時は`os.UserConfigDir()/AtlasNote`配下に置かれるため、実利用データの件数とバックアップ有無は未確認である。

空テーブル追加だけであり、既存ノート・Markdown本文・ノートブック・検索索引にデータ変換は発生しない。`TestOpenMigratesVersionFiveDatabaseWithoutChangingExistingNote`でversion 5 DBをfixtureにし、既存レコードが不変であることを検証している。

## 実装・検証状況

- `internal/note/tag_service_test.go`で、空文字、NUL・制御文字、100/101文字、Unicode空白、NFC差異、大文字小文字、同名作成・改名を検証する。
- 同テストで、重複IDの除去、未知タグ時の原子的な全体維持、全解除、ゴミ箱移動・復元後の関連保持、タグ付けで`notes.revision`と`updated_at`を変更しないことを検証する。
- `internal/database/database_test.go`で、version 5 DBからのmigration、migration失敗時のrollback、将来version DBの拒否、タグ・ノート完全削除時の外部キーCASCADEを検証する。
- `app_test.go`で、Wails公開APIからタグの構造化入力エラーを返すことを検証する。
- RepositoryはSquirrelのparameter bindingを使い、タグ名・IDをSQL文字列へ連結しない。タグ操作用の本文・タグ名ログは追加していない。
- `npm --prefix frontend run typecheck`、`npm --prefix frontend run test:tags`、`wails build`で、生成bindingsを含むフロントエンドの型整合性、タグStoreの回帰、ビルドを確認する。実機での手動UI操作確認は別途行う。
- `internal/note/tag_service_test.go`で、単一`tagId`指定時の一覧件数・ページング・ゴミ箱除外と、空の`tagId`入力を検証する。
- フロントエンドでは、タグStoreの複数タグ付与・重複付与防止、型検査、本番ビルド、Wails bindingsを含むアプリビルドを確認した。タグ切替時の古い要求の非反映、候補除外、キャンセル、追加失敗の目視確認は別途行う。

## 実装結果と後続範囲

1. schema version 6、databaseテスト、Tagモデル、Repository、Service、Wails APIを実装した。
2. 生成bindings、フロントAPI、Pinia Storeを接続した。
3. ノート編集画面のタグ付与・解除、サイドバーのタグ作成・改名・削除を実装した。
4. タグ候補検索・タグ名検索・検索APIへのタグ条件を廃止し、タグクリックによる単一タグの一覧遷移と、文字数表示左のアイコンから開くタグ選択ポップアップへ置き換えた。
