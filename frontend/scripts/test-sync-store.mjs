import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useSyncStore.ts')
const outDir = path.join(rootDir, '.tmp', 'sync-store-test')
const outFile = path.join(outDir, 'useSyncStore.mjs')

await mkdir(outDir, { recursive: true })
const source = (await readFile(sourcePath, 'utf8'))
  .replace("from '../api/sync'", "from './mock-sync.mjs'")
  .replace("from './useNotificationStore'", "from './mock-notifications.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(outFile, compiled.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-notifications.mjs'), `
export const notices = []
export function useNotificationStore() {
  return { notify(message, options) { notices.push({ message, options }) } }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-sync.mjs'), `
export const calls = { configure: [], test: [], sync: 0, syncInputs: [] }
let current = { status: 'disabled', outboxCount: 0, conflictCount: 0 }
let syncDeferred
let nextTestError
let nextSyncError
const syncResults = []
export function configureState(next) { current = next }
export function deferSync() { syncDeferred = deferred(); return syncDeferred }
export function queueSyncResult(result) { syncResults.push(result) }
export function rejectNextConfigurationCheck(error) { nextTestError = error }
export function rejectNextSync(error) { nextSyncError = error }
function deferred() { let resolve; const promise = new Promise((r) => { resolve = r }); return { promise, resolve } }
export async function getSyncStatus() { return current }
export async function listSyncConflicts() { return [] }
export async function testSyncConfiguration(input) {
  calls.test.push(input)
  if (nextTestError) {
    const error = nextTestError
    nextTestError = undefined
    throw error
  }
  return { success: true, remoteInitialized: false, message: 'ok' }
}
export async function configureSync(input) {
  calls.configure.push(input)
  current = {
    status: 'idle', outboxCount: 1, conflictCount: 0,
    connection: {
      webDAVURL: input.webDAVURL, username: input.username, status: 'idle', hasLastSync: false,
      syncIntervalSeconds: input.syncIntervalSeconds, allowInsecureHTTP: input.allowInsecureHTTP,
      customTLSCertificates: input.customTLSCertificates, ignoreTLSErrors: input.ignoreTLSErrors,
      proxyEnabled: input.proxyEnabled, proxyURL: input.proxyURL,
      proxyTimeoutSeconds: input.proxyTimeoutSeconds, failSafe: input.failSafe,
    },
  }
  return current
}
export async function syncNow(input) {
  calls.sync += 1
  calls.syncInputs.push(input)
  if (nextSyncError) {
    const error = nextSyncError
    nextSyncError = undefined
    throw error
  }
  if (syncDeferred) { const waiting = syncDeferred; syncDeferred = undefined; await waiting.promise }
  if (syncResults.length > 0) {
    const result = syncResults.shift()
    current = {
      ...current, status: result.status, outboxCount: result.remaining,
      connection: { ...current.connection, status: result.status },
    }
    return result
  }
  current = { ...current, status: 'synced', outboxCount: 0, connection: { ...current.connection, status: 'synced' } }
  return { status: 'synced', uploaded: 1, downloaded: 0, conflicts: 0, remaining: 0 }
}
export async function resolveSyncConflict() {}
export async function disconnectSync() { current = { status: 'disabled', outboxCount: 0, conflictCount: 0 } }
export async function prepareSyncRecovery(action) { return { token: 'token', action, localItems: 1, remoteItems: 1, message: 'preview' } }
export async function executeSyncRecovery() { return { action: 'reupload-local', restartRequired: false, message: 'done' } }
export async function quitForSyncRecovery() {}
`, 'utf8')

const realSetInterval = globalThis.setInterval
let pollingDelay = 0
globalThis.setInterval = ((callback, delay) => {
  pollingDelay = Number(delay)
  return { callback }
})

try {
  setActivePinia(createPinia())
  const { configureState, deferSync, queueSyncResult, rejectNextConfigurationCheck, rejectNextSync, calls } = await import(pathToFileURL(path.join(outDir, 'mock-sync.mjs')).href)
  const { useSyncStore } = await import(pathToFileURL(outFile).href)
  const store = useSyncStore()

  store.draft.webDAVURL = 'http://dav.example.test/atlasnote'
  store.draft.username = 'alice'
  store.draft.password = 'secret'
  store.draft.syncIntervalSeconds = 600
  store.draft.allowInsecureHTTP = true
  store.draft.failSafe = true
  store.draft.setupMode = 'initialize'

  rejectNextConfigurationCheck(new Error('webdav PROPFIND request returned HTTP 401 (Basic credentials were sent; server requested Basic authentication)'))
  await assert.rejects(store.checkConfiguration())
  assert.equal(
    store.configurationTestError,
    'Basic認証情報は送信済みですが、同期先がPROPFINDを401で拒否しました。WebDAVサーバー側のユーザー権限または認証プロキシ設定を確認してください。',
    'a PROPFIND 401 after Basic credentials were sent must not be presented as a password-only error',
  )
  rejectNextConfigurationCheck(new Error('webdav PROPFIND request returned HTTP 401 (Digest credentials were sent; server requested Digest authentication)'))
  await assert.rejects(store.checkConfiguration())
  assert.equal(
    store.configurationTestError,
    'Digest認証で拒否されました。ユーザー名とパスワード、またはWebDAVサーバー側のアクセス権を確認してください。',
    'a rejected Digest authentication attempt must not expose credentials',
  )

  await store.checkConfiguration()
  assert.equal(calls.test.length, 3, 'configuration check must use its read-only API')
  assert.equal(calls.configure.length, 0, 'configuration check must not persist settings')
  assert.equal(store.draft.password, 'secret', 'checking must keep the password for Apply')

  await store.configure()
  assert.equal(calls.configure[0].password, 'secret')
  assert.equal(calls.configure[0].allowInsecureHTTP, true)
  assert.equal(calls.configure[0].setupMode, 'initialize')
  assert.equal(store.draft.password, '', 'password must be cleared after successful Apply')
  assert.equal(calls.sync, 1, 'initialization must run after settings are persisted')
  assert.equal(pollingDelay, 600_000, 'polling must use the selected interval')
  assert.equal(store.statusLabel, '同期済み', 'synced is a stable runtime state, not idle')

  store.draft.setupMode = 'import'
  await store.checkConfiguration()
  assert.equal(calls.test.at(-1).setupMode, 'import', 'an explicit setup retry must not be replaced with update mode')
  store.draft.setupMode = 'update'

  store.draft.webDAVURL = 'https://changed.example.test'
  assert.equal(store.targetChanged, true)
  store.discardDraft()
  assert.equal(store.draft.webDAVURL, 'http://dav.example.test/atlasnote', 'Back must discard unsaved target changes')

  configureState({
    status: 'idle', outboxCount: 1, conflictCount: 0,
    connection: { ...store.statusResult.connection, status: 'idle', syncIntervalSeconds: 0 },
  })
  await store.refresh()
  const pending = deferSync()
  const first = store.runSync()
  const second = store.runSync()
  assert.equal(await second, null, 'a second sync must not start while the first is running')
  pending.resolve()
  await first
  assert.equal(calls.sync, 2)

  const realSetTimeout = globalThis.setTimeout
  const realClearTimeout = globalThis.clearTimeout
  const retryTimers = []
  globalThis.setTimeout = ((callback, delay) => {
    const timer = { callback, delay: Number(delay), cleared: false }
    retryTimers.push(timer)
    return timer
  })
  globalThis.clearTimeout = ((timer) => { timer.cleared = true })
  try {
    configureState({
      status: 'pending', outboxCount: 1, conflictCount: 0,
      connection: { ...store.statusResult.connection, status: 'pending', syncIntervalSeconds: 0 },
    })
    await store.refresh()
    queueSyncResult({ status: 'pending', uploaded: 0, downloaded: 0, conflicts: 0, remaining: 1 })
    queueSyncResult({ status: 'pending', uploaded: 0, downloaded: 0, conflicts: 0, remaining: 1 })
    await store.runSync({ importRemote: true })
    assert.equal(retryTimers[0].delay, 15_000, 'first retry must use the 15 second backoff')
    retryTimers[0].callback()
    for (let attempt = 0; attempt < 10 && retryTimers.length < 2; attempt += 1) {
      await new Promise((resolve) => realSetTimeout(resolve, 0))
    }
    assert.equal(calls.sync, 4, 'the scheduled retry must start another sync')
    assert.equal(calls.syncInputs.at(-1).importRemote, true, 'a retry must preserve the explicit import mode')
    assert.equal(retryTimers[1].delay, 60_000, 'second retry must use the 60 second backoff')

    await store.disconnect()
    assert.equal(retryTimers[1].cleared, true, 'disconnect must cancel a scheduled retry')
  } finally {
    globalThis.setTimeout = realSetTimeout
    globalThis.clearTimeout = realClearTimeout
  }

  rejectNextSync(new Error('webdav MKCOL request returned HTTP 403'))
  await assert.rejects(store.runSync({ forceRetry: true }))
  assert.equal(
    store.syncError,
    '同期先が同期用フォルダの作成（MKCOL）を拒否しました。ユーザーの書込み権限とWebDAVサーバー設定を確認してください。',
    'an initialization write failure must expose a safe, actionable explanation',
  )

  assert.equal(store.draft.allowInsecureHTTP, false, 'HTTP opt-in must reset after disconnect')
  assert.equal(store.draft.failSafe, true, 'fail-safe must default to enabled')
  store.dispose()
  console.log('sync store tests passed')
} finally {
  globalThis.setInterval = realSetInterval
  await rm(outDir, { recursive: true, force: true })
}
