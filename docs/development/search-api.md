# 検索API・ページング・入力検証設計

最終更新: 2026-07-14

## 目的と対象範囲

MVPの検索はWails APIの `SearchNotes` として提供し、Markdownのタイトル・本文をSQLite FTS5索引から検索する。索引方式は [`search-index.md`](search-index.md) を正とする。

検索ではノートブックとtrashの条件を指定できる。タグ遷移は通常一覧専用で、全文検索条件とは併用しない。バックリンクは別タスクで扱う。

## Wails API契約

Goのメソッドは次の形とする。

```go
SearchNotes(input note.SearchInput) (note.SearchResult, error)
```

### request

```text
SearchInput {
  Query          string   // 検索文字列
  Scope          string   // "all" | "title"
  NotebookID     *string  // 省略時は全ノートブック
  IncludeTrashed bool     // 既定false
  Page           int      // 1からのページ番号
  PageSize       int      // 既定30、最大100
}
```

- `Scope="all"` はタイトルと本文を検索する。
- `Scope="title"` は `notes.title` のparameterized `LIKE` を使用する。
- `NotebookID` は存在しないIDをゼロ件とし、バリデーションエラーにはしない。
- `IncludeTrashed=false` でtrashを除外する。検索結果からの誤操作を防ぐため、既定は現行アクティブノートと合わせる。

### response

```text
SearchResult {
  Items    []SearchItem
  Page     int
  PageSize int
  Total    int
  HasNext  bool
  Error    *SearchError
}

SearchItem {
  Note       Summary
  Snippet    string
  MatchScope string // "title" | "body" | "both"
}

SearchError {
  Code      string
  Message   string
  Field     string
  Retryable bool
}
```

- 成功時は `Error=nil` とする。
- 既知の入力・索引状態エラーは `Error` で返し、Go errorは `nil` とする。Wailsのエラー文字列の解析に依存させない。
- 予期しないDB障害、ファイルI/O、プログラム内部エラーはGo errorでrejectする。内部詳細はレスポンスに含めず、ログにも本文や検索語を出さない。
- `Snippet` は最大240文字、マークダウン原文を基に生成し、UIではHTMLとして直接挿入しない。

## ページングと並び順

- offset方式、1-basedの `Page` を使用する。MVPではカーソル方式を導入しない。
- `PageSize` の既定値は30、許可範囲は1〜100。
- `Page` の既定値は1、許可範囲は1〜10000。
- `Total` はフィルター後の総件数を返す。`HasNext` は `Page*PageSize < Total` で判定する。
- `Total` 用のcount queryとデータqueryは同じ検索条件・trash条件・notebook条件を使用する。
- `Scope="all"` の並び順はrelevance降順、`updated_at DESC`、`id ASC`の順とする。
- `Scope="title"` は `updated_at DESC`、`id ASC`の順とする。
- 同一スコアで同点の結果の順序を `id` で固定し、ページ間のフラップを防ぐ。

## 入力バリデーション

- 入力の前後のUnicode空白をtrimする。
- trim後の空文字列はエラーにせず、空の `SearchResult` を返す。検索入力の初期状態として扱いやすくするためである。
- クエリはtrim後200 Unicode文字まで。超過時は `SEARCH_QUERY_TOO_LONG` 。
- NULとUnicodeの制御文字は拒否し、 `SEARCH_QUERY_INVALID` を返す。改行・タブは空白区切りとして許可する。
- `Scope` は `all` または `title` のみ。それ以外は `SEARCH_SCOPE_INVALID` 。
- `Page` は1以上、10000以下。`PageSize` は1以上、100以下。不正値は `SEARCH_PAGE_INVALID` または `SEARCH_PAGE_SIZE_INVALID` 。
- 文字列をSQLやFTS5の検索式へ連結しない。通常検索はクエリをリテラルのフレーズへエスケープし、複数語はANDで結合する。全てparameter bindingを使用する。

## エラーコード

| Code | 種別 | Retryable | 用途 |
| --- | --- | --- | --- |
| `SEARCH_QUERY_TOO_LONG` | validation | false | クエリが200文字超過 |
| `SEARCH_QUERY_INVALID` | validation | false | NUL・制御文字を含む |
| `SEARCH_SCOPE_INVALID` | validation | false | scopeが未定義 |
| `SEARCH_PAGE_INVALID` | validation | false | pageが範囲外 |
| `SEARCH_PAGE_SIZE_INVALID` | validation | false | pageSizeが範囲外 |
| `SEARCH_INDEX_NOT_READY` | state | true | migration・再構築中で検索不可 |
| `SEARCH_INDEX_INCONSISTENT` | state | true | 索引と正本の不整合を検出 |
| `SEARCH_INDEX_FAILED` | internal | true | 索引更新・検索の内部失敗 |

`Message` はUI向けの安全な日本語とし、SQL文、ファイルパス、本文、スタックを含めない。フロントAPIクライアントは `Code` に応じて型付きドメインエラーへ変換する。

## 検索要求の競合対策

- サーバー側の結果に、要求のIDは持たせない。フロントの `createLatestRequestGuard` でリクエストを管理する。
- 新しい検索入力が発生したら、古い応答を画面に反映しない。
- キャンセルでバックエンドのDB処理を必ず中断する契約にしない。応答を受け取った時点で最新判定を行う。

## 必須受け入れテスト

- 日本語・ASCII・改行・記号・Markdownコードフェンスの検索
- タイトルのみ、本文のみ、全体のマッチ
- 空文字列、201文字、NUL、制御文字、不正scope・page・pageSize
- 初期値、最大値、境界ページ、ゼロ件、同点ソート
- trash・notebook条件と全文検索の併用
- 索引未準備、不整合、再構築中のエラーコード
- 連続検索の古い応答が画面を上書きしないこと
- SQL injection入力、FTS5特殊構文、検索語・本文をログに出さないこと
