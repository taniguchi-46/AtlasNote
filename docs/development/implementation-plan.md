# 実装計画

最終更新: 2026-07-13

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

### 2. タグ設計

- タグ名の正規化、長さ、重複、削除時の扱いを決める。
- ノートとの関連付け、検索、フィルターに必要な制約とINDEXを設計する。
- migrationとrollback方法を用意する。

### 3. 検索とフィルター

- 既存検索UIを実検索処理へ接続する。
- タイトル、本文、タグ検索を実装する。
- ノートブック、タグ、作成日、更新日のフィルターを実装する。
- 検索、フィルター、並び替えの組み合わせをテストする。

### 4. メモ管理

- 並び替えと最近開いたメモを実装する。
- バックリンクの抽出規則を決めて実装する。
- 関連メモの判定基準を決めて実装する。

### 5. エディタ改善

- Markdownを壊さない範囲でテーブルコピーを実装する。
- Markdown / Rich往復とクリップボード出力をテストする。

### 6. 品質課題

- 外部編集の検知とreconciliationを実装する。
- APIエラーの共通通知とbatch操作の部分成功・Promise処理は実装済み。
- 並行保存中の表示は実装済み。構造化ログも本文非記録で実装済み。大量ノート一覧は固定上限付きページングと追加読込を実装済み。
- 大量ノート時の検索・起動復旧（全件読み込み）を計測し、差分検知へ段階的に移行する。起動復旧の存在確認は管理ファイル一覧の一括取得へ置き換え済み。

## セキュリティ・整合性

- 検索文字列、タグ名、日付範囲、並び替え項目を検証する。
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
npm --prefix frontend run test:operation-logger
wails build
```

機能追加時は、対象機能の異常系・境界値・競合テストを追加します。
