import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { JSDOM } from 'jsdom'
import ts from 'typescript'
import { compileScript, parse } from 'vue/compiler-sfc'

const rootDir = process.cwd()
const componentPath = path.join(rootDir, 'src', 'components', 'ContentLockControls.vue')
const outDir = path.join(rootDir, '.tmp', 'content-lock-controls-test')
const componentOutFile = path.join(outDir, 'ContentLockControls.mjs')
const storeOutFile = path.join(outDir, 'mock-content-lock-store.mjs')

await mkdir(outDir, { recursive: true })

const componentSource = await readFile(componentPath, 'utf8')
const { descriptor, errors } = parse(componentSource, { filename: componentPath })
assert.deepEqual(errors, [], 'ContentLockControls.vue must parse without errors')

const compiled = compileScript(descriptor, {
  id: 'content-lock-controls-test',
  inlineTemplate: true,
})
const testableSource = compiled.content.replace(
  "from '../stores/useContentLockStore'",
  "from './mock-content-lock-store.mjs'",
)
const transpiled = ts.transpileModule(testableSource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})
await writeFile(componentOutFile, transpiled.outputText, 'utf8')
await writeFile(storeOutFile, `
import { reactive } from 'vue'

export const calls = {
  status: [],
  enable: [],
  unlock: [],
  lockNow: [],
  change: [],
  disable: [],
}

let failStatus = false
let configuredStatuses = {}

export const store = reactive({
  statuses: {},
  statusLoading: {},
  statusErrors: {},
  isBusy: false,
  async refreshTarget(target) {
    const key = target.type + ':' + target.id
    calls.status.push(structuredClone(target))
    store.statusLoading[key] = true
    delete store.statusErrors[key]
    await Promise.resolve()
    if (failStatus) {
      store.statusErrors[key] = {
        code: 'CONTENT_LOCK_UNAVAILABLE',
        message: 'ロック状態を取得できませんでした。',
      }
      store.statusLoading[key] = false
      return null
    }
    const status = structuredClone(configuredStatuses[key] ?? {
      protected: false,
      locked: false,
      explicitLock: false,
    })
    store.statuses[key] = status
    store.statusLoading[key] = false
    return status
  },
  async enable(target, passphrase, deleteAIRecords) {
    calls.enable.push({ target: structuredClone(target), passphrase, deleteAIRecords })
    const key = target.type + ':' + target.id
    const status = { protected: true, locked: false, explicitLock: true, source: target.type }
    configuredStatuses[key] = status
    store.statuses[key] = structuredClone(status)
    return { removed: false, unlocked: true, restartRequired: false }
  },
  async unlock(target, passphrase) {
    calls.unlock.push({ target: structuredClone(target), passphrase })
    return { removed: false, unlocked: true, restartRequired: false }
  },
  async lockNow(target) {
    calls.lockNow.push(structuredClone(target))
    return { removed: false, unlocked: false, restartRequired: false }
  },
  async changePassphrase(target, currentPassphrase, passphrase) {
    calls.change.push({ target: structuredClone(target), currentPassphrase, passphrase })
    return { removed: false, unlocked: true, restartRequired: false }
  },
  async disable(target, passphrase) {
    calls.disable.push({ target: structuredClone(target), passphrase })
    const key = target.type + ':' + target.id
    const status = { protected: false, locked: false, explicitLock: false }
    configuredStatuses[key] = status
    store.statuses[key] = structuredClone(status)
    return { removed: true, unlocked: false, restartRequired: false }
  },
})

export function useContentLockStore() {
  return store
}

export function resetMock() {
  for (const values of Object.values(calls)) values.length = 0
  failStatus = false
  configuredStatuses = {}
  store.statuses = {}
  store.statusLoading = {}
  store.statusErrors = {}
  store.isBusy = false
}

export function setStatusFailure(value) {
  failStatus = value
}

export function setConfiguredStatus(target, status) {
  configuredStatuses[target.type + ':' + target.id] = structuredClone(status)
}
`, 'utf8')

const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'https://atlasnote.local/',
})
const domGlobals = {
  window: dom.window,
  document: dom.window.document,
  Node: dom.window.Node,
  Element: dom.window.Element,
  HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  CustomEvent: dom.window.CustomEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle,
}
Object.assign(globalThis, domGlobals)
Object.defineProperty(globalThis, 'navigator', {
  configurable: true,
  value: dom.window.navigator,
})
dom.window.confirm = () => true

let activeMount = null

try {
  const { createApp, defineComponent, h, nextTick, ref } = await import('vue')
  const mock = await import(pathToFileURL(storeOutFile).href)
  const { default: ContentLockControls } = await import(pathToFileURL(componentOutFile).href)

  async function flushUI() {
    for (let index = 0; index < 4; index += 1) {
      await Promise.resolve()
      await nextTick()
    }
  }

  async function mountEditor(target) {
    const controls = ref(null)
    const saveResult = ref(null)
    const container = document.createElement('div')
    document.body.append(container)
    const Host = defineComponent({
      setup() {
        async function save(event) {
          event.preventDefault()
          saveResult.value = await controls.value.save()
        }
        return () => h('form', { onSubmit: save }, [
          h(ContentLockControls, {
            ref: controls,
            target,
            targetLabel: target.type === 'note' ? 'テストノート' : 'テストノートブック',
            deferSave: true,
          }),
          h('button', { class: 'shared-save', type: 'submit' }, '保存'),
        ])
      },
    })
    const app = createApp(Host)
    app.mount(container)
    activeMount = { app, container }
    await flushUI()
    return { app, container, controls, saveResult }
  }

  function unmountEditor(mounted) {
    mounted.app.unmount()
    mounted.container.remove()
    activeMount = null
  }

  function setInputValue(input, value) {
    input.value = value
    input.dispatchEvent(new dom.window.Event('input', { bubbles: true }))
  }

  mock.resetMock()
  mock.setStatusFailure(true)
  const failed = await mountEditor({ type: 'note', id: 'a'.repeat(32) })
  let toggle = failed.container.querySelector('[role="switch"]')
  assert.equal(toggle.disabled, true, 'toggle must wait for a successful status load')
  assert.match(failed.container.textContent, /ロック状態を取得できませんでした/, 'status failure must be visible')
  const retry = [...failed.container.querySelectorAll('button')].find((button) => button.textContent.includes('再試行'))
  assert.ok(retry, 'status failure must expose a retry action')
  mock.setStatusFailure(false)
  retry.click()
  await flushUI()
  toggle = failed.container.querySelector('[role="switch"]')
  assert.equal(toggle.disabled, false, 'retry must make the toggle interactive after recovery')
  unmountEditor(failed)

  for (const target of [
    { type: 'note', id: 'b'.repeat(32) },
    { type: 'notebook', id: 'c'.repeat(32) },
  ]) {
    mock.resetMock()
    const mounted = await mountEditor(target)
    const editorToggle = mounted.container.querySelector('[role="switch"]')
    assert.equal(editorToggle.getAttribute('aria-checked'), 'false')
    editorToggle.click()
    await flushUI()
    assert.equal(editorToggle.getAttribute('aria-checked'), 'true', target.type + ' toggle must turn on')
    let passphraseInputs = mounted.container.querySelectorAll('input[type="password"]')
    assert.equal(passphraseInputs.length, 2, target.type + ' toggle must reveal passphrase fields')
    editorToggle.click()
    await flushUI()
    assert.equal(editorToggle.getAttribute('aria-checked'), 'false', target.type + ' toggle must cancel a staged lock')
    assert.equal(mounted.container.querySelectorAll('input[type="password"]').length, 0)
    editorToggle.click()
    await flushUI()
    passphraseInputs = mounted.container.querySelectorAll('input[type="password"]')
    setInputValue(passphraseInputs[0], 'correct horse battery staple')
    setInputValue(passphraseInputs[1], 'correct horse battery staple')
    mounted.container.querySelector('form').dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }))
    await flushUI()
    assert.equal(mounted.saveResult.value, true, target.type + ' shared save must succeed')
    assert.deepEqual(mock.calls.enable.at(-1).target, target, target.type + ' save must use the displayed target')
    assert.equal(editorToggle.getAttribute('aria-checked'), 'true', target.type + ' must remain configured after save')
    unmountEditor(mounted)
    const reopened = await mountEditor(target)
    assert.equal(
      reopened.container.querySelector('[role="switch"]').getAttribute('aria-checked'),
      'true',
      target.type + ' must restore its configured state when the editor is reopened',
    )
    unmountEditor(reopened)
  }

  mock.resetMock()
  const notebookTarget = { type: 'notebook', id: 'd'.repeat(32) }
  mock.setConfiguredStatus(notebookTarget, {
    protected: true,
    locked: false,
    explicitLock: true,
    source: 'notebook',
  })
  const configured = await mountEditor(notebookTarget)
  const configuredToggle = configured.container.querySelector('[role="switch"]')
  assert.equal(configuredToggle.getAttribute('aria-checked'), 'true')
  configuredToggle.click()
  await flushUI()
  assert.equal(configuredToggle.getAttribute('aria-checked'), 'false', 'configured toggle must stage disable')
  const currentPassphrase = configured.container.querySelector('input[type="password"]')
  assert.ok(currentPassphrase, 'disabling a lock must request the current passphrase')
  setInputValue(currentPassphrase, 'correct horse battery staple')
  configured.container.querySelector('form').dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }))
  await flushUI()
  assert.equal(configured.saveResult.value, true)
  assert.deepEqual(mock.calls.disable.at(-1).target, notebookTarget)
  assert.equal(configuredToggle.getAttribute('aria-checked'), 'false')
  unmountEditor(configured)

  console.log('content lock controls tests passed')
} finally {
  if (activeMount) {
    activeMount.app.unmount()
    activeMount.container.remove()
  }
  dom.window.close()
  await rm(outDir, { recursive: true, force: true })
}
