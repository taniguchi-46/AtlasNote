# AI Project Template Setup

このテンプレートは、新しいプロジェクトに AI Agent 向けの共通ドキュメント、Codex 設定、汎用 skill を導入するための雛形です。

## コピー対象

```text
docs/
.agents/
.codex/AGENTS.md
README_AI_SECTION.md
```

`README_AI_SECTION.md` は既存 README に貼り付けるための断片です。コピー後に不要であれば削除して構いません。

## 使い方

1. `ai-project-template/` の中身を新しいプロジェクトのルートへコピーする。
2. `docs/status.md` のプレースホルダをプロジェクトに合わせて埋める。
3. `docs/architecture.md` に採用フレームワーク、ディレクトリ構成、データ構造を書く。
4. `docs/conventions.md` に命名規則、実装方針、確認コマンドを書く。
5. 必要な作業メモを `.agents/skills/skill.md` に追加する。
6. Codex を使う場合は `.codex/AGENTS.md` をプロジェクトに合わせて確認する。
7. README に AI ドキュメントの案内が必要なら `README_AI_SECTION.md` を追記する。

## 置き換えるプレースホルダ

| プレースホルダ | 内容 |
| --- | --- |
| `{{PROJECT_NAME}}` | プロジェクト名 |
| `{{PROJECT_TYPE}}` | サイト、API、アプリなどの種別 |
| `{{FRAMEWORK}}` | 主なフレームワーク |
| `{{LANGUAGE}}` | 主な言語 |
| `{{RUNTIME}}` | 実行環境 |
| `{{STYLING}}` | スタイリング方式 |
| `{{DEPLOYMENT}}` | 配信、デプロイ方式 |
| `{{PRIMARY_COMMANDS}}` | よく使うコマンド |

## 運用ルール

- AI 共通知識は `docs/` に置く。
- Agent 固有の行動指針は `.codex/`, `.agents/` などに置く。
- プロジェクト固有の状態は `docs/status.md` に集約する。
- 未確定の内容は TODO または保留事項として残してよい。
- テンプレートには特定プロジェクトだけの固有名やパスを残さない。
