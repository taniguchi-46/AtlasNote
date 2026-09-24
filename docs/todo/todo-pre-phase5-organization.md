# 整理センター TODO

## 実装

- [x] AppTopBarから開けるタスク概要／候補レビューの整理センターを追加する。
- [x] フローティング／右ドック、移動・リサイズ、最小化・復帰を追加する。
- [x] アプリ内メモリにレビュー状態を保持し、閉じる／再表示で候補・適用結果を維持する。
- [x] 保存空間・Notebook・各ノートのscopeごとに解析session、選択、結果を保持し、画面／scope移動で別sessionを置き換えない。
- [x] 古い解析応答はそのsessionだけを破棄し、ロックでは全sessionを無効化して遅延応答による復元を防ぐ。
- [x] 全保存空間／Notebook直下／Notebook子孫／対象ノートと直接の関連先を読み取り専用解析する。
- [x] title、Notebook分類・移動、未分類ノート検出、タグ付与、重複・空ノートのゴミ箱移動、相互リンク提案と情報候補を追加する。
- [x] 適用時に保存空間・保護状態・revision・タグ・関連対象を再検証し、同一ノートの候補を1バッチへまとめて既存Service/CASとノート操作queueを使う。
- [x] タグ付与成功時は表示中ノートが対象と一致し、応答時も同じ場合だけ表示タグを更新する。
- [x] 保存失敗／未実行候補は再試行可能にし、競合／stale候補は再解析を案内する。
- [x] AIWorkspace／バックリンク画面で共通の小型パネルを使い、閉じる操作では処理・選択・解析対象を破棄しない。
- [x] AIWorkspaceとバックリンクを独立して通常／フローティング切替、移動、リサイズ、最小化、復帰、閉じる操作に対応する。
- [x] 小型パネルに対象・変更前・変更案・理由を表示し、確認後に単件／一括承認できる。
- [ ] 大量データの解析コストを実データ相当で計測し、必要な場合だけ上限・段階読込を調整する。
- [x] AIWorkspaceとバックリンク画面へ共通小型パネルを展開する。

## 自動検証

- [x] 整理Storeの閉じる・最小化・復帰、逐次適用、失敗保持を確認する。
- [x] 空間scopeを保持したままノートA/B・sourceを移動し、選択／結果が戻ること、別scopeの逆順応答とロック後応答を確認する。
- [x] 解析失敗で直前の候補を保持することを確認する。
- [x] Go service testsで候補生成、Markdown保存、tag CAS、stale状態を確認する。
- [x] `go test ./... -count=1` を隔離APPDATA/GOCACHEで実行し、全Go package成功。
- [x] `test:organization`、`test:note-delete`、`test:auto-save`、`test:note-operation-queue`、`test:tags`、`test:note-links`、`test:notebook-hierarchy`、Frontend typecheck／lint／production build成功。
- [x] Wails v2.10.1 Windows/amd64 build成功。
- [x] `git diff --check` 成功。

## 手動受け入れ

共通サポートパネルへの統合で、上記のAI内／バックリンク内小型整理パネルは対象ノートscopeを開く導線へ置き換えた。バックリンク一覧の浮動表示は維持する。

- [x] 整理／AIタブ、共通枠、最小化／再開、ノート未選択時の整理表示を実装する。
- [x] scope別整理sessionとAI入力・処理状態をタブ切替で保持し、ロック時は安全上の消去を行う。
- [x] `test:organization`、`test:ai-workspace`、`test:ai-chat`、`test:content-locks`、`test:shortcuts`、Frontend typecheck／buildを実行する。
- [ ] Wails実画面で300／320px幅、右／下／浮動、移動・サイズ変更、AI応答中と整理適用中の切替、ロック後の非表示タブを確認する。

- [ ] Wails実画面でフローティング／ドック切替、移動・サイズ変更、最小化・復帰、Notebook scope、候補の変更前／変更案／理由と承認、失敗時の再解析、小型パネルの対象共有を確認する。Computer Useはブラウザー面のみ有効で、Windowsアプリ操作ができず未確認。
- [ ] 大規模保存空間で解析中の応答性と進捗の見せ方を評価する。

## 完了条件

- 変更候補を利用者が個別に確認・承認できる。
- 解析はMarkdown／SQLiteへ書き込まず、適用は既存保存・CAS・回復経路を維持する。
- 保護・ロック・ゴミ箱・stale revisionに対し安全側で停止する。
- 自動テスト成功後、手動UI受け入れと大規模データ評価の残件をstatusへ明記する。
