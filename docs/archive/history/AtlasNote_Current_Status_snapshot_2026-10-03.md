# AtlasNote 現在状況と残作業

## 現在状況

- Branch: `codex/cli-mcp-terminal-rearchitecture`
- Remote HEAD: `5d428a315810b9559310b65c8df2f473036d69bc`
- Stage 0〜E: 実装・主要監査完了
- CLI / MCP: 実接続確認済み
- Stage C: Reject / Approve / 再起動後の保存確認済み
- 単一Terminal: 実Wailsで主要操作確認済み
- VS Code風Terminal UI: 実装・監査・主要GUI確認済み
- Reject後の状態表示修正: 実Wails確認済み、未commit

## 優先実装

### 1. Multi Terminal
- Backendを単一sessionから複数session管理へ変更
- Terminal新規作成
- Terminal切替
- Terminal個別終了
- 全sessionの安全なShutdown
- 既存PTY lifecycle / Job Object / EOF / ACK契約を維持

### 2. Split Terminal
- 複数Terminalを左右分割表示
- Split作成 / 閉じる / focus切替
- resizeとPanel配置への追従

## Multi Terminal / Split後の残作業

- Stage C dirty draft E2E
- Stage C CAS conflict E2E
- Protected / Locked fail-closed実接続確認
- Storage Space切替後のpending operation無効化確認
- Stage E旧AI記録viewer実画面確認
- WebDAV同期 / Backupの安全なsmoke test
- 実CodexまたはClaude Code → MCP → GUI承認 → 保存 → 再起動E2E
- 最終独立監査

## 人間確認

- Terminal Cursor Blinkの目視
- 日本語IMEの漢字変換と操作感
- 最終UI/UX受け入れ

## 進行順

`Multi Terminal → Split Terminal → 残E2E → 実AI連携 → 最終監査 → 最終受け入れ`
