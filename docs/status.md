# プロジェクト状況

最終更新: 2026-07-15

## 現在のフェーズ

MVP（v0.1）の移行前必須項目とCI確認、Phase 2「整理・検索」の検索基盤、タグCRUD・ノート関連付け、ノートリンク・バックリンク、並び替え・最近更新した一覧・ノートブック移動・テーブルコピーは完了しています。関連メモはPhase 4へ完全移管しています。

Phase 3「同期」は、WebDAV同期設計のレビューと未確定事項の決定まで完了し、実装前の状態です。WebDAV通信、同期用migration、認証情報の保存処理は未実装です。

Phase 2の全体要件は `docs/development/scopes/scope.md`、詳細要件は `docs/development/scopes/scope-phese2.md` を正とします。

## 実装済み

- High着手条件のrevision・CAS・競合検出・ノート単位保存キューは実装・最終検証完了（2026-07-12）

- Wails v2 + Go + Vue 3 + TypeScript + Vite のデスクトップアプリ基盤
- Markdown本文とSQLiteメタデータを組み合わせたローカル保存
- Note / Notebook Repository、Service、Wails API
- 3ペインUI、ノートブック、お気に入り、ピン留め、ゴミ箱
- Markdown / RichエディタとMarkdown serializer
- SQLite / Markdown操作ジャーナル、補償処理、起動時復旧
- 自動保存、dirty draft、保存失敗時の再試行・破棄、終了前flush
- ノート選択の非同期応答逆転防止
- データディレクトリ単位の単一writer保証
- Notebook階層の循環防止
- migration境界、SQLite接続設定、Critical / High項目のCI検証
- Richエディタ変換時のraw HTML無効化と危険な属性・URLの回帰テスト
- schema version 3の `notes.revision` migration、既存行のrevision `1` backfill、Note / Summaryモデルへのrevision追加
- schema version 5の検索状態`content_mtime_ns` migrationと既存行の初回hash再照合
- schema version 6の`tags` / `note_tags` migration、Unicode正規化・case-foldによる同名防止、外部キーCASCADE
- schema version 7の`note_links` / `note_link_state` migration、target/source逆引きINDEX、外部キーCASCADE
- タグのRepository / Service / Wails API、構造化タグエラー、フロントAPI / Pinia Store
- ノート編集画面のタグ付与・解除、タグ候補検索・作成、サイドバーでのタグ一覧表示・改名・削除
- ノートリンクのMarkdown記法・抽出、SQLiteリンク索引、バックリンクAPI・Store・UI
- `expectedRevision`・構造化競合結果モデル、Repositoryの原子的な更新・削除CAS
- Serviceの通常更新・完全削除へのCAS接続、Wails / Storeからの `expectedRevision` 受け渡し
- ノートブック削除に伴うノートのtrash・切り離し時のrevision増加
- Wails APIの構造化競合結果とフロントAPIの型付き `NoteRevisionConflictError`
- Storeの `conflicted` draft状態、競合情報とローカル下書きの保持
- 永続revisionと区別したフロントdraft世代 `draftVersion`
- NoteEditorの保存競合・下書き保持表示
- 競合draftを破棄してサーバー最新版を再読み込む解決操作
- 競合draftを同じノートブックの新規ノートへコピー保存する解決操作
- autosave・メタデータ更新・削除を直列化するノート単位の操作lane
- autosave失敗laneの停止・手動再開、対象別 `flush`
- 保存要求数による正確な `isSaving` 表示
- ノート操作laneと保存要求カウンターの専用回帰テスト
- contentful SQLite FTS5 + trigramによるタイトル・本文検索、ページング、入力検証、再構築可能な索引
- 検索API、検索Store/UI、検索失敗時の共通通知と再試行アクション
- ノート・ノートブック・検索Store/APIの操作別エラーコード、共通通知、再試行アクション
- SHA-256 hashによる外部Markdown編集検知、revision更新、検索索引再構築
- Markdown欠落のMissingNotes報告とrename後の孤児ファイル隔離
- ノート一覧の固定上限付きページング、Store・一覧UIの追加読込
- 起動復旧のMarkdown存在確認をノートごとの`Stat`から管理ファイル一覧の一括取得へ変更
- 起動復旧・検索・一覧の大量データベンチマークと計測手順（`docs/development/performance.md`）
- 検索状態へのMarkdown mtime保存migration、mtime一致時の索引再利用、変更時hash照合フォールバック
- Markdown/Rich変換の空段落、code fence、URL、多重markの境界テスト
- batch操作の完了ID・失敗IDを保持する部分成功処理と、UIイベントのPromise rejection処理
- `noteAutoSave.ts`によるautosave coordinator分離とunexpected rejectionの失敗lane処理
- 本文を含めないoperationログ（note ID、処理段階、エラー分類のみ）
- 単一タグ遷移、解除・0件表示
- ノート一覧の許可リスト付き並び替え（更新日時、作成日時、タイトル）
- 「最近更新した」一覧（ローカル日付の当日00:00〜翌日00:00未満、`updated_at`基準、ゴミ箱除外）
- ノートブックのドラッグ＆ドロップ移動（循環配置防止、ルート移動）
- 表全体のMarkdown / Richコピー（Markdown入り`text/plain`・Rich貼り付け用`text/html`出力、標準MIME型、特殊文字・改行テスト）

## Phase 2の対象

- 既存検索UIへの実検索処理の接続（完了）
- タイトル検索、本文全文検索、タグ条件による通常一覧遷移（完了）
- タグの追加、編集、削除、ノートへの付与・解除、タグ名の候補検索（完了）
- ノートリンク・バックリンク（完了）
- テーブルコピー（完了）

## Phase 2着手前の設計事項

- revision、競合検出、保存キューの仕様は `docs/development/note-concurrency.md` で確定済み
- 全文検索の索引方式はcontentful SQLite FTS5 + trigramに確定済み
- 検索API、ページング、入力検証、エラー形式は `docs/development/search-api.md` で確定済み
- タグのデータモデルと制約（`docs/development/tag-design.md`で確定・実装済み）
- ノートリンク・バックリンクの記法、抽出規則、更新境界は設計・実装済み。関連メモはPhase 4の対象として移管済み。
- 検索とタグ遷移の画面状態、および並び替えとの組み合わせは実装済み。
- DB変更時のmigration、既存データへの影響、rollback方法

## 継続課題

- 大量ノート時の性能確認（ベンチマーク、一覧APIのページング、Store・一覧UIの追加読込、起動復旧の差分検知、5,000件基準値の記録まで完了。継続比較は未完了）
- 競合解決UIのコンポーネントテスト

## 保留事項

- デスクトップアプリの対応OSと配布方式
- 添付ファイルの保存設計
- Phase 3のWebDAV同期実装（認証、manifest方式のdurable outbox、同期競合解決）は確定済みの `docs/development/webdav-sync.md` を正とし、実装TODOを `docs/todo/todo-phese3.md` で管理する。
- API Keyの保存方式と暗号化方針
- AIプロバイダー、モデル選択、課金表示

## 主要コマンド

```bash
npm run frontend:build
npm run frontend:typecheck
npm run frontend:lint
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:note-list-view
npm --prefix frontend run test:serializer
npm --prefix frontend run test:table-copy
npm --prefix frontend run test:operation-logger
npm --prefix frontend run test:note-links
go test ./...
wails build
```

`frontend/wailsjs/`はGit管理対象外です。クリーンcheckout直後は、必要に応じて先に`wails build`でbindingsを生成します。

## 関連ファイル

| ファイル | 役割 |
| --- | --- |
| `README.md` | プロジェクト概要 |
| `docs/development/scopes/scope.md` | Phaseごとの機能要件と対象範囲 |
| `docs/development/scopes/scope-phese2.md` | Phase 2の詳細スコープ |
| `docs/development/implementation-plan.md` | 現在フェーズの実装順序 |
| `docs/development/webdav-sync.md` | Phase 3 WebDAV同期の確定設計 |
| `docs/todo/todo-phese3.md` | Phase 3の同期設計・実装TODO |
| `docs/development/note-concurrency.md` | revision、競合検出、保存キューの確定仕様 |
| `docs/development/search-index.md` | Markdown全文検索の索引方式、更新、再構築設計 |
| `docs/development/search-api.md` | 検索API、ページング、入力検証、エラー契約 |
| `docs/development/tag-design.md` | タグの制約、migration、API、実装・検証状況 |
| `docs/todo/todo-phese2.md` | Phase 2の作業チェックリスト |
| `docs/development/beginner-guide.md` | 初学者向け開発ガイド |
| `docs/development/setup.md` | 開発環境セットアップ |
| `docs/development/tech-stack.md` | 採用技術 |
| `docs/rules/architecture.md` | アーキテクチャとデータ設計 |
| `docs/rules/conventions.md` | 実装規約 |
| `docs/rules/BRANCHING.md` | Git運用ルール |
| `docs/rules/ai.md` | AI Agent共通ガイド |
