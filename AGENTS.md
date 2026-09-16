# Atlas Note Agent Instructions

このリポジトリでは、作業前に [.agents/AGENTS.md](.agents/AGENTS.md) を読み、そこから案内されるプロジェクト規約と仕様を確認する。

計画・実装・コードレビュー・リファクタリングでは、[開発ワークフロー補足](docs/rules/development-workflows.md) の対象に関係する契約と検証手順も確認する。共通スキルを使う場合も、このプロジェクト固有の契約を維持する。

`implementation-plan`、`implementation`、`code-review`、`refactoring` はユーザー共通の `~/.agents/skills/` で管理する。利用できない環境では、このリポジトリの指示と依頼範囲に沿って作業し、スキルを使用したとは報告しない。

`next-phase-review` は `.agents/skills/next-phase-review/` のプロジェクト専用スキルとして維持する。
