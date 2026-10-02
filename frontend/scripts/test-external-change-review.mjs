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
})
const { createApp, nextTick } = await import('vue')
const root = process.cwd()
const outDir = path.join(root, '.tmp', 'external-change-review-test')
await mkdir(outDir, { recursive: true })
const source = await readFile(path.join(root, 'src/components/ExternalChangeReview.vue'), 'utf8')
const { descriptor } = parse(source)
const compiled = compileScript(descriptor, { id: 'external-change-review-test', inlineTemplate: true }).content
  .replace("from '../api/externalChanges'", "from './mock-changes.mjs'")
  .replace("from '../stores/useNoteStore'", "from './mock-note-store.mjs'")
  .replace("from '../stores/useTagStore'", "from './mock-tag-store.mjs'")
await writeFile(path.join(outDir, 'ExternalChangeReview.mjs'), ts.transpileModule(compiled, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText)
await writeFile(path.join(outDir, 'mock-changes.mjs'), `
export let approvals = 0
export const review = {
  operationId: 'operation', kind: 'notes.request_update', storageSpaceId: 'space', targetIds: ['note'],
  impactCount: 1, createdAt: new Date().toISOString(), expiresAt: new Date(Date.now()+60000).toISOString(),
  updatedAt: new Date().toISOString(), state: 'pending_approval',
  items: [{ noteId: 'note', noteTitle: '対象ノート', before: { title: '変更前' }, after: { title: '変更後' }, reason: '確認理由', status: 'pending_approval' }],
}
export async function listExternalChangeReviews() { return [structuredClone(review)] }
export async function approveExternalChange() { approvals++; review.state = 'applied'; return { state: 'applied' } }
export async function rejectExternalChange() { review.state = 'rejected'; return { state: 'rejected' } }
`)
await writeFile(path.join(outDir, 'mock-note-store.mjs'), `
export let draft = '未保存の入力'
export let failFlush = true
export let runs = 0
export function allowFlush() { failFlush = false }
export function useNoteStore() { return {
  async runExternalChangeOperation(noteIds, operation) {
    runs++
    if (failFlush) return null
    draft = null
    return operation()
  },
  async fetchNotes() {},
} }
`)
await writeFile(path.join(outDir, 'mock-tag-store.mjs'), `
export function useTagStore() { return { async refreshNoteTagsIfActive() {} } }
`)

try {
  const api = await import(pathToFileURL(path.join(outDir, 'mock-changes.mjs')).href)
  const notes = await import(pathToFileURL(path.join(outDir, 'mock-note-store.mjs')).href)
  const component = (await import(pathToFileURL(path.join(outDir, 'ExternalChangeReview.mjs')).href)).default
  const app = createApp(component)
  const settle = async () => { await new Promise((resolve) => setTimeout(resolve, 0)); await nextTick() }
  const assertStates = (cardLabel, itemLabels) => {
    assert.equal(document.querySelector('.external-review-card > header > span').textContent, cardLabel)
    assert.deepEqual(
      [...document.querySelectorAll('.external-review-item > p:first-child')].map((item) => item.textContent),
      itemLabels.map((label) => `項目の状態: ${label}`),
    )
  }
  app.mount('#app')
  try {
    await settle()
    assertStates('承認待ち', ['承認待ち'])
    assert.match(document.body.textContent, /変更前/)
    assert.match(document.body.textContent, /変更後/)
    assert.match(document.body.textContent, /影響件数: 1/)
    document.querySelector('.external-review-approve').click()
    await settle()
    assert.equal(api.approvals, 0, 'GUI must not call backend when draft flush fails')
    assert.equal(notes.draft, '未保存の入力', 'failed flush must retain the draft')
    assert.equal(api.review.state, 'pending_approval', 'failed flush must retain the proposal')
    assert.match(document.body.textContent, /下書きと変更要求は保持/)
    notes.allowFlush()
    document.querySelector('.external-review-approve').click()
    await settle()
    assert.equal(api.approvals, 1)
    assert.equal(api.review.state, 'applied')
    // Reject changes only the operation state, retaining the old item status.
    api.review.state = 'pending_approval'
    document.querySelector('.external-review-heading button').click()
    await settle()
    document.querySelector('.external-review-actions button').click()
    await settle()
    assert.equal(api.review.state, 'rejected')
    assert.equal(api.review.items[0].status, 'pending_approval')
    assertStates('却下', ['却下'])
    assert.equal(document.querySelector('.external-review-actions'), null)

    api.review.state = 'expired'
    document.querySelector('.external-review-heading button').click()
    await settle()
    assertStates('期限切れ', ['期限切れ'])
    assert.equal(document.querySelector('.external-review-actions'), null)

    // Partial application can retain concrete results even after rejection/expiry.
    const itemStatuses = ['applied', 'conflict', 'save-failure', 'not-executed', 'not-applicable', 'stale']
    const itemLabels = ['適用済み', '競合', '保存失敗', '未適用', '適用対象外', '期限切れ']
    for (const [state, cardLabel] of [
      ['pending_approval', '承認待ち'], ['applied', '適用済み'], ['conflict', '競合'],
      ['rejected', '却下'], ['expired', '期限切れ'], ['applying', '適用中'],
    ]) {
      api.review.state = state
      api.review.items = itemStatuses.map((status, index) => ({ candidateId: `candidate-${index}`, status }))
      api.review.items.push(
        { candidateId: 'pending-status', status: 'pending_approval' },
        { candidateId: 'missing-status' }, { candidateId: 'empty-status', status: '' },
      )
      const originalItems = structuredClone(api.review.items)
      document.querySelector('.external-review-heading button').click()
      await settle()
      const pendingLabel = state === 'applying' ? '承認待ち' : cardLabel
      assertStates(cardLabel, [...itemLabels, pendingLabel, cardLabel, cardLabel])
      assert.deepEqual(api.review.items, originalItems, 'display helpers must not mutate backend item states')
    }
  } finally {
    app.unmount()
  }
  console.log('external change review tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
