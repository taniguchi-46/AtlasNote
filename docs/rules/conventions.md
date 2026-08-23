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
| Markdown ファイル | 安定 ID を使う | `note-id.md` |

## 実装

- Vue では表示部品、状態管理、Wails API 呼び出しを混ぜすぎない。
- Wails 経由の呼び出しは Composables や API クライアント層にまとめる。
- Go 側はユースケース、Repository、Storage の責務を分ける。
- SQLite 操作は Repository に閉じ込め、UI やサービス層に SQL 詳細を漏らさない。
- Markdown Storage は本文保存の責務を持ち、メタデータ管理は SQLite 側に寄せる。
- AI API Key は平文ログや例外メッセージに出さない。
- WebDAV 同期はローカルデータを正とする前提で、競合時の扱いを [`docs/development/webdav-sync.md`](../development/webdav-sync.md) に従って実装する。

## UI

- デスクトップ向けの実用アプリとして、密度が高くスキャンしやすい画面を優先する。
- 主要操作はキーボード操作とマウス操作の両方を想定する。
- Reka UI のアクセシビリティ前提を崩さない。
- UnoCSS のユーティリティを使い、独自 CSS は必要な範囲に絞る。
- AI機能は`AIWorkspace`の単一チャットtimelineへ統合する。コンポーザーでは開いているノートを削除不能の固定context chipとして表示し、`＋`メニューから追加ノート、Notebook検索scope、要約、文章作成6種、タイトル・タグ・分類・関連・重複・Web検索のスキル／ツールを選択する。Askは読み取り専用で、現行Agentは許可済み読み取りと候補生成だけに限定する。Agent変更提案は、端末ローカル設定の既定`review-required`では差分確認と利用者の明示適用を経て、`auto-update`では通常のAgent送信が返した検証済み本文1差分だけを既存のrevision/CAS・保存laneを通して適用する。送信ボタンは入力欄の右下内側へ置く。
- モデル切替ボタンは既存のAI設定画面を開く。AIのmode、入力、context、timeline、tool trace、結果を`useSettingsStore`や`localStorage`へ保存しない。右側／下側配置、希望寸法、非秘密のAgent本文編集権限だけを端末UI設定として保存し、狭いウィンドウでは実効寸法だけを調整する。
- Web検索はProvider能力を確認し、外部通信について利用者の明示確認を得た場合だけ実行する。構造化tool traceは画面メモリだけに保持し、ログ、SQLite、Markdown、WebDAVへ保存しない。

## エディタおよびフロントエンド実装時の追加ルール

### Tiptapエディタのカスタマイズ
- **パッケージのインポート**: `BubbleMenu` などのUIコンポーネントは `@tiptap/vue-3` の直下ではなく `@tiptap/vue-3/menus` などの詳細パスから読み込む必要がある場合がある。また、必要に応じて `@tiptap/extension-bubble-menu` などの関連パッケージをフロントエンドの依存関係に追加すること。
- **テーブルのネスト防止**: テーブルの中にさらにテーブルを挿入可能にする挙動を防ぐため、`TableCell` / `TableHeader` を `extend()` し、スキーマ内の `content` から `table` を除外してカスタマイズしたノードを使用する。
- **キーイベント処理**: ショートカットキーなどのエディタ内部のキーハンドリングは、エディタの設定時に `editorProps.handleKeyDown` を通じて行う。

### フロントエンド先行のUIモック
- **一時的プロパティの拡張**: フロントエンドで先行して実装するUI用の追加プロパティ（例: ノートブックの `icon`）は、既存のGoモデル定義（Wails自動生成コード）に影響を与えないよう、TypeScript側で定義する拡張インターフェース（例: `NotebookNode`）にオプショナルプロパティとして追加し、フロントエンド側で安全にフォールバック処理を行う。

## 確認

現在の基本確認コマンドは次のとおりです。対象機能に応じて個別テストを追加してください。

想定確認コマンド:

```bash
npm run build
npm run frontend:typecheck
npm run frontend:lint
go test ./...
wails build
```

ドキュメントのみの変更では、リンク切れ、プレースホルダ残り、古い技術前提がないかを確認する。
