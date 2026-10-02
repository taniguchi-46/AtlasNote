# AtlasNote Status

最終更新: 2026-10-03

## Current
CLI / MCP / Terminal再設計（Stage A〜E）は実装と主要監査を完了。CLI / MCPの実接続、変更のReject / Approve / 再起動後の保存、単一Terminalの主要操作は確認済み。Terminal UIはdock位置、appearance、文字サイズ・font、最大化／復元を備える。D.1 Multi Terminalの生成・切替・個別終了と全session Shutdownを実装。Multi Terminalの実Wails確認は未実施で、Splitは未実装。

## Completed
- CLI / MCP / Terminal再設計 Stage A〜E
- Terminal単一sessionの主要UI/UXと設定
- D.1 Multi Terminalの実装と自動テスト（実Wailsの手動確認は残る）
- Phase 4 v1〜v3（2026-08-24完了）
- Phase 2・3

Stage Aでは読み取りCLI / MCPと安全なIPC、Stage Bでは読み取り・候補公開、Stage Cでは承認付き変更要求、Stage DではPTY統合Terminal、Stage Eでは旧AI生成GUIを撤去して既存AI記録のread-only閲覧を実装。各Stageの詳細契約は[再編仕様書](development/specs/AtlasNote_CLI_MCP_Rearchitecture_Spec.md)を参照する。過去の作業履歴・検証記録は[Archive](archive/README.md)に保管。

## Current Work
Multi Terminalの実Wails確認を行い、その後Split Terminalの分割・focus・resizeへ進む。D.1の自動テストはsession別I/O・ACK・終了、Start／Shutdown競合、Windows実ConPTYの3 sessionと子process cleanupを対象とする。既存PTY lifecycle、Job Object、EOF、ACK契約を維持する。詳細チェックリストは[Current TODO](todo/current.md)。

## Remaining Manual Verification
CLI / MCP / Terminal再編のE2E、保護状態・保存空間切替、旧AI記録viewer、同期・Backup、TerminalのIME・表示確認、最終独立監査が残る。詳細は[Current TODO](todo/current.md)。整理センターの性能・実Wails UI確認は[専用TODO](todo/todo-pre-phase5-organization.md)。

## Known Issues
現行資料だけでは、再現中の不具合の有無を確定できない。未完了の監査・手動確認項目は不具合なしを示すものではない。

## Next
1. Multi Terminalの実Wails確認
2. Split Terminal
3. 受け入れ確認と最終独立監査

## 正本
- 現在状況: 本書
- 要求・機能仕様: [development](development/README.md)
- 恒久ルール: [rules](rules/ai.md)
- 未完了作業: [todo](todo/README.md)
- 完了記録: [archive](archive/README.md)
- Agent運用: ルート `AGENTS.md` と `.agents/`
