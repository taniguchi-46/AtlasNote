import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from 'vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>')
Object.assign(globalThis, {
  window: dom.window, document: dom.window.document, Node: dom.window.Node,
  Element: dom.window.Element, HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement, Event: dom.window.Event,
})
const { createPinia, setActivePinia } = await import('pinia')
const { createApp, h, nextTick, ref } = await import('vue')

const root = process.cwd()
const outDir = path.join(root, '.tmp', 'related-notes-test')
await mkdir(outDir, { recursive: true })
const compile = async (source, destination) => {
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  })
  await writeFile(path.join(outDir, destination), compiled.outputText, 'utf8')
}

try {
  const storeSource = (await readFile(path.join(root, 'src/stores/useNoteLinkStore.ts'), 'utf8'))
    .replace("from '../api/noteLinks'", "from './mock-links.mjs'")
    .replace("from '../api/relatedNotes'", "from './mock-related.mjs'")
    .replace("from '../utils/latestRequestGuard'", "from './guard.mjs'")
    .replace("from './useNotificationStore'", "from './mock-notifications.mjs'")
    .replace("from './useAIChatStore'", "from './mock-ai.mjs'")
  await compile(storeSource, 'store.mjs')
  await compile(await readFile(path.join(root, 'src/utils/latestRequestGuard.ts'), 'utf8'), 'guard.mjs')
  await writeFile(path.join(outDir, 'mock-links.mjs'), 'export async function listBacklinks() { return { items: [], page: 1, total: 0, hasNext: false } }\nexport async function searchNoteLinkTargets() { return { items: [] } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-notifications.mjs'), 'export function useNotificationStore() { return { dismissBySource() {}, notify() {} } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-ai.mjs'), 'export const added = []; export function useAIChatStore() { return { addNoteContext(id) { added.push(id); return true } } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-related.mjs'), `
export const pending = []
export function relatedNotes(input) {
  return new Promise((resolve, reject) => pending.push({ input, resolve, reject }))
}
`, 'utf8')
  await writeFile(path.join(outDir, 'mock-component-store.mjs'), "export { useNoteLinkStore } from './store.mjs'", 'utf8')
  await writeFile(path.join(outDir, 'mock-note-store.mjs'), 'export function useNoteStore() { return { async selectNote() {} } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-space-store.mjs'), 'export function useStorageSpaceStore() { return { isSwitching: false } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-organization-store.mjs'), 'export function useOrganizationStore() { return { openNote() {} } }', 'utf8')
  await writeFile(path.join(outDir, 'mock-notebook-store.mjs'), "export function useNotebookStore() { return { notebooks: [{ id: 'notebook-a', name: 'Notebook A' }, { id: 'notebook-b', name: 'Notebook B' }] } }", 'utf8')
  await writeFile(path.join(outDir, 'mock-popover.mjs'), `
import { defineComponent, h, inject, provide, toRef } from 'vue'
export const PopoverRoot = defineComponent({ props: ['open'], emits: ['update:open'], setup(props, { slots, emit }) {
  provide('popover', { open: toRef(props, 'open'), toggle: () => emit('update:open', !props.open) })
  return () => slots.default?.()
} })
export const PopoverTrigger = defineComponent({ setup(_, { slots }) {
  const popover = inject('popover')
  return () => h('span', { onClick: popover.toggle }, slots.default?.())
} })
export const PopoverPortal = defineComponent({ setup(_, { slots }) { return () => slots.default?.() } })
export const PopoverContent = defineComponent({ setup(_, { slots }) {
  const popover = inject('popover')
  return () => popover.open.value ? h('div', slots.default?.()) : null
} })
`, 'utf8')
  const componentSource = await readFile(path.join(root, 'src/components/NoteBacklinks.vue'), 'utf8')
  const { descriptor, errors } = parse(componentSource)
  assert.deepEqual(errors, [])
  const component = compileScript(descriptor, { id: 'related-panel-test', inlineTemplate: true }).content
    .replace("from '../stores/useNoteLinkStore'", "from './mock-component-store.mjs'")
    .replace("from '../stores/useNoteStore'", "from './mock-note-store.mjs'")
    .replace("from '../stores/useStorageSpaceStore'", "from './mock-space-store.mjs'")
    .replace("from './BacklinkPanelContent.vue'", "from './mock-panel.mjs'")
    .replace("from 'reka-ui'", "from './mock-popover.mjs'")
  await compile(component, 'NoteBacklinks.mjs')
  const panelSource = await readFile(path.join(root, 'src/components/BacklinkPanelContent.vue'), 'utf8')
  const panelDescriptor = parse(panelSource).descriptor
  const panel = compileScript(panelDescriptor, { id: 'related-panel-content-test', inlineTemplate: true }).content
    .replace("from '../stores/useOrganizationStore'", "from './mock-organization-store.mjs'")
    .replace("from '../stores/useNotebookStore'", "from './mock-notebook-store.mjs'")
    .replace("from '../stores/useNoteLinkStore'", "from './mock-component-store.mjs'")
  await compile(panel, 'mock-panel.mjs')

  const pinia = createPinia()
  setActivePinia(pinia)
  const { useNoteLinkStore } = await import(pathToFileURL(path.join(outDir, 'store.mjs')).href)
  const { pending } = await import(pathToFileURL(path.join(outDir, 'mock-related.mjs')).href)
  const { added } = await import(pathToFileURL(path.join(outDir, 'mock-ai.mjs')).href)
  const store = useNoteLinkStore()
  const candidate = { noteId: 'candidate', title: 'candidate', revision: 1, snippet: '', reasons: ['リンク'] }
  const first = store.loadRelated('old')
  const second = store.loadRelated('current')
  pending[1].resolve({ items: [candidate] })
  await second
  pending[0].resolve({ items: [{ ...candidate, noteId: 'old' }] })
  await first
  assert.deepEqual(store.relatedItems.map((item) => item.noteId), ['candidate'])

  const add = store.addRelatedContext('current', candidate)
  pending[2].resolve({ items: [candidate] })
  assert.equal(await add, true)
  assert.deepEqual(added, ['candidate'])

  const staleAdd = store.addRelatedContext('current', candidate)
  store.clearRelated()
  pending[3].resolve({ items: [candidate] })
  assert.equal(await staleAdd, false)
  assert.equal(store.relatedItems.length, 0)
  assert.deepEqual(added, ['candidate'])

  const changed = store.loadRelated('current')
  pending[4].resolve({ items: [candidate] })
  await changed
  const changedRevision = store.addRelatedContext('current', candidate)
  pending[5].resolve({ items: [{ ...candidate, revision: 2 }] })
  assert.equal(await changedRevision, false)
  assert.deepEqual(added, ['candidate'])

  store.setRelatedScope('notebook-a', true)
  const scoped = store.loadRelated('current')
  assert.deepEqual(pending[6].input, { noteId: 'current', notebookId: 'notebook-a', descendants: true, limit: 20 })
  pending[6].resolve({ items: [candidate] })
  await scoped
  const scopedAdd = store.addRelatedContext('current', candidate)
  assert.deepEqual(pending[7].input, pending[6].input)
  store.setRelatedScope('notebook-b', false)
  pending[7].resolve({ items: [candidate] })
  assert.equal(await scopedAdd, false)
  store.setRelatedScope(null, false)

  const { default: NoteBacklinks } = await import(pathToFileURL(path.join(outDir, 'NoteBacklinks.mjs')).href)
  const selected = ref('first-note')
  const app = createApp({ render: () => h(NoteBacklinks, { noteId: selected.value }) })
  app.use(pinia)
  app.mount(document.getElementById('app'))
  await nextTick()
  const beforeOpen = pending.length
  selected.value = 'second-note'
  await nextTick()
  assert.equal(pending.length, beforeOpen, 'closed panel must not request related notes')
  await store.refreshRelatedIfVisible('second-note')
  assert.equal(pending.length, beforeOpen, 'save while closed must not request related notes')
  document.querySelector('.backlink-trigger').click()
  await nextTick()
  assert.equal(pending.length, beforeOpen + 1)
  assert.equal(pending.at(-1).input.noteId, 'second-note')
  const notebookSelect = document.querySelector('#related-notebook-scope')
  notebookSelect.value = 'notebook-a'
  notebookSelect.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
  await nextTick()
  assert.deepEqual(pending.at(-1).input, { noteId: 'second-note', notebookId: 'notebook-a', descendants: false, limit: 20 })
  const descendantsCheckbox = document.querySelector('.related-scope input[type="checkbox"]')
  descendantsCheckbox.checked = true
  descendantsCheckbox.dispatchEvent(new dom.window.Event('change', { bubbles: true }))
  await nextTick()
  assert.deepEqual(pending.at(-1).input, { noteId: 'second-note', notebookId: 'notebook-a', descendants: true, limit: 20 })
  const beforeSave = pending.length
  const saveWhileOpen = store.refreshRelatedIfVisible('second-note')
  assert.equal(pending.length, beforeSave + 1, 'save while open must refresh related notes')
  assert.deepEqual(pending.at(-1).input, { noteId: 'second-note', notebookId: 'notebook-a', descendants: true, limit: 20 })
  pending.at(-1).resolve({ items: [candidate] })
  await saveWhileOpen
  document.querySelector('.backlink-trigger').click()
  await nextTick()
  assert.equal(store.relatedItems.length, 0)
  selected.value = 'third-note'
  await nextTick()
  await store.refreshRelatedIfVisible('third-note')
  assert.equal(pending.length, beforeSave + 1, 'hidden changes must not request related notes')
  pending[beforeOpen].resolve({ items: [candidate] })
  await nextTick()
  assert.equal(store.relatedItems.length, 0, 'closed panel must discard its old response')
  document.querySelector('.backlink-trigger').click()
  await nextTick()
  assert.equal(pending.at(-1).input.noteId, 'third-note')
  pending.at(-1).resolve({ items: [candidate] })
  await nextTick()
  app.unmount()

  console.log('related notes store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
