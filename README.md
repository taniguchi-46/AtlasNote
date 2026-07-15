# Atlas Note

> AIを前提としたローカルファーストの知識管理・Second Brainアプリ

## コンセプト

Atlas Note は、単なるメモアプリではなく、知識を蓄積・整理・活用するためのローカルファーストな Second Brain プラットフォームです。

目指すもの

- UpNote の使いやすさ
- Joplin の自由度
- AI による知識整理・ライティング支援
- 開発者向け機能
- WebDAV を中心とした同期
- ユーザー自身の API Key を利用した AI 機能

---

## AI Documents

このプロジェクトでは、AI Agent 向けの共通ドキュメントと設定を以下に整理しています。

- `docs/README.md`: ドキュメント全体の入口と正本の役割。
- `docs/rules/`: Agent 共通で参照するプロジェクト知識、設計、規約。
- `docs/status.md`: 現在の作業状況。
- `docs/development/scopes/scope.md`: Phase ごとの開発ロードマップと機能要件。
- `.agents/`: Agent 固有の行動指針とスキル (`.agents/AGENTS.md` など)。
- `.codex/`: テンプレート類。

作業前に `docs/README.md`、`docs/status.md`、`docs/development/scopes/scope.md`、`docs/rules/ai.md` を確認してください。

---

## 技術スタック

| カテゴリ | 採用 |
|----------|------|
| Desktop | Wails |
| Language | Go + TypeScript |
| Frontend | Vue 3 |
| Build | Vite |
| Styling | UnoCSS |
| UI | Reka UI |
| State | Composables + Pinia |
| Database | SQLite |
| Editor | Markdown textarea + Tiptap |
| Storage | Markdown |
| Data Access | Repository + Squirrel |

---

## 開発環境セットアップとガイド

- [ドキュメント入口](docs/README.md)
- 初めて開発に参加する方向けの全体像・解説: [初心者向け開発ガイド](docs/development/beginner-guide.md)
- 詳細な開発環境のセットアップと起動方法: [開発環境セットアップ](docs/development/setup.md)

---

## アーキテクチャ

```text
┌──────────────────────────────┐
│            Vue 3             │
├──────────────────────────────┤
│ Components                   │
│        │                     │
│        ▼                     │
│ Composables                  │
│        │                     │
│        ▼                     │
│ Pinia                        │
└──────────────┬───────────────┘
               │
               ▼
        Wails (Go Backend)
               │
               ▼
       Repository Layer
               │
               ▼
     Squirrel (Query Builder)
               │
               ▼
             SQLite
               │
               ▼
      Markdown Storage
```
