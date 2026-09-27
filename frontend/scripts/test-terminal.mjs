import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>')
Object.assign(globalThis, {
  window: dom.window, document: dom.window.document, navigator: dom.window.navigator,
  Node: dom.window.Node, Element: dom.window.Element, HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement, Event: dom.window.Event,
  requestAnimationFrame: callback => setTimeout(callback, 0), cancelAnimationFrame: clearTimeout,
  ResizeObserver: class { static instance; constructor(callback) { this.callback = callback; ResizeObserver.instance = this } observe() {} disconnect() {} },
})
Object.defineProperties(dom.window.HTMLElement.prototype, {
  clientWidth: { configurable: true, get: () => 640 },
  clientHeight: { configurable: true, get: () => 320 },
})
const { createApp, nextTick } = await import('vue')
const root = process.cwd()
const outDir = path.join(root, '.tmp', 'terminal-test')
await mkdir(outDir, { recursive: true })
const source = await readFile(path.join(root, 'src/components/TerminalComponent.vue'), 'utf8')
const support = await readFile(path.join(root, 'src/components/SupportWorkspace.vue'), 'utf8')
const appSource = await readFile(path.join(root, 'src/App.vue'), 'utf8')
assert.match(support, /store\.activeTab === 'terminal'/)
assert.match(support, /<TerminalComponent v-show=/)
assert.match(appSource, /\[data-terminal\]/)
const { descriptor } = parse(source)
const compiled = compileScript(descriptor, { id: 'terminal-test', inlineTemplate: true }).content
  .replace("from '@xterm/xterm'", "from './mock-xterm.mjs'")
  .replace("from '@xterm/addon-fit'", "from './mock-xterm.mjs'")
  .replace("import '@xterm/xterm/css/xterm.css'", '')
  .replace("from '../../wailsjs/runtime/runtime'", "from './mock-runtime.mjs'")
  .replace("from '../api/terminal'", "from './mock-api.mjs'")
await writeFile(path.join(outDir, 'TerminalComponent.mjs'), ts.transpileModule(compiled, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText)
await writeFile(path.join(outDir, 'mock-xterm.mjs'), `
export class Terminal {
  static instance
  constructor(options) { Terminal.instance = this; this.options = options; this.cols = 80; this.rows = 24; this.output = [] }
  loadAddon() {} open() {} focus() {} dispose() { this.disposed = true }
  write(data, callback) { this.output.push(Buffer.from(data).toString('utf8')); callback?.() }
  onData(callback) { this.input = callback; return { dispose: () => { this.input = null } } }
}
export class FitAddon { fit() {} }
`)
await writeFile(path.join(outDir, 'mock-runtime.mjs'), `
export const listeners = new Map()
export function EventsOn(name, callback) { listeners.set(name, callback); return () => listeners.delete(name) }
export function emit(name, event) { listeners.get(name)?.(event) }
`)
await writeFile(path.join(outDir, 'mock-api.mjs'), `
export const calls = { start: [], write: [], resize: [], ack: [], stop: [] }
let next = 0
export const terminalAPI = {
  async start(cols, rows) { calls.start.push([cols, rows]); return { sessionId: 'session-' + ++next } },
  async write(id, data) { calls.write.push([id, data]) },
  async resize(id, cols, rows) { calls.resize.push([id, cols, rows]) },
  async acknowledge(id, sequence) { calls.ack.push([id, sequence]) },
  async stop(id) { calls.stop.push(id) },
}
`)

try {
  const [{ default: Component }, { Terminal }, { emit }, { calls }] = await Promise.all([
    import(pathToFileURL(path.join(outDir, 'TerminalComponent.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-xterm.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-runtime.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-api.mjs')).href),
  ])
  const app = createApp(Component, { visible: true })
  app.mount('#app')
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.start[0], [80, 24])
  assert.equal(Terminal.instance.options.scrollback, 2000)
  Terminal.instance.input('日本語')
  await nextTick()
  assert.deepEqual(calls.write, [['session-1', '日本語']])
  emit('terminal:data', { sessionId: 'session-1', data: Buffer.from('こんにちは').toString('base64'), sequence: 1 })
  await nextTick()
  assert.deepEqual(Terminal.instance.output, ['こんにちは'])
  assert.deepEqual(calls.ack, [['session-1', 1]])
  Terminal.instance.cols = 100
  Terminal.instance.rows = 30
  ResizeObserver.instance.callback()
  await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.resize.at(-1), ['session-1', 100, 30])
  emit('terminal:exit', { sessionId: 'session-1', exitCode: 7 })
  await nextTick()
  assert.match(document.body.textContent, /終了コード: 7/)
  document.querySelectorAll('button')[0].click()
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.start[1], [100, 30])
  document.querySelectorAll('button')[1].click()
  await nextTick()
  assert.deepEqual(calls.stop, ['session-2'])
  app.unmount()
  assert.equal(Terminal.instance.disposed, true)
  console.log('terminal component tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
