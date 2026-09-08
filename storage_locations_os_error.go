package main

import (
	"errors"
	"os"
)

// Only classified OS causes become shared guidance; raw errors stay private.
func storageLocationOSGuidance(err error) (string, string) {
	if errors.Is(err, os.ErrPermission) {
		return "アクセス拒否", "フォルダへのアクセス権限を確認するか、利用可能な別のフォルダを選択してください。"
	}
	switch storageLocationPlatformOSCause(err) {
	case "busy":
		return "共有違反・使用中", "このフォルダやファイルを使用しているアプリを閉じてから再試行してください。"
	case "full":
		return "空き容量不足", "保存先の空き容量を確認して確保するか、空き容量のある別の保存先を選択してください。"
	}
	return "", ""
}
