import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const root = process.cwd()
const outDir = path.join(root, '.tmp', 'support-workspace-test')
await mkdir(outDir, { recursive: true })
try {
  const source = await readFile(path.join(root, 'src/stores/useSupportWorkspaceStore.ts'), 'utf8')
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } })
  const outFile = path.join(outDir, 'useSupportWorkspaceStore.mjs')
  await writeFile(outFile, compiled.outputText, 'utf8')
  const { useSupportWorkspaceStore } = await import(pathToFileURL(outFile).href)
  setActivePinia(createPinia())
  const store = useSupportWorkspaceStore()

  assert.equal(store.activeTab, 'organize')
  assert.equal(store.isOpen, false)
  store.open('organize')
  store.setPosition({ left: 120, width: 320 })
  store.toggleFloating()
  store.open('ai')
  const firstFocus = store.focusAIRequest
  store.minimize()
  store.restore()
  assert.equal(store.activeTab, 'ai')
  assert.equal(store.isMinimized, false)
  assert.equal(store.isFloating, true)
  assert.deepEqual([store.position.left, store.position.width], [120, 320])
  assert.equal(store.focusAIRequest, firstFocus + 1)
  store.toggleAI()
  assert.equal(store.isMinimized, true)
  store.toggleAI()
  assert.equal(store.isMinimized, false)
  assert.equal(store.activeTab, 'ai')
  store.open('organize')
  store.toggleAI()
  assert.equal(store.activeTab, 'ai')
  assert.equal(store.isMinimized, false)
  console.log('support workspace tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
