import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useAILibrarianStore.ts')
const chatStorePath = path.join(rootDir, 'src', 'stores', 'useAIChatStore.ts')
const timelineUtilityPath = path.join(rootDir, 'src', 'utils', 'aiWorkspaceTimeline.ts')
const panelPath = path.join(rootDir, 'src', 'components', 'AILibrarianPanel.vue')
const workspacePath = path.join(rootDir, 'src', 'components', 'AIWorkspace.vue')
const outDir = path.join(rootDir, '.tmp', 'ai-librarian-test')
const outFile = path.join(outDir, 'useAILibrarianStore.mjs')
const chatStoreOutFile = path.join(outDir, 'useAIChatStore.mjs')
const timelineUtilityOutFile = path.join(outDir, 'aiWorkspaceTimeline.mjs')

await mkdir(outDir, { recursive: true })

const [storeSource, chatStoreSource, timelineUtilitySource, panelSource, workspaceSource] = await Promise.all([
  readFile(sourcePath, 'utf8'),
  readFile(chatStorePath, 'utf8'),
  readFile(timelineUtilityPath, 'utf8'),
  readFile(panelPath, 'utf8'),
  readFile(workspacePath, 'utf8'),
])
const source = storeSource
  .replace("from '../api/ai'", "from './mock-ai-librarian.mjs'")
  .replace("from './useAIStore'", "from './mock-ai-store.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')
const compiledChatStore = ts.transpileModule(
  chatStoreSource.replace("from '../api/notes'", "from './mock-notes.mjs'"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: chatStorePath,
  },
)
await writeFile(chatStoreOutFile, compiledChatStore.outputText, 'utf8')
const compiledTimelineUtility = ts.transpileModule(timelineUtilitySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
  fileName: timelineUtilityPath,
})
await writeFile(timelineUtilityOutFile, compiledTimelineUtility.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-ai-store.mjs'), `
export function useAIStore() {
  return {
    configuredSetting: { providerID: 'openrouter', modelID: 'openai/gpt-test' },
    isLibrarianReady: true,
  }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-notes.mjs'), `
export async function listNotes() { return [] }
`, 'utf8')
await writeFile(path.join(outDir, 'mock-ai-librarian.mjs'), `
export const calls = { start: [], cancel: [] }
let listener
let pendingStart
let pendingCancel
let nextCancelResponse

export function onAILibrarianUpdate(nextListener) {
  listener = nextListener
  return () => { if (listener === nextListener) listener = undefined }
}

export function emit(event) {
  listener?.(event)
}

export function deferNextStart() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  pendingStart = { promise, resolve }
  return pendingStart
}

export function deferNextCancel() {
  let resolve
  let reject
  const promise = new Promise((done, fail) => {
    resolve = done
    reject = fail
  })
  pendingCancel = { promise, resolve, reject }
  return pendingCancel
}

export function queueCancelResponse(response) {
  nextCancelResponse = response
}

export async function startAILibrarian(input) {
  calls.start.push(input)
  if (pendingStart) {
    const deferred = pendingStart
    pendingStart = undefined
    await deferred.promise
  }
  return { requestID: 'request-1' }
}

export async function cancelAILibrarian(requestID) {
  calls.cancel.push(requestID)
  if (pendingCancel) {
    const deferred = pendingCancel
    pendingCancel = undefined
    await deferred.promise
  }
  const response = nextCancelResponse ?? { canceled: true }
  nextCancelResponse = undefined
  return response
}
`, 'utf8')

function event(overrides = {}) {
  return {
    requestID: 'request-1',
    noteID: 'note-1',
    baseRevision: 4,
    operation: 'related',
    phase: 'partial',
    sequence: 1,
    ...overrides,
  }
}

const input = {
  providerID: 'openrouter',
  modelID: 'openai/gpt-test',
  operation: 'related',
  noteID: 'note-1',
  baseRevision: 4,
  title: 'Target note',
  content: 'Target body',
  candidateCount: 5,
  candidates: [{ noteID: 'note-2', title: 'Candidate note' }],
}

try {
  assert.match(source, /onAILibrarianUpdate/, 'the store must subscribe to Wails librarian events')
  assert.doesNotMatch(source, /localStorage/, 'librarian state must remain in memory')
  assert.match(panelSource, /@click="librarianStore\.cancel\(\)"/, 'the cancel button must delegate to the store')
  assert.match(panelSource, /v-if="librarianStore\.state !== 'canceling'"/, 'canceling must hide the duplicate cancel action')
  assert.match(panelSource, /librarianStore\.state === 'canceled'/, 'the panel must expose the canceled state')
  assert.match(panelSource, /role="alert"/, 'typed librarian failures must be announced')
  assert.match(
    panelSource,
    /librarianStore\.error && \(librarianStore\.state === 'error' \|\| librarianStore\.isGenerating\)/,
    'cancel API failures must remain visible while the backend request is still active',
  )
  assert.match(workspaceSource, /completeLibrarianTimelineTrace\(/, 'terminal librarian states must update the timeline')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-ai-librarian.mjs')).href)
  const { useAILibrarianStore } = await import(pathToFileURL(outFile).href)
  const { useAIChatStore } = await import(pathToFileURL(chatStoreOutFile).href)
  const { completeLibrarianTimelineTrace } = await import(pathToFileURL(timelineUtilityOutFile).href)
  const store = useAILibrarianStore()
  const chatStore = useAIChatStore()

  function completeTimelineTrace(traceID) {
    return completeLibrarianTimelineTrace({
      traceID,
      state: store.state,
      errorMessage: store.error?.message,
      updateTimelineEntry: (entryID, update) => chatStore.updateTimelineEntry(entryID, update),
    })
  }

  const deferred = mock.deferNextStart()
  const starting = store.start(input)
  await Promise.resolve()
  mock.emit(event({ partialText: '{"candidates":' }))
  deferred.resolve()
  assert.equal(await starting, true)
  assert.equal(store.state, 'partial', 'an event received before start resolves must be replayed')
  assert.equal(store.partialText, '{"candidates":')

  mock.emit(event({
    phase: 'completed',
    sequence: 2,
    result: {
      operation: 'related',
      quality: 'normal',
      candidates: [{ noteID: 'note-2', score: 0.91, reason: 'same topic' }],
    },
  }))
  assert.equal(store.state, 'success')
  assert.equal(store.result.candidates[0].noteID, 'note-2')

  const secondInput = { ...input, noteID: 'note-3', baseRevision: 8 }
  const cancelTraceID = chatStore.appendToolTrace('related', '関連メモを生成しています。')
  assert.equal(await store.start(secondInput), true)
  mock.emit(event({ noteID: 'note-1', baseRevision: 4, partialText: 'ignored' }))
  assert.equal(store.partialText, '', 'late events for another note must be ignored')
  mock.emit(event({ noteID: 'note-3', baseRevision: 8, partialText: 'accepted' }))
  assert.equal(store.partialText, 'accepted')

  assert.equal(await store.cancel(), true)
  assert.deepEqual(mock.calls.cancel, ['request-1'])
  assert.equal(store.state, 'canceling')
  mock.emit(event({ noteID: 'note-3', baseRevision: 8, phase: 'partial', sequence: 2, partialText: 'ignored-after-cancel' }))
  assert.equal(store.state, 'canceling', 'partial events received after cancel starts must not reopen the cancel action')
  assert.equal(store.partialText, 'accepted')
  mock.emit(event({ noteID: 'note-3', baseRevision: 8, phase: 'canceled', sequence: 3 }))
  assert.equal(store.state, 'canceled')
  assert.equal(store.error?.code, 'AI_CANCELLED')
  assert.equal(store.error?.message, 'AI司書の生成をキャンセルしました。')
  assert.equal(store.isGenerating, false)
  assert.equal(store.result, null, 'cancellation must not retain a result')
  assert.equal(completeTimelineTrace(cancelTraceID), true)
  const canceledTrace = chatStore.timeline.find((entry) => entry.id === cancelTraceID)
  assert.equal(canceledTrace?.status, 'error')
  assert.equal(canceledTrace?.content, 'AI司書の生成をキャンセルしました。')
  mock.emit(event({
    noteID: 'note-3',
    baseRevision: 8,
    phase: 'completed',
    sequence: 3,
    result: {
      operation: 'related',
      quality: 'normal',
      candidates: [{ noteID: 'late-note', score: 1 }],
    },
  }))
  assert.equal(store.state, 'canceled', 'a completed event after cancellation must be ignored')
  assert.equal(store.result, null)

  const timeoutInput = { ...input, noteID: 'note-timeout', baseRevision: 9 }
  const timeoutTraceID = chatStore.appendToolTrace('related', '関連メモを生成しています。')
  const startsBeforeTimeout = mock.calls.start.length
  assert.equal(await store.start(timeoutInput), true)
  mock.emit(event({
    noteID: 'note-timeout',
    baseRevision: 9,
    phase: 'partial',
    sequence: 1,
    partialText: '破棄される途中結果',
  }))
  mock.emit(event({
    noteID: 'note-timeout',
    baseRevision: 9,
    phase: 'failed',
    sequence: 2,
    error: { code: 'AI_TIMEOUT', raw: 'raw-timeout-marker' },
  }))
  assert.equal(mock.calls.start.length, startsBeforeTimeout + 1, 'timeout must not retry automatically')
  assert.equal(store.state, 'error')
  assert.equal(store.error?.code, 'AI_TIMEOUT')
  assert.equal(store.error?.message, 'AI プロバイダーが時間内に応答しませんでした。')
  assert.doesNotMatch(store.error?.message ?? '', /raw-timeout-marker/)
  assert.equal(store.result, null)
  assert.equal(store.isGenerating, false)
  assert.equal(completeTimelineTrace(timeoutTraceID), true)
  const timeoutTrace = chatStore.timeline.find((entry) => entry.id === timeoutTraceID)
  assert.equal(timeoutTrace?.status, 'error')
  assert.equal(timeoutTrace?.content, 'AI プロバイダーが時間内に応答しませんでした。')
  mock.emit(event({
    noteID: 'note-timeout',
    baseRevision: 9,
    phase: 'completed',
    sequence: 3,
    result: {
      operation: 'related',
      quality: 'normal',
      candidates: [{ noteID: 'late-timeout-note', score: 1 }],
    },
  }))
  assert.equal(store.state, 'error', 'a completed event after timeout must be ignored')
  assert.equal(store.result, null)

  const pendingCancelInput = { ...input, noteID: 'note-pending-cancel', baseRevision: 10 }
  const pendingCancel = mock.deferNextStart()
  const pendingStart = store.start(pendingCancelInput)
  await Promise.resolve()
  const cancelCallsBeforeRequestID = mock.calls.cancel.length
  assert.equal(await store.cancel(), true, 'cancel must be reserved while the start response is pending')
  assert.equal(store.state, 'canceling')
  assert.equal(mock.calls.cancel.length, cancelCallsBeforeRequestID, 'cancel must wait until the request ID is known')
  pendingCancel.resolve()
  assert.equal(await pendingStart, true)
  assert.equal(mock.calls.cancel.length, cancelCallsBeforeRequestID + 1)
  assert.equal(mock.calls.cancel.at(-1), 'request-1')
  assert.equal(store.state, 'canceling')
  mock.emit(event({ noteID: 'note-pending-cancel', baseRevision: 10, phase: 'canceled', sequence: 1 }))
  assert.equal(store.state, 'canceled')
  assert.equal(store.isGenerating, false)

  const discardedCancelInput = { ...input, noteID: 'note-discarded-cancel', baseRevision: 18 }
  const discardedCancelStartGate = mock.deferNextStart()
  const discardedCancelResponseGate = mock.deferNextCancel()
  const discardedCancelStart = store.start(discardedCancelInput)
  await Promise.resolve()
  assert.equal(await store.cancel(), true)
  discardedCancelStartGate.resolve()
  await Promise.resolve()
  assert.equal(store.state, 'canceling')
  store.discard()
  discardedCancelResponseGate.resolve()
  assert.equal(
    await discardedCancelStart,
    false,
    'discard while the reserved cancel API is pending must invalidate the start result',
  )
  assert.equal(store.state, 'idle')

  const staleCancelInput = { ...input, noteID: 'note-stale-cancel', baseRevision: 19 }
  const staleCancelStartGate = mock.deferNextStart()
  const staleCancelResponseGate = mock.deferNextCancel()
  const staleCancelStart = store.start(staleCancelInput)
  await Promise.resolve()
  assert.equal(await store.cancel(), true)
  staleCancelStartGate.resolve()
  await Promise.resolve()
  assert.equal(store.state, 'canceling')
  store.markStaleForRevision('note-stale-cancel', 20)
  staleCancelResponseGate.resolve()
  assert.equal(
    await staleCancelStart,
    false,
    'stale while the reserved cancel API is pending must invalidate the start result',
  )
  assert.equal(store.state, 'stale')

  const discardedPendingInput = { ...input, noteID: 'note-discarded-pending', baseRevision: 11 }
  const discardedStartGate = mock.deferNextStart()
  const discardedStart = store.start(discardedPendingInput)
  await Promise.resolve()
  assert.equal(await store.cancel(), true)
  const cancelCallsBeforeDiscardedResponse = mock.calls.cancel.length
  store.discard()
  assert.equal(store.state, 'idle')
  discardedStartGate.resolve()
  assert.equal(await discardedStart, false, 'a discarded pending start must not reactivate the store')
  assert.equal(store.state, 'idle')
  assert.equal(mock.calls.cancel.length, cancelCallsBeforeDiscardedResponse + 1)
  assert.equal(mock.calls.cancel.at(-1), 'request-1', 'the late request ID must be canceled after discard')

  const stalePendingInput = { ...input, noteID: 'note-stale-pending', baseRevision: 12 }
  const staleStartGate = mock.deferNextStart()
  const staleStart = store.start(stalePendingInput)
  await Promise.resolve()
  assert.equal(await store.cancel(), true)
  const cancelCallsBeforeStaleResponse = mock.calls.cancel.length
  store.markStaleForRevision('note-stale-pending', 13)
  assert.equal(store.state, 'stale')
  staleStartGate.resolve()
  assert.equal(await staleStart, false, 'a stale pending start must not return to canceling')
  assert.equal(store.state, 'stale')
  assert.equal(store.isGenerating, false)
  assert.equal(mock.calls.cancel.length, cancelCallsBeforeStaleResponse + 1)
  assert.equal(mock.calls.cancel.at(-1), 'request-1', 'the late request ID must be canceled after stale')

  const cancelFailureInput = { ...input, noteID: 'note-cancel-failure', baseRevision: 14 }
  assert.equal(await store.start(cancelFailureInput), true)
  mock.emit(event({ noteID: 'note-cancel-failure', baseRevision: 14, partialText: '生成は継続中' }))
  mock.queueCancelResponse({
    canceled: false,
    error: { code: 'AI_TIMEOUT', raw: 'raw-cancel-failure-marker' },
  })
  assert.equal(await store.cancel(), false)
  assert.equal(store.state, 'partial', 'a typed cancel failure must return to the running state')
  assert.equal(store.error?.code, 'AI_TIMEOUT')
  assert.equal(store.error?.message, 'AI プロバイダーが時間内に応答しませんでした。')
  assert.doesNotMatch(store.error?.message ?? '', /raw-cancel-failure-marker/)
  assert.equal(store.isGenerating, true, 'cancel failure must retain the active backend request')
  mock.emit(event({
    noteID: 'note-cancel-failure',
    baseRevision: 14,
    phase: 'completed',
    sequence: 2,
    result: {
      operation: 'related',
      quality: 'normal',
      candidates: [{ noteID: 'completed-after-cancel-failure', score: 0.7 }],
    },
  }))
  assert.equal(store.state, 'success', 'the terminal event must remain observable after cancel failure')
  assert.equal(store.error, null)
  assert.equal(store.isGenerating, false)

  const staleAfterCancelFailureInput = { ...input, noteID: 'note-stale-after-cancel-failure', baseRevision: 21 }
  const staleAfterCancelFailureTraceID = chatStore.appendToolTrace('related', '関連メモを生成しています。')
  assert.equal(await store.start(staleAfterCancelFailureInput), true)
  mock.queueCancelResponse({
    canceled: false,
    error: { code: 'AI_TIMEOUT', raw: 'raw-stale-cancel-failure' },
  })
  assert.equal(await store.cancel(), false)
  assert.equal(store.error?.code, 'AI_TIMEOUT')
  store.markStaleForRevision('note-stale-after-cancel-failure', 22)
  assert.equal(store.state, 'stale')
  assert.equal(store.error, null, 'stale must clear a non-terminal cancel API error')
  assert.equal(completeTimelineTrace(staleAfterCancelFailureTraceID), true)
  const staleAfterCancelFailureTrace = chatStore.timeline.find((entry) => entry.id === staleAfterCancelFailureTraceID)
  assert.equal(staleAfterCancelFailureTrace?.status, 'error')
  assert.equal(staleAfterCancelFailureTrace?.content, '候補生成を完了できませんでした。')
  assert.doesNotMatch(staleAfterCancelFailureTrace?.content ?? '', /時間内に応答/)

  const completedRaceInput = { ...input, noteID: 'note-completed-race', baseRevision: 15 }
  assert.equal(await store.start(completedRaceInput), true)
  mock.queueCancelResponse({
    canceled: false,
    error: { code: 'AI_TIMEOUT', raw: 'raw-late-cancel-response' },
  })
  const completedCancelGate = mock.deferNextCancel()
  const completedCancel = store.cancel()
  await Promise.resolve()
  mock.emit(event({
    noteID: 'note-completed-race',
    baseRevision: 15,
    phase: 'completed',
    sequence: 1,
    result: {
      operation: 'related',
      quality: 'normal',
      candidates: [{ noteID: 'terminal-wins', score: 0.9 }],
    },
  }))
  completedCancelGate.resolve()
  assert.equal(await completedCancel, false)
  assert.equal(store.state, 'success', 'a late typed cancel failure must not overwrite completed')
  assert.equal(store.result?.candidates[0]?.noteID, 'terminal-wins')
  assert.equal(store.error, null)

  const canceledRaceInput = { ...input, noteID: 'note-canceled-race', baseRevision: 16 }
  assert.equal(await store.start(canceledRaceInput), true)
  const canceledCancelGate = mock.deferNextCancel()
  const canceledCancel = store.cancel()
  await Promise.resolve()
  mock.emit(event({ noteID: 'note-canceled-race', baseRevision: 16, phase: 'canceled', sequence: 1 }))
  canceledCancelGate.reject(new Error('AI_TIMEOUT raw-terminal-race'))
  assert.equal(await canceledCancel, false)
  assert.equal(store.state, 'canceled', 'a late thrown cancel error must not overwrite canceled')
  assert.equal(store.error?.code, 'AI_CANCELLED')

  const failedRaceInput = { ...input, noteID: 'note-failed-race', baseRevision: 17 }
  assert.equal(await store.start(failedRaceInput), true)
  const failedCancelGate = mock.deferNextCancel()
  const failedCancel = store.cancel()
  await Promise.resolve()
  mock.emit(event({
    noteID: 'note-failed-race',
    baseRevision: 17,
    phase: 'failed',
    sequence: 1,
    error: { code: 'AI_TIMEOUT', raw: 'raw-terminal-timeout' },
  }))
  failedCancelGate.resolve()
  assert.equal(await failedCancel, true)
  assert.equal(store.state, 'error', 'a late successful cancel response must not overwrite failed')
  assert.equal(store.error?.code, 'AI_TIMEOUT')

  assert.equal(await store.start({ ...input, noteID: 'note-1', baseRevision: 4 }), true)
  mock.emit(event({ phase: 'completed', sequence: 1, result: {
    operation: 'related',
    quality: 'normal',
    candidates: [{ noteID: 'note-2', score: 0.8 }],
  } }))
  assert.equal(store.state, 'success')
  store.markStaleForRevision('note-1', 5)
  assert.equal(store.state, 'stale', 'a newer note revision must stale an in-memory result')
  mock.emit(event({ phase: 'partial', sequence: 2, partialText: 'late' }))
  assert.equal(store.partialText, '', 'stale requests must ignore late events')

  console.log('AI librarian tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
