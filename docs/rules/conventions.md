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
- CLI／MCPへ公開するノート操作は共通読み取り境界と認証済みIPCを通し、外部プロセスからSQLite／Markdownを直接開かない。CLIとMCPのPrincipalを分離し、MCPはinitialize時の保存空間・接続セッション・利用者が明示したnote／Notebook scopeへ固定する。MCP権限は親接続と公開scopeの共通部分に限定し、正常終了または有効期限で失効させる。既定scopeは0件とし、公開範囲外のメタデータ・本文・検索結果を返さない。新しい公開操作は権限、保存空間scope、入力上限、型付きエラーを定義し、Stageごとの公開許可リストへ追加する。
- CLI／MCPの整理解析は既存Organization Serviceへ許可済みnote／Notebook集合を渡し、外部用に整理ロジックを複製しない。`P`と`R1`の両権限を必須とし、同一Note Serviceで保持済みのcontent accessだけをcontext経由で再利用する。broken-linkの実在判定と外部へ候補公開できる解析対象を分離し、restricted解析では明示公開note ID以外の実在／不存在を候補から区別できないようにする。保護・ロック・ゴミ箱・scope外対象を候補へ漏らさない。analysisIdは保存空間・外部クライアント・公開scope・期限へ束縛し、候補取得時にも現在の保護・ロック・ゴミ箱・revisionを再検証する。Stage Bの外部sessionからApplyを許可しない。
- Stage Cの外部W権限は承認待ち変更要求の登録に限定し、MCP childでは親権限とのintersectionを維持する。クライアントから承認時に変更内容を再送させず、GUIだけが本体内permitを発行・単回消費する。`operations.get`は本人の状態だけを返し、レビュー本文を返さない。GUI承認はdirty draftを既存保存laneでflushしてから対象note queueへ入り、バックエンドは適用直前にもrevision・scope・lock・保存空間を再検証する。保存失敗時はdraftとoperationを保持する。外部CLI／MCP用の別保存処理を作らない。
- SQLite 操作は Repository に閉じ込め、UI やサービス層に SQL 詳細を漏らさない。
- Markdown Storage は本文保存の責務を持ち、メタデータ管理は SQLite 側に寄せる。
- AI API Key は平文ログや例外メッセージに出さない。
- Mermaid専用実装は`codex/mermaid-full`で管理する。通常開発ブランチでは既存フェンスを通常のコードとして扱い、ソースを保持する。
- WebDAV 同期はローカルデータを正とする前提で、競合時の扱いを [`docs/development/webdav-sync.md`](../development/webdav-sync.md) に従って実装する。
- 添付画像は `atlasnote-attachment://<noteID>/<attachmentID>` の管理参照だけを本文へ保存し、data URL、ローカル絶対パス、Blob URL、base64本体を永続化しない。保存・読込・ZIP出力は `frontend/src/api/attachments.ts` とGoの添付Store／Wails APIを通し、コンポーネントからファイルシステムへ直接アクセスしない。
- 画像貼り付けの非同期処理ではノートID、本文、選択範囲、Richドキュメントの世代を再検証し、古い応答で本文を変更しない。保存失敗時は本文を保持し、入力データまたは保存済み添付を再試行状態へ残す。通常の文字・表の貼り付け経路を画像処理で上書きしない。

## UI

- デスクトップ向けの実用アプリとして、密度が高くスキャンしやすい画面を優先する。
- 主要操作はキーボード操作とマウス操作の両方を想定する。
- Reka UI のアクセシビリティ前提を崩さない。
- UnoCSS のユーティリティを使い、独自 CSS は必要な範囲に絞る。
- AI機能は`AIWorkspace`の単一チャットtimelineへ統合する。コンポーザーでは開いているノートを削除不能の固定context chipとして表示し、`＋`メニューから追加ノート、Notebook検索scope、要約、文章作成6種、タイトル・タグ・分類・関連・重複・Web検索のスキル／ツールを選択する。Askは読み取り専用で、現行Agentは許可済み読み取りと候補生成だけに限定する。Agent変更提案は、端末ローカル設定の既定`review-required`では差分確認と利用者の明示適用を経て、`auto-update`では通常のAgent送信が返した検証済み本文1差分だけを既存のrevision/CAS・保存laneを通して適用する。送信ボタンは入力欄の右下内側へ置く。
- モデル切替ボタンは既存のAI設定画面を開く。AIのmode、入力、context、timeline、tool trace、結果を`useSettingsStore`や`localStorage`へ保存しない。右側／下側配置、希望寸法、非秘密のAgent本文編集権限だけを端末UI設定として保存し、狭いウィンドウでは実効寸法だけを調整する。
- Web検索はProvider能力を確認し、外部通信について利用者の明示確認を得た場合だけ実行する。構造化tool traceは画面メモリだけに保持し、ログ、SQLite、Markdown、WebDAVへ保存しない。
- 整理センターの候補は「提案」として変更前・変更案・根拠を表示し、各書込操作に明示承認を求める。同一ノートへの承認候補は1バッチでCASし、ノート群は順次適用して競合・失敗を候補ごとに残す。情報候補と重複タグは自動で変更しない。整理とAIは`SupportWorkspace`の共通枠でタブ表示し、タブ切替ではStoreやコンポーネントを破棄しない。バックリンクの整理操作は対象note scopeを共通枠で開く。scope切替時は別sessionを無効化せず、ロック時だけ全sessionを無効化する。
- Local Intelligenceの関連候補は読み取り専用とし、リンク・タグ・FTSの派生索引から件数制限付きで取得する。索引不整合とロック状態取得失敗は結果を返さず、AI参照追加時は候補を再検証する。

## エディタおよびフロントエンド実装時の追加ルール

### Tiptapエディタのカスタマイズ
- **パッケージのインポート**: `BubbleMenu` などのUIコンポーネントは `@tiptap/vue-3` の直下ではなく `@tiptap/vue-3/menus` などの詳細パスから読み込む必要がある場合がある。また、必要に応じて `@tiptap/extension-bubble-menu` などの関連パッケージをフロントエンドの依存関係に追加すること。
- **テーブルのネスト防止**: テーブルの中にさらにテーブルを挿入可能にする挙動を防ぐため、`TableCell` / `TableHeader` を `extend()` し、スキーマ内の `content` から `table` を除外してカスタマイズしたノードを使用する。
- **キーイベント処理**: アプリ全体のショートカットは共通定義と`App.vue`のcapture listenerへ集約し、各コンポーネントへ固定キーを重複実装しない。本文Undo／Redoなどエディタ内部のキーハンドリングは設定値を参照し、Rich編集では`editorProps.handleKeyDown`、Markdown編集ではtextareaのkeydownを通じて行う。IME、AltGraph、ダイアログ、メニュー、キーバインド入力中の抑止を維持する。

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
