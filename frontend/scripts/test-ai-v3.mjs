import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const assistantPath = path.join(rootDir, 'src', 'stores', 'useAIAssistantStore.ts')
const writingPath = path.join(rootDir, 'src', 'stores', 'useAIWritingStore.ts')
const assistantPanelPath = path.join(rootDir, 'src', 'components', 'AIAssistantPanel.vue')
const writingPanelPath = path.join(rootDir, 'src', 'components', 'AIWritingPanel.vue')
const outDir = path.join(rootDir, '.tmp', 'ai-v3-test')

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

await mkdir(outDir, { recursive: true })

async function compile(sourcePath, outputName) {
  const source = (await readFile(sourcePath, 'utf8'))
    .replace("from '../api/ai'", "from './mock-ai.mjs'")
    .replace("from './useAIStore'", "from './mock-ai-store.mjs'")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  const outputPath = path.join(outDir, outputName)
  await writeFile(outputPath, compiled.outputText, 'utf8')
  return outputPath
}

await Promise.all([
  compile(assistantPath, 'useAIAssistantStore.mjs'),
  compile(writingPath, 'useAIWritingStore.mjs'),
])

await writeFile(path.join(outDir, 'mock-ai-store.mjs'), `
export function useAIStore() {
  return { configuredSetting: { providerID: 'openrouter', modelID: 'openai/gpt-test' } }
}
`, 'utf8')

await writeFile(path.join(outDir, 'mock-ai.mjs'), `
export const calls = {
  context: [],
  assistant: [],
  cancelAssistant: [],
  saveHistory: [],
  writing: [],
  saveArtifact: [],
  historyDeletes: [],
  artifactDeletes: [],
  historyGets: [],
  historyDeleteAll: 0,
  artifactDeleteAll: 0,
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

let source = {
  noteID: 'note-1',
  title: '対象ノート',
  revision: 1,
  contentByte: 120,
}
let pendingContext
let pendingAssistant
let activeAssistant
let assistantProposal
const contextErrors = []
const assistantErrors = []
const assistantCancelResponses = []
const writingErrors = []

export function setSource(nextSource) {
  source = clone(nextSource)
}

export function deferAssistant() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  pendingAssistant = { promise, resolve }
  return pendingAssistant
}

export function deferContext() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  pendingContext = { promise, resolve }
  return pendingContext
}

export function setAssistantProposal(nextProposal) {
  assistantProposal = nextProposal ? clone(nextProposal) : undefined
}

export function queueContextError(error) {
  contextErrors.push(clone(error))
}

export function queueAssistantError(error) {
  assistantErrors.push(clone(error))
}

export function queueAssistantCancelResponse(response) {
  assistantCancelResponses.push(clone(response))
}

export function queueWritingError(error) {
  writingErrors.push(clone(error))
}

export async function prepareAIContext(input) {
  calls.context.push(clone(input))
  if (pendingContext) {
    const deferred = pendingContext
    pendingContext = undefined
    await deferred.promise
  }
  const error = contextErrors.shift()
  if (error) return { sources: [], error }
  return { sources: [clone(source)] }
}

export async function runAIAssistant(input) {
  calls.assistant.push(clone(input))
  if (pendingAssistant) {
    const deferred = pendingAssistant
    pendingAssistant = undefined
    activeAssistant = { requestID: input.requestID, deferred }
    await deferred.promise
    if (activeAssistant?.deferred === deferred) activeAssistant = undefined
  }
  const error = assistantErrors.shift()
  if (error) return { error }
  return {
    result: {
      providerID: input.providerID,
      modelID: input.modelID,
      kind: input.kind,
      messages: [
        ...(input.messages ?? []),
        { role: 'user', content: input.question },
        { role: 'assistant', content: '回答マーカー' },
      ],
      sources: [clone(source)],
      ...(assistantProposal ? { proposal: clone(assistantProposal) } : {}),
    },
  }
}

export async function cancelAIAssistant(requestID) {
  calls.cancelAssistant.push(requestID)
  const queued = assistantCancelResponses.shift()
  if (queued) return queued
  return { canceled: activeAssistant?.requestID === requestID }
}

export async function saveAIHistory(input) {
  calls.saveHistory.push(clone(input))
  return {
    history: {
      id: input.id ?? 'history-1',
      kind: input.kind,
      title: input.title,
      providerID: input.providerID,
      modelID: input.modelID,
      status: 'saved',
      messages: input.messages,
      sources: input.sources,
      createdAt: '2026-07-28T00:00:00Z',
      updatedAt: '2026-07-28T00:00:00Z',
    },
  }
}

export async function listAIHistories() {
  return { items: [] }
}

export async function getAIHistory(id) {
  calls.historyGets.push(id)
  return { error: { code: 'AI_HISTORY_NOT_FOUND' } }
}

export async function deleteAIHistory(id) {
  calls.historyDeletes.push(id)
  return { deleted: true }
}

export async function deleteAllAIHistories() {
  calls.historyDeleteAll += 1
  return { deleted: true }
}

export async function runAIWriting(input) {
  calls.writing.push(clone(input))
  const error = writingErrors.shift()
  if (error) return { error }
  return {
    result: {
      providerID: input.providerID,
      modelID: input.modelID,
      kind: input.kind,
      content: '生成結果マーカー',
      sources: [clone(source)],
    },
  }
}

export async function saveAIArtifact(input) {
  calls.saveArtifact.push(clone(input))
  return {
    artifact: {
      id: input.id ?? 'artifact-1',
      kind: input.kind,
      title: input.title,
      providerID: input.providerID,
      modelID: input.modelID,
      content: input.content,
      status: 'saved',
      sources: input.sources,
      createdAt: '2026-07-28T00:00:00Z',
      updatedAt: '2026-07-28T00:00:00Z',
    },
  }
}

export async function listAIArtifacts() {
  return { items: [] }
}

export async function getAIArtifact() {
  return { error: { code: 'AI_ARTIFACT_NOT_FOUND' } }
}

export async function deleteAIArtifact(id) {
  calls.artifactDeletes.push(id)
  return { deleted: true }
}

export async function deleteAllAIArtifacts() {
  calls.artifactDeleteAll += 1
  return { deleted: true }
}

export async function deleteAllAIWritingArtifacts() {
  calls.artifactDeleteAll += 1
  return { deleted: true }
}
`, 'utf8')

try {
  const [assistantSource, writingSource, assistantPanelSource, writingPanelSource] = await Promise.all([
    readFile(assistantPath, 'utf8'),
    readFile(writingPath, 'utf8'),
    readFile(assistantPanelPath, 'utf8'),
    readFile(writingPanelPath, 'utf8'),
  ])
  for (const [name, source] of [
    ['assistant store', assistantSource],
    ['writing store', writingSource],
    ['assistant panel', assistantPanelSource],
    ['writing panel', writingPanelSource],
  ]) {
    assert.doesNotMatch(source, /localStorage/, `${name} must not persist AI state in localStorage`)
  }
  assert.match(assistantPanelSource, /window\.confirm/, 'assistant generation must require confirmation')
  assert.match(assistantPanelSource, /Agent変更提案/, 'Agent confirmation must explain that it only creates a proposal')
  assert.match(assistantPanelSource, /agentTarget/, 'Agent requests must identify the active-note revision')
  assert.match(assistantPanelSource, /formatCharacterCount/, 'assistant context previews must show note character counts')
  assert.match(assistantPanelSource, /contentTruncated/, 'assistant context previews must show truncated context')
  assert.match(writingPanelSource, /window\.confirm/, 'writing generation must require confirmation')
  assert.match(writingPanelSource, /formatCharacterCount/, 'writing context previews must show note character counts')
  assert.match(writingPanelSource, /contentTruncated/, 'writing context previews must show truncated context')
  assert.match(assistantPanelSource, /履歴を保存/, 'assistant history must be explicitly saveable')
  assert.match(writingPanelSource, /成果物を保存/, 'writing artifact must be explicitly saveable')
  assert.match(assistantPanelSource, /送信済み・応答待ち/)
  assert.match(writingPanelSource, /applyAIWritingContent/)

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-ai.mjs')).href)
  const { useAIAssistantStore } = await import(pathToFileURL(path.join(outDir, 'useAIAssistantStore.mjs')).href)
  const { useAIWritingStore } = await import(pathToFileURL(path.join(outDir, 'useAIWritingStore.mjs')).href)
  const longContextSource = Object.freeze({
    noteID: 'note-1',
    title: '長文コンテキスト',
    revision: 11,
    characterCount: 24_002,
    contentByte: 16 * 1024,
    totalContentByte: 72_006,
    contentTruncated: true,
    createdAt: '2026-08-24T00:00:00Z',
    updatedAt: '2026-08-24T01:00:00Z',
  })

  const assistant = useAIAssistantStore()
  assert.equal(await assistant.previewContext({
    kind: 'qa',
    question: '確認したいこと',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(mock.calls.assistant.length, 0, 'context preview must not call the provider')
  const deferredAssistant = mock.deferAssistant()
  const lateAssistant = assistant.ask({
    kind: 'qa',
    question: '遅延する質問',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  })
  for (let index = 0; index < 5 && mock.calls.assistant.length === 0; index += 1) {
    await Promise.resolve()
  }
  assert.equal(mock.calls.assistant.length, 1, 'the deferred provider request must have started')
  assert.equal(assistant.state, 'generating', 'a user message must be acknowledged before the provider responds')
  assert.deepEqual(assistant.messages, [{ role: 'user', content: '遅延する質問' }])
  assert.equal(
    await assistant.loadHistory('history-while-generating'),
    false,
    'history loading must be rejected while a response is in flight',
  )
  assert.deepEqual(
    mock.calls.historyGets,
    [],
    'busy history rejection must happen before reading and replacing conversation state',
  )
  const clearedRequestID = mock.calls.assistant.at(-1).requestID
  assert.match(clearedRequestID, /^assistant-\S+$/, 'assistant runs must carry a non-empty request ID')
  assistant.clearConversation()
  assert.equal(
    mock.calls.cancelAssistant.at(-1),
    clearedRequestID,
    'clearing an active conversation must request best-effort backend cancellation',
  )
  deferredAssistant.resolve()
  assert.equal(await lateAssistant, false, 'a response that arrives after clear must be discarded')
  assert.equal(assistant.state, 'idle')
  assert.deepEqual(assistant.messages, [])

  const contextCallsBeforeCancel = mock.calls.context.length
  const assistantCallsBeforeContextCancel = mock.calls.assistant.length
  const deferredContext = mock.deferContext()
  const contextCancelRequest = assistant.ask({
    kind: 'qa',
    question: 'コンテキスト確認中に停止する質問',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  })
  for (let index = 0; index < 5 && mock.calls.context.length === contextCallsBeforeCancel; index += 1) {
    await Promise.resolve()
  }
  assert.equal(assistant.state, 'loading-context')
  assert.equal(await assistant.cancel(), true)
  assert.equal(assistant.state, 'error')
  assert.equal(assistant.error?.code, 'AI_CANCELLED')
  deferredContext.resolve()
  assert.equal(await contextCancelRequest, false)
  assert.equal(
    mock.calls.assistant.length,
    assistantCallsBeforeContextCancel,
    'canceling context preparation must stop before the provider call',
  )
  assert.deepEqual(assistant.messages, [])
  assistant.clearConversation()

  const cancelAgentRequest = {
    kind: 'qa',
    mode: 'agent',
    question: '生成中のAgent依頼を停止する',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
    agentTarget: { noteID: 'note-1', baseRevision: 1 },
  }
  assert.equal(await assistant.previewContext(cancelAgentRequest), true)
  mock.setAssistantProposal({
    targetNoteID: 'note-1',
    targetTitle: '対象ノート',
    baseRevision: 1,
    reason: 'キャンセル時には採用しない',
    before: '変更前',
    after: '変更後',
    affectedFields: ['content'],
  })
  const savesBeforeCancel = mock.calls.saveHistory.length
  const assistantCallsBeforeCancel = mock.calls.assistant.length
  const cancelCallsBeforeCancel = mock.calls.cancelAssistant.length
  const deferredCanceledAssistant = mock.deferAssistant()
  const canceledAssistant = assistant.ask(cancelAgentRequest)
  for (let index = 0; index < 5 && mock.calls.assistant.length === assistantCallsBeforeCancel; index += 1) {
    await Promise.resolve()
  }
  const canceledRequestID = mock.calls.assistant.at(-1).requestID
  mock.queueAssistantError({ code: 'AI_CANCELLED', raw: 'raw-user-cancel' })
  assert.equal(await assistant.cancel(), true)
  assert.equal(assistant.state, 'canceling')
  assert.equal(assistant.isBusy, true, 'accepted cancellation must stay busy until the run terminates')
  assert.equal(mock.calls.cancelAssistant.at(-1), canceledRequestID)
  assert.equal(await assistant.cancel(), false, 'a canceling request must reject duplicate stop actions')
  assert.equal(mock.calls.cancelAssistant.length, cancelCallsBeforeCancel + 1)
  deferredCanceledAssistant.resolve()
  assert.equal(await canceledAssistant, false)
  assert.equal(assistant.state, 'error')
  assert.equal(assistant.isBusy, false)
  assert.equal(assistant.error?.code, 'AI_CANCELLED')
  assert.equal(assistant.error?.message, 'AI処理をキャンセルしました。')
  assert.doesNotMatch(assistant.error?.message ?? '', /raw-user-cancel/)
  assert.deepEqual(assistant.messages, [{ role: 'user', content: cancelAgentRequest.question }])
  assert.equal(assistant.proposal, null, 'a canceled Agent run must not expose a proposal')
  assert.equal(mock.calls.saveHistory.length, savesBeforeCancel, 'a canceled Agent run must not save history')
  await Promise.resolve()
  assert.equal(mock.calls.assistant.length, assistantCallsBeforeCancel + 1, 'cancellation must not retry automatically')
  assistant.clearConversation()
  mock.setAssistantProposal(null)
  assert.equal(await assistant.ask({
    ...cancelAgentRequest,
    question: '停止後に明示的に再実行する',
  }), true, 'a later run must start only after an explicit retry')
  assert.equal(mock.calls.assistant.length, assistantCallsBeforeCancel + 2)
  assert.equal(mock.calls.saveHistory.length, savesBeforeCancel)
  assistant.clearConversation()

  const cancelFailureRequest = {
    kind: 'qa',
    question: '停止API失敗後も終端を監視する',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }
  assert.equal(await assistant.previewContext(cancelFailureRequest), true)
  const deferredAfterCancelFailure = mock.deferAssistant()
  const assistantCallsBeforeCancelFailure = mock.calls.assistant.length
  const requestAfterCancelFailure = assistant.ask(cancelFailureRequest)
  for (let index = 0; index < 5 && mock.calls.assistant.length === assistantCallsBeforeCancelFailure; index += 1) {
    await Promise.resolve()
  }
  mock.queueAssistantCancelResponse({
    canceled: false,
    error: { code: 'AI_NETWORK_UNAVAILABLE', raw: 'raw-cancel-api-failure' },
  })
  assert.equal(await assistant.cancel(), false)
  assert.equal(assistant.state, 'generating')
  assert.equal(assistant.isBusy, true, 'a failed stop request must keep monitoring the original run')
  assert.equal(assistant.error?.code, 'AI_NETWORK_UNAVAILABLE')
  assert.equal(assistant.error?.message, 'ネットワークに接続できません。')
  assert.doesNotMatch(assistant.error?.message ?? '', /raw-cancel-api-failure/)
  deferredAfterCancelFailure.resolve()
  assert.equal(await requestAfterCancelFailure, true)
  assert.equal(assistant.state, 'success')
  assert.equal(assistant.error, null, 'the original successful terminal result must clear the stop error')
  assistant.clearConversation()

  mock.setSource(longContextSource)
  const assistantBoundaryQuestion = 'a'.repeat(8000)
  const assistantCallsBeforeLongContext = mock.calls.assistant.length
  const assistantSavesBeforeLongContext = mock.calls.saveHistory.length
  const longAssistantRequest = {
    kind: 'qa',
    question: assistantBoundaryQuestion,
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }
  assert.equal(await assistant.previewContext(longAssistantRequest), true)
  assert.equal(
    mock.calls.assistant.length,
    assistantCallsBeforeLongContext,
    'long context preview must not call the provider',
  )
  assert.deepEqual(
    assistant.contextSources[0],
    longContextSource,
    'assistant preview must preserve the backend truncation metadata',
  )
  assert.equal(await assistant.ask(longAssistantRequest), true)
  assert.equal(mock.calls.assistant.length, assistantCallsBeforeLongContext + 1)
  assert.equal(mock.calls.assistant.at(-1).question.length, 8000)
  assert.deepEqual(mock.calls.assistant.at(-1).expectedSources, [
    { noteID: 'note-1', inputRevision: 11 },
  ])
  assert.deepEqual(
    assistant.sources[0],
    longContextSource,
    'assistant result must preserve the backend truncation metadata',
  )
  assert.equal(
    mock.calls.saveHistory.length,
    assistantSavesBeforeLongContext,
    'long assistant responses must not be saved automatically',
  )
  assistant.clearConversation()

  const assistantCallsBeforeContextLimit = mock.calls.assistant.length
  const assistantContextCallsBeforeLimit = mock.calls.context.length
  const assistantSavesBeforeContextLimit = mock.calls.saveHistory.length
  mock.queueContextError({ code: 'AI_INPUT_TOO_LARGE', raw: 'raw-assistant-context-limit' })
  assert.equal(await assistant.ask({
    kind: 'qa',
    mode: 'agent',
    question: '長文コンテキストを整理して',
    noteIDs: ['note-1', 'note-2', 'note-3', 'note-4'],
    searchQuery: '',
    includeBacklinks: false,
    agentTarget: { noteID: 'note-1', baseRevision: 11 },
  }), false)
  assert.equal(mock.calls.context.length, assistantContextCallsBeforeLimit + 1)
  assert.equal(
    mock.calls.assistant.length,
    assistantCallsBeforeContextLimit,
    'an oversized context must stop before the assistant provider call',
  )
  assert.equal(mock.calls.saveHistory.length, assistantSavesBeforeContextLimit)
  assert.equal(assistant.state, 'error')
  assert.equal(assistant.error?.code, 'AI_INPUT_TOO_LARGE')
  assert.equal(
    assistant.error?.message,
    '送信する質問または参照資料が大きすぎます。対象を減らして再試行してください。',
  )
  assert.doesNotMatch(assistant.error?.message ?? '', /raw-assistant-context-limit/)
  assert.deepEqual(assistant.messages, [])
  assert.equal(assistant.proposal, null)
  await Promise.resolve()
  assert.equal(
    mock.calls.assistant.length,
    assistantCallsBeforeContextLimit,
    'an oversized context must not retry automatically',
  )
  assistant.clearConversation()
  assert.equal(await assistant.ask({
    kind: 'qa',
    mode: 'agent',
    question: '対象を減らして再試行',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
    agentTarget: { noteID: 'note-1', baseRevision: 11 },
  }), true, 'a smaller context must run only after an explicit retry')
  assert.equal(mock.calls.assistant.length, assistantCallsBeforeContextLimit + 1)
  assistant.clearConversation()

  mock.setSource({
    noteID: 'note-1',
    title: '対象ノート',
    revision: 1,
    contentByte: 120,
  })

  for (const failure of [
    {
      code: 'AI_TIMEOUT',
      message: 'AI プロバイダーが時間内に応答しませんでした。',
      question: 'タイムアウトするAgent要求',
    },
    {
      code: 'AI_CANCELLED',
      message: 'AI処理をキャンセルしました。',
      question: 'キャンセル済みのAgent要求',
    },
    {
      code: 'AI_INVALID_RESPONSE',
      message: 'AI プロバイダーから有効な回答を受け取れませんでした。',
      question: '不正応答になるAgent要求',
      retryQuestion: '不正応答後に明示再試行',
    },
    {
      code: 'AI_INPUT_TOO_LARGE',
      message: '送信する質問または参照資料が大きすぎます。対象を減らして再試行してください。',
      question: '入力上限になるAgent要求',
      retryQuestion: '対象を減らして明示再試行',
    },
  ]) {
    const callsBeforeFailure = mock.calls.assistant.length
    const savesBeforeFailure = mock.calls.saveHistory.length
    mock.queueAssistantError({ code: failure.code, raw: `raw-${failure.code}` })
    assert.equal(await assistant.ask({
      kind: 'qa',
      mode: 'agent',
      question: failure.question,
      noteIDs: ['note-1'],
      searchQuery: '',
      includeBacklinks: false,
      agentTarget: { noteID: 'note-1', baseRevision: 1 },
    }), false)
    assert.equal(mock.calls.assistant.length, callsBeforeFailure + 1, `${failure.code} must not retry automatically`)
    assert.equal(mock.calls.saveHistory.length, savesBeforeFailure, `${failure.code} must not save history`)
    assert.equal(assistant.state, 'error')
    assert.equal(assistant.error?.code, failure.code)
    assert.equal(assistant.error?.message, failure.message)
    assert.doesNotMatch(assistant.error?.message ?? '', new RegExp(`raw-${failure.code}`))
    assert.deepEqual(assistant.messages, [{ role: 'user', content: failure.question }])
    assert.equal(assistant.proposal, null, `${failure.code} must not retain an Agent proposal`)
    await Promise.resolve()
    assert.equal(mock.calls.assistant.length, callsBeforeFailure + 1, `${failure.code} must not retry in a later tick`)
    assistant.clearConversation()
    if (failure.retryQuestion) {
      assert.equal(await assistant.ask({
        kind: 'qa',
        mode: 'agent',
        question: failure.retryQuestion,
        noteIDs: ['note-1'],
        searchQuery: '',
        includeBacklinks: false,
        agentTarget: { noteID: 'note-1', baseRevision: 1 },
      }), true, `${failure.code} must succeed only after an explicit retry`)
      assert.equal(mock.calls.assistant.length, callsBeforeFailure + 2)
      assert.equal(mock.calls.saveHistory.length, savesBeforeFailure)
      assistant.clearConversation()
    }
  }

  assert.equal(await assistant.previewContext({
    kind: 'qa',
    question: '確認したいこと',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(await assistant.ask({
    kind: 'qa',
    question: '確認したいこと',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(mock.calls.assistant.at(-1).expectedSources[0].inputRevision, 1)
  mock.setSource({
    noteID: 'note-2',
    title: '追加ノート',
    revision: 4,
    contentByte: 80,
  })
  assert.equal(await assistant.previewContext({
    kind: 'qa',
    question: '追加ノートについて',
    noteIDs: ['note-2'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(await assistant.ask({
    kind: 'qa',
    question: '追加ノートについて',
    noteIDs: ['note-2'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.deepEqual(
    assistant.sources.map((item) => [item.noteID, item.revision]),
    [['note-1', 1], ['note-2', 4]],
    'a multi-turn conversation must retain the source snapshot from every turn',
  )
  assert.equal(mock.calls.saveHistory.length, 0, 'assistant responses must not be saved automatically')
  assert.equal(await assistant.save('質問履歴'), true)
  assert.equal(mock.calls.saveHistory.length, 1, 'assistant history must be saved only explicitly')
  assert.deepEqual(
    mock.calls.saveHistory[0].sources,
    [
      { noteID: 'note-1', inputRevision: 1 },
      { noteID: 'note-2', inputRevision: 4 },
    ],
  )
  assistant.markStaleForRevision('note-1', 2)
  assert.equal(assistant.state, 'stale', 'a newer source revision must stale the in-memory conversation')
  assert.equal(await assistant.save('古い履歴'), false, 'stale conversations must not be saved')
  assert.equal(await assistant.removeAllHistories(), true)
  assert.equal(mock.calls.historyDeleteAll, 1)

  mock.setSource({
    noteID: 'note-1',
    title: '対象ノート',
    revision: 5,
    contentByte: 120,
  })
  mock.setAssistantProposal({
    targetNoteID: 'note-1',
    targetTitle: '対象ノート',
    baseRevision: 5,
    reason: '重複を減らすため',
    before: '変更前',
    after: '変更後',
    affectedFields: ['content'],
  })
  assert.equal(await assistant.ask({
    kind: 'qa',
    mode: 'agent',
    question: '本文を整理して',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
    agentTarget: { noteID: 'note-1', baseRevision: 5 },
  }), true)
  assert.deepEqual(mock.calls.assistant.at(-1).agentTarget, { noteID: 'note-1', baseRevision: 5 })
  assert.equal(assistant.proposal?.targetNoteID, 'note-1')
  assistant.clearConversation()
  assert.equal(assistant.proposal, null, 'clearing a conversation must also clear its proposed edit')
  mock.setAssistantProposal(null)

  const writing = useAIWritingStore()
  writing.clear()
  mock.setSource(longContextSource)
  const writingBoundaryInstruction = 'w'.repeat(12000)
  const writingCallsBeforeLongContext = mock.calls.writing.length
  const writingSavesBeforeLongContext = mock.calls.saveArtifact.length
  const longWritingContext = {
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }
  assert.equal(await writing.previewContext(longWritingContext), true)
  assert.equal(
    mock.calls.writing.length,
    writingCallsBeforeLongContext,
    'long writing context preview must not call the provider',
  )
  assert.deepEqual(
    writing.contextSources[0],
    longContextSource,
    'writing preview must preserve the backend truncation metadata',
  )
  assert.equal(await writing.generate({
    providerID: 'openrouter',
    modelID: 'openai/gpt-test',
    kind: 'document',
    instruction: writingBoundaryInstruction,
    ...longWritingContext,
  }), true)
  assert.equal(mock.calls.writing.length, writingCallsBeforeLongContext + 1)
  assert.equal(mock.calls.writing.at(-1).instruction.length, 12000)
  assert.deepEqual(mock.calls.writing.at(-1).expectedSources, [
    { noteID: 'note-1', inputRevision: 11 },
  ])
  assert.deepEqual(
    writing.sources[0],
    longContextSource,
    'writing result must preserve the backend truncation metadata',
  )
  assert.equal(mock.calls.saveArtifact.length, writingSavesBeforeLongContext)
  writing.clear()

  const writingCallsBeforeContextLimit = mock.calls.writing.length
  const writingContextCallsBeforeLimit = mock.calls.context.length
  const writingSavesBeforeContextLimit = mock.calls.saveArtifact.length
  mock.queueContextError({ code: 'AI_INPUT_TOO_LARGE', raw: 'raw-writing-context-limit' })
  assert.equal(await writing.generate({
    providerID: 'openrouter',
    modelID: 'openai/gpt-test',
    kind: 'document',
    instruction: writingBoundaryInstruction,
    noteIDs: ['note-1', 'note-2', 'note-3', 'note-4'],
    searchQuery: '',
    includeBacklinks: false,
  }), false)
  assert.equal(mock.calls.context.length, writingContextCallsBeforeLimit + 1)
  assert.equal(
    mock.calls.writing.length,
    writingCallsBeforeContextLimit,
    'an oversized context must stop before the writing provider call',
  )
  assert.equal(mock.calls.saveArtifact.length, writingSavesBeforeContextLimit)
  assert.equal(writing.state, 'error')
  assert.equal(writing.error?.code, 'AI_INPUT_TOO_LARGE')
  assert.equal(
    writing.error?.message,
    '送信する目的または参照資料が大きすぎます。対象を減らして再試行してください。',
  )
  assert.doesNotMatch(writing.error?.message ?? '', /raw-writing-context-limit/)
  assert.equal(writing.content, '')
  assert.deepEqual(writing.sources, [])
  assert.equal(writing.targetSource, null)
  await Promise.resolve()
  assert.equal(
    mock.calls.writing.length,
    writingCallsBeforeContextLimit,
    'an oversized writing context must not retry automatically',
  )
  writing.clear()
  assert.equal(await writing.generate({
    providerID: 'openrouter',
    modelID: 'openai/gpt-test',
    kind: 'document',
    instruction: '対象を減らして文章を再生成',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true, 'a smaller writing context must run only after an explicit retry')
  assert.equal(mock.calls.writing.length, writingCallsBeforeContextLimit + 1)
  assert.equal(mock.calls.saveArtifact.length, writingSavesBeforeContextLimit)
  writing.clear()

  for (const failure of [
    {
      code: 'AI_TIMEOUT',
      message: 'AI プロバイダーが時間内に応答しませんでした。',
      instruction: 'タイムアウトする文章作成',
    },
    {
      code: 'AI_INVALID_RESPONSE',
      message: 'AI プロバイダーから有効な文章を受け取れませんでした。',
      instruction: '不正応答になる文章作成',
      retryInstruction: '不正応答後に明示再生成',
    },
    {
      code: 'AI_INPUT_TOO_LARGE',
      message: '送信する目的または参照資料が大きすぎます。対象を減らして再試行してください。',
      instruction: writingBoundaryInstruction,
      retryInstruction: '目的を短くして明示再生成',
    },
  ]) {
    const callsBeforeFailure = mock.calls.writing.length
    const savesBeforeFailure = mock.calls.saveArtifact.length
    mock.queueWritingError({ code: failure.code, raw: `raw-writing-${failure.code}` })
    assert.equal(await writing.generate({
      providerID: 'openrouter',
      modelID: 'openai/gpt-test',
      kind: 'document',
      instruction: failure.instruction,
      noteIDs: ['note-1'],
      searchQuery: '',
      includeBacklinks: false,
    }), false)
    assert.equal(mock.calls.writing.length, callsBeforeFailure + 1, `${failure.code} must not retry automatically`)
    assert.equal(mock.calls.saveArtifact.length, savesBeforeFailure, `${failure.code} must not save an artifact`)
    assert.equal(writing.state, 'error')
    assert.equal(writing.error?.code, failure.code)
    assert.equal(writing.error?.message, failure.message)
    assert.doesNotMatch(writing.error?.message ?? '', new RegExp(`raw-writing-${failure.code}`))
    assert.equal(writing.content, '')
    assert.deepEqual(writing.sources, [])
    assert.equal(writing.targetSource, null)
    await Promise.resolve()
    assert.equal(mock.calls.writing.length, callsBeforeFailure + 1, `${failure.code} must not retry in a later tick`)
    writing.clear()
    if (failure.retryInstruction) {
      assert.equal(await writing.generate({
        providerID: 'openrouter',
        modelID: 'openai/gpt-test',
        kind: 'document',
        instruction: failure.retryInstruction,
        noteIDs: ['note-1'],
        searchQuery: '',
        includeBacklinks: false,
      }), true, `${failure.code} must succeed only after an explicit retry`)
      assert.equal(mock.calls.writing.length, callsBeforeFailure + 2)
      assert.equal(mock.calls.saveArtifact.length, savesBeforeFailure)
      writing.clear()
    }
  }

  mock.setSource({
    noteID: 'note-1',
    title: '対象ノート',
    revision: 1,
    contentByte: 120,
  })
  assert.equal(await writing.previewContext({
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(await writing.generate({
    providerID: 'openrouter',
    modelID: 'openai/gpt-test',
    kind: 'document',
    instruction: '設計文書を作る',
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(mock.calls.writing.at(-1).expectedSources[0].inputRevision, 1)
  assert.equal(mock.calls.saveArtifact.length, 0, 'writing output must not be saved automatically')
  writing.updateContent('編集済み成果物')
  assert.equal(await writing.save('設計文書'), true)
  assert.equal(mock.calls.saveArtifact.length, 1, 'writing artifact must be saved only explicitly')
  assert.equal(mock.calls.saveArtifact[0].content, '編集済み成果物')
  writing.markStaleForRevision('note-1', 2)
  assert.equal(writing.state, 'stale', 'a newer source revision must stale the in-memory artifact')
  assert.equal(await writing.save('古い成果物'), false, 'stale artifacts must not be saved')
  assert.equal(await writing.removeAllArtifacts(), true)
  assert.equal(mock.calls.artifactDeleteAll, 1)

  console.log('AI v3 tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
