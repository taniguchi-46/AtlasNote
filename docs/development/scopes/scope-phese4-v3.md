# Phase 4詳細スコープ v3：AIアシスタント・ライティング・履歴

最終更新: 2026-07-28

決定状態: 正式決定（2026-07-27）／v3保存仕様追補確定（2026-07-28）

## 位置付け

本書は、Phase 4 v2（AI司書・実行体験）の後に実装するv3の要求範囲と、Phase 4全体を完了と判定するための最終条件を定義する。v1は [`scope-phese4.md`](scope-phese4.md)、v2は [`scope-phese4-v2.md`](scope-phese4-v2.md)、v3の作業チェックは [`../../todo/todo-phese4-v3.md`](../../todo/todo-phese4-v3.md) を正とする。

v3では、AIアシスタントとAIライティングを追加し、利用者が明示的に保存したAI履歴・生成成果物を端末内で管理する。AI設定、資格情報、履歴、生成成果物はv3でもWebDAV同期しない。

## 目的

ノートを検索・整理するだけでなく、選択した知識を根拠として質問、発想、文章作成を行えるようにする。同時に、生成結果の保存・削除・再生成・migrationを明示的なデータ契約として確立する。

## 保存仕様（2026-07-28確定）

詳細な正本は [`ai-integration.md`](../ai-integration.md) の「v3保存仕様（D-03/D-04追補）」とする。v3では次の5項目を確定する。

1. 保存は利用者の明示操作時だけ行い、AI履歴・生成成果物はSQLiteのローカル管理データとして分離保存する。schema version 12で `ai_histories`、`ai_history_messages`、`ai_history_sources`、`ai_artifacts`、`ai_artifact_sources` を追加し、WebDAV同期対象にはしない。
2. 保存するのは、保存操作時のuser／assistantメッセージと、明示保存された最終編集済み成果物だけとする。system prompt、内部指示、raw context、Provider request body、API Key、Authorization、raw provider error、生成中chunkは保存しない。
3. 自動期限は設けず、削除はsoft-deleteではないアプリケーション上の完全削除とする。messages／sourcesを含めてトランザクションで削除するが、SQLiteの物理ページ消去までは保証しない。
4. 参照元ノートを完全削除しても履歴・成果物は保持する。`note_id`／`input_revision` を残し、参照不能は `orphaned`、revision不一致は `stale` と表示する。自動rebase・自動再生成はしない。
5. Wails clean build成功・AI司書テスト失敗のCI run #30360052157は既知の受け入れ例外として扱う。CI成功とは記録せず、例外を残したままv3の仕様確定・実装準備を進める。

## 対象範囲

### 1. AIアシスタント

- 現在ノートまたは利用者が明示的に選択したノートを対象にQ&Aを行う。
- SQLiteの既存検索・リンク索引を使ったRAG検索を提供する。対象ノート、検索範囲、送信内容を利用者に示す。
- アイデア壁打ちとブレインストーミングを単発または短い会話として提供する。
- 全ノートの自動送信、無制限なコンテキスト拡張、添付ファイル・画像・音声の自動送信は行わない。

### 2. AIライティング

- 利用者が指定した目的に対するプロンプト生成・改善を提供する。
- 選択したノートまたは入力内容からREADME、ドキュメント、ブログ記事、要件定義の草案を生成する。
- 生成文はプレビュー、編集、コピー、明示保存を経て確定する。
- Markdown本文への自動上書き、既存文書への無確認追記、自動公開は行わない。

### 3. AI履歴・生成成果物のローカル保存

- Markdown本文を正本とし、AI履歴・生成成果物はSQLiteの管理データとして保存する。
- 保存対象は、利用者が明示的に保存した会話または成果物に限定する。生成中のchunk、API Key、Authorization、raw provider errorは保存しない。
- 保存レコードには、安定ID、種別、対象ノートまたは会話ID、入力時点のrevision、Provider ID、モデルID、生成日時、更新日時、本文または結果、状態を持たせる。
- 履歴・成果物は個別削除、一括削除、再生成を提供する。
- AIアシスタントとAIライティングはAIワークスペース下部コンポーザーの`＋`メニューから切り替え、共通入力欄で質問または作成指示を送信する。保存済み履歴・成果物はヘッダーの履歴アイコンから開き、一覧・読込・削除する。v1要約とv2候補は一時状態のため、この一覧に含めない。
- 結果をノート本文、タグ、分類、リンクへ適用するときは既存のrevision/CAS・保存lane・Tag／Link Serviceを通す。
- revisionが変わった結果はstaleとして表示し、自動rebase・自動上書き・自動retryは行わない。
- schema、migration、既存データ保持、rollbackを実装前に確定する。

### 4. WebDAV同期境界

- AI設定、credential reference、API Key、AI履歴、生成成果物、会話履歴はWebDAV同期しない。
- Phase 3のノート、ノートブック、タグ、ノートタグの同期契約を維持する。
- AI用のmanifest、object、entity、outbox、conflict、CAS、schema version、migrationは追加しない。
- 他端末ではAI履歴・生成成果物を共有せず、必要な場合は端末ごとに再生成または再保存する。

### 5. v3のセキュリティ・受け入れ

- 保存・削除・再生成の前後で、API Key、本文、プロンプト、raw provider errorが意図しない保存先へ出ないことを検証する。
- AI履歴の削除がSQLiteの管理データと関連する一時ファイルを残さないことを検証する。
- AI利用失敗後も、通常のローカル保存・編集・検索・既存同期が継続することを検証する。
- 実キー・実endpointなしのtest doubleを使い、手動受け入れ・CI記録にも秘密情報を残さない。

## 対象外

- Groq、OpenAI、Ollama、LM StudioのProvider追加。追加ProviderはPhase 4完了後の別スコープとする。
- AIデータのWebDAV同期、端末間のAI履歴共有、クラウド履歴サービス。
- AIエージェント、外部サービス操作、添付ファイル・画像・音声のAI連携。
- 利用者の確認なしのノート本文・タグ・分類・リンクの変更。
- 全ノートを対象にした自動バッチ処理。
- モデル学習、Providerの課金管理、アカウント作成。

## Phase 4完了条件

- v1の完了条件、v2の完了条件、v3の完了条件をすべて満たしている。
- v3のschema、migration、rollback、削除、再生成、revision/CASの受け入れが完了している。
- AI履歴・成果物がWebDAVへ流れず、既存同期契約が回帰していない。
- AIアシスタント、AIライティング、明示保存・削除・再生成のFrontend／Go／CI／手動受け入れが成功している。
- `docs/status.md`、Phase 4 v1〜v3のscope／TODOが実装状態と一致している。
