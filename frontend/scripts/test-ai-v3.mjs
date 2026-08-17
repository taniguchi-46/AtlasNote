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
let pendingAssistant
let assistantProposal

export function setSource(nextSource) {
  source = clone(nextSource)
}

export function deferAssistant() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  pendingAssistant = { promise, resolve }
  return pendingAssistant
}

export function setAssistantProposal(nextProposal) {
  assistantProposal = nextProposal ? clone(nextProposal) : undefined
}

export async function prepareAIContext(input) {
  calls.context.push(clone(input))
  return { sources: [clone(source)] }
}

export async function runAIAssistant(input) {
  calls.assistant.push(clone(input))
  if (pendingAssistant) {
    const deferred = pendingAssistant
    pendingAssistant = undefined
    await deferred.promise
  }
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
  assistant.clearConversation()
  deferredAssistant.resolve()
  assert.equal(await lateAssistant, false, 'a response that arrives after clear must be discarded')
  assert.equal(assistant.state, 'idle')
  assert.deepEqual(assistant.messages, [])

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

  mock.setSource({
    noteID: 'note-1',
    title: '対象ノート',
    revision: 1,
    contentByte: 120,
  })
  const writing = useAIWritingStore()
  writing.clear()
  assert.equal(await writing.previewContext({
    noteIDs: ['note-1'],
    searchQuery: '',
    includeBacklinks: false,
  }), true)
  assert.equal(mock.calls.writing.length, 0, 'writing context preview must not call the provider')
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
