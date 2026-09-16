# Atlas Note 開発ワークフロー補足

共通スキルで計画・実装・レビュー・リファクタリングを行う際、対象に関係する契約を確認する。仕様の正本は `docs/rules/ai.md` が案内する各資料とし、この補足だけで現行仕様を断定しない。

## Atlas Noteの不変条件を確認する

対象変更に関係する行だけを確認する。

| 対象 | 確認する契約 |
| --- | --- |
| 保存・復旧 | Markdown本文を正本とし、操作ID付き一時ファイル作成 → SQLiteメタデータとoperation journalの同一transaction確定 → Markdown正本への置換 → journal削除の順序を守る。Markdown確定失敗時はSQLiteを戻し、補償失敗時は一時ファイルとjournalを復旧用に残す |
| revision・競合 | `expectedRevision` / CAS、stale要求の完全な無変更、一要求一revision、競合draft保持、自動merge・自動rebase・強制上書き禁止を守る |
| 非同期・状態 | ノート単位queue、単一in-flight、別lane分離、latest-wins、失敗lane停止、`flush` / `flushAll`、終了前flush、古い応答による選択逆転防止を守る |
| 不整合・復旧 | SQLiteレコードや孤児Markdownを自動削除せず、未完了操作と復旧情報を保持し、Wails API公開前に復旧する |
| DB・検索・タグ | SQL parameter binding、transaction、外部キー、UNIQUE、CASCADEを守る。FTS5を再構築可能な派生データとして扱い、索引失敗でMarkdown保存を戻さない。タグ操作でMarkdown、revision、`updated_at`、FTS5、保存journalを変更しない |
| セキュリティ・ログ | path traversalと危険なURLを防ぐ。raw HTMLはMarkdown正本に保持したまま、Rich変換時に実行可能なDOM、イベント属性、危険なURLとして解釈しない。本文、タイトル、検索語、ファイルパス、秘密情報、内部stackをログや利用者向けエラーへ出さない |
| API・責務 | 対象仕様で構造化結果と定めた競合などの既知エラーを文字列解析へ依存させず処理し、DB・I/O・内部障害は既存のGo error / Promise rejection契約を維持する。Component → Store / Composable / API client → Wails → Service → Repository / Storageの境界を守る |


## 技術前提と検証

- ローカルファーストを維持し、Markdown本文を正本、SQLiteをメタデータ・関連・再構築可能な索引として扱う。
- リファクタリングでも保存・補償・復旧、revision、非同期処理の契約を維持する。DB変更時はschema version、rollback、失敗時の不変性も確認する。
- `README.md`、`docs/README.md`、`docs/status.md`、`docs/rules/ai.md`、`docs/rules/architecture.md`、`docs/rules/conventions.md` と対象scope・TODO・仕様を参照する。
- 実際の `package.json`、`frontend/package.json`、`go.mod`、対象テストから検証コマンドを選ぶ。変更したGoファイルの `gofmt`、対象Go packageのテスト、`go test ./...`、関連する `npm --prefix frontend run test:<name>`、`npm run frontend:typecheck`、`npm run frontend:lint`、`npm run frontend:build` をリスクに応じて使う。
- Wails連携・bindingsへ影響する場合だけ `wails build` を選ぶ。`frontend/wailsjs/`、`frontend/dist/` など生成元のある成果物を手編集しない。
- 調査・計画だけではテストやbuildを実行しない。レビューだけではformatter、自動修正、生成物を更新するbuildを実行しない。
