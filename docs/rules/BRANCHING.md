# ブランチ運用ルール

Atlas Note のブランチ、コミット、PR の基本ルールです。以下のPhase統合ブランチ構成は従来の運用モデルです。実際の作業ブランチとマージ先は、依頼内容・現在のブランチ・リポジトリの運用指示を確認して決めます。

## ブランチ構造

```text
main
  └─ dev
      └─ dev-PhaseN
          ├─ feature/<topic>
          ├─ fix/<topic>
          ├─ docs/<topic>
          └─ chore/<topic>
```

## 各ブランチの役割

| ブランチ | 役割 | マージ先 |
| --- | --- | --- |
| `main` | 安定版。常にビルド可能な状態を維持する | - |
| `dev` | 開発統合用。次のPhaseブランチの起点 | `main` |
| `dev-PhaseN` | Phase単位の実装・検証を統合する | `dev` |
| `feature/<topic>` | 新機能の追加 | 対象の `dev-PhaseN` |
| `fix/<topic>` | バグ修正 | 対象の `dev-PhaseN` |
| `docs/<topic>` | ドキュメントのみの変更 | 対象の `dev-PhaseN` |
| `chore/<topic>` | 設定、依存関係、リポジトリ整備 | 対象の `dev-PhaseN` |

`codex/<topic>` 形式の作業ブランチも利用されています。ここに記載したブランチ名だけから作業先を推測せず、既存ブランチの確認やブランチ作成・改名・マージ・pushは、作業依頼とリポジトリの指示に従ってください。

## 機能開発フロー

作業開始時は現在のブランチと変更状態を確認し、依頼で指定されたブランチを使います。

```bash
git status --short --branch
git branch --list
```

ブランチの作成・切替・pushは、リポジトリ固有の指示と依頼に従って行います。上記のPhase統合フローを機械的に適用しないでください。

## コミット規則

形式:

```text
<type>(<scope>): <summary>
```

## type 一覧

| type | 用途 | 例 |
| --- | --- | --- |
| `feat` | 新機能の追加 | ノート編集、タグ管理、検索機能 |
| `fix` | バグ修正 | 保存失敗、表示崩れ、同期エラー |
| `refactor` | 機能変更を伴わない整理 | Repository 分離、Composable 分割 |
| `docs` | ドキュメントのみの変更 | README 更新、設計メモ追加 |
| `test` | テスト追加・修正 | Go ユニットテスト、Vue コンポーネントテスト |
| `chore` | ビルド、設定、依存関係 | Wails 設定、Vite 設定、依存更新 |
| `perf` | パフォーマンス改善 | 検索高速化、DB クエリ改善 |
| `ci` | CI/CD 設定変更 | GitHub Actions 設定 |

## scope 例

`note` / `editor` / `tag` / `search` / `sync` / `ai` / `settings` / `db` / `storage` / `ui` / `docs`

## コミット例

```text
feat(note): ノート作成ユースケースを追加
fix(editor): Markdown 保存時の改行処理を修正
docs(ai): Agent 向け作業ルールを更新
refactor(db): NoteRepository の責務を整理
chore(wails): 初期 Wails 設定を追加
```

## PR ルール

- `main` への直接 push は避ける。
- PR タイトルは可能な範囲でコミット規則と同じ形式にする。
- PR 本文に変更内容、確認コマンド、未確認事項を書く。
- データ保存、同期、API Key 周辺の変更では、データ消失や秘匿情報漏えいの観点を明記する。
