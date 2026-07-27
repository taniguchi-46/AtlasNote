# Phase 4 TODO：v3

決定状態: 正式決定（2026-07-27）

## TODOの目的

v2のAI司書を基盤に、AIアシスタント、AIライティング、利用者が明示的に保存したAI履歴・生成成果物を追加する。Phase 4はこのv3の完了をもって完了とする。

詳細スコープは [`scope-phese4-v3.md`](../development/scopes/scope-phese4-v3.md)、v1は [`todo-phese4.md`](todo-phese4.md)、v2は [`todo-phese4-v2.md`](todo-phese4-v2.md) を正とする。

## v3開始条件

- [ ] v1の完了条件を満たす。
- [ ] v2の完了条件を満たす。
- [ ] AI履歴・生成成果物を保存する対象、正本、削除、再生成、保持期間をレビュー承認する。
- [ ] v3でもAI設定、資格情報、AI履歴、生成成果物をWebDAV同期しないことを承認する。
- [ ] DB schema、migration、rollback、既存データへの影響を実装前に確定する。

## 1. AIアシスタント

- [ ] 現在ノートまたは利用者が選択したノートを対象にQ&Aを実装する。
- [ ] 既存の検索・リンク索引を使ったRAG検索を実装する。
- [ ] 送信対象ノート、検索範囲、本文の送信範囲を送信前に表示する。
- [ ] アイデア壁打ちとブレインストーミングを実装する。
- [ ] 空結果、候補なし、長文、対象ノート切替、revision差異、timeout、cancelをテストする。

## 2. AIライティング

- [ ] プロンプト生成・改善を実装する。
- [ ] README、ドキュメント、ブログ記事、要件定義の草案生成を実装する。
- [ ] 生成文のプレビュー、編集、コピー、明示保存を実装する。
- [ ] 確認なしの本文上書き、既存文書への自動追記、自動公開を行わない。

## 3. AI履歴・生成成果物のデータモデル

- [ ] 会話、要約、タイトル、タグ、分類、関連候補、執筆結果の保存対象を決める。
- [ ] 安定ID、種別、対象ノート／会話ID、入力revision、Provider ID、モデルID、生成日時、更新日時、結果、状態のschemaを決める。
- [ ] API Key、Authorization、raw provider error、不要な本文・プロンプトを保存しない境界を決める。
- [ ] 利用者が明示的に保存した結果だけを永続化する。
- [ ] 個別削除、一括削除、再生成、stale表示、保持期間を実装する。
- [ ] 既存のrevision/CAS・保存lane・Tag／Link Serviceへの明示適用を実装する。
- [ ] schema migration、既存データ保持、rollback、失敗時の不変性を検証する。

## 4. 同期境界

- [ ] AI設定、credential reference、API Key、履歴、生成成果物がsync outbox、manifest、object、conflictを更新しないことを検証する。
- [ ] Phase 3のノート、ノートブック、タグ、ノートタグ同期がAI機能追加後も回帰しないことを検証する。
- [ ] 他端末ではAI履歴・生成成果物を同期せず、再生成・再保存が必要であることをUIと仕様に明記する。

## 5. テスト・CI・受け入れ

- [ ] `test:ai-assistant`相当のmock Wails APIテストを追加する。
- [ ] `test:ai-history`相当の保存、削除、再生成、stale、競合テストを追加する。
- [ ] GoでRepository、Service、Wails API、migration、rollbackをtest doubleと一時DBで検証する。
- [ ] AI失敗後もローカル保存、編集、検索、既存同期が継続することを検証する。
- [ ] 実キーなしでAI設定、Q&A、ライティング、保存、削除、再生成、キーボード操作を手動受け入れする。
- [ ] CIにv3関連テストを追加し、秘密情報・本文・endpoint・raw errorを記録しない。

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
