# Mermaid対応

## 対象範囲

- 通常ノートのRich（WYSIWYG）エディタで、`mermaid`言語付きコードフェンスを図として表示する。
- Markdownモードでは、従来どおり` ```mermaid `コードフェンスを編集する。
- Rich表示では編集可能なMermaidソースと、読み取り専用の図を併記する。
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
- Mermaidの描画はNodeView内に閉じ、図のHTML／SVGをProseMirror文書へ挿入しない。
- 描画は短いデバウンスを挟み、古い非同期結果を世代番号で破棄する。ノート切替、ロック、アンマウント後にDOMを更新しない。
- ライト／ダークテーマ変更時は現在のソースを再描画する。

## 安全性契約

- Mermaidはnpm依存として遅延読込し、`startOnLoad: false`、`securityLevel: "strict"`、`htmlLabels: false`、`suppressErrorRendering: true`を固定する。
- `maxTextSize: 50000`、`maxEdges: 500`を描画設定へ明示する。
- init／frontmatter設定、`click`／callback、外部URL・画像・アイコン記法は入力段階で拒否する。
- 生成SVGは専用のサニタイズを通し、スクリプト、`foreignObject`、画像、イベント属性、外部参照を許可しない。
- サニタイズ済みSVGはBlob URLで`img`へ表示し、再描画・アンマウント時にURLを破棄する。
- 構文や本文をログへ出さず、エラーは利用者向けの固定メッセージへ変換する。

## 完了条件

- 有効なMermaidコードフェンスがRich表示で図になる。
- 通常のコードブロックとMarkdownモードの挙動が変わらない。
- Markdown→Rich→Markdownでソースと`mermaid`言語名が保持される。
- 空・不正・制限超過・安全性検証失敗時にソースを失わず、図単位でフォールバックする。
- 複数図、テーマ変更、連続編集、ノート切替、ロック、アンマウントで古い描画が残らない。
- 外部リソース、危険なSVG、任意イベントが実行・取得されない。
- 対象テスト、Frontend typecheck／build、既存のMarkdown・保存・エクスポート回帰テストが成功する。

Mermaidの設定・APIは[公式Usage](https://mermaid.js.org/config/usage)、セキュリティ設定は[securityLevel](https://mermaid.js.org/config/schema-docs/config-properties-securitylevel.html)を参照する。
