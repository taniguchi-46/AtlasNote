# 物理保存場所

最終更新: 2026-09-08

## 目的

Atlas Noteの物理的な保存場所を、ノートを分ける論理保存空間とは別に管理する。保存領域（データルート）とバックアップ保存領域（アーカイブルート）を選択でき、初回起動時はデータを作成する前に利用者へ確認する。

## 用語とディレクトリ

- データルート: `storage-spaces.json`、SQLite、Markdown、同期復旧領域、論理保存空間の`spaces/<ID>/`を管理するルート。
- アーカイブルート: `.atlasnote-backups/<spaceID>/`を置くルート。データルートと同じ場合は従来どおりデータルート内へ保存する。
- 論理保存空間: 「設定 > 保存場所」の下部にある保存空間設定で管理するノート単位の分離。物理保存場所を直接表すものではない。

```text
<data-root>/
├─ storage-spaces.json
├─ atlasnote.db                 # 既定の「メイン」空間が使う場合
├─ notes/
├─ .sync-recovery/
└─ spaces/<spaceID>/            # 追加した論理保存空間

<archive-root>/
└─ .atlasnote-backups/<spaceID>/
   └─ generations/

<data-root>/
└─ .atlasnote-restore/<spaceID>/  # アーカイブルートが別の場合
   ├─ staging/
   ├─ rollback/
   └─ pending.json
```

保存場所の設定ファイルは、既定のOSユーザー設定領域にある`AtlasNote/storage-locations.json`へ原子的に保存する。設定ファイルをデータルートの判定対象から分離することで、初回設定後の再起動時に設定ファイルだけを理由として保存場所を拒否しない。既存の保存空間台帳やバックアップ世代へ設定ファイルを混在させない。

## 初回起動

1. `ATLAS_NOTE_DATA_DIR`が設定されていれば、その値を環境上書きとして使用する。環境上書き中は保存場所をUIから変更できない。
2. 保存済みの`storage-locations.json`があれば、そのデータルートとアーカイブルートを検証して使用する。
3. どちらもなければ既定のデータルートを検査する。Windowsでは`FOLDERID_LocalDocuments\AtlasNote`（通常は`C:\Users\<ユーザー>\Documents\AtlasNote`）を既定とし、旧版の`%AppData%\AtlasNote`に既存データがあれば先に引き継ぐ。空、またはまだ存在しない場合は`setup-required`で停止し、SQLite・保存空間台帳・ロックを作成しない。
4. 初回画面ではOSのフォルダ選択ダイアログから保存領域とバックアップ保存領域を選ぶ。空のフォルダ、または既存のAtlas Note保存場所を選べる。無関係な既存ファイル、symlink、書き込み不能な場所は拒否する。
5. 「この設定で開始」で設定を保存し、アプリを再起動してから通常の初期化を行う。

保存済みルートが読めない、書き込めない、無関係なファイルだけがある、またはOSのアクセス制御・同期状態などで利用できない場合は`storage-recovery`で停止する。この画面では元のルートを削除・移動せず、別の空フォルダまたは既存のAtlas Noteルートを選択して設定を保存できる。検証エラーは既存の`STORAGE_LOCATION_*`コードに、安全な日本語理由、検証段階、対象役割、取得できるOSエラー番号、診断IDを添えて表示する。

再起動時の移行が失敗した場合も`storage-recovery`へ戻す。復旧画面では、保留マーカーへ原子的に保存された同じ移行を再試行するか、元の保存場所へ戻すか、別の保存場所へ切り替える。別の空フォルダへ切り替える場合は元データを自動移行せず、元の保存場所を保持したまま新しい保存場所を初期化する。画面上の選択取消はメモリ上の候補だけを取り消し、保留マーカーの取消とは別の操作として扱う。

保留マーカーはv2で、データ／バックアップごとの配置計画（`unchanged`、`open-existing`、`copy-required`）と、`prepared` → `data-placed` → `backup-placed` → `config-committed`の進捗を保存する。v1マーカーは読み取り互換とし、配置済みかどうかを対象フォルダの内容から推測しない。判断できない旧マーカーは移行を実行せず、明示的なフォルダ再選択で新しい`switch`意図へ置き換える。

既存のAtlas Note保存場所を既定ルートで検出した場合は、既存データを移動せずそのまま開く。既定ルート以外の保存場所を指定した場合も、既存データを勝手に削除しない。

## 保存場所の変更

- 実行中のDB、Markdown、ロックを別ルートへホットスワップしない。変更は設定ファイルと保留マーカーへ記録し、再起動時に適用する。
- 移行先が空なら、現在のデータルートを一時ディレクトリへコピーしてから原子的に配置する。移行元は保持する。
- 移行先に有効なAtlas Noteデータがある場合は、それを既存データとして開き、上書き・マージしない。
- バックアップ保存領域を変更する場合は`.atlasnote-backups`だけを移行対象とする。データルートが別のアーカイブルートを指定しているとき、データコピーへバックアップ世代を重複させない。
- 中断した移行は操作ID付きのstage markerと保留マーカーから安全に再試行し、設定のコミットと補助markerの削除が完了するまで保留状態を残す。移行元は常に保持する。
- `copy-required`のstageは、データでは`<target>.atlasnote-migration-<operationID>`、バックアップでは`<target>/.atlasnote-backups.atlasnote-migration-<operationID>`に作成する。stage内の`.atlasnote-migration-stage.json`が操作ID、種別、source、targetの全てと一致する場合だけ、部分コピーを再利用して残りを続行する。別操作のstage、同名の偽装、symlink／reparse point、不正なmarkerや内容は削除・上書きせず失敗として残す。
- UIの移行再試行APIは、上記の所有markerと現在のフェーズを読み取り専用で検証してから再起動を要求する。再試行API自身はコピー、stage cleanup、設定コミットを行わない。バックアップstageがある場合は、sourceの`.atlasnote-backups`が読めることを確認し、sourceがない状態を「バックアップなし」の正常系として扱わない。
- Windowsでは、stage cleanupまたはsource読み取りが共有ハンドルで拒否された場合、owned marker・stage・保留マーカーを保持して失敗する。ハンドル解放後の再試行でstageを再利用して移行を完了する経路を、データstage、分離バックアップstage、同一targetルートのバックアップstageで検証済みである（`internal/config/storage_locations_migration_windows_test.go`）。

保存場所の変更は「設定 > 保存場所」から行う。パス文字列を自由入力するAPIは提供せず、選択した場所の検証結果と安全なエラーだけをUIへ返す。保存場所に関する診断記録はOSユーザーキャッシュ配下の独立領域へ、最大100件・256KiB以内で原子的に保存する。記録にはパス、ファイル名、ユーザー名、ノート本文、環境値、秘密情報、raw errorを含めず、保存できない場合はメモリ上の制限付き記録へ退避する。設定画面と復旧画面では記録を表示し、安全なレポートだけをクリップボードへコピーできる。

フォルダ選択時は、選択したルートの内容・リンク・読み書き・一時ファイルやフォルダのclose／rename／削除に必要なアクセスと、候補ペアの構造的なパス関係を検証する。同一ルートは許可し、相互包含は拒否する。未変更の相手側が利用不能でも有効候補を保持できるため、両保存先が利用不能な復旧状態でもどちらからでも再選択できる。両ルートの完全検証は適用直前と再起動時に行い、適用時の検証拒否では未確定候補だけを戻し、永続保留マーカーは保持する。検証のために選択フォルダ自体を削除しない。

OS原因を確実に判定できる読み書きエラーは、アクセス拒否・共有違反／使用中・空き容量不足を日本語の理由と対処へ反映する。OS定数はプラットフォームごとに分類し、既存コード・段階・OS番号は維持する。未知の原因は一般メッセージへ戻し、特定の同期サービスやセキュリティ製品が原因と断定しない。

診断履歴は書込みロック下で最新のディスク内容を検証し、新しい記録を診断IDで重複排除して記録順に追加した後、100件・256KiB上限を適用して原子的に保存する。他のStoreで期限外となった古いキャッシュは再投入しない。起動後に破損・リンク化した履歴も上書きせず、ロック競合や書込み失敗では従来どおりそのStoreの上限付きメモリ記録へ退避する。

## バックアップ・復元との境界

自動バックアップの保存先はアーカイブルートであり、ノートのSQLite・Markdownだけを世代化する。保存空間台帳、保存場所設定、ロック、Credential Store、同期復旧領域はバックアップ世代に含めない。

アーカイブルートがデータルートと異なる場合、復元のstage、rollback、pending markerはデータルート内の`.atlasnote-restore/<spaceID>/`へ置く。これにより復元の再起動処理が外部アーカイブの管理領域と混ざらない。復元の完全性検証、フェーズ再開、rollbackの契約は[`backup-restore.md`](backup-restore.md)を参照する。

## 対象外

- 論理保存空間ごとの任意外部フォルダ割り当て
- 保存場所の自動削除、上書き、オンライン中のデータ移動
- 任意パス文字列を受け取る公開API

## 実装と確認

- 設定・検査・移行: `internal/config/storage_locations*.go`
- Wails API: `storage_locations_api.go`
- フロントAPI / Store / UI: `frontend/src/api/storageLocations.ts`、`frontend/src/stores/useStorageLocationStore.ts`、`frontend/src/components/StorageLocation*.vue`
- 回帰テスト: `internal/config/storage_locations_test.go`、`internal/config/storage_locations_validation_test.go`、`storage_locations_app_test.go`、`storage_locations_candidate_test.go`、`internal/diagnostics/store_test.go`、`internal/backup/service_test.go`

```text
go test ./internal/config ./internal/diagnostics ./internal/backup . -count=1
npm --prefix frontend run typecheck
npm --prefix frontend run test:storage-location-setup
npm --prefix frontend run test:storage-spaces
npm --prefix frontend run test:backups
npm --prefix frontend run build
```
