# Phase 4詳細スコープ：AI

最終更新: 2026-07-22

## 位置付け

この文書は、Phase 4「AI」の要求範囲と実装前に合意すべき境界を定義する。実装順序と未完了の確認事項は [`todo-phese4.md`](../../todo/todo-phese4.md) を参照する。

Phase 3の受け入れは、非本番の実WebDAVで動作OKを確認して完了している。Phase 4はD-01〜D-07の実装前設計を承認済みであり、D-02（AI認証・秘密情報）とD-05（Provider adapter・実行制御）のGo実装および関連Goテスト、D-06（AI設定UI・要約操作）とmock Wails APIを使う`test:ai-store`を完了している。D-07のCI拡張・保存/同期境界の受け入れは未完了である。

## 参照する正本

- 要求範囲: [`scope.md`](scope.md)
- 現在状況: [`../../status.md`](../../status.md)
- Phase 4実装前TODO: [`../../todo/todo-phese4.md`](../../todo/todo-phese4.md)
- 既存のローカル保存・競合契約: [`../note-concurrency.md`](../note-concurrency.md)
- Phase 3同期契約: [`../webdav-sync.md`](../webdav-sync.md)
- アーキテクチャ・データ境界: [`../../rules/architecture.md`](../../rules/architecture.md)

## 目的

ユーザー自身が選択したAIプロバイダーを利用し、ローカルファーストのノートを要約・整理・検索・執筆支援に活用できるようにする。AIを利用できない場合も、既存のローカル保存・編集・検索・同期を継続できることを前提とする。

## 対象範囲

### AI設定（v1）

- OpenRouterまたはGemini APIの認証設定とプロバイダー切り替え
- モデル一覧を選択中のプロバイダーから利用者の明示操作で取得するモデル選択
- 表示名、モデルID、v1の要約に使用可能か、取得できる場合の入力・出力上限、取得日時、現在の利用可否の表示
- 接続確認、認証失敗、利用不能、レート制限、タイムアウトの表示
- v1ではOpenRouterとGemini APIを初期対応とし、Groq、OpenAI、Ollama、LM Studioは後続に回す。
- 取得できないモデル情報は`不明`とし、価格・速度・ツール呼び出し・画像/音声対応・ストリーミング対応はv1で表示しない。選択済みモデルが利用不可になった場合は自動切り替えせず、再選択を求める。

### AI司書（v1）

- メモ要約（接続確認後の単発テキスト生成）
- 要約は現在表示中の画面だけで保持する一時結果とし、ノート本文への追記、Markdown・SQLite・検索索引・WebDAV outboxへの保存は行わない。必要な内容は利用者が明示的にコピーする。

### AI司書（後続候補）

- タイトル生成
- タグ生成
- 自動分類
- 関連メモ提案
- 重複メモ検出

### AIアシスタント／ライティング（v1対象外・後続候補）

- メモQ&A、RAG検索、アイデア壁打ち、ブレインストーミング
- プロンプト生成・改善
- README、ドキュメント、ブログ記事、要件定義の生成

### v1対象外・後続範囲

- ストリーミング、部分応答、RAG検索、バッチ処理
- 利用者によるキャンセルと構造化出力はv2で対応する。

## 実装前に確定する境界

### 1. AI認証・秘密情報

- v1で受け付ける秘密情報はOpenRouterの通常の推論用APIキーと、Gemini APIで有効な認証キーまたはGemini API向けに制限済みのAPIキーだけとする。OpenRouterのOAuth/PKCE、管理APIキー、BYOK設定、ローカルプロバイダー接続情報はv1の対象外とする。
- プロバイダーID、モデルID、ランダムなcredential referenceだけを非秘密設定として扱い、実キーはWebDAVと分離したAI用OS CredentialStoreへ保存する。OpenRouterとGemini APIのキーは相互に再利用しない。
- OS CredentialStoreへの保存を常に試み、利用できない場合だけプロセス内のsession-only保持へ切り替える。再起動後は再入力を求め、平文のSQLite、Markdown、`localStorage`、設定ファイル、`.env`、環境変数へfallbackしない。
- キーは空文字、改行、制御文字を拒否し、形式・接頭辞だけで有効性を判定しない。保存済みキーの値・一部・長さはUIへ返さず、プロバイダーごとの削除と全AIキー削除を提供する。更新では新しいキーの保存と非秘密設定の更新に成功してから旧キーを削除する。
- APIキー、Authorizationヘッダー、`x-goog-api-key`、ノート本文、プロンプト、生成結果、raw provider error bodyをログ・エラー・通知・診断・クラッシュ情報へ出さない。Providerのdebug機能を有効化しない。
- 接続先はOpenRouterとGemini APIの固定HTTPSホストだけとし、利用者によるendpoint、HTTP、TLS無効化、redirect、proxyの設定・使用をv1で許可しない。接続確認とモデル一覧取得はノート本文を送らない読み取り専用操作とし、失敗時に資格情報や設定へ副作用を残さない。
- 生成の自動retryは行わず、利用者の手動retryだけを許可する。アプリ内の金額上限やProviderの請求・キー管理は行わず、利用上限はProvider側で設定する。timeout、出力上限、rate limitの実行契約はD-05で確定済みとする。

### 2. AI設定・プロバイダー契約

- D-05として、Go側のProvider adapterは`ListModels`、`CheckConnection`、`GenerateSummary`だけを公開する。UI/Wails APIへは正規化済みのモデル情報、要約結果、型付き安全エラーだけを返し、プロバイダー固有のrequest/response、HTTPヘッダー、raw error bodyはadapter外へ出さない。
- 接続確認はノート本文を送らない読み取り専用操作とする。OpenRouterは通常の推論用キーで`GET /api/v1/key`を呼び認証成否だけを使い、Gemini APIは`x-goog-api-key`ヘッダーでモデル一覧を1件取得して認証成否だけを使う。モデル一覧は利用者の明示操作で取得し、OpenRouterはtext入力・text出力、Gemini APIは`generateContent`対応のモデルだけをv1候補に正規化する。
- 要約生成は固定の要約指示と現在ノート本文だけを送る単発・非ストリーミングとする。Gemini APIはstable `v1`の保存を伴わない`generateContent`を使う。OpenRouterは選択済みの具体的モデルIDだけを使い、`zdr: true`、`data_collection: "deny"`、`allow_fallbacks: false`を必須とする。`openrouter/auto`、別モデル・別providerへの自動切替、会話履歴、ツール、ファイル、画像・音声、構造化出力、任意プロンプトはv1の対象外とする。
- 本文入力はUTF-8で12 KiBまで、出力は最大512 tokensとする。超過時は送信・自動切詰め・分割・バッチ化を行わずエラーとする。接続確認とモデル一覧取得のdeadlineは10秒、要約生成は60秒とし、自動・バックグラウンド・別モデル・別providerへのretryを行わない。アプリ全体の同時要約生成は1件だけとする。
- 空結果、途中終了、出力上限到達、非text出力、不正JSONは要約結果として採用しない。エラーは`AI_AUTH_FAILED`、`AI_MODEL_UNAVAILABLE`、`AI_INPUT_TOO_LARGE`、`AI_RATE_LIMITED`、`AI_TIMEOUT`、`AI_NETWORK_UNAVAILABLE`、`AI_PROVIDER_UNAVAILABLE`、`AI_BUSY`、`AI_INVALID_RESPONSE`へ正規化する。利用者によるキャンセル、構造化出力、ストリーミング、部分応答はv2で対応する。
- D-07で実キーを使わないProvider test doubleを用い、固定endpoint、接続確認時の本文非送信、モデル正規化、privacy設定、上限・deadline・単一実行・retryなし、型付きエラー、結果破棄、秘密情報非露出を検証する。

### 3. 生成結果の保存境界

- D-03として、v1のメモ要約は現在表示中のノートのUIメモリだけに保持する一時結果とする。Markdown本文、SQLite、検索索引、操作journal、別成果物、WebDAV outboxへは保存しない。
- 要約開始時の`baseRevision`はメモリだけに保持する。生成中にノートのrevisionが変わった場合は「古い内容から生成された要約」と表示し、自動適用・自動rebase・自動retryは行わない。利用者が必要な結果だけを明示的にコピーする。
- 画面遷移、ノート切替、再読み込み、アプリ終了で要約結果を破棄する。生成元モデル、生成日時、入力本文、プロンプト、生成結果をv1の履歴・キャッシュとして保持しないため、DB schema、migration、Markdown形式は変更しない。
- チャット履歴の保持はv1の対象外とし、v2以降で保存先、保持期間、削除、プライバシー表示、端末間同期、migration、競合を別途承認する。

### 4. WebDAV同期との境界

- D-04として、v1ではPhase 3のWebDAV同期対象をノート、ノートブック、タグ、ノートタグだけに維持する。AI用のentity、manifest/object、change set、outbox、conflict、snapshotは追加しない。
- 一時要約、入力本文、プロンプト、生成結果、チャット履歴、AI設定のプロバイダーID・モデルID・credential reference、AI API KeyはWebDAV同期対象外とする。AI設定と資格情報は端末ローカルであり、端末ごとに設定・接続確認・モデル選択を行う。
- D-03によりv1の要約は永続化しないため、要約生成は既存同期のoutboxを作成・更新しない。同期のpull・競合解決・復旧・再アップロード・再ダウンロードもAI関連データを扱わない。
- WebDAVのformat、manifest、object、entity型、schema version、migration、CAS・競合契約、同期Serviceを変更しない。v2以降でAI履歴その他を永続化する場合は、D-03でデータモデルを承認してから、同期の可否をD-04追補で決定する。

### 5. UI・データフロー

- D-06として、設定画面にAIタブを追加し、既存の同期設定と同じ下書き・接続確認・`適用`/`OK`の流れを使う。API Key入力欄は常に空のpassword入力とし、接続確認の失敗時に保存しない。プロバイダー別の削除と全AIキー削除には明示確認を求め、session-only時は表示する。
- モデル一覧は利用者が明示的に更新し、接続確認済みのv1候補から選ぶ。選択済みモデルが利用不可の場合は再選択を求め、自動切替・自動更新を行わない。
- `AIで要約`はエディターツールバーから実行する。設定・接続確認・モデル選択が未完了の場合、ゴミ箱内ノート・空本文・12 KiB超過本文の場合、または既存draftの保存が失敗・競合した場合は、外部送信せずに理由を表示する。機密ノートの自動判定・自動マスキング・自動切詰め・分割・バッチ処理は行わない。
- 要約前に正常保存済みの現在ノート本文と`revision`をsnapshotにし、送信のたびに、送信先プロバイダー・モデルID、固定の要約指示と本文だけを送ること、結果を保存・同期しないことを明示して確認する。タイトル、他ノート、添付、画像、会話履歴を送らず、確認を記憶・自動送信しない。
- 要約実行中はアプリ全体で1件だけを許可する。キャンセル・部分応答はv1で表示しない。ノート切替・画面遷移・再読み込み時は結果を破棄し、開始済み要求は完了後に結果を無視する。正常結果は一時パネルで`コピー`・`破棄`だけを許可し、`baseRevision`と現在revisionが違う場合は古い内容からの要約として警告する。
- 操作元にはinlineの安全な状態を表示し、対象ノートが非表示なら共通通知を使う。型付きエラーだけを日本語の案内へ対応付け、本文、プロンプト、生成結果、API Key、raw provider messageは表示・通知・ログに出さない。rate limitの`Retry-After`は待機目安だけを表示し、待機後も利用者が再度確認して手動retryする。

### 6. テスト・受け入れ条件

- D-07として、実キー、実endpoint、実ノート本文、実プロンプト、実生成結果を自動テスト・手動受け入れ・CI・fixture・ログへ使わない。テスト用endpointを実行時の設定として公開せず、Provider adapterのHTTP transportまたはProvider interfaceだけをテストで注入する。
- GoのProvider adapter/Application Service/CredentialStore/Wails APIテストで、固定通信契約、モデル正規化、privacy設定、保存無効、上限・deadline・retryなし・単一実行、型付きエラー、資格情報・本文非露出、保存/同期境界、AI失敗後のローカル機能継続を検証する。
- frontendは既存のNode script方式でmock Wails APIを使う`test:ai-store`を追加し、AI設定下書き、送信前確認、保存済みsnapshot、状態遷移、手動retry、結果破棄・stale表示・コピーだけ、通知非露出を検証する。新しいVueテスト依存は追加しない。
- CIは既存のWails build、`go test ./...`、frontend typecheck、既存frontend scriptsを維持し、`test:ai-store`を追加する。Providerへの実通信を必要としない。
- 手動受け入れでは実キーなしで、AI設定タブ、password入力、未設定時の送信阻止、送信確認、状態表示、コピー/破棄、キーボード操作を確認する。実装後の結果は`todo-phese4.md`に秘密情報を除いて記録する。

## 対象外

- Phase 4の設計承認前のAI API実装、DB migration、WebDAV契約変更
- Atlas Note側でのモデル学習、プロバイダーアカウントの作成・課金契約・請求管理
- ユーザーの明示確認なしのノート本文の上書き、秘密情報の自動同期
- 添付ファイル・履歴のAI連携（別スコープで扱う）
- v1でのチャット履歴の永続化

## Phase 4開始条件

- `todo-phese4.md` の必須設計・セキュリティ・受け入れ項目を完了する。
- AI認証、保存、同期境界、プロバイダー共通契約、UI状態、テスト方針をレビューで承認する。
- 実キーを使わない自動テスト、秘密情報非露出の確認、失敗時にローカル機能を継続できることの受け入れ条件を確定する。
