# Phase 4 AI統合設計

最終更新: 2026-07-22

ステータス: 設計承認完了（D-01〜D-07承認済み、Phase 4コード実装は未着手）

## 1. 位置付け

本書は、Phase 4「AI」の実装前に決定する認証・プロバイダー・生成結果・同期境界・UI・受け入れ条件を記録する設計書です。要求範囲は [`scope-phese4.md`](scopes/scope-phese4.md)、未完了項目は [`todo-phese4.md`](../todo/todo-phese4.md)、現在状況は [`../status.md`](../status.md) を参照します。

本書の「決定結果」が`承認済み`になるまで、AI API実装、DB migration、WebDAV契約変更は開始しません。決定内容に応じて、`scope-phese4.md`、`todo-phese4.md`、[`../rules/architecture.md`](../rules/architecture.md)、[`environment.md`](environment.md) の記載を更新します。

## 2. 7項目の決定表

| ID | 決定項目 | 決める範囲 | 現状 | 決定結果 | 主な影響先 |
| --- | --- | --- | --- | --- | --- |
| D-01 | v1対象範囲・プロバイダー | 初回対応するAI司書・アシスタント・ライティング機能、対応プロバイダー、モデル選択・能力表示、優先順位 | v1はAI設定とメモ要約。初期プロバイダーはOpenRouterとGemini API。接続確認・単発テキスト生成までを対応し、モデル一覧はプロバイダーから取得する | 承認済み（2026-07-22） | `scope-phese4.md`、Provider実装、UI |
| D-02 | AI認証・秘密情報 | API Key、アクセストークン、ローカル接続情報の入力検証、OS CredentialStore、セッション限定fallback、更新・削除・再認証、HTTPS・local endpoint・proxy・redirect・timeout・retry・rate limit・費用上限、ログの非露出 | v1はOpenRouterとGemini APIのキーを個別にOS CredentialStoreへ保存し、利用不可時のみsession-onlyとする。固定HTTPS接続先のみを使い、proxy・redirect・自動retry・アプリ内の金額上限は提供しない | 承認済み（2026-07-22） | CredentialStore、設定UI、Provider transport、ログ・エラー |
| D-03 | 生成結果の保存・データモデル | 要約、タイトル、タグ、分類、関連候補、Q&A、執筆結果ごとの保存要否、正本、版・モデル・日時、削除・再生成、ユーザー確認、`revision` / CAS / 保存laneとの接続 | v1のメモ要約は現在表示中の画面だけで保持する一時結果とし、Markdown・SQLite・索引・WebDAV outboxへ保存しない。チャット履歴の永続化はv2以降で別途設計する | 承認済み（2026-07-22） | 要約UI、D-07（DB schema・migration・WebDAV契約の変更なし） |
| D-04 | WebDAV同期境界 | AI生成結果を同期対象外のまま維持するか、新しいentity・manifest/object・outbox・conflict・CAS・migrationを追加するか | v1は既存の同期対象だけを維持する。要約、AI設定、credential reference、AI資格情報は端末ローカルとし、WebDAV同期しない | 承認済み（2026-07-22） | `webdav-sync.md`、D-07（schema・同期Service・復旧・競合の変更なし） |
| D-05 | Provider共通契約・実行制御 | Go側interface、型付きrequest/result/error、認証方式、endpoint差分、streaming、cancel、partial response、context長、入力・出力上限、timeout、retry、quota・rate limit・費用上限 | v1はGo側Provider adapterで接続確認・モデル一覧・単発要約を提供する。Geminiは保存を伴わない`generateContent`、OpenRouterはZDR・データ収集拒否・下流fallback無効で実行する | 承認済み（2026-07-22） | Go Application Service、Wails API、Provider adapter、D-07 |
| D-06 | UI・データフロー | 未設定、接続確認、生成中、cancel、partial、success、failure、retry、rate limit、offline、送信前確認、機密ノート・長文・大量ノート・空結果の挙動 | v1は設定画面のAIタブとエディターの要約操作を使う。本文を毎回確認して送信し、結果は画面だけに表示してコピー・破棄する | 承認済み（2026-07-22） | Vue Component、Pinia、API client、通知、アクセシビリティ、D-07 |
| D-07 | テスト・受け入れ条件 | 実キーを使わないprovider test double、契約・UI状態・秘密情報非露出・データ保全・競合テスト、ローカル機能継続、受け入れ記録とCI確認 | 実endpointを使わないGo adapter/service/Wails APIテストと、mock Wails APIを使うfrontend Storeテストを追加する。CIと受け入れ記録には秘密情報を残さない | 承認済み（2026-07-22） | Go tests、frontend scripts、CI、受け入れ記録 |

## 3. 維持する既存契約

Phase 4の決定は、次の既存契約を変更しないことを前提とします。

- Markdown本文を正本とし、SQLiteはメタデータと再構築可能な索引に使う。
- AI処理開始時の`revision`をbase revisionとして保持する。
- ストリーミング途中のchunkをノート正本へ逐次保存しない。
- 生成結果を確定するときだけ、既存の`expectedRevision`・CAS・ノート単位保存laneを通す。
- v1のメモ要約は現在表示中の画面だけで保持する一時結果であり、ノート正本へ確定しない。したがってv1では`expectedRevision`・CAS・ノート単位保存laneを呼び出さない。
- 生成中にユーザー編集などでrevisionが進んだ場合は競合とし、自動上書き・自動rebaseを行わない。
- Phase 3のWebDAV契約、DB schema、migrationは、AIの設計承認なしに変更しない。
- API Key、アクセストークン、Authorizationヘッダー、プロンプト、ノート本文をSQLite、Markdown、`localStorage`、`.env`、ログ、エラー、診断情報へ保存・出力しない。
- AIが利用できない場合も、ローカル保存・編集・検索・既存同期を継続できる状態を維持する。

既存のrevision・CAS・ストリーミング契約の詳細は [`note-concurrency.md`](note-concurrency.md)、Phase 3同期契約の詳細は [`webdav-sync.md`](webdav-sync.md) を正とします。

## 4. 決定方法

各IDについて、次の順序で決定します。

1. `scope-phese4.md`、`todo-phese4.md`、既存コード・テストから確認できる事実と制約を整理する。
2. 複数の選択肢、既存データ・同期・セキュリティ・実装コストへの影響を記録する。
3. 推奨案を示す。ただし、推奨案はレビューで承認されるまで決定事項にしない。
4. 承認された案を、この表の「決定結果」と詳細記録へ反映し、承認日を記録する。
5. 決定に対応するTODOだけを完了にし、必要な場合にscope、architecture、environment、statusを更新する。

決定結果の状態は次の4種類に限定します。

| 状態 | 意味 |
| --- | --- |
| `未決定` | 選択肢または判断材料を整理中。実装へ進まない |
| `レビュー待ち` | 推奨案と影響を記録済み。承認待ち |
| `承認済み` | 設計レビューで採用が確定し、実装条件を満たす |
| `保留` | 外部条件や追加調査が必要。保留理由を記録する |

依存関係があるため、原則として`D-01`、`D-02`、`D-03`、`D-04`、`D-05`、`D-06`、`D-07`の順に確認します。D-04で同期対象を追加する場合は、Phase 3契約の変更計画を別途作成します。

## 5. 詳細決定記録

### D-01: v1対象範囲・プロバイダー

- 状態: `承認済み`
- 決定内容:
  - v1の機能範囲はAI設定とメモ要約とする。要約は接続確認後の単発テキスト生成で返す。
  - 初期対応プロバイダーはOpenRouterとGemini APIとする。Groq、OpenAI、Ollama、LM Studioは後続に回す。
  - v1の対応レベルは接続確認と単発テキスト生成までとする。ストリーミング、部分応答、RAG、バッチ処理、他のAI司書・アシスタント・ライティング機能はv1の対象外とする。
  - 利用者によるキャンセルと構造化出力はv2で対応する。
  - モデル一覧は、選択中のプロバイダーから利用者の明示操作で取得する。固定のモデル一覧は持たない。
  - 能力表示は、表示名、モデルID、v1の要約に使用可能か、取得できる場合の入力・出力上限、取得日時、現在の利用可否に限定する。取得できない値は`不明`とし、価格・速度・ツール呼び出し・画像/音声対応・ストリーミング対応はv1で表示しない。
  - 一覧にない選択済みモデルは`利用不可・再選択が必要`と表示し、自動的に別モデルへ切り替えない。
- 選択肢: 6プロバイダーを初回対応にする案、OpenRouterとGemini APIだけを初回対応にする案、単一プロバイダーだけを初回対応にする案を比較した。モデル一覧は固定リストとプロバイダー取得を比較した。
- 採用理由: v1を設定と要約の一連の利用フローに絞り、初期プロバイダーを限定することで、認証・実行契約・UI・テストの複雑さを抑える。プロバイダーごとに異なるモデル情報は、共通の最小表示項目へ正規化する。
- 既存データ・API・UIへの影響: 要約結果の保存・反映はD-03、認証情報の保存はD-02、Provider adapterと実行契約はD-05、UI状態はD-06、テスト詳細はD-07で決定する。この決定だけではDB schema、migration、WebDAV契約を変更しない。
- セキュリティ・データ保全上のリスクと軽減策: 接続確認とモデル一覧取得でノート本文を送信せず、資格情報をログへ出さない。秘密情報の保存方式と接続先制御はD-02で確定する。モデルの自動切り替えを行わず、意図しない送信先・費用・生成結果の変化を防ぐ。
- 追加・更新するテストと受け入れ条件: 実キー・実endpointを使わないtest doubleで、モデル一覧の正規化、`不明`表示、利用不可モデルの再選択要求、接続確認、単発要約を検証する。詳細なテスト設計はD-07で確定する。
- 承認日・承認者: 2026-07-22・ユーザー

### D-02: AI認証・秘密情報

- 状態: `承認済み`
- 決定内容:
  - v1で受け付ける秘密情報は、OpenRouterの通常の推論用APIキーと、Gemini APIで有効な認証キーまたはGemini API向けに制限済みのAPIキーのみとする。OpenRouterのOAuth/PKCE、管理APIキー、BYOK設定、ローカルプロバイダー接続情報はv1の対象外とする。
  - OpenRouterとGemini APIの資格情報はプロバイダーごとに分離する。非秘密のプロバイダーID、モデルID、ランダムなcredential referenceだけをアプリ設定として保持し、実キーはWebDAVとは分離したAI用OS CredentialStoreへ保存する。
  - 保存時はOS CredentialStoreへの保存を常に試みる。利用できない場合だけプロセス内のsession-only保持へ切り替え、再起動後は再入力を求める。平文のSQLite、Markdown、`localStorage`、設定ファイル、`.env`、環境変数へfallbackしない。
  - キー入力欄は常に空で表示し、保存済みキーの一部・長さ・値を表示しない。プロバイダーごとのキー削除と、全AIキーの削除を提供する。キー更新では新しいキーの保存と非秘密設定の更新が成功してから旧キーを削除する。
  - キーは空文字、改行、制御文字を拒否する。形式や接頭辞だけで有効性を判定せず、接続確認で検証する。モデルはD-01で決めたプロバイダー取得一覧から選択する。
  - 接続先はOpenRouterとGemini APIの固定HTTPSホストだけとし、利用者がendpoint、HTTP、TLS無効化、redirect、proxyを設定・使用することはv1で許可しない。接続確認とモデル一覧取得はノート本文を送らない読み取り専用操作とし、失敗時に資格情報や設定へ副作用を残さない。
  - 生成の自動retryは行わず、利用者の明示的な手動retryだけを許可する。アプリ内の金額上限やProviderの請求・キー管理は行わず、利用上限はProvider側で設定する。timeout、出力上限、rate limitの実行契約はD-05で確定する。
  - APIキー、Authorizationヘッダー、`x-goog-api-key`、ノート本文、プロンプト、生成結果、raw provider error bodyはログ、通知、診断、クラッシュ情報へ出力しない。Providerのdebug機能も有効化しない。
- 選択肢: OS CredentialStoreとsession-only fallbackを採用する案、平文の設定ファイル・`.env`・SQLiteへ保存する案、キーを毎回入力させる案を比較した。通信経路は固定HTTPS直結と、カスタムendpoint・proxy許可を比較した。
- 採用理由: 既存のWebDAV資格情報と同じOS secure store・session-onlyの安全境界を再利用しつつ、AIとWebDAVの資格情報・送信先を分離する。初期のクラウドプロバイダーを固定し、意図しない送信先、資格情報転送、再試行による費用増加を防ぐ。
- 既存データ・API・UIへの影響: AI設定の永続化では非秘密設定とcredential referenceだけを扱い、実キーをWails APIの設定取得結果へ返さない。CredentialStoreの共通化・AI用service namespace、Providerごとの認証ヘッダー、timeout・エラー型はD-05で実装する。DB schema、migration、WebDAV契約はこの決定だけでは変更しない。
- セキュリティ・データ保全上のリスクと軽減策: OS secure store利用不可時は明示的にsession-onlyと通知し、再起動後の無自覚な資格情報再利用を防ぐ。プロバイダー変更時に他方のキーを再利用せず、redirect・proxy・raw error bodyを禁止してキーやノート本文の漏えい経路を減らす。
- 追加・更新するテストと受け入れ条件: 実キー・実endpointを使わないtest doubleで、プロバイダー別資格情報の分離、session-only fallback、更新・削除、失敗時の副作用なし、HTTP/redirect/proxy拒否、ログ・エラーの秘密情報非露出、手動retryだけを検証する。詳細なテスト設計はD-07で確定する。
- 承認日・承認者: 2026-07-22・ユーザー

### D-03: 生成結果の保存・データモデル

- 状態: `承認済み`
- 決定内容:
  - v1のメモ要約は、要約を要求した時点で表示しているノートのUIメモリだけに保持する一時結果とする。Markdown本文への追記、SQLite、検索索引、操作journal、別成果物、WebDAV outboxには保存しない。
  - 要約生成の開始時に`baseRevision`をメモリ上だけで保持する。完了時に現在のrevisionと異なる場合は「古い内容から生成された要約」と表示するが、ノート本文への自動適用、自動rebase、自動retryは行わない。利用者は必要な結果だけを明示的にコピーできる。
  - 画面遷移、ノート切替、再読み込み、アプリ終了で要約結果を破棄する。生成元モデル、生成日時、入力本文、プロンプト、生成結果もv1では履歴・キャッシュとして保持しない。
  - v1ではチャット履歴を保持しない。v2以降で履歴を永続化する場合は、保存先、保持期間、個別・一括削除、プライバシー表示、端末間同期、migration、競合を実装前に別のD-03追補として承認する。保存方式はこの決定では仮定しない。
  - v1の決定によりDB schema、migration、Markdown形式、WebDAV同期契約の変更は行わない。
- 選択肢: 要約をMarkdown本文へ追記する案、SQLiteにローカル保存する案、画面上の一時結果だけにする案を比較した。チャット履歴はv1で永続化する案とv2以降へ分離する案を比較した。
- 採用理由: v1は要約の生成価値を確認する段階であり、正本・索引・同期・削除・競合の設計を同時に増やさない。一時結果なら既存ノートの整合性を変えず、履歴が持つ本文・プロンプト・応答の保持責任もv1へ持ち込まない。
- 既存データ・API・UIへの影響: 要約UIは一時状態と`baseRevision`を管理し、本文更新API、Repository、SQLite、検索索引、操作journal、outboxを呼び出さない。D-04ではv1に同期対象がないことを前提に、将来の履歴などを同期対象にするかだけを検討する。
- セキュリティ・データ保全上のリスクと軽減策: 要約画面を閉じると結果を再利用できないが、ノート本文への意図しない書込み、生成結果の端末残留、同期による漏えい・競合を防ぐ。履歴の永続化を検討する際は、D-02の秘密情報非露出方針に加え、本文・プロンプト・応答の保持と削除の境界を明示する。
- 追加・更新するテストと受け入れ条件: D-07で、要約生成後にMarkdown・SQLite・索引・操作journal・outboxへ書込みがないこと、ノート切替・再読み込みで一時結果が破棄されること、revision不一致時に自動反映しないこと、利用者の明示コピー以外で本文が変わらないことを検証する。
- 承認日・承認者: 2026-07-22・ユーザー

### D-04: WebDAV同期境界

- 状態: `承認済み`
- 決定内容:
  - v1ではPhase 3のWebDAV同期対象を、既存のノート、ノートブック、タグ、ノートタグだけに維持する。AI用のentity、manifest/object、change set、outbox、conflict、snapshotは追加しない。
  - v1の一時要約、入力本文、プロンプト、生成結果、チャット履歴、AI設定のプロバイダーID・モデルID・credential reference、AI API KeyはいずれもWebDAV同期対象外とする。AI設定と資格情報は端末ローカルに保持し、端末ごとに設定・接続確認・モデル選択を行う。
  - D-03によりv1の要約は永続化しないため、要約生成では既存同期のoutboxを作成・更新しない。同期のpull・競合解決・復旧・再アップロード・再ダウンロードでもAI用データを扱わない。
  - WebDAVのformat、manifest、object、entity型、schema version、migration、CAS・競合契約、同期Serviceの実装は変更しない。
  - v2以降でチャット履歴その他のAIデータを永続化する場合は、まずD-03追補でデータモデルを承認し、その後に同期するかをD-04追補で決定する。同期を選ぶ場合は新entity、識別子、payload、保持・削除、outbox、3-way競合、migration、rollback、端末再生成を別途承認する。
- 選択肢: v1からAI要約・設定を新しい同期entityとして追加する案、要約だけを同期する案、AI関連データをすべて同期対象外に維持する案を比較した。
- 採用理由: v1に永続化されるAI結果がなく、既存同期は4種類のentityだけを検証・適用する契約である。credential referenceを同期しても別端末のOS CredentialStoreには対応するキーがなく、設定だけが到着して誤認や再認証失敗を招くため、AI設定も端末ローカルとする。
- 既存データ・API・UIへの影響: `webdav-sync.md`のAI関連対象外を明確化するだけで、同期形式・schema・migration・Sync Service・既存の同期受け入れ結果は変更しない。D-06で、AI設定が端末ごとに必要であることを画面上で分かるようにする。
- セキュリティ・データ保全上のリスクと軽減策: 端末ごとにAI設定とキーを再入力する手間はあるが、ノート本文・プロンプト・生成結果・credential referenceがWebDAVへ送信されること、端末間の無効な資格情報参照、AI履歴の同期競合を防ぐ。
- 追加・更新するテストと受け入れ条件: D-07で、AI設定の変更と要約生成がsync outbox・manifest・objectを作成または更新しないこと、同期のupload/download/競合解決がAI関連データを扱わないこと、別端末ではAI設定・資格情報の再設定が必要なことを実キーなしで検証する。
- 承認日・承認者: 2026-07-22・ユーザー

### D-05: Provider共通契約・実行制御

- 状態: `承認済み`
- 決定内容:
  - Go側のProvider adapterは、`ListModels`、`CheckConnection`、`GenerateSummary`の3操作だけを公開する。UI/Wails APIは共通のモデル情報、要約結果、型付き安全エラーだけを受け取り、プロバイダー固有のrequest/response、HTTPヘッダー、raw error bodyはadapter外へ出さない。
  - 接続確認はノート本文を送らない読み取り専用操作とする。OpenRouterは通常の推論用キーで`GET /api/v1/key`を呼び、認証成否だけを使って応答本文を破棄する。Gemini APIは`x-goog-api-key`ヘッダーでモデル一覧を1件取得し、認証成否だけを使う。モデル一覧は利用者の明示操作で全ページを取得し、OpenRouterではtext入力・text出力のモデル、Geminiでは`generateContent`対応モデルだけをv1候補に正規化する。
  - 要約生成は単発・非ストリーミングとし、OpenRouterは`/api/v1/chat/completions`へ`stream: false`、Gemini APIはstable `v1`の`models/{model}:generateContent`へ保存を明示的に無効化した要求を送る。固定の要約指示と現在ノートの本文だけを送信し、会話履歴、ツール、プラグイン、URLコンテキスト、ファイル、画像・音声、構造化出力、Provider debug、利用者指定の任意プロンプトは送らない。
  - OpenRouterでは選択済みの具体的モデルIDだけを使い、`openrouter/auto`、モデル配列によるfallback、別モデルへの自動切替を許可しない。requestのprovider設定は`zdr: true`、`data_collection: "deny"`、`allow_fallbacks: false`を必須とする。条件に合う下流endpointがない場合は要約を実行せず、`AI_MODEL_UNAVAILABLE`として再選択を求める。
  - 本文入力はUTF-8で12 KiBまで、生成出力は最大512 tokensとする。超過時は送信・自動切詰め・分割・バッチ化を行わず`AI_INPUT_TOO_LARGE`を返す。temperature、top-p、seedなどの生成パラメータはv1で指定・UI公開しない。
  - 接続確認とモデル一覧取得のdeadlineは10秒、要約生成のdeadlineは60秒とする。自動retry、バックグラウンドretry、別モデル・別providerへのretryは行わない。`Retry-After`が得られる場合だけ安全な待機秒数として返し、利用者が明示的に再試行する。アプリ全体で同時に実行できる要約生成は1件とし、実行中は`AI_BUSY`を返す。利用者によるキャンセルはv2対象のままとし、アプリ終了時だけ内部contextを中止できる。
  - 正常な完了状態で空白を除くtext出力だけを要約結果として採用する。途中終了、出力上限到達、空結果、非text出力、不正JSONは結果を破棄してエラーにする。共通エラーは少なくとも`AI_AUTH_FAILED`、`AI_MODEL_UNAVAILABLE`、`AI_INPUT_TOO_LARGE`、`AI_RATE_LIMITED`、`AI_TIMEOUT`、`AI_NETWORK_UNAVAILABLE`、`AI_PROVIDER_UNAVAILABLE`、`AI_BUSY`、`AI_INVALID_RESPONSE`へ正規化し、raw provider messageは含めない。
- 選択肢: ProviderごとのAPIをUIから直接呼ぶ案、Go adapterで差分を吸収する案を比較した。GeminiのInteractions APIをstatefulに使う案、保存を伴わない単発`generateContent`を使う案を比較した。OpenRouterの既定routingを使う案と、ZDR・データ収集拒否・下流fallback無効を必須にする案を比較した。
- 採用理由: v1の用途は単発要約だけであり、Provider固有の会話状態・ストリーミング・再試行を持ち込む必要がない。GeminiのInteractions APIは既定でInteractionを保存するため、保存を伴わない単発生成を選ぶ。OpenRouterは下流providerへのroutingを行うため、ZDR・データ収集拒否・fallback無効を要求して、ノート本文の再送と保持範囲を最小化する。
- 既存データ・API・UIへの影響: 新規の外部通信はGo Application ServiceとProvider adapterに限定する。本文更新、Repository、Markdown、SQLite、検索索引、操作journal、WebDAV outboxには接続しない。D-06で型付きエラーと`Retry-After`の表示、送信前確認、実行中状態を定義済みとする。
- セキュリティ・データ保全上のリスクと軽減策: OpenRouterの厳格なprivacy条件により選べるモデルが少なくなるが、条件外の下流providerへの送信・同一本文の自動再送を防ぐ。Geminiの単発要求は保存を明示的に無効化し、会話IDや履歴を保持しない。両adapterで資格情報、本文、プロンプト、生成結果、raw error bodyをログ・通知・診断へ出さない。
- 追加・更新するテストと受け入れ条件: D-07で、Provider test doubleを使い、固定endpoint・認証ヘッダー・接続確認時の本文非送信、モデル正規化、Gemini保存無効、OpenRouterのZDR/データ収集拒否/fallback無効、12 KiB/512 tokens/deadline、単一実行、retryなし、型付きエラー、途中・空・不正応答の破棄、秘密情報非露出を検証する。
- 参照: [Gemini API versions](https://ai.google.dev/gemini-api/docs/api-versions)、[Gemini API logs and datasets](https://ai.google.dev/gemini-api/docs/logs-datasets)、[OpenRouter provider routing](https://openrouter.ai/docs/guides/routing/provider-selection)、[OpenRouter ZDR](https://openrouter.ai/docs/guides/features/zdr)
- 承認日・承認者: 2026-07-22・ユーザー

### D-06: UI・データフロー

- 状態: `承認済み`
- 決定内容:
  - 設定画面に`AI`タブを追加し、既存の同期設定と同じ下書き・接続確認・`適用`/`OK`の流れを使う。プロバイダーごとにOpenRouterまたはGemini APIを選び、API Key入力欄は常に空のpassword入力とする。接続確認とモデル一覧取得は下書きのキーで実行して失敗時に保存しない。`適用`/`OK`でのみ非秘密設定と資格情報を更新し、保存済みキー・一部・長さは表示しない。プロバイダー別キー削除と全AIキー削除には明示確認を求め、session-only時はその旨を表示する。
  - モデル一覧は利用者の`モデルを更新`操作で取得する。接続確認に成功したプロバイダーのv1候補から選択し、選択済みモデルが一覧にない場合は`利用不可・再選択が必要`として要約を送信しない。未取得項目は`不明`と表示し、モデルの自動切替・自動更新は行わない。
  - エディターのツールバーに`AIで要約`操作を置く。AI設定・接続確認・モデル選択が未完了の場合は外部送信せず、AI設定タブを開く。ゴミ箱内ノート、空本文、UTF-8で12 KiBを超える本文も送信せず、本文の自動マスキング・切詰め・分割・バッチ処理を行わない。機密ノートの自動判定はせず、送信前確認で利用者が判断する。
  - 要約操作は、既存の未保存draftを先に正常に保存してから、現在のノートの本文と`revision`を不変のスナップショットとして取得する。保存失敗または競合時は送信しない。タイトル、他ノート、添付、画像、会話履歴は送信しない。確認ダイアログの表示中に対象ノートまたは選択済みプロバイダー・モデルが変わった場合も送信しない。
  - 要約を送信するたびに確認ダイアログを表示する。送信先プロバイダー・モデルID、送信するのは固定の要約指示と現在ノート本文だけであること、結果は保存・同期されず画面上だけに保持されることを示す。確認を記憶する選択肢や自動送信は設けない。
  - 要約実行中はアプリ全体で1件だけを許可し、他の実行操作は`AI_BUSY`として無効化する。利用者によるキャンセル・部分応答はv1で表示しない。ノート切替・画面遷移・再読み込みでは要約結果を直ちに破棄し、すでに開始した要求は完了まで待って結果を無視する。アプリ終了時だけ内部contextを中止できる。
  - 正常結果は現在のノートに属する一時パネルだけに表示し、`コピー`と`破棄`だけを提供する。ノート本文への挿入・保存・自動反映は行わない。開始時の`baseRevision`と現在revisionが異なる場合は「古い内容から生成された要約」と警告しつつコピーを許可する。
  - 操作元には直近の状態をinlineで表示し、失敗後に対象ノートが表示中でない場合だけ共通通知を使う。`AI_AUTH_FAILED`、`AI_MODEL_UNAVAILABLE`、`AI_INPUT_TOO_LARGE`、`AI_RATE_LIMITED`、`AI_TIMEOUT`、`AI_NETWORK_UNAVAILABLE`、`AI_PROVIDER_UNAVAILABLE`、`AI_BUSY`、`AI_INVALID_RESPONSE`は日本語の安全な案内へ対応付ける。本文・プロンプト・生成結果・API Key・raw provider messageは表示・通知・ログに含めない。`Retry-After`がある場合だけ安全な待機時間を表示し、待機後も利用者が`再試行`を押して再度送信確認を通過した場合だけ実行する。
- 選択肢: AI設定を即時保存する案と下書き・接続確認・明示適用にする案、要約前の毎回確認と初回だけの確認を比較した。未保存本文をそのまま送る案と保存成功後のsnapshotを送る案、要約を本文へ挿入する案と一時パネルでコピーだけ許可する案を比較した。
- 採用理由: 既存の同期設定、Pinia、通知、保存laneの責務境界を再利用し、資格情報と送信内容の意図しない保存・送信を防ぐ。毎回の確認と保存済みsnapshotにより、ノートの対象・送信範囲・`revision`を利用者とアプリの両方で明確にできる。自動マスキングは誤検知・見逃しのどちらも避けられないため、v1には持ち込まない。
- 既存データ・API・UIへの影響: `SettingsModal`、既存の設定Store、`NoteEditor`、新規のAI用memory Store/API client、Wails APIを接続する。要約結果・モデル一覧の表示状態はUIメモリだけとし、Markdown、SQLite、検索索引、操作journal、WebDAV outbox、`localStorage`へ保存しない。要約前の既存draft保存だけは通常のNote Service・保存laneを使う。
- セキュリティ・データ保全上のリスクと軽減策: 本文を外部へ送るリスクは、毎回の送信前確認、固定送信範囲、12 KiB上限、固定provider/model、秘密情報・本文・結果の非露出で低減する。保存失敗・競合時に送信を止め、結果を自動適用しないことで、ノートの正本・revision/CAS・同期状態を壊さない。
- 追加・更新するテストと受け入れ条件: D-07で、AI設定下書きの破棄・接続確認失敗時の非保存、モデル再選択、送信前確認、空・ゴミ箱・長文の送信阻止、保存失敗・競合時の送信阻止、本文だけのsnapshot、単一実行、切替後の結果破棄、stale表示、コピーだけ、型付きエラー・手動retry、通知・ログの非露出、アクセシブルな状態表示を実キーなしで検証する。
- 承認日・承認者: 2026-07-22・ユーザー

### D-07: テスト・受け入れ条件

- 状態: `承認済み`
- 決定内容:
  - 実キー、実endpoint、実ユーザーのノート本文、実プロンプト、実生成結果を自動テスト・手動受け入れ・CI・fixture・ログへ使わない。テスト用endpointを利用者設定、環境変数、`.env`、ビルド設定として公開しない。Provider adapterのHTTP transportまたはProvider interfaceだけをテストで注入し、固定host/pathへの要求を実通信せずに検証する。
  - GoのProvider adapter契約テストで、OpenRouter/Gemini APIの固定host/path/method、認証ヘッダー、接続確認時の本文非送信、モデル一覧のページング・v1候補正規化、Geminiの保存無効・非ストリーミング、OpenRouterの具体的モデル/ZDR/データ収集拒否/fallback無効を検証する。12 KiB入力、512 tokens出力、10秒/60秒deadline、retryなし、`Retry-After`、空・途中・非text・不正JSONの破棄、型付き安全エラーを検証する。
  - GoのApplication Service・CredentialStore・Wails APIテストで、プロバイダー別資格情報、session-only fallback、更新・削除、失敗時の非保存、同時要約1件、要約前の保存成功/失敗/競合、`baseRevision`、安全エラーだけの返却を検証する。合成した秘密情報・本文・raw provider error markerがAPI応答、エラー、ログ、設定、SQLite、Markdown、操作journal、WebDAV outboxに現れないことを確認する。
  - データ保全テストで、要約成功・失敗・切替・stale・破棄の前後に、ノート本文、SQLiteメタデータ、検索索引、操作journal、同期outbox/manifest/objectがAI結果によって更新されないことを確認する。AI失敗後も通常のローカル保存・編集・検索・既存同期が継続することを確認する。
  - フロントエンドは既存のNodeスクリプト方式に合わせ、mock Wails APIを差し替える`test:ai-store`を追加する。AI設定下書き、接続確認失敗時の非保存、モデル再選択、送信前確認、保存済みsnapshot、単一実行、状態遷移、手動retry、`Retry-After`待機表示、切替後の結果破棄、stale表示、コピーだけ、通知・ログ非露出を検証する。Vueコンポーネント用の新規テスト依存は追加しない。
  - CIでは既存の`wails build -clean`、`go test ./...`、frontend typecheck、既存frontend scriptsを維持し、`npm --prefix frontend run test:ai-store`を追加する。CIはネットワーク資格情報を必要とせず、providerへの実通信をしない。
  - 手動受け入れは実キーなしで、AI設定タブのpassword入力・設定破棄・未設定時の送信阻止・送信確認ダイアログ・安全な状態表示・コピー/破棄導線・キーボード操作を確認する。Provider成功・失敗・競合の全状態はtest doubleを使う自動テストを受け入れ根拠とする。実装完了後は`todo-phese4.md`に実施日、対象HEAD、OS、実行コマンド、test double結果、手動確認結果だけを記録し、秘密情報・endpoint・本文・プロンプト・生成結果・raw errorを記録しない。
- 選択肢: 実プロバイダーへの統合テスト、テスト用endpointをアプリに持たせる案、HTTP transport/Provider fakeだけをテストで注入する案を比較した。Vueのテスト基盤を追加する案と、既存のNode scriptでStoreをmockする案を比較した。
- 採用理由: 実通信と実キーを使う受け入れは費用・保持・再現性・秘密情報露出のリスクがあり、v1の安全境界そのものを検証できない。既存のGo `httptest`・fake CredentialStore、frontend scriptのmock方式を拡張すれば、依存追加なしにProvider契約とUI状態を再現できる。
- 既存データ・API・UIへの影響: `internal/ai`相当の新規テスト、`app_test.go`相当のWails APIテスト、frontend AI Store test script、`frontend/package.json`、`.github/workflows/ci.yml`、実装後の`todo-phese4.md`受け入れ記録を更新する。DB schema、migration、WebDAV同期形式をテストのために変更しない。
- セキュリティ・データ保全上のリスクと軽減策: fixtureやassertionの文字列にも合成値以外を入れず、テスト出力に秘密情報や本文を出さない。test doubleの注入はテスト用constructor/transportに閉じ、実行時のカスタムendpoint・proxy・環境変数fallbackには接続しない。テスト失敗時もfixture DBと一時ディレクトリを破棄し、既存データを触らない。
- 追加・更新するテストと受け入れ条件: 上記のGo・frontend・CI・手動受け入れを実装し、すべて成功することをPhase 4完了条件とする。実装前の本決定は、D-01〜D-06のすべての安全境界をテスト可能な受け入れ項目へ対応付ける。
- 承認日・承認者: 2026-07-22・ユーザー

### 未承認項目用テンプレート

決定表の各IDを承認するときは、次の形式で理由と影響を残します。

```markdown
### D-XX: 決定項目

- 状態: `未決定` / `レビュー待ち` / `承認済み` / `保留`
- 決定内容: 未記入
- 選択肢: 未記入
- 採用理由: 未記入
- 既存データ・API・UIへの影響: 未記入
- セキュリティ・データ保全上のリスクと軽減策: 未記入
- 追加・更新するテストと受け入れ条件: 未記入
- 承認日・承認者: 未記入
```

## 6. Phase 4開始ゲート

次の条件をすべて満たすまで、Phase 4のコード実装を開始しません。

- D-01〜D-07が`承認済み`または、対象外とする理由付きで確定している。
- [`todo-phese4.md`](../todo/todo-phese4.md) の必須設計・セキュリティ・受け入れ項目が完了している。
- 生成結果の保存・破棄・再生成・競合の扱いが、revision/CAS・操作journal・ノート単位laneと矛盾しない。
- AI利用失敗時もローカル保存・編集・検索・既存同期を継続できることをテスト方針で保証している。
- 実キー、秘密情報、実endpointを使わずにProvider契約とUI状態を検証できる。
- DB schema、migration、WebDAV契約の変更が必要な場合、その影響・rollback・受け入れ条件が別途承認されている。

## 7. 対象外

- 本書の追加だけでAI API、Provider、Wails API、UI、DB migration、テストを実装すること。
- Phase 3の`webdav-sync.md`を未承認のまま変更すること。
- 実キー、実endpoint、ユーザーのノート本文をfixture・ログ・受け入れ記録へ持ち込むこと。
