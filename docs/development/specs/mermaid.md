# Mermaid機能のブランチ分離

最終更新: 2026-09-20

| ブランチ | 内容 |
| --- | --- |
| `codex/pre-phase5-future-features` | 通常開発用。Mermaid専用機能なし |
| `codex/mermaid-full` | 描画・挿入・ソース編集・かんたん編集を保持 |

Mermaidの描画・挿入・専用編集は `codex/mermaid-full` に分離した。通常開発用の `codex/pre-phase5-future-features` では、既存のMermaidフェンスを通常のコードブロックとして表示・編集し、ソースを保存する。

分離時の未コミット変更11ファイルは、対応ブランチのコミット `eafb13e` に保持した。分離前の詳細仕様・専用テストも同ブランチに残す。

通常開発ブランチではMermaid専用コンポーネント・描画処理・依存パッケージ・CIの専用テストを外す。Markdown serializer、保存形式、DB、同期、バックアップは変更しない。既存ノートのMermaidソースは削除・変換しない。診断履歴の旧Mermaidエラー識別子はGo側で引き続き受理する。

作業ツリーに未保存の変更がないことを確認してから、`git switch codex/mermaid-full` で対応版へ、`git switch codex/pre-phase5-future-features` で通常版へ切り替える。切替後は `npm --prefix frontend ci` で依存関係を合わせる。

通常版ではserializer・Markdown安全性・表コピー・自動保存・コンテンツロックの回帰を確認する。対応版では加えて `npm --prefix frontend run test:mermaid` を実行する。既存の `main` と `codex/mermaid-support` は変更しない。
