# 実装規約

`Atlas Note` の命名、構成、実装ルールです。

## 基本

- 既存の設計と命名を優先する。
- 変更は依頼範囲に絞る。
- 共通化は重複や複雑さを実際に減らす場合だけ行う。
- ローカルファーストを前提に、ネットワーク接続がなくても主要機能が動く設計を優先する。
- 仕様が未確定の場合は、実装で固定せず `docs/status.md` の保留事項に残す。

## 命名

| 対象 | ルール | 例 |
| --- | --- | --- |
| Vue コンポーネント | PascalCase | `NoteEditor.vue`, `TagList.vue` |
| Composable | `use` で始める camelCase | `useNotes.ts`, `useSyncStatus.ts` |
| Pinia Store | `use...Store` | `useNoteStore`, `useSettingsStore` |
| TypeScript 型 | PascalCase | `NoteSummary`, `SyncState` |
| Go パッケージ | 小文字の単語。責務単位で分ける | `note`, `repository`, `sync` |
| Go 型 | PascalCase | `NoteRepository`, `SyncService` |
| Go インターフェース | 振る舞いを表す名前 | `NoteStore`, `KeyProvider` |
| DB テーブル | snake_case | `notes`, `note_tags`, `sync_states` |
| Markdown ファイル | 安定 ID または slug を使う方針。詳細は未確定 | `note-id.md` |

## 実装

- Vue では表示部品、状態管理、Wails API 呼び出しを混ぜすぎない。
- Wails 経由の呼び出しは Composables や API クライアント層にまとめる。
- Go 側はユースケース、Repository、Storage の責務を分ける。
- SQLite 操作は Repository に閉じ込め、UI やサービス層に SQL 詳細を漏らさない。
- Markdown Storage は本文保存の責務を持ち、メタデータ管理は SQLite 側に寄せる。
- AI API Key は平文ログや例外メッセージに出さない。
- WebDAV 同期はローカルデータを正とする前提で、競合時の扱いを明示してから実装する。

## UI

- デスクトップ向けの実用アプリとして、密度が高くスキャンしやすい画面を優先する。
- 主要操作はキーボード操作とマウス操作の両方を想定する。
- Reka UI のアクセシビリティ前提を崩さない。
- UnoCSS のユーティリティを使い、独自 CSS は必要な範囲に絞る。

## 確認

現時点ではアプリ本体のコマンドが未確定です。実装開始後、実際に存在するコマンドへ更新してください。

想定確認コマンド:

```bash
npm run build
npm run typecheck
npm run lint
go test ./...
wails build
```

ドキュメントのみの変更では、リンク切れ、プレースホルダ残り、古い技術前提がないかを確認する。
