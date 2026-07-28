# Phase 4 TODO：v2

決定状態: 正式決定（2026-07-27）

## TODOの目的

v1で確立した安全な単発要約を基盤に、AI司書、ストリーミング、部分応答、キャンセル、構造化出力を追加する。v2ではAI履歴・生成成果物を永続化せず、Phase 3 WebDAV契約と既存のローカル保存契約を維持する。

詳細スコープは [`scope-phese4-v2.md`](../development/scopes/scope-phese4-v2.md)、v1の前提は [`scope-phese4.md`](../development/scopes/scope-phese4.md)、v3の後続範囲は [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md) を正とする。

## v2開始条件

- [x] v1の秘密情報保存テストと手動受け入れ記録を完了する。
- [x] v1のOpenRouter／Gemini、CredentialStore、safe error、送信前確認の契約を回帰確認する。
- [x] v2の構造化出力、部分応答、キャンセル、適用境界をレビュー承認する。
- [x] v2ではDB schema、migration、AI用WebDAV entityを追加しないことを確認する。

## 1. 実行基盤

- [ ] ストリーミングのrequest／response契約と途中切断時の破棄条件を決める。
- [ ] 部分応答の表示状態、完了、失敗、キャンセル、staleの状態を決める。
- [ ] 利用者キャンセルがProvider、Application Service、Wails、Pinia、UIまで伝播することを確認する。
- [ ] タイトル、タグ、分類、関連候補用の構造化出力schemaと安全なparse errorを決める。
- [ ] retryなし、fallbackなし、秘密情報非露出、同時実行上限を維持する。

## 2. AI司書

- [ ] タイトル候補生成を実装する。
- [ ] タグ候補生成と既存タグとの重複整理を実装する。
- [ ] 分類候補生成を実装する。自動ノートブック移動は行わない。
- [ ] 関連メモの判定基準を決める。
- [ ] AIを利用した関連メモの抽出方式を決める。
- [ ] 関連メモ候補を実装する。
- [ ] 関連度が低い、候補がない、大量ノートの場合の挙動をテストする。
- [ ] 重複メモ候補を実装する。自動削除・自動統合は行わない。

## 3. 明示適用・データ保全

- [ ] 候補のプレビュー、個別採用、破棄、手動retryを実装する。
- [ ] タイトル、タグ、分類、リンクの採用操作を既存Serviceとrevision/CASへ接続する。
- [ ] 保存失敗、revision競合、ノート切替、キャンセル時に候補を適用しない。
- [ ] AI候補、途中応答、プロンプト、生成結果、履歴をMarkdown、SQLite、操作journal、検索索引、WebDAV outboxへ保存しないことを確認する。

## 4. UI・受け入れ

- [ ] 生成中、部分応答、キャンセル、成功、失敗、空結果、stale、競合をinlineで表示する。
- [ ] キーボードでキャンセル、採用、破棄、retryを操作できることを確認する。
- [ ] `test:ai-librarian`相当の既存Node script方式テストを追加する。Vue用の新規テスト依存は追加しない。
- [ ] Go test doubleでstream、cancel、partial、structured output、safe error、非保存境界を検証する。
- [ ] AI失敗後もローカル保存、編集、検索、既存同期が継続することを受け入れる。

## v2完了条件

- [ ] v2の詳細スコープに記載したAI司書5機能を実装・検証する。
- [ ] 関連メモの4つの未完了項目を完了する。
- [ ] ストリーミング、部分応答、キャンセル、構造化出力の異常系を検証する。
- [ ] DB schema、migration、AI用WebDAV entityを変更せずに完了する。
- [ ] CI、手動受け入れ、受け入れ記録を完了する。

## 絶対遵守事項

- 実キー、実endpoint、実ノート本文、raw provider errorをテスト・ログ・受け入れ記録へ持ち込まない。
- 自動上書き、自動削除、自動リンク作成、自動分類を行わない。
- v3の履歴保存・AIアシスタント・ライティングをv2へ前倒ししない。
