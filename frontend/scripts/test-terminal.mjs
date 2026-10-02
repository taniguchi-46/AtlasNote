import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'https://atlasnote.test' })
Object.assign(globalThis, {
  window: dom.window, document: dom.window.document, navigator: dom.window.navigator,
  Node: dom.window.Node, Element: dom.window.Element, HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement, Event: dom.window.Event,
  localStorage: dom.window.localStorage,
  requestAnimationFrame: callback => setTimeout(callback, 0), cancelAnimationFrame: clearTimeout,
  ResizeObserver: class { static instance; constructor(callback) { this.callback = callback; ResizeObserver.instance = this } observe() {} disconnect() {} },
})
Object.defineProperties(dom.window.HTMLElement.prototype, {
  clientWidth: { configurable: true, get: () => 640 },
  clientHeight: { configurable: true, get: () => 320 },
})
const { createApp, nextTick, reactive, h } = await import('vue')
const { createPinia, setActivePinia } = await import('pinia')
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
  .replace("from '../stores/useSettingsStore'", "from './settings.mjs'")
  .replace("from '../utils/terminalAppearance'", "from './appearance.mjs'")
const transpile = source => ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText
await writeFile(path.join(outDir, 'appearance.mjs'), transpile(await readFile(path.join(root, 'src/utils/terminalAppearance.ts'), 'utf8')))
await writeFile(path.join(outDir, 'keyboardShortcuts.mjs'), transpile(await readFile(path.join(root, 'src/utils/keyboardShortcuts.ts'), 'utf8')))
await writeFile(path.join(outDir, 'notebookIcons.mjs'), "export const DEFAULT_NOTEBOOK_ICON = 'book'; export const isKnownNotebookIcon = () => true")
const settingsSource = (await readFile(path.join(root, 'src/stores/useSettingsStore.ts'), 'utf8'))
  .replace("from '../utils/notebookIcons'", "from './notebookIcons.mjs'")
  .replace("from '../utils/keyboardShortcuts'", "from './keyboardShortcuts.mjs'")
  .replace("from '../utils/terminalAppearance'", "from './appearance.mjs'")
await writeFile(path.join(outDir, 'settings.mjs'), transpile(settingsSource))
await writeFile(path.join(outDir, 'TerminalComponent.mjs'), ts.transpileModule(compiled, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText)
await writeFile(path.join(outDir, 'mock-xterm.mjs'), `
export class Terminal {
  static instances = []
  constructor(options) { Terminal.instances.push(this); this.options = options; this.cols = 80; this.rows = 24; this.output = [] }
  loadAddon(addon) { addon.activate(this) } open(host) { this.host = host; this.element = host } focus() { this.focused = true } dispose() { this.disposed = true }
  write(data, callback) { this.output.push(Buffer.from(data).toString('utf8')); callback?.() }
  onData(callback) { this.input = callback; return { dispose: () => { this.input = null } } }
}
export class FitAddon {
  static calls = []
  activate(terminal) { this.terminal = terminal }
  fit() {
    if (this.terminal.host.style.display === 'none') throw new Error('fit of a hidden session')
    FitAddon.calls.push(this.terminal)
    this.terminal.cols = Math.floor(1200 / this.terminal.options.fontSize)
    this.terminal.rows = Math.floor(360 / this.terminal.options.fontSize)
  }
}
`)
await writeFile(path.join(outDir, 'mock-runtime.mjs'), `
export const listeners = new Map()
export function EventsOn(name, callback) { listeners.set(name, callback); return () => listeners.delete(name) }
export function emit(name, event) { listeners.get(name)?.(event) }
`)
await writeFile(path.join(outDir, 'mock-api.mjs'), `
export const calls = { start: [], write: [], resize: [], ack: [], stop: [] }
let next = 0
const stops = new Map()
export const control = { beforeStart: null, startGate: null, failStop: false, writeGate: null }
export function resolveStop(id) { stops.get(id)?.(); stops.delete(id) }
export const terminalAPI = {
  async start(cols, rows) {
    calls.start.push([cols, rows]); const id = 'session-' + ++next
    control.beforeStart?.(id)
    if (control.startGate) await control.startGate
    return { sessionId: id }
  },
  async write(id, data) { calls.write.push([id, data]); if (control.writeGate?.id === id) await control.writeGate.promise },
  async resize(id, cols, rows) { calls.resize.push([id, cols, rows]) },
  async acknowledge(id, sequence) { calls.ack.push([id, sequence]) },
  async stop(id) {
    calls.stop.push(id)
    if (control.failStop) { control.failStop = false; throw new Error('stop failure') }
    await new Promise(resolve => { stops.set(id, resolve) })
  },
}
`)

try {
  const [{ default: Component }, { Terminal, FitAddon }, { emit }, { calls, resolveStop, control }, { useSettingsStore }, { TERMINAL_THEMES }] = await Promise.all([
    import(pathToFileURL(path.join(outDir, 'TerminalComponent.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-xterm.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-runtime.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'mock-api.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'settings.mjs')).href),
    import(pathToFileURL(path.join(outDir, 'appearance.mjs')).href),
  ])
  setActivePinia(createPinia())
  let settings = useSettingsStore()
  assert.equal(settings.terminalFontSize, 14)
  assert.equal(settings.terminalTheme, 'atlas-dark')
  const toolbarTarget = document.createElement('div')
  document.body.append(toolbarTarget)
  const props = reactive({ visible: true, toolbarTarget })
  const app = createApp({ render: () => h(Component, props) })
  const settle = async () => { await nextTick(); await new Promise(resolve => setTimeout(resolve, 20)) }
  const tabs = () => [...document.querySelectorAll('.terminal-session-tab')]
  const buttons = () => toolbarTarget.querySelectorAll('button')
  const output = (id, text, sequence = 1) => emit('terminal:data', { sessionId: id, data: Buffer.from(text).toString('base64'), sequence })
  app.mount('#app')
  await settle()
  const first = Terminal.instances[0]
  assert.deepEqual(calls.start[0], [80, 24])
  assert.deepEqual(calls.resize[0], ['session-1', 85, 25])
  assert.equal(buttons()[0].disabled, false, 'plus stays available with a live session')
  assert.equal(first.options.scrollback, 2000)
  for (let i = 0; i < 2; i++) { buttons()[0].click(); await settle() }
  const [a, b, c] = Terminal.instances
  assert.equal(calls.start.length, 3)
  assert.deepEqual(tabs().map(tab => tab.textContent.trim()), ['shell 1', 'shell 2', 'shell 3'])
  assert.equal(tabs()[2].getAttribute('aria-selected'), 'true')
  output('session-1', 'A 日本語')
  output('session-2', 'B buffer')
  output('session-3', 'C buffer')
  output('unknown', 'ignored')
  emit('terminal:exit', { sessionId: 'unknown', exitCode: 99 })
  await settle()
  assert.deepEqual([a.output, b.output, c.output], [['A 日本語'], ['B buffer'], ['C buffer']])
  assert.deepEqual(calls.ack, [['session-1', 1], ['session-2', 1], ['session-3', 1]])
  const fitCount = FitAddon.calls.length
  tabs()[0].click(); await settle()
  assert.equal(tabs()[0].getAttribute('aria-selected'), 'true')
  assert.equal(a.host.style.display, '')
  assert.equal(b.host.style.display, 'none')
  assert.equal(c.host.style.display, 'none')
  assert.deepEqual(FitAddon.calls.slice(fitCount), [a])
  assert.deepEqual(c.output, ['C buffer'], 'switch keeps the other buffer')
  a.input('日本語'); await settle()
  assert.deepEqual(calls.write.at(-1), ['session-1', '日本語'])
  let unblockWrite
  control.writeGate = { id: 'session-1', promise: new Promise(resolve => { unblockWrite = resolve }) }
  a.input('blocked A'); await settle()
  tabs()[1].click(); await settle()
  b.input('independent B'); await settle()
  assert.deepEqual(calls.write.at(-1), ['session-2', 'independent B'])
  unblockWrite(); control.writeGate = null
  tabs()[0].click(); await settle()
  for (const [id, preset] of Object.entries(TERMINAL_THEMES)) {
    settings.terminalTheme = id; await nextTick()
    for (const terminal of [a, b, c]) assert.deepEqual(terminal.options.theme, preset.theme)
    for (const color of Object.values(preset.theme)) assert.match(color, /^#[0-9a-f]{6}$/i)
    assert.equal(document.querySelector('.terminal-panel').style.getPropertyValue('--terminal-background'), preset.theme.background)
  }
  for (const font of ['Consolas', 'Cascadia Mono', 'monospace', 'default']) {
    const count = calls.resize.length
    settings.terminalFont = font; await settle()
    for (const terminal of [a, b, c]) {
      assert.match(terminal.options.fontFamily, /monospace/)
      if (font !== 'default') assert.ok(terminal.options.fontFamily.includes(font))
    }
    assert.equal(calls.resize.length, count + 1)
    assert.equal(calls.resize.at(-1)[0], 'session-1')
    assert.equal(localStorage.getItem('atlas-terminal-font-family'), font)
  }
  settings.terminalFontSize = 20
  settings.terminalCursorStyle = 'bar'
  settings.terminalCursorBlink = false
  settings.terminalScrollback = 10000
  const appearanceFits = FitAddon.calls.length
  await settle()
  for (const terminal of [a, b, c]) {
    assert.equal(terminal.options.fontSize, 20)
    assert.equal(terminal.options.cursorStyle, 'bar')
    assert.equal(terminal.options.cursorBlink, false)
    assert.equal(terminal.options.scrollback, 10000)
  }
  assert.deepEqual(FitAddon.calls.slice(appearanceFits), [a])
  assert.deepEqual(calls.resize.at(-1), ['session-1', 60, 18])
  assert.equal(localStorage.getItem('atlas-terminal-font-size'), '20')
  assert.equal(calls.start.length, 3, 'settings never restart PTYs')
  const beforeHide = [calls.start.length, calls.stop.length, calls.resize.length, FitAddon.calls.length]
  props.visible = false; await settle()
  settings.terminalFontSize = 18; await settle()
  ResizeObserver.instance.callback(); await settle()
  output('session-2', 'hidden B', 2)
  assert.equal(toolbarTarget.querySelector('.terminal-toolbar').style.display, 'none')
  assert.deepEqual([calls.start.length, calls.stop.length, calls.resize.length, FitAddon.calls.length], beforeHide)
  props.visible = true; await settle()
  assert.deepEqual(calls.resize.at(-1), ['session-1', 66, 20])
  tabs()[1].click(); await settle()
  assert.deepEqual(b.output, ['B buffer', 'hidden B'])
  const resizeCount = calls.resize.length
  settings.terminalFontSize = 12
  await nextTick(); ResizeObserver.instance.callback(); await settle()
  assert.equal(calls.resize.length, resizeCount + 1)
  assert.deepEqual(calls.resize.at(-1), ['session-2', 100, 30])
  buttons()[1].click(); await nextTick()
  assert.deepEqual(calls.stop, ['session-2'])
  assert.equal(buttons()[1].disabled, true)
  buttons()[1].click()
  assert.equal(calls.stop.length, 1, 'no duplicate Stop')
  emit('terminal:exit', { sessionId: 'session-2', exitCode: 0, stopped: true })
  resolveStop('session-2'); await settle()
  assert.equal(b.disposed, true)
  assert.deepEqual(tabs().map(tab => tab.textContent.trim()), ['shell 1', 'shell 3'])
  assert.equal(tabs()[1].getAttribute('aria-selected'), 'true', 'active deletion selects right neighbor')
  assert.equal(a.disposed, undefined)
  assert.equal(c.disposed, undefined)
  output('session-2', 'late output', 3)
  emit('terminal:exit', { sessionId: 'session-2', exitCode: 9 })
  assert.deepEqual(c.output, ['C buffer'])
  emit('terminal:exit', { sessionId: 'session-3', exitCode: 7 }); await settle()
  assert.match(tabs()[1].textContent, /終了: 7/)
  assert.match(document.querySelector('.terminal-status').textContent, /終了コード: 7/)
  assert.equal(c.input, null)
  assert.deepEqual(c.output, ['C buffer'], 'natural exit preserves buffer')
  assert.equal(buttons()[1].disabled, false, 'ended tab can be closed')
  output('session-3', 'post-exit output', 2)
  emit('terminal:exit', { sessionId: 'session-3', exitCode: 88 })
  assert.deepEqual(c.output, ['C buffer'])
  await settle(); assert.match(tabs()[1].textContent, /終了: 7/, 'duplicate exit is ignored')
  buttons()[0].click(); await settle()
  const d = Terminal.instances[3]
  assert.match(tabs()[2].textContent, /shell 4/)
  assert.equal(d.options.fontSize, 12)
  assert.deepEqual(d.options.theme, TERMINAL_THEMES['high-contrast'].theme)
  assert.equal(d.options.cursorBlink, false)
  assert.equal(d.options.scrollback, 10000)
  tabs()[1].click(); await settle()
  const stopsBeforeEnded = calls.stop.length
  buttons()[1].click(); await settle()
  assert.equal(calls.stop.length, stopsBeforeEnded, 'closing ended tab does not stop another PTY')
  assert.equal(c.disposed, true)
  assert.equal(tabs()[1].getAttribute('aria-selected'), 'true')
  control.failStop = true
  buttons()[1].click(); await settle()
  assert.match(document.querySelector('.terminal-error').textContent, /停止に失敗/)
  assert.equal(d.disposed, undefined, 'failed Stop retains tab and buffer')
  buttons()[1].click(); await settle()
  tabs()[0].click(); await settle()
  resolveStop('session-4'); await settle()
  assert.equal(tabs()[0].getAttribute('aria-selected'), 'true', 'delayed Stop does not steal active selection')
  assert.equal(d.disposed, true)
  buttons()[1].click(); await settle(); resolveStop('session-1'); await settle()
  assert.equal(tabs().length, 0)
  assert.match(document.querySelector('.terminal-empty').textContent, /＋/)
  assert.equal(buttons()[1].disabled, true)
  const startsBeforeReopen = calls.start.length
  props.visible = false; await settle(); props.visible = true; await settle()
  assert.equal(calls.start.length, startsBeforeReopen, 'reopening empty panel does not auto-create')
  // Unknown early events replay only into the ID returned by StartTerminal.
  let releaseStart
  control.startGate = new Promise(resolve => { releaseStart = resolve })
  control.beforeStart = id => {
    output('unknown-early', 'ignored')
    output('session-2', 'stale closed')
    output(id, 'early buffer')
    emit('terminal:exit', { sessionId: id, exitCode: 5 })
  }
  buttons()[0].click(); await settle()
  assert.equal(buttons()[0].disabled, true)
  settings.terminalFontSize = 18; await settle()
  releaseStart(); await settle()
  const e = Terminal.instances[4]
  assert.deepEqual(e.output, ['early buffer'])
  assert.deepEqual(calls.ack.at(-1), ['session-5', 1])
  assert.match(tabs()[0].textContent, /shell 5.*終了: 5/)
  assert.equal(e.options.fontSize, 18, 'pending session gets current settings')
  buttons()[1].click(); await settle()
  control.startGate = null
  control.beforeStart = null
  buttons()[0].click(); await settle()
  const f = Terminal.instances[5]
  assert.match(tabs()[0].textContent, /shell 6/)
  for (const button of document.querySelectorAll('.terminal-toolbar button')) {
    assert.ok(button.title); assert.ok(button.getAttribute('aria-label'))
  }
  app.unmount()
  assert.equal(f.disposed, true)
  assert.equal(calls.stop.at(-1), 'session-6')
  resolveStop('session-6')
  setActivePinia(createPinia())
  settings = useSettingsStore()
  const restoredProps = reactive({ visible: false })
  const restoredApp = createApp({ render: () => h(Component, restoredProps) })
  restoredApp.mount('#app'); await settle()
  const previousStarts = calls.start.length
  assert.equal(tabs().length, 0, 'hidden mount does not start a session')
  restoredProps.visible = true; await settle()
  assert.equal(calls.start.length, previousStarts + 1)
  const restored = Terminal.instances[6]
  assert.equal(restored.options.fontSize, 18)
  assert.equal(restored.options.cursorBlink, false)
  assert.equal(restored.options.scrollback, 10000)
  assert.deepEqual(restored.options.theme, TERMINAL_THEMES['high-contrast'].theme)
  restoredApp.unmount(); resolveStop('session-7')
  // Unmount during Start must collect the returned backend session.
  control.startGate = new Promise(resolve => { releaseStart = resolve })
  const pendingApp = createApp(Component, { visible: true })
  pendingApp.mount('#app'); await settle()
  pendingApp.unmount(); releaseStart(); await settle()
  assert.equal(calls.stop.at(-1), 'session-8')
  resolveStop('session-8'); await settle()
  assert.equal(Terminal.instances[7].disposed, true)
  control.startGate = null
  // Complete Start while the panel is hidden: buffer first, renderer on reveal.
  const hiddenProps = reactive({ visible: true })
  control.startGate = new Promise(resolve => { releaseStart = resolve })
  const hiddenApp = createApp({ render: () => h(Component, hiddenProps) })
  hiddenApp.mount('#app'); await settle()
  hiddenProps.visible = false; await settle()
  const fitsBeforeHiddenStart = FitAddon.calls.length
  releaseStart(); await settle()
  const hidden = Terminal.instances[8]
  assert.equal(hidden.element, undefined, 'hidden startup defers renderer creation')
  assert.equal(FitAddon.calls.length, fitsBeforeHiddenStart)
  output('session-9', 'before renderer'); await settle()
  assert.deepEqual(hidden.output, ['before renderer'])
  hiddenProps.visible = true; await settle()
  assert.ok(hidden.element)
  assert.equal(FitAddon.calls.at(-1), hidden)
  hiddenApp.unmount(); resolveStop('session-9')
  control.startGate = null
  for (const key of ['font-family', 'cursor-style', 'cursor-blink', 'scrollback']) localStorage.setItem('atlas-terminal-' + key, 'invalid')
  localStorage.setItem('atlas-terminal-font-size', 'bad')
  localStorage.setItem('atlas-terminal-theme', 'unknown')
  setActivePinia(createPinia())
  settings = useSettingsStore()
  assert.equal(settings.terminalFontSize, 14)
  assert.equal(settings.terminalTheme, 'atlas-dark')
  assert.equal(settings.terminalFont, 'default')
  assert.equal(settings.terminalCursorStyle, 'block')
  assert.equal(settings.terminalCursorBlink, true)
  assert.equal(settings.terminalScrollback, 2000)
  localStorage.removeItem('atlas-ai-workspace-placement')
  setActivePinia(createPinia())
  settings = useSettingsStore()
  assert.equal(settings.aiWorkspacePlacement, 'bottom')
  localStorage.setItem('atlas-ai-workspace-placement', 'right')
  setActivePinia(createPinia())
  settings = useSettingsStore()
  assert.equal(settings.aiWorkspacePlacement, 'right', 'saved right placement is retained')
  const settingsPanel = await readFile(path.join(root, 'src/components/TerminalSettingsPanel.vue'), 'utf8')
  const panelScript = compileScript(parse(settingsPanel).descriptor, { id: 'terminal-settings-test', inlineTemplate: true }).content
    .replace("from '../stores/useSettingsStore'", "from './settings.mjs'")
    .replace("from '../utils/terminalAppearance'", "from './appearance.mjs'")
  await writeFile(path.join(outDir, 'panel.mjs'), transpile(panelScript))
  const { default: Panel } = await import(pathToFileURL(path.join(outDir, 'panel.mjs')).href)
  const panelApp = createApp(Panel)
  panelApp.mount('#app')
  const placement = document.querySelector('[aria-label="パネルの表示位置"]')
  assert.equal(placement.querySelectorAll('button')[1].getAttribute('aria-pressed'), 'true')
  placement.querySelector('button').click()
  await nextTick()
  assert.equal(settings.aiWorkspacePlacement, 'bottom')
  assert.equal(localStorage.getItem('atlas-ai-workspace-placement'), 'bottom')
  assert.equal(placement.querySelector('select'), null)
  const sizes = document.querySelector('[aria-label="ターミナルの文字サイズ"]')
  sizes.querySelectorAll('button')[6].click()
  await nextTick()
  assert.equal(settings.terminalFontSize, 20)
  const themes = document.querySelector('[aria-label="ターミナルの配色"]')
  themes.querySelectorAll('button')[2].click()
  await nextTick()
  assert.equal(settings.terminalTheme, 'light')
  assert.equal(themes.querySelectorAll('button')[2].getAttribute('aria-pressed'), 'true')
  for (const [selector, value, field] of [['#terminal-font', 'Cascadia Mono', 'terminalFont'], ['#terminal-cursor', 'bar', 'terminalCursorStyle'], ['#terminal-scrollback', '5000', 'terminalScrollback']]) {
    const control = document.querySelector(selector)
    control.value = value
    control.dispatchEvent(new Event('change', { bubbles: true }))
    await nextTick()
    assert.equal(String(settings[field]), value)
  }
  document.querySelector('input[type="checkbox"]').click()
  await nextTick()
  assert.equal(settings.terminalCursorBlink, false)
  panelApp.unmount()
  console.log('terminal component tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
