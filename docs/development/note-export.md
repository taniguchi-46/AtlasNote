# ノートエクスポート

最終更新: 2026-08-27

## 目的と対象範囲

- 現在アクティブな単一ノートの本文を、HTMLまたはPDFファイルとして直接出力する。
- ノートタイトルは保存ダイアログの推奨ファイル名と文書メタデータにだけ使用し、本文先頭へ重複して追加しない。
- 添付ファイル、画像の埋め込み、複数ノートの一括出力は対象外とする。画像要素は画像データを出力せず、`alt`文字列だけを本文へ残す。

要求範囲は [`scopes/scope-pre-phase5.md`](scopes/scope-pre-phase5.md) を正とする。

## 正本と責務境界

- エディターのdirty draftを既存のノート単位保存laneでflushし、保存に成功したMarkdownとrevisionだけをエクスポート対象にする。保存失敗または競合時はエクスポートを開始しない。
- Markdownファイルを本文の正本、SQLiteをメタデータ・派生索引の正本とする。エクスポートは読み取り専用であり、Markdown、SQLite、操作journal、検索・リンク索引、sync outboxを変更せず、自動同期も予約しない。
- フロントエンドはAPIクライアントとPinia Storeを経由し、形式、`noteId`、`expectedRevision`、保存済みMarkdownのsnapshot、形式別payloadだけをWails APIへ渡す。ComponentからWails APIやファイルAPIを直接呼ばない。
- 保存先はGo側のOSネイティブ`SaveFileDialog`で選択する。フロントエンドへ保存先パスを返さず、成功結果には出力ファイルのbasenameだけを返す。利用者によるキャンセルと失敗は構造化結果として区別する。
- 同時エクスポートは1件に制限する。エクスポートの準備中と保存ダイアログ表示中を含めてbusyとし、保存空間の切替を拒否する。

## 実行順序と競合検証

1. フロントエンドは対象ノートIDを固定し、共通コンテンツロック解除フローを通す。
2. エディターのdraftをflushし、アクティブノートが変わっていないことを確認する。
3. 保存済みMarkdownからHTML断片またはPDFを生成し、同じMarkdownのsnapshotと`expectedRevision`をGo側へ渡す。
4. Go側は入力形式と上限を事前検証してから、ネイティブ保存ダイアログを表示する。ダイアログ表示中はコンテンツアクセスgateを保持しない。
5. 保存先の選択後、Go側はexportアクセスgateを取得し、既存のNote Serviceから本文を最終読み込みする。現在のrevision、Markdown、ロック状態をrequestのsnapshotと再照合する。
6. 一致する場合だけ、同じgateを保持したまま形式別payloadを検証し、ファイルを原子的に確定する。競合、再ロック、本文差異があればファイルを書き込まず終了する。

`expectedRevision`だけでなくMarkdown snapshotも再照合することで、revision契約を迂回した外部変更や不整合を安全側で検出する。

## 入力上限

| 入力 | 上限 | 超過時 |
| --- | ---: | --- |
| 保存済みMarkdown | 2 MiB | `NOTE_EXPORT_TOO_LARGE` |
| HTML断片 | 8 MiB | `NOTE_EXPORT_TOO_LARGE` |
| Base64復号後のPDF | 32 MiB | `NOTE_EXPORT_TOO_LARGE` |

形式に不要なpayloadが同時に指定された場合は、ファイルダイアログや本文読み込みより前に拒否する。本文とHTML断片は有効なUTF-8として検証する。

## HTMLエクスポート

- 保存済みMarkdownを通常のRichエディターと同じTiptap設定でHTML断片へ変換する。raw HTMLは既存方針どおり実行可能な要素として解釈しない。
- Go側は`golang.org/x/net/html`で断片を再解析し、見出し、段落、改行、太字、斜体、取り消し線、引用、リスト、コード、区切り線、単純表、許可URLのリンクだけをallowlistで再構築する。
- リンクは`https`、`http`、`mailto`、`tel`、有効な`atlasnote://note/<ID>`、ページ内アンカーだけを許可する。イベント属性、`class`、`style`およびその他の属性は破棄する。
- `script`、`style`、`iframe`、埋め込み要素、SVG、canvas、音声・動画、フォーム、外部画像などは出力しない。画像は`alt`文字列だけを保持する。
- 出力は`<!doctype html>`、UTF-8、`lang="ja"`、viewport、制限付きCSP、埋め込み固定CSSを持つstandalone HTMLとする。外部CSS、JavaScript、画像、フォントを取得せずオフラインで表示できるようにする。

## PDFエクスポート

- PDF生成には`pdfmake` 0.3.11（MIT）を使用する。共通の依存追加禁止方針に対するPDF限定の承認済み例外とする。
- 日本語表示にはNoto Sans JP Regular／Boldをアプリへ同梱し、PDFへ埋め込む。フォントはSIL Open Font License 1.1に従い、ライセンス本文も配布物へ含める。実行時の外部フォント取得は行わない。
- Tiptapの安全な文書モデルからpdfmakeの文書定義へ変換し、A4縦、固定余白、白背景で直接PDFを生成する。日本語本文は文字として埋め込み、選択・検索できる状態を維持する。
- 対応構造は見出し、段落、太字、斜体、取り消し線、リスト、タスクリスト、引用、インラインコード、コードブロック、区切り線、単純表、許可URLのリンクとする。未対応要素は安全なテキストへフォールバックする。
- 画像データは取り込まず、`alt`文字列だけを保持する。外部リソースを取得しない。
- フロントエンドは生成したPDFをBase64 payloadとしてGo側へ渡す。Go側は復号後の上限、`%PDF-`ヘッダー、終端`%%EOF`、revisionとMarkdown snapshotを再検証してから保存する。

## コンテンツロックと平文警告

- ロック中のノートは共通解除gateを通し、Go側でも保存直前にロック状態を再検証する。保存ダイアログ表示中に自動ロックした場合はエクスポートを拒否する。
- 保護対象でも解除済みのノートをエクスポートすると、暗号化された保存空間の外に平文ファイルが作成される。形式ごとに明示警告を表示し、利用者が確認したrequestだけを許可する。Go側でも確認フラグを必須とする。
- exportアクセスgateは、保存先選択後の最終本文読み込みからファイル確定まで保持する。ロック有効化・無効化などのwriter操作はexport→AI→通常アクセスの順でgateを取得し、デッドロックと平文書き込み途中の再暗号化を防止する。
- 自動ロックの固定期限はエクスポート操作で延長しない。

## ファイル保存

- 推奨ファイル名はノートタイトルから制御文字、パス区切り、OS予約名などを除去して生成し、空の場合は`note.html`または`note.pdf`とする。利用者が拡張子を省略した場合は選択形式の拡張子を補う。
- 選択先と同じディレクトリに一時ファイルを作成し、write、file sync、closeを完了してから置換する。失敗時は一時ファイルを片付け、確定前の既存ファイルを変更しない。
- 本文、HTML／PDF payload、保存先フルパス、保護内容をログへ出力しない。結果とログに必要な場合もbasenameと構造化エラーだけを扱う。

## 結果とエラー

| コード | 意味 |
| --- | --- |
| `NOTE_EXPORT_BUSY` | 別のエクスポートが実行中 |
| `NOTE_EXPORT_INVALID_INPUT` | note ID、revision、snapshot、形式別payloadなどが不正 |
| `NOTE_EXPORT_INVALID_FORMAT` | HTML／PDF以外の形式が指定された |
| `NOTE_EXPORT_NOTE_NOT_FOUND` | 保存前の最終確認で対象ノートが存在しない |
| `NOTE_EXPORT_LOCKED` | 対象ノートが未解除または保存前に再ロックされた |
| `NOTE_EXPORT_STALE` | revisionまたはMarkdown snapshotが現在の正本と一致しない |
| `NOTE_EXPORT_PROTECTED_CONFIRMATION_REQUIRED` | 保護本文の平文出力が明示確認されていない |
| `NOTE_EXPORT_TOO_LARGE` | Markdown、HTML、PDFの上限を超えた |
| `NOTE_EXPORT_RENDER_FAILED` | HTMLの安全化またはPDFの生成・検証に失敗した |
| `NOTE_EXPORT_WRITE_FAILED` | 一時ファイル作成、書き込み、sync、close、置換に失敗した |
| `NOTE_EXPORT_UNAVAILABLE` | ServiceまたはWails APIを利用できない |

保存ダイアログのキャンセルはエラーコードを持たない`cancelled: true`として返す。成功時は`cancelled: false`と`exportedName`だけを返し、フルパスを含めない。

## 確認

自動テストでは次を確認する。

- 入力形式、UTF-8、各サイズ上限、形式別payload、PDFヘッダー・終端の検証。
- HTMLのタグ・属性・URL allowlist、raw HTML、外部画像、スクリプト、イベント属性、CSP、タイトルescape。
- 日本語、空本文、各Rich構造、長文・複数ページ、表・コード・リンクを含むHTML／PDF生成。
- 保存ダイアログのキャンセル、推奨ファイル名、拡張子補完、原子的置換、失敗時cleanupと既存ファイル保護。
- revision／Markdown競合、未解除ロック、保護本文の確認、ダイアログ中の自動ロック、同時実行拒否。
- 保存前flush失敗・競合時の中止、アクティブノート変更、エクスポート中の保存空間切替拒否。
- エクスポート前後でMarkdown、SQLite、sync outbox、操作journalが変化しないこと。

```bash
go test ./internal/noteexport ./internal/contentlock ./internal/note -count=1
go test ./... -count=1
npm --prefix frontend run test:note-export
npm --prefix frontend run test:storage-spaces
npm --prefix frontend run test:content-locks
npm --prefix frontend run test:serializer
npm --prefix frontend run test:markdown-safety
npm run frontend:typecheck
npm run frontend:build
wails build -clean
```

手動確認ではHTMLのオフライン表示、PDFの日本語選択・複数ページ、空ノート、保存キャンセル・上書き・書き込み不能、保護警告、解除キャンセル、保存ダイアログ中の自動ロックを確認する。

## 対象外

- 複数ノートの一括出力、ノートブック単位の出力。
- 添付ファイル、画像データ、外部CSS・JavaScript、同梱Noto Sans JP以外のフォントの埋め込みまたは取得。
- カスタムテンプレート、カスタムCSS、用紙・余白・印刷設定の変更。
- Mermaid、LaTeX、PDF暗号化・パスワード保護、電子署名。
- 出力ファイルの自動同期、履歴管理、エクスポート後の自動起動。
