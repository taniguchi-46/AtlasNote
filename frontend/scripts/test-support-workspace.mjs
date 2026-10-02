import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>')
Object.assign(globalThis, {
  window: dom.window, document: dom.window.document, Node: dom.window.Node,
  Element: dom.window.Element, HTMLElement: dom.window.HTMLElement, SVGElement: dom.window.SVGElement,
  ResizeObserver: class { observe() {} unobserve() {} disconnect() {} },
})
const { createPinia, setActivePinia } = await import('pinia')
const { createApp, nextTick } = await import('vue')
const captures = new Map()
dom.window.HTMLElement.prototype.setPointerCapture = function (id) { captures.set(id, this) }
dom.window.HTMLElement.prototype.hasPointerCapture = function (id) { return captures.get(id) === this }
dom.window.HTMLElement.prototype.releasePointerCapture = function (id) { if (captures.get(id) === this) captures.delete(id) }
function pointer(target, type, x = 100, y = 100, pointerId = 1, extra = {}) {
  const event = new dom.window.Event(type, { bubbles: true, cancelable: true })
  Object.assign(event, { clientX: x, clientY: y, pointerId, isPrimary: true, button: 0, ...extra })
  target.dispatchEvent(event)
  return event
}

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
  // Mount the actual component: events in the capture target must work without window listeners.
  const transpile = source => ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } }).outputText
  const { descriptor } = parse(component)
  const mountedSource = compileScript(descriptor, { id: 'support-test', inlineTemplate: true }).content
    .replace("from '../stores/useSupportWorkspaceStore'", "from './useSupportWorkspaceStore.mjs'")
    .replace("from '../stores/useSettingsStore'", "from './mock-settings.mjs'")
    .replace("from '../stores/useNoteStore'", "from './mock-note.mjs'")
    .replace(/import atlasNoteLogo from [^\n]+/, "const atlasNoteLogo = ''")
    .replace(/import (ExternalChangeReview|TerminalComponent) from [^\n]+/g, 'const $1 = { render: () => null }')
  await writeFile(path.join(outDir, 'component.mjs'), transpile(mountedSource))
  await writeFile(path.join(outDir, 'mock-settings.mjs'), `
export const AI_WORKSPACE_BOTTOM_HEIGHT_MIN = 180, AI_WORKSPACE_BOTTOM_HEIGHT_MAX = 760
export const AI_WORKSPACE_RIGHT_WIDTH_MIN = 300, AI_WORKSPACE_RIGHT_WIDTH_MAX = 960
import { reactive } from 'vue'
export const settings = reactive({ aiWorkspacePlacement: 'right', aiWorkspaceRightWidth: 480, aiWorkspaceBottomHeight: 360, setAIWorkspaceRightWidth(size) { this.aiWorkspaceRightWidth = size }, setAIWorkspaceBottomHeight(size) { this.aiWorkspaceBottomHeight = size } })
export function useSettingsStore() { return settings }
`)
  await writeFile(path.join(outDir, 'mock-note.mjs'), 'export function useNoteStore() { return { activeNote: null } }')
  const { default: Component } = await import(pathToFileURL(path.join(outDir, 'component.mjs')).href)
  store.setPosition({ left: 120, top: 100, width: 320, height: 300 })
  const mounted = createApp(Component)
  mounted.mount('#app')
  await nextTick()
  const header = document.querySelector('.support-header')
  const space = document.querySelector('.support-drag-space')
  for (const element of document.querySelectorAll('.support-header button, .support-header nav')) {
    pointer(element, 'pointerdown')
    assert.equal(captures.size, 0, 'tabs and actions do not capture')
  }
  pointer(space, 'pointerdown', 150, 120)
  assert.equal(captures.get(1), header)
  pointer(header, 'pointermove', 190, 150, 2)
  assert.equal(store.position.left, 120, 'other pointers cannot move the panel')
  pointer(header, 'pointermove', 190, 150)
  assert.deepEqual([store.position.left, store.position.top], [160, 130])
  pointer(header, 'pointermove', 10000, 10000)
  assert.deepEqual([store.position.left, store.position.top], [window.innerWidth - 328, window.innerHeight - 308])
  pointer(header, 'pointermove', -10000, -10000)
  assert.deepEqual([store.position.left, store.position.top], [8, 8])
  pointer(header, 'pointerup')
  assert.equal(captures.size, 0)
  pointer(header, 'pointermove', 500, 500)
  assert.equal(store.position.left, 8, 'pointerup removed movement listeners')
  for (const ending of ['pointercancel', 'lostpointercapture']) {
    pointer(space, 'pointerdown')
    pointer(header, ending)
    assert.equal(captures.size, 0)
  }
  pointer(space, 'pointerdown', 100, 100, 1, { button: 2 })
  assert.equal(captures.size, 0)
  const resizer = document.querySelector('.support-floating-resize')
  pointer(resizer, 'pointerdown')
  assert.equal(captures.get(1), resizer)
  pointer(resizer, 'pointermove', 140, 130)
  assert.deepEqual([store.position.width, store.position.height], [360, 330])
  pointer(resizer, 'pointerup')
  pointer(space, 'pointerdown')
  store.minimize()
  await nextTick()
  assert.equal(captures.size, 0, 'minimize releases capture')
  store.restore()
  store.toggleFloating()
  await nextTick()
  pointer(space, 'pointerdown')
  assert.equal(captures.size, 0, 'dock does not drag')
  const { settings } = await import(pathToFileURL(path.join(outDir, 'mock-settings.mjs')).href)
  const section = document.querySelector('.support-workspace')
  const originalHeight = settings.aiWorkspaceBottomHeight
  const originalWidth = settings.aiWorkspaceRightWidth
  store.toggleMaximized()
  await nextTick()
  assert.equal(section.style.position, 'absolute')
  assert.match(section.style.inset, /^0(px)?$/)
  assert.equal(document.querySelector('.support-dock-resize'), null)
  store.toggleMaximized()
  await nextTick()
  assert.equal(section.style.height, originalHeight + 'px')
  settings.aiWorkspacePlacement = 'right'
  await nextTick()
  settings.aiWorkspacePlacement = 'bottom'
  await nextTick()
  assert.equal(section.classList.contains('is-bottom-panel'), true)
  store.toggleMaximized()
  await nextTick()
  store.minimize()
  await nextTick()
  store.restore()
  await nextTick()
  assert.equal(store.isMaximized, true)
  store.toggleMaximized()
  await nextTick()
  assert.equal(settings.aiWorkspaceBottomHeight, originalHeight)
  assert.equal(settings.aiWorkspaceRightWidth, originalWidth)
  store.toggleFloating()
  store.toggleMaximized()
  await nextTick()
  assert.equal(store.isMaximized, false, 'floating maximize is disabled')
  pointer(space, 'pointerdown')
  window.dispatchEvent(new dom.window.Event('blur'))
  assert.equal(captures.size, 0, 'blur releases capture')
  pointer(space, 'pointerdown')
  mounted.unmount()
  assert.equal(captures.size, 0, 'unmount releases capture')
  assert.match(component, /\.support-header nav\{flex:0 0 auto\}/)
  assert.match(component, /\.support-drag-space\{flex:1/)
  assert.match(component, /touch-action:none;--wails-draggable:no-drag/)
  assert.doesNotMatch(component, /<img/)
  console.log('support workspace tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
