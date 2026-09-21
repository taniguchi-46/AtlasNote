# 開発環境方針

Atlas Note の開発環境方針をまとめる。

具体的なセットアップ手順、確認済みバージョン、実行コマンドは [セットアップ](setup.md) を、採用技術は [技術スタック](tech-stack.md) を参照してください。

## 基本方針

- メインの開発環境はローカル環境とする。
- Docker は開発体験の主軸ではなく、検証・CI・ビルド補助に使う。
- Wasm は初期段階では採用しない。必要な処理が明確になった時点で再検討する。
- `.env`、API キー、トークン、パスワードなどの秘密情報はリポジトリに含めない。

## ローカル開発環境

ローカル環境では、Wails アプリをそのまま起動して開発する。

主な理由は、Wails が OS の WebView、ファイルシステム、将来的な Keychain 連携などのネイティブ機能に依存するため。Docker 内だけで GUI アプリの起動確認まで完結させると、環境構築が複雑になりやすい。

依存関係、バージョン、実行コマンドの正本は `package.json`、`frontend/package.json`、`go.mod`、[セットアップ](setup.md) とする。CIの確認項目は [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) を参照する。

## Docker の扱い

Docker は採用候補とする。ただし、初期段階では Wails GUI 開発の主環境にはしない。

主な用途:

- CI と同じ条件でのビルド確認
- Go / Node.js の依存関係確認
- `npm run build` や `go test ./...` の再現性確認
- 将来的なリリースビルド補助

初期段階で避けること:

- Docker 内で Wails GUI 開発を完結させること
- OS 固有の WebView / Keychain / ファイルアクセスまで Docker 前提で設計すること
- まだ未確定の構成に対して複雑な Dockerfile を先に作ること

必要になった場合の最小構成案:

- `Dockerfile.dev`
  - Go と Node.js の検証用
  - フロントエンドビルドと Go テストを実行できる構成
- `docker-compose.yml`
  - 複数サービスが必要になった場合のみ追加
  - SQLite 単体利用の間は必須ではない

## Wasm の扱い

Wasm は初期段階では採用しない。

現時点の構成では、Go 側のアプリケーションサービスと Vue 側の UI で主要機能を実装できる見込みがある。Wasm を早期に導入すると、ビルド、デバッグ、型連携、配布の複雑さが増える。

再検討する条件:

- Markdown 解析、検索、差分計算などで重い処理が発生した場合
- 外部ライブラリを安全に隔離して実行したい場合
- 将来的にブラウザ版と処理を共有したい場合
- Go / TypeScript の通常実装では性能要件を満たせない場合

再検討時の確認項目:

- Wails との連携方法
- ビルド成果物の配置方法
- TypeScript からの呼び出し方法
- テスト方法
- パフォーマンス測定方法
- 配布サイズへの影響

## バージョン管理方針

開発環境の差異を減らすため、以下を明示的に管理する。

- Go バージョン
- Node.js バージョン
- Wails CLI バージョン
- パッケージマネージャー
- 主要依存パッケージ
- 確認コマンド

管理方法:

- README または `docs/development/environment.md` に明記する
- `.node-version` または `.nvmrc` を置く
- Go は `go.mod` の `go` ディレクティブで管理する
- パッケージマネージャーは `package.json` の `packageManager` で固定する
- Docker は検証用の固定環境として使う

## 秘密情報の扱い

以下はリポジトリに含めない。

- `.env`
- API キー
- パスワード
- トークン
- WebDAV 認証情報
- AI API 認証情報

`.env.example` の `WEBDAV_ENDPOINT`、`WEBDAV_USERNAME`、`WEBDAV_PASSWORD` は設定名の候補を示すだけで、現在の実行時設定としては読み込まれていない。現時点の設定コードが環境変数から読むのは `ATLAS_NOTE_DATA_DIR` である。この値は個別の保存空間ではなくAtlas Noteの管理ルートを指定し、既存ルートを「メイン」、追加空間を管理ルート内の`spaces/<内部ID>/`として扱う。`ATLAS_NOTE_DATA_DIR` が設定されている場合は物理保存場所のUI変更を無効にする。未設定時の保存領域・バックアップ保存領域はOSユーザー設定領域の`AtlasNote/storage-locations.json`で管理し、Windowsの既定データルートはLocal Documents配下である。詳細は `docs/development/storage-locations.md` を参照する。Phase 3の同期契約は `docs/development/webdav-sync.md` の確定設計を正とし、実装ではこれらの値を平文設定へ永続保存せず、CredentialStoreへ分離する。Phase 4のAI APIキー、プロバイダー、モデルも`.env`や環境変数では設定せず、アプリ設定とAI用OS CredentialStoreで管理する。

AI設定はアプリの設定画面で管理する。AI API KeyはWebDAVとは分離したAI用OS CredentialStoreへ保存し、利用できない場合だけsession-onlyとする。実キーを`.env`、環境変数、SQLite、Markdown、`localStorage`へ保存しない。

## 今後決めること

- Dockerfile を作るタイミング
- Wasm を再検討する条件の詳細
- デスクトップアプリの配布対象OSとビルド手順
