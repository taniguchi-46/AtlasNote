# Phase 4 TODO：v2

決定状態: 正式決定（2026-07-27）／進捗分類更新（2026-08-01）

## TODOの目的

v1で確立した安全な単発要約を基盤に、AI司書、ストリーミング、部分応答、キャンセル、構造化出力を追加する。v2ではAI履歴・生成成果物を永続化せず、Phase 3 WebDAV契約と既存のローカル保存契約を維持する。

詳細スコープは [`scope-phese4-v2.md`](../development/scopes/scope-phese4-v2.md)、v1の前提は [`scope-phese4.md`](../development/scopes/scope-phese4.md)、v3の後続範囲は [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md) を正とする。

## 進捗分類（2026-08-01）

- `【実装済み】`: 対応するコードまたはテストが存在し、実装の根拠を確認できる項目。
- `【未検証】`: コードの一部または対応方針は存在するが、必要な異常系・手動受け入れ・境界テストの記録が不足している項目。

v2は主要なAI司書機能、単一チャットtimelineへの統合、ローカル自動テスト・Wails clean buildを確認済みである。利用者キャンセル、候補採用時の競合、キーボード操作、AI失敗後の既存機能継続の手動受け入れ、および最新差分に対するCI記録が残るため、v2完了とは判定しない。

## v2開始条件

- [x] 【実装済み】v1の秘密情報保存テストと手動受け入れ記録を完了する。
- [x] 【実装済み】v1のOpenRouter／Gemini、CredentialStore、safe error、送信前確認の契約を回帰確認する。
- [x] 【実装済み】v2の構造化出力、部分応答、キャンセル、適用境界をレビュー承認する。
- [x] 【実装済み】v2ではDB schema、migration、AI用WebDAV entityを追加しないことを確認する。

## 1. 実行基盤

- [x] 【実装済み】ストリーミングのrequest／response契約と途中切断時の破棄条件を決める。OpenRouter／GeminiのSSE parserと不完全応答テストを実装済み。
- [x] 【実装済み】Merian部分応答の表示状態、完了、失敗、キャンセル、staleの状態を決める。Go event phaseとPinia stateに反映済み。
- [ ] 【未検証】利用者キャンセルがProvider、Application Service、Wails、Pinia、UIまで伝播することを確認する。各層のコードと単体テストはあるが、通しの手動受け入れ記録がない。
- [x] 【実装済み】タイトル、タグ、分類、関連候補用の構造化出力schemaと安全なparse errorを決める。Go側のschema生成・正規化・safe errorを実装済み。
- [x] 【実装済み】retryなし、fallbackなし、秘密情報非露出、同時実行上限を維持する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で生成lockを含むGo testが成功した。

## 2. AI司書

- [x] 【実装済み】タイトル候補生成を実装する。
- [x] 【実装済み】タグ候補生成と既存タグとの重複整理を実装する。
- [x] 【実装済み】分類候補生成を実装する。自動ノートブック移動は行わない。
- [x] 【実装済み】関連メモの判定基準を決める。検索・バックリンク由来の候補poolと関連度を実装済み。
- [x] 【実装済み】AIを利用した関連メモの抽出方式を決める。構造化候補をProvider adapterから取得する。
- [x] 【実装済み】関連メモ候補を実装する。
- [ ] 【一部自動検証済み】関連度が低い、候補がない、大量ノートの場合の挙動をテストする。低関連度・空候補のGoテストに加え、正常な候補0件をWails mockからFrontend実Store・timelineまで2026-08-24に確認した。大量ノートと手動UI受け入れは未確認。
- [x] 【実装済み】重複メモ候補を実装する。自動削除・自動統合は行わない。

## 3. 明示適用・データ保全

- [x] 【実装済み】候補のプレビュー、個別採用、破棄、手動retryを実装する。
- [x] 【実装済み】タイトル、タグ、分類、リンクの採用操作を既存Serviceとrevision/CASへ接続する。
- [ ] 【未検証】保存失敗、revision競合、ノート切替、キャンセル時に候補を適用しない。実装上のガードはあるが、候補採用の統合テストと手動受け入れが不足。
- [ ] 【未検証】AI候補、途中応答、プロンプト、生成結果、履歴をMarkdown、SQLite、操作journal、検索索引、WebDAV outboxへ保存しないことを確認する。PiniaのlocalStorage非使用テストはあるが、全保存境界の検証記録が不足。

## 4. UI・受け入れ

- [x] 【実装済み】生成中、部分応答、キャンセル、成功、失敗、空結果、stale、競合をinlineで表示する。
- [x] 【実装済み】AI司書5操作を単一チャットtimelineの下部コンポーザーへ統合する。開いているノートの固定context chip、`＋`メニュー、Ask／Agent切替、入力欄内右下の送信、tool trace直後の候補カード、右側／下側配置・ドラッグ寸法・狭幅表示を実装し、`test:ai-chat`と`test:ai-workspace`で回帰確認した（2026-08-01）。
- [ ] 【未検証】キーボードでキャンセル、採用、破棄、retryを操作できることを確認する。標準buttonによる操作はあるが、専用の手動記録・テストがない。
- [x] 【実装済み】`test:ai-librarian`相当の既存Node script方式テストを維持・拡張し、単一チャットのcontext、tool trace、送信lock、未採用結果の置換防止を`test:ai-chat`／`test:ai-workspace`へ追加した。2026-08-01にローカルのFrontend・Go回帰テストとWails clean buildで成功を確認した。
- [x] 【実装済み】Go test doubleでstream、cancel、partial、structured output、safe error、非保存境界を検証する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で生成lockを含むGo testが成功した。
- [x] 【自動検証済み】AI失敗後もローカル保存、編集、検索、既存同期outbox更新が継続することをApp統合テストで確認した（2026-08-23）。
- [ ] 【未検証】AI失敗後のローカル保存、編集、検索、既存同期の継続を実画面で手動受け入れする。

## v2完了条件

- [ ] 【未検証】v2の詳細スコープに記載したAI司書5機能を実装・検証する。5操作の実装は済みだが、統合・手動検証が未完了。
- [ ] 【未検証】関連メモの4つの未完了項目を完了する。関連・重複候補の実装はあるが、判定境界と大量ノートの受け入れが未確認。
- [x] 【実装済み】ストリーミング、部分応答、キャンセル、構造化出力の異常系を検証する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で関連テストが成功した。
- [x] 【実装済み】DB schema、migration、AI用WebDAV entityを変更せずに完了する。v2実装コミットではAI保存用schema・migration・WebDAV entityを追加していない。
- [ ] 【一部自動検証済み】CI、手動受け入れ、受け入れ記録を完了する。commit `6e7a2dfbd6e8f4a27f74355460714951c7db8b7d`までのチャット刷新、Agent編集権限UI分岐、cancel／timeout応答差分はCI run [#32666344532](https://github.com/taniguchi-46/AtlasNote/actions/runs/32666344532) で全工程成功した。後続の空結果・長文テストとAssistant／Agent利用者停止差分のCI反映、手動受け入れ記録は未完了である。

## 2026-08-01の仕様決定

- [x] 【決定済み】Web検索を、明示確認付きのProvider管理ツールとして正式scopeへ含める。実装済みの安全契約は`ai-chat.md`、Phase 4での正式対象はv3 scopeを正とする。
- [x] 【決定済み】UI上の`Agent`は制限付きAgentモードとして正式化する。任意のAIエージェントや外部サービス操作は許可せず、将来の書き込み・編集は差分確認と利用者の明示適用を必須にする。実装タスクはv3 TODOで追跡する。

## 絶対遵守事項

- 実キー、実endpoint、実ノート本文、raw provider errorをテスト・ログ・受け入れ記録へ持ち込まない。
- 自動上書き、自動削除、自動リンク作成、自動分類を行わない。
- v3の履歴保存・AIアシスタント・ライティングをv2へ前倒ししない。
