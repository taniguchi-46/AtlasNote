# AI 共通ガイド

Atlas NoteのAgentは、この文書と依頼対象に関係する正本を参照します。全資料を毎回読むのではなく、現在状況から対象仕様へ絞ってください。

## 参照順

1. `README.md` と `docs/README.md` で入口を確認する。
2. `docs/status.md` で現在の状態と優先作業を確認する。
3. 対象に関係する `docs/rules/` を確認する。
4. 対象機能のscope、設計資料、未完了TODOを確認する。
5. 関連実装とテストで、文書に書かれた状態を照合する。

## 基本方針

- 既存の設計、データ契約、命名、UIを尊重し、最小差分を優先する。
- ユーザーの未関係変更を保持する。秘密情報を読んだり、ログや回答に出したりしない。
- 文書と実装が食い違う場合は、状況、scope、機能仕様、コード・テストの根拠を分けて示す。
- 正本にない仕様を断定しない。未確定事項は実装で固定せず、TODOまたはstatusで扱う。
- ノート本文をMarkdownの正本、SQLiteをメタデータ・関連情報・再構築可能な索引として扱う。
- 移動・削除前にMarkdownリンク、Agent / Skill、コード、CI、scriptからの参照を確認する。

## 正本の案内

| 目的 | 正本 |
| --- | --- |
| 現況と次の作業 | `docs/status.md` |
| 要求・Phase範囲 | `docs/development/scopes/` |
| 現行機能仕様 | `docs/development/specs/` の対象資料 |
| 恒久ルール | `docs/rules/` |
| 未完了作業 | `docs/todo/` |
| 完了記録・旧資料 | `docs/archive/` |
| Agent運用 | `AGENTS.md`、`.agents/AGENTS.md` |
