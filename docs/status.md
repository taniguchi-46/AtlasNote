# プロジェクト状況

最終更新: 2026-07-13

## 現在のフェーズ

MVP（v0.1）の移行前必須項目とCI確認は完了し、Phase 2「整理・検索」の検索基盤を実装中です。

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
- Markdown/Rich変換の空段落、code fence、URL、多重markの境界テスト
- batch操作の完了ID・失敗IDを保持する部分成功処理と、UIイベントのPromise rejection処理
- `noteAutoSave.ts`によるautosave coordinator分離とunexpected rejectionの失敗lane処理
- 本文を含めないoperationログ（note ID、処理段階、エラー分類のみ）

## Phase 2の対象

- 既存検索UIへの実検索処理の接続（完了）
- タイトル検索、本文全文検索（完了）、タグ検索
- タグの追加、編集、削除
- ノートブック、タグ、作成日、更新日のフィルター
- 並び替え、最近開いたメモ、バックリンク、関連メモ
- テーブルコピー

## Phase 2着手前の設計事項

- revision、競合検出、保存キューの仕様は `docs/development/note-concurrency.md` で確定済み
- 全文検索の索引方式はcontentful SQLite FTS5 + trigramに確定済み
- 検索API、ページング、入力検証、エラー形式は `docs/development/search-api.md` で確定済み
- タグのデータモデルと制約
- バックリンクの抽出規則と関連メモの判定基準
- 検索、フィルター、並び替えを組み合わせるAPIと画面状態
- DB変更時のmigration、既存データへの影響、rollback方法

## 継続課題

- 大量ノート時の性能確認（ベンチマーク、一覧APIのページング、Store・一覧UIの追加読込、起動復旧の存在確認の一括化を実装済み。実運用件数での継続比較と差分検知は未完了）
- 競合解決UIのコンポーネントテスト

## 保留事項

- デスクトップアプリの対応OSと配布方式
- 添付ファイルの保存設計
- WebDAV同期時の競合解決
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
npm --prefix frontend run test:serializer
npm --prefix frontend run test:operation-logger
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
| `docs/development/note-concurrency.md` | revision、競合検出、保存キューの確定仕様 |
| `docs/development/search-index.md` | Markdown全文検索の索引方式、更新、再構築設計 |
| `docs/development/search-api.md` | 検索API、ページング、入力検証、エラー契約 |
| `docs/todo/todo-phese2.md` | Phase 2の作業チェックリスト |
| `docs/development/beginner-guide.md` | 初学者向け開発ガイド |
| `docs/development/setup.md` | 開発環境セットアップ |
| `docs/development/tech-stack.md` | 採用技術 |
| `docs/rules/architecture.md` | アーキテクチャとデータ設計 |
| `docs/rules/conventions.md` | 実装規約 |
| `docs/rules/BRANCHING.md` | Git運用ルール |
| `docs/rules/ai.md` | AI Agent共通ガイド |
