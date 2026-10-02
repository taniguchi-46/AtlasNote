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
  Element: dom.window.Element, HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement, Event: dom.window.Event,
})
const { createApp, nextTick } = await import('vue')
const root = process.cwd()
const outDir = path.join(root, '.tmp', 'legacy-ai-records-test')
await mkdir(outDir, { recursive: true })
const transpile = (source) => ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText
const flush = async () => { await new Promise(resolve => setTimeout(resolve, 0)); await nextTick() }

try {
  const apiSource = (await readFile(path.join(root, 'src/api/legacyAIRecords.ts'), 'utf8'))
    .replace("from '../../wailsjs/go/app/App'", "from './mock-app.mjs'")
  await writeFile(path.join(outDir, 'records.mjs'), transpile(apiSource))
  await writeFile(path.join(outDir, 'mock-app.mjs'), `
export const calls = []
let artifacts = []
export function setFixtures(writingCount, summaryCount) {
  artifacts = [
    ...Array.from({ length: writingCount }, (_, index) => ({ id: 'writing-' + index, kind: 'document', title: '文書 ' + index, status: 'saved', content: '本文 ' + index, createdAt: '2026-01-01T00:00:00Z' })),
    ...Array.from({ length: summaryCount }, (_, index) => ({ id: 'summary-' + index, kind: 'summary', title: '要約 ' + index, status: 'saved', content: '要約本文 ' + index, createdAt: '2020-01-01T00:00:00Z' })),
  ]
}
export async function ListAIHistories() { calls.push('ListAIHistories'); return { items: [] } }
export async function GetAIHistory(id) { calls.push('GetAIHistory'); return { history: { id, messages: [{ role: 'user', content: '質問' }] } } }
export async function ListAIArtifactsPage(kind, offset) {
  calls.push('ListAIArtifactsPage:' + kind + ':' + offset)
  const filtered = artifacts.filter(item => (kind === 'summary') === (item.kind === 'summary'))
  return { items: filtered.slice(offset, offset + 100), hasNext: filtered.length > offset + 100 }
}
export async function GetAIArtifact(id) { calls.push('GetAIArtifact'); return { artifact: artifacts.find(item => item.id === id) } }
`)
  await writeFile(path.join(outDir, 'mock-settings.mjs'), "import { reactive } from 'vue'; export const settings = reactive({ aiWorkspacePlacement: 'right', isSettingsOpen: true }); export function useSettingsStore() { return settings }")
  await writeFile(path.join(outDir, 'mock-locks.mjs'), 'export function useContentLockStore() { return { locks: [] } }')
  const viewer = await readFile(path.join(root, 'src/components/LegacyAIRecordsPanel.vue'), 'utf8')
  const { descriptor, errors } = parse(viewer)
  assert.deepEqual(errors, [])
  const component = compileScript(descriptor, { id: 'legacy-ai-records-test', inlineTemplate: true }).content
    .replace("from '../api/legacyAIRecords'", "from './records.mjs'")
    .replace("from '../stores/useSettingsStore'", "from './mock-settings.mjs'")
    .replace("from '../stores/useContentLockStore'", "from './mock-locks.mjs'")
  await writeFile(path.join(outDir, 'viewer.mjs'), transpile(component))
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-app.mjs')).href)
  mock.setFixtures(140, 10)
  const { default: Viewer } = await import(pathToFileURL(path.join(outDir, 'viewer.mjs')).href)
  const app = createApp(Viewer)
  app.mount(document.getElementById('app'))
  await flush()
  assert.equal(document.querySelector('.workspace-placement'), null, 'placement is in Terminal settings')
  const writing = () => document.querySelector('[aria-label="旧AI成果物"]')
  const summary = () => document.querySelector('[aria-label="保存済み要約"]')
  const refresh = async () => { document.querySelector('.legacy-records > button').click(); await flush() }
  assert.equal(writing().querySelectorAll('li').length, 100)
  assert.equal(summary().querySelectorAll('li').length, 10, 'summaries after the combined first 100 remain reachable')
  writing().querySelector('button:last-child').click()
  await flush()
  assert.equal(writing().querySelectorAll('li').length, 140)
  assert.equal(new Set([...writing().querySelectorAll('li button')].map(button => button.textContent)).size, 140)
  assert.equal(writing().querySelector('button:last-child')?.textContent?.includes('さらに表示') ?? false, false)
  writing().querySelectorAll('li button')[139].click()
  await flush()
  assert.match(document.querySelector('.legacy-detail').textContent, /本文 139/)
  summary().querySelectorAll('li button')[9].click()
  await flush()
  assert.match(document.querySelector('.legacy-detail').textContent, /要約本文 9/)

  mock.setFixtures(0, 101)
  await refresh()
  assert.equal(summary().querySelectorAll('li').length, 100)
  summary().querySelector('button:last-child').click()
  await flush()
  assert.equal(summary().querySelectorAll('li').length, 101, 'the 101st summary is reachable')
  assert.equal(new Set([...summary().querySelectorAll('li button')].map(button => button.textContent)).size, 101)

  for (const count of [0, 100, 101]) {
    mock.setFixtures(count, 0)
    await refresh()
    assert.equal(writing().querySelectorAll('li').length, Math.min(count, 100))
    assert.equal(summary().querySelectorAll('li').length, 0)
    if (count === 101) {
      writing().querySelector('button:last-child').click()
      await flush()
      assert.equal(writing().querySelectorAll('li').length, 101)
      assert.equal(new Set([...writing().querySelectorAll('li button')].map(button => button.textContent)).size, 101)
    } else {
      assert.equal(writing().textContent.includes('成果物をさらに表示'), false)
    }
  }
  assert.ok(mock.calls.includes('ListAIArtifactsPage:writing:100'))
  assert.ok(mock.calls.includes('ListAIArtifactsPage:summary:0'))
  assert.ok(mock.calls.every(call => /^(ListAIHistories|ListAIArtifactsPage|GetAIHistory|GetAIArtifact)/.test(call)))
  assert.doesNotMatch(viewer, /v-html|generateAISummary|runAIAssistant|runAIWriting|DeleteAI/)
  app.unmount()
  console.log('legacy AI records paging tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
