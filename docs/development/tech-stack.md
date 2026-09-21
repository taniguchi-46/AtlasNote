# 技術スタック

Atlas Note で現在採用している技術と、その責務をまとめます。バージョン、セットアップ、確認コマンドは [セットアップ](setup.md) を正とします。

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
| Sync | WebDAV（同期契約は [webdav-sync.md](webdav-sync.md)） |
| AI | ユーザー自身の API Key をOS Credential Store経由で利用 |

設計上の判断、Docker／Wasmの利用方針、秘密情報の扱いは [開発環境方針](environment.md) を参照してください。
