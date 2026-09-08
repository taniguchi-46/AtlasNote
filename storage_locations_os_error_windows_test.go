//go:build windows

package main

import "golang.org/x/sys/windows"

func storageOSCauseCases() []storageOSCauseCase {
	return []storageOSCauseCase{
		{windows.ERROR_ACCESS_DENIED, "アクセス拒否", "アクセス権限"},
		{windows.ERROR_SHARING_VIOLATION, "共有違反・使用中", "アプリを閉じて"},
		{windows.ERROR_LOCK_VIOLATION, "共有違反・使用中", "アプリを閉じて"},
		{windows.ERROR_DISK_FULL, "空き容量不足", "空き容量"},
		{windows.ERROR_HANDLE_DISK_FULL, "空き容量不足", "空き容量"},
		{9999, "書き込み不可", "別の通常フォルダ"},
	}
}
