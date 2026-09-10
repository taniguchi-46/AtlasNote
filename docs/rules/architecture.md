# アーキテクチャ

`Atlas Note` の設計情報をまとめます。

## 全体構成

Atlas Note は Wails を使うデスクトップアプリです。UI は Vue 3、アプリケーションロジックと OS / DB / ファイルアクセスは Go 側に寄せ、データは SQLite と Markdown ファイルを組み合わせて扱う方針です。

```text
Vue 3 / TypeScript / Vite
  ├─ Components
  ├─ Composables
  └─ Pinia
       │
       ▼
Wails Bridge
       │
       ▼
Go Backend
  ├─ Application Services
  ├─ Repository Layer
  ├─ Squirrel Query Builder
  ├─ SQLite
  └─ Markdown Storage
```

## 採用技術

| 項目 | 内容 |
| --- | --- |
| フレームワーク | Wails + Vue 3 + Vite |
| 言語 | Go + TypeScript |
| スタイル | UnoCSS + Reka UI |
| 実行環境 | Wails デスクトップアプリ、開発時は Go / Node.js / Vite |
| 配信 / デプロイ | デスクトップアプリとして配布予定。詳細は未確定 |

## 主要モジュール

| モジュール | 役割 |
| --- | --- |
| Vue Components | ノート一覧、エディタ、設定画面などの表示部品 |
| Composables | UI ロジック、Wails API 呼び出し、入力状態の再利用可能な処理 |
| Pinia | ノート選択、検索条件、同期状態などのフロントエンド状態管理 |
| Wails Bridge | TypeScript から Go のアプリケーションサービスを呼び出す境界 |
| Go Application Services | ユースケース単位の処理、トランザクション、入力検証 |
| Repository Layer | SQLite と Markdown Storage への永続化を隠蔽する層 |
| SQLite | ノートのメタデータ、タグ、リンク、検索用インデックスなど |
| Markdown Storage | ノート本文の永続化 |
| Storage Spaces | 保存ルート内でSQLite、Markdown、同期状態、AIローカルデータ、単一writer lockを空間ごとに分離する。詳細は`docs/development/storage-spaces.md`を正とする |
| Note Export | アクティブな単一ノートの保存済みMarkdown snapshotを、revision・コンテンツロック再検証後にHTML／PDFへ変換し、OSネイティブ保存ダイアログで選択したパスへ原子的に出力する。詳細は`docs/development/note-export.md`を正とする |
| Backup / Restore | アクティブな保存空間のSQLite・Markdownを設定されたアーカイブルートへ世代保存し、SHA-256・SQLite integrity検証、再起動前stage、起動時swap／rollbackで復元する。詳細は`docs/development/backup-restore.md`を正とする |
| WebDAV Sync | `docs/development/webdav-sync.md` のPhase 3契約に従うformat/head/manifest/object、durable outbox、競合、フェイルセーフ、復旧処理。コア実装済み |
| AI Integration | ユーザー自身の API Key を使う知識整理、要約、AIアシスタント、ライティング支援。AI機能は`AIWorkspace`の単一チャットtimelineへ統合し、開いているノートを固定コンテキスト、追加ノートとNotebookを明示コンテキスト／検索scopeとして扱う。Askは読み取り専用で、制限付きAgentは開いているノート本文の単一差分だけを提案する。端末ローカル設定の既定`review-required`では明示適用時だけ、`auto-update`では通常のAgent送信が返した検証済み提案だけを既存のrevision/CAS・保存laneを通して適用する。Web検索は明示確認付きのOpenRouter Web Search／Exaだけを使うProvider管理ツールで、任意の外部操作は許可しない。成功した要約履歴、明示保存した会話・成果物は端末ローカルSQLiteに保存し、WebDAV同期しない。詳細は`docs/development/ai-chat.md`を正とする |

## データ / 状態管理

- ノート本文は Markdown ファイルとして保存する方針。
- ノートのメタデータ、タグ、リンク、同期状態、検索補助情報は SQLite に保存する方針。
- ノート本文のファイル名は安定 ID を使った `note-id.md` とし、ユーザー入力をファイルパスへ直接使用しない。
- SQL 組み立てには Squirrel を使い、直接 SQL 文字列を散らさない。
- フロントエンドの画面状態は Composables と Pinia で管理する。
- AIワークスペースの右側／下側配置、右側幅／下側高さ、非秘密のAgent本文編集権限は`useSettingsStore`の端末UI設定に保持する。保存した寸法は希望値として扱い、狭いウィンドウでは表示時だけ実効寸法を縮小する。AIのmode、入力、追加コンテキスト、timeline、構造化tool trace、生成結果、API Keyは`localStorage`へ保持しない。tool traceは画面メモリだけに置き、SQLite、Markdown、WebDAVへ保存しない。
- アプリ内ショートカットは`KeyboardEvent.code`基準の単一定義と`useSettingsStore`で管理し、version付き端末UI設定として`localStorage`へ保存する。アプリ操作は`App.vue`のcapture listener、本文Undo／Redoは`NoteEditor`のMarkdown履歴とProseMirror historyへ分離してdispatchする。本文履歴はメモリ限定で、ノート切替、外部再読込、競合破棄、モード切替、ロック時に破棄する。詳細は`docs/development/keyboard-shortcuts.md`を正とする。
- Wails API は画面から直接乱用せず、Composables または API クライアント層に寄せる。
- 同期用のhead ETag、manifest/object hash、last-synced base、durable outboxは、ローカルrevisionと操作journalから分離して管理する。詳細は `docs/development/webdav-sync.md` を正とする。

### ノート保存空間

- `ATLAS_NOTE_DATA_DIR`はAtlas Noteの管理ルートとし、ルート直下の既存SQLite・`notes/`・`atlasnote.lock`を移動せず「メイン」として扱う。
- 新しい保存空間は表示名ではなく128 bitの内部IDから`spaces/<ID>/`を導出し、各空間に既存のSQLite、Markdown、`.sync-recovery/`、`atlasnote.lock`を配置する。
- 現在の空間はversion付き`storage-spaces.json`で管理し、短時間の`storage-spaces.lock`と一時ファイル・sync・renameで更新する。不正な台帳を自動上書きしない。
- 実行中のRepository／Serviceは切り替えず、dirty draftのflush、同期の一時停止、AI／同期／インポート／エクスポートbusy確認、対象空間の事前検証後に選択を保存する。現在プロセスのDB・lock解放後にアプリを自動再起動し、選択先を初期化する。
- 設定画面だけで一覧・作成・選択を提供する。削除、改名、外部保存先、暗号化は後続スコープとする。詳細は`docs/development/storage-spaces.md`を正とする。

### SQLite / Markdown の整合性

- 本文を伴う作成・更新は、操作 ID 付き一時ファイルを書き出してから、SQLite のメタデータ変更と `note_storage_operations` への操作記録を同一トランザクションで確定する。
- SQLite 確定後に一時ファイルを `note-id.md` へ置き換え、完了後に操作記録を削除する。
- Markdown の確定に失敗した場合は、SQLite を操作前の状態へ戻す。補償処理まで失敗した場合は操作記録と一時ファイルを残し、次回復旧対象にする。
- 完全削除では Markdown を操作 ID 付き削除ファイルへ退避してから SQLite を削除し、退避ファイルの削除が残った場合は操作記録から再開する。
- アプリ起動時は Wails API を公開する前に未完了操作を復旧し、本文ハッシュ、`content_path`、Markdown の存在を確認する。
- SQLite に対応しない Markdown や一時ファイルは削除せず、`notes/recovery/` へ退避する。
- SQLite レコードに対応する Markdown を復旧できない場合は、SQLite レコードを自動削除せず起動エラーとする。
- 同一プロセス内のノート・ノートブック操作は Service で直列化し、重複する自動保存による世代ずれを防ぐ。
- Sync Serviceはremote比較から受信適用・同期状態commitまでNote Serviceの同期専用ゲートを保持する。同期中に開始されたローカル変更は完了後に実行してoutboxへ残し、remote適用や復旧commitとの競合で消失させない。
- アプリ起動時はSQLiteやMarkdownへアクセスする前に、データディレクトリ直下の `atlasnote.lock` をOSレベルで排他取得する。同じデータディレクトリを使用する2つ目のプロセスはwriterとして初期化しない。
- ロックはアプリ終了時にSQLite接続を閉じてから解放する。ロックファイルの存在自体ではなくOSロックの取得結果で判定し、異常終了後にファイルが残っても次回起動を妨げない。
- 単一writer保証とは別に、整数 `revision` と `expectedRevision` による同一端末内のCASを管理する。端末間の比較には同期用のhead、manifest、object、baseを使用する。
- revision、競合検出、ノート単位保存キューの確定仕様は `docs/development/note-concurrency.md` を正とする。
- ローカル保存キューと同期用durable outboxは分離し、ローカルrevisionを端末間の新旧比較には使用しない。
- 空の同期先を検出した場合は既定ONのフェイルセーフでlocal正本へのremote適用を止める。再アップロードはheadの`If-Match`成功後だけlocal同期状態を更新する。
- remote正本からの全再取得は実行中のDB・notesへ直接適用せず、`.sync-recovery/staging/`の別vaultでhash・payload・SQLite integrityを検証する。次回起動時にデータロック取得後かつSQLite open前に現行vaultを`.sync-recovery/backups/`へ退避してswapし、失敗時はrollbackする。

### バックアップと復元

- バックアップの正本はアクティブ保存空間のSQLiteとMarkdownであり、検索索引などの派生データを別管理しない。自動バックアップは同期排他ゲートと保存空間スナップショットゲートを保持してコピーする。
- バックアップ世代は設定されたアーカイブルートの`.atlasnote-backups/<spaceID>/generations/`へ保存する。manifestのファイル一覧・サイズ・SHA-256を使って追加ファイル、欠落、symlink、パス traversalを検証する。物理保存場所の選択と移行は`docs/development/storage-locations.md`を正とする。
- 復元はプレビュー確認トークンを必要とし、検証済みコピーをstagingへ作ってから`pending.json`で次回起動へ引き渡す。データロック取得後かつSQLite open前に現行データを安全用世代とrollback領域へ退避し、フェーズマーカーによって中断後も再開できるようにする。
- 復元と同期復旧のpending状態は同時に処理せず、復元失敗時は現行データを優先してrollbackする。詳細な保存上限、API、テストは`docs/development/backup-restore.md`を正とする。

### Markdown全文検索

- Markdown本文はファイルを正本とし、SQLite FTS5のcontentful索引は破棄・再構築可能な派生データとする。
- 日本語の部分一致を優先し、FTS5の `trigram` tokenizerを使用する。
- `notes` テーブルに本文カラムを追加しない。
- 索引更新失敗でMarkdown正本の保存をrollbackせず、不整合は検出・再構築する。
- 索引方式、更新タイミング、再構築の確定仕様は `docs/development/search-index.md` を正とする。

### タグとノート関連

- タグはSQLiteの`tags`テーブル、ノートとの多対多関連は`note_tags`テーブルへ保存し、Markdown本文へタグ情報を書き込まない。
- タグ名はServiceでNFC正規化、Unicode空白の正規化、制御文字拒否、100 Unicode文字以内の検証を行う。Unicode case-fold後の`normalized_name`にはUNIQUE制約を置き、表示名の大文字小文字は保持する。
- `note_tags`は`(note_id, tag_id)`複合主キーと両方向の`ON DELETE CASCADE`を使う。タグ別ノート検索に備え、`(tag_id, note_id)`の逆引きINDEXを置く。
- タグの作成・改名・削除、ノートのタグ付与・解除はRepository / Service / Wails API / フロントAPI / Piniaの責務境界を通す。ComponentからWails APIを直接呼ばない。
- タグ操作はMarkdown、`notes.updated_at`、`notes.revision`、FTS5索引、保存操作ジャーナルを変更しない。ゴミ箱内ノートのタグは保持し、UIからの変更だけを無効化する。
- タグの確定仕様、migration、rollback、構造化エラーは `docs/development/tag-design.md` を正とする。タグクリックは単一タグの通常一覧へ遷移し、ノートブック選択および全文検索条件とは同時に保持しない。

### ノートリンク・バックリンク

- ノートリンクは標準Markdownリンクの `atlasnote://note/<32桁小文字hex ID>` を使う。ノートIDをリンク先の正本とし、タイトル変更ではMarkdownのリンクラベルを書き換えない。
- Markdown本文は正本のまま保持し、`note_links`（source / targetの複合主キー）と`note_link_state`（revision・本文hash・mtime）をSQLiteの再構築可能な派生索引として管理する。存在しないリンク先は本文に残すが、索引には登録しない。
- 抽出はコードフェンス、インラインコード、エスケープされたリンク、画像、外部URLを除外し、同一ノートへの重複リンクを1件にまとめる。自己リンク・循環リンクは許可する。
- ノート保存・起動復旧で索引を更新し、索引更新失敗はMarkdown保存をrollbackせず、バックリンク取得をエラーとして通知する。復旧処理で再構築できるようにする。
- バックリンクは対象ノートへの逆引きで、ゴミ箱のsourceノートを除外してページングする。ゴミ箱化・復元では関係を保持し、完全削除では外部キーのCASCADEで関係を削除する。
- リンクの抽出規則、migration、API、Store、UIの確定仕様は実装とテスト（`internal/note/link_*_test.go`、`frontend/scripts/test-note-links.mjs`）を正とする。

## 外部連携

| 連携 | 方針 |
| --- | --- |
| WebDAV | Phase 3で採用する同期方式。コア実装済み・実サーバー受け入れ確認中で、`head`/manifest/object配置、HTTPS/Basic認証、明示的HTTP/TLS/proxy設定、outbox、競合、フェイルセーフ、復旧は `docs/development/webdav-sync.md` を正とする |
| AI API | ユーザー自身の API Key を利用する。初期プロバイダーはOpenRouterとGemini APIで、モデル一覧はプロバイダーから取得する。AI設定のプロバイダーID・モデルID・credential referenceとAPI Key、要約履歴、AI会話、成果物は端末ローカルであり、WebDAV同期しない。接続先は固定HTTPSのみで、proxy・redirect・自動retryは提供しない。Go側Provider adapterは接続確認、モデル一覧、要約、AI司書、AIアシスタント、ライティングを公開し、Gemini APIは保存を伴わない`generateContent`、OpenRouterはZDR・データ収集拒否・下流fallback無効で実行する。要約は選択モデルのコンテキスト長から安全な入力上限を判定し、本文の自動切詰め・分割は行わない。AI設定は下書き・接続確認・明示適用で更新し、外部送信前に保存済み本文とrevisionのsnapshotを確認する。Web検索はProvider能力と実行ごとの明示確認が揃う場合だけ許可し、外部結果を信頼できない入力として扱う |
| OS Keychain | WebDAVパスワードとAI API KeyはCredential Manager / Keychain / Secret Serviceへ別のservice namespaceで保存し、利用不可時はsession限定とする。AI API Keyを`.env`、SQLite、Markdown、`localStorage`へ保存しない |

### 外部Markdownのraw HTML

- Markdown本文は正本のまま保存し、raw HTMLを保存時に削除・書換しない。
- 通常ノートのMarkdownからRichエディタへ変換するときはMarkdownパーサーのHTML解釈を無効にし、raw HTMLを実行可能なDOM要素へ変換しない。
- `<script>`などの要素、`onclick`などのイベント属性、raw HTML内の`javascript:` URLはすべてHTMLとして許可しない。
- 通常のMarkdownリンクと画像URLにもパーサーのURL検証を適用し、危険なURLをリンクまたは画像として生成しない。
- 通常ノートのRichエディタは引き続きraw HTMLを無効にする。AIの回答プレビューだけは、Markdownと既存履歴のHTML混在を受け付けるが、DOMベースの専用サニタイザーで許可タグ・属性・URL schemeを限定してからTiptapへ渡す。
- AIプレビューで許可する要素は見出し、段落、改行、リスト、引用、強調、コード、区切り線、リンクに限定する。`script`、`style`、イベント属性、外部リソース要素、画像、SVG、フォーム、`javascript:`・`data:`・`file:` URLは許可しない。
- AIの保存済み回答は原文を保持し、表示時に毎回サニタイズする。プロンプトのMarkdown出力規則はユーザビリティ向けであり、セキュリティ境界は表示時サニタイズとする。

### Mermaid図のRich表示

- Mermaidは通常ノートのRichエディタで`codeBlock.attrs.language === 'mermaid'`のときだけNodeViewとして表示し、Markdown本文とProseMirrorの永続ノードへ生成SVGを保存しない。
- MarkdownモードとエクスポートはMermaidのコードソースを扱い、AI回答プレビューは対象外とする。raw HTML、raw SVG、`div.mermaid`はMermaid入力として扱わない。
- Mermaidは固定した安全設定と入力上限で描画し、init／frontmatter設定、click／callback、外部画像・アイコン・URLを拒否する。生成SVGは専用サニタイズ後にBlob URLの`img`として表示し、外部参照・イベント・`foreignObject`を許可しない。
- 描画は非同期結果の世代管理とアンマウント時のBlob URL破棄を行い、ノート切替・ロック・テーマ変更時に古い図が残らないようにする。詳細は `docs/development/mermaid.md` を正とする。

### 外部ノートインポート

- md・txt・HTMLのインポートはGo側のOSネイティブファイルダイアログで選択し、フロントエンドからファイルパスを受け取らない。フロントエンドはAPIクライアントとPinia Storeを経由して保存先、タイトル決定方式、構造化結果だけを扱う。
- `internal/noteimport` はUTF-8入力の変換とタイトル決定を担当し、タイトル方式は自動・ファイル名・先頭見出し・メタデータから選ぶ。ノート作成は既存の `note.Service.Create` に委譲する。直接SQLiteやMarkdownファイルへ書き込まないため、操作journal、派生索引、sync outbox、コンテンツロックを迂回しない。
- HTMLはDOM解析後に許可構造だけをMarkdownへ再構築し、`hidden`属性を持つ本文要素と子孫を破棄する。raw HTML、属性、スクリプト、CSS、外部リソース、危険URLを保存しない。通常のmd本文はBOMを除いて保持し、既存のraw HTML表示時安全化に従う。
- 複数ファイルは1ファイル1ノートとし、変換失敗はファイル単位で返す。保存失敗後は成功済みノートを保持して残りを中止する。詳細は `docs/development/note-import.md` を正とする。

### ノートエクスポート

- HTML／PDFエクスポートは、アクティブな単一ノートのdirty draftを保存laneでflushしてから、保存済みMarkdownと`expectedRevision`をsnapshotとして扱う。出力は読み取り専用で、Markdown、SQLite、操作journal、派生索引、sync outboxを変更しない。
- 保存先はGo側のOSネイティブダイアログで選択し、フロントエンドへパスを返さない。ダイアログ中はコンテンツgateを保持せず、選択後の最終本文・revision・lock再検証から同一ディレクトリの一時ファイルを使った原子的確定までexport gateを保持する。コンテンツロックのwriter取得順はexport→AI→通常アクセスとする。
- HTMLはTiptapが生成した断片をGo側でallowlistにより再サニタイズし、CSPと固定CSSを含む自己完結UTF-8文書へ変換する。PDFは`pdfmake` 0.3.11と同梱Noto Sans JPを使い、選択可能な日本語を含むA4縦の文書として直接生成する。外部リソースや画像データは出力しない。
- 保護済み・解除済みノートは暗号化領域外へ平文を作ることを明示警告し、確認済みrequestだけを許可する。本文、形式別payload、保存先フルパスはログへ出さない。詳細は `docs/development/note-export.md` を正とする。

## 未確定事項

- 関連メモ（Phase 4）に必要なデータ構造と更新境界。
