# 設計・仕様書

機能とシステム境界の設計仕様を領域ごとに案内します。開発手順や技術入門は [`../guides/README.md`](../guides/README.md)、要求範囲・完了条件は [`../scopes/README.md`](../scopes/README.md) を参照してください。

## 外部連携・platform

| 文書 | 内容 |
| --- | --- |
| [CLI / MCP / Terminal再編](AtlasNote_CLI_MCP_Rearchitecture_Spec.md) | 外部CLI、MCP、IPC、承認、統合Terminal |
| [WebDAV同期](webdav-sync.md) | 同期形式、outbox、競合、復旧 |
| [Windows配布](windows-distribution.md) | Windows配布物、installer、uninstall |
| [キーボードショートカット](keyboard-shortcuts.md) | アプリ操作・設定・本文履歴 |
| [Mermaid](mermaid.md) | 対応branchとソース保持方針 |

## ノート・保存・データ保全

| 文書 | 内容 |
| --- | --- |
| [ノートrevision・競合・保存queue](note-concurrency.md) | CAS、競合、保存lane |
| [添付画像](attachments.md) | 添付の保存、検証、暗号化、復旧 |
| [保存空間](storage-spaces.md) | 空間分離、切替、再起動 |
| [物理保存場所](storage-locations.md) | データルート、移行、初回起動 |
| [コンテンツロック](content-locks.md) | ロックと本文保護 |
| [バックアップ・復元](backup-restore.md) | 完全性検証、復旧、rollback |
| [ノートインポート](note-import.md) | 形式変換、入力検証、保存境界 |
| [ノートエクスポート](note-export.md) | 出力形式、snapshot、原子的保存 |

## 検索・整理

| 文書 | 内容 |
| --- | --- |
| [検索索引](search-index.md) | Markdown全文検索索引 |
| [検索API](search-api.md) | 検索API、入力検証、エラー契約 |
| [タグ設計](tag-design.md) | タグ制約、migration、API |
| [整理センター](organization-center.md) | 整理候補、承認、保存・競合境界 |
| [Local Intelligence](local-intelligence.md) | 関連候補API |