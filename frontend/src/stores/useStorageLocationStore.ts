import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  applyStorageLocations,
  cancelPendingStorageLocationMigration,
  getStorageLocationStatus,
  retryPendingStorageLocationMigration,
  selectStorageLocation,
  type StorageLocationError,
  type StorageLocationStatus,
} from '../api/storageLocations'

type PrepareChange = () => Promise<{ ready: boolean; rollback?: () => void | Promise<void> }>
type RestartApplication = () => Promise<void>

const unavailableError: StorageLocationError = {
  code: 'STORAGE_LOCATION_UNAVAILABLE',
  message: '保存場所を利用できませんでした。現在のデータは変更していません。',
}
const developmentRestartError = 'automatic restart is unavailable in Wails development mode'

function restartFailureMessage(cause: unknown) {
  const message = cause instanceof Error ? cause.message : typeof cause === 'string' ? cause : ''
  if (message.includes(developmentRestartError)) {
    return '保存場所を保存しました。Atlas Noteを閉じて、wails devを再実行してください。'
  }
  return '保存場所を保存しましたが、自動再起動できませんでした。Atlas Noteを手動で再起動してください。'
}

export const useStorageLocationStore = defineStore('storage-locations', () => {
  const status = ref<StorageLocationStatus | null>(null)
  const isLoading = ref(false)
  const isOperating = ref(false)
  const error = ref<StorageLocationError | null>(null)
  let prepareChange: PrepareChange | null = null
  let restartApplication: RestartApplication | null = null

  const isBusy = computed(() => isLoading.value || isOperating.value)

  async function initialize() {
    if (isLoading.value) return false
    isLoading.value = true
    error.value = null
    try {
      const result = await getStorageLocationStatus()
      if (result.error || !result.status) {
        error.value = result.error ?? unavailableError
        return false
      }
      status.value = result.status
      return true
    } catch {
      error.value = unavailableError
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function choose(kind: 'data' | 'backup') {
    if (isBusy.value) return false
    isOperating.value = true
    error.value = null
    try {
      const result = await selectStorageLocation(kind)
      if (result.error) {
        error.value = result.error
        return false
      }
      if (result.status) status.value = result.status
      return !result.canceled
    } catch {
      error.value = unavailableError
      return false
    } finally {
      isOperating.value = false
    }
  }

  async function apply() {
    if (isBusy.value) return false
    isOperating.value = true
    error.value = null
    let preparation: { ready: boolean; rollback?: () => void | Promise<void> } | null = null
    try {
      preparation = prepareChange ? await prepareChange() : { ready: true }
      if (!preparation.ready) return false
      const result = await applyStorageLocations()
      if (result.error) {
        error.value = result.error
        await preparation.rollback?.()
        return false
      }
      if (result.status) status.value = result.status
      if (result.restartRequired && restartApplication) {
        try {
          await restartApplication()
        } catch (cause) {
          await preparation.rollback?.()
          error.value = { code: 'STORAGE_LOCATION_RESTART_FAILED', message: restartFailureMessage(cause) }
          return false
        }
      }
      return true
    } catch {
      await preparation?.rollback?.()
      error.value = unavailableError
      return false
    } finally {
      isOperating.value = false
    }
  }

  async function runPendingMigrationAction(action: () => Promise<{
    status?: StorageLocationStatus
    restartRequired: boolean
    error?: StorageLocationError
  }>) {
    if (isBusy.value) return false
    isOperating.value = true
    error.value = null
    try {
      const result = await action()
      if (result.error) {
        error.value = result.error
        return false
      }
      if (result.status) status.value = result.status
      if (result.restartRequired && restartApplication) {
        try {
          await restartApplication()
        } catch (cause) {
          error.value = { code: 'STORAGE_LOCATION_RESTART_FAILED', message: restartFailureMessage(cause) }
          return false
        }
      }
      return true
    } catch {
      error.value = unavailableError
      return false
    } finally {
      isOperating.value = false
    }
  }

  function cancelPendingMigration() {
    return runPendingMigrationAction(cancelPendingStorageLocationMigration)
  }

  function retryPendingMigration() {
    return runPendingMigrationAction(retryPendingStorageLocationMigration)
  }

  function setLifecycle(prepare: PrepareChange, restart: RestartApplication) {
    prepareChange = prepare
    restartApplication = restart
  }

  function clearLifecycle() {
    prepareChange = null
    restartApplication = null
  }

  function clearError() {
    error.value = null
  }

  return {
    status,
    isLoading,
    isOperating,
    isBusy,
    error,
    initialize,
    choose,
    apply,
    cancelPendingMigration,
    retryPendingMigration,
    setLifecycle,
    clearLifecycle,
    clearError,
  }
})
