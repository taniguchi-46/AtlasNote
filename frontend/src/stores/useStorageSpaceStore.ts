import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  createStorageSpace,
  listStorageSpaces,
  selectStorageSpace,
  type StorageSpace,
  type StorageSpaceError,
} from '../api/storageSpaces'
import type { StorageSpaceSwitchPreparation } from '../services/storageSpaceSwitch'

type PrepareSwitch = () => Promise<StorageSpaceSwitchPreparation>
type RestartApplication = () => Promise<void>

const unavailableError: StorageSpaceError = {
  code: 'STORAGE_SPACE_UNAVAILABLE',
  message: '保存空間を利用できませんでした。データは変更していません。',
}

const developmentRestartError = 'automatic restart is unavailable in Wails development mode'

function restartFailureMessage(cause: unknown) {
  const message = cause instanceof Error ? cause.message : typeof cause === 'string' ? cause : ''
  if (message.includes(developmentRestartError)) {
    return '保存空間は選択済みです。Atlas Noteを閉じて、wails devを再実行してください。'
  }
  return '保存空間は選択済みですが、自動再起動できませんでした。Atlas Noteを手動で再起動してください。'
}

export const useStorageSpaceStore = defineStore('storage-spaces', () => {
  const spaces = ref<StorageSpace[]>([])
  const activeSpaceId = ref('')
  const isLoading = ref(false)
  const isCreating = ref(false)
  const isSwitching = ref(false)
  const error = ref<StorageSpaceError | null>(null)
  let prepareSwitch: PrepareSwitch | null = null
  let restartApplication: RestartApplication | null = null

  const activeSpace = computed(() => (
    spaces.value.find((space) => space.id === activeSpaceId.value) ?? null
  ))
  const isBusy = computed(() => isLoading.value || isCreating.value || isSwitching.value)

  async function initialize() {
    if (isLoading.value) return false
    isLoading.value = true
    error.value = null
    try {
      const result = await listStorageSpaces()
      if (result.error) {
        error.value = result.error
        spaces.value = []
        activeSpaceId.value = ''
        return false
      }
      spaces.value = result.spaces ?? []
      activeSpaceId.value = result.activeSpaceId
      return true
    } catch {
      error.value = unavailableError
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function create(name: string) {
    if (isBusy.value) return null
    isCreating.value = true
    error.value = null
    try {
      const result = await createStorageSpace(name)
      if (result.error || !result.space) {
        error.value = result.error ?? unavailableError
        return null
      }
      activeSpaceId.value = result.activeSpaceId ?? activeSpaceId.value
      spaces.value = [
        ...spaces.value.filter((space) => space.id !== result.space!.id),
        result.space,
      ]
      return result.space
    } catch {
      error.value = unavailableError
      return null
    } finally {
      isCreating.value = false
    }
  }

  async function switchTo(id: string) {
    if (isBusy.value || id === activeSpaceId.value) return id === activeSpaceId.value
    if (!prepareSwitch || !restartApplication) {
      error.value = unavailableError
      return false
    }

    isSwitching.value = true
    error.value = null
    let preparation: StorageSpaceSwitchPreparation | null = null
    let selectionPersisted = false
    try {
      preparation = await prepareSwitch()
      if (!preparation.ready) return false

      const result = await selectStorageSpace(id)
      if (result.error || !result.space) {
        error.value = result.error ?? unavailableError
        await preparation.rollback?.()
        return false
      }
      selectionPersisted = true
      activeSpaceId.value = result.activeSpaceId ?? result.space.id
      spaces.value = spaces.value.map((space) => ({
        ...space,
        active: space.id === activeSpaceId.value,
      }))
      if (result.restartRequired) await restartApplication()
      return true
    } catch (cause) {
      if (!selectionPersisted) await preparation?.rollback?.()
      error.value = selectionPersisted
        ? {
            code: 'STORAGE_SPACE_RESTART_FAILED',
            message: restartFailureMessage(cause),
          }
        : unavailableError
      return false
    } finally {
      isSwitching.value = false
    }
  }

  function setSwitchLifecycle(prepare: PrepareSwitch, restart: RestartApplication) {
    prepareSwitch = prepare
    restartApplication = restart
  }

  function clearSwitchLifecycle() {
    prepareSwitch = null
    restartApplication = null
  }

  function clearError() {
    error.value = null
  }

  return {
    spaces,
    activeSpaceId,
    activeSpace,
    isLoading,
    isCreating,
    isSwitching,
    isBusy,
    error,
    initialize,
    create,
    switchTo,
    setSwitchLifecycle,
    clearSwitchLifecycle,
    clearError,
  }
})
