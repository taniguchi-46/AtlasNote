# プロジェクト状況

最終更新: 2026-08-25

## 現在のフェーズ

MVP（v0.1）の移行前必須項目とPhase 2「整理・検索」の対象機能は実装済みです。Phase 2のCI受け入れは [GitHub ActionsのCI run #29383600495](https://github.com/taniguchi-46/AtlasNote/actions/runs/29383600495) で成功しています。残課題とPhase 3への持ち越し条件は下記に分けて記録します。関連メモはPhase 4 v2へ完全移管しています。

Phase 3「同期」は、schema version 10、WebDAVクライアント、CredentialStore、durable outbox、同期Service、Joplin方式の設定UI、空同期先フェイルセーフ、安全な再アップロード/再ダウンロード復旧、ローカル自動検証、非本番の実WebDAV受け入れ、手動UI受け入れ、CI最終確認まで完了しています（2026-07-19）。実サーバーまたは同期実装の更新時は回帰確認を継続します。

要求範囲は `docs/development/scopes/scope.md`、Phase 3の同期契約は `docs/development/webdav-sync.md`、Phase 3の実装順序は `docs/development/implementation-plan.md`、進捗・受け入れ記録は `docs/todo/todo-phese3.md` を正とします。Phase 4 v1〜v3の進捗・スコープは、各versionのscope／TODO（`scope-phese4*.md`、`todo-phese4*.md`）で管理します。v1〜v3の自動検証、最終CI、利用者による手動UI受け入れが完了したため、2026-08-24付でPhase 4完了とします。

## 実装済み

- High着手条件のrevision・CAS・競合検出・ノート単位保存キューは実装・最終検証完了（2026-07-12）

- Wails v2 + Go + Vue 3 + TypeScript + Vite のデスクトップアプリ基盤
- Markdown本文とSQLiteメタデータを組み合わせたローカル保存
- Note / Notebook Repository、Service、Wails API
- 3ペインUI、ノートブック、お気に入り、ピン留め、ゴミ箱
- Markdown / RichエディタとMarkdown serializer
- SQLite / Markdown操作ジャーナル、補償処理、起動時復旧
- 自動保存、dirty draft、保存失敗時の再試行・破棄、終了前flush
- ノート選択の非同期応答逆転防止
- データディレクトリ単位の単一writer保証
- Pre-Phase 5「ノート保存空間の分割」。既存ルートを移動せず「メイン」として登録し、追加空間を内部ID配下へ作成する。空間ごとにSQLite、Markdown、WebDAV設定・outbox・競合、AIローカル設定・履歴・成果物、同期復旧、単一writer lockを分離する。設定画面の一覧から選択し、同期／AI busy確認とdirty draftのflush、対象空間の事前検証後に選択を保存して自動再起動する。削除・改名・外部フォルダ選択・暗号化は対象外。設計は`docs/development/storage-spaces.md`を正とする（2026-08-25）
- Notebook階層の循環防止
- migration境界、SQLite接続設定、Critical / High項目のCI検証
- Richエディタ変換時のraw HTML無効化と危険な属性・URLの回帰テスト
- schema version 3の `notes.revision` migration、既存行のrevision `1` backfill、Note / Summaryモデルへのrevision追加
- schema version 5の検索状態`content_mtime_ns` migrationと既存行の初回hash再照合
- schema version 6の`tags` / `note_tags` migration、Unicode正規化・case-foldによる同名防止、外部キーCASCADE
- schema version 7の`note_links` / `note_link_state` migration、target/source逆引きINDEX、外部キーCASCADE
- schema version 8〜10の同期状態・outbox・conflict・HTTP許可・同期間隔・フェイルセーフ・TLS・proxy設定migration
- Atlas Note固有のformat/head/manifest/object、strong ETag、tombstone、durable outboxによるWebDAV同期
- 単一WebDAV URL、同期間隔、読み取り専用設定確認、Apply/OK/戻るdraft方式の同期設定UI
- HTTPS既定・明示的HTTP許可、custom root CA、明示的TLS error ignore、HTTP/HTTPS proxy、redirect拒否
- target一致時だけの資格情報再利用、OS CredentialStore保存とsession-only fallback通知
- 空同期先フェイルセーフ、確認token付き条件更新によるlocal再アップロード
- remote全件を別SQLite/notesへ検証し、起動時に旧vaultをbackupしてswap/rollbackする再ダウンロード復旧
- タグのRepository / Service / Wails API、構造化タグエラー、フロントAPI / Pinia Store
- ノート編集画面の既存タグ選択ポップアップ、サイドバーでのタグ一覧表示・作成・改名・削除
- ノートリンクのMarkdown記法・抽出、SQLiteリンク索引、バックリンクAPI・Store・UI
- `expectedRevision`・構造化競合結果モデル、Repositoryの原子的な更新・削除CAS
- Serviceの通常更新・完全削除へのCAS接続、Wails / Storeからの `expectedRevision` 受け渡し
- ノートブック削除に伴うノートのtrash・切り離し時のrevision増加
- Wails APIの構造化競合結果とフロントAPIの型付き `NoteRevisionConflictError`
- Storeの `conflicted` draft状態、競合情報とローカル下書きの保持
- 永続revisionと区別したフロントdraft世代 `draftVersion`
- NoteEditorの保存競合・下書き保持表示
- 競合draftを破棄してサーバー最新版を再読み込む解決操作
- 競合draftを同じノートブックの新規ノートへコピー保存する解決操作
- autosave・メタデータ更新・削除を直列化するノート単位の操作lane
- autosave失敗laneの停止・手動再開、対象別 `flush`
- 保存要求数による正確な `isSaving` 表示
- ノート操作laneと保存要求カウンターの専用回帰テスト
- contentful SQLite FTS5 + trigramによるタイトル・本文検索、ページング、入力検証、再構築可能な索引
- 検索API、検索Store/UI、検索失敗時の共通通知と再試行アクション
- ノート・ノートブック・検索Store/APIの操作別エラーコード、共通通知、再試行アクション
- SHA-256 hashによる外部Markdown編集検知、revision更新、検索索引再構築
- Markdown欠落のMissingNotes報告とrename後の孤児ファイル隔離
- ノート一覧の固定上限付きページング、Store・一覧UIの追加読込
- 起動復旧のMarkdown存在確認をノートごとの`Stat`から管理ファイル一覧の一括取得へ変更
- 起動復旧・検索・一覧の大量データベンチマークと計測手順（`docs/development/performance.md`）
- 検索状態へのMarkdown mtime保存migration、mtime一致時の索引再利用、変更時hash照合フォールバック
- Markdown/Rich変換の空段落、code fence、URL、多重markの境界テスト
- batch操作の完了ID・失敗IDを保持する部分成功処理と、UIイベントのPromise rejection処理
- `noteAutoSave.ts`によるautosave coordinator分離とunexpected rejectionの失敗lane処理
- 本文を含めないoperationログ（note ID、処理段階、エラー分類のみ）
- 単一タグ遷移、解除・0件表示
- ノート一覧の許可リスト付き並び替え（更新日時、作成日時、タイトル）
- 「最近更新した」一覧（ローカル日付の当日00:00〜翌日00:00未満、`updated_at`基準、ゴミ箱除外）
- ノートブックのドラッグ＆ドロップ移動（循環配置防止、ルート移動）
- 表全体のMarkdown / Richコピー（Markdown入り`text/plain`・Rich貼り付け用`text/html`出力、標準MIME型、特殊文字・改行テスト）
- Phase 4 v3のAIアシスタント／AIライティング基本経路、schema version 12のローカル履歴・成果物とversion 13の要約履歴、明示保存・個別／一括削除、stale／orphaned評価、WebDAV非同期境界テスト。2026-08-23にWails公開API→Service→Repository→一時SQLiteのライフサイクル、version 10既存データ保持、version 12→13専用rollback、Provider失敗後のローカル保存・検索・同期outbox継続を追加検証し、2026-08-24に空結果・候補なし・長文のFrontend通し異常系、最終CI、手動UI受け入れまで完了した
- Phase 4のAIワークスペースを単一チャットtimelineへ刷新。開いているノートの固定context chip、追加ノート／Notebook検索scope、要約・文章作成6種・タイトル・タグ・分類・関連・重複・Web検索の`＋`メニュー、Ask／Agent切替、入力欄右下の送信ボタン、候補の明示採用、構造化tool traceとtrace直後の候補カードを実装した。Web検索は実行ごとの明示確認を伴うOpenRouter Web Search／Exa固定のProvider管理ツールとする。Notebook scopeは直下ノートIDへ最大10件で解決し、本文・revisionは既存バックエンドでsnapshot化する。制限付きAgentは開いているノート本文の単一差分を構造化提案として表示し、端末ローカル設定の既定「提案のみ」では明示適用／破棄、「更新可能」では送信前確認後に検証済み提案だけをrevision/CAS・ノート単位保存queueで自動適用する。適用成功後は保存済み本文を開いているMarkdown／WYSIWYGエディタへ直ちに反映する。Agent保存中に作成されたdraftは未開始autosaveを取り消して競合として保持し、古い本文の後追い保存とエディタ上書きを防ぐ。自動適用後も変更前後の差分を現在のtimelineで確認でき、競合・保存失敗時は本文を変更しない。右側／下側配置・ドラッグ寸法、狭幅対応、履歴・成果物の既存保存境界を維持し、`test:auto-save`、`test:agent-proposal`、`test:ai-chat`、`test:ai-v3`、`test:ai-workspace`を追加・更新した。エディタ即時反映と保存中draft保護の自動テストは2026-08-23、手動UI受け入れは2026-08-24に完了した
- AIコンテキストへ全文文字数、送信済み本文バイト数、全文バイト数、切り詰め有無、作成日時、更新日時を追加し、OpenRouterの`stream`誤判定によるAgent拒否を修正した。Geminiを含むモデル能力一覧は不明値を実行時判定へ委譲し、SSEの機械可読エラーを安全なAIエラーへ分類する回帰テストを追加した（2026-08-15）。
- Phase 4 v2の大量候補pool、候補採用異常系、全保存境界、キーボード操作契約を2026-08-24に自動検証した。AI司書は20件上限・重複／不正候補除外、保存失敗・revision競合・ノート切替・cancel時の非適用、partial／prompt／candidate／resultのMarkdown・SQLite・検索索引・操作journal・WebDAV outbox非保存を確認した。Assistantは正式差分レビューで検出した生成中revision変更とcancel応答待ち中clearの競合を修正し、stale結果・Agent提案・履歴を採用せず、request IDと送信lockを終端まで維持する再現テストを追加した。

## Phase 2の完了範囲

- 既存検索UIへの実検索処理の接続（完了）
- タイトル検索、本文全文検索、タグ条件による通常一覧遷移（完了）
- タグの追加、編集、削除、ノートへの付与・解除、単一タグの通常一覧遷移（完了。タグ名検索と全文検索へのタグ条件は対象外）
- ノートリンク・バックリンク（完了）
- テーブルコピー（完了）

## Phase 2で確定した設計

- revision、競合検出、保存キューの仕様は `docs/development/note-concurrency.md` で確定済み
- 全文検索の索引方式はcontentful SQLite FTS5 + trigramに確定済み
- 検索API、ページング、入力検証、エラー形式は `docs/development/search-api.md` で確定済み
- タグのデータモデルと制約（`docs/development/tag-design.md`で確定・実装済み）
- ノートリンク・バックリンクの記法、抽出規則、更新境界は設計・実装済み。関連メモの当時の未完了項目はPhase 4 v2のAI司書へ移管し、2026-08-24に受け入れを完了した。
- 検索とタグ遷移の画面状態、および並び替えとの組み合わせは実装済み。
- schema version 3〜7のmigration、既存データへの影響、rollback方法を確認済み

## Phase 2 CI受け入れ結果

- 2026-07-15の`dev-Phase2`、commit `5dc5df4`に対するCI run #29383600495が成功した。
- Wails build、Go tests、Frontend typecheck、serializer、autosave、note selection/delete、notebook hierarchy、note operation queue、batch、notifications、tags、operation logger、note links、note list view、table copy、Markdown safetyの全ステップが成功した。
- Phase 2 CIの結果はPhase 2の完了記録であり、Phase 3の受け入れ判定は下記のPhase 3 CI結果と実WebDAV受け入れ記録を根拠とする。

## Phase 3 CI受け入れ結果

- `dev-phese3` の受け入れ対象HEAD（commit `a84203673a09bea1d45a021da0d1e7745236a5d0`）に対する [GitHub Actions CI run #29658225886](https://github.com/taniguchi-46/AtlasNote/actions/runs/29658225886) が成功した（2026-07-18）。
- Wails build、Go tests、Frontend typecheck、同期を含むFrontendテストの全ステップが成功した。

## 継続課題

- 大量ノート時の性能確認（ベンチマーク、一覧APIのページング、Store・一覧UIの追加読込、起動復旧の差分検知、5,000件基準値の記録まで完了。Phase 3受け入れ後も同期・一覧更新の比較を継続する）
- 競合解決UIのコンポーネントテスト（Phase 3受け入れ後もUI変更時に追加確認する）
- Rich機能を追加する際のserializer round-tripテスト（Rich serializer変更時のみ対応し、Phase 3同期の開始条件にはしない）

## Phase 3受け入れ・Phase 4完了記録

- WebDAV同期の設計レビューと未確定事項の決定は完了済みです。
- Phase 2のCI受け入れ条件、Phase 3のCI、非本番の実WebDAV相互運用、手動UI受け入れを確認済みです。Phase 3受け入れは完了とします。
- 実サーバーまたは同期実装の更新時は、`docs/todo/todo-phese3.md` の受け入れ記録に従って回帰確認します。
- Phase 4 v1はD-01〜D-07の設計承認、実装、保存/同期境界テスト、CI、ローカル受け入れを完了しています（2026-07-27）。v1の初期プロバイダーはOpenRouterとGemini APIで、固定HTTPSの接続確認・モデル一覧・単発テキスト要約だけを提供します。Phase 4全体の完了条件はv2のAI司書・実行体験とv3のAIアシスタント・ライティング・ローカル履歴までを含み、2026-08-24のv3受け入れ完了をもってPhase 4完了としました。GitHub ActionsのD-07 CIは[run #30229339977](https://github.com/taniguchi-46/AtlasNote/actions/runs/30229339977)で成功しています。
- Phase 4 v3の保存仕様（明示保存する会話・成果物、生成成功時に自動保存する要約履歴、SQLiteローカル管理データ、アプリケーション上の完全削除、参照元ノート削除後の保持、CI例外の扱い）は確定しています。schema version 12〜13の詳細は `docs/development/ai-integration.md` を正とします。
- Phase 4 v3の実装・検証記録は `docs/todo/todo-phese4-v3.md` で管理しています。基本実装、制限付きAgentの本文差分提案・編集権限設定（明示適用／検証済み自動適用）、適用成功後のエディタ即時反映、Agent編集権限UI分岐、AI司書とAssistant／Agentの利用者cancel、timeout応答、空結果・候補なし・長文、v2の大量候補pool・候補採用・全保存境界、Wails API／SQLite／migration／rollback／Provider失敗後継続の自動テストを完了しました。実画面の手動UI受け入れも利用者が「現状OK」と確認しています（2026-08-24）。
- 2026-08-23のローカル自動受け入れでは、実Providerを呼ばずに`go test ./...`、AI関連Frontend 9 script、Frontend typecheck、Frontend production buildが成功した。ローカル環境にWails CLIがなく未実行だったWails clean buildは、後続の最終CI run [#32722645563](https://github.com/taniguchi-46/AtlasNote/actions/runs/32722645563) で成功を確認した。
- 2026-08-24にAgent編集権限UI分岐を動的テストへ拡張し、`review-required`の自動保存0回、`auto-update`の保存1回、応答待ち中の設定変更に対する送信開始時権限の固定、提案なし・送信失敗時の保存0回、自動適用失敗時の提案保持を確認した。`test:ai-workspace`、`test:agent-proposal`、`test:ai-chat`、Frontend typecheck／production buildは成功した。
- 2026-08-24にFrontendのcancel／timeout異常系を拡張した。AI司書は利用者cancelとtimeoutをWails mock、実Pinia Store、実chat timelineまで通し、開始応答前のcancel予約、遅延完了の破棄、安全なエラー、自動retryなし、cancel API失敗時の実行継続・終端監視を確認した。Assistant／Agentは`AI_TIMEOUT`／`AI_CANCELLED`応答時の提案・自動適用・履歴保存なしと安全なtimelineエラー、Writingはtimeout時の成果物非保存を確認した。Go全体、Frontend全22 script、typecheck、production buildはローカルで成功した。この検証時点で未実装だったAssistant／Agentの利用者停止操作は、後続差分で実装・自動検証した。
- `origin/dev-phese4`のcommit `e8f6816f60e61c4de149aaa45f778812c0ad86a8`に対する最終CI [run #32722645563](https://github.com/taniguchi-46/AtlasNote/actions/runs/32722645563) は2026-08-24に全工程成功した。Wails clean build、Go tests、Frontend typecheck、全Frontend scriptが成功し、大量候補pool、候補採用、全保存境界、キーボード操作契約、Assistantの生成中revision変更・cancel中clear競合修正まで確認した。
- 2026-08-24の後続ローカル自動受け入れでは、実Providerを呼ばないWails mockで、AI司書の正常な候補0件、要約・Assistant・Writingの空／無効応答、長文contextの切り詰めmetadata、`AI_INPUT_TOO_LARGE`時の安全なエラー、非保存、自動retryなし、明示再試行を追加検証した。この差分は後続のCI run #32700754252に含まれ、全工程成功を確認した。
- 2026-08-24にAssistant／Agentの利用者停止操作を追加した。Frontend生成のrequest IDをWails API、Service、Provider contextまで相関し、context準備中のProvider非呼び出し、生成中の停止、`canceling`中の送信lock、wrong／stale／terminal済みIDの拒否、停止API失敗後の終端監視、Provider停止と生成lock解放、`AI_CANCELLED`の安全な表示、下書き・user entry保持、Agent提案・自動適用・履歴保存・自動retryなしを自動検証した。この差分は後続のCI run #32700754252に含まれ、Wails clean buildを含む全工程成功を確認した。
- 2026-08-24の最新ローカル自動受け入れでは、v2の大量候補pool、候補採用異常系、全保存境界、キーボード操作契約と、Assistantの生成中revision変更・cancel中clear競合修正を追加した。`go test ./... -count=1`、Frontend全22 script、typecheckを含むproduction build、差分検査が成功した。この追加分はcommit `e8f6816f60e61c4de149aaa45f778812c0ad86a8`へ反映し、CI run [#32722645563](https://github.com/taniguchi-46/AtlasNote/actions/runs/32722645563) の成功を確認した。
- CI run [#30527792029](https://github.com/taniguchi-46/AtlasNote/actions/runs/30527792029) はWails clean build、Go tests、Frontend typecheck、全Frontend scriptを含む全工程に成功しました（2026-07-30）。AI司書キャンセル時の生成ロックに関する既知CI例外は解消し、当時残っていた手動受け入れと追加検証も2026-08-24に完了しました。
- AI自由記述のMarkdown出力契約とAI専用DOMサニタイズ表示を実装し、通常ノートのraw HTML無効化・AI原文保存・危険なURL／外部リソース遮断を維持しました。`test:ai-markup-safety`、既存AIテスト、Go全体テスト、Frontend typecheck/buildはローカル成功済みです（2026-08-01）。当時ローカル未確認だったWails統合ビルドは、後続のCI run #32666344532で成功を確認済みです。

## 保留事項

- デスクトップアプリの対応OSと配布方式
- 添付ファイルの保存設計
- Phase 3のWebDAV同期の確定設計は `docs/development/webdav-sync.md` を正とし、実装順序を `docs/development/implementation-plan.md`、進捗・受け入れ記録を `docs/todo/todo-phese3.md` で管理する。受け入れは完了済みで、更新時の回帰確認のみ継続する。
- Phase 4 v1〜v3は承認・実装・自動検証・利用者による手動UI受け入れを完了し、2026-08-24付でPhase 4完了とする。今後はAI関連実装またはUI変更時の回帰確認として管理する。チャット履歴の永続化はv3の確定保存仕様に従う。正本は [`scope-phese4.md`](development/scopes/scope-phese4.md)、[`scope-phese4-v2.md`](development/scopes/scope-phese4-v2.md)、[`scope-phese4-v3.md`](development/scopes/scope-phese4-v3.md)、各TODO、`docs/development/ai-integration.md` とする。

## 主要コマンド

```bash
npm run frontend:build
npm run frontend:typecheck
npm run frontend:lint
npm --prefix frontend run test:auto-save
npm --prefix frontend run test:note-operation-queue
npm --prefix frontend run test:sync
npm --prefix frontend run test:storage-spaces
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:notifications
npm --prefix frontend run test:tags
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:note-list-view
npm --prefix frontend run test:serializer
npm --prefix frontend run test:table-copy
npm --prefix frontend run test:markdown-safety
npm --prefix frontend run test:operation-logger
npm --prefix frontend run test:note-links
npm --prefix frontend run test:ai-chat
npm --prefix frontend run test:ai-workspace
go test ./...
wails build
```

`frontend/wailsjs/`はGit管理対象外です。クリーンcheckout直後は、必要に応じて先に`wails build`でbindingsを生成します。

## 関連ファイル

| ファイル | 役割 |
| --- | --- |
| `docs/README.md` | ドキュメント入口と正本の役割 |
| `README.md` | プロジェクト概要 |
| `docs/development/scopes/scope.md` | Phaseごとの機能要件と対象範囲 |
| `docs/development/scopes/scope-phese2.md` | Phase 2の詳細スコープ |
| `docs/development/scopes/scope-phese4.md` | Phase 4 v1の実装前詳細スコープ |
| `docs/development/scopes/scope-phese4-v2.md` | Phase 4 v2のAI司書・実行体験スコープ |
| `docs/development/scopes/scope-phese4-v3.md` | Phase 4 v3のAIアシスタント・ライティング・履歴スコープ |
| `docs/development/ai-chat.md` | 単一AIチャット、context、Ask／Agent、ツール実行・保存境界 |
| `docs/development/implementation-plan.md` | 現在フェーズの実装順序 |
| `docs/development/webdav-sync.md` | Phase 3 WebDAV同期の確定設計 |
| `docs/development/storage-spaces.md` | 保存空間のディレクトリ、台帳、分離境界、再起動切替 |
| `docs/todo/todo-phese3.md` | Phase 3の同期設計・実装TODO |
| `docs/todo/todo-phese4.md` | Phase 4 v1の実装前課題・受け入れTODO |
| `docs/todo/todo-phese4-v2.md` | Phase 4 v2の実装・検証TODO |
| `docs/todo/todo-phese4-v3.md` | Phase 4 v3の実装・検証TODOと完了条件 |
| `docs/development/note-concurrency.md` | revision、競合検出、保存キューの確定仕様 |
| `docs/development/search-index.md` | Markdown全文検索の索引方式、更新、再構築設計 |
| `docs/development/search-api.md` | 検索API、ページング、入力検証、エラー契約 |
| `docs/development/tag-design.md` | タグの制約、migration、API、実装・検証状況 |
| `docs/todo/todo-phese2.md` | Phase 2の実績・残課題 |
| `docs/development/beginner-guide.md` | 初学者向け開発ガイド |
| `docs/development/setup.md` | 開発環境セットアップ |
| `docs/development/tech-stack.md` | 採用技術 |
| `docs/rules/architecture.md` | アーキテクチャとデータ設計 |
| `docs/rules/conventions.md` | 実装規約 |
| `docs/rules/BRANCHING.md` | Git運用ルール |
| `docs/rules/ai.md` | AI Agent共通ガイド |
