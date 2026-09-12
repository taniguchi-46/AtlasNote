# Mermaid対応

## 対象範囲

- 通常ノートのRich（WYSIWYG）エディタで、`mermaid`言語付きコードフェンスを図として表示する。
- Markdownモードでは、従来どおり` ```mermaid `コードフェンスを編集する。
- Rich表示ではソースを画面上に常時表示せず、読み取り専用の図だけを表示する。図の「編集」から開くダイアログでソースとプレビューを編集する。
- 構文エラーや安全性検証の失敗時もソースを保持し、図の領域だけへエラーを表示する。
- Mermaidの生成SVGは表示時だけ作成し、Markdown、SQLite、同期、バックアップへ保存しない。

## 対象外

- AI回答プレビューでのMermaid描画
- HTML／PDFエクスポートへの図の埋め込み（既存仕様どおりコードソースを出力）
- raw HTML、raw SVG、`<div class="mermaid">`による描画
- Mermaidのクリック操作、外部画像、外部アイコン、CDN、外部フォント
- 専用の図作成ウィザードや新しい永続ノード型

## 表示契約

- 既存のTiptap `codeBlock`ノードと`language`属性を使う。保存形式とserializerの契約は変更しない。
- `language`が`mermaid`（大文字小文字を区別しない）の場合だけRich表示へ図を追加する。
- Mermaidの描画はNodeView内に閉じ、図のHTML／SVGをProseMirror文書へ挿入しない。NodeViewのcontentDOMにあるソースは編集状態を保持するためだけに使い、通常の画面では視覚的に隠す。
- 編集ダイアログの保存は対象NodeViewの位置、ノード内容、アクティブノートID、エディタ世代を再検証する。条件が変わった場合は保存せず閉じる。成功時はソーステキストだけを1トランザクションで置換し、通常のTiptap履歴とautosaveへ渡す。
- コンテンツロック前は、開いている編集ダイアログのDOM値を回収してTiptap本文へ反映してからdraftをflushする。入力ガードはdraft保存完了後のロックAPIとロック後の表示反映まで保持し、保存／ロック失敗時に解除する。IMEのcompositionendを待てないロックでも入力を失わない。
- 描画は短いデバウンスを挟み、古い非同期結果を世代番号で破棄する。ノート切替、ロック、アンマウント後にDOMを更新しない。
- ライト／ダークテーマ変更時は現在のソースを再描画する。

## クリップボード契約

- Rich貼り付けは、`text/plain`のMermaidコードフェンス、Mermaidの宣言で始まるraw source、または`text/html`の`code.language-mermaid`／`data-language="mermaid"`だけを対象とする。
- フェンス付きMarkdownは周囲の段落・コード・表を含めて通常の安全なMarkdown→Rich変換へ渡す。raw sourceは`language: "mermaid"`の既存`codeBlock`へ変換する。
- raw source／末尾がMermaidのfenced Markdownの貼り付け後は、非編集のソースcontentDOMへ選択を残さず、図の直後の段落へカーソルを移す。段落がなければ空段落を作成する。通常のコードブロックの選択と周囲の複数ブロックは変更しない。
- SVG、`div.mermaid`、Mermaid以外の言語付きHTMLコードはMermaid入力として扱わない。HTMLにMermaidコードがない場合だけ、plain textの判定へフォールバックする。
- RichでMermaid図をコピーした場合のplain textは、ソースをバッククォートフェンスで包んだMarkdownとする。ソース内のバッククォート列より長いフェンスを選び、通常のコードブロックと表のコピー契約は変更しない。
- Mermaidの貼り付け・コピーは本文のcanonical Markdownを変更する特別な保存形式を追加せず、通常の`onUpdate`、Undo、autosaveを通す。

## 安全性契約

- Mermaidはnpm依存として遅延読込し、`startOnLoad: false`、`securityLevel: "strict"`、`htmlLabels: false`、`suppressErrorRendering: true`を固定する。
- `maxTextSize: 50000`、`maxEdges: 500`を描画設定へ明示する。
- init／frontmatter設定、`click`／callback、外部URL・画像・アイコン記法は入力段階で拒否する。
- YAML／JSONのキーやURLを部分的に判定せず、`@{...}`メタデータとsequenceの`properties`／`details`を構文の入口で拒否する。画像以外の拡張shape・edge設定を含む`@{...}`も対象となる。従来の角括弧・丸括弧・菱形などのノード記法は利用できる。
- directive、リンク／アイコン、raw HTML（`<br/>`を含む）、Markdown画像、CSSの`url()`／`@import`を拒否する。style／classDef／linkStyle内のCSSエスケープ・コメントも拒否する。clickなどの命令は改行・セミコロン・sequenceDiagramヘッダー直後の文境界で判定し、通常ラベル中の単語は許可する。メタデータ・URLなどの禁止トークンはコメントやラベル内でも保守的に拒否し、原文は変更しない。
- 生成SVGは専用のサニタイズを通し、スクリプト、`foreignObject`、画像、イベント属性、外部参照を許可しない。
- SVG名前空間`http://www.w3.org/2000/svg`は外部リソースURLと区別して保持・補完し、シリアライズ後もXMLとして再解析できることを確認する。
- サニタイズ済みSVGはBlob URLで`img`へ表示し、再描画・アンマウント時にURLを破棄する。
- 構文や本文をログへ出さず、エラーは利用者向けの固定メッセージへ変換する。

## 完了条件

- 有効なMermaidコードフェンスがRich表示で図になる。
- Rich表示が図のみとなり、編集ダイアログからソースを変更できる。通常のコードブロックとMarkdownモードの挙動が変わらない。
- Markdown→Rich→Markdownでソースと`mermaid`言語名が保持される。
- fenced/raw/HTMLの貼り付けとMermaidソースのコピーが往復し、通常のコード・表のコピーや貼り付けを横取りしない。
- 空・不正・制限超過・安全性検証失敗時にソースを失わず、図単位でフォールバックする。
- 複数図、テーマ変更、連続編集、ノート切替、ロック、アンマウントで古い描画が残らない。
- 外部リソース、危険なSVG、任意イベントが実行・取得されない。
- 対象テスト、Frontend typecheck／build、既存のMarkdown・保存・エクスポート回帰テストが成功する。

Mermaidの設定・APIは[公式Usage](https://mermaid.js.org/config/usage)、セキュリティ設定は[securityLevel](https://mermaid.js.org/config/schema-docs/config-properties-securitylevel.html)を参照する。

## 過去の回帰検証（2026-09-10〜11）

- 11.17.2の[imageSquare実装](https://github.com/mermaid-js/mermaid/blob/mermaid%4011.17.2/packages/mermaid/src/rendering-util/rendering-elements/shapes/imageSquare.ts)はSVG出力前に画像を取得するため、出力サニタイズだけでは防げない。引用キー、YAML／JSONエスケープ、相対URL、sequence画像プロパティ、単独CR／セミコロン区切りなど54入力を、Mermaidのinitialize／parse／renderより前に拒否するテストを追加した。
- `npm --prefix frontend run test:mermaid`は実Mermaidの正常描画、SVG名前空間、外部参照除去と、実NodeViewをコンパイルした遅延応答テストを実行する。テーマ変更・連続編集・ノート内容置換・ロックに伴うNodeView破棄・アンマウント後の結果破棄とBlob URL破棄を確認する。Wails画面全体の手動受け入れとは区別する。
- 同コマンドには実Tiptap統合テストも含め、fenced/raw/HTMLクリップボード判定、周辺ブロック保持、`codeBlock`化、貼り付け後のEnter、ソース置換のUndo、動的フェンスを確認する。
- 実Chromiumでは外部通信をrouteで遮断・監視し、禁止入力54件のAPI呼び出し・画像取得・外部リクエスト0件、およびフローチャート／シーケンス図のlight／dark全4画像のBlob読み込みを確認した。
- serializer、Markdown safety、auto-save、note-export、Frontend typecheck／build、Wails buildは成功。buildの既存依存由来のannotation／chunk-size警告は残る。

## 今回の再確認（2026-09-12）

- `npm run frontend:typecheck`、`npm --prefix frontend run test:mermaid`、`npm --prefix frontend run test:note-delete`、`npm --prefix frontend run test:serializer`が成功した。Mermaidテストには実コンポーネントをコンパイルしたダイアログ／NodeView接続、IME中のbefore-lock回収、外部source同期、Tiptapのraw／fenced貼り付け・Enter・コピー・Undo、動的フェンスを含む。
- `test:auto-save`、`test:note-operation-queue`、`test:note-batch`、`test:note-selection`、`test:table-copy`、`test:markdown-safety`、`test:note-export`、`test:content-locks`の関連回帰テストが成功した。コンテンツロックは遅延draft保存／API、保存失敗・保存例外・API失敗・API例外、同時ロック要求の入力ガード復旧も確認した。
- `npm --prefix frontend run build`によるFrontend production buildが成功した。既存依存由来のannotation／chunk-size警告は残る。
- Wailsの実画面全体、実ブラウザーでの手動UI受け入れ、今回の変更に対するGoテストは今回実施していない。

実ブラウザー検証はプロジェクト依存を追加せず、別ターミナルでローカルViteを起動して再実行できる。

```powershell
npm --prefix frontend run dev -- --host 127.0.0.1 --port 5187 --strictPort
npx --yes --package @playwright/cli playwright-cli -s=mermaid-fix open about:blank
npx --yes --package @playwright/cli playwright-cli -s=mermaid-fix run-code --filename frontend/scripts/mermaid-browser-run.cjs
npx --yes --package @playwright/cli playwright-cli -s=mermaid-fix close
```

runnerはアプリを起動しない空ページを返し、ローカルの検証モジュールだけを読み込む。Viteの初回依存最適化でページが再読込された場合は、最適化完了後にrun-codeを再実行する。
