import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>')
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  navigator: dom.window.navigator,
  Node: dom.window.Node,
  Element: dom.window.Element,
  HTMLElement: dom.window.HTMLElement,
  SVGElement: dom.window.SVGElement,
  Event: dom.window.Event,
})
const { createPinia, setActivePinia } = await import('pinia')
const { createApp, nextTick } = await import('vue')

const root = process.cwd()
const outDir = path.join(root, '.tmp', 'organization-store-test')
const outFile = path.join(outDir, 'useOrganizationStore.mjs')
await mkdir(outDir, { recursive: true })
const source = (await readFile(path.join(root, 'src/stores/useOrganizationStore.ts'), 'utf8'))
  .replace("from '../api/organization'", "from './mock-organization.mjs'")
  .replace("from './useNoteStore'", "from './mock-note-store.mjs'")
  .replace("from './useTagStore'", "from './mock-tag-store.mjs'")
  .replace("from './useSupportWorkspaceStore'", "from './mock-support-workspace-store.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(outFile, compiled.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-organization.mjs'), `
export const calls = []
export const discardedSessionIds = []
export let invalidationCount = 0
let analysisFactory
let failure = false
let sessionNumber = 0
let applyStatuses = {}
const deferredInputs = new Set()
const pendingAnalyses = new Map()
const progressListeners = new Set()
export function onOrganizationProgress(listener) { progressListeners.add(listener); return () => progressListeners.delete(listener) }
export function emitOrganizationProgress(requestId, phase, processedNotes, totalNotes) {
  for (const listener of progressListeners) listener({ requestId, phase, processedNotes, totalNotes })
}
function inputKey(input) { return input.scope === 'note' ? 'note:' + input.noteId : input.scope === 'space' ? 'space' : input.scope + ':' + input.notebookId }
export function configureAnalysis(factory) { analysisFactory = factory }
export function failAnalysis() { failure = true }
export function deferAnalysisFor(key) { deferredInputs.add(key) }
export function resolveAnalysis(key, value) {
  const resolve = pendingAnalyses.get(key)
  if (!resolve) throw new Error('deferred analysis was not found: ' + key)
  pendingAnalyses.delete(key)
  resolve(value)
}
export function setApplyStatuses(next) { applyStatuses = next }
export async function analyzeOrganization(input) {
  calls.push(['analyze', input])
  if (failure) { failure = false; throw new Error('unavailable') }
  const source = typeof analysisFactory === 'function' ? analysisFactory(input) : analysisFactory
  const value = structuredClone({ ...source, sessionId: 'session-' + (++sessionNumber) })
  const key = inputKey(input)
  if (deferredInputs.has(key)) {
    deferredInputs.delete(key)
    return new Promise((resolve) => pendingAnalyses.set(key, resolve))
  }
  return value
}
export async function applyOrganizationCandidates(input) {
  calls.push(['apply', input.sessionId, [...input.candidateIds]])
  return input.candidateIds.map((candidateId) => ({ candidateId, status: applyStatuses[candidateId] ?? 'applied' }))
}
export async function discardOrganizationAnalysis(sessionId) { discardedSessionIds.push(sessionId) }
export async function invalidateOrganizationAnalyses() { invalidationCount += 1 }
`, 'utf8')
await writeFile(path.join(outDir, 'mock-note-store.mjs'), `
let store
export function configureNoteStore(next) { store = next }
export function useNoteStore() { return store }
`, 'utf8')
await writeFile(path.join(outDir, 'mock-tag-store.mjs'), `
export const refreshedNoteIds = []
export let activeNoteId = null
export function setActiveNoteId(noteId) { activeNoteId = noteId }
export function useTagStore() {
  return { async refreshNoteTagsIfActive(noteId) {
    if (activeNoteId !== noteId) return false
    refreshedNoteIds.push(noteId)
    return true
  } }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-support-workspace-store.mjs'), `
export const support = { activeTab: '', isMinimized: false, open(tab) { this.activeTab = tab; this.isMinimized = false }, minimize() { this.isMinimized = true }, restore() { this.isMinimized = false } }
export function useSupportWorkspaceStore() { return support }
`, 'utf8')
await writeFile(path.join(outDir, 'mock-notebook-store.mjs'), `
export function useNotebookStore() { return { notebooks: [{ id: 'notebook-a', name: 'Notebook A' }, { id: 'notebook-b', name: 'Notebook B' }] } }
`, 'utf8')
const componentSource = await readFile(path.join(root, 'src/components/OrganizationCenter.vue'), 'utf8')
const { descriptor } = parse(componentSource)
const component = compileScript(descriptor, { id: 'organization-center-test', inlineTemplate: true }).content
  .replace("from '../stores/useOrganizationStore'", "from './useOrganizationStore.mjs'")
  .replace("from '../stores/useNoteStore'", "from './mock-note-store.mjs'")
  .replace("from '../stores/useNotebookStore'", "from './mock-notebook-store.mjs'")
await writeFile(path.join(outDir, 'OrganizationCenter.mjs'), ts.transpileModule(component, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText, 'utf8')

try {
  const mockAPI = await import(pathToFileURL(path.join(outDir, 'mock-organization.mjs')).href)
  const noteMock = await import(pathToFileURL(path.join(outDir, 'mock-note-store.mjs')).href)
  const tagMock = await import(pathToFileURL(path.join(outDir, 'mock-tag-store.mjs')).href)
  const supportMock = await import(pathToFileURL(path.join(outDir, 'mock-support-workspace-store.mjs')).href)
  const { useOrganizationStore } = await import(pathToFileURL(outFile).href)
  setActivePinia(createPinia())
  const noteOperations = []
  noteMock.configureNoteStore({
    activeNote: null,
    summaries: [],
    async runOrganizationOperation(noteId, operation) {
      noteOperations.push(noteId)
      return operation()
    },
  })

  const base = { spaceId: 'space', before: {}, proposed: {}, reason: '確認理由', applicable: true }
  const spaceCandidates = [
    { ...base, id: 'space-one', noteId: 'note-a', kind: 'tag-assignment', noteTitle: 'A' },
    { ...base, id: 'space-two', noteId: 'note-a', kind: 'tag-assignment', noteTitle: 'A' },
    { ...base, id: 'space-three', noteId: 'note-b', kind: 'title', noteTitle: 'B' },
  ]
  const noteCandidates = (noteId) => [
    { ...base, id: `${noteId}-title`, noteId, kind: 'title', noteTitle: noteId.toUpperCase(), before: { title: 'Before' }, proposed: { title: 'After' } },
  ]
  const makeAnalysis = (input) => {
    const candidates = input.scope === 'note' ? noteCandidates(input.noteId) : spaceCandidates
    return {
      sessionId: 'replaced-by-mock', spaceId: 'space', scope: input.scope,
      noteId: input.noteId, notebookId: input.notebookId,
      startedAt: new Date().toISOString(), candidates, analyzedNotes: candidates.length,
      skippedLocked: 0, skippedTrash: 0,
    }
  }
  mockAPI.configureAnalysis(makeAnalysis)
  const store = useOrganizationStore()
  store.open('organize')
  assert.equal(supportMock.support.activeTab, 'organize')
  assert.equal(await store.analyze({ scope: 'space' }), true)
  store.selectApplicable()
  assert.deepEqual(store.selectedCandidateIds, ['space-one', 'space-two', 'space-three'])

  tagMock.setActiveNoteId('note-b')
  mockAPI.setApplyStatuses({ 'space-one': 'applied', 'space-two': 'conflict', 'space-three': 'save-failure' })
  await store.applySelected()
  const applyCalls = mockAPI.calls.filter(([kind]) => kind === 'apply')
  assert.deepEqual(applyCalls.map(([, , ids]) => ids), [['space-one', 'space-two'], ['space-three']], 'same-note candidates share one backend batch')
  assert.deepEqual(noteOperations, ['note-a', 'note-b'], 'each note group uses its existing serialized operation lane')
  assert.deepEqual(store.selectedCandidateIds, ['space-three'], 'conflicts require reanalysis while save failures remain retryable')
  assert.equal(store.outcomes['space-two'].status, 'conflict')
  assert.equal(store.outcomes['space-three'].status, 'save-failure')
  assert.deepEqual(tagMock.refreshedNoteIds, [], 'applying tags to an inactive note must not disturb the visible note tags')
  await store.applyCandidate('space-two')
  assert.equal(mockAPI.calls.filter(([kind]) => kind === 'apply').length, 2, 'conflicted candidates are not retried against a stale analysis')
  mockAPI.setApplyStatuses({ 'space-three': 'applied' })
  await store.applySelected()
  assert.equal(store.getSession('space').outcomes['space-three'].status, 'applied')

  // A different scope in a mini panel must not replace the center's review state.
  assert.equal(await store.analyze({ scope: 'space' }), true)
  store.toggleCandidate('space-one')
  store.toggleCandidate('space-two')
  mockAPI.setApplyStatuses({ 'space-one': 'save-failure' })
  await store.applyCandidate('space-one')
  const savedSpace = store.getSession('space')
  const savedSpaceSessionId = savedSpace.analysis.sessionId
  const savedSpaceSelection = [...savedSpace.selectedCandidateIds]
  const savedSpaceOutcome = savedSpace.outcomes['space-one'].status
  const savedSpaceCandidates = savedSpace.analysis.candidates

  supportMock.support.minimize()
  const miniAResult = store.openMini('note-a', 'ai')
  await miniAResult
  assert.equal(store.analysis.sessionId, savedSpaceSessionId, 'opening a mini panel leaves the center scope selected')
  const noteA = store.getSessionForNote('note-a')
  assert.equal(noteA.analysis.noteId, 'note-a')
  store.toggleCandidate('note-a-title', store.noteSessionKey('note-a'))
  mockAPI.setApplyStatuses({ 'note-a-title': 'not-executed' })
  await store.applyCandidate('note-a-title', store.noteSessionKey('note-a'))
  const noteASessionId = noteA.analysis.sessionId
  const noteASelection = [...noteA.selectedCandidateIds]
  const noteAOutcome = noteA.outcomes['note-a-title'].status
  const analyzeCallCount = mockAPI.calls.filter(([kind]) => kind === 'analyze').length
  await store.openNote('note-a')
  assert.equal(store.centerSessionKey, store.noteSessionKey('note-a'))
  assert.equal(supportMock.support.activeTab, 'organize')
  assert.equal(mockAPI.calls.filter(([kind]) => kind === 'analyze').length, analyzeCallCount, 'opening an existing note review must not reanalyze')
  assert.deepEqual(store.selectedCandidateIds, noteASelection)
  store.showScope('space')
  await store.openMini('note-b', 'related')
  const noteBSessionId = store.getSessionForNote('note-b').analysis.sessionId
  store.openMini('note-a', 'related')
  assert.equal(store.miniSessionKey, store.noteSessionKey('note-a'))
  assert.equal(store.getSessionForNote('note-a').analysis.sessionId, noteASessionId)
  assert.deepEqual(store.getSessionForNote('note-a').selectedCandidateIds, noteASelection)
  assert.equal(store.getSessionForNote('note-a').outcomes['note-a-title'].status, noteAOutcome)
  store.closeMini()
  store.open('tasks')
  assert.equal(store.analysis.sessionId, savedSpaceSessionId)
  assert.equal(store.getSession('space').analysis.candidates, savedSpaceCandidates)
  assert.deepEqual(store.selectedCandidateIds, savedSpaceSelection)
  assert.equal(store.outcomes['space-one'].status, savedSpaceOutcome)
  assert.equal(store.getSessionForNote('note-b').analysis.sessionId, noteBSessionId)
  supportMock.support.minimize()
  supportMock.support.restore()
  supportMock.support.minimize()
  store.open('organize')
  assert.equal(store.analysis.sessionId, savedSpaceSessionId, 'close/minimize/restore keeps center review data')

  // Out-of-order responses from independent scopes are cached independently.
  mockAPI.deferAnalysisFor('space')
  mockAPI.deferAnalysisFor('note:note-e')
  const delayedSpace = store.analyze({ scope: 'space' })
  const delayedNote = store.openMini('note-e', 'ai')
  const beforeReverseResponses = mockAPI.invalidationCount
  mockAPI.resolveAnalysis('note:note-e', makeAnalysis({ scope: 'note', noteId: 'note-e' }))
  assert.equal(await delayedNote, true)
  const currentNoteB = store.getSessionForNote('note-e')
  const currentNoteBSessionId = currentNoteB.analysis.sessionId
  mockAPI.resolveAnalysis('space', makeAnalysis({ scope: 'space' }))
  assert.equal(await delayedSpace, true)
  assert.equal(store.getSessionForNote('note-e').analysis.sessionId, currentNoteBSessionId, 'late space response cannot replace the mini scope')
  assert.equal(mockAPI.invalidationCount, beforeReverseResponses, 'a valid response from another scope must not invalidate all sessions')
  mockAPI.setApplyStatuses({ 'note-e-title': 'applied' })
  await store.applyCandidate('note-e-title', store.noteSessionKey('note-e'))
  assert.equal(mockAPI.calls.at(-1)[1], currentNoteBSessionId, 'the latest mini session remains applicable')

  // Replacing a same-scope request discards only the obsolete returned session.
  mockAPI.deferAnalysisFor('note:note-c')
  const obsoleteMiniRequest = store.openMini('note-c', 'related')
  await Promise.resolve()
  const currentMiniRequest = store.analyze({ scope: 'note', noteId: 'note-c' })
  assert.equal(await currentMiniRequest, true)
  const validNoteCSessionId = store.getSessionForNote('note-c').analysis.sessionId
  const obsolete = makeAnalysis({ scope: 'note', noteId: 'note-c' })
  mockAPI.resolveAnalysis('note:note-c', obsolete)
  assert.equal(await obsoleteMiniRequest, false)
  assert.ok(mockAPI.discardedSessionIds.includes(obsolete.sessionId))
  assert.equal(store.getSessionForNote('note-c').analysis.sessionId, validNoteCSessionId)

  // Lock clears all body-derived UI state and discards a response that arrives later.
  mockAPI.deferAnalysisFor('note:note-d')
  const delayedLockedAnalysis = store.openMini('note-d', 'related')
  await Promise.resolve()
  const lockedRequestID = mockAPI.calls.filter(([kind]) => kind === 'analyze').at(-1)[1].requestId
  const beforeLockInvalidation = mockAPI.invalidationCount
  await store.clearForLock()
  mockAPI.emitOrganizationProgress(lockedRequestID, 'reading', 100, 200)
  assert.deepEqual(store.sessions, {}, 'progress arriving after lock cannot restore a session')
  const lockedResponse = makeAnalysis({ scope: 'note', noteId: 'note-d' })
  mockAPI.resolveAnalysis('note:note-d', lockedResponse)
  assert.equal(await delayedLockedAnalysis, false)
  assert.deepEqual(store.sessions, {})
  assert.equal(mockAPI.invalidationCount, beforeLockInvalidation + 1, 'only lock uses global session invalidation')
  assert.ok(mockAPI.discardedSessionIds.includes(lockedResponse.sessionId), 'late locked response is discarded individually')

  // Drive the real center component and Store through the controls used to select scopes and approve candidates.
  const uiPinia = createPinia()
  setActivePinia(uiPinia)
  const uiStore = useOrganizationStore()
  const { default: OrganizationCenter } = await import(pathToFileURL(path.join(outDir, 'OrganizationCenter.mjs')).href)
  const app = createApp(OrganizationCenter)
  app.use(uiPinia)
  const rootElement = document.querySelector('#app')
  app.mount(rootElement)
  const click = async (label) => {
    const button = [...rootElement.querySelectorAll('button')].find((item) => item.textContent.includes(label))
    assert.ok(button, `missing button: ${label}`)
    button.click()
    await nextTick()
    await new Promise((resolve) => setTimeout(resolve, 0))
    await nextTick()
  }
  const choose = async (label, value) => {
    const select = [...rootElement.querySelectorAll('label')].find((item) => item.textContent.includes(label))?.querySelector('select')
    assert.ok(select, `missing select: ${label}`)
    select.value = value
    select.dispatchEvent(new Event('change', { bubbles: true }))
    await nextTick()
  }
  const analysisCount = () => mockAPI.calls.filter(([kind]) => kind === 'analyze').length
  await click('ノート整理')
  await choose('解析範囲', 'notebook-a')
  assert.equal(uiStore.centerSessionKey, 'notebook:notebook-a:notebook')
  await click('読み取り解析')
  const notebookA = uiStore.getSession('notebook:notebook-a:notebook')
  uiStore.toggleCandidate('space-two')
  mockAPI.setApplyStatuses({ 'space-one': 'save-failure' })
  await uiStore.applyCandidate('space-one')
  const notebookSnapshot = {
    id: notebookA.analysis.sessionId,
    candidates: notebookA.analysis.candidates,
    selection: [...notebookA.selectedCandidateIds],
    outcome: notebookA.outcomes['space-one'].status,
  }
  await uiStore.openNote('note-a')
  await nextTick()
  const beforeReturn = analysisCount()
  await click('前の整理範囲へ')
  assert.equal(uiStore.centerSessionKey, 'notebook:notebook-a:notebook')
  assert.equal(analysisCount(), beforeReturn, 'returning to Notebook A uses the existing analysis')
  assert.equal(uiStore.analysis.sessionId, notebookSnapshot.id)
  assert.equal(uiStore.analysis.candidates, notebookSnapshot.candidates)
  assert.deepEqual(uiStore.selectedCandidateIds, notebookSnapshot.selection)
  assert.equal(uiStore.outcomes['space-one'].status, notebookSnapshot.outcome)
  assert.equal(rootElement.querySelector('.organization-controls select').value, 'notebook-a')

  const beforeScopeSelection = analysisCount()
  await choose('Notebook範囲', 'descendants')
  assert.equal(uiStore.centerSessionKey, 'notebook:notebook-a:descendants')
  assert.equal(analysisCount(), beforeScopeSelection, 'selecting a scope never silently reanalyzes it')
  await click('読み取り解析')
  const descendantsId = uiStore.analysis.sessionId
  uiStore.toggleCandidate('space-two')
  const descendantsSelection = [...uiStore.selectedCandidateIds]
  await uiStore.openNote('note-a')
  await nextTick()
  const beforeDescendantsReturn = analysisCount()
  await click('前の整理範囲へ')
  assert.equal(uiStore.analysis.sessionId, descendantsId)
  assert.deepEqual(uiStore.selectedCandidateIds, descendantsSelection)
  assert.equal([...rootElement.querySelectorAll('.organization-controls select')][1].value, 'descendants')
  assert.equal(analysisCount(), beforeDescendantsReturn)
  await choose('Notebook範囲', 'notebook')
  assert.equal(uiStore.analysis.sessionId, notebookSnapshot.id, 'selecting the direct scope restores its session')
  await choose('解析範囲', '')
  assert.equal(uiStore.centerSessionKey, 'space')
  await click('読み取り解析')
  const spaceId = uiStore.analysis.sessionId
  await uiStore.openNote('note-a')
  await nextTick()
  const beforeSpaceReturn = analysisCount()
  await click('前の整理範囲へ')
  assert.equal(uiStore.analysis.sessionId, spaceId)
  assert.equal(analysisCount(), beforeSpaceReturn)

  mockAPI.configureAnalysis((input) => input.noteId === 'note-filter'
    ? { ...makeAnalysis(input), candidates: [
      { ...base, id: 'filter-duplicate', noteId: 'note-filter', kind: 'duplicate-note', noteTitle: 'Filter' },
      { ...base, id: 'filter-title', noteId: 'note-filter', kind: 'title', noteTitle: 'Filter' },
    ] }
    : makeAnalysis(input))
  await uiStore.openNote('note-filter')
  await nextTick()
  await click('タスク支援')
  assert.deepEqual(
    [...rootElement.querySelectorAll('.organization-counts button')].map((button) => button.textContent.replace(/\s+/g, '')),
    ['1重複ノート', '1タイトル候補'],
    'candidate kind counts preserve first-seen order and totals',
  )
  uiStore.toggleCandidate('filter-title')
  await click('重複ノート')
  assert.equal(rootElement.querySelectorAll('.organization-candidate').length, 1)
  assert.match(rootElement.querySelector('.organization-footer').textContent, /表示外 1件を選択中/)
  await click('表示中の適用可能を選択')
  assert.deepEqual(uiStore.selectedCandidateIds, ['filter-title', 'filter-duplicate'], 'bulk selection adds visible candidates without discarding hidden selections')
  const beforeApply = mockAPI.calls.filter(([kind]) => kind === 'apply').length
  await click('選択した候補を承認して適用')
  assert.deepEqual(mockAPI.calls.filter(([kind]) => kind === 'apply').slice(beforeApply).map(([, , ids]) => ids), [['filter-duplicate']])
  assert.deepEqual(uiStore.selectedCandidateIds, ['filter-title'], 'hidden selection remains available after a filtered apply')
  mockAPI.configureAnalysis((input) => makeAnalysis(input))
  await uiStore.analyze({ scope: 'note', noteId: 'note-filter' })
  await nextTick()
  assert.equal(rootElement.querySelectorAll('.organization-candidate').length, 0)
  assert.match(rootElement.querySelector('.organization-empty').textContent, /この種類に該当する候補はありません/)
  assert.doesNotMatch(rootElement.querySelector('.organization-empty').textContent, /現在の解析範囲で確認が必要な項目は見つかりませんでした/)
  await uiStore.openNote('note-target')
  await nextTick()
  assert.equal(rootElement.querySelectorAll('.organization-candidate').length, 1, 'the previous kind filter cannot hide another note')
  assert.match(rootElement.querySelector('.organization-candidate').textContent, /タイトル候補/)
  assert.doesNotMatch(rootElement.textContent, /この種類に該当する候補はありません/)

  mockAPI.configureAnalysis((input) => input.noteId === 'note-large'
    ? { ...makeAnalysis(input), candidates: Array.from({ length: 120 }, (_, index) => ({
      ...base, id: `large-${index}`, noteId: 'note-large', kind: 'title', noteTitle: `Synthetic ${index}`,
    })) }
    : makeAnalysis(input))
  await uiStore.openNote('note-large')
  await nextTick()
  assert.equal(rootElement.querySelectorAll('.organization-candidate').length, 100, 'large analyses render only the first page')
  assert.match(rootElement.textContent, /候補 100 \/ 120件を表示中/)
  uiStore.toggleCandidate('large-119')
  await click('表示中の適用可能を選択')
  assert.equal(uiStore.selectedCandidateIds.length, 101, 'bulk selection keeps an existing selection outside the rendered page')
  const beforePageApply = mockAPI.calls.filter(([kind]) => kind === 'apply').length
  await click('選択した候補を承認して適用')
  assert.equal(mockAPI.calls.filter(([kind]) => kind === 'apply').slice(beforePageApply)[0][2].length, 100, 'bulk apply excludes candidates outside the rendered page')
  assert.deepEqual(uiStore.selectedCandidateIds, ['large-119'])
  await click('さらに20件表示')
  assert.equal(rootElement.querySelectorAll('.organization-candidate').length, 120)

  mockAPI.deferAnalysisFor('note:note-progress')
  const pendingProgressAnalysis = uiStore.openNote('note-progress')
  await Promise.resolve()
  const progressRequestID = mockAPI.calls.filter(([kind]) => kind === 'analyze').at(-1)[1].requestId
  mockAPI.emitOrganizationProgress('other-request', 'reading', 50, 200)
  assert.equal(uiStore.progress, null, 'unrelated request progress is ignored')
  mockAPI.emitOrganizationProgress(progressRequestID, 'reading', 100, 200)
  await nextTick()
  assert.match(rootElement.querySelector('.organization-progress').textContent, /ノートを確認中 100 \/ 200件/)
  assert.equal(rootElement.querySelector('.organization-progress-bar').value, 100)
  mockAPI.emitOrganizationProgress(progressRequestID, 'proposing', 200, 200)
  await nextTick()
  assert.match(rootElement.querySelector('.organization-progress').textContent, /候補をまとめています/)
  assert.equal(rootElement.querySelector('.organization-progress-bar'), null)
  mockAPI.resolveAnalysis('note:note-progress', makeAnalysis({ scope: 'note', noteId: 'note-progress' }))
  await pendingProgressAnalysis
  await nextTick()
  assert.equal(rootElement.querySelector('.organization-progress'), null, 'progress is cleared on completion')
  app.unmount()
  console.log('organization store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
