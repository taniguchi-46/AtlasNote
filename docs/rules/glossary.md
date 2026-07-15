# 用語集

| 用語 | 意味 |
| --- | --- |
| Atlas Note | AI を前提としたローカルファーストの知識管理 / Second Brain アプリ |
| ローカルファースト | ネットワーク接続がなくても主要機能を使えるよう、ローカル保存を中心にする設計方針 |
| Second Brain | 個人の知識、メモ、リンク、発想を蓄積・整理・再利用するための仕組み |
| Wails | Go バックエンドと Web フロントエンドでデスクトップアプリを作るフレームワーク |
| Reka UI | Vue 向けのアクセシブルな UI プリミティブ |
| UnoCSS | ユーティリティファースト CSS エンジン |
| Tiptap | ProseMirror ベースのリッチテキストエディタ |
| Markdown textarea | Markdown原文を直接編集する入力欄 |
| Squirrel | Go 向け SQL クエリビルダー |
| WebDAV | Phase 3の同期方式。設計確定・実装前 |
| revision | SQLiteに保存するノート単位の同一端末内CASトークン。端末間の新旧比較には使わない |
| expectedRevision | 更新・完全削除時に要求側が指定するローカルrevision。現在値と一致した場合だけ適用する |
| CAS | Compare-And-Swap。期待値が現在値と一致する場合だけ更新する方式 |
| draftVersion | フロントエンドの入力snapshot世代。永続revisionとは別のメモリ上の値 |
| change set | 1回のローカル操作で変更された全entityをまとめた同期単位 |
| head | リモートで現在のmanifest hashと世代を指す唯一の可変リソース |
| manifest | entity keyとobject hashの一覧を持つ不変JSON |
| object | entityのactive payloadまたはdeleted tombstoneを持つ不変JSON |
| tombstone | 完全削除を表すdeleted object。trash状態とは区別する |
| strong ETag | リモートheadの世代を検証し、`If-Match`条件付き更新に使う強いETag |
| last-synced base | 端末間の3-way比較に使う、最後に同期済みのmanifest/object情報 |
| durable outbox | クラッシュ後も再開できるようSQLiteへ永続化する未送信change setのキュー |
| CredentialStore | WebDAV資格情報をOSのsecure storeへ保存・取得する境界。利用できない場合はセッション限定とする |
