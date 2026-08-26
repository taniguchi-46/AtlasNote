# 開発・設計資料

実装前は対象機能のscope、設計契約、TODOの順に確認します。現在の進捗は [`../status.md`](../status.md)、作業チェックは [`../todo/README.md`](../todo/README.md) を参照します。

## AI（Phase 4）

| 文書 | 役割 |
| --- | --- |
| [AIチャット](ai-chat.md) | 単一チャット、context、Ask／Agent、Provider管理Web検索、変更提案のUI・実装状態 |
| [AI統合](ai-integration.md) | AI設定、Provider adapter、資格情報、v1決定記録、保存・同期境界 |
| [Phase 4 scope](scopes/README.md#phase-4-ai) | v1／v2／v3の対象範囲と完了条件 |

## コア設計・同期

| 文書 | 役割 |
| --- | --- |
| [ノート競合・保存キュー](note-concurrency.md) | revision、CAS、保存lane |
| [ノート保存空間](storage-spaces.md) | 保存ルート、空間ごとのSQLite・Markdown・同期・lock、再起動切替 |
| [ノートインポート](note-import.md) | md・txt・HTMLの安全な変換、保存・ロック・部分成功契約 |
| [WebDAV同期](webdav-sync.md) | Phase 3の同期契約、競合、復旧 |
| [検索索引](search-index.md) | Markdown全文検索の索引方式 |
| [検索API](search-api.md) | 検索API、入力検証、エラー契約 |
| [タグ設計](tag-design.md) | タグの制約、migration、API |
| [性能計測](performance.md) | 大量ノート時の計測方法・基準 |

## ガイド・環境

| 文書 | 役割 |
| --- | --- |
| [セットアップ](setup.md) | 開発環境の構築 |
| [開発環境方針](environment.md) | 実行環境と運用上の注意 |
| [技術スタック](tech-stack.md) | 採用技術と役割 |
| [初心者向けガイド](beginner-guide.md) | 開発の基本手順 |
| [実装計画](implementation-plan.md) | Phase 3の実装順序・回帰確認 |

## 文書の役割分担

- scope: 要求範囲、対象外、完了条件
- 設計資料: データ・API・UI・同期などの確定契約
- TODO: 実装、テスト、手動受け入れの進捗
- status: プロジェクト全体の現況と次の判断
