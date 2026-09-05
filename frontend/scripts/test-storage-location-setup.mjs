import assert from 'node:assert/strict'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const setupPath = path.join(rootDir, 'src', 'components', 'StorageLocationSetupScreen.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const startupApiPath = path.join(rootDir, 'src', 'api', 'startup.ts')
const storageApiPath = path.join(rootDir, 'src', 'api', 'storageLocations.ts')
const stylePath = path.join(rootDir, 'src', 'style.css')
const storePath = path.join(rootDir, 'src', 'stores', 'useStorageLocationStore.ts')
const outDir = path.join(rootDir, '.tmp', 'storage-location-setup-test')
const storeOutFile = path.join(outDir, 'useStorageLocationStore.mjs')

const [setupSource, appSource, startupApiSource, storageApiSource, styleSource] = await Promise.all([
  readFile(setupPath, 'utf8'),
  readFile(appPath, 'utf8'),
  readFile(startupApiPath, 'utf8'),
  readFile(storageApiPath, 'utf8'),
  readFile(stylePath, 'utf8'),
])

await mkdir(outDir, { recursive: true })
const compiledStore = ts.transpileModule(
  (await readFile(storePath, 'utf8')).replace("from '../api/storageLocations'", "from './mock-storage-locations.mjs'"),
  { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } },
)
await writeFile(storeOutFile, compiledStore.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-storage-locations.mjs'), `
export const calls = { status: 0, choices: [], applies: 0, cancels: 0, retries: 0 }
let statusResult = { status: null, error: undefined }
let selectionResult = { status: null, canceled: false, error: undefined }
let mutationResult = { status: null, restartRequired: false, error: undefined }
function clone(value) { return value == null ? value : JSON.parse(JSON.stringify(value)) }
export function setStatusResult(value) { statusResult = value }
export function setSelectionResult(value) { selectionResult = value }
export function setMutationResult(value) { mutationResult = value }
export async function getStorageLocationStatus() {
  calls.status += 1
  return clone(statusResult)
}
export async function selectStorageLocation(kind) {
  calls.choices.push(kind)
  return clone(selectionResult)
}
export async function applyStorageLocations() {
  calls.applies += 1
  return clone(mutationResult)
}
export async function cancelPendingStorageLocationMigration() {
  calls.cancels += 1
  return clone(mutationResult)
}
export async function retryPendingStorageLocationMigration() {
  calls.retries += 1
  return clone(mutationResult)
}
`, 'utf8')

assert.match(setupSource, /mode\?: 'setup' \| 'recovery'/)
assert.match(setupSource, /const isRecovery = computed/)
assert.match(setupSource, /自動設定済み/)
assert.match(setupSource, /元の保存場所に戻す/)
assert.match(setupSource, /同じ移行を再試行する/)
assert.match(setupSource, /別の空フォルダで開始する場合、元データは自動移行されません/)
assert.match(setupSource, /インストールされているアプリ/)
assert.match(setupSource, />終了<\/button>/)
assert.match(setupSource, /overflow-y: auto/)
assert.match(setupSource, /var\(--bg-app\)/)
assert.match(setupSource, /var\(--text-primary\)/)
assert.match(appSource, /phase === 'storage-recovery'/)
assert.match(appSource, /startupStatus && !startupStatus\.ready/)
assert.match(appSource, /phase !== 'storage-recovery'/)
assert.match(startupApiSource, /'storage-recovery'/)
assert.match(startupApiSource, /OpenInstalledApps/)
assert.match(storageApiSource, /cancelPendingStorageLocationMigration/)
assert.match(storageApiSource, /retryPendingStorageLocationMigration/)
assert.match(styleSource, /--text-muted: #8b949e/)
assert.match(styleSource, /--text-muted: #475569/)

setActivePinia(createPinia())
const mock = await import(pathToFileURL(path.join(outDir, 'mock-storage-locations.mjs')).href)
const { useStorageLocationStore } = await import(pathToFileURL(storeOutFile).href)
const store = useStorageLocationStore()
const initialStatus = {
  dataRoot: 'C:/Notes',
  backupRoot: 'D:/Backups',
  recoveryRequired: true,
  pendingMigration: true,
  pendingRestart: true,
  pendingSelection: false,
  setupRequired: false,
  environmentOverride: false,
  dataRootChangeAllowed: true,
}
mock.setStatusResult({
  status: initialStatus,
  error: { code: 'STORAGE_LOCATION_VALIDATION_FAILED', message: '保留情報を確認できません。' },
})
assert.equal(await store.initialize(), false, 'a recoverable status may still report a marker error')
assert.deepEqual(store.status, initialStatus, 'status must remain available when initialization also returns an error')
assert.equal(store.error.code, 'STORAGE_LOCATION_VALIDATION_FAILED')

mock.setSelectionResult({
  status: { ...initialStatus, pendingDataRoot: 'C:/NewNotes', pendingSelection: true },
  canceled: false,
})
assert.equal(await store.choose('data'), true)
assert.equal(store.status.pendingDataRoot, 'C:/NewNotes')
assert.deepEqual(mock.calls.choices, ['data'])

let rollbackCalls = 0
store.setLifecycle(
  async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
  async () => {},
)
mock.setMutationResult({
  restartRequired: false,
  error: { code: 'STORAGE_LOCATION_VALIDATION_FAILED', message: '選択先が変わりました。' },
})
assert.equal(await store.apply(), false)
assert.equal(rollbackCalls, 1, 'failed apply must roll back the preparation')
assert.equal(store.error.code, 'STORAGE_LOCATION_VALIDATION_FAILED')

let restartCalls = 0
store.setLifecycle(
  async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
  async () => { restartCalls += 1 },
)
mock.setMutationResult({
  status: { ...initialStatus, pendingRestart: true },
  restartRequired: true,
})
assert.equal(await store.apply(), true)
assert.equal(restartCalls, 1)
assert.equal(mock.calls.applies, 2)

mock.setMutationResult({ restartRequired: true })
assert.equal(await store.cancelPendingMigration(), true)
assert.equal(await store.retryPendingMigration(), true)
assert.equal(mock.calls.cancels, 1)
assert.equal(mock.calls.retries, 1)

console.log('storage location setup tests passed')
