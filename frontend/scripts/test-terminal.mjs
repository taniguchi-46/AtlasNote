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
  static instance
  constructor(options) { Terminal.instance = this; this.options = options; this.cols = 80; this.rows = 24; this.output = [] }
  loadAddon() {} open() {} focus() {} dispose() { this.disposed = true }
  write(data, callback) { this.output.push(Buffer.from(data).toString('utf8')); callback?.() }
  onData(callback) { this.input = callback; return { dispose: () => { this.input = null } } }
}
export class FitAddon {
  static calls = []
  fit() { FitAddon.calls.push(Terminal.instance.options.fontSize); Terminal.instance.cols = Math.floor(1200 / Terminal.instance.options.fontSize); Terminal.instance.rows = Math.floor(360 / Terminal.instance.options.fontSize) }
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
let finishStop
export function resolveStop() { finishStop?.() }
export const terminalAPI = {
  async start(cols, rows) { calls.start.push([cols, rows]); return { sessionId: 'session-' + ++next } },
  async write(id, data) { calls.write.push([id, data]) },
  async resize(id, cols, rows) { calls.resize.push([id, cols, rows]) },
  async acknowledge(id, sequence) { calls.ack.push([id, sequence]) },
  async stop(id) { calls.stop.push(id); await new Promise(resolve => { finishStop = resolve }) },
}
`)

try {
  const [{ default: Component }, { Terminal, FitAddon }, { emit }, { calls, resolveStop }, { useSettingsStore }, { TERMINAL_THEMES }] = await Promise.all([
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
  app.mount('#app')
  await nextTick()
  assert.equal(toolbarTarget.querySelectorAll('button')[0].disabled, true, 'start is disabled during automatic startup')
  assert.equal(toolbarTarget.querySelectorAll('button')[1].disabled, true, 'stop is disabled before session creation')
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.start[0], [85, 25])
  assert.equal(Terminal.instance.options.scrollback, 2000)
  const buttons = () => toolbarTarget.querySelectorAll('button')
  assert.equal(buttons()[0].disabled, true)
  assert.equal(buttons()[1].disabled, false)
  assert.equal(buttons()[0].getAttribute('aria-label'), '新しいターミナル')
  assert.equal(buttons()[0].textContent, '')
  settings.terminalFontSize = 20
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.equal(Terminal.instance.options.fontSize, 20)
  assert.equal(FitAddon.calls.at(-1), 20)
  assert.deepEqual(calls.resize.at(-1), ['session-1', 60, 18])
  assert.equal(localStorage.getItem('atlas-terminal-font-size'), '20')
  for (const [id, preset] of Object.entries(TERMINAL_THEMES)) {
    settings.terminalTheme = id
    await nextTick()
    assert.deepEqual(Terminal.instance.options.theme, preset.theme)
    for (const key of ['background', 'foreground', 'cursor', 'cursorAccent', 'selectionBackground', 'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white', 'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite']) assert.match(preset.theme[key], /^#[0-9a-f]{6}$/i)
    assert.equal(document.querySelector('.terminal-panel').style.getPropertyValue('--terminal-background'), preset.theme.background)
  }
  assert.equal(document.querySelectorAll('.terminal-session-tab').length, 1)
  assert.match(document.querySelector('.terminal-session-tab').textContent, /shell 1/)
  for (const button of document.querySelectorAll('.terminal-toolbar button')) {
    assert.ok(button.title)
    assert.ok(button.getAttribute('aria-label'))
  }
  for (const font of ['Consolas', 'Cascadia Mono', 'monospace', 'default']) {
    const resizeCount = calls.resize.length
    settings.terminalFont = font
    await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
    assert.match(Terminal.instance.options.fontFamily, /monospace/)
    if (font !== 'default') assert.ok(Terminal.instance.options.fontFamily.includes(font))
    assert.equal(calls.resize.length, resizeCount + 1, 'font changes fit then resize same session')
    assert.equal(localStorage.getItem('atlas-terminal-font-family'), font)
  }
  for (const style of ['bar', 'underline', 'block']) {
    settings.terminalCursorStyle = style
    await nextTick()
    assert.equal(Terminal.instance.options.cursorStyle, style)
    assert.equal(localStorage.getItem('atlas-terminal-cursor-style'), style)
  }
  settings.terminalCursorBlink = false
  settings.terminalScrollback = 10000
  await nextTick()
  assert.equal(Terminal.instance.options.cursorBlink, false)
  assert.equal(Terminal.instance.options.scrollback, 10000)
  assert.equal(localStorage.getItem('atlas-terminal-cursor-blink'), 'false')
  assert.equal(localStorage.getItem('atlas-terminal-scrollback'), '10000')
  assert.equal(calls.start.length, 1, 'appearance does not restart the PTY')
  props.visible = false
  await nextTick()
  assert.equal(toolbarTarget.querySelector('.terminal-toolbar').style.display, 'none')
  const resizeCount = calls.resize.length
  settings.terminalFontSize = 18
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.equal(calls.resize.length, resizeCount, 'hidden terminal fits when shown')
  props.visible = true
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.resize.at(-1), ['session-1', 66, 20])
  Terminal.instance.input('日本語')
  await nextTick()
  assert.deepEqual(calls.write, [['session-1', '日本語']])
  emit('terminal:data', { sessionId: 'session-1', data: Buffer.from('こんにちは').toString('base64'), sequence: 1 })
  await nextTick()
  assert.deepEqual(Terminal.instance.output, ['こんにちは'])
  assert.deepEqual(calls.ack, [['session-1', 1]])
  settings.terminalFontSize = 12
  await nextTick()
  ResizeObserver.instance.callback()
  await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.resize.at(-1), ['session-1', 100, 30])
  emit('terminal:exit', { sessionId: 'session-1', exitCode: 7 })
  await nextTick()
  assert.match(document.body.textContent, /終了コード: 7/)
  assert.equal(buttons()[0].getAttribute('aria-label'), 'ターミナルを新規作成・再起動')
  assert.equal(buttons()[0].disabled, false)
  assert.equal(buttons()[1].disabled, true)
  toolbarTarget.querySelectorAll('button')[0].click()
  await nextTick(); await new Promise(resolve => setTimeout(resolve, 20))
  assert.deepEqual(calls.start[1], [100, 30])
  toolbarTarget.querySelectorAll('button')[1].click()
  await nextTick()
  assert.deepEqual(calls.stop, ['session-2'])
  assert.equal(buttons()[1].disabled, true, 'stop is disabled during a pending request')
  buttons()[1].click()
  assert.equal(calls.stop.length, 1, 'cannot send duplicate stop requests')
  emit('terminal:exit', { sessionId: 'session-2', exitCode: 0, stopped: true })
  resolveStop()
  await nextTick()
  assert.equal(buttons()[1].disabled, true, 'stop remains disabled after exit')
  app.unmount()
  assert.equal(Terminal.instance.disposed, true)
  setActivePinia(createPinia())
  settings = useSettingsStore()
  assert.equal(settings.terminalFontSize, 12)
  assert.equal(settings.terminalTheme, 'high-contrast')
  const restoredApp = createApp(Component, { visible: false })
  restoredApp.mount('#app')
  assert.equal(Terminal.instance.options.cursorBlink, false)
  assert.equal(Terminal.instance.options.scrollback, 10000)
  assert.equal(Terminal.instance.options.fontSize, 12, 'new terminal uses persisted font size')
  assert.deepEqual(Terminal.instance.options.theme, TERMINAL_THEMES['high-contrast'].theme)
  restoredApp.unmount()
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
