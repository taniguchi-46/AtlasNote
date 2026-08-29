import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import {
  cancelBackupRestore,
  createAutomaticBackup,
  executeBackupRestore,
  getBackupStatus,
  listBackups,
  previewBackupRestore,
  type AutomaticBackupResult,
  type BackupError,
  type BackupRestorePreview,
  type BackupStatusResult,
  type BackupSummary,
} from '../api/backups'
import type { BackupLifecycleDependencies, BackupPreparation } from '../services/backupLifecycle'
import { useNotificationStore } from './useNotificationStore'

const automaticEnabledStorageKey = 'atlas-backup-automatic-enabled'
const schedulerIntervalMs = 15 * 60 * 1000

const unavailableError: BackupError = {
  code: 'BACKUP_UNAVAILABLE',
  message: 'バックアップを利用できませんでした。既存のデータは変更していません。',
}

const developmentRestartError = 'automatic restart is unavailable in Wails development mode'

type RestartApplication = () => Promise<void>

function readAutomaticEnabled() {
  return localStorage.getItem(automaticEnabledStorageKey) !== 'false'
}

function restartFailureMessage(cause: unknown) {
  const message = cause instanceof Error ? cause.message : typeof cause === 'string' ? cause : ''
  if (message.includes(developmentRestartError)) {
    return '復元の準備は完了しています。Atlas Noteを閉じて、wails devを再実行してください。'
  }
  return '復元の準備は完了していますが、自動再起動できませんでした。Atlas Noteを手動で再起動してください。'
}

export const useBackupStore = defineStore('backups', () => {
  const backups = ref<BackupSummary[]>([])
  const status = ref<BackupStatusResult | null>(null)
  const automaticEnabled = ref(readAutomaticEnabled())
  const isLoading = ref(false)
  const isOperating = ref(false)
  const error = ref<BackupError | null>(null)
  let prepareOperation: (() => Promise<BackupPreparation>) | null = null
  let restartApplication: RestartApplication | null = null
  let schedulerTimer: ReturnType<typeof setInterval> | null = null

  const notificationStore = useNotificationStore()
  const isBusy = computed(() => isLoading.value || isOperating.value)

  watch(automaticEnabled, (enabled) => {
    localStorage.setItem(automaticEnabledStorageKey, String(enabled))
  })

  async function refreshStatus() {
    try {
      const result = await getBackupStatus()
      if (result.error) {
        error.value = result.error
        status.value = null
        return false
      }
      status.value = result
      return true
    } catch {
      error.value = unavailableError
      status.value = null
      return false
    }
  }

  async function initialize() {
    if (isLoading.value) return false
    isLoading.value = true
    error.value = null
    try {
      const [listResult, statusResult] = await Promise.all([listBackups(), getBackupStatus()])
      if (listResult.error || statusResult.error) {
        error.value = listResult.error ?? statusResult.error ?? unavailableError
        backups.value = []
        status.value = null
        return false
      }
      backups.value = listResult.backups ?? []
      status.value = statusResult
      return true
    } catch {
      error.value = unavailableError
      backups.value = []
      status.value = null
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function runAutomaticBackup(): Promise<AutomaticBackupResult | null> {
    if (!automaticEnabled.value || isOperating.value) return null
    isOperating.value = true
    let preparation: BackupPreparation | null = null
    try {
      preparation = prepareOperation ? await prepareOperation() : { ready: true }
      if (!preparation.ready) return null

      const result = await createAutomaticBackup()
      if (result.error) {
        error.value = result.error
        await preparation.rollback?.()
        return result
      }
      if (result.backup) {
        backups.value = [result.backup, ...backups.value.filter((item) => item.id !== result.backup!.id)]
        status.value = status.value
          ? { ...status.value, automaticDue: false, lastAutomaticAt: result.backup.createdAt }
          : status.value
      }
      await preparation.rollback?.()
      return result
    } catch {
      await preparation?.rollback?.()
      error.value = unavailableError
      return null
    } finally {
      isOperating.value = false
    }
  }

  async function runAutomaticIfDue() {
    if (!automaticEnabled.value || status.value?.automaticDue !== true || isBusy.value) return null
    const result = await runAutomaticBackup()
    if (result?.error) {
      notificationStore.notify(result.error.message, {
        kind: 'warning', source: 'backup', code: result.error.code,
        retryable: true, dedupeKey: `backup:auto:${result.error.code}`,
      })
    }
    return result
  }

  async function previewRestore(backupID: string): Promise<BackupRestorePreview | null> {
    if (isOperating.value) return null
    isOperating.value = true
    error.value = null
    try {
      const result = await previewBackupRestore(backupID)
      if (result.error || !result.preview) {
        error.value = result.error ?? unavailableError
        return null
      }
      return result.preview
    } catch {
      error.value = unavailableError
      return null
    } finally {
      isOperating.value = false
    }
  }

  async function restore(preview: BackupRestorePreview) {
    if (isOperating.value) return false
    if (!prepareOperation || !restartApplication) {
      error.value = unavailableError
      return false
    }

    error.value = null
    isOperating.value = true
    let preparation: BackupPreparation | null = null
    let restorePrepared = false
    try {
      preparation = await prepareOperation()
      if (!preparation.ready) return false

      const result = await executeBackupRestore(preview.token)
      if (result.error || !result.restartRequired) {
        error.value = result.error ?? unavailableError
        await preparation.rollback?.()
        return false
      }
      restorePrepared = true
      await restartApplication()
      return true
    } catch (cause) {
      if (restorePrepared) await cancelBackupRestore().catch(() => {})
      await preparation?.rollback?.()
      error.value = {
        code: 'BACKUP_RESTART_FAILED',
        message: restorePrepared ? restartFailureMessage(cause) : unavailableError.message,
      }
      return false
    } finally {
      isOperating.value = false
    }
  }

  function setLifecycle(
    prepare: () => Promise<BackupPreparation>,
    restart: RestartApplication,
  ) {
    prepareOperation = prepare
    restartApplication = restart
  }

  function clearLifecycle() {
    prepareOperation = null
    restartApplication = null
  }

  function startScheduler() {
    if (schedulerTimer) clearInterval(schedulerTimer)
    schedulerTimer = setInterval(() => {
      if (!automaticEnabled.value || isBusy.value) return
      void refreshStatus().then((available) => {
        if (available) void runAutomaticIfDue()
      })
    }, schedulerIntervalMs)
  }

  function dispose() {
    if (schedulerTimer) clearInterval(schedulerTimer)
    schedulerTimer = null
    clearLifecycle()
  }

  function clearError() {
    error.value = null
  }

  return {
    backups,
    status,
    automaticEnabled,
    isLoading,
    isOperating,
    isBusy,
    error,
    initialize,
    refreshStatus,
    runAutomaticBackup,
    runAutomaticIfDue,
    previewRestore,
    restore,
    setLifecycle,
    clearLifecycle,
    startScheduler,
    dispose,
    clearError,
  }
})
