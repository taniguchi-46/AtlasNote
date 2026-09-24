# TODO索引

TODOは、実装・自動テスト・手動受け入れの進捗を記録するチェックリストです。プロジェクト全体の状態は [`../status.md`](../status.md)、要求範囲と完了条件は [`../development/scopes/README.md`](../development/scopes/README.md) を正とします。

| Phase | TODO | 現在の扱い |
| --- | --- | --- |
| Phase 2：整理・検索 | [アーカイブ](../archive/phase2/todo.md) | 完了記録 |
| Phase 3：同期 | [アーカイブ](../archive/phase3/todo.md) | 受け入れ完了の記録 |
| Phase 4 v1：AI設定・単発要約 | [todo-phese4.md](todo-phese4.md) | 完了記録 |
| Phase 4 v2：AI司書・実行体験 | [todo-phese4-v2.md](todo-phese4-v2.md) | 手動・統合受け入れが残る |
| Phase 4 v3：AIアシスタント・ライティング・履歴 | [todo-phese4-v3.md](todo-phese4-v3.md) | 制限付きAgentの変更提案・差分確認を含む残作業を管理 |
| Pre-Phase 5：整理センター | [todo-pre-phase5-organization.md](todo-pre-phase5-organization.md) | 実装済み。性能評価と手動UI受け入れを管理 |

## 更新ルール

- `【実装済み】` はコードまたは自動テストで根拠を確認できる項目にだけ付ける。
- `【未検証】` は手動受け入れ、異常系、統合テストなどが残る項目に付ける。
- `【決定済み・未実装】` は仕様は合意済みでも、コード・テスト・受け入れが完了していない項目に付ける。
- CI、手動受け入れ、実キーを使わない確認記録には、本文、プロンプト、API Key、endpoint、raw provider errorを残さない。

## テンプレート

- [TODOテンプレート](todo-templet/todo-templet.md)
