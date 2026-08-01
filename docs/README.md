# Atlas Note ドキュメント

Atlas Noteの仕様、設計、進捗、作業チェックの入口です。ファイルの配置は維持し、用途ごとのREADMEから辿れる構成にしています。

## まず読む

1. [プロジェクト状況](status.md)で現在の実装状態・残作業・受け入れ状況を確認する。
2. [開発ロードマップ](development/scopes/scope.md)でPhaseごとの対象範囲を確認する。
3. [開発・設計資料の索引](development/README.md)または[TODO索引](todo/README.md)から、対象機能の正本へ進む。
4. 実装時は[アーキテクチャ](rules/architecture.md)、[実装規約](rules/conventions.md)、[AI共通ガイド](rules/ai.md)を確認する。

## ディレクトリ案内

| 場所 | 役割 | 入口 |
| --- | --- | --- |
| `docs/status.md` | 現在の実装・検証状態、残課題、保留事項 | [プロジェクト状況](status.md) |
| `docs/development/` | 機能設計、同期・データ契約、開発ガイド | [開発・設計資料](development/README.md) |
| `docs/development/scopes/` | Phaseごとの要求範囲・対象外・完了条件 | [scope索引](development/scopes/README.md) |
| `docs/todo/` | 実装・検証・受け入れのチェックリスト | [TODO索引](todo/README.md) |
| `docs/rules/` | アーキテクチャ、実装規約、Git、用語 | [ルール一覧](rules/) |

## Phase 4 AIの正本

| 目的 | 文書 |
| --- | --- |
| 単一チャット、context、Ask／Agent、Web検索、変更提案のUI契約 | [AIチャット](development/ai-chat.md) |
| AI設定、Provider、資格情報、v1の決定記録、保存境界 | [AI統合](development/ai-integration.md) |
| Phase 4 v1／v2／v3の対象範囲・完了条件 | [scope索引](development/scopes/README.md) |
| Phase 4の実装・検証状況 | [TODO索引](todo/README.md) |

現行実装と決定済み・未実装の仕様は、必ず区別して記載します。文書間に矛盾がある場合は、`docs/status.md`、該当scope、TODOを確認し、未確定事項を勝手に実装済みとして扱いません。

## 更新ルール

- 要求範囲・対象外・完了条件を変えるときは、該当するscopeを更新する。
- 実装・テスト・手動受け入れの進捗は、該当するTODOと`docs/status.md`を更新する。
- 実装状態に依存するUI・API・保存境界は、機能設計資料に記録する。
- 既存ファイルを移動する前に、`rg`でMarkdownリンクとコード参照を確認する。
