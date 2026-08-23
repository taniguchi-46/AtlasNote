# Phase 4 TODO：v3

決定状態: 正式決定（2026-07-27）／v3保存仕様確定（2026-07-28）／進捗分類更新・Agent本文編集権限実装（2026-08-18）／エディタ即時反映・自動受け入れ拡張（2026-08-23）

## TODOの目的

v2のAI司書を基盤に、AIアシスタント、AIライティング、利用者が明示的に保存したAI履歴・生成成果物を追加する。Phase 4はこのv3の完了をもって完了とする。

詳細スコープは [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md)、v1は [`todo-phese4.md`](todo-phese4.md)、v2は [`todo-phese4-v2.md`](todo-phese4-v2.md) を正とする。

## 進捗分類（2026-08-23）

- `【実装済み】`: 対応するGo／Wails／Frontendコードと自動テストを確認できる項目。
- `【自動検証済み】`: 実APIを使わない統合・回帰テストまで完了し、手動確認を別項目で管理する項目。
- `【一部自動検証済み】`: 層ごとの自動テストは追加済みだが、UIまで連結した動的テストなどが残る項目。
- `【未検証】`: 実装はあるが、異常系の追加テスト、手動受け入れ、または最新差分のCI確認が残る項目。
- `【決定済み・未実装】`: 仕様は合意済みだが、コード、テスト、受け入れが未完了の項目。

v3の保存基盤、AIアシスタント／ライティングの基本経路、明示保存・削除・stale／orphaned境界、単一チャットtimeline、Provider管理Web検索、共通下書き、制限付きAgentの本文差分提案・編集権限設定（既定の明示適用と検証済み自動適用）、保存成功後のエディタ即時反映、v3相当のFrontend／Goテストは確認済みである。Wails公開APIからService、Repository、一時SQLiteまでの履歴・成果物ライフサイクル、version 10既存データ保持、version 12→13再構築失敗時のrollback、Provider失敗後のローカル機能継続も2026-08-23に自動検証した。手動受け入れ、空結果・長文のFrontend通し異常系、Assistant／Agentの利用者cancel、最新ローカル差分のCI記録は未完了として残す。

## v3保存仕様の確定（2026-07-28）

詳細な保存契約は [`ai-integration.md`](../development/ai-integration.md) の「v3保存仕様（D-03/D-04追補）」を正とする。実装前に確定した5項目は次のとおり。

1. `ai_histories`／`ai_history_messages`／`ai_history_sources` と `ai_artifacts`／`ai_artifact_sources` をschema version 12で追加し、履歴と成果物をSQLiteのローカル管理データとして保存する。WebDAV同期のentity、outbox、manifest、object、conflictには追加しない。
2. 明示保存したuser／assistantメッセージと最終編集済み成果物だけを保存し、system prompt、内部指示、raw context、request body、API Key、Authorization、raw provider error、生成中chunkは保存しない。
3. 自動保持期限は設けない。個別・一括削除は本体とmessages／sourcesを含むアプリケーション上の完全削除とし、soft-delete・tombstone・AI一時ファイルは作らない。物理媒体の消去は保証しない。
4. 参照元ノート削除後も保存済みデータを残す。`note_id`／`input_revision` を保持し、参照不能は `orphaned`、revision不一致は `stale` とする。自動rebase・自動再生成はせず、再生成は明示操作に限定する。
5. CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) はWails clean build、Go tests、Frontend typecheck、全Frontend scriptを含む全工程に成功した。AI司書キャンセル時の生成ロックに関する既知CI例外は解消したが、手動受け入れと追加検証は残る。

この確定はv3の保存設計を承認するものであり、未完了のv1／v2完了条件や、CI成功の完了条件を満たしたことを意味しない。

## v3開始条件

- [x] 【実装済み】v1の完了条件を満たす。D-01〜D-07の設計承認、実装、保存／同期境界テスト、CI、ローカル受け入れは2026-07-27に完了している。
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
- [x] 【実装済み】単一チャットで現在ノートを固定contextとし、明示追加ノートとNotebook scopeを最大10ノートへ解決してQ&A／壁打ちへ渡す。Ask／AgentのUIモードは共通の安全な読み取り・候補生成だけを許可し、無確認のノート変更・任意外部操作は行わない。
- [x] 【実装済み】Web検索を、実行ごとの明示確認、OpenRouter Web Search／Exa固定、検索結果・合計最大3件、実行回数1回の応答検証、公開HTTPS出典だけの表示というProvider管理ツールとして単一チャットへ統合する。
- [x] 【実装済み】制限付きAgentモードで、開いている1ノートの本文変更案を作成する。タイトル・タグ・所属Notebook・リンクは既存の個別候補採用フローを維持する。対象ノート、基準revision、変更理由、変更前後、影響フィールドをtimelineの提案カードへ表示する。端末ローカル設定の既定`review-required`では利用者が明示的に適用／破棄し、`auto-update`では外部送信前の確認後に通常のAgent送信が返した検証済み提案だけを既存のノート更新API、revision/CAS、保存laneを通して自動適用する。stale・競合・保存失敗・キャンセルでは変更を保存せず、適用後も差分を表示する。構造化出力、差分一意性、Frontend回帰テストを2026-08-18に更新した。
- [x] 【追加修正済み】Agent適用成功後、同一ノートの保存済み本文をMarkdown／WYSIWYGエディタへ直ちに反映する。Agent保存中に作成されたdraftは未開始autosaveを取り消して競合として保持し、ローカル編集の上書きと反映処理からの重複autosaveを防ぐ回帰テストを追加した（2026-08-23）。
- [x] 【追加修正済み】AI参照sourceへ全文文字数、送信本文・全文バイト数、切り詰め有無、作成日時、更新日時を渡し、OpenRouterのモデル一覧で`stream`を誤って非対応扱いする判定を修正した。Agent能力の不明値は実行時判定へ委譲し、Gemini/OpenRouterの構造化SSEエラーを安全な型付きエラーへ分類する回帰テストを追加した（2026-08-15）。
- [ ] 【一部自動検証済み】空結果、候補なし、長文、対象ノート切替、revision差異、timeout、cancelを通しでテストする。Goでは候補なし、長文上限、timeout分類、context cancel時のProvider停止・安全なエラー・生成lock解放を確認した。Frontendでは対象ノート切替、事前revision差異、差分不一致、保存時CAS競合、保存失敗に加え、AI司書の利用者cancel／timeoutをWails mockから実Store・timelineまで通し、開始応答前のcancel予約とcancel API失敗後の終端監視も確認した。Assistant／Agentの`AI_TIMEOUT`／`AI_CANCELLED`応答、Writing timeoutの安全な非保存と手動retryも2026-08-24に自動検証した。Assistant／Agentを利用者操作で停止するAPI・Store・UIは未実装であり、空結果・長文のFrontend通し確認とともに残る。

## 2. AIライティング

- [x] 【実装済み】プロンプト生成・改善を実装する。
- [x] 【実装済み】README、ドキュメント、ブログ記事、要件定義の草案生成を実装する。
- [x] 【実装済み】生成文のプレビュー、編集、コピー、明示保存、新規ノート作成を実装する。
- [x] 【実装済み】確認なしの本文上書き、既存文書への自動追記、自動公開を行わない。明示確認後の追記・本文置換はrevision/CAS・保存laneを通す。

## 3. AI履歴・生成成果物のデータモデル

- [x] 【実装済み】会話、要約、タイトル、タグ、分類、関連候補、執筆結果の保存対象を決める。
- [x] 【実装済み】安定ID、種別、対象ノート／会話ID、入力revision、Provider ID、モデルID、生成日時、更新日時、結果、状態のschemaを実装する。
- [x] 【実装済み】API Key、Authorization、raw provider error、不要な本文・プロンプトを保存しない境界を実装する。
- [x] 【実装済み】利用者が明示的に保存した会話・成果物と、生成成功時の要約履歴を永続化する。
- [x] 【実装済み】個別削除、一括削除、再生成、stale／orphaned表示、無期限保持を実装する。
- [x] 【実装済み】保存済みAIデータ自体はノート本文を変更せず、参照revisionを動的に評価する。ノートへの自動適用は行わない。
- [x] 【自動検証済み】schema migration、既存データ保持、rollback、Provider失敗時の不変性をv3専用受け入れで検証する。version 10のノート・タグ・同期状態保持、version 12→13のartifact／source保持、再構築途中の強制失敗時に`user_version`・元データ・indexを復元する専用テスト、およびProvider失敗前後のMarkdown・検索・journal・同期状態不変テストを2026-08-23に確認した。

## 4. 同期境界

- [x] 【実装済み】AI設定、credential reference、API Key、履歴、生成成果物がsync outbox、manifest、object、conflictを更新しない境界を実装・テストする。
- [x] 【実装済み】Phase 3のノート、ノートブック、タグ、ノートタグ同期がAI機能追加後も回帰しないことをCI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) で確認する。
- [x] 【実装済み】他端末ではAI履歴・生成成果物を同期せず、再生成・再保存が必要であることを仕様とUIに明記する。

## 5. テスト・CI・受け入れ

- [x] 【実装済み】`test:ai-assistant`相当のmock Wails APIテストを追加する（`test:ai-v3`）。
- [x] 【実装済み】`test:ai-history`相当の保存、削除、再生成、stale／orphaned境界テストを追加する。
- [x] 【実装済み】AI機能を単一チャットtimelineの下部共通コンポーザーへ統合する。`＋`メニューからノート／Notebook、要約、文章作成6種、AI司書、Web検索を選択でき、Ask／Agent切替、入力欄内右下の送信、tool trace直後の結果カード、保存済み履歴・成果物・要約履歴への導線を実装した。履歴読込・削除、モデル設定、UI配置、ドラッグ寸法、狭幅表示を`test:ai-workspace`／`test:ai-chat`で確認した（2026-08-01）。
- [x] 【実装済み】AI自由記述のMarkdown出力契約と、既存HTMLを含む回答を安全に見出し・リストへ変換するAI専用DOMサニタイズ表示を追加した。通常ノートのraw HTML無効化、原文保存、危険なURL・外部リソース遮断を維持し、`test:ai-markup-safety`、既存AIテスト、Goテスト、Frontend typecheck/buildで確認した（2026-08-01）。
- [x] 【実装済み】Ask、Agent、文章作成、Web検索はChatGPTライクな共通下書きを正とする。機能別下書きは保持・永続化せず、固定scopeツールは追加context・下書きを使用しない。
- [x] 【自動検証済み】GoでRepository、Service、Wails API、migration、rollbackをtest doubleと一時DBで通し検証する。`TestAppAIRecordLifecycleUsesLocalDatabaseWithoutChangingSyncState`で保存、一覧、取得、stale、orphaned、個別・一括削除とWebDAV同期表の不変性をApp公開APIから確認し、database専用テストでmigration／rollbackを確認した（2026-08-23）。
- [x] 【自動検証済み】AI失敗後もローカル保存、編集、検索、既存同期outbox更新が継続することをApp統合テストで確認する（2026-08-23）。
- [ ] 【未検証】AI失敗後のローカル保存、編集、検索、既存同期の継続を実画面で手動受け入れする。
- [ ] 【未検証】実キーなしでAI設定、Q&A、ライティング、保存、削除、再生成、キーボード操作を手動受け入れする。
- [x] 【実装済み】CIにv3関連テストを追加し、秘密情報・本文・endpoint・raw errorを記録しないテスト境界を維持する。

## v3完了条件（Phase 4完了条件）

- [ ] v1、v2、v3の全完了条件を満たす。
- [ ] AIアシスタント、AIライティング、明示保存・削除・再生成が利用者から確認できる。
- [x] migration、rollback、revision/CAS、削除、stale、競合を自動テストで検証済みである（2026-08-23）。
- [x] AIデータがWebDAVへ流れず、既存同期契約が回帰していないことをApp統合テストとGo全体テストで確認済みである（2026-08-23）。
- [ ] Go、Frontend、CI、手動受け入れ、差分レビューを完了する。
- [x] 【自動検証済み】制限付きAgentの変更提案、差分確認、編集権限設定（明示適用／検証済み自動適用）、revision/CAS・競合・保存失敗・保存中ノート切替境界を確認した。StoreのCAS・競合・保存失敗・保存中ノート切替に加え、UI権限フローで`review-required`の自動保存0回、`auto-update`の保存1回、応答待ち中の設定変更に対する送信開始時権限の固定、提案なし・送信失敗時の保存0回、自動適用失敗時の提案保持を2026-08-24に動的テストした。
- [ ] `docs/status.md`とPhase 4 v1〜v3のscope／TODOを最終状態へ更新する。

## 対象外

- Groq、OpenAI、Ollama、LM StudioのProvider追加
- AIデータのWebDAV同期、クラウド履歴、端末間履歴共有
- 任意のAIエージェント、任意コマンド実行、任意の外部サービス操作、添付ファイル・画像・音声のAI連携。明示確認付きのProvider管理Web検索と、差分確認および利用者が選んだ編集権限に限定したAgent変更提案は対象内とする。
- Providerの課金管理、アカウント作成、モデル学習
