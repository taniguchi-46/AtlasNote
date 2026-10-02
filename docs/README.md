# Atlas Note ドキュメント

現在状況、機能仕様、恒久ルール、未完了作業、完了記録の入口です。各情報は正本に集約し、他の文書では必要な参照だけを置きます。

## まず読む

1. [現在状況](status.md)でテーマ、未確認事項、次の作業を確認する。
2. [開発資料索引](development/README.md)から対象機能の仕様を開く。
3. 実装時は[Agent共通ガイド](rules/ai.md)と、対象に関係するルールを確認する。
4. 作業項目は[TODO索引](todo/README.md)、完了記録は[Archive](archive/README.md)を参照する。

## 正本

| 目的 | 正本 |
| --- | --- |
| 現在状況・次の優先作業 | [status.md](status.md) |
| 要求・機能仕様・開発ガイド | [development/](development/README.md) |
| 恒久ルール | [rules/](rules/ai.md) |
| 未完了作業 | [todo/](todo/README.md) |
| 完了済みPhase・過去の作業履歴 | [archive/](archive/README.md) |
| 利用者向け案内 | [user-readme.md](user-readme.md) |
| Agent運用 | ルートの `AGENTS.md` と `.agents/` |

## 更新ルール

- 現在状況と次の優先作業は `status.md` だけで管理する。
- 要求・設計契約は対象のdevelopment資料、完了条件はscope、作業進捗は現役TODOへ記録する。
- 完了済みPhaseのscope・TODO・受け入れ記録は `archive/` に移す。
- 文書を移動する前に、MarkdownリンクとAgent、Skill、コード、CI、scriptからの参照を確認する。