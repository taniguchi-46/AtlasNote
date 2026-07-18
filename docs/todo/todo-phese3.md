# Phase 3 TODO

## TODOの目的

Phase 3「同期」で、ローカルのMarkdown正本とSQLiteメタデータの整合性を維持したWebDAV同期を設計・実装する。

機能要件は `../development/scopes/scope.md`、ローカル保存と競合検出の既存契約は `../development/note-concurrency.md`、Phase 3の同期設計は `../development/webdav-sync.md` を正とする。

このファイルはPhase 3の進捗チェックリストです。同期契約の変更は [`webdav-sync.md`](../development/webdav-sync.md)、実装順序の変更は [`implementation-plan.md`](../development/implementation-plan.md) に反映します。

## 現状・前提

- Phase 2の関連メモはPhase 4へ完全移管し、同期機能の対象には含めない。
- Markdown本文を正本とし、SQLiteの検索・リンク・タグなどの派生データは再構築可能に維持する。
- ローカルの`revision`は端末内CAS用であり、端末間の新旧比較には使用しない。
- ローカル保存キューと同期用のdurable outboxは分離する。
- WebDAVの認証、strong ETag・last-synced base、manifest方式のdurable outbox、端末間競合解決は設計レビューで確定済みである。

## 現在の設計状態

- [x] `docs/development/webdav-sync.md` に同期対象、リモート配置、状態遷移、競合、認証、migration、テストの設計を追加する。
- [x] `docs/development/implementation-plan.md`、`docs/status.md`、`docs/rules/architecture.md`、`docs/development/environment.md` から確定設計を参照する。
- [x] 設計レビューで未確定事項を決定し、設計上の実装開始条件を満たす。

## Phase 3開始条件（完了記録）

- 設計レビューは完了している。
- WebDAV通信、同期用migration、認証情報の保存処理のコア実装と、非本番の実WebDAVサーバー相互運用・手動UI受け入れは完了している。今後も同じ環境で回帰確認を継続する。
- Phase 2のCI受け入れ条件と残課題の扱いを確認し、Phase 3の実装開始条件を満たしたことを記録している。

## 実行手順

### 1. 同期設計を確定する

- [x] 同期対象、change set、`head`/manifest/objectのリモート配置、ノートの作成・更新・削除の表現を決定する。
- [x] CredentialStoreの保存先、同期設定タブ、入力検証、ログ・エラー表示での秘密情報非露出を決定する。
- [x] strong ETag、同期元hash、last-synced baseを含む同期状態のデータモデルを決定する。
- [x] durable outboxのスキーマ、manifest commit、送信順序、クラッシュ後の復旧、最大3回retry方針を決定する。
- [x] 手動同期、自動同期（5秒debounce・選択間隔poll）、オフライン、timeout、部分成功時の状態遷移を決定する。
- [x] 端末間の更新・削除・同時更新に対する競合検出と利用者向け解決操作を決定する。
- [x] 既存のMarkdown操作journal、revision/CAS、ローカル保存laneとの責務境界を決定する。

### 2. 実装前の受け入れ条件を確認する（完了）

- [x] `../development/webdav-sync.md` をレビューし、設計、DB変更、migration、rollback方法、既存データへの影響を確定する。
- [x] Repository / Service / Wails API / フロントAPI / Store / UIの変更範囲を整理する。
- [x] WebDAVサーバー障害、認証失敗、secure store unavailable、競合、復旧、秘密情報非露出のテストケースを整理する。
- [x] 同期開始前に、Phase 2の対象テストを含むCI受け入れ条件を確認する（ローカルでCI相当の全テスト・Wails buildを実行済み）。

### 3. 設計承認後に実装・検証する

- [x] 設計レビュー内容を、`docs/development/implementation-plan.md` のPhase 3順序へ反映する。
- [x] 実装後、正常系・異常系・競合・データ保全を自動テストし、非本番の実WebDAVサーバーで受け入れ確認する。実環境の回帰確認は継続する。
- [x] schema version 10へ同期間隔、フェイルセーフ、custom TLS、TLS error ignore、proxy設定を追加し、version 9 backfillと制約をテストする。
- [x] Joplinと同じ設定項目・draft方式、単一WebDAV URL、読み取り専用設定確認、Apply/OK/戻るを実装する。
- [x] target変更時の資格情報再利用を禁止し、HTTPS既定・明示的HTTP許可、custom CA、TLS error ignore、HTTP/HTTPS proxyを実装・テストする。
- [x] 空同期先フェイルセーフを既定ONで実装し、初回upload、空local、tombstoneの境界をテストする。
- [x] 条件付きlocal再アップロードと、別領域stage・起動時backup/swap/rollbackによるremote再ダウンロードを実装・テストする。
- [x] 手動同期、自動同期、同期競合解決のUI状態を確認する。
- [x] 非本番の実WebDAVでPhase 3受け入れを実施し、動作OKを記録する（2026-07-19。接続先・資格情報は記録しない）。
- [x] 関連ドキュメント、TODO、CIを実装状態へ更新する。

## Phase 3受け入れ記録

- 実施日: 2026-07-19
- 環境: 非本番の実WebDAV（接続先、ユーザー名、パスワード、Authorizationヘッダーは記録しない）
- 結果: Phase 3の受け入れ動作はOK。Phase 3受け入れを完了とする。
- CI: `dev-phese3` のHEADに対する [GitHub Actions CI run #29658225886](https://github.com/taniguchi-46/AtlasNote/actions/runs/29658225886) が成功している。
- 継続確認: WebDAVサーバーまたは同期実装を更新したときは、同じ非本番環境で回帰確認を行う。この継続確認は受け入れ完了後の運用課題であり、Phase 3未完了を意味しない。

## 受け入れ後の継続確認

- [ ] WebDAVサーバーまたは同期実装を更新したとき、同じ非本番の実WebDAV環境で回帰確認を行い、結果をこの記録へ追記する。

## 注意事項

- WebDAV通信や認証を先に実装して仕様を固定しない。
- 同期失敗がMarkdown正本の保存、ローカルdraft、既存の操作journalを壊さないようにする。
- APIキー、パスワード、ノート本文、認証ヘッダーをログや利用者向けエラーへ出さない。

## 絶対遵守事項

- 既存のRepository / Service / Wails API / Piniaの責務境界を維持する。
- ユーザー入力をファイルパス、SQL、ログへ直接使用しない。
- migrationは確定設計とテスト方針を確認したschema version 8〜10として追加済み。既存DBへdown migrationは行わない。
