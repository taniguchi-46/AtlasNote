# アーキテクチャ

`Atlas Note` の設計情報をまとめます。

## 全体構成

Atlas Note は Wails を使うデスクトップアプリです。UI は Vue 3、アプリケーションロジックと OS / DB / ファイルアクセスは Go 側に寄せ、データは SQLite と Markdown ファイルを組み合わせて扱う方針です。

```text
Vue 3 / TypeScript / Vite
  ├─ Components
  ├─ Composables
  └─ Pinia
       │
       ▼
Wails Bridge
       │
       ▼
Go Backend
  ├─ Application Services
  ├─ Repository Layer
  ├─ Squirrel Query Builder
  ├─ SQLite
  └─ Markdown Storage
```

## 採用技術

| 項目 | 内容 |
| --- | --- |
| フレームワーク | Wails + Vue 3 + Vite |
| 言語 | Go + TypeScript |
| スタイル | UnoCSS + Reka UI |
| 実行環境 | Wails デスクトップアプリ、開発時は Go / Node.js / Vite |
| 配信 / デプロイ | デスクトップアプリとして配布予定。詳細は未確定 |

## 主要モジュール

| モジュール | 役割 |
| --- | --- |
| Vue Components | ノート一覧、エディタ、設定画面などの表示部品 |
| Composables | UI ロジック、Wails API 呼び出し、入力状態の再利用可能な処理 |
| Pinia | ノート選択、検索条件、同期状態などのフロントエンド状態管理 |
| Wails Bridge | TypeScript から Go のアプリケーションサービスを呼び出す境界 |
| Go Application Services | ユースケース単位の処理、トランザクション、入力検証 |
| Repository Layer | SQLite と Markdown Storage への永続化を隠蔽する層 |
| SQLite | ノートのメタデータ、タグ、リンク、検索用インデックスなど |
| Markdown Storage | ノート本文の永続化 |
| WebDAV Sync | 将来の同期処理。競合解決方針は未確定 |
| AI Integration | ユーザー自身の API Key を使う知識整理、要約、ライティング支援 |

## データ / 状態管理

- ノート本文は Markdown ファイルとして保存する方針。
- ノートのメタデータ、タグ、リンク、同期状態、検索補助情報は SQLite に保存する方針。
- SQL 組み立てには Squirrel を使い、直接 SQL 文字列を散らさない。
- フロントエンドの画面状態は Composables と Pinia で管理する。
- Wails API は画面から直接乱用せず、Composables または API クライアント層に寄せる。

## 外部連携

| 連携 | 方針 |
| --- | --- |
| WebDAV | ローカルデータの同期に使う予定。認証方式と競合解決は未確定 |
| AI API | ユーザー自身の API Key を利用する。保存方式、対応プロバイダ、モデル選択は未確定 |
| OS Keychain | API Key 保存先の候補。採用可否は未確定 |

## 未確定事項

- 実際のディレクトリ構成。
- Markdown と SQLite の同期整合性を保つ方法。
- 検索方式。SQLite FTS、外部インデックス、または別方式のどれを採用するか。
- AI 機能の呼び出し境界を Go 側に集約するか、フロントエンド側の設定 UI とどう分けるか。
