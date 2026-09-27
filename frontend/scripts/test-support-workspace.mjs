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

  assert.equal(store.activeTab, 'terminal')
  assert.equal(store.isOpen, false)
  store.open('changes')
  store.setPosition({ left: 120, width: 320 })
  store.toggleFloating()
  store.open('terminal')
  store.minimize()
  store.restore()
  assert.equal(store.activeTab, 'terminal')
  assert.equal(store.isMinimized, false)
  assert.equal(store.isFloating, true)
  assert.deepEqual([store.position.left, store.position.width], [120, 320])
  store.open('changes')
  assert.equal(store.activeTab, 'changes')
  const component = await readFile(path.join(root, 'src/components/SupportWorkspace.vue'), 'utf8')
  const topbar = await readFile(path.join(root, 'src/components/AppTopBar.vue'), 'utf8')
  const app = await readFile(path.join(root, 'src/App.vue'), 'utf8')
  assert.match(component, /<ExternalChangeReview v-show=/)
  assert.match(component, /<TerminalComponent v-show=/)
  assert.doesNotMatch(component, /<OrganizationCenter|<AIWorkspace|store\.open\('organize'\)|store\.open\('ai'\)/)
  assert.match(topbar, /open-terminal/)
  assert.match(app, /supportStore\.open\('terminal'\)/)
  console.log('support workspace tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
