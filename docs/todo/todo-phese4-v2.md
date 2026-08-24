# Phase 4 TODO：v2

決定状態: 正式決定（2026-07-27）／進捗分類更新（2026-08-01）／自動・手動受け入れ完了（2026-08-24）

## TODOの目的

v1で確立した安全な単発要約を基盤に、AI司書、ストリーミング、部分応答、キャンセル、構造化出力を追加する。v2ではAI履歴・生成成果物を永続化せず、Phase 3 WebDAV契約と既存のローカル保存契約を維持する。

詳細スコープは [`scope-phese4-v2.md`](../development/scopes/scope-phese4-v2.md)、v1の前提は [`scope-phese4.md`](../development/scopes/scope-phese4.md)、v3の後続範囲は [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md) を正とする。

## 進捗分類（2026-08-24）

- `【実装済み】`: 対応するコードまたはテストが存在し、実装の根拠を確認できる項目。
- `【自動検証済み】`: 実Providerを使わない統合・境界テストまで完了し、手動確認を別項目で管理する項目。
- `【一部自動検証済み】`: 自動テストは完了したが、実画面での手動受け入れが残る項目。
- `【未検証】`: コードの一部または対応方針は存在するが、必要な異常系・手動受け入れ・境界テストの記録が不足している項目。

v2は主要なAI司書機能、単一チャットtimelineへの統合、大量候補pool、候補採用異常系、全保存境界、キーボード操作契約の自動検証を完了した。最終実装差分を含むcommit `e8f6816f60e61c4de149aaa45f778812c0ad86a8`はCI run [#32722645563](https://github.com/taniguchi-46/AtlasNote/actions/runs/32722645563) で全工程成功し、実画面の手動UI受け入れも利用者が「現状OK」と確認したため、2026-08-24付でv2完了と判定する。

## 2026-08-24 Codex自動タスク

- [x] commit `d56e6c0b640be86b1da25ecb0d6412dfad725b88` の差分を、仕様・データ保全・異常系・テスト観点で正式レビューした。生成中revision変更とcancel応答待ち中のclearに関する2件の競合を検出し、request追跡を終端まで維持してstale結果を破棄する修正と再現テストを追加した。
- [x] AI司書の大量ノート候補poolについて、20件上限、上限超過拒否、重複・current note・不正ID除外、snippet上限、要求候補数を超える結果の拒否を実Providerなしの自動テストで確認した。
- [x] 候補採用時の保存失敗、revision競合、ノート切替、キャンセルで、候補が暗黙適用されないことを実Pinia Store・実Note Store・Wails mockのFrontend統合テストで確認した。
- [x] AI候補、途中応答、プロンプト、生成結果、履歴がMarkdown、SQLite、操作journal、検索索引、WebDAV outboxへ保存されないことを一時DB・data directoryの前後不変性とmarker非保存で確認した。
- [x] キーボードでキャンセル、採用、破棄、retryへ到達できるnative button、accessible name、handler、busy guard、Enter／Space既定動作の契約を自動テストで確認した。
- [x] Go全体、Frontend全22 script、typecheckを含むproduction build、差分検査を完了した。CI run [#32700754252](https://github.com/taniguchi-46/AtlasNote/actions/runs/32700754252) はcommit `d56e6c0b640be86b1da25ecb0d6412dfad725b88`を対象にWails clean buildを含む全工程で成功している。

実キー・実endpoint・実ノート本文を使わずに上記の自動テストと全体回帰が成功したため、この節のCodex自動タスクは完了とする。後続差分はcommit `e8f6816f60e61c4de149aaa45f778812c0ad86a8`としてCI全工程の成功を確認し、実画面の手動UI受け入れは利用者確認により完了した（2026-08-24）。

## v2開始条件

- [x] 【実装済み】v1の秘密情報保存テストと手動受け入れ記録を完了する。
- [x] 【実装済み】v1のOpenRouter／Gemini、CredentialStore、safe error、送信前確認の契約を回帰確認する。
- [x] 【実装済み】v2の構造化出力、部分応答、キャンセル、適用境界をレビュー承認する。
- [x] 【実装済み】v2ではDB schema、migration、AI用WebDAV entityを追加しないことを確認する。

## 1. 実行基盤

- [x] 【実装済み】ストリーミングのrequest／response契約と途中切断時の破棄条件を決める。OpenRouter／GeminiのSSE parserと不完全応答テストを実装済み。
- [x] 【実装済み】Merian部分応答の表示状態、完了、失敗、キャンセル、staleの状態を決める。Go event phaseとPinia stateに反映済み。
- [x] 【受入済み】利用者キャンセルがProvider、Application Service、Wails、Pinia、UIまで伝播することを、自動テストと利用者による実画面の手動UI受け入れで確認した（2026-08-24）。
- [x] 【実装済み】タイトル、タグ、分類、関連候補用の構造化出力schemaと安全なparse errorを決める。Go側のschema生成・正規化・safe errorを実装済み。
- [x] 【実装済み】retryなし、fallbackなし、秘密情報非露出、同時実行上限を維持する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で生成lockを含むGo testが成功した。

## 2. AI司書

- [x] 【実装済み】タイトル候補生成を実装する。
- [x] 【実装済み】タグ候補生成と既存タグとの重複整理を実装する。
- [x] 【実装済み】分類候補生成を実装する。自動ノートブック移動は行わない。
- [x] 【実装済み】関連メモの判定基準を決める。検索・バックリンク由来の候補poolと関連度を実装済み。
- [x] 【実装済み】AIを利用した関連メモの抽出方式を決める。構造化候補をProvider adapterから取得する。
- [x] 【実装済み】関連メモ候補を実装する。
- [x] 【受入済み】関連度が低い、候補がない、大量ノートの場合の挙動をテストする。低関連度・空候補、大量候補poolの20件上限、重複・current note・不正候補除外、snippet上限、結果件数上限を自動検証し、実画面の表示・操作も利用者が確認した（2026-08-24）。
- [x] 【実装済み】重複メモ候補を実装する。自動削除・自動統合は行わない。

## 3. 明示適用・データ保全

- [x] 【実装済み】候補のプレビュー、個別採用、破棄、手動retryを実装する。
- [x] 【実装済み】タイトル、タグ、分類、リンクの採用操作を既存Serviceとrevision/CASへ接続する。
- [x] 【受入済み】保存失敗、revision競合、ノート切替、キャンセル時に候補を適用しない。実Store統合テストで保存API呼出しと正本変更がないことを確認し、実画面の手動UI受け入れも利用者が完了した（2026-08-24）。
- [x] 【自動検証済み】AI候補、途中応答、プロンプト、生成結果、履歴をMarkdown、SQLite、操作journal、検索索引、WebDAV outboxへ保存しないことを、一時DB・data directory・各保存表の前後不変性とmarker非保存で確認した（2026-08-24）。

## 4. UI・受け入れ

- [x] 【実装済み】生成中、部分応答、キャンセル、成功、失敗、空結果、stale、競合をinlineで表示する。
- [x] 【実装済み】AI司書5操作を単一チャットtimelineの下部コンポーザーへ統合する。開いているノートの固定context chip、`＋`メニュー、Ask／Agent切替、入力欄内右下の送信、tool trace直後の候補カード、右側／下側配置・ドラッグ寸法・狭幅表示を実装し、`test:ai-chat`と`test:ai-workspace`で回帰確認した（2026-08-01）。
- [x] 【受入済み】キーボードでキャンセル、採用、破棄、retryを操作できることを確認する。native button、accessible name、handler、busy guard、focus可能性、Enter／Space既定動作を専用テストで確認し、実画面の手動操作も利用者が確認した（2026-08-24）。
- [x] 【実装済み】`test:ai-librarian`相当の既存Node script方式テストを維持・拡張し、単一チャットのcontext、tool trace、送信lock、未採用結果の置換防止を`test:ai-chat`／`test:ai-workspace`へ追加した。2026-08-01にローカルのFrontend・Go回帰テストとWails clean buildで成功を確認した。
- [x] 【実装済み】Go test doubleでstream、cancel、partial、structured output、safe error、非保存境界を検証する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で生成lockを含むGo testが成功した。
- [x] 【自動検証済み】AI失敗後もローカル保存、編集、検索、既存同期outbox更新が継続することをApp統合テストで確認した（2026-08-23）。
- [x] 【受入済み】AI失敗後もローカル保存、編集、検索、既存同期が継続することを、利用者が実画面で手動受け入れした（2026-08-24）。

## v2完了条件

- [x] 【受入済み】v2の詳細スコープに記載したAI司書5機能を実装・検証し、5操作、候補採用異常系、実画面の手動UI受け入れを完了した（2026-08-24）。
- [x] 【受入済み】関連メモの4項目を完了した。判定境界、低関連度、候補なし、大量候補poolを自動検証し、実画面の手動UI受け入れも完了した（2026-08-24）。
- [x] 【実装済み】ストリーミング、部分応答、キャンセル、構造化出力の異常系を検証する。CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で関連テストが成功した。
- [x] 【実装済み】DB schema、migration、AI用WebDAV entityを変更せずに完了する。v2実装コミットではAI保存用schema・migration・WebDAV entityを追加していない。
- [x] 【受入済み】CI、手動受け入れ、受け入れ記録を完了した。最終実装差分を含むcommit `e8f6816f60e61c4de149aaa45f778812c0ad86a8`に対するCI run [#32722645563](https://github.com/taniguchi-46/AtlasNote/actions/runs/32722645563) でWails clean build、Go tests、Frontend typecheck、全Frontend scriptを含む全工程が成功し、実画面の手動UI受け入れは利用者が「現状OK」と確認した（2026-08-24）。

## 2026-08-01の仕様決定

- [x] 【決定済み】Web検索を、明示確認付きのProvider管理ツールとして正式scopeへ含める。実装済みの安全契約は`ai-chat.md`、Phase 4での正式対象はv3 scopeを正とする。
- [x] 【決定済み】UI上の`Agent`は制限付きAgentモードとして正式化する。任意のAIエージェントや外部サービス操作は許可せず、将来の書き込み・編集は差分確認と利用者の明示適用を必須にする。実装タスクはv3 TODOで追跡する。

## 絶対遵守事項

- 実キー、実endpoint、実ノート本文、raw provider errorをテスト・ログ・受け入れ記録へ持ち込まない。
- 自動上書き、自動削除、自動リンク作成、自動分類を行わない。
- v3の履歴保存・AIアシスタント・ライティングをv2へ前倒ししない。
