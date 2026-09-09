package main

import (
	"atlasnote/internal/appcleanup"
	"context"
	"strings"
)

const maintenanceCommand = "--atlasnote-maintenance"

func runMaintenanceCommand(args []string) (bool, int) {
	if len(args) == 0 || args[0] != maintenanceCommand {
		return false, 0
	}
	request, err := appcleanup.ParseMaintenanceRequest(args[1:], appcleanup.RegisteredUserSID)
	if err != nil {
		reportMaintenanceFailure("対象利用者のSIDと通常利用の記録を確認できません。追加削除は実行していません。本人のWindowsセッションでAtlas Noteを管理者として実行せず一度起動し、終了してください。別管理者のUACが必要な場合は、本人の通常権限のPowerShellからAtlasNote.exe --atlasnote-maintenance --registered-user --delete-display-settings --delete-credentials（必要な削除対象だけ指定）を実行後、追加削除を選ばずアンインストールしてください。")
		return true, 1
	}
	result := appcleanup.NewService().Run(context.Background(), request)
	if result.Error != nil {
		reportMaintenanceFailure(result.Error.Message)
		return true, 1
	}
	return true, 0
}
func reportMaintenanceFailure(message string) {
	if strings.TrimSpace(message) == "" {
		message = "保守処理を完了できませんでした。"
	}
	reportMaintenanceFailurePlatform(message)
}
