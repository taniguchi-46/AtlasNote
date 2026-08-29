# Phase 2 詳細スコープ

> Phase 2の要求・実装・検証記録です。現在の作業状況と残課題は `../../status.md`、Phase 3の作業は `../../todo/todo-phese3.md` を参照してください。

## 目的

MVPで構築したローカル保存基盤と3ペインUIを維持しながら、ノートを探す、分類する、並べる、関連付けるための機能を追加する。

## 現状

- 検索入力UI、タイトル・本文検索、SQLite FTS5索引、タグ条件による通常一覧遷移は実装済み。
- ノートブック、お気に入り、ピン留め、ゴミ箱は実装済み。
- タグCRUD、ノートとの関連、タグ選択ポップアップ、ノートリンク・バックリンクは実装済み。タグ名を入力して候補を絞り込む機能は対象外とする。
- 並び替え、最近更新した一覧、ノートブックのドラッグ＆ドロップ移動は実装済み。
- ノート本文はMarkdown、メタデータはSQLiteを正として扱う。

## 対象機能

### 検索

- MVPで先行実装した検索UIへの実検索処理の接続
- タイトル検索
- 本文全文検索
- タグ検索
- 検索結果が0件の場合の表示
- 検索中、検索失敗時の表示

### タグ

- タグ追加
- タグ編集
- タグ削除
- ノートへのタグ付与と解除
- 同名タグの重複防止
- タグ名の入力検証
- タグ名クリックによる単一タグのノート一覧遷移

### メモ管理

- 並び替え（`updatedAt` / `createdAt` / `title`、`asc` / `desc`）
- 最近更新した一覧（ローカル日付の当日00:00〜翌日00:00未満、`updated_at`基準、アクティブノートのみ）
- ノートブックのドラッグ＆ドロップ移動（子階層・ルート、循環配置防止）
- バックリンク

### エディタ改善

- テーブルコピー
- Markdownを壊さないクリップボード出力
- Markdown / Rich往復後の内容保持

## 実装時に確定した事項

### 検索基盤（確定・実装済み）

- SQLite FTS5 contentful索引とtrigram tokenizerを採用した。詳細は [`search-index.md`](../search-index.md) を正とする。
- タイトル検索と本文全文検索の責務を分離した。
- Markdown外部変更時はhashでreconciliationし、索引を再構築可能にした。
- 検索結果のページング、最大件数、並び順、入力検証を [`search-api.md`](../search-api.md) に確定した。

### タグ設計（完了）

確定仕様と実装・検証状況は `docs/development/tag-design.md` を正とする。

- タグ名の正規化、最大長、大文字小文字の扱い
- UNIQUE制約とINDEX
- ノートとタグの関連テーブル
- タグ削除時の関連解除

### バックリンク（確定・実装済み）

- ノートリンクの記法と抽出規則（`atlasnote://note/<32桁小文字hex ID>`、コード・画像・外部URL除外）
- リンク先が存在しない場合の扱い（本文は保持し、索引には登録しない）
- 索引更新のタイミング（ノート保存・起動復旧、失敗時は再構築可能）

### API・画面状態（確定・実装済み）

- 検索、並び替えを組み合わせる入力形式
- バリデーションエラーと内部エラーの区別
- 最新の検索要求だけを画面へ反映する競合対策
- 検索条件の保持と初期化方法

## セキュリティ・整合性要件

- 検索文字列、タグ名を検証する。
- 並び替え項目と方向は許可リストで制限する。
- SQLはRepository層でパラメータ化し、ユーザー入力をSQL文字列へ直接連結しない。
- ノート本文、検索語、秘密情報をログへ不用意に出さない。
- 索引は再構築可能にし、Markdown本文を正本として維持する。
- DB変更時はmigration失敗をrollbackできるようにする。
- タグや索引の不整合がノート本文の保存を壊さないようにする。

## DB変更時の確認事項

- 主キー、外部キー、NULL許可、UNIQUE制約、INDEX
- 既存データへの影響
- migrationの実行順序
- rollback方法
- 将来版DBの拒否
- タグ削除、ノート削除時の参照整合性

DBスキーマとmigrationは、この文書では確定しない。対象機能の設計完了後に最小変更で追加する。

## 対象外

- WebDAV同期と同期競合解決（Phase 3）
- AI連携（Phase 4以降）
- 関連メモ（Phase 4へ完全移管）
- 添付ファイル、画像貼り付け、ノート本文・添付ファイルのドラッグ＆ドロップ
- 自動バックアップ、バックアップ復元
- グローバルショートカット
- モバイル対応
- プラグインシステム

## Phase 2対応済み品質課題と残課題

- Markdownのhashによる外部編集・rename・deleteの検知とreconciliationは実装済み。
- store / APIエラーの共通通知、batch操作の部分成功、未処理Promiseの整理は完了。
- 並行保存中の状態表示、構造化ログ、大量ノートの基準値計測は完了。
- Markdown / Rich変換の境界テストは完了。Rich機能追加時のserializer round-tripテストは継続課題。
- 競合解決UIのコンポーネントテストと性能基準値の継続比較は `docs/status.md` で管理する。

## 完了条件（実装後の検証基準）

- 対象機能の正常系、異常系、境界値テストが成功する。
- 検索、タグ遷移、並び替えを併用しても結果が一貫する。
- 今日の境界、ゴミ箱・復元・完全削除後の最近更新一覧、並び替え入力の拒否をテストする。
- ノートブックの自己・子孫移動拒否とルート移動をテストする。
- ユーザー入力によるSQLインジェクションや情報漏えいがない。
- 既存の保存、編集、ノートブック機能を壊していない。
- DB変更がある場合、migrationとrollbackを確認できる。
- 型検査、フロントエンドテスト、Goテスト、Wailsビルドが成功する。
- 関連ドキュメントが実装状態と一致する。

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
npm --prefix frontend run test:note-list-view
npm --prefix frontend run test:serializer
npm --prefix frontend run test:table-copy
wails build
```

Phase 2の機能追加時は、対象API、Repository、Store、UIのテストを追加する。
