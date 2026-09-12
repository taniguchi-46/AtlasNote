import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { pathToFileURL } from 'node:url'
import path from 'node:path'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from 'vue/compiler-sfc'
import ts from 'typescript'

const dom = new JSDOM('<!doctype html><body></body>', { url: 'https://atlasnote.test' })
for (const key of ['window', 'document', 'Element', 'HTMLElement', 'SVGElement', 'Node']) {
  globalThis[key] = dom.window[key]
}
const { createApp, h, nextTick, reactive } = await import('vue')
const outDir = path.resolve('.tmp/mermaid-node-view-test')
await mkdir(outDir, { recursive: true })
try {
  const source = await readFile('src/components/MermaidCodeBlockView.vue', 'utf8')
  const { descriptor } = parse(source)
  const compiled = compileScript(descriptor, { id: 'mermaid-test', inlineTemplate: true }).content
    .replace("from '@tiptap/vue-3'", "from './mocks.mjs'")
    .replace("from '../stores/useAppStore'", "from './mocks.mjs'")
    .replace("from '../stores/useNoteStore'", "from './mocks.mjs'")
    .replace("from '../utils/mermaidRenderer'", "from './mocks.mjs'")
    .replace("from './MermaidEditDialog.vue'", "from './mocks.mjs'")
  await writeFile(path.join(outDir, 'view.mjs'), ts.transpileModule(compiled, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  }).outputText)
  await writeFile(path.join(outDir, 'mocks.mjs'), `
import { h, reactive } from 'vue'
export const nodeViewProps = { node: Object, editor: Object, extension: Object, getPos: Function }
export const NodeViewWrapper = (_, { slots }) => h('pre', slots.default?.())
export const NodeViewContent = () => h('code')
export const MermaidEditDialog = { render: () => null }
export default MermaidEditDialog
export const store = reactive({ theme: 'light' })
export const useAppStore = () => store
export const useNoteStore = () => ({ isNoteDeletionPreparing: () => false })
export const pending = []
export const renderMermaidDiagram = (source, options) => new Promise((resolve, reject) => {
  pending.push({ source, options, resolve, reject })
})
`)
  const View = (await import(pathToFileURL(path.join(outDir, 'view.mjs')).href)).default
  const { store, pending } = await import(pathToFileURL(path.join(outDir, 'mocks.mjs')).href)
  const created = [], revoked = []
  URL.createObjectURL = () => { const url = `blob:test-${created.length}`; created.push(url); return url }
  URL.revokeObjectURL = url => revoked.push(url)
  const props = reactive({
    node: { attrs: { language: 'mermaid' }, textContent: 'first' },
    editor: { isDestroyed: false, isEditable: true },
    extension: { storage: { noteId: 'node-view-test', generation: 1 } },
    getPos: () => 0,
  })
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(View, props) })
  app.mount(host)
  const debounce = async () => { await nextTick(); await new Promise(resolve => setTimeout(resolve, 280)) }
  const finish = async (index, text = 'diagram') => {
    pending[index].resolve({ ok: true, svg: '<svg/>', altText: text })
    await Promise.resolve(); await nextTick()
  }
  await debounce()
  props.node.textContent = 'second'
  await debounce()
  await finish(0, 'stale edit')
  assert.equal(created.length, 0)
  await finish(1)
  assert.equal(host.querySelector('img')?.src, created[0])
  store.theme = 'dark'
  await debounce()
  assert.equal(pending[2].options.theme, 'dark')
  props.node.textContent = 'third'
  await debounce()
  await finish(2, 'stale theme')
  assert.equal(created.length, 1)
  await finish(3)
  assert.deepEqual(revoked, [created[0]])
  // Note replacement uses setContent; a reused NodeView receives a new node.
  props.node = { attrs: { language: 'mermaid' }, textContent: 'next note' }
  await debounce()
  // Lock clears editor content, removing this NodeView (same teardown as unmount).
  app.unmount()
  await finish(4, 'late locked note')
  assert.equal(created.length, 2)
  assert.deepEqual(revoked, created)
  assert.equal(host.querySelector('img'), null)
  // A language change cancels both a pending render and its preview URL.
  const second = createApp({ render: () => h(View, props) })
  second.mount(host)
  await debounce()
  await finish(5)
  props.node.textContent = 'pending'
  await debounce()
  props.node.attrs.language = 'typescript'
  await nextTick()
  await finish(6)
  assert.equal(created.length, 3)
  assert.deepEqual(revoked, created)
  second.unmount()
  host.remove()
  console.log('Mermaid NodeView lifecycle tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
  dom.window.close()
}
