import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const storePath = path.join(rootDir, 'src', 'stores', 'useContentLockStore.ts')
const settingsPath = path.join(rootDir, 'src', 'components', 'SettingsModal.vue')
const storageSpacesPath = path.join(rootDir, 'src', 'components', 'StorageSpaceSettingsPanel.vue')
const notebookPath = path.join(rootDir, 'src', 'components', 'NotebookTreeItem.vue')
const noteListPath = path.join(rootDir, 'src', 'components', 'NoteList.vue')
const lockControlsPath = path.join(rootDir, 'src', 'components', 'ContentLockControls.vue')
const lockSettingsPath = path.join(rootDir, 'src', 'components', 'ContentLockSettingsPanel.vue')
const unlockDialogPath = path.join(rootDir, 'src', 'components', 'ContentUnlockDialog.vue')
const noteStorePath = path.join(rootDir, 'src', 'stores', 'useNoteStore.ts')
const appPath = path.join(rootDir, 'src', 'App.vue')
const beforeLockPath = path.join(rootDir, 'src', 'utils', 'contentLockBeforeLock.ts')
const outDir = path.join(rootDir, '.tmp', 'content-lock-store-test')
const outFile = path.join(outDir, 'useContentLockStore.mjs')
const beforeLockOut = path.join(outDir, 'contentLockBeforeLock.mjs')

await mkdir(outDir, { recursive: true })
const source = (await readFile(storePath, 'utf8'))
  .replace("from '../api/contentLocks'", "from './mock-content-locks.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(outFile, compiled.outputText, 'utf8')
const beforeLockSource = await readFile(beforeLockPath, 'utf8')
await writeFile(beforeLockOut, ts.transpileModule(beforeLockSource, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-content-locks.mjs'), `
export const calls = { list: 0, status: [], enable: [], unlock: [], lockNow: [], lockTargets: [], required: [], change: [], disable: [], spaces: 0 }
let locks = []
let requiredLocks = new Map()
let failingStatusKeys = new Set()
let deferStatusResponses = false
const pendingStatusResponses = []
let batchFailure = null
let batchThrow = false
export function setBatchFailure(value) { batchFailure = value }
export function setBatchThrow(value) { batchThrow = value }
let lockNowFailure = null
let deferLockNow = false
let throwLockNow = false
const pendingLockNow = []
let statuses = {
  'space:main': { protected: false, locked: false, explicitLock: false },
  'note:note-1': { protected: false, locked: false, explicitLock: false },
}
export async function listContentLocks() { calls.list += 1; return { locks: structuredClone(locks) } }
export async function listRequiredContentLocks(target) {
  calls.required.push(structuredClone(target))
  return { locks: structuredClone(requiredLocks.get(target.type + ':' + target.id) ?? []) }
}
export function setRequiredLocks(target, values) {
  requiredLocks.set(target.type + ':' + target.id, structuredClone(values))
}
export async function listStorageSpaceLockStatuses() {
  calls.spaces += 1
  return { statuses: [{ spaceId: 'main', protected: Boolean(statuses['space:main'].protected), locked: Boolean(statuses['space:main'].locked) }] }
}
export async function getContentLockStatus(target) {
  calls.status.push(target)
  const key = target.type + ':' + target.id
  if (failingStatusKeys.has(key)) throw new Error('status unavailable')
  if (deferStatusResponses) {
    return new Promise((resolve) => pendingStatusResponses.push({ key, resolve }))
  }
  return structuredClone(statuses[key] ?? { protected: false, locked: false, explicitLock: false })
}
export function setStatusFailure(target, enabled) {
  const key = target.type + ':' + target.id
  if (enabled) failingStatusKeys.add(key)
  else failingStatusKeys.delete(key)
}
export function setDeferredStatusResponses(enabled) { deferStatusResponses = enabled }
export function setLockNowFailure(error) { lockNowFailure = error ? structuredClone(error) : null }
export function setDeferredLockNow(enabled) { deferLockNow = enabled }
export function setLockNowThrow(enabled) { throwLockNow = enabled }
export function resolveLockNow() {
  const resolve = pendingLockNow.shift()
  if (!resolve) throw new Error('pending lockNow request was not found')
  resolve()
}
export function resolveStatusResponse(index, status) {
  const pending = pendingStatusResponses[index]
  if (!pending) throw new Error('pending status response was not found')
  pending.resolve(structuredClone(status))
}
export async function enableContentLock(input) {
  calls.enable.push(input)
  if (input.passphrase === 'requires-confirmation' && !input.deleteAIRecords) {
    return { removed: false, unlocked: false, restartRequired: false, error: { code: 'CONTENT_LOCK_AI_RECORDS_CONFIRMATION_REQUIRED', message: 'confirm', aiRecordCount: 2 } }
  }
  const key = input.targetType + ':' + input.targetId
  statuses[key] = { protected: true, locked: false, explicitLock: true, source: input.targetType }
  locks = [{ id: key, targetType: input.targetType, targetId: input.targetId, targetName: input.targetId, unlocked: true, createdAt: '', updatedAt: '' }]
  return { lock: locks[0], removed: false, unlocked: true, restartRequired: false }
}
export async function unlockContentLock(input) {
  calls.unlock.push(input)
  const key = input.targetType + ':' + input.targetId
  statuses[key] = { protected: true, locked: false, explicitLock: true, source: input.targetType }
  locks = locks.map((lock) => lock.targetType === input.targetType && lock.targetId === input.targetId ? { ...lock, unlocked: true } : lock)
  for (const [targetKey, values] of requiredLocks) {
    requiredLocks.set(targetKey, values.filter((lock) => lock.targetType !== input.targetType || lock.targetId !== input.targetId))
  }
  return { removed: false, unlocked: true, restartRequired: false }
}
export async function lockContentNow(target) {
  calls.lockNow.push(target)
  if (deferLockNow) await new Promise((resolve) => pendingLockNow.push(resolve))
  if (throwLockNow) throw new Error('lock API failed')
  if (lockNowFailure) return { removed: false, unlocked: false, restartRequired: false, error: structuredClone(lockNowFailure) }
  const key = target.type + ':' + target.id
  statuses[key] = { protected: true, locked: true, explicitLock: true, source: target.type }
  locks = locks.map((lock) => lock.targetType === target.type && lock.targetId === target.id ? { ...lock, unlocked: false } : lock)
  return { lock: locks.find((lock) => lock.targetType === target.type && lock.targetId === target.id), removed: false, unlocked: false, restartRequired: false }
}
export async function lockContentTargetsNow(targets) {
  calls.lockTargets.push(structuredClone(targets))
  if (batchThrow) throw new Error('batch API failed')
  if (batchFailure) return { locks: [], error: structuredClone(batchFailure) }
  const keys = new Set(targets.map((target) => target.type + ':' + target.id))
  const locked = locks
    .filter((lock) => keys.has(lock.targetType + ':' + lock.targetId) && lock.unlocked)
    .map((lock) => ({ ...lock, unlocked: false }))
  locks = locks.map((lock) => keys.has(lock.targetType + ':' + lock.targetId) ? { ...lock, unlocked: false } : lock)
  return { locks: structuredClone(locked) }
}
export async function changeContentLockPassphrase(input) { calls.change.push(input); return { removed: false, unlocked: true, restartRequired: false } }
export async function disableContentLock(input) {
  calls.disable.push(input)
  const key = input.targetType + ':' + input.targetId
  statuses[key] = { protected: false, locked: false, explicitLock: false }
  locks = []
  return { removed: true, unlocked: false, restartRequired: false }
}
`, 'utf8')

try {
  const [settingsSource, storageSpacesSource, notebookSource, noteListSource, lockControlsSource, lockSettingsSource, unlockDialogSource, noteStoreSource, appSource] = await Promise.all([
    readFile(settingsPath, 'utf8'),
    readFile(storageSpacesPath, 'utf8'),
    readFile(notebookPath, 'utf8'),
    readFile(noteListPath, 'utf8'),
    readFile(lockControlsPath, 'utf8'),
    readFile(lockSettingsPath, 'utf8'),
    readFile(unlockDialogPath, 'utf8'),
    readFile(noteStorePath, 'utf8'),
    readFile(appPath, 'utf8'),
  ])
  assert.match(settingsSource, /value="locks"/, 'Settings must expose a lock tab')
  assert.match(settingsSource, /ContentLockSettingsPanel/, 'Settings must render lock management')
  assert.match(settingsSource, /\.settings-panel\s*>\s*:deep\(section\s*>\s*h3\)/, 'Settings page headings must cross component scoped-style boundaries')
  assert.match(storageSpacesSource, /openSpaceLockDialog/, 'storage spaces must offer per-space lock management')
  assert.match(notebookSource, /編集・ロック設定/, 'notebook editing must include lock settings')
  assert.match(noteListSource, /編集・ロック設定/, 'note editing must include lock settings')
  assert.match(notebookSource, /<ContentLockControls[\s\S]*type: 'notebook'/, 'notebook editing must pass a notebook lock target')
  assert.match(noteListSource, /<ContentLockControls[\s\S]*type: 'note'/, 'note editing must pass a note lock target')
  const notebookLockControlsIndex = notebookSource.indexOf('<ContentLockControls')
  const notebookIconPickerIndex = notebookSource.indexOf('<NotebookIconPicker v-model="editIcon"')
  assert.ok(notebookLockControlsIndex < notebookIconPickerIndex, 'notebook lock settings must appear before icon selection')
  assert.match(notebookSource, /defer-save/, 'notebook editing must defer its lock mutation to the shared Save button')
  assert.match(noteListSource, /defer-save/, 'note editing must defer its lock mutation to the shared Save button')
  assert.doesNotMatch(notebookSource, /start-in-enable-mode/, 'notebook editing must not pre-open a separate lock form')
  assert.doesNotMatch(noteListSource, /start-in-enable-mode/, 'note editing must not pre-open a separate lock form')
  assert.match(notebookSource, /await lockControlsRef\.value\.save\(\)/, 'notebook shared Save must apply the lock change')
  const notebookLockSaveIndex = notebookSource.indexOf('await lockControlsRef.value.save()')
  const notebookDetailsUpdateIndex = notebookSource.indexOf('notebookStore.updateNotebookDetails(')
  assert.ok(notebookLockSaveIndex < notebookDetailsUpdateIndex, 'lock changes must be saved before notebook metadata')
  assert.match(noteListSource, /await lockControls\.save\(\)/, 'note shared Save must apply the lock change')
  assert.match(notebookSource, /<NotebookIconPicker v-model="editIcon"/, 'notebook icon selection must be part of the shared edit popover')
  assert.doesNotMatch(notebookSource, /isIconPickerOpen/, 'the displayed notebook icon must no longer open a standalone picker')
  assert.match(notebookSource, /:global\(\.notebook-edit-popover\)/, 'notebook popover must style its teleported root globally')
  assert.match(noteListSource, /:global\(\.note-edit-popover\)/, 'note popover must style its teleported root globally')
  assert.match(lockControlsSource, /deferSave/, 'lock controls must support a parent-owned save flow')
  assert.match(lockControlsSource, /role="switch"/, 'editor lock settings must use an accessible toggle')
  assert.match(lockControlsSource, /aria-label="ロック"/, 'editor lock settings must preserve an accessible switch name')
  assert.match(lockControlsSource, /content-lock-toggle-track/, 'editor lock settings must render a switch track')
  assert.doesNotMatch(lockControlsSource, /content-lock-toggle-label/, 'editor lock settings must not render on/off labels')
  assert.doesNotMatch(lockControlsSource, /トグルを有効にすると、パスフレーズ欄が表示されます。/, 'editor lock settings must not render redundant toggle guidance')
  assert.doesNotMatch(lockControlsSource, /パスフレーズを入力し、下の保存でロックを有効にします。/, 'editor lock settings must not render redundant enable guidance')
  assert.match(lockControlsSource, /targetStatusError/, 'lock controls must expose status load failures')
  assert.match(lockControlsSource, />再試行</, 'lock controls must let the user retry a failed status load')
  assert.match(lockControlsSource, /defineExpose\(\{\s*save: submit/, 'lock controls must expose its pending save operation')
  assert.match(lockControlsSource, /@click="startEnable"/, 'lock controls must expose the enable action')
  assert.match(lockControlsSource, /lockStore\.enable\(props\.target/, 'lock controls must call the lock API for its target')
  assert.match(notebookSource, /ロック設定の読み込みが完了していません/, 'notebook shared Save must not fail silently')
  assert.match(noteListSource, /ロック設定の読み込みが完了していません/, 'note shared Save must not fail silently')
  assert.match(appSource, /StorageSpaceUnlockScreen/, 'locked startup must render an unlock screen')
  assert.match(appSource, /setBeforeLock/, 'locking must flush pending drafts before key removal')
  assert.match(appSource, /createContentLockBeforeLock/, 'locking must flush editor input before drafts')
  assert.match(appSource, /flushEditorInput/, 'the app lock hook must include the active editor bridge')
  assert.match(appSource, /ContentUnlockDialog/, 'the normal workspace must host the shared unlock dialog')
  assert.match(appSource, /createContentLockAutoLock/, 'the app must schedule fixed-time content locks')
  assert.match(lockSettingsSource, /contentLockAutoLockMinutes/, 'lock settings must expose the auto-lock duration')
  assert.match(unlockDialogSource, /background-color: var\(--bg-editor\)/, 'unlock dialog must use an opaque surface')
  assert.match(noteStoreSource, /requestAccess\(\{ type: 'note', id \}/, 'note selection must request access before reading content')
  assert.match(notebookSource, /await notebookStore\.selectNotebook/, 'notebook selection must await the lock access gate')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-content-locks.mjs')).href)
  const { useContentLockStore } = await import(pathToFileURL(outFile).href)
  const { createContentLockBeforeLock } = await import(pathToFileURL(beforeLockOut).href)
  const store = useContentLockStore()
  const noteTarget = { type: 'note', id: 'note-1' }

  assert.equal(await store.refresh(), true)
  assert.equal(await store.refreshTarget(noteTarget).then((status) => status.protected), false)
  assert.equal(store.statusLoading['note:note-1'], false)
  assert.equal(store.statusErrors['note:note-1'], undefined)

  const unavailableTarget = { type: 'note', id: 'note-unavailable' }
  mock.setStatusFailure(unavailableTarget, true)
  assert.equal(await store.refreshTarget(unavailableTarget), null)
  assert.equal(store.statusLoading['note:note-unavailable'], false)
  assert.equal(store.statusErrors['note:note-unavailable'].code, 'CONTENT_LOCK_UNAVAILABLE')
  mock.setStatusFailure(unavailableTarget, false)
  assert.equal(await store.refreshTarget(unavailableTarget).then((status) => status.protected), false)
  assert.equal(store.statusErrors['note:note-unavailable'], undefined)

  const orderedTarget = { type: 'note', id: 'note-ordered' }
  mock.setDeferredStatusResponses(true)
  const olderStatusRequest = store.refreshTarget(orderedTarget)
  const newerStatusRequest = store.refreshTarget(orderedTarget)
  await Promise.resolve()
  mock.resolveStatusResponse(1, { protected: true, locked: true, explicitLock: true, source: 'note' })
  await newerStatusRequest
  mock.resolveStatusResponse(0, { protected: false, locked: false, explicitLock: false })
  await olderStatusRequest
  mock.setDeferredStatusResponses(false)
  assert.equal(store.statuses['note:note-ordered'].locked, true, 'an older response must not overwrite the latest status')
  assert.equal(store.statusLoading['note:note-ordered'], false)
  assert.equal(await store.refreshSpaceStatuses(), true)
  assert.equal(store.spaceStatuses.main.locked, false)

  const confirmation = await store.enable(noteTarget, 'requires-confirmation')
  assert.equal(confirmation.error.code, 'CONTENT_LOCK_AI_RECORDS_CONFIRMATION_REQUIRED')
  assert.equal(mock.calls.enable.length, 1)
  const enabled = await store.enable(noteTarget, 'correct horse battery staple', true)
  assert.equal(enabled.error, undefined)
  assert.equal(store.statuses['note:note-1'].protected, true)
  assert.equal(store.locks.length, 1)
  assert.deepEqual(store.lastChangedTarget, noteTarget)
  store.clearLastChangedTarget()
  assert.equal(store.lastChangedTarget, null)

  const lockOrder = []
  let flushes = 0
  let editorInputLocked = false
  let editorEditable = true
  const editorBridge = {
    flushEditorInput: () => {
      lockOrder.push('editor-input')
      return true
    },
    setContentLockPending: (pending) => {
      editorInputLocked = pending
      editorEditable = !pending
      lockOrder.push(pending ? 'input-lock' : 'input-unlock')
      return true
    },
  }
  let releaseDraftFlush
  let draftFlushStarted = false
  const draftFlush = new Promise((resolve) => { releaseDraftFlush = resolve })
  store.setBeforeLock(createContentLockBeforeLock(
    () => editorBridge,
    async () => {
      lockOrder.push('draft-flush')
      flushes += 1
      draftFlushStarted = true
      await draftFlush
      return true
    },
  ))
  mock.setDeferredLockNow(true)
  const lockPromise = store.lockNow(noteTarget)
  await Promise.resolve()
  assert.equal(draftFlushStarted, true, 'the product store must await the real draft flush before the lock API')
  assert.equal(editorInputLocked, true, 'the editor input guard must start before the draft flush')
  assert.equal(mock.calls.lockNow.length, 0, 'the lock API must wait for the held draft flush')
  releaseDraftFlush()
  await waitFor(() => mock.calls.lockNow.length === 1)
  assert.equal(mock.calls.lockNow.length, 1, 'the lock API must start after the draft flush')
  assert.equal(editorInputLocked, true, 'the editor input guard must cover the lock API wait')
  mock.resolveLockNow()
  const locked = await lockPromise
  assert.equal(locked.error, undefined)
  assert.equal(flushes, 1)
  assert.deepEqual(lockOrder.slice(0, 3), ['editor-input', 'input-lock', 'draft-flush'],
    'editor input must flush and lock before draft persistence')
  assert.equal(editorInputLocked, false, 'a completed request must release its own guard')
  assert.equal(editorInputLocked, false)
  assert.equal(editorEditable, true)

  let releaseConcurrentFlush
  let concurrentFlushes = 0
  const concurrentFlush = new Promise((resolve) => { releaseConcurrentFlush = resolve })
  store.setBeforeLock(createContentLockBeforeLock(
    () => editorBridge,
    async () => {
      concurrentFlushes += 1
      await concurrentFlush
      return true
    },
  ))
  const firstConcurrentLock = store.lockNow(noteTarget)
  await waitFor(() => concurrentFlushes === 1)
  const secondConcurrentLock = await store.lockNow(noteTarget)
  assert.equal(secondConcurrentLock.error.code, 'CONTENT_LOCK_UNAVAILABLE', 'a concurrent lock must be rejected before it can touch the editor guard')
  assert.equal(concurrentFlushes, 1)
  assert.equal(editorInputLocked, true, 'a rejected concurrent lock must not release the active input guard')
  releaseConcurrentFlush()
  await waitFor(() => mock.calls.lockNow.length === 2)
  mock.resolveLockNow()
  const firstConcurrentResult = await firstConcurrentLock
  assert.equal(firstConcurrentResult.error, undefined)
  assert.equal(editorInputLocked, false)

  mock.setDeferredLockNow(false)
  store.setBeforeLock(createContentLockBeforeLock(
    () => editorBridge,
    async () => false,
  ))
  const failedSave = await store.lockNow(noteTarget)
  assert.equal(failedSave.error.code, 'CONTENT_LOCK_UNAVAILABLE')
  assert.equal(mock.calls.lockNow.length, 2, 'a failed draft flush must not call the lock API')
  assert.equal(editorInputLocked, false, 'a failed draft flush must restore editor input')
  assert.equal(editorEditable, true)

  store.setBeforeLock(createContentLockBeforeLock(
    () => editorBridge,
    async () => { throw new Error('draft flush failed') },
  ))
  const thrownFlush = await store.lockTargetsNow([noteTarget])
  assert.equal(thrownFlush.error.code, 'CONTENT_LOCK_UNAVAILABLE', 'a draft flush exception must preserve the unavailable result')
  assert.equal(mock.calls.lockTargets.length, 0, 'a draft flush exception must not call the batch lock API')
  assert.equal(editorInputLocked, false, 'a draft flush exception must restore editor input')
  assert.equal(editorEditable, true)

  mock.setLockNowFailure({ code: 'CONTENT_LOCK_FAILED', message: 'lock failed' })
  store.setBeforeLock(createContentLockBeforeLock(
    () => editorBridge,
    async () => true,
  ))
  const failedApi = await store.lockNow(noteTarget)
  assert.equal(failedApi.error.code, 'CONTENT_LOCK_FAILED')
  assert.equal(editorInputLocked, false, 'a failed lock API must restore editor input')
  assert.equal(editorEditable, true)
  mock.setLockNowFailure(null)

  mock.setLockNowThrow(true)
  const thrownApi = await store.lockNow(noteTarget)
  assert.equal(thrownApi.error.code, 'CONTENT_LOCK_UNAVAILABLE')
  assert.equal(editorInputLocked, false, 'an exception from the lock API must restore editor input')
  assert.equal(editorEditable, true)
  mock.setLockNowThrow(false)

  await store.unlock(noteTarget, 'correct horse battery staple')
  const batchLocked = await store.lockTargetsNow([noteTarget, noteTarget])
  assert.equal(batchLocked.error, undefined)
  assert.equal(mock.calls.lockTargets.length, 1)
  assert.deepEqual(mock.calls.lockTargets[0], [noteTarget], 'auto-lock must de-duplicate due targets')
  assert.equal(editorInputLocked, false)

  let failedFlushes = 0
  store.setBeforeLock(async () => { failedFlushes += 1; return false })
  const deferred = await store.lockTargetsNow([noteTarget])
  assert.equal(deferred.error.code, 'CONTENT_LOCK_SAVE_FAILED')
  assert.equal(failedFlushes, 1)
  assert.equal(mock.calls.lockTargets.length, 1, 'failed draft flush must keep the session key available')

  const protectedTarget = { type: 'note', id: 'note-protected' }
  const parentLock = { id: 'parent-lock', targetType: 'notebook', targetId: 'notebook-parent', targetName: '親ノートブック', unlocked: false, createdAt: '', updatedAt: '' }
  const noteLock = { id: 'note-lock', targetType: 'note', targetId: 'note-protected', targetName: '保護ノート', unlocked: false, createdAt: '', updatedAt: '' }
  mock.setRequiredLocks(protectedTarget, [parentLock, noteLock])
  const access = store.requestAccess(protectedTarget, '保護ノート')
  await Promise.resolve()
  assert.equal(store.isBusy, false, 'opening the access dialog must not leave the lock store busy')
  assert.equal(store.accessRequest.requiredLocks.length, 2)
  const firstUnlock = await store.unlockAccess('parent passphrase')
  assert.equal(firstUnlock.error, undefined, `unlock error after ${mock.calls.unlock.length} RPC call(s)`)
  assert.equal(store.accessRequest.requiredLocks.length, 1, 'each successful passphrase must re-check remaining locks')
  assert.equal(store.accessRequest.requiredLocks[0].targetType, 'note')
  const secondUnlock = await store.unlockAccess('note passphrase')
  assert.equal(secondUnlock.error, undefined)
  assert.equal(await access, true, 'the original selection proceeds only after every required lock is unlocked')
  assert.equal(store.accessRequest, null)

  mock.setRequiredLocks(protectedTarget, [noteLock])
  const cancelledAccess = store.requestAccess(protectedTarget, '保護ノート')
  await Promise.resolve()
  store.cancelAccessRequest()
  assert.equal(await cancelledAccess, false, 'cancelling an unlock dialog must preserve the original selection')

  const disabled = await store.disable(noteTarget, 'correct horse battery staple')
  assert.equal(disabled.removed, true)
  assert.equal(store.statuses['note:note-1'].protected, false)
  assert.equal(mock.calls.disable.length, 1)

  // Execute the product App refresh function, not a copy of its guard logic.
  const refreshSource = appSource.slice(
    appSource.indexOf('async function handleLockedTargets('),
    appSource.indexOf('function handleOpenNoteImport('),
  )
  const refreshJS = ts.transpileModule(refreshSource, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  }).outputText
  assert.match(appSource, /setAfterLock\(handleLockedTargets\)/)
  assert.doesNotMatch(appSource, /lastLockedTargets?|setContentLockPending\(false\)/,
    'App must not keep a second asynchronous notification/guard-release path')
  assert.match(appSource, /onBeforeUnmount\(\(\) => \{\s*contentLockStore.setBeforeLock\(null\)\s*contentLockStore.setAfterLock\(null\)/)

  for (const firstKind of ['single', 'batch']) {
    for (const refreshFails of [false, true]) {
      const raceStore = useContentLockStore(createPinia())
      const b = { type: 'note', id: 'race-b' }
      const a = { type: 'note', id: 'race-a' }
      await raceStore.enable(b, 'test passphrase', true)
      let guarded = false
      let saveCount = 0
      let releaseRefresh
      let refreshStarted = false
      const refreshGate = new Promise((resolve) => { releaseRefresh = resolve })
      const warnings = []
      const raceEditor = {
        flushEditorInput: () => true,
        setContentLockPending: (pending) => { guarded = pending; return true },
      }
      const appRefresh = new Function(
        'contentLockStore', 'noteStore', 'startupStatus', 'getStartupStatus',
        'appStore', 'searchStore', 'notificationStore',
        refreshJS + '\nreturn handleLockedTargets',
      )(raceStore, {
        refreshActiveNoteLockStatus: async () => {},
        fetchNotes: async () => {
          refreshStarted = true
          await refreshGate
          if (refreshFails) throw new Error('view unavailable')
        },
      }, { value: null }, async () => ({ ready: true }),
      { sidebarSection: 'notes' }, { isActive: false }, { notify: (...args) => warnings.push(args) })
      raceStore.setBeforeLock(createContentLockBeforeLock(() => raceEditor, async () => {
        saveCount += 1
        return true
      }))
      raceStore.setAfterLock(appRefresh)
      const first = firstKind === 'single' ? raceStore.lockNow(b) : raceStore.lockTargetsNow([b, b])
      await waitFor(() => refreshStarted)
      assert.equal(raceStore.isBusy, true, 'API success must retain busy during App refresh')
      assert.equal(guarded, true)
      const callsBefore = mock.calls.lockNow.length + mock.calls.lockTargets.length
      assert.ok((await raceStore.lockNow(a)).error)
      assert.ok((await raceStore.lockTargetsNow([a])).error)
      assert.equal(saveCount, 1, 'later locks must not acquire a guard or begin saving')
      assert.equal(mock.calls.lockNow.length + mock.calls.lockTargets.length, callsBefore)
      assert.equal(guarded, true, 'duplicate rejection must preserve the first guard')
      if (firstKind === 'batch' && refreshFails) {
        raceStore.setBeforeLock(null)
        raceStore.setAfterLock(null)
        assert.equal(guarded, true, 'unregistering during refresh cannot release an active request')
      }
      releaseRefresh()
      assert.equal((await first).error, undefined, 'view failure cannot undo successful key removal')
      assert.equal(warnings.length, refreshFails ? 1 : 0)
      assert.equal(guarded, false)
      assert.equal(raceStore.isBusy, false)

      // After the old refresh settles, every subsequent failure unwinds its own guard.
      for (const failure of ['save', 'save-throw', 'api', 'api-throw', 'empty', 'success']) {
        raceStore.setBeforeLock(createContentLockBeforeLock(() => raceEditor, async () => {
          if (failure === 'save-throw') throw new Error('save unavailable')
          return failure !== 'save'
        }))
        mock.setBatchFailure(failure === 'api' ? { code: 'CONTENT_LOCK_FAILED', message: 'failed' } : null)
        mock.setBatchThrow(failure === 'api-throw')
        if (failure === 'success') await raceStore.enable(a, 'test passphrase', true)
        const result = await raceStore.lockTargetsNow([a])
        assert.equal(Boolean(result.error), !['empty', 'success'].includes(failure), failure)
        if (failure === 'empty') assert.deepEqual(result.locks, [])
        assert.equal(guarded, false, failure + ' must restore editing')
        assert.equal(raceStore.isBusy, false, failure + ' must restore busy')
      }
      mock.setBatchFailure(null)
      mock.setBatchThrow(false)
      raceStore.setAfterLock(async () => { throw new Error('unexpected refresh exception') })
      assert.equal((await raceStore.lockNow(a)).error, undefined)
      assert.equal(raceStore.error.code, 'CONTENT_LOCK_AFTER_LOCK_REFRESH_FAILED')
      assert.equal(guarded, false, 'an unexpected callback exception must release input')
      assert.equal(raceStore.isBusy, false)
    }
  }

  // Unmount removes future callbacks, but the captured editor is still released
  // when an already-running API settles. A replacement editor is never touched.
  const detachedStore = useContentLockStore(createPinia())
  let originalGuarded = false
  let replacementTouches = 0
  let currentEditor = {
    flushEditorInput: () => true,
    setContentLockPending: (pending) => { originalGuarded = pending; return true },
  }
  detachedStore.setBeforeLock(createContentLockBeforeLock(() => currentEditor, async () => true))
  detachedStore.setAfterLock(async () => { throw new Error('must have been unregistered') })
  mock.setDeferredLockNow(true)
  const detachedCalls = mock.calls.lockNow.length
  const detachedLock = detachedStore.lockNow(noteTarget)
  await waitFor(() => mock.calls.lockNow.length > detachedCalls)
  detachedStore.setBeforeLock(null)
  detachedStore.setAfterLock(null)
  currentEditor = { flushEditorInput: () => true, setContentLockPending: () => { replacementTouches += 1; return true } }
  mock.resolveLockNow()
  assert.equal((await detachedLock).error, undefined)
  assert.equal(originalGuarded, false)
  assert.equal(replacementTouches, 0)
  assert.equal(detachedStore.isBusy, false)
  mock.setDeferredLockNow(false)

  console.log('content lock store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function waitFor(predicate) {
  const deadline = Date.now() + 1000
  while (Date.now() < deadline) {
    if (predicate()) return
    await new Promise((resolve) => setTimeout(resolve, 0))
  }
  throw new Error('condition was not reached')
}
