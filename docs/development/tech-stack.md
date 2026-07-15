# 技術スタック

Atlas Note で現在採用している技術をまとめます。バージョンの詳細は `package.json`、`frontend/package.json`、`go.mod` を正とします。

| カテゴリ | 採用 |
| --- | --- |
| Desktop | Wails |
| Backend | Go |
| Frontend | Vue 3 |
| Language | Go + TypeScript |
| Build | Vite |
| Styling | UnoCSS |
| UI | Reka UI |
| State | Composables + Pinia |
| Database | SQLite |
| Editor | Markdown textarea + Tiptap |
| Storage | Markdown |
| Data Access | Repository + Squirrel |
| Sync | WebDAV（Phase 3設計確定、実装前） |
| AI | ユーザー自身の API Key を利用する方針 |

## 未確定事項

- デスクトップアプリの配布対象 OS とビルド手順。
- AI 機能の詳細設計。

WebDAVの同期契約は [`webdav-sync.md`](webdav-sync.md) で確定しており、実装順序は [`implementation-plan.md`](implementation-plan.md) で管理する。

## 開発コマンド

```bash
npm run dev
npm run build
npm run frontend:typecheck
npm run frontend:lint
go test ./...
```

環境構築と個別テストは `docs/development/setup.md` を参照してください。
