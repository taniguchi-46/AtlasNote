# Atlas Note 開発者向け初心者ガイド

このドキュメントは、`Atlas Note` のプロジェクトに新しく参加した開発者が、迅速に開発へ合流できるように全体像や開発の手順を解説したガイドです。

---

## 1. プロジェクト概要・詳細

### Atlas Note とは
`Atlas Note` は、AIの活用を前提とした、**ローカルファーストの知識管理・Second Brain（第二の脳）アプリ**です。

### コンセプトと目指すもの
- **使いやすさと自由度の両立**:
  - `UpNote` の洗練されたユーザー体験とUIの使いやすさ。
  - `Joplin` のようなオープンで自由度の高い拡張性。
- **AI 連携**: ユーザー自身が所持している API Key を使用して、知識の整理やライティング支援、要約などをローカル環境主体で実行します。
- **開発者向け機能**: マークダウンを標準とし、コードスニペットの管理などを容易にします。
- **データ所有権**: データはローカルファーストで動作し、WebDAVなどを通じたデバイス間同期を検討しています。

---

## 2. 使用している技術スタック

本プロジェクトで採用されている技術スタックは以下の通りです。
（詳細なバージョン情報は `package.json`、`frontend/package.json`、`go.mod` を正とします）

| カテゴリ | 技術・ライブラリ | 説明 |
| --- | --- | --- |
| **Desktop** | Wails (v2) | Go で書かれたバックエンドと Web 技術のフロントエンドを結ぶデスクトップアプリフレームワーク。 |
| **Backend** | Go | OS機能へのアクセス、ファイル保存、DB操作、AI連携などを担います。 |
| **Frontend** | Vue 3 + TypeScript | UI のコンポーネント構成、状態管理、レンダリングロジック。 |
| **Build Tool** | Vite | フロントエンド用の高速ビルドツール。 |
| **Styling** | UnoCSS | 自由度の高い On-demand CSS ユーティリティエンジン。 |
| **UI Component** | Reka UI | アクセシビリティに配慮したヘッドレスUIライブラリ。 |
| **State Management** | Composables + Pinia | アプリケーションのグローバル状態管理と状態ロジックの再利用。 |
| **Database** | SQLite + Squirrel | ノートのメタデータ、タグ、リンク等の高速なクエリ・検索用。Squirrel はクエリビルダー。 |
| **Editor** | Tiptap | Markdown textarea と Tiptap を併用した Rich / Markdown エディタ。 |
| **Storage** | Markdown ファイル | ノート本文は SQLite に直接入れるのではなく、Markdown ファイル（`note-id.md`）として直接保存します。 |

---

## 3. コードの見方・分業（アーキテクチャ）

### 全体アーキテクチャ
フロントエンドからバックエンドへのデータの流れは以下のように層（レイヤー）で分断されています。

```text
Vue 3 / TypeScript / Vite  (フロントエンド)
  ├─ Components  (表示部品: ノート一覧、エディタなど)
  ├─ Composables (UIロジック、状態・処理の共通化)
  └─ Pinia       (グローバルな画面状態管理: 選択ノート、検索条件)
       │
       ▼ (Wails Bridge による自動生成 TypeScript 呼び出し)
Wails Bridge
       │
       ▼ (Go バックエンド)
Go Backend
  ├─ Application Services  (ユースケースごとの処理、トランザクション、バリデーション)
  ├─ Repository Layer      (DBとファイル永続化の隠蔽層)
  ├─ Squirrel Query Builder (SQLクエリ構築)
  ├─ SQLite                (ノートのメタデータ、タグ、リンクなど)
  └─ Markdown Storage      (ノート本文のファイル保存)
```

### レイヤーごとの責務とコーディングルール
コードを読む・書く際は、以下の分業ルールに従ってください。

1. **表示とロジックの分離**:
   - `Vue Components` (UI) に SQLite の操作や Wails の API 呼び出しを直接散らさない。
   - 状態管理は `Pinia` や `Composables` を経由する。
2. **Wails API の集約**:
   - TypeScript から Go へのアクセスは自動生成された `frontend/wailsjs/go/...` を使用するが、これも Composable や API クライアント層（`useNotes.ts` など）にまとめ、各コンポーネントから直接乱用しない。
3. **データ永続化の分離**:
   - Go 側では、SQLite にクエリを投げる SQL ロジックはすべて `Repository` 層に閉じる。`Services` や API 層に SQL 文字列を漏らさない。
   - ノートの「本文」は Markdown ファイルに保存し、その「管理情報（タイトル、更新日時、パス等）」は SQLite に保存する。この両者の同期整合性はジャーナル処理と補償処理（トランザクション風の仕組み）で行っている。

---

## 4. よく使うコマンドと開発手順

開発に必要なツール（Go、Wails CLI など）は、管理者権限不要で開発できるように、プロジェクト内の `.tools/` 配下に配置されています。

### 開発の準備 (PowerShell)
新しくターミナルを開いたときは、`.tools/` 配下の Go および Wails にパスを通す必要があります。

```powershell
# PATH の追加 (PowerShell)
$env:Path = "C:\Users\mt252\Desktop\MerianProjects\AtlasNote\.tools\go-bin;C:\Users\mt252\Desktop\MerianProjects\AtlasNote\.tools\go\bin;$env:Path"

# パスが通ったことの確認
go version     # go1.26.4 windows/amd64 等が表示されればOK
wails version  # v2.10.1 等が表示されればOK
```

### 依存関係のインストール
プロジェクトの依存関係をインストールします。

```powershell
# フロントエンドの依存関係
npm --prefix frontend install

# バックエンドの依存関係
go mod tidy
```

### 開発サーバーの起動 (ホットリロード)
UI とバックエンドの変更を即時反映しながら動作確認できます。

```powershell
wails dev
```
起動するとデスクトップ版のウィンドウが立ち上がります。ブラウザで確認したい場合は `http://localhost:34115` を開きます。
（※ `wails dev` は実行中ターミナルを占有します。終了するには `Ctrl + C` を入力します）

### 本本ビルド
アプリケーションをビルドして実行可能ファイルを作成します。

```powershell
wails build
```
ビルドに成功すると、`build/bin/AtlasNote.exe` が生成されます。

### テストおよびコード品質チェック
コードを変更した際は、以下の検証コマンドを実行して品質を担保してください。

```powershell
# バックエンドのテスト実行
go test ./...

# フロントエンドの型チェック
npm run frontend:typecheck

# フロントエンドの静的解析(Lint)
npm run frontend:lint

# フロントエンド個別自動テスト (機能ごと)
npm --prefix frontend run test:auto-save            # 自動保存テスト
npm --prefix frontend run test:note-selection       # ノート選択非同期テスト
npm --prefix frontend run test:note-delete          # 削除フローテスト
npm --prefix frontend run test:notebook-hierarchy   # 階層循環テスト
npm --prefix frontend run test:serializer           # シリアライザテスト
```

---

## 5. 関連ドキュメント一覧

さらに詳しく知りたい場合は、以下のドキュメントを参照してください。

- **プロジェクトのルール**
  - [ai.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/rules/ai.md): AI Agent（および人間）の共通開発ガイド。
  - [architecture.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/rules/architecture.md): 本文/SQLite整合性など、より詳細なアーキテクチャ設計。
  - [conventions.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/rules/conventions.md): 命名規則（PascalCase, camelCase等）やTiptapの拡張ルール。
  - [BRANCHING.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/rules/BRANCHING.md): Git のブランチ運用・コミット規約。
- **現在の状況・セットアップ**
  - [setup.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/development/setup.md): 開発環境のより詳細なセットアップ手順や Codex 特有の権限問題の解説。
  - [status.md](file:///c:/Users/mt252/Desktop/MerianProjects/AtlasNote/docs/status.md): 現在の実装済み機能と、次のフェーズでやるべき開発タスクの進捗管理。
