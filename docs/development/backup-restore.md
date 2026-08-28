# 自動バックアップ・バックアップ復元

## 目的と範囲

アクティブな保存空間のSQLiteメタデータとMarkdown本文を、アプリ内の管理領域へ自動保存する。保存空間の切替、外部フォルダへのバックアップ、手動インポート、クラウド同期はこの機能の範囲に含めない。

## 保存場所と内容

バックアップは管理ルートから次の固定パスへ保存する。

```text
.atlasnote-backups/<spaceID>/
  generations/<backupID>/
    manifest.json
    atlasnote.db
    notes/...
  staging/...
  rollback/...
  pending.json
```

`backupID` は暗号学的乱数から生成した32桁の小文字hexとする。`manifest.json` には対象保存空間、種別、作成日時、SQLite schema version、ファイル数・合計サイズ、各ファイルのSHA-256を記録する。`notes/` 以下は通常の本文だけでなく、既存の保存復旧に必要な管理ファイルも含めてコピーする。

アクティブなロックファイル、保存空間台帳、同期復旧領域、Credential Store、端末UI設定はバックアップへ含めない。SQLiteはオンラインバックアップAPIで独立したデータベースとして作成し、WAL／SHMサイドカーは残さない。

## 自動バックアップ

- 自動バックアップは既定で有効。設定値は端末のUI設定として`localStorage`にだけ保持する。
- 最後の自動バックアップから24時間以上経過したとき、アプリ起動時または15分間隔のスケジューラで作成する。
- 自動バックアップは最大10世代、復元前の安全用バックアップは最大3世代保持する。
- フロントエンドは同期・AI・インポート・エクスポート・本文ロック・保存空間切替のbusy状態を確認し、dirty draftを既存の保存laneでflushしてから同期を一時停止する。
- Go側ではNote Serviceの同期排他ゲート、コンテンツロックの保存空間スナップショットゲートの順に取得し、SQLiteとMarkdownを同じ境界でコピーする。
- 作成途中の世代は`staging/`に置き、全ファイル、SQLite integrity、schema version、マニフェストを検証してから`generations/`へ原子的に移動する。作成に失敗した場合は既存世代を削除しない。
- 起動復旧がMarkdown欠落を報告している場合、自動バックアップを作成しない。

## 復元

1. 設定画面でバックアップを選び、Go側でマニフェスト、全ファイルのSHA-256、SQLite integrity、schema versionを検証する。
2. プレビュー時にマニフェストハッシュへ束縛した5分間有効の確認トークンを発行する。実行時にもトークン、マニフェスト、ファイルを再検証する。
3. 検証済み世代を`staging/`へコピーし、別SQLiteとして開いてmigration、コンテンツロック復旧、Note Service復旧、integrity、WAL checkpointを実行する。
4. `pending.json`を原子的に保存し、フロントエンドがアプリを再起動する。再起動できなかった場合は、まだ適用していない待機状態を取り消す。
5. 次回起動時、データロック取得後かつSQLite open前に、現行SQLiteとMarkdownを`rollback/`へ退避してステージをインストールする。現行データは復元前の安全用世代として`generations/`にも保存する。
6. `staged`、`current-backed-up`、`installed`のフェーズをマーカーへ記録する。途中終了時は次回起動で同じフェーズを再開し、検証失敗時はrollbackして現行データを保持する。

同期復旧の`pending.json`と同時に適用せず、どちらかが待機中の場合はもう一方を開始しない。復元後はSQLiteを再度検証し、問題がなければマーカーと一時領域を削除する。

## セキュリティと入力境界

- バックアップID、マニフェストの相対パス、サイズ、ハッシュ、schema versionを検証する。
- マニフェストにないファイル、シンボリックリンク、パス traversal、未知のトップレベルファイルは拒否する。
- SQLite検証は読み取り専用・immutable接続で行い、検証のために対象ファイルを新規作成しない。
- Wails APIはファイルパスや内部管理領域をフロントエンドへ返さず、エラーは構造化された安全なメッセージへ変換する。

## 実装とテスト

- Go実装: `internal/backup/`、SQLiteスナップショット: `internal/database/backup.go`
- Wails API: `app.go`
- フロントエンド: `frontend/src/api/backups.ts`、`frontend/src/stores/useBackupStore.ts`、`frontend/src/components/BackupSettingsPanel.vue`
- 自動テスト: `internal/backup/*_test.go`、`internal/database/backup_test.go`、`frontend/scripts/test-backups.mjs`

関連確認コマンドは次のとおり。

```text
go test ./...
npm --prefix frontend run test:backups
wails build -clean
```
