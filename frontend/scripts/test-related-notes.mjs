import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const root = process.cwd()
const outDir = path.join(root, '.tmp', 'related-notes-test')
await mkdir(outDir, { recursive: true })
try {
  const source = (await readFile(path.join(root, 'src/stores/useNoteLinkStore.ts'), 'utf8'))
    .replace("from '../api/noteLinks'", "from './mock-links.mjs'")
    .replace("from '../api/relatedNotes'", "from './mock-related.mjs'")
    .replace("from '../utils/latestRequestGuard'", "from './guard.mjs'")
    .replace("from './useNotificationStore'", "from './mock-notifications.mjs'")
  for (const [name, content] of [
    ['store.mjs', source],
    ['guard.mjs', await readFile(path.join(root, 'src/utils/latestRequestGuard.ts'), 'utf8')],
  ]) {
    await writeFile(path.join(outDir, name), ts.transpileModule(content, {
      compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
    }).outputText)
  }
  await writeFile(path.join(outDir, 'mock-links.mjs'), 'export async function listBacklinks() { return { items: [], page: 1, total: 0, hasNext: false } }\nexport async function searchNoteLinkTargets() { return { items: [] } }')
  await writeFile(path.join(outDir, 'mock-notifications.mjs'), 'export function useNotificationStore() { return { dismissBySource() {}, notify() {} } }')
  await writeFile(path.join(outDir, 'mock-related.mjs'), 'export const pending = []; export function relatedNotes(input) { return new Promise(resolve => pending.push({ input, resolve })) }')
  setActivePinia(createPinia())
  const { useNoteLinkStore } = await import(pathToFileURL(path.join(outDir, 'store.mjs')).href)
  const { pending } = await import(pathToFileURL(path.join(outDir, 'mock-related.mjs')).href)
  const store = useNoteLinkStore()
  const first = store.loadRelated('old')
  const second = store.loadRelated('current')
  pending[1].resolve({ items: [{ noteId: 'current', revision: 1 }] })
  await second
  pending[0].resolve({ items: [{ noteId: 'old', revision: 1 }] })
  await first
  assert.deepEqual(store.relatedItems.map(item => item.noteId), ['current'])
  store.setRelatedScope('notebook-a', true)
  const scoped = store.loadRelated('current')
  assert.deepEqual(pending[2].input, { noteId: 'current', notebookId: 'notebook-a', descendants: true, limit: 20 })
  pending[2].resolve({ items: [] })
  await scoped
  const panel = await readFile(path.join(root, 'src/components/BacklinkPanelContent.vue'), 'utf8')
  assert.match(panel, /関連候補/)
  assert.match(panel, /ノートを開く/)
  assert.doesNotMatch(panel, /関連ノートを整理|参照に追加/)
  console.log('related notes tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
