# Atlas Note ドキュメント

Atlas Note のドキュメント入口です。現在の状況、要求範囲、確定設計、実装順序、作業チェックリストを役割ごとに分けて管理します。

## 参照順

1. [プロジェクト状況](status.md)で現在のフェーズ、完了事項、残課題、移行条件を確認する。
2. [開発ロードマップ](development/scopes/scope.md)で要求範囲と対象外を確認する。
3. 対象機能の「正本」と明記された設計資料を確認する。
4. 現在フェーズの実装順序とTODOを確認する。
5. 実装前後に[AI共通ガイド](rules/ai.md)、[アーキテクチャ](rules/architecture.md)、[実装規約](rules/conventions.md)を確認する。

## 正本と役割

| 目的 | 正本 | 役割 |
| --- | --- | --- |
| 現在状況 | [docs/status.md](status.md) | 実装済み、残課題、保留事項、確認状況 |
| 要求範囲 | [scope.md](development/scopes/scope.md) | Phaseごとの機能要件と対象外 |
| Phase 3同期設計 | [webdav-sync.md](development/webdav-sync.md) | 同期対象、リモート形式、競合、認証、outboxの確定契約 |
| Phase 3実装順序 | [implementation-plan.md](development/implementation-plan.md) | 設計に従った実装ステップ |
| Phase 3進捗 | [todo-phese3.md](todo/todo-phese3.md) | 実装・検証チェックリスト |
| 横断ルール | [docs/rules/](rules/) | Agent、アーキテクチャ、命名、Git、用語 |

仕様の決定事項は設計資料、現在の実装状態はコードとテスト、進捗は `docs/status.md` とTODOを根拠にします。文書同士が矛盾する場合は、勝手に解釈を固定せず、正本と未確定事項を確認します。

## 開発ガイド

- [初心者向け開発ガイド](development/beginner-guide.md)
- [開発環境セットアップ](development/setup.md)
- [開発環境方針](development/environment.md)
- [技術スタック](development/tech-stack.md)

## 設計資料

- [ノートrevision・競合検出・保存キュー](development/note-concurrency.md)
- [Markdown全文検索索引](development/search-index.md)
- [検索API](development/search-api.md)
- [タグ設計](development/tag-design.md)
- [大量ノート性能計測](development/performance.md)

## Phase 2の記録

- [Phase 2詳細スコープ](development/scopes/scope-phese2.md)
- [Phase 2実績・残課題](todo/todo-phese2.md)
