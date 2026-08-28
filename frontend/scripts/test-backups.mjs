import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const storePath = path.join(rootDir, 'src', 'stores', 'useBackupStore.ts')
const lifecyclePath = path.join(rootDir, 'src', 'services', 'backupLifecycle.ts')
const panelPath = path.join(rootDir, 'src', 'components', 'BackupSettingsPanel.vue')
const settingsPath = path.join(rootDir, 'src', 'components', 'SettingsModal.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const outDir = path.join(rootDir, '.tmp', 'backups-test')
const storeOutFile = path.join(outDir, 'useBackupStore.mjs')
const lifecycleOutFile = path.join(outDir, 'backupLifecycle.mjs')

await mkdir(outDir, { recursive: true })
const storeSource = (await readFile(storePath, 'utf8'))
  .replace("from '../api/backups'", "from './mock-backups.mjs'")
  .replace("from '../services/backupLifecycle'", "from './backupLifecycle.mjs'")
  .replace("from './useNotificationStore'", "from './mock-notification.mjs'")
const compiledStore = ts.transpileModule(storeSource, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
const compiledLifecycle = ts.transpileModule(await readFile(lifecyclePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(storeOutFile, compiledStore.outputText, 'utf8')
await writeFile(lifecycleOutFile, compiledLifecycle.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-notification.mjs'), `
export const notifications = []
export function useNotificationStore() {
  return { notify(message, options) { notifications.push({ message, ...options }) } }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-backups.mjs'), `
export const calls = { status: 0, list: 0, create: 0, preview: [], execute: [], cancel: 0 }
const backup = {
  id: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', kind: 'automatic', createdAt: '2026-08-28T00:00:00Z',
  sizeBytes: 100, fileCount: 2, restorable: true,
}
let automaticDue = true
let nextExecuteResult
export function setAutomaticDue(value) { automaticDue = value }
export function setNextExecuteResult(value) { nextExecuteResult = value }
function clone(value) { return JSON.parse(JSON.stringify(value)) }
export async function getBackupStatus() {
  calls.status += 1
  return {
    automaticEnabled: true, automaticDue, lastAutomaticAt: automaticDue ? undefined : backup.createdAt,
    backupCount: 1, pendingRestore: false,
  }
}
export async function listBackups() { calls.list += 1; return { backups: [clone(backup)] } }
export async function createAutomaticBackup() {
  calls.create += 1
  automaticDue = false
  return { created: true, skipped: false, backup: clone(backup) }
}
export async function previewBackupRestore(id) {
  calls.preview.push(id)
  return {
    preview: {
      token: 'restore-token', backupId: id, createdAt: backup.createdAt,
      sizeBytes: backup.sizeBytes, fileCount: backup.fileCount, message: 'replace current data',
    },
  }
}
export async function executeBackupRestore(token) {
  calls.execute.push(token)
  const result = nextExecuteResult ?? { backupId: backup.id, restartRequired: true, canceled: false }
  nextExecuteResult = undefined
  return clone(result)
}
export async function cancelBackupRestore() { calls.cancel += 1; return { canceled: true, restartRequired: false } }
`, 'utf8')

const localStorageValues = new Map()
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
    getItem(key) { return localStorageValues.has(key) ? localStorageValues.get(key) : null },
    setItem(key, value) { localStorageValues.set(key, String(value)) },
    removeItem(key) { localStorageValues.delete(key) },
  },
})

try {
  const [panelSource, settingsSource, appSource] = await Promise.all([
    readFile(panelPath, 'utf8'),
    readFile(settingsPath, 'utf8'),
    readFile(appPath, 'utf8'),
  ])
  assert.match(settingsSource, /value="backups"/, 'Settings must contain the backup tab')
  assert.match(settingsSource, /BackupSettingsPanel/, 'Settings must render the backup panel')
  assert.match(panelSource, /自動バックアップを有効にする/, 'the panel must expose the automatic-backup switch')
  assert.match(panelSource, /保存済みバックアップ/, 'the panel must expose backup history')
  assert.match(panelSource, /復元/, 'the panel must expose restore')
  assert.doesNotMatch(panelSource, /フォルダを選択|保存先を選択/, 'the panel must not expose external backup paths')
  assert.match(appSource, /prepareBackupOperation/, 'App must own backup preparation')
  assert.match(appSource, /isStorageSpaceBusy/, 'backup must coordinate with storage-space switching')
  assert.match(appSource, /backupRestoreSafetyBackupId/, 'App must report the restore safety backup')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-backups.mjs')).href)
  const { useBackupStore } = await import(pathToFileURL(storeOutFile).href)
  const { prepareBackupOperation } = await import(pathToFileURL(lifecycleOutFile).href)
  const store = useBackupStore()

  assert.equal(store.automaticEnabled, true)
  assert.equal(await store.initialize(), true)
  assert.equal(store.backups.length, 1)
  assert.equal(store.backups[0].restorable, true)

  const lifecycleEvents = []
  let storageSpaceBusy = true
  const dependencies = {
    isStorageSpaceBusy: () => storageSpaceBusy,
    isSyncBusy: () => false,
    isAIBusy: () => false,
    isImportBusy: () => false,
    isExportBusy: () => false,
    isContentLockBusy: () => false,
    suspendSync: () => { lifecycleEvents.push('suspend'); return true },
    resumeSync: () => { lifecycleEvents.push('resume') },
    flushAllDirtyNotes: async () => { lifecycleEvents.push('flush'); return true },
    notify: (_message, code) => { lifecycleEvents.push(code) },
  }
  assert.equal((await prepareBackupOperation(dependencies)).ready, false)
  assert.deepEqual(lifecycleEvents, ['BACKUP_STORAGE_SPACE_BUSY'])
  storageSpaceBusy = false
  const prepared = await prepareBackupOperation(dependencies)
  assert.equal(prepared.ready, true)
  assert.deepEqual(lifecycleEvents, ['BACKUP_STORAGE_SPACE_BUSY', 'suspend', 'flush'])
  await prepared.rollback()
  assert.deepEqual(lifecycleEvents, ['BACKUP_STORAGE_SPACE_BUSY', 'suspend', 'flush', 'resume'])

  let preparationCalls = 0
  let rollbackCalls = 0
  store.setLifecycle(
    async () => {
      preparationCalls += 1
      return { ready: true, rollback: () => { rollbackCalls += 1 } }
    },
    async () => {},
  )
  assert.equal((await store.runAutomaticIfDue()).created, true)
  assert.equal(mock.calls.create, 1)
  assert.equal(preparationCalls, 1)
  assert.equal(rollbackCalls, 1)
  assert.equal(store.status.automaticDue, false)

  const preview = await store.previewRestore(store.backups[0].id)
  assert.equal(preview.token, 'restore-token')
  let restartCalls = 0
  store.setLifecycle(
    async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
    async () => { restartCalls += 1 },
  )
  assert.equal(await store.restore(preview), true)
  assert.equal(restartCalls, 1)
  assert.equal(mock.calls.execute.length, 1)

  const failedPreview = await store.previewRestore(store.backups[0].id)
  mock.setNextExecuteResult({ backupId: store.backups[0].id, restartRequired: true, canceled: false })
  store.setLifecycle(
    async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
    async () => { throw new Error('automatic restart is unavailable in Wails development mode') },
  )
  assert.equal(await store.restore(failedPreview), false)
  assert.equal(mock.calls.cancel, 1)
  assert.equal(store.error.code, 'BACKUP_RESTART_FAILED')
  assert.match(store.error.message, /wails dev/)
  assert.equal(rollbackCalls, 2)

  mock.setAutomaticDue(true)
  await store.refreshStatus()
  store.automaticEnabled = false
  assert.equal(await store.runAutomaticIfDue(), null)
  assert.equal(mock.calls.create, 1, 'disabled automatic backup must not run')
  store.dispose()
  console.log('backup tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
