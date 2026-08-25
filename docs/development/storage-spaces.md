# ノート保存空間

最終更新: 2026-08-25

## 目的

ノート、同期設定、AIのローカル管理データを保存空間ごとに分離し、後続のユーザー向けロック、インポート、エクスポートの安全な保存境界を提供する。

## ディレクトリ構成

`ATLAS_NOTE_DATA_DIR` は個別の保存空間ではなく、Atlas Noteが管理する保存ルートを示す。

```text
ATLAS_NOTE_DATA_DIR/
├─ storage-spaces.json
├─ storage-spaces.lock
├─ atlasnote.db
├─ atlasnote.lock
├─ notes/
├─ .sync-recovery/
└─ spaces/
   └─ <32桁の小文字hex ID>/
      ├─ atlasnote.db
      ├─ atlasnote.lock
      ├─ notes/
      └─ .sync-recovery/
```

- 既存のルート直下データは、初回起動時に既定の「メイン」として登録し、ファイルやディレクトリを移動しない。
- 新しい保存空間だけを `spaces/<ID>/` に作成する。IDは暗号学的乱数による128 bit値で、表示名をパスに使用しない。
- `storage-spaces.json` はversion、現在の保存空間ID、ID・表示名・作成日時だけを保持する。利用者指定パスは保持しない。

## 分離境界

保存空間ごとに次を独立させる。

- SQLite全体。ノート、Notebook、タグ、リンク、検索索引、操作journal、WebDAV接続・outbox・競合、AI設定・履歴・成果物を含む。
- Markdown正本と回復用ファイル。
- `atlasnote.lock` によるOSレベルの単一writer保証。
- WebDAV再ダウンロード用の `.sync-recovery/`。

WebDAVパスワードとAI API Keyは従来どおりOS CredentialStoreへ保存する。保存空間ごとのSQLiteが個別のcredential referenceを保持し、秘密情報を管理台帳、Markdown、`localStorage`へ保存しない。

## 作成と切替

- 保存空間の一覧・作成・選択は「設定 > 保存空間」だけから行う。トップバーには選択UIを置かない。
- 新規作成時は内部IDのディレクトリを作り、対象ロック取得、SQLite open・migration、Markdownディレクトリ作成まで成功してから管理台帳へ追加する。
- 切替時は、同期処理とAI処理が停止中であることを確認し、自動同期を一時停止して全dirty draftを保存する。保存失敗時は現在の空間と下書きを維持する。
- 対象空間のロック取得、SQLite open・migration、Markdownディレクトリ検証に成功した場合だけ選択を管理台帳へ保存する。
- 実行中のServiceやRepositoryは差し替えない。製品版では、選択保存後にAtlas Noteを終了し、DB・lockを解放してから同じ実行ファイルを自動起動し、選択先を開く。
- `wails dev` では、生成された子バイナリだけを再起動すると開発サーバーを失うため、自動再起動を行わない。選択は保存済みなので、画面の案内に従って現在の開発プロセスを終了し、`wails dev` を再実行する。

## 安全性

- 管理台帳は一時ファイルへの書込み、file sync、renameで原子的に更新し、読み書きの短時間だけ `storage-spaces.lock` を取得する。
- 台帳が不正、巨大、未知version、重複ID、重複名、不正IDの場合は起動を安全側で停止し、台帳を自動修復・上書きしない。
- 新規空間のパスは管理ルート配下から内部導出し、`..`、絶対パス、symlinkを経由する保存先を受け付けない。
- 同じ保存空間の2つ目のwriterは拒否する。別の保存空間は独立したlockを使う。
- 現スコープでは削除、改名、外部フォルダ選択、暗号化を提供しない。

## 互換性

- DBスキーマの追加migrationはない。保存空間ごとに既存schemaをそのまま使用する。
- 旧バージョンは従来どおりルート直下の「メイン」を開ける。`spaces/` 内の保存空間は削除されず、旧バージョンから見えない。
- 管理台帳を手動削除・編集すると選択情報を失うため、復旧操作として扱わない。

## 確認コマンド

```bash
go test ./internal/config ./internal/notespace -count=1
go test ./... -count=1
npm --prefix frontend run test:storage-spaces
npm --prefix frontend run test:sync
npm run frontend:typecheck
wails build -clean
```
