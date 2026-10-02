# Codex Agent Guide

このファイルは、Atlas NoteでCodexが作業するときの行動指針です。共通ルールは [`docs/rules/ai.md`](../docs/rules/ai.md) を参照してください。

## 参照順

1. `README.md`（プロジェクト入口）
2. `docs/status.md`（現況と次の優先作業）
3. `docs/rules/ai.md` と依頼対象に関係するrules
4. 対象機能のscope・仕様・TODO

## 基本方針

- 既存のWails / Go / TypeScript / Vue 3 / UnoCSSの設計に合わせ、変更を依頼範囲に絞る。
- 関連資料と実装を確認してから編集する。仕様と実装が食い違う場合は断定せず、根拠と影響を示す。
- ユーザーが作った未関係の変更を戻さない。秘密情報を表示・変更しない。
- 製品コードや設定を変更した場合は、status・architecture・conventionsの更新要否を判断する。
- 実装後は、対象とリスクに応じた確認を行う。調査・計画だけではテストやbuildを実行しない。

必要に応じて [Atlas Note開発チェックリスト](skills/skill.md) も参照する。

## 完了報告

変更したファイル、内容と理由、実行した確認、未確認事項を簡潔に報告する。