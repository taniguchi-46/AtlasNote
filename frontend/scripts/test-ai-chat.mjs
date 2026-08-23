import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const storePath = path.join(rootDir, 'src', 'stores', 'useAIChatStore.ts')
const timelineUtilityPath = path.join(rootDir, 'src', 'utils', 'aiWorkspaceTimeline.ts')
const outDir = path.join(rootDir, '.tmp', 'ai-chat-test')
const outFile = path.join(outDir, 'useAIChatStore.mjs')
const timelineUtilityOutFile = path.join(outDir, 'aiWorkspaceTimeline.mjs')
const mockNotesFile = path.join(outDir, 'mock-notes.mjs')

const [storeSource, timelineUtilitySource] = await Promise.all([
  readFile(storePath, 'utf8'),
  readFile(timelineUtilityPath, 'utf8'),
])
assert.doesNotMatch(storeSource, /localStorage/, 'AI chat content and tool traces must remain memory-only')
assert.match(storeSource, /ref<AIChatMode>\('ask'\)/, 'the chat store must use the Ask/Agent mode contract')
assert.match(storeSource, /kind:\s*'active-note'/, 'the active note must have a distinct fixed-context kind')
assert.match(storeSource, /kind:\s*'notebook'/, 'notebooks must be represented as a context scope')
assert.match(storeSource, /MAX_CONTEXT_NOTE_IDS\s*=\s*10/, 'resolved note context must respect the backend limit')
assert.match(storeSource, /kind:\s*'tool-trace'/, 'structured tool progress must be represented in the in-memory timeline')
assert.match(storeSource, /kind:\s*'agent-proposal'/, 'Agent body proposals must remain in the in-memory timeline')
assert.match(storeSource, /function markAgentProposalStale/, 'Agent proposals must become conflicts when the target revision changes')
assert.match(storeSource, /\|\s*'writing'/, 'writing must remain an allowlisted chat tool')

await mkdir(outDir, { recursive: true })

const compiled = ts.transpileModule(
  storeSource.replace("from '../api/notes'", "from './mock-notes.mjs'"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: storePath,
  },
)
await writeFile(outFile, compiled.outputText, 'utf8')
const compiledTimelineUtility = ts.transpileModule(timelineUtilitySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
  fileName: timelineUtilityPath,
})
await writeFile(timelineUtilityOutFile, compiledTimelineUtility.outputText, 'utf8')
await writeFile(mockNotesFile, createMockNotesModule(), 'utf8')

try {
  setActivePinia(createPinia())
  const mockNotes = await import(pathToFileURL(mockNotesFile).href)
  const { useAIChatStore } = await import(pathToFileURL(outFile).href)
  const { recordAssistantTimelineFailure } = await import(pathToFileURL(timelineUtilityOutFile).href)
  const store = useAIChatStore()

  store.setActiveNoteContext({ id: 'active-note', title: '開いているノート' })
  assert.deepEqual(store.contexts, [{
    kind: 'active-note',
    id: 'active-note',
    label: '開いているノート',
  }])
  assert.deepEqual(store.resolvedNoteIDs, ['active-note'])
  assert.equal(store.addNoteContext('active-note', '重複'), false, 'the fixed active note must not be duplicated')

  assert.equal(await store.loadContextCatalog(), true)
  assert.equal(store.isContextCatalogReady, true)
  assert.equal(mockNotes.calls.list, 1)
  assert.equal(store.catalogNotes.some((note) => note.isTrashed), false, 'trashed notes must not be selectable')
  assert.equal(store.addNoteContext('explicit-note', '明示ノート'), true)
  assert.equal(store.addNoteContext('explicit-note', '重複'), false, 'explicit note contexts must be deduplicated')
  assert.equal(store.addNotebookContext('notebook-a', '設計'), true)
  assert.equal(store.addNotebookContext('notebook-a', '重複'), false, 'notebook scopes must be deduplicated')

  assert.equal(store.resolvedNoteIDs.length, 10, 'the active note and explicit contexts must share the ten-note limit')
  assert.deepEqual(
    store.resolvedNoteIDs.slice(0, 2),
    ['active-note', 'explicit-note'],
    'the fixed active note and explicit notes must take precedence over notebook search results',
  )
  assert.deepEqual(
    store.resolvedNoteIDs.slice(2),
    Array.from({ length: 8 }, (_, index) => `notebook-note-${String(10 - index).padStart(2, '0')}`),
    'notebook scope results must be resolved by updatedAt descending within the shared limit',
  )
  assert.equal(store.notebookResolvedCounts['notebook-a'], 8)
  assert.equal(store.notebookOmissions['notebook-a'], 2, 'overflowing notebook results must be reported, not silently lost')
  assert.equal(store.addNotebookContext('notebook-b', '別Notebook'), true)
  assert.equal(store.resolvedNoteIDs.length, 10, 'multiple notebook scopes must share one ten-note limit')
  assert.equal(store.notebookResolvedCounts['notebook-b'], 0)
  assert.equal(store.notebookOmissions['notebook-b'], 2)

  store.removeContext('note', 'active-note')
  assert.equal(store.activeNoteContext?.id, 'active-note', 'the fixed active-note context must not be removable')
  store.removeContext('note', 'explicit-note')
  assert.equal(store.contexts.some((context) => context.id === 'explicit-note'), false)

  store.setMode('agent')
  store.setDraft('候補を出して')
  store.selectTool('writing')
  const userID = store.appendUserMessage('文章を作成して')
  const toolID = store.appendToolTrace('writing', '文章を生成しています')
  store.updateTimelineEntry(toolID, { status: 'success', content: '文章を生成しました' })
  store.appendAssistantMessage('文章案です', [{ url: 'https://example.com/source', title: '参照元' }])

  assert.equal(store.mode, 'agent')
  assert.equal(store.draft, '候補を出して')
  assert.equal(store.selectedTool, 'writing')
  assert.equal(store.timeline[0].id, userID)
  assert.deepEqual(
    store.timeline.map((entry) => [entry.role, entry.kind]),
    [
      ['user', 'message'],
      ['tool', 'tool-trace'],
      ['assistant', 'message'],
    ],
    'messages and tool traces must share one ordered timeline',
  )
  assert.equal(store.timeline[1].status, 'success')
  assert.equal(store.timeline[2].citations?.[0]?.url, 'https://example.com/source')

  const proposal = {
    targetNoteID: 'active-note',
    targetTitle: '開いているノート',
    baseRevision: 1,
    reason: '重複を減らすため',
    before: '変更前',
    after: '変更後',
    affectedFields: ['content'],
  }
  const proposalID = store.appendAgentProposalPlaceholder()
  assert.equal(store.timeline.at(-1)?.proposalState, 'generating')
  store.resolveAgentProposal(proposalID, '本文の変更を提案します。', proposal)
  assert.equal(store.timeline.at(-1)?.proposalState, 'awaiting-review')
  assert.equal(store.timeline.at(-1)?.proposal?.before, '変更前')
  store.setAgentProposalState(proposalID, 'applied', 'Agentが本文を更新しました。')
  assert.equal(store.timeline.at(-1)?.proposalState, 'applied')
  assert.equal(store.timeline.at(-1)?.proposal?.before, '変更前', 'applied proposal must retain the before diff')
  assert.equal(store.timeline.at(-1)?.proposal?.after, '変更後', 'applied proposal must retain the after diff')
  store.markAgentProposalStale('active-note', 2)
  assert.equal(store.timeline.at(-1)?.proposalState, 'applied', 'applied proposal must remain viewable after later revisions')

  const failedProposalID = store.appendAgentProposalPlaceholder()
  store.resolveAgentProposal(failedProposalID, '本文の変更を提案します。', proposal)
  store.setAgentProposalState(failedProposalID, 'save-failure', '本文の保存に失敗しました。')
  const failedProposal = store.timeline.find((entry) => entry.id === failedProposalID)
  assert.equal(failedProposal?.proposalState, 'save-failure')
  assert.deepEqual(
    failedProposal?.proposal,
    proposal,
    'a save failure must retain the proposal payload for review and retry',
  )

  const staleProposalID = store.appendAgentProposalPlaceholder()
  store.resolveAgentProposal(staleProposalID, '本文の変更を提案します。', proposal)
  store.markAgentProposalStale('active-note', 2)
  assert.equal(store.timeline.at(-1)?.proposalState, 'conflict')
  store.discardAgentProposal(staleProposalID)
  assert.equal(store.timeline.at(-1)?.proposalState, 'discarded')
  assert.equal(store.timeline.at(-1)?.proposal, undefined, 'discard must clear the in-memory proposal payload')

  store.setActiveNoteContext({ id: 'next-note', title: '次のノート' })
  assert.equal(store.mode, 'agent', 'mode choice may remain while note-scoped content is reset')
  assert.equal(store.draft, '')
  assert.equal(store.selectedTool, null)
  assert.deepEqual(store.timeline, [])
  assert.deepEqual(store.explicitContexts, [])
  assert.deepEqual(store.resolvedNoteIDs, ['next-note'])

  store.addNotebookContext('notebook-a', '設計')
  store.setDraft('下書き')
  store.appendToolTrace('summary', '要約中')
  store.clearConversation({ keepContexts: true })
  assert.equal(store.draft, '')
  assert.deepEqual(store.timeline, [])
  assert.equal(store.explicitContexts.length, 1, 'an explicit keep-context clear must preserve selected scopes')

  mockNotes.failNextList()
  const preservedCatalogLength = store.catalogNotes.length
  assert.equal(await store.loadContextCatalog(), false)
  assert.equal(store.isContextCatalogReady, false)
  assert.equal(
    store.catalogNotes.length,
    preservedCatalogLength,
    'a transient refresh failure must not erase the last visible catalog',
  )
  assert.match(store.contextError ?? '', /読み込めませんでした/)
  assert.equal(
    store.addNotebookContext('unresolved-notebook', '未解決'),
    false,
    'a notebook scope must not be accepted when its note catalog cannot be resolved',
  )
  assert.match(store.contextError ?? '', /ノート一覧を読み込んで/)

  setActivePinia(createPinia())
  const limitStore = useAIChatStore()
  limitStore.setActiveNoteContext({ id: 'limit-active', title: '上限確認' })
  assert.equal(
    limitStore.addNotebookContext('notebook-a', '未読込'),
    false,
    'a fresh store must resolve the catalog before accepting a notebook context',
  )
  for (let index = 1; index <= 9; index += 1) {
    assert.equal(
      limitStore.addNoteContext(`limit-note-${index}`, `明示ノート${index}`),
      true,
      'the active note plus nine explicit notes must fit the shared limit',
    )
  }
  assert.equal(limitStore.resolvedNoteIDs.length, 10)
  assert.equal(
    limitStore.addNoteContext('limit-overflow', '上限超過'),
    false,
    'an explicit note beyond the ten-note limit must be rejected instead of silently omitted',
  )
  assert.match(limitStore.contextError ?? '', /最大10件/)
  limitStore.removeContext('note', 'limit-note-1')
  assert.equal(limitStore.contextError, null, 'removing a context must clear the actionable limit error')
  assert.equal(limitStore.addNoteContext('limit-overflow', '上限内'), true)
  assert.equal(limitStore.contextError, null, 'a successful context addition must clear the prior error')

  for (const failure of [
    { code: 'AI_TIMEOUT', message: 'AI プロバイダーが時間内に応答しませんでした。' },
    { code: 'AI_CANCELLED', message: 'AI処理をキャンセルしました。' },
  ]) {
    setActivePinia(createPinia())
    const failureStore = useAIChatStore()
    failureStore.setMode('agent')
    failureStore.setDraft(`${failure.code}でも保持する下書き`)
    const userEntryID = failureStore.appendUserMessage(`${failure.code}になるAgent要求`)
    const proposalEntryID = failureStore.appendAgentProposalPlaceholder()
    recordAssistantTimelineFailure({
      errorMessage: failure.message,
      tool: null,
      agentProposalEntryID: proposalEntryID,
      traceID: null,
      removeTimelineEntry: (entryID) => failureStore.removeTimelineEntry(entryID),
      appendError: (content, tool) => failureStore.appendError(content, tool),
      updateTimelineEntry: (entryID, update) => failureStore.updateTimelineEntry(entryID, update),
    })
    assert.equal(failureStore.timeline.some((entry) => entry.id === proposalEntryID), false)
    assert.deepEqual(
      failureStore.timeline.map((entry) => [entry.id, entry.kind, entry.content]),
      [
        [userEntryID, 'message', `${failure.code}になるAgent要求`],
        [failureStore.timeline[1].id, 'error', failure.message],
      ],
      `${failure.code} must retain the user entry and append one safe error`,
    )
    assert.equal(failureStore.draft, `${failure.code}でも保持する下書き`)
    assert.equal(
      failureStore.timeline.filter((entry) => entry.kind === 'error').length,
      1,
      `${failure.code} must append exactly one timeline error`,
    )
  }

  console.log('AI chat tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

function createMockNotesModule() {
  const notebookNotes = Array.from({ length: 10 }, (_, index) => ({
    id: `notebook-note-${String(index + 1).padStart(2, '0')}`,
    notebookId: 'notebook-a',
    title: `Notebook note ${index + 1}`,
    isFavorite: false,
    isPinned: false,
    isTrashed: false,
    revision: 1,
    createdAt: new Date(Date.UTC(2026, 0, index + 1)).toISOString(),
    updatedAt: new Date(Date.UTC(2026, 0, index + 1)).toISOString(),
  }))

  return `
export const calls = { list: 0 }
let shouldFail = false
const notes = ${JSON.stringify([
    {
      id: 'active-note',
      title: 'Active',
      isFavorite: false,
      isPinned: false,
      isTrashed: false,
      revision: 1,
      createdAt: '2026-01-01T00:00:00.000Z',
      updatedAt: '2026-01-01T00:00:00.000Z',
    },
    {
      id: 'explicit-note',
      notebookId: 'notebook-a',
      title: 'Explicit',
      isFavorite: false,
      isPinned: false,
      isTrashed: false,
      revision: 1,
      createdAt: '2026-01-20T00:00:00.000Z',
      updatedAt: '2026-01-20T00:00:00.000Z',
    },
    ...notebookNotes,
    {
      id: 'trashed-note',
      notebookId: 'notebook-a',
      title: 'Trash',
      isFavorite: false,
      isPinned: false,
      isTrashed: true,
      revision: 1,
      createdAt: '2026-01-30T00:00:00.000Z',
      updatedAt: '2026-01-30T00:00:00.000Z',
    },
    {
      id: 'notebook-b-note-01',
      notebookId: 'notebook-b',
      title: 'Notebook B 1',
      isFavorite: false,
      isPinned: false,
      isTrashed: false,
      revision: 1,
      createdAt: '2026-01-11T00:00:00.000Z',
      updatedAt: '2026-01-11T00:00:00.000Z',
    },
    {
      id: 'notebook-b-note-02',
      notebookId: 'notebook-b',
      title: 'Notebook B 2',
      isFavorite: false,
      isPinned: false,
      isTrashed: false,
      revision: 1,
      createdAt: '2026-01-12T00:00:00.000Z',
      updatedAt: '2026-01-12T00:00:00.000Z',
    },
  ])}

export function failNextList() {
  shouldFail = true
}

export async function listNotes() {
  calls.list += 1
  if (shouldFail) {
    shouldFail = false
    throw new Error('list failed')
  }
  return structuredClone(notes)
}
`
}
