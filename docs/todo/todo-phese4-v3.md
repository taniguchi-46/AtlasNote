# Phase 4 TODO：v3

決定状態: 正式決定（2026-07-27）／v3保存仕様確定（2026-07-28）

## TODOの目的

v2のAI司書を基盤に、AIアシスタント、AIライティング、利用者が明示的に保存したAI履歴・生成成果物を追加する。Phase 4はこのv3の完了をもって完了とする。

詳細スコープは [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md)、v1は [`todo-phese4.md`](todo-phese4.md)、v2は [`todo-phese4-v2.md`](todo-phese4-v2.md) を正とする。

## 進捗分類（2026-07-28）

- `【実装済み】`: 対応するGo／Wails／Frontendコードと自動テストを確認できる項目。
- `【未検証】`: 実装はあるが、異常系の追加テスト、手動受け入れ、またはWails APIの通し確認が残る項目。
- `【CIブロック】`: ローカルでは確認できても、既知のCI run [#30360052157](https://github.com/taniguchi-46/AtlasNote/actions/runs/30360052157) のAI司書テスト失敗が残るため、完了判定できない項目。

v3の保存基盤、AIアシスタント／ライティングの基本経路、明示保存・削除・stale／orphaned境界、v3相当のFrontend／Goテストは実装済み。手動受け入れ、Wails APIの個別通し確認、追加異常系、既知CI例外は未完了として残す。

## v3保存仕様の確定（2026-07-28）

詳細な保存契約は [`ai-integration.md`](../development/ai-integration.md) の「v3保存仕様（D-03/D-04追補）」を正とする。実装前に確定した5項目は次のとおり。

1. `ai_histories`／`ai_history_messages`／`ai_history_sources` と `ai_artifacts`／`ai_artifact_sources` をschema version 12で追加し、履歴と成果物をSQLiteのローカル管理データとして保存する。WebDAV同期のentity、outbox、manifest、object、conflictには追加しない。
2. 明示保存したuser／assistantメッセージと最終編集済み成果物だけを保存し、system prompt、内部指示、raw context、request body、API Key、Authorization、raw provider error、生成中chunkは保存しない。
3. 自動保持期限は設けない。個別・一括削除は本体とmessages／sourcesを含むアプリケーション上の完全削除とし、soft-delete・tombstone・AI一時ファイルは作らない。物理媒体の消去は保証しない。
4. 参照元ノート削除後も保存済みデータを残す。`note_id`／`input_revision` を保持し、参照不能は `orphaned`、revision不一致は `stale` とする。自動rebase・自動再生成はせず、再生成は明示操作に限定する。
5. CI run [#30360052157](https://github.com/taniguchi-46/AtlasNote/actions/runs/30360052157) はWails clean build成功、`internal/ai/librarian_test.go:86` の既知のタイミング依存テスト失敗として受け入れ例外に記録する。CI成功扱いにはせず、ユーザー指示どおり修正せずにv3仕様の確定・実装準備を進める。

この確定はv3の保存設計を承認するものであり、未完了のv1／v2完了条件や、CI成功の完了条件を満たしたことを意味しない。

## v3開始条件

- [ ] v1の完了条件を満たす。
- [ ] v2の完了条件を満たす。
- [x] AI履歴・生成成果物を保存する対象、正本、削除、再生成、保持期間をレビュー承認する（2026-07-28確定）。
- [x] v3でもAI設定、資格情報、AI履歴、生成成果物をWebDAV同期しないことを承認する（2026-07-28確定）。
- [x] 【実装済み】DB schema、migration、rollback、既存データへの影響を確定・実装する。
  - schema version 12の5テーブルを追加し、既存migrationの単一トランザクションとrollbackテスト、version 10からの移行テストを利用する。AIテーブルは既存データを変更せずに追加される。

## 1. AIアシスタント

- [x] 【実装済み】現在のactive noteを対象にQ&Aを実装する。
- [x] 【実装済み】既存の検索・リンク索引を使ったRAG検索を実装する。
- [x] 【実装済み】送信対象ノート、revision、参照バイト数、検索条件を送信前に確認する。
- [x] 【実装済み】アイデア壁打ちとブレインストーミングを実装する。
- [ ] 【未検証】空結果、候補なし、長文、対象ノート切替、revision差異、timeout、cancelを通しでテストする。

## 2. AIライティング

- [x] 【実装済み】プロンプト生成・改善を実装する。
- [x] 【実装済み】README、ドキュメント、ブログ記事、要件定義の草案生成を実装する。
- [x] 【実装済み】生成文のプレビュー、編集、コピー、明示保存を実装する。
- [x] 【実装済み】確認なしの本文上書き、既存文書への自動追記、自動公開を行わない。

## 3. AI履歴・生成成果物のデータモデル

- [x] 【実装済み】会話、要約、タイトル、タグ、分類、関連候補、執筆結果の保存対象を決める。
- [x] 【実装済み】安定ID、種別、対象ノート／会話ID、入力revision、Provider ID、モデルID、生成日時、更新日時、結果、状態のschemaを実装する。
- [x] 【実装済み】API Key、Authorization、raw provider error、不要な本文・プロンプトを保存しない境界を実装する。
- [x] 【実装済み】利用者が明示的に保存した結果だけを永続化する。
- [x] 【実装済み】個別削除、一括削除、再生成、stale／orphaned表示、無期限保持を実装する。
- [x] 【実装済み】保存済みAIデータ自体はノート本文を変更せず、参照revisionを動的に評価する。ノートへの自動適用は行わない。
- [ ] 【未検証】schema migration、既存データ保持、rollback、Provider失敗時の不変性をv3専用受け入れで検証する。

## 4. 同期境界

- [x] 【実装済み】AI設定、credential reference、API Key、履歴、生成成果物がsync outbox、manifest、object、conflictを更新しない境界を実装・テストする。
- [ ] 【CIブロック】Phase 3のノート、ノートブック、タグ、ノートタグ同期がAI機能追加後も回帰しないことをCIで確認する。
- [x] 【実装済み】他端末ではAI履歴・生成成果物を同期せず、再生成・再保存が必要であることを仕様とUIに明記する。

## 5. テスト・CI・受け入れ

- [x] 【実装済み】`test:ai-assistant`相当のmock Wails APIテストを追加する（`test:ai-v3`）。
- [x] 【実装済み】`test:ai-history`相当の保存、削除、再生成、stale／orphaned境界テストを追加する。
- [x] 【実装済み】AIアシスタントとAIライティングをAIワークスペース下部共通コンポーザーの`＋`メニューと入力欄へ統合し、保存済み履歴・成果物をヘッダーの履歴アイコンから開けるようにする。履歴読込・削除、モデル設定への導線、UI配置、ドラッグによる寸法調整を`test:ai-workspace`で確認する。
- [ ] 【未検証】GoでRepository、Service、Wails API、migration、rollbackをtest doubleと一時DBで通し検証する。
- [ ] 【未検証】AI失敗後もローカル保存、編集、検索、既存同期が継続することを手動・統合で検証する。
- [ ] 【未検証】実キーなしでAI設定、Q&A、ライティング、保存、削除、再生成、キーボード操作を手動受け入れする。
- [x] 【実装済み】CIにv3関連テストを追加し、秘密情報・本文・endpoint・raw errorを記録しないテスト境界を維持する。

## v3完了条件（Phase 4完了条件）

- [ ] v1、v2、v3の全完了条件を満たす。
- [ ] AIアシスタント、AIライティング、明示保存・削除・再生成が利用者から確認できる。
- [ ] migration、rollback、revision/CAS、削除、stale、競合が検証済みである。
- [ ] AIデータがWebDAVへ流れず、既存同期契約が回帰していない。
- [ ] Go、Frontend、CI、手動受け入れ、差分レビューを完了する。
- [ ] `docs/status.md`とPhase 4 v1〜v3のscope／TODOを最終状態へ更新する。

## 対象外

- Groq、OpenAI、Ollama、LM StudioのProvider追加
- AIデータのWebDAV同期、クラウド履歴、端末間履歴共有
- AIエージェント、外部サービス操作、添付ファイル・画像・音声のAI連携
- Providerの課金管理、アカウント作成、モデル学習
