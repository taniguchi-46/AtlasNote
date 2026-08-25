import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const storePath = path.join(rootDir, 'src', 'stores', 'useStorageSpaceStore.ts')
const guardPath = path.join(rootDir, 'src', 'services', 'storageSpaceSwitch.ts')
const panelPath = path.join(rootDir, 'src', 'components', 'StorageSpaceSettingsPanel.vue')
const settingsPath = path.join(rootDir, 'src', 'components', 'SettingsModal.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const topBarPath = path.join(rootDir, 'src', 'components', 'AppTopBar.vue')
const outDir = path.join(rootDir, '.tmp', 'storage-spaces-test')
const storeOutFile = path.join(outDir, 'useStorageSpaceStore.mjs')
const guardOutFile = path.join(outDir, 'storageSpaceSwitch.mjs')

await mkdir(outDir, { recursive: true })
const storeSource = (await readFile(storePath, 'utf8'))
  .replace("from '../api/storageSpaces'", "from './mock-storage-spaces.mjs'")
const compiledStore = ts.transpileModule(storeSource, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
const compiledGuard = ts.transpileModule(await readFile(guardPath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(storeOutFile, compiledStore.outputText, 'utf8')
await writeFile(guardOutFile, compiledGuard.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-storage-spaces.mjs'), `
export const calls = { list: 0, create: [], select: [], order: [] }
let spaces = [
  { id: 'main-id', name: 'メイン', active: true, legacy: true, createdAt: '2026-08-25T00:00:00Z' },
]
let activeSpaceId = 'main-id'
let nextSelectError
function clone(value) { return JSON.parse(JSON.stringify(value)) }
export function failNextSelection(error) { nextSelectError = error }
export function activate(id) {
  activeSpaceId = id
  spaces = spaces.map((space) => ({ ...space, active: space.id === id }))
}
export async function listStorageSpaces() {
  calls.list += 1
  return { spaces: clone(spaces), activeSpaceId }
}
export async function createStorageSpace(name) {
  calls.create.push(name)
  if (spaces.some((space) => space.name === name)) {
    return { restartRequired: false, error: { code: 'STORAGE_SPACE_NAME_CONFLICT', message: '同じ名前の保存空間がすでにあります。' } }
  }
  const space = { id: 'work-id', name, active: false, legacy: false, createdAt: '2026-08-25T01:00:00Z' }
  spaces.push(space)
  return { space: clone(space), activeSpaceId, restartRequired: false }
}
export async function selectStorageSpace(id) {
  calls.select.push(id)
  calls.order.push('select')
  if (nextSelectError) {
    const error = nextSelectError
    nextSelectError = undefined
    return { restartRequired: false, error }
  }
  activeSpaceId = id
  spaces = spaces.map((space) => ({ ...space, active: space.id === id }))
  return { space: clone(spaces.find((space) => space.id === id)), activeSpaceId, restartRequired: true }
}
`, 'utf8')

try {
  const [panelSource, settingsSource, appSource, topBarSource] = await Promise.all([
    readFile(panelPath, 'utf8'),
    readFile(settingsPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(topBarPath, 'utf8'),
  ])
  assert.match(settingsSource, /value="storage-spaces"/, 'Settings must contain the storage-space tab')
  assert.match(settingsSource, /StorageSpaceSettingsPanel/, 'Settings must render the storage-space panel')
  assert.match(panelSource, /保存空間の一覧/, 'the panel must expose a storage-space list')
  assert.match(panelSource, /新しい保存空間/, 'the panel must expose internal space creation')
  assert.match(panelSource, /保存して再起動/, 'switching must clearly state the automatic restart flow')
  assert.match(panelSource, /aria-current/, 'the active storage space must be exposed accessibly')
  assert.doesNotMatch(panelSource, /フォルダを選択|ディレクトリを選択|保存空間を削除|名前を変更/, 'external folders, deletion, and rename are out of scope')
  assert.doesNotMatch(topBarSource, /保存空間|StorageSpace/, 'the top bar must not select storage spaces')
  assert.match(appSource, /prepareStorageSpaceSwitch/, 'App must own switch preparation')
  assert.match(appSource, /await RestartApp\(\)/, 'a successful selection must automatically restart the app')
  assert.match(appSource, /aiAssistantStore\.isBusy/, 'assistant work must block switching')
  assert.match(appSource, /aiLibrarianStore\.isGenerating/, 'librarian work must block switching')
  assert.match(appSource, /aiWritingStore\.isBusy/, 'writing work must block switching')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-storage-spaces.mjs')).href)
  const { useStorageSpaceStore } = await import(pathToFileURL(storeOutFile).href)
  const store = useStorageSpaceStore()
  assert.equal(await store.initialize(), true)
  assert.equal(store.activeSpace.name, 'メイン')
  const created = await store.create('仕事')
  assert.equal(created.name, '仕事')
  assert.equal(store.spaces.length, 2)
  assert.equal(store.activeSpaceId, 'main-id', 'creating a space must not switch it')
  assert.equal(await store.create('仕事'), null, 'duplicate names must fail safely')
  assert.equal(store.error.code, 'STORAGE_SPACE_NAME_CONFLICT')

  let prepareCalls = 0
  let rollbackCalls = 0
  let restartCalls = 0
  store.setSwitchLifecycle(
    async () => {
      prepareCalls += 1
      return { ready: false }
    },
    async () => { restartCalls += 1 },
  )
  assert.equal(await store.switchTo('work-id'), false)
  assert.equal(prepareCalls, 1)
  assert.equal(mock.calls.select.length, 0, 'a rejected preparation must not persist selection')

  store.setSwitchLifecycle(
    async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
    async () => { completeCalls += 1 },
  )
  mock.failNextSelection({ code: 'STORAGE_SPACE_IN_USE', message: '使用中です。' })
  assert.equal(await store.switchTo('work-id'), false)
  assert.equal(rollbackCalls, 1, 'a backend selection failure must resume the current space')
  assert.equal(store.activeSpaceId, 'main-id')

  store.setSwitchLifecycle(
    async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
    async () => {
      restartCalls += 1
      throw new Error('automatic restart is unavailable in Wails development mode')
    },
  )
  assert.equal(await store.switchTo('work-id'), false)
  assert.equal(store.activeSpaceId, 'work-id', 'a persisted selection must remain selected after restart failure')
  assert.equal(store.error.code, 'STORAGE_SPACE_RESTART_FAILED')
  assert.match(store.error.message, /wails dev/, 'development restart failures must explain how to restart the dev runner')
  assert.equal(rollbackCalls, 1, 'sync must remain suspended after selection is persisted')
  assert.equal(restartCalls, 1)

  mock.activate('main-id')
  assert.equal(await store.initialize(), true)
  mock.calls.order.length = 0
  store.setSwitchLifecycle(
    async () => ({ ready: true, rollback: () => { rollbackCalls += 1 } }),
    async () => {
      mock.calls.order.push('restart')
      restartCalls += 1
    },
  )
  assert.equal(await store.switchTo('work-id'), true)
  assert.deepEqual(mock.calls.order, ['select', 'restart'], 'selection must persist before the app restarts')
  assert.equal(store.activeSpaceId, 'work-id')
  assert.equal(rollbackCalls, 1, 'a successful switch must keep sync suspended for exit')
  assert.equal(restartCalls, 2)

  const { prepareStorageSpaceSwitch } = await import(pathToFileURL(guardOutFile).href)
  const events = []
  let syncBusy = false
  let aiBusy = false
  let flushResult = true
  const dependencies = {
    isSyncBusy: () => syncBusy,
    isAIBusy: () => aiBusy,
    suspendSync: () => { events.push('suspend'); return true },
    resumeSync: () => { events.push('resume') },
    flushAllDirtyNotes: async () => { events.push('flush'); return flushResult },
    notify: (_message, code) => { events.push(code) },
  }

  syncBusy = true
  assert.equal((await prepareStorageSpaceSwitch(dependencies)).ready, false)
  assert.deepEqual(events, ['STORAGE_SPACE_SYNC_BUSY'])
  syncBusy = false
  events.length = 0
  aiBusy = true
  assert.equal((await prepareStorageSpaceSwitch(dependencies)).ready, false)
  assert.deepEqual(events, ['STORAGE_SPACE_AI_BUSY'])
  aiBusy = false
  events.length = 0
  flushResult = false
  assert.equal((await prepareStorageSpaceSwitch(dependencies)).ready, false)
  assert.deepEqual(events, ['suspend', 'flush', 'resume', 'STORAGE_SPACE_DRAFT_SAVE_FAILED'])
  events.length = 0
  flushResult = true
  const prepared = await prepareStorageSpaceSwitch(dependencies)
  assert.equal(prepared.ready, true)
  assert.deepEqual(events, ['suspend', 'flush'])
  await prepared.rollback()
  await prepared.rollback()
  assert.deepEqual(events, ['suspend', 'flush', 'resume'], 'rollback must resume sync exactly once')

  console.log('storage space tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
