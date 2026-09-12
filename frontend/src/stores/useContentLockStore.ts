import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  changeContentLockPassphrase,
  disableContentLock,
  enableContentLock,
  getContentLockStatus,
  listContentLocks,
  listRequiredContentLocks,
  listStorageSpaceLockStatuses,
  lockContentNow,
  lockContentTargetsNow,
  unlockContentLock,
  type ContentLock,
  type ContentLockError,
  type ContentLockMutationResult,
  type ContentLockStatus,
  type ContentLockTarget,
  type StorageSpaceLockStatus,
} from '../api/contentLocks'
import type {
  ContentLockBeforeLockResult,
  ContentLockPreparation,
} from '../utils/contentLockBeforeLock'

const unavailableError: ContentLockError = {
  code: 'CONTENT_LOCK_UNAVAILABLE',
  message: 'ロックを利用できませんでした。データは変更していません。',
}

type BeforeLock = () => Promise<ContentLockBeforeLockResult>

type ContentLockAccessRequest = {
  target: ContentLockTarget
  targetLabel: string
  requiredLocks: ContentLock[]
}

function targetKey(target: ContentLockTarget) {
  return `${target.type}:${target.id}`
}

function failureResult(error: ContentLockError): ContentLockMutationResult {
  return {
    removed: false,
    unlocked: false,
    restartRequired: false,
    error,
  }
}

function failureListResult(error: ContentLockError) {
  return { locks: [] as ContentLock[], error }
}

function targetForLock(lock: ContentLock): ContentLockTarget {
  return { type: lock.targetType, id: lock.targetId }
}

export const useContentLockStore = defineStore('content-locks', () => {
  const locks = ref<ContentLock[]>([])
  const statuses = ref<Record<string, ContentLockStatus>>({})
  const statusLoading = ref<Record<string, boolean>>({})
  const statusErrors = ref<Record<string, ContentLockError>>({})
  const spaceStatuses = ref<Record<string, StorageSpaceLockStatus>>({})
  const isLoading = ref(false)
  const isBusy = ref(false)
  const error = ref<ContentLockError | null>(null)
  const lastChangedTarget = ref<ContentLockTarget | null>(null)
  // Timestamps live only in the renderer session. They never contain a
  // passphrase or key and intentionally reset when the app is restarted.
  const unlockedAt = ref<Record<string, number>>({})
  const unlockVersion = ref(0)
  const accessRequest = ref<ContentLockAccessRequest | null>(null)
  const statusRequestVersions = new Map<string, number>()
  let afterLock: ((targets: ContentLockTarget[]) => Promise<void>) | null = null
  let beforeLock: BeforeLock | null = null
  let accessRequestResolver: ((allowed: boolean) => void) | null = null

  async function prepareBeforeLock() {
    if (!beforeLock) {
      return { allowed: true, preparation: null as ContentLockPreparation | null }
    }

    const result = await beforeLock()
    if (result === false) {
      return { allowed: false, preparation: null as ContentLockPreparation | null }
    }
    if (result === true) {
      return { allowed: true, preparation: null as ContentLockPreparation | null }
    }

    if (result.ok !== true) {
      return { allowed: false, preparation: null as ContentLockPreparation | null }
    }

    return { allowed: true, preparation: result }
  }

  function finishBeforeLock(preparation: ContentLockPreparation | null) {
    try {
      preparation?.finish()
    } catch {
      // Releasing the renderer-side input guard must not turn a completed
      // lock mutation into an unhandled promise rejection.
    }
  }

  const lockByTarget = computed(() => new Map(locks.value.map((lock) => [
    `${lock.targetType}:${lock.targetId}`,
    lock,
  ])))

  function syncUnlockedTimestamps(nextLocks: ContentLock[]) {
    const now = Date.now()
    const next: Record<string, number> = {}
    for (const lock of nextLocks) {
      if (!lock.unlocked) continue
      const key = targetKey(targetForLock(lock))
      next[key] = unlockedAt.value[key] ?? now
    }
    const previousKeys = Object.keys(unlockedAt.value)
    const nextKeys = Object.keys(next)
    const changed = previousKeys.length !== nextKeys.length
      || nextKeys.some((key) => unlockedAt.value[key] !== next[key])
    if (changed) {
      unlockedAt.value = next
      unlockVersion.value += 1
    }
  }

  function rememberUnlocked(target: ContentLockTarget) {
    unlockedAt.value = { ...unlockedAt.value, [targetKey(target)]: Date.now() }
    unlockVersion.value += 1
  }

  function forgetUnlocked(target: ContentLockTarget) {
    const key = targetKey(target)
    if (!(key in unlockedAt.value)) return
    const next = { ...unlockedAt.value }
    delete next[key]
    unlockedAt.value = next
    unlockVersion.value += 1
  }

  async function refresh() {
    if (isLoading.value) return false
    isLoading.value = true
    error.value = null
    try {
      const result = await listContentLocks()
      if (result.error) {
        error.value = result.error
        locks.value = []
        return false
      }
      locks.value = result.locks ?? []
      syncUnlockedTimestamps(locks.value)
      return true
    } catch {
      error.value = unavailableError
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function refreshTarget(target: ContentLockTarget) {
    const key = targetKey(target)
    const requestVersion = (statusRequestVersions.get(key) ?? 0) + 1
    statusRequestVersions.set(key, requestVersion)
    statusLoading.value = { ...statusLoading.value, [key]: true }
    if (statusErrors.value[key]) {
      const nextErrors = { ...statusErrors.value }
      delete nextErrors[key]
      statusErrors.value = nextErrors
    }
    try {
      const status = await getContentLockStatus(target)
      if (statusRequestVersions.get(key) === requestVersion) {
        statuses.value = { ...statuses.value, [key]: status }
      }
      return status
    } catch {
      if (statusRequestVersions.get(key) === requestVersion) {
        statusErrors.value = { ...statusErrors.value, [key]: unavailableError }
      }
      return null
    } finally {
      if (statusRequestVersions.get(key) === requestVersion) {
        statusLoading.value = { ...statusLoading.value, [key]: false }
      }
    }
  }

  async function refreshSpaceStatuses() {
    try {
      const result = await listStorageSpaceLockStatuses()
      if (result.error) {
        error.value = result.error
        return false
      }
      const next: Record<string, StorageSpaceLockStatus> = {}
      for (const status of result.statuses ?? []) next[status.spaceId] = status
      spaceStatuses.value = next
      return true
    } catch {
      error.value = unavailableError
      return false
    }
  }

  async function runMutation(
    target: ContentLockTarget,
    operation: () => Promise<ContentLockMutationResult>,
    notifyChange = true,
    busyAlreadyHeld = false,
  ) {
    if (!busyAlreadyHeld && isBusy.value) return failureResult(unavailableError)
    if (!busyAlreadyHeld) {
      isBusy.value = true
      error.value = null
    }
    try {
      const result = await operation()
      if (result.error) {
        error.value = result.error
        return result
      }
      await Promise.all([
        refresh(),
        refreshTarget(target),
        target.type === 'space' ? refreshSpaceStatuses() : Promise.resolve(true),
      ])
      if (notifyChange) lastChangedTarget.value = { ...target }
      return result
    } catch {
      error.value = unavailableError
      return failureResult(unavailableError)
    } finally {
      if (!busyAlreadyHeld) isBusy.value = false
    }
  }

  async function enable(target: ContentLockTarget, passphrase: string, deleteAIRecords = false) {
    const result = await runMutation(target, () => enableContentLock({
      targetType: target.type,
      targetId: target.id,
      passphrase,
      deleteAIRecords,
    }))
    if (!result.error && result.unlocked) rememberUnlocked(target)
    return result
  }

  async function unlock(target: ContentLockTarget, passphrase: string) {
    const result = await runMutation(target, () => unlockContentLock({
      targetType: target.type,
      targetId: target.id,
      passphrase,
    }))
    if (!result.error && result.unlocked) rememberUnlocked(target)
    return result
  }

  async function lockNow(target: ContentLockTarget) {
    if (isBusy.value) return failureResult(unavailableError)

    isBusy.value = true
    error.value = null
    let preparation: ContentLockPreparation | null = null
    try {
      const prepared = await prepareBeforeLock()
      if (!prepared.allowed) return failureResult(unavailableError)
      preparation = prepared.preparation

      const result = await runMutation(target, () => lockContentNow(target), false, true)
      if (!result.error) {
        forgetUnlocked(target)
        await refreshAfterLock([target])
      }
      return result
    } catch {
      error.value = unavailableError
      return failureResult(unavailableError)
    } finally {
      finishBeforeLock(preparation)
      isBusy.value = false
    }
  }

  async function changePassphrase(target: ContentLockTarget, currentPassphrase: string, newPassphrase: string) {
    const result = await runMutation(target, () => changeContentLockPassphrase({
      targetType: target.type,
      targetId: target.id,
      currentPassphrase,
      newPassphrase,
    }))
    if (!result.error && result.unlocked) rememberUnlocked(target)
    return result
  }

  async function disable(target: ContentLockTarget, passphrase: string) {
    const result = await runMutation(target, () => disableContentLock({
      targetType: target.type,
      targetId: target.id,
      passphrase,
    }))
    if (!result.error && result.removed) forgetUnlocked(target)
    return result
  }

  async function lockTargetsNow(targets: ContentLockTarget[]) {
    const unique = new Map<string, ContentLockTarget>()
    for (const target of targets) unique.set(targetKey(target), { ...target })
    const pendingTargets = Array.from(unique.values())
    if (pendingTargets.length === 0) return { locks: [] as ContentLock[] }
    if (isBusy.value) return failureListResult(unavailableError)

    isBusy.value = true
    error.value = null
    let preparation: ContentLockPreparation | null = null
    try {
      const prepared = await prepareBeforeLock()
      if (!prepared.allowed) {
        const saveError: ContentLockError = {
          code: 'CONTENT_LOCK_SAVE_FAILED',
          message: '保存に失敗したため自動ロックを保留しました。保存後に再試行します。',
        }
        error.value = saveError
        isBusy.value = false
        return failureListResult(saveError)
      }
      preparation = prepared.preparation
    } catch {
      finishBeforeLock(preparation)
      error.value = unavailableError
      isBusy.value = false
      return failureListResult(unavailableError)
    }

    try {
      const result = await lockContentTargetsNow(pendingTargets)
      if (result.error) {
        error.value = result.error
        return result
      }

      const lockedTargets = (result.locks ?? []).map(targetForLock)
      await Promise.all([
        refresh(),
        ...lockedTargets.map((target) => refreshTarget(target)),
        lockedTargets.some((target) => target.type === 'space') ? refreshSpaceStatuses() : Promise.resolve(true),
      ])
      for (const target of lockedTargets) forgetUnlocked(target)
      if (lockedTargets.length > 0) await refreshAfterLock(lockedTargets)
      return result
    } catch {
      error.value = unavailableError
      return failureListResult(unavailableError)
    } finally {
      finishBeforeLock(preparation)
      isBusy.value = false
    }
  }

  async function requestAccess(target: ContentLockTarget, targetLabel: string) {
    cancelAccessRequest()
    error.value = null
    try {
      const result = await listRequiredContentLocks(target)
      if (result.error) {
        error.value = result.error
        return false
      }
      const requiredLocks = result.locks ?? []
      if (requiredLocks.length === 0) return true
      return await new Promise<boolean>((resolve) => {
        accessRequestResolver = resolve
        accessRequest.value = {
          target: { ...target },
          targetLabel,
          requiredLocks,
        }
      })
    } catch {
      error.value = unavailableError
      return false
    }
  }

  async function unlockAccess(passphrase: string) {
    const request = accessRequest.value
    const currentLock = request?.requiredLocks[0]
    if (!request || !currentLock) return failureResult(unavailableError)

    const requestedTarget = { ...request.target }

    const result = await unlock(targetForLock(currentLock), passphrase)
    if (result.error) return result
    try {
      const remaining = await listRequiredContentLocks(requestedTarget)
      if (remaining.error) {
        error.value = remaining.error
        return failureResult(remaining.error)
      }
      const requiredLocks = remaining.locks ?? []
      if (requiredLocks.length === 0) {
        finishAccessRequest(true)
      } else if (accessRequest.value?.target.type === requestedTarget.type && accessRequest.value.target.id === requestedTarget.id) {
        accessRequest.value = { ...request, requiredLocks }
      }
      return result
    } catch {
      error.value = unavailableError
      return failureResult(unavailableError)
    }
  }

  function finishAccessRequest(allowed: boolean) {
    const resolver = accessRequestResolver
    accessRequestResolver = null
    accessRequest.value = null
    resolver?.(allowed)
  }

  function cancelAccessRequest() {
    if (!accessRequest.value && !accessRequestResolver) return
    finishAccessRequest(false)
  }

  function setBeforeLock(handler: BeforeLock | null) {
    beforeLock = handler
  }

  function setAfterLock(handler: ((targets: ContentLockTarget[]) => Promise<void>) | null) {
    afterLock = handler
  }

  async function refreshAfterLock(targets: ContentLockTarget[]) {
    try {
      await afterLock?.(targets)
    } catch {
      // The keys have already been removed. A view refresh failure must not
      // report the successful mutation as failed or retain its input guard.
      error.value = {
        code: 'CONTENT_LOCK_AFTER_LOCK_REFRESH_FAILED',
        message: 'ロック後の表示を更新できませんでした。',
      }
    }
  }

  function clearLastChangedTarget() {
    lastChangedTarget.value = null
  }

  function clearError() {
    error.value = null
  }

  return {
    locks,
    statuses,
    statusLoading,
    statusErrors,
    spaceStatuses,
    lockByTarget,
    isLoading,
    isBusy,
    error,
    lastChangedTarget,
    unlockedAt,
    unlockVersion,
    accessRequest,
    refresh,
    refreshTarget,
    refreshSpaceStatuses,
    enable,
    unlock,
    lockNow,
    lockTargetsNow,
    changePassphrase,
    disable,
    requestAccess,
    unlockAccess,
    cancelAccessRequest,
    setBeforeLock,
    setAfterLock,
    clearLastChangedTarget,
    clearError,
  }
})
