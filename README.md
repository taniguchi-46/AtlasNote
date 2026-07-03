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

- `docs/rules/`: Agent 共通で参照するプロジェクト知識、設計、規約。
- `docs/status.md`: 現在の作業状況。
- `.agents/`: Agent 固有の行動指針とスキル (`.agents/AGENTS.md` など)。
- `.codex/`: テンプレート類。

作業前に `docs/status.md` と `docs/rules/ai.md` を確認してください。

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
| Editor | Tiptap + CodeMirror |
| Storage | Markdown |
| Data Access | Repository + Squirrel |

---

## 開発環境セットアップ

開発環境のセットアップと起動方法は `docs/development/setup.md` を参照してください。

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
