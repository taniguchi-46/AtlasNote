# Phase 4 TODO

## TODOの目的

Phase 4「AI」の実装前に、AI認証・秘密情報、生成結果の保存、Phase 3 WebDAV同期との境界、プロバイダー共通契約、受け入れ条件を確定する。未確定事項を実装で先に固定しない。

詳細スコープは [`scope-phese4.md`](../development/scopes/scope-phese4.md)、現在状況は [`../status.md`](../status.md)、既存のローカル保存・競合契約は [`../development/note-concurrency.md`](../development/note-concurrency.md)、Phase 3同期契約は [`../development/webdav-sync.md`](../development/webdav-sync.md) を正とする。

## 現状・前提

- Phase 3の受け入れは、2026-07-19に非本番の実WebDAVで動作OKを確認して完了している。実サーバーまたは同期実装の更新時は回帰確認を継続する。
- Phase 4のAI実装は未着手である。
- D-04は承認済みである。v1ではPhase 3のWebDAV同期対象を増やさず、要約、AI設定、credential reference、AI資格情報を同期対象外として維持する。AI設定と資格情報は端末ごとに設定する。
- D-01は承認済みである。v1はAI設定とメモ要約、初期プロバイダーはOpenRouterとGemini API、対応レベルは接続確認と単発テキスト生成、モデル一覧はプロバイダーから利用者の明示操作で取得する。利用者によるキャンセルと構造化出力はv2で対応する。
- D-02は承認済みである。AIキーはプロバイダーごとに分離してAI用OS CredentialStoreへ保存し、利用不可時だけsession-onlyとする。固定HTTPS接続先のみを使い、proxy・redirect・自動retry・アプリ内の金額上限はv1で提供しない。
- D-03は承認済みである。v1のメモ要約は現在表示中の画面だけで保持する一時結果とし、Markdown本文、SQLite、検索索引、操作journal、WebDAV outboxへ保存しない。チャット履歴の永続化はv2以降で別途設計する。
- D-05は承認済みである。Go側Provider adapterで接続確認・モデル一覧・単発要約を提供し、Gemini APIは保存を伴わない`generateContent`、OpenRouterはZDR・データ収集拒否・下流fallback無効で実行する。本文入力は12 KiB、出力は512 tokens、要約生成のdeadlineは60秒とする。
- D-06は承認済みである。AI設定は下書き・接続確認・明示適用で更新し、要約は送信前の毎回確認、正常保存済み本文のsnapshot、画面だけのコピー・破棄に限定する。
- D-07は承認済みである。実キー・実endpointなしのProvider fake/HTTP transport、CredentialStore fake、mock Wails APIで契約・UI状態・秘密情報非露出・データ保全を検証し、CIと受け入れ記録に秘密情報を残さない。

## Phase 4開始条件

Phase 4の実装開始前に、以下の必須項目を完了・レビュー承認する。

- [x] D-01として、`scope-phese4.md` のv1対象範囲、対象外、優先順位、初期プロバイダー、モデル選択・能力表示方針を承認する（2026-07-22）。
- [x] D-02として、AI認証・秘密情報の保存先、session-only fallback、削除・更新・再認証、接続先制御、ログ非露出方針を承認する（2026-07-22）。
- [x] D-03として、v1のメモ要約を画面上の一時結果だけに限定し、保存先、正本、版管理、削除・再生成の契約を持たないことを承認する（2026-07-22）。
- [x] D-04として、v1ではPhase 3 WebDAV同期の対象外を維持し、AI用の同期entity・outbox・schema・migrationを追加しないことを承認する（2026-07-22）。
- [x] D-05として、v1のプロバイダー共通API、モデルメタデータの正規化、単発生成、timeout、retry、rate limit、費用上限を決定する。ストリーミング・部分応答はv1対象外、利用者によるキャンセルと構造化出力はv2で対応する（2026-07-22）。
- [x] D-06として、未設定・認証失敗・利用不能・オフライン・生成中・成功・失敗・再試行のUI状態とエラー契約を決定する。キャンセル・部分応答はv1対象外とする（2026-07-22）。
- [x] D-07として、実キーを使わない自動テスト、秘密情報非露出、ノート本文の送信範囲を含む受け入れ条件を決定する（2026-07-22）。

## 1. AI認証・セキュリティ

- [x] D-02として、v1のAPIキー種別と入力検証を定義する。OpenRouterとGemini APIのキーだけを受け付け、空文字・改行・制御文字を拒否する（2026-07-22）。
- [x] D-02として、AI用OS CredentialStoreを優先し、利用不可時だけsession-only保持、再起動後の再入力を要求する（2026-07-22）。
- [ ] 平文SQLite、Markdown、`localStorage`、設定ファイル、クラッシュダンプへの秘密情報保存を防ぐテストを追加する。
- [x] D-02として、ログ、エラー、通知、診断情報でAPI Key、Authorization、`x-goog-api-key`、本文、プロンプト、生成結果、raw provider error bodyを非露出にする（2026-07-22）。
- [x] D-02として、固定HTTPS接続先のみを許可し、local endpoint、proxy、redirect、HTTP、TLS無効化をv1対象外とする。自動retry・アプリ内の金額上限も提供しない。timeout、出力上限、rate limitの実行契約はD-05で決定する（2026-07-22）。
- [x] D-02として、プロバイダー別の資格情報分離、更新・削除、再認証、secure store unavailable時の挙動を決定する（2026-07-22）。

## 2. 保存・データモデル

- [x] D-03として、v1の要約は画面上の一時結果だけに限定し、Markdown本文、SQLite、検索索引、操作journal、別成果物、WebDAV outboxへ保存しない（2026-07-22）。
- [x] D-03として、v1の要約をノート本文へ自動適用しない。revision不一致時は古い内容からの要約として表示し、利用者が必要な結果だけを明示的にコピーする（2026-07-22）。
- [x] D-03として、v1では保存処理がないため`revision` / CAS、操作journal、ノート単位lane、MarkdownとSQLiteの更新を呼び出さない（2026-07-22）。
- [x] D-03として、v1ではDB schema、migration、既存データ、rollbackへの変更がないことを確認する（2026-07-22）。
- [ ] v2以降でタイトル、タグ、分類、関連候補、Q&A、執筆結果またはチャット履歴を保存する場合は、正本、スキーマ、生成元モデル、生成日時、入力版、保持期間、削除・再生成、migration、rollbackを実装前に決定する。

## 3. WebDAV・同期境界

- [x] D-04として、v1では既存のノート、ノートブック、タグ、ノートタグだけを同期対象とし、AI生成結果の対象外を維持する（2026-07-22）。
- [x] D-04として、AI設定のプロバイダーID・モデルID・credential referenceとAI API Keyも端末ローカルとし、同期しない（2026-07-22）。
- [x] D-04として、v1ではAI用entity、manifest/object、change set、outbox、conflict、CAS、schema、migrationを追加せず、既存の `webdav-sync.md` 契約を維持する（2026-07-22）。
- [ ] D-07で、AI設定の変更と要約生成がsync outbox・manifest・objectを更新せず、同期のupload/download/競合解決がAI関連データを扱わないことを検証する。
- [ ] v2以降でAI履歴その他を永続化する場合は、D-03でデータモデルを決めた後に、同期entity、保持・削除、outbox、競合、migration、rollback、端末間再生成の契約をD-04追補として承認する。

## 4. プロバイダー共通契約

- [x] D-05として、Go側のProvider adapterを`ListModels`、`CheckConnection`、`GenerateSummary`に限定し、型付きrequest/result/errorと固定endpointを決定する（2026-07-22）。
- [x] D-01として、v1はOpenRouterとGemini APIを初期対応とし、Groq、OpenAI、Ollama、LM Studioは後続に回す。モデル一覧はプロバイダーから取得し、能力表示は共通の最小項目に正規化する（2026-07-22）。
- [x] D-05として、v1の単発生成は本文入力12 KiB、出力512 tokens、接続確認・モデル一覧10秒、生成60秒のdeadlineとし、自動retry・自動切詰め・分割・バッチ化・別モデル/別providerへのfallbackを行わない（2026-07-22）。
- [x] D-05として、接続確認、認証失敗、rate limit、quota超過、利用不能、プロバイダー切り替え時のエラーを型付き安全エラーへ正規化し、raw provider messageを返さないことを決定する（2026-07-22）。
- [ ] D-07で、実キーを使わないProvider test doubleにより、固定endpoint、本文非送信の接続確認、privacy設定、上限・deadline・単一実行・retryなし、正常結果以外の破棄、型付きエラーを検証する。

## 5. UI・受け入れ・テスト

- [x] D-06として、AI設定画面の下書き・接続確認・明示適用、更新・削除・再認証・session-only状態を定義する。キーは常に空のpassword入力とし、接続確認失敗時に保存しない（2026-07-22）。
- [x] D-06として、v1の要約について、生成中・成功・失敗・手動retry・空結果・stale表示を定義する。キャンセル・部分応答はv1対象外とする（2026-07-22）。
- [x] D-06として、本文を毎回確認して送信し、空・ゴミ箱・12 KiB超過・保存失敗・競合時は送信しないこと、機密ノートの自動判定・自動マスキング・大量ノート処理を行わないことを決定する（2026-07-22）。
- [ ] D-07に従い、実キーを使わないProvider fake/HTTP transport、CredentialStore fake、契約テスト、Wails APIテスト、AI Storeテストを追加する。
- [ ] D-07に従い、要約の成功・失敗・切替・破棄がMarkdown、SQLite、索引、操作journal、WebDAV outboxを更新せず、AI利用不能時もローカル保存・編集・検索・既存同期が継続することを受け入れる。
- [ ] D-07に従い、CIへ`test:ai-store`を追加し、実装後の受け入れ記録に使用環境・対象HEAD・テスト対象・結果だけを残す。秘密情報、実endpoint、本文、プロンプト、生成結果、raw errorを記録しない。

## 6. 実装着手後の順序（設計承認後）

- [ ] 認証・秘密情報境界とprovider共通契約を実装する。
- [ ] D-03の一時結果方針を実装し、要約生成がMarkdown・SQLite・操作journal・outboxを更新しないことを確認する。
- [ ] v1のAI設定・メモ要約のUI状態とエラー通知を実装する。ストリーミングと利用者によるキャンセルはv2で対応する。
- [ ] 保存・同期境界の受け入れテストと既存回帰テストを実行する。

## 完了条件

- 必須の設計・セキュリティ・保存・同期境界がレビュー承認済みである。
- 実キーや秘密情報をテスト・ログ・成果物へ持ち込まずに、provider契約とUI状態を検証できる。
- 生成結果の保存・破棄・再生成・競合の扱いが明文化され、既存のrevision/CAS・操作journal・同期契約と矛盾しない。
- Phase 4の実装・テスト・受け入れ結果を本TODOと `docs/status.md` に記録する。

## 絶対遵守事項

- API Key、アクセストークン、Authorizationヘッダー、ノート本文、プロンプトをログやエラーへ出さない。
- 実キーをリポジトリ、テストfixture、`.env`、SQLite、Markdownへ保存しない。
- Phase 3のWebDAV契約、DB schema、migrationを設計承認なしに変更しない。
- AI利用失敗でローカルのMarkdown正本、SQLiteメタデータ、保存lane、既存同期を壊さない。
