export type StorageSpaceSwitchPreparation = {
  ready: boolean
  rollback?: () => void | Promise<void>
}

export type StorageSpaceSwitchDependencies = {
  isBackupBusy: () => boolean
  isSyncBusy: () => boolean
  isAIBusy: () => boolean
  isImportBusy: () => boolean
  isExportBusy: () => boolean
  suspendSync: () => boolean
  resumeSync: () => void
  flushAllDirtyNotes: () => Promise<boolean>
  notify: (message: string, code: string) => void
}

export async function prepareStorageSpaceSwitch(
  dependencies: StorageSpaceSwitchDependencies,
): Promise<StorageSpaceSwitchPreparation> {
  if (dependencies.isBackupBusy()) {
    dependencies.notify('バックアップ処理の完了後に保存空間を切り替えてください', 'STORAGE_SPACE_BACKUP_BUSY')
    return { ready: false }
  }
  if (dependencies.isSyncBusy()) {
    dependencies.notify('同期の完了後に保存空間を切り替えてください', 'STORAGE_SPACE_SYNC_BUSY')
    return { ready: false }
  }
  if (dependencies.isAIBusy()) {
    dependencies.notify('AI処理の完了またはキャンセル後に保存空間を切り替えてください', 'STORAGE_SPACE_AI_BUSY')
    return { ready: false }
  }
  if (dependencies.isImportBusy()) {
    dependencies.notify('インポートの完了後に保存空間を切り替えてください', 'STORAGE_SPACE_IMPORT_BUSY')
    return { ready: false }
  }
  if (dependencies.isExportBusy()) {
    dependencies.notify('エクスポートの完了後に保存空間を切り替えてください', 'STORAGE_SPACE_EXPORT_BUSY')
    return { ready: false }
  }
  if (!dependencies.suspendSync()) {
    dependencies.notify('同期を停止できなかったため、保存空間を切り替えませんでした', 'STORAGE_SPACE_SYNC_BUSY')
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
      dependencies.notify('未保存の変更を保存できないため、保存空間を切り替えませんでした', 'STORAGE_SPACE_DRAFT_SAVE_FAILED')
      return { ready: false }
    }
    if (dependencies.isSyncBusy()) {
      rollback()
      dependencies.notify('同期の完了後に保存空間を切り替えてください', 'STORAGE_SPACE_SYNC_BUSY')
      return { ready: false }
    }
    if (dependencies.isBackupBusy()) {
      rollback()
      dependencies.notify('バックアップ処理が開始されたため、保存空間を切り替えませんでした', 'STORAGE_SPACE_BACKUP_BUSY')
      return { ready: false }
    }
    if (dependencies.isAIBusy()) {
      rollback()
      dependencies.notify('AI処理が開始されたため、保存空間を切り替えませんでした', 'STORAGE_SPACE_AI_BUSY')
      return { ready: false }
    }
    if (dependencies.isImportBusy()) {
      rollback()
      dependencies.notify('インポートが開始されたため、保存空間を切り替えませんでした', 'STORAGE_SPACE_IMPORT_BUSY')
      return { ready: false }
    }
    if (dependencies.isExportBusy()) {
      rollback()
      dependencies.notify('エクスポートが開始されたため、保存空間を切り替えませんでした', 'STORAGE_SPACE_EXPORT_BUSY')
      return { ready: false }
    }
    return { ready: true, rollback }
  } catch {
    rollback()
    dependencies.notify('未保存の変更を確認できないため、保存空間を切り替えませんでした', 'STORAGE_SPACE_DRAFT_SAVE_FAILED')
    return { ready: false }
  }
}
