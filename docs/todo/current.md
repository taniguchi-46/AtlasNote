# Current TODO

更新: 2026-10-03

Multi Terminal / Splitの実装を優先し、その後にCLI / MCP / Terminal再編の残る受け入れ確認と最終監査を行う。要求と現在状況は[status](../status.md)および[再編仕様書](../development/specs/AtlasNote_CLI_MCP_Rearchitecture_Spec.md)を参照する。

## Multi Terminal

- [x] Backendの単一session管理を複数session管理へ変更する。
- [x] Terminal sessionの新規作成、切替、個別終了を実装する。
- [x] 全sessionを安全にShutdownする。
- [x] 既存PTY lifecycle、Job Object、EOF、ACK契約を維持し、対象テストを更新する。
- [ ] MANUAL_REQUIRED: 実Wailsで3 Terminalの別コマンド実行、tab切替・buffer保持、2番だけ終了・1番/3番継続、新規shell 4、Ctrl+C isolation、日本語IME、panel resize、app終了後のchild process残存なしを確認する。

## Split Terminal

- [ ] 複数Terminalを左右分割表示する。
- [ ] Splitの作成・終了、focus切替を実装する。
- [ ] Panel配置・サイズ変更に追従させる。

## 受け入れ・最終確認

- [ ] Stage Cのdirty draftとCAS conflictをE2E確認する。
- [ ] Protected / Locked fail-closedと保存空間切替後のpending operation無効化を実接続で確認する。
- [ ] Stage Eの旧AI記録viewerを実画面で確認する。
- [ ] WebDAV同期とBackupの安全なsmoke testを行う。
- [ ] 実CodexまたはClaude CodeからMCP、GUI承認、保存、再起動までのE2Eを確認する。
- [ ] Terminal Cursor Blink、日本語IME、最終UI/UXを実Wailsで受け入れ確認する。
- [ ] 最終独立監査を実施する。

整理センターの性能評価と実Wails UI確認は[専用TODO](todo-pre-phase5-organization.md)で管理する。
