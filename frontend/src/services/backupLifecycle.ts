export type BackupPreparation = {
  ready: boolean
  rollback?: () => void | Promise<void>
}

export type BackupLifecycleDependencies = {
  isStorageSpaceBusy: () => boolean
  isSyncBusy: () => boolean
  isAIBusy: () => boolean
  isImportBusy: () => boolean
  isExportBusy: () => boolean
  isContentLockBusy: () => boolean
  suspendSync: () => boolean
  resumeSync: () => void
  flushAllDirtyNotes: () => Promise<boolean>
  notify: (message: string, code: string) => void
}

export async function prepareBackupOperation(
  dependencies: BackupLifecycleDependencies,
): Promise<BackupPreparation> {
  if (dependencies.isStorageSpaceBusy()) {
    dependencies.notify('保存空間の処理が完了してからバックアップを実行してください', 'BACKUP_STORAGE_SPACE_BUSY')
    return { ready: false }
  }
  if (dependencies.isSyncBusy()) {
    dependencies.notify('同期の完了後にバックアップを実行してください', 'BACKUP_SYNC_BUSY')
    return { ready: false }
  }
  if (dependencies.isAIBusy()) {
    dependencies.notify('AI処理の完了またはキャンセル後にバックアップを実行してください', 'BACKUP_AI_BUSY')
    return { ready: false }
  }
  if (dependencies.isImportBusy()) {
    dependencies.notify('インポートの完了後にバックアップを実行してください', 'BACKUP_IMPORT_BUSY')
    return { ready: false }
  }
  if (dependencies.isExportBusy()) {
    dependencies.notify('エクスポートの完了後にバックアップを実行してください', 'BACKUP_EXPORT_BUSY')
    return { ready: false }
  }
  if (dependencies.isContentLockBusy()) {
    dependencies.notify('ロック処理の完了後にバックアップを実行してください', 'BACKUP_CONTENT_LOCK_BUSY')
    return { ready: false }
  }
  if (!dependencies.suspendSync()) {
    dependencies.notify('同期を停止できなかったため、バックアップを実行しませんでした', 'BACKUP_SYNC_BUSY')
    return { ready: false }
  }

  let resumed = false
  const rollback = () => {
    if (resumed) return
    resumed = true
    dependencies.resumeSync()
  }

  try {
    if (!await dependencies.flushAllDirtyNotes()) {
      rollback()
      dependencies.notify('未保存の変更を保存できないため、バックアップを実行しませんでした', 'BACKUP_DRAFT_SAVE_FAILED')
      return { ready: false }
    }
    if (dependencies.isSyncBusy()) {
      rollback()
      dependencies.notify('同期が開始されたため、バックアップを実行しませんでした', 'BACKUP_SYNC_BUSY')
      return { ready: false }
    }
    if (dependencies.isStorageSpaceBusy()) {
      rollback()
      dependencies.notify('保存空間の処理が開始されたため、バックアップを実行しませんでした', 'BACKUP_STORAGE_SPACE_BUSY')
      return { ready: false }
    }
    if (dependencies.isAIBusy()) {
      rollback()
      dependencies.notify('AI処理が開始されたため、バックアップを実行しませんでした', 'BACKUP_AI_BUSY')
      return { ready: false }
    }
    if (dependencies.isImportBusy()) {
      rollback()
      dependencies.notify('インポートが開始されたため、バックアップを実行しませんでした', 'BACKUP_IMPORT_BUSY')
      return { ready: false }
    }
    if (dependencies.isExportBusy()) {
      rollback()
      dependencies.notify('エクスポートが開始されたため、バックアップを実行しませんでした', 'BACKUP_EXPORT_BUSY')
      return { ready: false }
    }
    if (dependencies.isContentLockBusy()) {
      rollback()
      dependencies.notify('ロック処理が開始されたため、バックアップを実行しませんでした', 'BACKUP_CONTENT_LOCK_BUSY')
      return { ready: false }
    }
    return { ready: true, rollback }
  } catch {
    rollback()
    dependencies.notify('未保存の変更を確認できないため、バックアップを実行しませんでした', 'BACKUP_DRAFT_SAVE_FAILED')
    return { ready: false }
  }
}
