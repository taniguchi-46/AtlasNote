# Phase 4 AI統合設計

最終更新: 2026-07-20

ステータス: 設計レビュー前

## 1. 位置付け

本書は、Phase 4「AI」の実装前に決定する認証・プロバイダー・生成結果・同期境界・UI・受け入れ条件を記録する設計書です。要求範囲は [`scope-phese4.md`](scopes/scope-phese4.md)、未完了項目は [`todo-phese4.md`](../todo/todo-phese4.md)、現在状況は [`../status.md`](../status.md) を参照します。

本書の「決定結果」が`承認済み`になるまで、AI API実装、DB migration、WebDAV契約変更は開始しません。決定内容に応じて、`scope-phese4.md`、`todo-phese4.md`、[`../rules/architecture.md`](../rules/architecture.md)、[`environment.md`](environment.md) の記載を更新します。

## 2. 7項目の決定表

| ID | 決定項目 | 決める範囲 | 現状 | 決定結果 | 主な影響先 |
| --- | --- | --- | --- | --- | --- |
| D-01 | v1対象範囲・プロバイダー | 初回対応するAI司書・アシスタント・ライティング機能、対応プロバイダー、モデル選択・能力表示、優先順位 | Gemini、Groq、OpenRouter、OpenAI、Ollama、LM Studioが候補。優先順位は未確定 | 未決定（レビュー待ち） | `scope-phese4.md`、Provider実装、UI |
| D-02 | AI認証・秘密情報 | API Key、アクセストークン、ローカル接続情報の入力検証、OS CredentialStore、セッション限定fallback、更新・削除・再認証、HTTPS・local endpoint・proxy・redirect・timeout・retry・rate limit・費用上限、ログの非露出 | 平文SQLite、Markdown、`localStorage`、設定ファイルへの秘密情報保存は許可しない。具体的な保存方式は未確定 | 未決定（レビュー待ち） | CredentialStore、設定UI、Provider transport、ログ・エラー |
| D-03 | 生成結果の保存・データモデル | 要約、タイトル、タグ、分類、関連候補、Q&A、執筆結果ごとの保存要否、正本、版・モデル・日時、削除・再生成、ユーザー確認、`revision` / CAS / 保存laneとの接続 | Phase 3のAI生成結果は同期対象外。Markdown・SQLite・別成果物へ保存するかは未確定 | 未決定（レビュー待ち） | Note Service、Repository、Markdown、SQLite、UI |
| D-04 | WebDAV同期境界 | AI生成結果を同期対象外のまま維持するか、新しいentity・manifest/object・outbox・conflict・CAS・migrationを追加するか | Phase 3契約ではAI生成結果を同期対象外としている。変更する場合は別設計が必要 | 未決定（レビュー待ち） | `webdav-sync.md`、schema、同期Service、復旧・競合 |
| D-05 | Provider共通契約・実行制御 | Go側interface、型付きrequest/result/error、認証方式、endpoint差分、streaming、cancel、partial response、context長、入力・出力上限、timeout、retry、quota・rate limit・費用上限 | Provider実装、共通エラー、ストリーム形式、test doubleは未作成 | 未決定（レビュー待ち） | Go Application Service、Wails API、Provider adapter、テスト |
| D-06 | UI・データフロー | 未設定、接続確認、生成中、cancel、partial、success、failure、retry、rate limit、offline、送信前確認、機密ノート・長文・大量ノート・空結果の挙動 | AI設定・生成UIは未実装。既存の設定・Pinia・通知・非同期処理の責務境界を維持する | 未決定（レビュー待ち） | Vue Component、Pinia、API client、通知、アクセシビリティ |
| D-07 | テスト・受け入れ条件 | 実キーを使わないprovider test double、契約・UI状態・秘密情報非露出・データ保全・競合テスト、ローカル機能継続、受け入れ記録とCI確認 | AI用テスト、fixture、CIステップは未作成。実キー・実endpointは使用しない | 未決定（レビュー待ち） | Go tests、frontend scripts、CI、受け入れ記録 |

## 3. 維持する既存契約

Phase 4の決定は、次の既存契約を変更しないことを前提とします。

- Markdown本文を正本とし、SQLiteはメタデータと再構築可能な索引に使う。
- AI処理開始時の`revision`をbase revisionとして保持する。
- ストリーミング途中のchunkをノート正本へ逐次保存しない。
- 生成結果を確定するときだけ、既存の`expectedRevision`・CAS・ノート単位保存laneを通す。
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

## 5. 詳細決定記録のテンプレート

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
