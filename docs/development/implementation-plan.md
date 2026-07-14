# 実装計画

最終更新: 2026-07-14

## 目的

MVPで構築した保存・編集基盤を維持しながら、Phase 2「整理・検索」を段階的に実装します。

機能要件は `docs/development/scopes/scope.md`、現在状況は `docs/status.md` を正とします。

## 開発方針

- Markdownをノート本文の正とする。
- 既存のRepository / Service / Wails API / Piniaの責務境界を維持する。
- 検索方式やDB構造を実装で先に固定せず、比較と影響確認を行ってから決定する。
- DB変更時は既存データへの影響、migration、rollback方法を先に明文化する。
- UIは既存の3ペイン構成を維持し、Phase 2に必要な範囲だけ変更する。
- 追加ライブラリは既存技術で実現できないことを確認してから検討する。
- ユーザー入力は検証し、SQLはパラメータ化されたRepository経由で実行する。

## 実装順序

### 0. revision・競合・保存キュー

実装状況: 完了（2026-07-12）

- 確定仕様は `docs/development/note-concurrency.md` を正とする。
- ノート単位の整数revisionと `expectedRevision` によるCASを実装する。
- stale更新ではMarkdown、SQLite、操作ジャーナル、`updated_at`を変更しない。
- autosaveとメタデータ更新を同じノート単位queueで直列化する。
- ローカル保存キューと将来の同期用durable outboxを分離する。
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
- バックリンクの抽出規則を決めて実装する。
- 関連メモの判定基準を決めて実装する。

### 5. エディタ改善

実装状況: 完了（2026-07-14）。

- コピー対象はMarkdown / Richともにカーソル位置の表全体に統一し、セル・行・列単位の独自コピーは対象外とする。
- Markdownモードは表の元ソース、Richモードは表ノードから生成したMarkdownをコピーする。
- `text/plain` には同じMarkdownを、Rich貼り付け用の`text/html`には表構造を出力する。`ClipboardItem`にはWebView2が扱える標準MIME型だけを渡す。
- RichコピーでClipboardItemの書き込みに失敗した場合は、Markdownだけへ黙ってフォールバックせず、エラー通知と本文を含まない失敗ログを残す。Markdown→Rich / Rich→Rich貼り付け時も表構造を保持する。
- Markdownセルの改行、インライン装飾、区切り文字、特殊文字をエスケープし、Markdown / Rich往復とClipboard出力をテストする。

### 6. 品質課題

- 外部編集の検知とreconciliationを実装する。
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
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:serializer
npm --prefix frontend run test:table-copy
npm --prefix frontend run test:operation-logger
wails build
```

機能追加時は、対象機能の異常系・境界値・競合テストを追加します。
