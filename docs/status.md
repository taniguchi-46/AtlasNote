# プロジェクト状況

最終更新: 2026-09-26

## CLI・MCP・統合ターミナル再編 Stage B（2026-09-26）

既存`internal/organize.Service`の一括候補生成と30分・最大5件sessionを再利用し、非破壊の`organize.analyze`／`organize.get_candidates`をCLIとMCPへ追加した。外部解析sessionは保存空間、クライアント／MCPセッション、明示公開note／Notebook集合へ束縛し、既定公開0件、別session、期限切れ、偽造ID、不正cursor、保護・ロック・ゴミ箱・scope／revision変更をfail-closedで拒否する。両操作は`P`と`R1`を必須とする。外部読み取りが同一Note Serviceで保持済みのcontent accessをOrganization Serviceが再利用し、保護状態変更との相互待ちを防ぐ。MCPでは公開対象だけを解析エンジンへ渡し、Notebook・タグ候補も公開対象から導出可能な集合へ限定する。broken-link判定用の実在ID集合は解析可能集合から分離し、restricted解析では明示公開note ID以外のscope外実在／不存在、保護、ロック、ゴミ箱を同じ候補結果として扱い、対象情報を外部結果へ漏らさない。通常GUI／unrestricted解析は従来のbroken-link判定を維持する。外部sessionは既存GUI Applyから適用できない。候補は件数とJSONサイズの両方でページングし、DB schema、Markdown、既存GUI、保存・同期・バックアップは変更していない。

資料に記録済みの5,000ノート解析約20.5秒に対して従来15秒IPC timeoutでは切断されるため、同期処理のままclient／server write timeoutを60秒へ延長した。独立ジョブ、ストリーミング、外部進捗プロトコルは追加していない。Stage B対象Goテスト、既存Stage A／GUI回帰、`go vet ./...`、Frontend整理・コンテンツロック回帰、typecheck、lint、production build、bindings生成付きWails Windows/amd64 clean buildとNSIS作成、`git diff --check`は成功した。隔離したAPPDATA／LOCALAPPDATAでの`go test ./... -count=1`はStage Bを含む全対象パッケージが成功し、既知の`internal/appcleanup` Windows pin／cleanupテストだけがユーザープロファイルへのアクセス拒否で失敗した。実Claude Code／Copilot接続、Wails GUI手動操作、パッケージ版Windows実機確認、5,000ノート再計測はStage A〜E後の最終統合テストへ持ち越す。

## CLI・MCP・統合ターミナル再編 Stage 0（2026-09-26）

`codex/support` を `codex/pre-phase5-future-features` へfast-forward統合し、統合HEADから `codex/cli-mcp-terminal-rearchitecture` を作成した。Go全テスト、Frontend全34テスト、型チェック、別一時出力先へのbuildは成功。既存 `frontend/dist` への通常buildは使用中ファイルの `EBUSY` で未完了。現行コードと仕様案の差分は [再編仕様書](development/AtlasNote_CLI_MCP_Rearchitecture_Spec.md#stage-0-実コード照合2026-09-26) に記録した。CLI/MCP/IPCと統合ターミナルは未実装で、旧AI・整理GUIと保存済みAIデータは維持する。

## CLI・MCP・統合ターミナル再編 Stage A（2026-09-26）

既存Note Service、検索、バックリンク、Local Intelligenceを再利用する共通読み取り境界と、アクティブ保存空間へ束縛した認証済みローカルIPCを実装した。`AtlasNote.exe` の読み取りCLIとstdio MCPから `notes.list/get/search/backlinks/related`、`notebooks.list`、`tags.list` を利用できる。監査後、MCPをinitialize時の保存空間・接続先・明示公開note／Notebookへ固定する短命セッションへ分離し、既定公開0件、未公開メタデータ／本文／検索結果の除外、アプリ再起動・保存空間切替後の自動再接続禁止、実装済みMCP `2025-06-18`だけの版表明へ修正した。IPC単独の起動失敗ではGUIを維持し、外部応答中は既存mutation gateで本文・タグ・revision・ゴミ箱状態を固定する。保護・ロック・ゴミ箱、別保存空間scope、権限、revision、検索／リンク索引整合性は本体側でfail-closedに検証する。書き込み、整理候補、保存空間切替、ロック解除、統合ターミナルは未実装で、Stage B以降へは進んでいない。監査修正後、隔離したテスト用APPDATA／LOCALAPPDATAでの `go test ./... -count=1`、`go vet ./...`、MCP公開scope・版交渉・再接続禁止、IPC失敗時GUI維持、本文／タグ／revision／ゴミ箱状態の並行整合性回帰、Frontend全 `test:*`、typecheck、lint、別一時出力先へのproduction build、bindings生成付きWails Windows/amd64 clean build、`git diff --check` は成功した。先行実装時の実行ファイル経由CLI／MCP smokeも成功している。Wails GUIの手動操作は未確認で、Claude Code CLIが環境に存在しないため実Claude Code接続も未確認。入出力と接続設定は [再編仕様書のStage A実装契約](development/AtlasNote_CLI_MCP_Rearchitecture_Spec.md#stage-a-実装契約2026-09-26) を正とする。

再監査後、MCPセッションをstdio正常終了時に即時失効し、30分の有効期限、発行時の期限切れ回収、認証時の期限切れ拒否を追加した。セッション権限は親接続と公開scopeの共通部分へ限定し、親がR0だけなら公開scopeがあってもR1を付与しない。正常な起動・終了を65回繰り返す回帰、期限切れ・失効・R0親接続を確認した。Windows descriptorは継承を禁止した保護DACLに現ユーザーだけの単一ACEを持ち、他ユーザー向け読取ACEがないことを実生成ファイルで確認した。5,000ノートの`notes.list --limit 1`とGUI保存の並行計測では保存待ち約755msを再現したため、必要件数とnext cursor判定用1件が集まった時点で一覧走査を止める最小修正を行い、同条件を約22msへ短縮した。全Goテスト、`go vet ./...`、Frontend typecheck／lint、Wails Windows/amd64 clean build、標準入出力を明示したWindows実行ファイル同士のMCP initialize smokeは成功した。Claude Code CLIは環境に存在しないため実クライアント接続は未確認で、Stage B以降へは進んでいない。

## Local Intelligence 初期版（2026-09-25）

コードレビュー指摘の候補上限とNotebook範囲、検索の複数語根拠、非表示時の不要な関連候補要求を修正。80件超の範囲外候補を含む合成fixture、表示切替とscope切替のFrontend回帰を追加した。

既存FTS検索の一致範囲と短語抜粋を補正し、リンク・共通タグ・語句一致による件数制限付き関連候補APIとバックリンク内の表示／AI参照追加を実装。索引revision・正本ハッシュ・保護・ロック・ゴミ箱を再確認し、読み取り専用とした。テスト用APPDATA/LOCALAPPDATAに隔離した`go test ./...`、Frontend関連Store・AIチャット・コンテンツロックテスト、Frontend build／lint、bindings生成付きWails Windows/amd64 buildは成功。今回の修正前に行った5,000ノート・2KB本文の合成ベンチマークは一回の計測で約107.7ms、約2.05MB、30,803 allocs。今回の候補SQL修正後は未計測。Computer Useによる実Wails画面では既存ノートの関連候補・抜粋・根拠表示、「参照に追加」の成功表示、「ノートを開く」の遷移を確認した。ロック時の表示消去、大量データでの実画面応答性、候補品質の広い評価と、整理センター前タスクの手動受け入れは未確認。詳細は[`development/local-intelligence.md`](development/local-intelligence.md)。

## 整理とAIの共通サポートパネル（2026-09-24）

`SupportWorkspace`で整理／AIを1つの枠へ統合した。共通ヘッダーのタブとドック／浮動切替、×最小化、アイコン再開を追加し、エディターの兄弟要素に配置した。ノート未選択またはAI無効でも整理を開ける。タブ切替では両機能をアンマウントせず、整理scopeごとの解析・選択・結果とAIの下書き・timeline・処理を保持する。AIとバックリンクの整理導線は対象ノートscopeを共通パネルに開き、既存sessionでは不要な再解析をしない。ロック時は整理sessionとAIの表示内容・進行中の応答を無効化する。右／下ドック寸法の希望値は既存設定を使い、狭い領域では表示だけ下側へ移す。Frontend typecheck／lint／production build、整理・AI Workspace・AIチャット・コンテンツロック・ショートカットの回帰テスト、bindings生成付きWails Windows/amd64 build、`git diff --check`が成功。Wails実画面での手動受け入れは未確認。

整理候補の大量表示を避けるため、一覧は100件ずつ追加表示し、一括選択・適用を表示中の候補に限定した。タグ0件の保存空間ではノートごとのタグ取得を省いた。`test:organization`、Frontend typecheck／lint／production build、Go整理テストは成功。合成データのGo解析では2KB本文の5,000ノートで変更前約21.5秒・割当量403.7MB、変更後約20.5秒・341.0MB、候補はいずれも31,250件だった（一回ずつの測定）。候補生成と返却は依然として全件で、Wails転送・描画を含む実画面応答性とデータ分布を変えた計測は未確認。詳細は整理TODOの計測記録を参照する。

続く応答性対応では、解析中のノート確認件数・進捗バーと候補生成段階を件数のみのWailsイベントで通知・表示する。要求IDとscope／ロック世代で古い通知を無視する。Go／Frontendの進捗回帰、Frontend typecheck、bindings生成付きWails Windows/amd64 buildが成功。解析結果は引き続き全件生成後に返すため、実Wails画面での進捗表示・操作応答と候補生成段階の待ち時間は未確認。

候補一覧の種類別件数を全候補の単一走査で集計し、表示中の選択件数を`Set`で照合するよう変更した。`test:organization`とFrontend typecheckが成功。実Wails画面での効果測定は未確認。

## 整理センター第一段階（2026-09-23）

共通の整理パネルに加え、AIWorkspace／バックリンクへ独立した通常・フローティングパネルを追加。保存空間scopeとノートscopeごとに候補・選択・結果・適用状態を保持し、scope間の古い応答をsession単位で破棄する。ロック時は全scopeを無効化し、遅延応答で候補を復元しない。小型パネルにも対象・変更前・変更案・理由を表示してから候補単件／一括適用する。全保存空間、Notebook直下、子孫Notebook、対象ノートと直接の関連先を解析できる。候補はタイトル、Notebook分類・移動、未分類検出、タグ付与、重複／空ノートのゴミ箱移動、相互リンクと、リンク切れ・孤立・関連・重複タグの情報提示。同一ノートの候補をバッチ適用し、Note Serviceのrevision/CAS・journal・ノート単位queue、タグCASを通して保護・ロック・ゴミ箱・関連先・stale状態を再確認する。失敗は再試行可能、競合は再解析を案内する。データベースschema変更なし。

設計は [`development/organization-center.md`](development/organization-center.md)、Pre-Phase 5 scopeは [`development/scopes/scope-pre-phase5.md`](development/scopes/scope-pre-phase5.md)、TODOは [`todo/todo-pre-phase5-organization.md`](todo/todo-pre-phase5-organization.md) を正とする。自動検証とWails v2.10.1 Windows/amd64 buildの実行結果はこのTODOへ記録する。Computer Useはブラウザー面しか公開せずWindowsアプリ画面を操作できなかったため、実画面手動受け入れと実測性能評価は未確認。

## ドキュメント整理（2026-09-21）

完了済みPhase 2／3のスコープ・TODO・受け入れ記録を `docs/archive/` へ移動した。現行の設計契約（WebDAV同期、revision・CAS、検索、タグ）は `docs/development/` に維持し、各索引と参照先をアーカイブへ更新した。開発環境方針と技術スタックの重複は、セットアップ手順を`setup.md`、採用技術を`tech-stack.md`、判断基準・秘密情報を`environment.md`へ分離して整理した。

## ブランチ分離（2026-09-20）

Mermaidの描画・挿入・専用編集は `codex/mermaid-full` に分離した。通常開発用の `codex/pre-phase5-future-features` では、既存のMermaidフェンスを通常のコードブロックとして表示・編集し、ソースを保存する。未コミットだった11ファイルの編集も `codex/mermaid-full` の `eafb13e` に保持済み。自動保存・添付画像・削除・ロック・AI履歴は継続する。以下の過去のMermaid実装・検証記録は分離前／対応ブランチの記録であり、通常開発ブランチの提供機能ではない。

分離後の検証: `npm run frontend:build`（型チェック含む）、`test:code-block`、`test:serializer`、`test:markdown-safety`、`test:table-copy`、`test:auto-save`、`test:note-delete`、`test:content-locks`、`test:shortcuts`、`test:operation-logger`、`node scripts/test-attachments.mjs` は成功。表コピーのテストには既存の画像幅モジュールのコンパイルを補い、Mermaidフェンスと空段落のRich往復・編集・Undo／Redoを専用描画なしで回帰確認した。Wails実画面の手動確認は未実施。

## 分離前の作業記録

Mermaid図の図種別カタログと、要素・接続・注釈を行単位で編集する「かんたん編集」を追加した。未対応の行は原文を保持し、ソースタブから継続して編集できる。Mermaidは表示ハンドルの上下ドラッグで10〜200%を変更する表示専用倍率とし、現在の100%を従来の50%相当へ合わせ、図上の右ボタン操作または「編集」ボタンでソース編集ダイアログを開く。添付画像の幅は右下ハンドルで変更し、管理参照を変えずMarkdown本文に保存する。`npm --prefix frontend run test:mermaid`、`npm run frontend:typecheck`、`npm run frontend:lint`は成功した。実Wails画面での上下ドラッグ・右ボタン編集・かんたん編集の手動受け入れは未実施。

P2追加機能を実装。自動保存のON／OFFと`Ctrl+S`明示保存、Mermaid挿入・表示倍率調整・Markdown入力ルール、PNG／JPEGのノート本文へのドラッグ＆ドロップ、完了済みAI会話のローカル履歴保存・一覧・再開・新規チャット、ユーザー向け利用ガイドの初回ノート生成とHelp画面のFAQ／トラブルシューティング整理を追加した。入力・保存・同期・AI履歴の世代競合を保護し、履歴一覧は全件取得とした。Frontend typecheck／lint、関連Frontend回帰、`internal/ai`／`internal/app`のGoテストは成功。`go test ./... -count=1`は既存の`internal/appcleanup` Windows固有テストが環境のアクセス拒否で失敗しており、Wails実画面の手動受け入れは未確認。

自動保存中の追加入力で先行保存の成功応答が破棄され、古いrevisionによって次の保存が自己競合する不具合を修正。成功した保存は下書き世代にかかわらず保存済みsnapshotへ反映し、最新下書きの消去・保存完了通知だけを世代一致で制御する。既存の実Storeテスト（`test:note-delete`）へ保存中入力、表示切替、最新下書き保持、本当の外部revision競合の回帰ケースを追加して成功を確認した。既存の競合コピーは変更しない。実Wails画面での連続入力による確認は未実施。

追加再レビューのHigh（先行ロックの表示更新完了が後続ロックの入力ガードを解除する競合）を修正。StoreがAppの表示更新までawaitし、同じbusy区間のfinallyで取得元エディタのガードを解放する。通知watcherを廃止し、単一／batchの表示更新保留中の後続拒否、保存・API・表示更新失敗、空結果、重複、アンマウント後始末を回帰確認した。content-locks／Mermaid／保存・削除・serializerの関連回帰と型チェック込みFrontend buildが成功。実OS IME／Wails画面での手動受け入れは未確認。

Priority 1の添付画像保存を実装。PNG／JPEGのサイズ・寸法・画素数・実デコード検証、管理参照、Rich／Markdown貼り付け、autosave／履歴接続、再試行可能な失敗保持、暗号化・ロック再エンコード、再帰バックアップ、ネイティブZIP保存を追加した。追加レビューのHigh 1〜5として、暗号化格納上限の共有計算、添付保存と同期・バックアップ排他、保護添付ZIPの明示確認・再検証、画像貼り付け操作世代ガード、既存WebDAV形式への添付manifest／本体／outbox／復旧接続を実装した。保護された保存空間の同期拒否は維持する。Go添付・暗号化・同期往復・バックアップ・App統合テスト、Frontend typecheckを確認済み。Wails実画面の手動貼り付けと複数OSでのネイティブダイアログ確認は未確認（2026-09-13）。

Pre-Phase 5の将来機能として、削除操作を下書きflush→同一ノートlane操作の順に統一し、保存／CAS失敗時の本文・draft保全、対象単位のアクティブノート終了、部分成功、選択応答の世代保護を追加した。Mermaid Rich表示は図のみ＋編集ダイアログへ更新し、fenced/raw/HTML貼り付け、動的フェンスコピー、Tiptap Undo・autosave接続を追加した。コンテンツロック中のIME入力はdraft保存からロック後の表示反映まで保護する。`test:note-delete`、`test:serializer`、Mermaid renderer／NodeView／実Tiptap統合テスト、Frontend typecheck、Frontend production buildが成功している。Wails画面全体の手動受け入れは今回未確認。

Priority 2の追加レビュー3件を修正した。両保存先が利用不能な復旧時の候補選択、複数Store間の診断記録の保持、OS原因別の日本語理由／対処を回帰テストで確認した。続くPriority 4では、構造化ノート入出力とWindowsアンインストール時の任意追加削除を実装した。`go test ./... -count=1`、Frontendのtypecheck、保存場所setup・note delete・storage spaces・backups・operation loggerのテストが成功。診断履歴のA→B→A・保持上限・起動後の破損／リンク・ロック競合もtemp fixtureで成功した。OS理由分類部分はLinux／macOS向けテストバイナリのクロスコンパイルまで確認し、他OSでの実行・手動UI・インストーラー再検証は今回未実施。commit／pushは再レビュー後に行う。

保存場所の再起動時移行失敗を`storage-recovery`へ接続し、元へ戻す／別の空フォルダへ切り替える／同じ移行を再試行する保留操作、候補の再検証、v2移行進捗の永続化、Windowsインストーラーの日本語化・安全なアンインストールを実装済み。Priority 4ではJSON／CSVの複数ノートインポート、JSON／CSV／TXTの単一ノートエクスポート、既定OFFの表示設定・キャッシュ／現ユーザーCredential Store削除選択を追加した。追加削除は通常起動・migrationから分離し、ノート、バックアップ、保存空間、復旧情報、保存場所管理情報を保持する。保存場所の検証失敗は段階・役割・OSエラー番号を含む安全な理由へ分類し、独立した上限付き診断履歴を設定／復旧画面で表示・コピーできる。Go・フロントエンドのテスト、Frontend production build、同梱Wails CLIによるclean build、NSIS 3.12のアンインストール回帰ハーネスは確認済み。実際のインストール／アンインストール操作、Windowsの「インストールされているアプリ」経由の確認は未実施（2026-09-09）。

2026-09-08のPriority 2回帰確認では、`go test ./... -count=1`、Frontendのtypecheck／lint、note delete・保存場所setup・storage space・backup・operation loggerのFrontendテスト、Frontend production build、同梱Wails CLIによるWindows clean build、NSISアンインストール回帰ハーネスが成功した。Windowsのstage linkサブテストはシンボリックリンク作成権限不足でskipされている。

## 現在のフェーズ

MVP（v0.1）の移行前必須項目とPhase 2「整理・検索」の対象機能は実装済みです。Phase 2のCI受け入れは [GitHub ActionsのCI run #29383600495](https://github.com/taniguchi-46/AtlasNote/actions/runs/29383600495) で成功しています。残課題とPhase 3への持ち越し条件は下記に分けて記録します。関連メモはPhase 4 v2へ完全移管しています。

Phase 3「同期」は、schema version 10、WebDAVクライアント、CredentialStore、durable outbox、同期Service、Joplin方式の設定UI、空同期先フェイルセーフ、安全な再アップロード/再ダウンロード復旧、ローカル自動検証、非本番の実WebDAV受け入れ、手動UI受け入れ、CI最終確認まで完了しています（2026-07-19）。実サーバーまたは同期実装の更新時は回帰確認を継続します。

要求範囲は `docs/development/scopes/scope.md`、Phase 3の同期契約は `docs/development/webdav-sync.md` を正とします。Phase 2／3の進捗・受け入れ記録は `docs/archive/` に保管します。Phase 4 v1〜v3の進捗・スコープは、各versionのscope／TODO（`scope-phese4*.md`、`todo-phese4*.md`）で管理します。v1〜v3の自動検証、最終CI、利用者による手動UI受け入れが完了したため、2026-08-24付でPhase 4完了とします。

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
- Pre-Phase 5「ノート保存空間の分割」。既存ルートを移動せず「メイン」として登録し、追加空間を内部ID配下へ作成する。空間ごとにSQLite、Markdown、WebDAV設定・outbox・競合、AIローカル設定・履歴・成果物、同期復旧、単一writer lockを分離する。設定画面の一覧から選択し、同期／AI busy確認とdirty draftのflush、対象空間の事前検証後に選択を保存して自動再起動する。保存空間の削除・改名・外部フォルダ選択は対象外。設計は`docs/development/storage-spaces.md`を正とする（2026-08-25）
- Pre-Phase 5「ロック機能」。保存空間・ノートブック・ノート単位でMarkdown本文を認証付き暗号化し、名前・タイトルは平文として保持する。保存空間は設定画面から、ノートブック・ノートは3ペインの編集ポップアップから設定でき、設定画面のロック一覧で解除・パスフレーズ変更を行う。ロック済みノート／ノートブックの選択時には、継承元を含む未解除ロックを共通ダイアログで順に要求し、キャンセル時は選択を変更しない。解除時点から固定時間で再ロックする設定（既定はアプリ終了時のみ、1/5/15/30/60分）を備え、操作では期限を延長しない。期限到来時は下書きを保存し、失敗時は鍵を保持して再試行する。保護対象はAIで利用不可とし、既存AI記録は明示確認後だけ削除する。旧平文同期先は設定前に切断し、暗号化同期形式を別途実装するまで保護済み保存空間の同期は拒否する。設計は`docs/development/content-locks.md`を正とする（2026-08-26）
- Pre-Phase 5「md・txt・HTML・JSON・CSVからインポート」。OSネイティブの複数ファイル選択から、最上位・既存ノートブック・新規トップレベルノートブックへ保存する。旧形式は1ファイル1ノート、JSON／CSVは1ファイル最大1,000ノートとして扱う。タイトルは自動・ファイル名・先頭見出し・メタデータから選択でき、構造化レコードはtitleを優先し、候補がない場合はファイル名へフォールバックする。HTMLは許可した文書構造だけをMarkdownへ変換し、`hidden`属性を持つ本文と子孫、raw HTML、属性、スクリプト、CSS、外部リソースを保存しない。既存のNote Serviceを通じてMarkdown、SQLite、操作journal、検索・リンク索引、同期outbox、コンテンツロックを維持し、変換失敗はファイル単位、構造化入力はファイル全体の検証失敗、保存失敗は成功済みノートを保持する部分成功として扱う。インポート中は保存空間の切替を拒否する。設計は`docs/development/note-import.md`を正とする（2026-09-09）
- Pre-Phase 5「単一ノートのHTML・PDF・JSON・CSV・TXTエクスポート」。dirty draftを既存保存laneでflushし、保存済みMarkdownとrevisionをsnapshotとしてGo側で再検証した後、OSネイティブ保存ダイアログの選択先へ原子的に出力する。HTMLはallowlist再サニタイズ、CSP、固定CSSを持つ自己完結文書とし、PDFは同梱Noto Sans JPを使うA4縦の直接生成、JSON／CSVはGo側の正本snapshot生成、TXTは安全な可視テキスト変換とする。外部リソース・画像データは含めず、保護ノートは平文出力警告と明示確認を必須とする。エクスポート中の保存空間切替と重複実行を拒否し、保存先フルパス・本文・payloadをログや結果へ返さない。設計は`docs/development/note-export.md`を正とする（2026-09-09）
- Pre-Phase 5「アプリ内グローバルショートカット」。Undo／Redoを含む全操作を設定画面で変更・解除・初期化でき、既定は本文Undo`Ctrl + Z`、Redo`Ctrl + Y`、新規ノート`Ctrl + N`、検索`Ctrl + F`、設定`Ctrl + ,`とする。MarkdownとRich双方の本文履歴を既存autosaveへ接続し、Tiptapの固定Undoキーマップを無効化した。設定はversion付き端末ローカル`localStorage`、本文履歴はメモリ限定とし、ノート切替、外部再読込、競合破棄、モード切替、ロック時に破棄する。OS全体のシステムホットキーとAgent適用結果のUndoは対象外。設計は`docs/development/keyboard-shortcuts.md`を正とする（2026-08-28、手動UI受け入れ未完了）
- Pre-Phase 5「自動バックアップ・バックアップ復元」。アクティブ保存空間のSQLite・Markdownを設定されたアーカイブルートへ24時間間隔で世代保存し、manifestのSHA-256とSQLite integrityを検証する。設定画面の既定ON切替、最大10世代の自動バックアップ、最大3世代の復元前安全用バックアップ、プレビュー確認トークン、stage／pending marker、起動時swap・rollback、同期復旧との競合防止を実装した。詳細は`docs/development/backup-restore.md`を正とする（2026-08-28）
- Pre-Phase 5「物理保存場所選択」。データルートとバックアップ保存領域のOSフォルダ選択、空の既定領域での初回`setup-required`、既存領域の引き継ぎ、再起動時の非破壊移行、環境変数固定時のUI制限を実装した。論理保存空間ごとの外部フォルダ割り当ては対象外。詳細は`docs/development/storage-locations.md`を正とする（2026-08-29）
- 2026-09-08に、物理保存場所選択の候補・Apply・再起動時移行で、パス単位の候補検証、RootValidationErrorによる段階／役割／OSエラー番号の分類、markerだけに依存しないdata root再検証、失敗時の保留状態保持を追加した。保存場所エラーの独立診断履歴は安全なallowlistと上限を持ち、設定／復旧画面から表示・コピーできる。Wails生成バインディング、Frontend回帰テスト、Go全体テスト、NSIS権限回帰ハーネスで確認済み。
- 2026-09-06に、保存場所移行のデータstage・分離バックアップstage・同一targetルートのバックアップstage・source読み取り拒否をWindowsの実共有ハンドルで検証した。共有中はowned marker・stage・保留マーカーを保持し、ハンドル解放後に再試行して完了すること、誤ったバックアップ残骸検査パスを修正したことを確認した。marker不一致、look-alike sibling、marker linkも変更なしで拒否する。
- 2026-09-06に、同一／分離アーカイブルートのApp統合テストで、自動バックアップからの復元を再起動で適用し、復元されたノートのrevisionを基準にtrash後のrevisionで完全削除できること、他ノート・自動バックアップ・復元安全用バックアップ・保留マーカーが保持されることを確認した。Frontendの実Pinia Storeテストでは、trash後のstale lock応答がrevisionを巻き戻さず、最新のlock応答だけを反映することも確認した。
- 2026-09-09に、構造化JSON／CSVの全件検証付き複数ノートインポート、JSON／CSV／TXTのcanonical snapshotエクスポート、CSV数式セル無害化を実装した。Windowsアンインストールでは、既定OFFの追加削除ページから、現在ユーザーの既知のWebViewデータとCredential Store参照だけを削除できる保守コマンドを追加し、通常起動・migration・ノート／バックアップ／保存場所管理情報の削除を行わないことをfixture／mockとNSISコンパイルで確認した。
- Mermaid専用実装は`codex/mermaid-full`に保持（通常開発ブランチではソース表示のみ）。
- Notebook階層の循環防止
- migration境界、SQLite接続設定、Critical / High項目のCI検証
- Richエディタ変換時のraw HTML無効化と危険な属性・URLの回帰テスト
- schema version 3の `notes.revision` migration、既存行のrevision `1` backfill、Note / Summaryモデルへのrevision追加
- schema version 5の検索状態`content_mtime_ns` migrationと既存行の初回hash再照合
- schema version 6の`tags` / `note_tags` migration、Unicode正規化・case-foldによる同名防止、外部キーCASCADE
- schema version 7の`note_links` / `note_link_state` migration、target/source逆引きINDEX、外部キーCASCADE
- schema version 8〜10の同期状態・outbox・conflict・HTTP許可・同期間隔・フェイルセーフ・TLS・proxy設定migration
- schema version 16で、旧版のノートブック削除時に残った存在しない`notebook_id`参照を起動時に安全に切り離し、ゴミ箱操作・コンテンツロック状態取得・削除時の同期payloadを整合
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
- 実サーバーまたは同期実装の更新時は、`docs/archive/phase3/todo.md` の受け入れ記録に従って回帰確認します。
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
- 保護された保存空間の暗号化WebDAV同期形式（現行WebDAVでは保護本文・保護添付の同期を拒否）
- Phase 3のWebDAV同期の確定設計は `docs/development/webdav-sync.md` を正とし、完了済みの進捗・受け入れ記録は `docs/archive/phase3/todo.md` に保管する。更新時の回帰確認のみ継続する。
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
npm --prefix frontend run test:backups
npm --prefix frontend run test:note-batch
npm --prefix frontend run test:note-selection
npm --prefix frontend run test:note-delete
npm --prefix frontend run test:note-export
npm --prefix frontend run test:note-import
npm --prefix frontend run test:notifications
npm --prefix frontend run test:tags
npm --prefix frontend run test:notebook-hierarchy
npm --prefix frontend run test:note-list-view
npm --prefix frontend run test:serializer
npm --prefix frontend run test:table-copy
npm --prefix frontend run test:markdown-safety
npm --prefix frontend run test:operation-logger
node frontend/scripts/test-attachments.mjs
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
| `docs/archive/phase2/scope.md` | Phase 2の詳細スコープ（アーカイブ） |
| `docs/development/scopes/scope-phese4.md` | Phase 4 v1の実装前詳細スコープ |
| `docs/development/scopes/scope-phese4-v2.md` | Phase 4 v2のAI司書・実行体験スコープ |
| `docs/development/scopes/scope-phese4-v3.md` | Phase 4 v3のAIアシスタント・ライティング・履歴スコープ |
| `docs/development/ai-chat.md` | 単一AIチャット、context、Ask／Agent、ツール実行・保存境界 |
| `docs/development/implementation-plan.md` | 現在フェーズの実装順序 |
| `docs/development/webdav-sync.md` | Phase 3 WebDAV同期の確定設計 |
| `docs/development/storage-spaces.md` | 保存空間のディレクトリ、台帳、分離境界、再起動切替 |
| `docs/development/backup-restore.md` | 自動バックアップ、完全性検証、再起動時の復元・rollback |
| `docs/development/note-export.md` | 単一ノートのHTML・PDF出力、snapshot再検証、ロック、原子的保存 |
| `docs/development/note-import.md` | md・txt・HTML・JSON・CSVの安全な変換、全件検証、保存契約 |
| `docs/archive/phase3/todo.md` | Phase 3の同期設計・実装TODO（アーカイブ） |
| `docs/todo/todo-phese4.md` | Phase 4 v1の実装前課題・受け入れTODO |
| `docs/todo/todo-phese4-v2.md` | Phase 4 v2の実装・検証TODO |
| `docs/todo/todo-phese4-v3.md` | Phase 4 v3の実装・検証TODOと完了条件 |
| `docs/development/note-concurrency.md` | revision、競合検出、保存キューの確定仕様 |
| `docs/development/search-index.md` | Markdown全文検索の索引方式、更新、再構築設計 |
| `docs/development/search-api.md` | 検索API、ページング、入力検証、エラー契約 |
| `docs/development/tag-design.md` | タグの制約、migration、API、実装・検証状況 |
| `docs/archive/phase2/todo.md` | Phase 2の実績・残課題（アーカイブ） |
| `docs/development/beginner-guide.md` | 初学者向け開発ガイド |
| `docs/development/setup.md` | 開発環境セットアップ |
| `docs/development/tech-stack.md` | 採用技術 |
| `docs/rules/architecture.md` | アーキテクチャとデータ設計 |
| `docs/rules/conventions.md` | 実装規約 |
| `docs/rules/BRANCHING.md` | Git運用ルール |
| `docs/rules/ai.md` | AI Agent共通ガイド |

### 優先度4レビュー修正（2026-09-09）

追加削除の対象を既知のWebView leaf領域に限定し、保存先重複の事前拒否とハンドルによるパス差し替え防止を追加。本人確認を通常利用時のSID記録とWindowsセッション／実行トークンの照合へ変更。終了処理とアプリ活動マーカーの解放を子プロセス起動前に完了させ、別保存空間の同時利用は保持。従来MD／TXT／HTMLの2 MiB入力上限を復元し、CSV／JSONの32 MiBと各本文2 MiBを維持。fixture／mock回帰とNSIS実引数検証を追加。実機UAC・WebView2配置・インストールからの一連の手動受け入れは未確認。詳細は`docs/development/windows-distribution.md`、`docs/development/note-import.md`を参照。
検証結果: 一時APPDATAへ隔離した `go test ./... -count=1`、Frontend typecheck・note-import・note-export、NSIS uninstall・追加削除dispatch／SID回帰、差分検査が成功。隔離前の全Goテストは旧配置の実プロファイルを参照して7件失敗したため、その結果は成功扱いにしない。既存installer-optionsハーネスはコンパイル成功後、Windowsのセキュリティ検出でEXE起動が遮断され、実行検証は未完了。保護設定の変更や検出回避は行っていない。
