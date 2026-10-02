# Atlas Note 開発チェックリスト

Atlas Noteの開発作業で、`.agents/AGENTS.md` と [`docs/rules/ai.md`](../../docs/rules/ai.md) を補う短いチェックリストです。独立した仕様の正本ではありません。

1. [`docs/status.md`](../../docs/status.md) で現況と次の優先作業を確認する。
2. [`docs/README.md`](../../docs/README.md) と対象機能のscope・仕様・TODOから正本を確認する。
3. 対象に関係するarchitecture、conventions、development workflowの契約を読む。
4. 既存実装、呼び出し元、関連テストを確認して最小差分で進める。
5. 変更した文書・実装のリンク、進捗、確認結果を正本へ反映する。

ノート本文はMarkdownを正本、SQLiteはメタデータ・関連情報・再構築可能な索引として扱います。同期、資格情報、復旧、Agent提案の契約は対象仕様を確認してください。