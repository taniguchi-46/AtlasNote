# Atlas Note

> AIを前提としたローカルファーストの知識管理・Second Brainアプリ

## コンセプト

Atlas Note は、単なるメモアプリではなく、知識を蓄積・整理・活用するためのローカルファーストな Second Brain プラットフォームです。

目指すもの

- UpNote の使いやすさ
- Joplin の自由度
- Obsidian の知識管理
- AI による知識整理・ライティング支援
- 開発者向け機能
- WebDAV を中心とした同期
- ユーザー自身の API Key を利用した AI 機能

---

# 技術スタック

| カテゴリ | 採用 |
|----------|------|
| Desktop | Wails |
| Language | Go + TypeScript |
| Frontend | Vue 3 |
| Build | Vite |
| Styling | UnoCSS |
| UI | Reka UI |
| State | Composable + Pinia |
| Database | SQLite |
| Editor | Tiptap + CodeMirror |
| Storage | Markdown |
| Data Access | Repository + Squirrel |

---

# アーキテクチャ

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
