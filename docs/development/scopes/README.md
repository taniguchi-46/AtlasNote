# Scope索引

Phaseごとの要求範囲、対象外、完了条件を管理します。進捗はscopeではなく、対応するTODOと [`../../status.md`](../../status.md) に記録します。

| Phase | scope | TODO・補足 |
| --- | --- | --- |
| 全体ロードマップ | [scope.md](scope.md) | Phaseの位置付けと将来計画 |
| Phase 5前：確定機能 | [scope-pre-phase5.md](scope-pre-phase5.md) | 実装対象・順序・未確定設計 |
| Phase 2：整理・検索 | [scope-phese2.md](scope-phese2.md) | [Phase 2 TODO](../../todo/todo-phese2.md) |
| Phase 4 v1：AI設定・単発要約 | [scope-phese4.md](scope-phese4.md) | [Phase 4 v1 TODO](../../todo/todo-phese4.md)、[AI統合](../ai-integration.md) |
| Phase 4 v2：AI司書・実行体験 | [scope-phese4-v2.md](scope-phese4-v2.md) | [Phase 4 v2 TODO](../../todo/todo-phese4-v2.md)、[AIチャット](../ai-chat.md) |
| Phase 4 v3：AIアシスタント・ライティング・履歴 | [scope-phese4-v3.md](scope-phese4-v3.md) | [Phase 4 v3 TODO](../../todo/todo-phese4-v3.md)、[AIチャット](../ai-chat.md) |

## Phase 4 AI

v1からv3までを順番に完了した時点でPhase 4を完了とします。単一チャットの横断契約は[AIチャット](../ai-chat.md)、AI設定と保存境界の決定記録は[AI統合](../ai-integration.md)を正とします。

- v2のAI司書候補は、現在ノートへの候補提示と明示採用を扱う。
- v3は、Q&A、ライティング、ローカル履歴、明示確認付きProvider管理Web検索を扱う。
- 制限付きAgentの変更提案・差分確認・明示適用はv3の決定済み・未実装範囲である。
