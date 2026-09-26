# AtlasNote CLI・MCP・統合ターミナル再編 仕様書

- 作成日: 2026-09-26
- 状態: Codexへの実装依頼用ドラフト。既存コードの検証で判明した差異は実装前に本書へ記録して修正する。
- 対象リポジトリ: `taniguchi-46/AtlasNote`
- 作業開始基準: `codex/pre-phase5-future-features` に `codex/support` を統合した状態
- 作業ブランチ案: `codex/cli-mcp-terminal-rearchitecture`（同名が存在する場合は重複しない別名を使う）
- 今回の目的: 既存の知識管理・整理エンジンを再利用し、ローカルCLI・MCP・統合ターミナルを中核とする操作体系へ段階的に移行する。

## Stage 0 実コード照合（2026-09-26）

- `git fetch origin --prune` 後、`origin/codex/pre-phase5-future-features` は `8cde360cea691a192af36a6d13d1462de770440c`、`origin/codex/support` は `21b90ea0ccc5dd6d297345df7ab55811745ec6ac`。後者が2コミット先行し、遅延は0。ローカルの `codex/pre-phase5-future-features` をfast-forwardで統合し、同じHEADから `codex/cli-mcp-terminal-rearchitecture` を作成した。リモートへのpushは行っていない。
- 現行の読み取り経路は `internal/app/app.go` のWails API → `internal/note.Service` → Repository / Markdown Store。CLI、MCP、外部プロセス用の認証済みIPCは未実装。`GetNote` は本文を返すが、`note.Service.Get` の保護判定はロック中のアクセス拒否が中心で、外部クライアント向けの「解除済み保護ノートも拒否」は別途必要。
- `NoteListInput` は `page` / `pageSize` / `tagId` / sort / `todayOnly`（最大100件/ページ）で、提案表の `notebookId` / `cursor` は既存APIにない。`SearchInput` と `BacklinkListInput` もページ番号式。`SearchInput` はNotebook範囲とゴミ箱指定を持つ。`RelatedNoteInput` はNotebook範囲と子孫指定、上限20件を持つ。`ListTags` / `ListNotebooks` は現時点でページングなし。`GetNote` の返却型にタグ一覧はなく、`ListNoteTags` が別API。
- `note.Service.List` / `ListPage` / `Search` / `ListBacklinks` はGUI用に保護・ロック状態を注記する。外部向けR0/R1では、保護・ロック・ゴミ箱を除くための共通フィルタと、ページング後の件数・cursor整合性の設計が必要。検索と関連候補の索引不整合判定も外部公開前に確認する。
- 整理は `internal/app/organization_api.go` → `internal/organize.Service` → `note.Service`。`Analyze` は全候補を一度に返し、進捗はWailsイベント。sessionはメモリ内で最大5件・30分、`ApplyCandidates` はsession上の候補IDと対象revisionを検証する。提案表の候補ページング、外部接続のキャンセル・進捗配信は未実装。
- 保存系は `frontend/src/stores/useNoteStore.ts` のdirty draft / ノート単位queueと、Go側のrevision/CAS・journal・同期ゲートを使う。整理GUIは `useOrganizationStore.ts` → `runOrganizationOperation` でflushしてから適用する。外部CLI/MCPがGo Serviceを直接呼べば、このFrontendゲートを迂回するため、Stage Cでは本体側の承認・保存ゲートが必要。
- `SupportWorkspace.vue` は右/下ドックと浮動・リサイズを実装済みだが、PTYや対話ターミナルはない。`AIWorkspace.vue`、`OrganizationCenter.vue`、AI履歴・成果物APIは稼働中であり、Stage Eの代替とデータアクセス検証まで保持する。
- Stage 0のテストでは、Go全パッケージ、Frontend型チェック、34件のFrontendテスト、別の一時出力先へのFrontend buildが成功。既存 `frontend/dist` の画像が使用中で通常のbuildは `EBUSY` となった。GUI実画面とCLI/MCP連携は未確認。

## Stage A 実装契約（2026-09-26）

- AtlasNote本体は、起動中のアクティブ保存空間だけを対象に、`127.0.0.1` のランダムポートへ認証必須の読み取りIPCを公開する。接続descriptorは管理ルートの `.atlasnote-ipc.json` に原子的に作成し、256 bitのセッショントークン、クライアントID、保存空間IDを束縛する。descriptorの直接利用は利用者が明示実行するローカルCLI用の`R0` / `R1`接続とする。MCPはinitialize時にこの接続から別の短命セッションを1回だけ発行し、その時点の保存空間、接続先、公開note／Notebook IDへ固定する。MCPセッションの権限は親接続の権限と公開scopeが要求する権限の共通部分とし、有効期限は発行から30分とする。stdio MCPの正常終了時は即時失効し、期限切れセッションは新規発行時に回収して認証時にも拒否する。アプリ再起動・保存空間切替・接続断後に既存MCPプロセスが新しいdescriptorを読み直すことは禁止し、`MCP_RESTART_REQUIRED`を返してプロセス再起動を要求する。Windowsでは作成前の一時ファイルから現在ユーザー専用の保護DACLを設定する。要求本文は1 MiB、応答は3 MiB、接続とサーバー処理にはタイムアウトを設け、アプリ終了・保存空間全体のロック時にIPCを停止してdescriptorを無効化する。IPCの起動・descriptor公開だけに失敗した場合は外部接続を無効化し、復旧済みのGUI、DB、データロックを停止しない。これは任意シェルや同一ユーザー権限の別プロセスをサンドボックス化するものではない。
- 共通読み取り境界は `internal/readapi` とし、SQLiteやMarkdownを直接開かず、既存 `note.Service` の一覧、取得、検索、バックリンク、関連候補、Notebook、タグAPIを再利用する。本体側で要求ごとにクライアント権限と保存空間scopeを照合し、保護（解除済みを含む）・ロック・ゴミ箱のノートを本文／一覧／抜粋から除外する。MCPの既定公開範囲は0件で、`AtlasNote.exe mcp --note <id>`または`--notebook <id>`を繰り返して利用者が明示したnote、またはそのNotebookに直接所属するnoteだけを公開する。公開範囲外のタイトル、本文、抜粋、バックリンク、関連候補、Notebook名、タグ名は返さない。CLIは従来どおりアクティブ保存空間内の保護されていない対象を利用できる。本文取得の `expectedRevision`、検索索引とリンク索引のrevision・本文hashも返却直前に検証し、外部応答の構築中は既存のmutation gateで本文、タグ、revision、ゴミ箱状態を固定する。
- 公開操作は `notes.list`、`notes.get`、`notes.search`、`notes.backlinks`、`notes.related`、`notebooks.list`、`tags.list` のみ。Stage B以降の解析、候補適用、書き込み、削除、保存空間切替、資格情報、ロック解除は公開しない。
- CLIは既存実行ファイルのサブコマンドとして動作する。例: `AtlasNote.exe notes get <note-id> --expected-revision 3 --json`、`AtlasNote.exe notes search "query" --limit 30 --json`、`AtlasNote.exe notebooks list --json`。`--json`ではstdoutへ共通レスポンスを1件だけ出力し、終了コードは0=成功、2=入力・認証・権限・scope拒否、3=revision競合、4=接続・索引・内部読み取り失敗とする。
- MCPは `AtlasNote.exe mcp` でstdio JSON-RPCサーバーとして起動する。実装済みプロトコル版は`2025-06-18`だけとし、クライアントが別版を要求した場合も未実装版を表明せず、この版を返す。`initialize`、`ping`、`tools/list`、`tools/call`を実装し、同名の7ツールを公開する。ツール結果は互換用のJSON text contentと、共通レスポンスを持つ`structuredContent`を返し、業務エラーは`isError=true`とする。stdoutはMCPフレーム専用で、追加の診断出力を混在させない。
- cursorは操作と正規化済みfilterのfingerprintへ束縛したopaque offsetで、別操作・別条件への再利用を拒否する。既存ページングと同様、ページ間にノートが変更された場合のsnapshot固定は行わない。本文添付の取得、ヘッドレス起動、複数の同時アクティブ保存空間、Stage B以降の操作はStage A対象外。

### Stage A 共通レスポンス例

```json
{
  "apiVersion": "1",
  "requestId": "opaque-request-id",
  "status": "ok",
  "data": {
    "notes": [],
    "nextCursor": "opaque-cursor"
  },
  "error": null
}
```

主な安定エラーは `AUTHENTICATION_FAILED`、`PERMISSION_DENIED`、`SCOPE_MISMATCH`、`RESOURCE_UNAVAILABLE`、`REVISION_MISMATCH`、`INDEX_INCONSISTENT`、`APP_NOT_RUNNING`、`MCP_RESTART_REQUIRED`。保護・ロック・別空間・存在しない対象は本文や存在情報を過度に漏らさないよう、安全なメッセージへ正規化する。

### MCP接続設定例

```json
{
  "mcpServers": {
    "atlasnote": {
      "command": "C:\\path\\to\\AtlasNote.exe",
      "args": ["mcp", "--note", "<公開するnote ID>", "--notebook", "<公開するNotebook ID>"]
    }
  }
}
```

AtlasNote本体を先に起動し、対象の保存空間をアクティブにしてから接続する。`--note`と`--notebook`は繰り返し指定でき、省略時はnote／Notebook／タグのメタデータ一覧を含めて0件公開となる。`--notebook`は指定Notebookに直接所属するnoteだけを含み、子孫Notebookは自動公開しない。公開scopeまたは保存空間を変更した場合はMCPプロセスを再起動する。descriptorやセッショントークンをMCP設定へ転記しない。

## Stage B 実装契約（2026-09-26）

- `organize.analyze` と `organize.get_candidates` をCLI／MCP共通境界へ追加した。CLIは `AtlasNote.exe organize analyze --scope <space|notebook|descendants|note> [--notebook <id>|--note <id>] [--limit 1..100] --json` と `AtlasNote.exe organize candidates <analysisId> [--kind <kind>] [--limit 1..100] [--cursor <cursor>] --json` を使う。MCPも同名の2ツールを公開し、Stage Aと同じ共通レスポンス、requestId、終了コード、stdio `structuredContent` 契約を使う。
- 解析は既存`internal/organize.Service`の一括候補生成、30分・最大5件のメモリsessionを再利用する。外部解析sessionには保存空間、外部クライアントID、MCP公開note／Notebook集合を追加で束縛し、別クライアント、別MCPセッション、期限切れ、偽造IDを`ANALYSIS_UNAVAILABLE`で拒否する。外部sessionは既存GUIのApply経路からも適用できないプレビュー専用とする。
- MCP解析は明示公開note、または明示公開Notebookへ直接所属するnoteだけを候補エンジンへ渡す。Notebook・タグ候補も公開対象から導出できる集合に限定し、未公開note／NotebookのID、タイトル、本文、整理理由、件数を返さない。restricted解析ではリンク先の実在・保護状態を候補から判別できないようbroken-link候補を返さない。通常GUI／unrestricted解析では実在しないIDを従来どおりbroken候補にする。既定公開0件では`P`権限を付与せず、解析できない。`descendants`でも子Notebookは自動公開せず、同じMCP起動時に明示公開された対象だけを解析する。
- `analyze`と`get_candidates`の両方で認証済みIPC、保存空間、クライアント、`P`と`R1`の両権限、公開scope、整理session期限、現在の保護・ロック・ゴミ箱、候補revisionを再検証する。外部読み取りが保持済みのcontent accessは同一Note Serviceのcontextに限ってOrganization Serviceで再利用し、保護状態変更との相互待ちを避けつつ保護・ロック境界を維持する。解析後に対象がscope外へ移動した場合や保護・ロック・ゴミ箱・revisionが変わった場合は、候補や候補件数を返さず解析sessionを利用不可として扱う。
- `analyze`は要約と候補の最初のページ、`get_candidates`は任意kindの候補ページと`expiresAt`を返す。cursorはanalysisIdとkindへ束縛する。候補は最大100件に加え、外部候補1件256 KiB・候補ページ512 KiBで制限し、3 MiB IPC上限とMCPのJSON text／structuredContent二重表現に余裕を持たせる。大きすぎる候補は外部公開集合から除外する。
- 既存解析が5,000ノートで約20.5秒を要する記録に対し、15秒のIPC client／server write timeoutでは必ず切断されるため、同期処理とキャンセル伝播を維持したまま上限だけ60秒へ延長した。読み取り中と候補生成段階間でcontextキャンセルを確認する。独立ジョブ、ストリーミング、詳細進捗プロトコルはStage Bでは追加しない。
- Stage Bは候補生成・取得のみで、候補適用、ノート更新・削除、GUI承認、統合ターミナル、SQLite／Markdown直接アクセス、DB schema変更は行わない。

## 0. 最優先の前提と非目標

1. AtlasNoteはローカルファーストのノートアプリ。既存3ペイン、Markdown正本、SQLiteメタデータ、WebDAV同期、ロック、revision/CAS、操作journal、バックアップ、未保存draft保護を維持する。
2. 最終的には現在のAI・整理専用GUIを廃止する。ただし、**安全な代替機能とデータ移行経路の完成・検証前に削除しない**。既存のノート本文・AI履歴・成果物・資格情報を黙って消去しない。
3. UIは「書く・読む・探す・差分を比較し承認する」、ローカルCLIは決定的な操作、Claude Code/Codex等は必要時の推論に役割を分ける。特定AIの契約・CLIインストール・ログインを必須にしない。
4. MCPはAI用のツール公開プロトコルであり、CLIの対話端末表示や保存排他を実現する技術ではない。統合ターミナルとアプリ本体のIPCは別途必要。
5. 既存Go Serviceの利用を優先し、CLI・MCPそれぞれに独自DB/Markdown書き込みを作らない。
6. 今回はMermaid専用ブランチ、Canvas、グラフ、Bases型ビュー、プロパティ、プラグインAPIの新規実装を対象外とする。既存バックリンク等は維持する。

## 1. ブランチ統合と開始前検証（最初に実施）

対象: `codex/pre-phase5-future-features` ← `codex/support`。前回のGitHub比較時点ではsupportがpre-phase5より2コミット先行、0コミット遅延。ただし**実行時の最新リモートを必ず再取得して確認**する。

1. `git status --short`、現ブランチ、未追跡ファイル、ローカル未pushコミット、remote設定を確認。変更があれば退避方針を報告し、勝手な破棄・`reset --hard`・`clean -fd`・強制pushを行わない。
2. `git fetch origin --prune`。`git log --oneline --decorate --graph`、`git merge-base --is-ancestor origin/codex/pre-phase5-future-features origin/codex/support`、`git rev-list --left-right --count` で関係を再確認。`codex/support`の変更が対象へすでに入っていたら重複マージしない。
3. ローカルのpre-phase5を安全に更新してsupportをマージ。既存のブランチ保護・リモート未反映の変更があれば中止して衝突内容を報告。衝突時は内容を比較して解決し、片側の丸ごと採用をしない。
4. マージ後、`git diff --check`、Goテスト、Frontend型チェック/ビルドと、AI・整理・関連ノート・コンテンツロック・保存・同期の既存回帰を実行。失敗を記録して修正/切り分けるまで次段階へ進まない。
5. 統合コミットを作成した後に、`codex/cli-mcp-terminal-rearchitecture` 作業ブランチを作成する。**プッシュはユーザーが明示した既存運用・権限に従う**。本作業の後続コミットは専用ブランチのみ。
6. マージ時点のSHA、変更されたファイル、テスト結果、未解決課題をレポートする。

## 2. 現行実装に関する調査の起点

以下は前回のGitHub調査から得た候補。正確な関数・制約・既存テストの所在をCodexが統合後の実コードで再確認する。

| 既存モジュール | 既知の機能 | 再利用・改修方針 |
| --- | --- | --- |
| `internal/note` | Note/Notebook、タグ、リンク、検索、関連候補、revision/CAS | 共通の操作基盤として再利用。取得・検索・関連候補は読み取りツール化。 |
| `internal/organize/service.go` | 保存空間/Notebook/ノート範囲の整理候補生成と短期session | CLI・MCPの解析ツールへ橋渡し。大量解析時の範囲・上限・キャンセルを評価。 |
| `internal/organize/apply.go` | 候補IDによる適用、状態・revision・保護確認 | 既存の検証とNote Service経由の更新を再利用。新しい承認/未保存draftゲートを追加。 |
| `internal/app/organization_api.go` | Wails向けAnalyze/Apply API | 参照実装。CLI/MCPからWails公開APIを直接呼ぶのではなく共通アプリケーションサービスへ分離。 |
| `internal/note/related_service.go` | リンク・タグ・語句から説明可能な関連候補 | `notes.related`へ。意味的類似を保証するものではない。 |
| `frontend/src/components/SupportWorkspace.vue` | 右/下ドック・フローティング・リサイズ | 統合ターミナルと共通結果パネルの外枠として再利用可能か確認。 |
| `frontend/src/components/AIWorkspace.vue` / `OrganizationCenter.vue` | 現行の専用GUI | 代替完成後に廃止。構造化表示・適用ロジックを先に抽出。 |
| `frontend/src/components/AIAgentProposalCard.vue` | before/afterの比較表示 | AI専用型を除去した共通差分UIへ利用を検討。 |
| `frontend/src/stores/useOrganizationStore.ts` / `useNoteStore.ts` | 解析session・選択状態・dirty draft・ノート操作lane | GUI固有の操作を外部呼び出しが迂回しないよう、保存ゲートを再設計。 |
| `internal/ai` | Provider adapter、コンテキスト保護、AI履歴 | コマンド・MCPへの依存を作らない。廃止前に保存済みデータの閲覧・移行方針を決める。 |

## 3. 提案アーキテクチャ

```text
既存AtlasNote GUI ─┬─ エディタ・検索・ノート管理 ────────┐
                  ├─ 統合ターミナル（xterm.js候補）      │
                  │    ├─ atlasnote CLI ───────────┐  │
                  │    └─ Claude/Codex等 ─ MCP ───┤  │
                  └─ 共通差分/承認/結果パネル          │  │
                                                  ▼  ▼
                         ローカル認証済みIPC / 実行調停層
                                      │
                         共通Application Service
                           ├─ Note/Tag/Link/Search
                           ├─ Organize/RelatedNotes
                           └─ Approval/Mutation Gate
                                      │
                            既存Service/Repository
                                      │
                            SQLite + Markdown正本
```

- **単一writer:** GUI起動中、AtlasNote本体が既存data lockを保持。CLI/MCP子プロセスは同一保存領域のDB/Markdownへ直接書かず、IPCを介して本体が検証・実行する。
- **起動方式:** 最初はデスクトップ本体の稼働中のみ接続可能とする。ヘッドレス利用やアプリ非起動時のwriter取得は別フェーズ。
- **MCP:** 外部CLIが起動するstdio MCPアダプターが、アプリ本体のローカルIPCへ要求を中継する案。MCPクライアントごとのプロセス起動、接続断、認証、権限を検証。GUIのWails Bridgeそのものは外部プロセス用APIではない。
- **IPC:** OSローカルのみ。Windowsなら名前付きパイプ等を候補とし、他OSへの抽象化を設計。起動中の同一ユーザー/プロセス権限、推測困難なセッショントークン、接続元検証、リクエスト上限、タイムアウト、ログ非露出を実装する。localhostポートの無認証公開はしない。
- **非同期結果:** GUIの承認UIへは、CLI出力文字列のスクレイピングではなく型付きの操作ID・候補ID・実行状態で連携。要求・結果・承認は本体側に一元化。
- **外部CLIの制限:** MCPの読み取り・書き込み権限は**任意シェルのファイルアクセスを制限しない**。統合ターミナルから起動したClaude/Codexが生のファイルパスへアクセスできる点を明示し、MCP内のロック/保護をOS全体のサンドボックスとして説明しない。保護ノートを扱う機能では直接アクセス経路のリスクを検討する。

## 4. 権限モデル（提案）

| レベル | 権限 | 既定 | 例 |
| --- | --- | --- | --- |
| R0 | 読み取り専用・メタデータ | CLIは明示接続した保存空間、MCPは利用者が公開したscopeのみ | ノートID/タイトル一覧、Notebook/タグ一覧 |
| R1 | 本文・検索結果・抜粋の読み取り | CLIはアクティブ保存空間、MCPは利用者が公開したscopeのみ。MCPの既定は0件。保護・ロック・ゴミ箱は除外 | `notes.get`, `notes.search`, `notes.related` |
| P | 変更候補の生成/プレビュー（非書込） | 許可 | `organize.analyze`, `notes.propose_edit` |
| W | 保存を伴う変更要求 | 既定は利用者のGUI承認待ち | ノート作成・更新・タグ変更・移動・ゴミ箱移動、整理候補適用 |
| X | 危険操作 | MVPでは非公開 | 完全削除、保存空間切替、バックアップ復元、WebDAV設定、ロック解除、資格情報取得、任意OSコマンド実行 |

- デフォルト権限は**接続クライアント単位・保存空間単位・要求scope単位**で管理。CLI利用者自身がターミナルから明示実行する操作と、外部AIが自律呼び出しする操作を混同しない。MCPの公開scopeは起動引数からinitialize時の短命セッションへコピーし、接続中に拡張・別空間へ付け替えない。
- ノート保護状態は解析時だけでなく実際の読み取り/適用直前にも再評価する。別保存空間のノート、ロック中ノート、保護ノート、ゴミ箱内本文をMCPから取得させない。失敗は存在情報を過度に漏らさない型付きエラーとする。
- 「AIからの書き込み要求」は提案の作成まで。**同一AIが承認トークンを生成、受領、再利用して自己承認できない**ようにする。承認はAtlasNoteの信頼済みGUI操作と本体側の短命・単回使用トークンで成立。
- 直接実行を許可するローカルCLIの書き込み範囲は、本人による明示コマンド・個別権限設定と安全性の検証後に別途決定。MVPではAI/MCP起点の変更は必ずGUI承認。

## 5. 共通レスポンス形式（CLI JSONとMCP structuredContentの基礎）

```json
{
  "apiVersion": "1",
  "requestId": "opaque-request-id",
  "status": "ok",
  "data": {},
  "error": null
}
```

- `status`: `ok | pending_approval | conflict | rejected | error`。`error`: `{ "code": "STABLE_CODE", "message": "安全な説明", "retryable": false }` または `null`。
- ID・文字列・optional項目・上限・日時表記を型付きスキーマで管理。IDは表示名やOSパスではなく内部の安定IDを使う。
- 大きい結果は`limit`/`cursor`によるページング。全件本文・全件候補の無制限返却は禁止。機密の本文・鍵・生の例外や絶対パスを標準エラー/診断ログへ出さない。
- CLIは`--json`で機械可読な単一JSONをstdoutへ、進捗/人向け案内はstderrへ。exit codeは0=成功、2=入力・権限エラー、3=競合/期限切れ、4=接続・保存失敗など、詳細は実装前に固定。
- MCPはSDK/プロトコルの正式なエラースキーマに従い、上記の業務結果をstructuredContent等で伝える。**独自JSONをMCPプロトコル本体と取り違えない**。

## 6. CLI・MCPに公開する機能一覧（初期契約案）

以下は**新しい公開名と提案スキーマ**であり、現行実装に同名APIが存在するという意味ではない。`scope`は現在の保存空間内に限定する。全ツールで`requestId`、アクセス検証、型付き結果を共通化する。

| MCPツール名 / CLIコマンド例 | 権限 | 入力（主要項目） | 出力（主要項目） | 初期段階 |
| --- | --- | --- | --- | --- |
| `notes.list` / `atlasnote notes list --json` | R0 | `notebookId?`, `tagId?`, `sort?`, `limit`, `cursor?` | `notes[{id,title,revision,updatedAt}]`, `nextCursor?` | A |
| `notes.get` / `atlasnote notes get <id> --json` | R1 | `noteId`, `expectedRevision?` | `note{id,title,content,revision,notebookId,tags}` | A |
| `notes.search` / `atlasnote notes search <query> --json` | R1 | `query`, `notebookId?`, `limit`, `cursor?` | `matches[{noteId,title,snippet,revision}]`, `nextCursor?` | A |
| `notebooks.list` / `atlasnote notebooks list --json` | R0 | `parentId?`, `includeDescendants?`, `limit`, `cursor?` | `notebooks[{id,name,parentId}]`, `nextCursor?` | A |
| `tags.list` / `atlasnote tags list --json` | R0 | `limit`, `cursor?` | `tags[{id,name}]`, `nextCursor?` | A |
| `notes.backlinks` / `atlasnote notes backlinks <id> --json` | R1 | `noteId`, `limit`, `cursor?` | `items[{noteId,title,revision}]`, `nextCursor?` | A |
| `notes.related` / `atlasnote notes related <id> --json` | R1 | `noteId`, `notebookId?`, `descendants?`, `limit` (既存上限20以内) | `items[{noteId,title,revision,snippet,reasons[]}]` | A |
| `organize.analyze` / `atlasnote organize analyze --scope ... --json` | P/R1 | `scope=space|notebook|descendants|note`, `notebookId?`, `noteId?`, `limit?` | `analysisId`, `summary`, `candidates[{id,kind,noteId,reason,before,proposed,applicable,baseRevision}]`, `nextCursor?` | B |
| `organize.get_candidates` / `atlasnote organize candidates <analysisId> --json` | P/R1 | `analysisId`, `kind?`, `limit`, `cursor?` | `candidates[]`, `nextCursor?`, `expiresAt` | B |
| `organize.request_apply` / `atlasnote organize request-apply <analysisId> ...` | W | `analysisId`, `candidateIds[]`, `expectedRevisions?` | `operationId`, `status=pending_approval`, `reviewSummary` | C |
| `notes.propose_edit` / `atlasnote notes propose-edit <id> ...` | P/R1 | `noteId`, `baseRevision`, `before`, `after`, `reason`（大容量時は別途サイズ制限） | `proposalId`, `diffSummary`, `expiresAt` | C |
| `notes.request_create` / `atlasnote notes request-create ...` | W | `title`, `content`, `notebookId?` | `operationId`, `status=pending_approval` | C |
| `notes.request_update` / `atlasnote notes request-update ...` | W | `noteId`, `expectedRevision`, `patch`（フィールドallowlist） | `operationId`, `status=pending_approval` | C |
| `notes.request_move` / `atlasnote notes request-move ...` | W | `noteId`, `expectedRevision`, `targetNotebookId` | `operationId`, `status=pending_approval` | C |
| `notes.request_tags` / `atlasnote notes request-tags ...` | W | `noteId`, `expectedRevision`, `addTagIds[]`, `removeTagIds[]` | `operationId`, `status=pending_approval` | C |
| `notes.request_trash` / `atlasnote notes request-trash ...` | W | `noteId`, `expectedRevision` | `operationId`, `status=pending_approval` | C |
| `operations.get` / `atlasnote operations get <id> --json` | R0（本人の要求のみ） | `operationId` | `state`, `items[{candidateId?,status,message}]`, `updatedAt` | C |

**公開禁止（初期）**: `notes.delete_permanently`、`storage.switch`、`backup.restore`、`sync.configure`、`credentials.*`、`contentlock.unlock`、任意シェル実行。既存のGUIにある操作を機械的にすべてMCPへ公開しない。

### 6.1 既存機能との差分として必ず調査する点

- `notes.list/search/backlinks`が既存に持つページング、ソート、フィルターの範囲を踏襲。新しいcursor方式は必要な場合のみ変換する。
- 既存organize解析は全件候補生成・短命メモリsession（最大5件、30分）である。`get_candidates`のページングや期限、キャンセルは**追加設計が必要**。最初から既存機能であると見なさない。
- `notes.propose_edit`の汎用的な複数箇所・複数ノート編集は現行の単一本文Agent差分を流用して完成扱いにしない。
- `notes.request_tags`等は、既存のtag CASと本文revisionの関係を確認してから契約を確定する。
- 読み取り時の保護・ロック判定、検索索引の正本整合性、添付参照の扱いを既存Service経由で検証する。

## 7. 変更承認と保存の契約

```text
外部AI / CLIが提案 → 本体で受理・検証 → pending_approval
  → GUIで対象・変更前・変更後・理由・影響件数を表示
    ├─ 利用者が明示承認 → dirty draft flush / 対象revision・保護・scopeを再確認
    │                         → 既存Service / CAS / journalで適用
    │                         → 結果・部分成功・失敗・復元導線を返却
    └─ 却下 / 期限切れ → 保存データを変更せず終了
```

- 承認の**前**にも操作要求を検査し、承認の**直前と適用直前**に最新状態を再確認。承認をまたぐ間にノート選択/保存空間/ロック/ノートrevisionが変わった場合はfail-closed。
- 編集中のノートには既存のdirty draft/保存laneと連携。CLI/MCPがServiceを直呼びしてGUIの未保存変更を上書きする経路を許さない。
- 既存整理候補IDはサーバー側sessionに紐付く。クライアントが`before/proposed`を書き換えて送っても適用根拠にしない。
- 複数ノートの更新で部分成功が生じる場合は、成功/失敗の対象と理由を個別返却。自動マージ、自動再試行、一括完全削除は禁止。
- GUI承認待ちの要求を外部AIが直接承認できるAPIは提供しない。
- ロック・保存空間変更・アプリ終了・CLI接続断時はpending要求の無効化/中止条件を明示し、状態不整合を残さない。

## 8. 統合ターミナルの範囲

**最小実装:** 既存3ペインを維持したまま、右ドック/下ドックでターミナルを開閉。必要に応じて既存フローティング枠を流用。フォーカス/ショートカット競合、複数行入出力、ANSI色、Unicode/日本語入力、ウィンドウリサイズ、終了コード、プロセス停止、アプリ終了時の後始末に対応。

- Frontend: xterm.js等の採用を技術検証し、既存Vueレイアウトへ統合。ターミナルからの入力はユーザーの明示入力以外で勝手に送らない。
- Go: OSごとのPTYを抽象化。WindowsはConPTY等の適合性を確認し、Claude Code/Codexをインストール済みの場合に起動できるようにする。CLI不在・認証未完了・終了時は明確に表示する。
- 外部CLIのインストール/契約/認証はAtlasNoteに同梱・代行しない。初期版は対応実績のあるCLIの手動起動とMCP接続ガイドを提供。
- `xterm.js`のみでOSコマンド実行はできない。PTYと子プロセスの安全なライフサイクルを別実装する。
- 任意のOSコマンドを動かせる端末であることによる、直接ファイル操作/秘密情報アクセスとMCP権限モデルの限界をUI・仕様書に記載。

## 9. GUI廃止・移行範囲

1. 移行完了まで`AIWorkspace`、`OrganizationCenter`、既存AI設定/Storeは稼働状態を維持し、既存機能を壊さない。
2. 共通の結果/承認パネルは、ローカル解析候補とMCP/CLI由来の編集提案を同じ型で表示。対象ノート・変更前後・理由・変更単位・対象revision・適用結果を見せる。
3. 既存`AIAgentProposalCard`の差分描画、`SupportWorkspace`の配置/サイズ変更、整理Storeの候補表示などは**責務分離できる部分のみ**再利用。
4. ターミナル/MCP/承認UIと既存機能の同等性を回帰テストし、旧GUIへ入るリンク/ショートカット/設定を新画面へ置換。
5. 既存AI履歴・成果物の閲覧/エクスポート・移行/非移行を明示してから、旧GUIと不要なProvider依存を削除。既存SQLiteテーブル・資格情報を黙って削除しない。移行仕様が未確定なら削除工程を保留。

## 10. 分割実装と完了条件

### Stage 0: 統合・棚卸し
- supportをpre-phase5へ統合し、専用ブランチ作成。既存API/Service/GUI依存図、現行テスト、保存・同期境界、撤去候補を最新コードで確定。
- **完了条件:** cleanな作業開始点、SHA・差分・テスト結果・未確定事項が記録されている。

### Stage A: 読み取りCLI + MCP + 安全なIPC
- `notes.list/get/search/backlinks/related`、`notebooks.list`、`tags.list`を優先。共通Serviceアクセス、接続認証、scope、制限、型付きエラー、JSON契約を用意。
- **完了条件:** GUI動作中にCLIとMCP双方から実データの読み取りが可能。保護・ロック・別空間・未接続・索引不整合の拒否と、既存データ非変更を確認。

### Stage B: 整理の読み取り・候補公開
- organize解析と候補取得をツール化。結果制限・session期限・キャンセル/進捗・大量ノート応答を再評価。
- **完了条件:** 解析は非破壊、候補根拠が再現可能、候補の偽造・古いsessionを拒否。

### Stage C: 共通変更確認・安全な更新
- `request_*`、提案、操作状態、GUI承認UI、dirty draftゲートを実装。CAS、ロック、保存空間、失敗/部分成功を検証。
- **完了条件:** 無承認の書き込み不可、承認中のノート変更で競合を検出、保存失敗時に下書き/候補が残る。

#### Stage C 実装契約（2026-09-27）

- `internal/readapi.Service`をCLI/MCP共通のApplication Service境界として使う。Wは変更要求を`pending_approval`として登録する権限であり、保存・承認権限ではない。`notes.propose_edit`はP/R1を要求し、保存済み本文と`before`、`baseRevision`を一致検証する。変更本文は256 KiB、GUIレビューpayloadは1 MiB、操作は最大64件とする。
- 操作は本体メモリで30分保持し、`operationId`、発行元`clientId`、保存空間、kind、対象ID、expected revision、サーバー側変更内容、生成・期限・状態を保持する。期限後は適用不可で、状態照会用に最大さらに30分保持する。上限到達時は最も古い終了済み操作（applied/rejected/conflict/expired）から回収し、pending操作だけで満杯なら新規要求を拒否する。アプリ終了・保存空間切替ではServiceごと失効する。DB schemaとMarkdown正本は操作登録では変更しない。
- MCP child sessionは従来どおり正常終了・期限で失効し、以後そのsessionから`operations.get`はできない。GUIに届いたpending操作はsession終了だけでは破棄せず、操作自身の期限までレビュー可能にする。これはCLIのone-shot要求とGUI確認の時間差を許すためで、承認時にも保存空間、scope、保護・ロック・ゴミ箱、revision、整理sessionを再検証する。CLIの`clientId`はアプリ稼働中の認証済みIPC descriptorに束縛され、同じdescriptorを用いた後続のone-shot CLIから状態照会できる。MCP childには個別`clientId`を発行する。
- 利用者のGUI操作は`ListExternalChangeReviews`、`ApproveExternalChange`、`RejectExternalChange`のWails bridgeだけに公開する。承認時に本体内で30秒の単回使用permitを生成し、操作ID・保存空間・現在の対象revision/本文hash/タグ状態に束縛して同一呼び出し内で消費する。token生成・受領・適用APIはローカルIPC、CLI、MCPには存在しない。`operations.get`は発行元の状態・項目結果・更新時刻のみ返し、review payloadを返さない。
- GUIは既存SupportWorkspaceに変更確認タブを追加する。対象・変更前後・理由・影響件数・状態を確認後、明示承認する。承認前に`useNoteStore.flushAllDirtyNotes({ mode: 'explicit' })`と対象ノートqueueを通す。flush失敗ではバックエンドを呼ばず、draftとpending操作を保持する。保存失敗でも操作はpendingのまま保持する。revision変更は自動マージ・自動再試行せずconflictとして操作とreview内容を残す。
- `organize.request_apply`はクライアントから候補内容を受け取らず、同じclient/space/scopeの解析sessionに保存されたcandidate IDだけを受ける。旧GUIの`ApplyCandidates`は外部sessionを引き続き拒否する。GUI承認後の専用呼び出しが同じ候補適用エンジンを使い、複数候補の項目別結果を維持する。
- restricted MCPのWは親権限との共通部分に限定する。既存ノートは公開noteまたは公開Notebook内だけ、createとmoveの宛先は明示公開Notebookだけ許可する。note-only scopeからroot作成は不可。非restricted CLIのroot作成は`notebookId`省略時だけ許可する。タグ変更と整理のtag-assignmentは適用直前にも対象tagの公開scopeを確認する。作成・更新・移動・タグ・ゴミ箱移動は既存Note Service、タグCAS、保存journalを通す。外部プロセスからDB/Markdownを直接保存する経路は追加しない。

### Stage D: 統合ターミナル + 外部CLI連携
- 右/下ドック、PTY、CLI起動/終了、MCP接続、表示状態/ショートカットの受け入れ。
- **完了条件:** WindowsのWails実画面で実際の対話CLI、ノート取得、候補生成、承認・適用を一連で操作できる。

### Stage E: 旧GUI整理・既存データ保全
- 旧AI/整理専用GUIを撤去。依存のなくなったStore/API/設定を整理し、現行AI記録へのアクセス方法を維持/移行。
- **完了条件:** 既存3ペイン/同期/保存/バックアップ/保護を維持、旧導線が残らず、記録データが消失しない。

**実装順は依存関係に応じてStage C/Dを調整可能。ただし代替完成前にStage Eを実施しない。**

## 11. テスト・受け入れチェック

- Go: 各Service、IPC認証、権限/scope、MCPツール契約、候補ID偽造、revision競合、ノート操作ゲート、例外・キャンセル・接続断を自動テスト。
- Frontend: ターミナル開閉/配置、承認UI、dirty draft保持、旧操作との回帰、ショートカット衝突、ロック切替を自動テスト。
- Wails: Windows実画面で対話CLIの起動・日本語入力・リサイズ・終了、編集しながらのMCP要求、保護/ロック切替、WebDAV同期との競合を手動受け入れ。
- 全Stage共通: `go test ./...`、既存Frontendテスト、typecheck、build、`git diff --check`。実行不可項目は実行したふりをせず、理由と再現手順を記載。
- 大量データ: 既存5,000ノート解析の約20.5秒は参考値。GUI/IPC/MCP越しの実測は未確認なので別途計測する。
- ドキュメント: ツール一覧・入出力JSON例・権限・MCP接続設定・CLI利用例・既存AIデータ移行・既知の制約を更新。

## 12. Codexの各Stage完了時の報告形式

`実施Stage / 対象ブランチとHEAD SHA / 変更ファイル / 実装内容 / 自動テスト・ビルド結果 / 未実施の手動確認 / 互換性・データ保全上の課題 / 次Stageへの着手可否`

失敗や仕様の不明点があれば、安全側に倒して当該Stageの破壊的変更を止め、具体的な差分・問題箇所・代替案を報告する。
