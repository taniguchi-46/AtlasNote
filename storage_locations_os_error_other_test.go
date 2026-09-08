//go:build !windows

package main

import "syscall"

func storageOSCauseCases() []storageOSCauseCase {
	return []storageOSCauseCase{
		{syscall.EACCES, "アクセス拒否", "アクセス権限"},
		{syscall.EBUSY, "共有違反・使用中", "アプリを閉じて"},
		{syscall.ENOSPC, "空き容量不足", "空き容量"},
		{32, "書き込み不可", "別の通常フォルダ"},
		{9999, "書き込み不可", "別の通常フォルダ"},
	}
}
