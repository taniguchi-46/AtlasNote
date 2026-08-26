import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const apiPath = path.join(rootDir, 'src', 'api', 'noteImport.ts')
const storePath = path.join(rootDir, 'src', 'stores', 'useNoteImportStore.ts')
const modalPath = path.join(rootDir, 'src', 'components', 'NoteImportModal.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const topBarPath = path.join(rootDir, 'src', 'components', 'AppTopBar.vue')
const switchGuardPath = path.join(rootDir, 'src', 'services', 'storageSpaceSwitch.ts')
const outDir = path.join(rootDir, '.tmp', 'note-import-test')
const storeOutFile = path.join(outDir, 'useNoteImportStore.mjs')

await mkdir(outDir, { recursive: true })
const storeSource = (await readFile(storePath, 'utf8'))
  .replace("from '../api/noteImport'", "from './mock-note-import.mjs'")
const compiledStore = ts.transpileModule(storeSource, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(storeOutFile, compiledStore.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-note-import.mjs'), `
export const calls = []
let nextResult = { cancelled: false, imported: [], failures: [] }
let nextError = null
let pendingResolver = null
let deferNext = false
export function setNextResult(result) { nextResult = structuredClone(result); nextError = null }
export function setNextError(error) { nextError = error; deferNext = false }
export function deferResult() { deferNext = true; nextError = null }
export function resolveDeferred(result) {
  if (!pendingResolver) throw new Error('no deferred import')
  const resolve = pendingResolver
  pendingResolver = null
  deferNext = false
  resolve(structuredClone(result))
}
export async function importNotes(input) {
  calls.push(structuredClone(input))
  if (nextError) throw nextError
  if (deferNext) return new Promise((resolve) => { pendingResolver = resolve })
  return structuredClone(nextResult)
}
`, 'utf8')

try {
  const [apiSource, modalSource, appSource, topBarSource, switchGuardSource] = await Promise.all([
    readFile(apiPath, 'utf8'),
    readFile(modalPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(topBarPath, 'utf8'),
    readFile(switchGuardPath, 'utf8'),
  ])

  assert.match(apiSource, /ImportNotes/, 'the API wrapper must call the Wails ImportNotes method')
  assert.match(apiSource, /notebookId/, 'the API contract must include an existing notebook destination')
  assert.match(apiSource, /newNotebookName/, 'the API contract must include a new notebook destination')
  assert.match(apiSource, /titleMode/, 'the API contract must include the selected title mode')
  assert.match(topBarSource, /import-notes/, 'the top bar must expose the import action')
  assert.match(topBarSource, /ノートをインポート/, 'the import action needs an accessible label')
  assert.doesNotMatch(modalSource, /wailsjs/, 'the component must call the store rather than Wails directly')
  assert.match(modalSource, /useNoteImportStore/, 'the modal must use the import store')
  assert.match(modalSource, /requestAccess\(/, 'an existing notebook destination must pass the common lock gate')
  assert.match(modalSource, /value="root"/, 'root must be the default destination option')
  assert.match(modalSource, /value="notebook"/, 'an existing notebook destination option is required')
  assert.match(modalSource, /value="new-notebook"/, 'a new notebook destination option is required')
  assert.match(modalSource, /id="note-import-title-mode"/, 'the modal must provide a title-mode select')
  assert.match(modalSource, /value="auto"/, 'automatic title selection must be available')
  assert.match(modalSource, /value="filename"/, 'filename title selection must be available')
  assert.match(modalSource, /value="heading"/, 'heading title selection must be available')
  assert.match(modalSource, /value="metadata"/, 'metadata title selection must be available')
  assert.match(modalSource, /titleMode\.value = 'auto'/, 'opening the modal must reset the title mode to auto')
  assert.match(modalSource, /選択した情報がない場合は、ファイル名を使用します。/, 'title fallback must be explained')
  assert.match(modalSource, /ファイルの選択をキャンセルしました/, 'picker cancellation must not be shown as an error')
  assert.match(modalSource, /成功したノートは保持されています/, 'partial persistence results must explain retained notes')
  assert.match(appSource, /handleNoteImportCompleted/, 'App must refresh the workspace after an import')
  assert.match(appSource, /syncStore\.scheduleAutoSync\(\)/, 'imported notes must schedule automatic sync')
  assert.match(appSource, /noteStore\.fetchNotes\(\[\], noteStore\.activeTagId/, 'note list refresh must preserve the active tag filter')
  assert.match(appSource, /notebookStore\.fetchNotebooks\(\)/, 'new notebook imports must refresh notebook state')
  assert.match(switchGuardSource, /isImportBusy/, 'storage-space switching must observe import activity')
  assert.match(switchGuardSource, /STORAGE_SPACE_IMPORT_BUSY/, 'storage-space switching needs a dedicated import-busy outcome')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-note-import.mjs')).href)
  const { useNoteImportStore } = await import(pathToFileURL(storeOutFile).href)
  const store = useNoteImportStore()

  mock.setNextResult({
    cancelled: false,
    imported: [{ sourceName: 'note.md', noteId: 'note-1', title: 'Imported' }],
    failures: [],
  })
  const completed = await store.run({ notebookId: 'notebook-1', titleMode: 'filename' })
  assert.equal(completed.imported.length, 1)
  assert.equal(store.error, null)
  assert.equal(store.lastResult.imported[0].noteId, 'note-1')
  assert.deepEqual(mock.calls.at(-1), { notebookId: 'notebook-1', titleMode: 'filename' })

  mock.setNextResult({
    cancelled: false,
    imported: [{ sourceName: 'kept.md', noteId: 'note-2', title: 'Kept' }],
    failures: [{ sourceName: 'bad.html', code: 'NOTE_IMPORT_INVALID_HTML', message: '変換できませんでした。' }],
    error: { code: 'NOTE_IMPORT_PERSISTENCE_FAILED', message: '残りを停止しました。', retryable: true },
  })
  const partial = await store.run({ newNotebookName: '取り込み' })
  assert.equal(partial.imported.length, 1)
  assert.equal(partial.failures.length, 1)
  assert.equal(store.error.code, 'NOTE_IMPORT_PERSISTENCE_FAILED')

  mock.setNextResult({ cancelled: true, imported: [], failures: [] })
  const cancelled = await store.run({})
  assert.equal(cancelled.cancelled, true)
  assert.equal(store.error, null)

  mock.deferResult()
  const pending = store.run({})
  assert.equal(store.isBusy, true)
  assert.equal(await store.run({}), null, 'duplicate requests must be ignored while a batch is running')
  mock.resolveDeferred({ cancelled: false, imported: [], failures: [] })
  await pending
  assert.equal(store.isBusy, false)

  mock.setNextError(new Error('Wails unavailable'))
  assert.equal(await store.run({}), null)
  assert.equal(store.error.code, 'NOTE_IMPORT_UNAVAILABLE')
  store.reset()
  assert.equal(store.error, null)
  assert.equal(store.lastResult, null)

  console.log('note import tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
