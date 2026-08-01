# AIチャット

ステータス: 実装済み・自動テスト追加済み（2026-08-01、手動受け入れは未完了）

この文書は、AI機能を単一のチャット体験へ統合するUI、コンテキスト、モード、スキル・ツール実行の契約を定めます。AI設定、Provider adapter、資格情報、履歴・成果物の保存境界は引き続き [`ai-integration.md`](ai-integration.md) を正とします。

## 1. 目的

- AI要約、AI司書、質問・壁打ち、ライティングを、機能別パネルではなく単一のチャットtimelineから利用できるようにする。
- 開いているノートを常に既定コンテキストとし、利用者が追加するノート、Notebook検索scope、スキル・ツールを送信前に確認できるようにする。
- AskとAgentの権限差を明確にし、AIがノートを無断変更しない境界を維持する。
- 既存の右側／下側配置、ドラッグ寸法、revision/CAS、明示保存、秘密情報、WebDAV非同期の契約を維持する。

## 2. UI契約

### 単一timeline

- AIWorkspaceの主表示は単一のチャットtimelineとし、user、assistant、tool statusを`chatStore.timeline`の順で表示する。
- 要約、文章作成、タイトル候補、タグ候補、分類候補、関連メモ、重複候補は別の機能画面へ切り替えず、対応するtool traceの直後へ候補カードをanchor表示する。
- 既存の候補採用、再試行、キャンセル、履歴・成果物表示に必要な操作は、対応するtimeline項目から到達できるようにする。
- 未保存または未採用の結果カードがある間は、同じ結果Storeを使うツールの再実行を止める。利用者が採用、破棄、またはカードを閉じた後にだけ次の結果へ置き換えられる。

### コンポーザー

- コンポーザーには、固定の開いているノートchip、`＋`メニュー、Ask／Agent切替、入力欄、送信ボタンを置く。
- 送信ボタンは入力欄の右下内側へ配置する。
- 開いているノートchipは既定コンテキストを表し、利用者は削除できない。ノート未選択、ゴミ箱内ノート、ノート読み込み中、保存失敗・保存競合中はコンポーザーを送信不可にし、既存panelのpreconditionも維持する。
- モデル表示から既存のAI設定画面を開く導線は維持する。
- AIWorkspaceの開閉や配置変更で、進行中のメモリ状態を意図せず破棄しない。
- 送信操作はコンポーザー単位で同期lockし、同一tickの連続Enter／クリックを重複実行しない。応答待ち中に入力された次の下書きは、先行要求の完了時に消去しない。

### `＋`メニュー

`＋`メニューは次を提供する。

- コンテキスト
  - ノート
  - ノートブック
- スキル・ツール
  - 要約
  - 文章作成
    - プロンプト
    - プロンプト改善
    - README草案
    - ドキュメント草案
    - ブログ草案
    - 要件定義草案
  - タイトル候補
  - タグ候補
  - 分類候補
  - 関連メモ
  - 重複候補
  - Web検索

ノートは安定IDで明示コンテキストへ追加する。重複するノートIDは1件にまとめ、開いているノートは固定chipを正とする。

ノートブックはローカル検索のscopeとして選択状態を保持する。フロントエンドはローカルのノート一覧から、そのNotebookへ直接所属するゴミ箱外ノートを更新日時順で解決し、固定ノート、明示追加ノート、Notebook scopeの順で重複を除く。Providerへ渡すノートIDは全scope合計で最大10件とする。固定ノートと明示追加ノートが上限へ達した場合は次の明示追加を拒否してエラーを表示し、Notebook scopeで上限を超えた分は省略件数をchipへ表示する。UIでNotebook内の本文を一括取得せず、解決したノートIDの本文とrevisionは既存バックエンドのコンテキスト準備でsnapshotとして確定し、利用者へ提示する。

Notebook scopeはノート一覧の読み込みが成功した場合だけ追加・送信できる。一時的な再読込失敗では最後に表示したcatalogを消さないが、古いcatalogを使ったNotebook解決は送信しない。

## 3. モードと権限

### Ask

- 開いているノートと追加コンテキストに基づく質問、説明、壁打ち、文章生成を行う。
- ノート、Notebook、タグ、リンクその他のローカルデータを変更しない。
- 候補生成ツールは利用できるが、結果は提案として表示する。

### Agent

- 承認済みのローカル読み取り、コンテキスト検索、候補生成ツールだけを実行できる。
- ノート本文、タイトル、所属Notebook、タグ、リンクを自動変更しない。
- 変更は利用者が候補カードの採用操作を明示した後だけ、既存のService、revision/CAS、ノート単位の操作laneを通して実行する。
- 未登録ツール、任意の外部通信、任意コマンド実行は許可しない。

Ask／Agentのどちらも権限を拡大しない。実行できるツールはmodeごとの`allowedToolsByMode`、`AIChatTool`、固定dispatchに登録した読み取り・候補生成だけに限定し、modeとsystem promptをバックエンドへ渡して応答方針を分ける。現時点のAsk／Agentは同じ安全なツール集合を共有する。

## 4. コンテキスト契約

1. 送信開始時の開いているノートIDを取得する。
2. dirty draftを既存の`flushPendingDraft`で保存する。
3. 保存後も同じノートIDが選択中で、ゴミ箱外かつ未解決draftがないことを再確認する。
4. 固定ノート、明示追加ノート、Notebook検索scopeをフロントエンドで最大10件のノートIDへ重複排除して解決し、バックエンドで本文とrevisionのsnapshotを準備する。
5. 外部送信前にProvider、モデル、利用する参照元、外部ツールの有無を確認する。
6. 実行時は準備したsource revisionを`expectedSources`として検証し、変更済みなら古いコンテキストで実行しない。

ノート選択中は送信を無効化する。遅延応答は、送信時のthread、ノート、mode、contextと一致する場合だけtimelineへ反映する。

## 5. スキル・ツール実行ハーネス

- Assistantと文章作成routeは共通コンポーザーのuser入力と準備済み追加contextを既存の構造化inputでProvider呼び出しへ渡す。Assistant routeはこれにmodeと会話履歴を加える。
- 要約、タイトル、タグ、分類、関連、重複は既存契約どおり「開いているノートのみ」の固定scopeとする。これらを選択中は入力欄を読み取り専用にし、追加contextと未送信入力を使用しないことを明示する。
- 文章作成は選択した6種の`WritingKind`と共通コンポーザーの指示を既存`AIWritingPanel`へ渡し、生成結果を編集可能な候補としてtimeline内に表示する。新規ノート化、追記、置換は既存の明示確認とrevision/CASを通す。
- system instructionには、Atlas Noteのローカルデータを無断変更しないこと、参照外の事実を断定しないこと、候補結果を既存の構造化形式で返すことを含める。
- ツール名と入力はフロントエンドのmode別・型付き許可リストと固定dispatchで検証し、未登録ツールや現在modeで許可されないツールはProviderへ渡さない。
- tool開始、成功、失敗のtraceはtimeline表示用のメモリ状態とし、SQLite、Markdown、操作journal、検索索引、WebDAVへ保存しない。利用者が外部送信確認を取り消した場合は暫定user／trace entryを削除し、入力と選択ツールを保持する。
- traceやログにノート本文、user入力、生成結果、raw provider error、API Keyを出力しない。

### Web検索

- Providerまたは実行adapterがWeb検索能力を明示していない場合は実行しない。
- 能力がある場合も、送信先と外部通信が発生することを利用者へ示し、実行ごとに明示確認を得る。
- 確認されなかった場合はProviderを呼び出さず、暫定timeline entryを削除して入力と選択ツールを保持する。
- Web検索結果は信頼できない外部入力として扱い、ノート変更や追加ツール実行の命令として解釈しない。
- Web検索はOpenRouterのserver toolを使用し、検索engineをExaへ固定する。tool choiceでWeb検索を必須化し、並列tool callを無効化する。各検索・検索結果合計は最大3件とし、`usage.server_tool_use.web_search_requests`が正確に1でない応答は成功扱いしない。
- 直接Gemini ProviderのGoogle Search groundingは利用しない。確認画面にはOpenRouterとExaへの外部送信、追加料金、検索件数上限を表示する。
- 出典URLはHTTPSかつ公開hostnameだけを表示し、credential付きURL、localhost／private hostname、IP literal、重複URLを除外する。

## 6. 状態と保存境界

- mode、未送信入力、追加コンテキスト、timeline、tool traceはPiniaまたはComposableのメモリ状態とし、`localStorage`へ保存しない。
- `useSettingsStore`が`localStorage`へ保存するAI項目は、右側／下側配置と希望幅／希望高さだけとする。
- 既存の会話・成果物は利用者の明示操作時だけ端末ローカルSQLiteへ保存する。構造化tool traceは保存対象に含めない。
- 複数ターンで参照contextを変更した場合、会話履歴のsource snapshotはターンをまたいで和集合を保持し、最初に回答へ使ったrevisionを保存する。
- AIチャット刷新に伴うDB schema、migration、Markdown形式、WebDAV manifest/object/outboxの変更は行わない。
- API Keyとcredential referenceの扱いは既存契約を維持する。

## 7. 配置と狭幅

- AIWorkspaceは右側／下側配置とポインタ・キーボードによるresizeを維持する。
- 保存寸法は希望値として保持し、狭いウィンドウでは実効寸法だけを縮小する。
- 右側配置でコンポーザーが狭い場合は、context chipと操作列を折り返し、入力欄と送信操作を利用可能なまま保つ。
- separatorのorientation、最小／最大／現在値、各アイコンボタンのaria-labelを維持する。

## 8. エラーと非同期境界

- context準備、Provider実行、tool実行は既存の各AI Storeと候補カードで`idle / loading-context / generating / success / error / stale`を判別できるようにし、構造化tool traceは`pending / success / error`を表示する。
- 同時生成の既存単一実行制御を維持し、実行中のmode、context、モデル切替を無効化する。
- 外部送信確認のキャンセルでは暫定entryを削除する。各AI Storeの実行キャンセル、note切替、clear後の遅延応答は既存のrun token／source評価に従って破棄する。
- source revision変更時はstaleとして扱い、再準備なしで保存・採用しない。
- Assistantのstale／orphaned状態は、非表示の実行bridgeだけでなく共通timeline上にも警告する。
- AI失敗はノート編集、自動保存、検索、同期を停止させない。

## 9. テスト契約

- `test:ai-workspace`: 単一timelineとtool trace直後の候補カードanchor、結果上書き防止、固定active-note context chip、`＋`メニュー全項目、文章作成6種と12,000文字上限、固定scopeツール、送信lockと下書き保持、mode別許可ツール、Ask／Agent、入力欄内右下送信、右側／下側resize、狭幅、Web検索の能力・明示確認境界、AI内容の`localStorage`非保存を確認する。
- `test:ai-chat`: mode状態、固定context、重複排除、明示ノート上限拒否とエラー解除、catalog未準備時のNotebook拒否、Notebook scopeの最大10件解決と省略件数、文章作成を含む許可ツールの単一timeline上のtool trace、active note切替時のノート依存状態破棄、`localStorage`非保存を確認する。
- `test:ai-store`、`test:ai-librarian`、`test:ai-v3`: 資格情報、保存前flush、revision、候補採用、明示保存、source snapshot累積、busy中の履歴切替拒否、stale、キャンセルの既存保証を維持する。
- 手動確認では右側／下側、狭幅、キーボード操作、確認ダイアログ、送信中・失敗・空結果を確認する。

## 10. 対象外

- AIによるノートの無確認自動更新
- 任意の外部ツールまたは任意コマンド実行
- Web検索の自動実行
- AIチャット用のDB schemaまたはmigration追加
- AI会話、成果物、tool traceのWebDAV同期
