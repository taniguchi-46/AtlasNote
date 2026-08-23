import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import ts from 'typescript'

const rootDir = process.cwd()
const utilityPath = path.join(rootDir, 'src', 'utils', 'agentEditProposal.ts')
const highlightUtilityPath = path.join(rootDir, 'src', 'utils', 'agentEditorHighlight.ts')
const noteStorePath = path.join(rootDir, 'src', 'stores', 'useNoteStore.ts')
const proposalCardPath = path.join(rootDir, 'src', 'components', 'AIAgentProposalCard.vue')
const outDir = path.join(rootDir, '.tmp', 'agent-proposal-test')
const outFile = path.join(outDir, 'agentEditProposal.mjs')
const highlightOutFile = path.join(outDir, 'agentEditorHighlight.mjs')
const noteStoreOutFile = path.join(outDir, 'useNoteStore.mjs')
const mockNotesFile = path.join(outDir, 'mock-notes.mjs')

const [utilitySource, highlightUtilitySource, noteStoreSource, proposalCardSource] = await Promise.all([
  readFile(utilityPath, 'utf8'),
  readFile(highlightUtilityPath, 'utf8'),
  readFile(noteStorePath, 'utf8'),
  readFile(proposalCardPath, 'utf8'),
])

assert.match(noteStoreSource, /function applyAgentEditProposal/)
assert.match(noteStoreSource, /noteOperations\.enqueue\(noteId/)
assert.match(noteStoreSource, /expectedRevision: proposal\.baseRevision/)
assert.match(noteStoreSource, /NoteRevisionConflictError/)
assert.match(noteStoreSource, /applyAgentEditHunk\(latest\.content, proposal\)/)
assert.match(noteStoreSource, /proposal\.targetNoteID\.trim\(\)/)
assert.match(noteStoreSource, /return pendingDraft \? 'applied-with-draft-conflict' : 'applied'/)
assert.match(noteStoreSource, /return 'save-failure'/)
assert.match(noteStoreSource, /agentEditorHighlight/)
assert.match(noteStoreSource, /start: patched\.range\.start/)
assert.match(noteStoreSource, /end: patched\.range\.end/)
assert.match(noteStoreSource, /if \(!pendingDraft\) \{[\s\S]*?agentEditorHighlight\.value =/)
assert.match(noteStoreSource, /async function selectNote[\s\S]*?clearAgentEditorHighlight\(\)/)
assert.match(noteStoreSource, /async function newNote[\s\S]*?clearAgentEditorHighlight\(\)/)
const agentApplyStart = noteStoreSource.indexOf('async function applyAgentEditProposal')
const agentApplyEnd = noteStoreSource.indexOf('function discardAllDrafts', agentApplyStart)
const agentApplySource = noteStoreSource.slice(agentApplyStart, agentApplyEnd)
const updateNoteIndex = agentApplySource.indexOf('const updated = await updateNote')
const pendingDraftIndex = agentApplySource.indexOf('const pendingDraft = getDraft(noteId)', updateNoteIndex)
const cancelDraftSaveIndex = agentApplySource.indexOf('autoSave.cancel(noteId)', pendingDraftIndex)
const retainConflictIndex = agentApplySource.indexOf("status: 'conflicted'", cancelDraftSaveIndex)
const applyPersistedNoteIndex = agentApplySource.indexOf('applyPersistedNote(updated)', updateNoteIndex)
const appliedOutcomeIndex = agentApplySource.indexOf("'applied-with-draft-conflict'", applyPersistedNoteIndex)
assert.ok(
  updateNoteIndex >= 0
    && pendingDraftIndex > updateNoteIndex
    && cancelDraftSaveIndex > pendingDraftIndex
    && retainConflictIndex > cancelDraftSaveIndex
    && applyPersistedNoteIndex > updateNoteIndex
    && appliedOutcomeIndex > applyPersistedNoteIndex,
  'a successful Agent save must retain edits made while saving before publishing its outcome',
)
assert.match(agentApplySource, /actualRevision: updated\.revision/)
assert.match(agentApplySource, /error\.value = 'Agentの変更提案をノートへ反映できませんでした'/)
assert.doesNotMatch(agentApplySource, /error\.value = e instanceof Error/)
assert.doesNotMatch(proposalCardSource, /v-html/)
assert.doesNotMatch(proposalCardSource, /<pre>/)
assert.match(proposalCardSource, /createAgentEditVisualDiff/)
assert.match(proposalCardSource, /role="region"/)
assert.match(proposalCardSource, /tabindex="0"/)
assert.match(proposalCardSource, /aria-label="削除される本文"/)
assert.match(proposalCardSource, /aria-label="追加される本文"/)
assert.match(proposalCardSource, /visualDiff\.beforeLines/)
assert.match(proposalCardSource, /visualDiff\.afterLines/)
assert.match(proposalCardSource, /is-removed/)
assert.match(proposalCardSource, /is-added/)
assert.match(proposalCardSource, /is-placeholder/)
assert.match(proposalCardSource, /gridRow: line\.rowNumber \+ 1/)
assert.match(proposalCardSource, /is-word-change/)
assert.match(
  proposalCardSource,
  /\.ai-agent-proposal-diff-line\.is-removed\s*\{[^}]*var\(--color-danger\)[^}]*box-shadow: inset 3px 0 0 var\(--color-danger\)/s,
)
assert.match(
  proposalCardSource,
  /\.ai-agent-proposal-diff-line\.is-added\s*\{[^}]*var\(--color-success\)[^}]*box-shadow: inset 3px 0 0 var\(--color-success\)/s,
)
assert.match(proposalCardSource, /\.is-word-change\s*\{[^}]*font-weight: 600;/s)
assert.match(
  proposalCardSource,
  /\.is-removed \.is-word-change\s*\{[^}]*var\(--color-danger\)/s,
)
assert.match(
  proposalCardSource,
  /\.is-added \.is-word-change\s*\{[^}]*var\(--color-success\)/s,
)
assert.match(proposalCardSource, /@container \(min-width: 520px\)/)
assert.match(proposalCardSource, /grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/)
assert.match(proposalCardSource, /display: contents/)
assert.match(proposalCardSource, /本文へ適用/)
assert.match(proposalCardSource, /提案を破棄/)

await mkdir(outDir, { recursive: true })
const compiled = ts.transpileModule(utilitySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
  fileName: utilityPath,
})
await writeFile(outFile, compiled.outputText, 'utf8')
const compiledHighlightUtility = ts.transpileModule(highlightUtilitySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
  fileName: highlightUtilityPath,
})
await writeFile(highlightOutFile, compiledHighlightUtility.outputText, 'utf8')

for (const utilityName of ['noteAutoSave', 'noteOperationQueue']) {
  const source = await readFile(path.join(rootDir, 'src', 'utils', `${utilityName}.ts`), 'utf8')
  const compiledUtility = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(path.join(outDir, `${utilityName}.mjs`), compiledUtility.outputText, 'utf8')
}

const compiledStore = ts.transpileModule(
  noteStoreSource
    .replace("from '../api/notes'", "from './mock-notes.mjs'")
    .replace("from '../utils/latestRequestGuard'", "from './mock-utilities.mjs'")
    .replace("from '../utils/noteAutoSave'", "from './noteAutoSave.mjs'")
    .replace("from '../utils/noteOperationQueue'", "from './noteOperationQueue.mjs'")
    .replace("from '../utils/requestCounter'", "from './mock-utilities.mjs'")
    .replace("from '../utils/deleteNotesSequentially'", "from './mock-utilities.mjs'")
    .replace("from '../utils/updateNotesSequentially'", "from './mock-utilities.mjs'")
    .replace("from '../utils/agentEditProposal'", "from './agentEditProposal.mjs'")
    .replace("from './useSettingsStore'", "from './mock-stores.mjs'")
    .replace("from './useNotificationStore'", "from './mock-stores.mjs'")
    .replace("from './useAppStore'", "from './mock-stores.mjs'"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: noteStorePath,
  },
)
await writeFile(noteStoreOutFile, compiledStore.outputText, 'utf8')
await writeFile(mockNotesFile, `
export const calls = { updateNote: [] }
let pendingUpdate = null

export class NoteRevisionConflictError extends Error {}

export function deferUpdate() {
  let resolve
  let reject
  const promise = new Promise((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  })
  pendingUpdate = { promise, resolve, reject }
  return pendingUpdate
}

export async function updateNote(id, input) {
  calls.updateNote.push({ id, input })
  if (!pendingUpdate) throw new Error('unexpected note update')
  const update = pendingUpdate
  pendingUpdate = null
  return update.promise
}

export async function listNotesPage() {
  return { items: [], page: 1, pageSize: 100, total: 0, hasNext: false }
}
export async function getNote() { throw new Error('unexpected note read') }
export async function createNote() { throw new Error('unexpected note create') }
export async function deleteNote() { throw new Error('unexpected note delete') }
`, 'utf8')
await writeFile(path.join(outDir, 'mock-utilities.mjs'), `
export class NoteDeleteError extends Error {}

export function createLatestRequestGuard() {
  let version = 0
  return {
    begin() {
      const requestVersion = ++version
      return () => requestVersion === version
    },
  }
}

export function createRequestCounter(onChange) {
  let count = 0
  return {
    begin() {
      count += 1
      onChange(count)
      let ended = false
      return () => {
        if (ended) return
        ended = true
        count -= 1
        onChange(count)
      }
    },
    getCount: () => count,
  }
}

export async function deleteNotesSequentially(ids, operation) {
  for (const id of ids) await operation(id)
  return ids
}

export async function updateNotesSequentially(ids, operation) {
  for (const id of ids) await operation(id)
  return ids
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-stores.mjs'), `
const notifications = { dismissBySource() {}, notify() {} }
const settings = { editorFirstLineStyle: 'paragraph' }
const app = { sortOption: '', sidebarSection: 'all' }

export function useNotificationStore() { return notifications }
export function useSettingsStore() { return settings }
export function useAppStore() { return app }
export function parseNoteSortOption() { return null }
`, 'utf8')

try {
  const { applyAgentEditHunk, createAgentEditVisualDiff } = await import(pathToFileURL(outFile).href)
  const {
    createAgentEditorTextHighlight,
    findChangedTopLevelBlockRange,
  } = await import(pathToFileURL(highlightOutFile).href)

  assert.deepEqual(
    applyAgentEditHunk('前文\n変更前\n後文', { before: '変更前', after: '変更後' }),
    { status: 'ok', content: '前文\n変更後\n後文', range: { start: 3, end: 6 } },
  )
  assert.deepEqual(
    applyAgentEditHunk('変更前と変更前', { before: '変更前', after: '変更後' }),
    { status: 'conflict', reason: 'ambiguous' },
  )
  assert.deepEqual(
    applyAgentEditHunk('別本文', { before: '変更前', after: '変更後' }),
    { status: 'conflict', reason: 'not-found' },
  )
  assert.deepEqual(
    applyAgentEditHunk('', { before: '', after: '新規本文' }),
    { status: 'conflict', reason: 'invalid-empty-target' },
  )
  assert.deepEqual(
    applyAgentEditHunk('同じ本文', { before: '同じ本文', after: '同じ本文' }),
    { status: 'conflict', reason: 'unchanged' },
  )
  assert.deepEqual(
    applyAgentEditHunk('見出し\n本文\n末尾', { before: '本文\n', after: '<安全な置換>\n' }),
    { status: 'ok', content: '見出し\n<安全な置換>\n末尾', range: { start: 4, end: 12 } },
  )
  assert.deepEqual(
    applyAgentEditHunk('😀\r\n変更前', { before: '変更前', after: '変更後😀' }),
    { status: 'ok', content: '😀\r\n変更後😀', range: { start: 4, end: 9 } },
  )
  assert.deepEqual(
    applyAgentEditHunk('既存の変更後\n変更前', { before: '変更前', after: '変更後' }),
    { status: 'ok', content: '既存の変更後\n変更後', range: { start: 7, end: 10 } },
  )

  testVisualDiff(createAgentEditVisualDiff)
  testEditorHighlightUtilities(createAgentEditorTextHighlight, findChangedTopLevelBlockRange)

  await testAgentSavePublishesHighlight()
  await testAgentSaveRetainsConcurrentDraft()
  await testAgentSaveFailureDoesNotPublishHighlight()

  console.log('Agent proposal tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

function testVisualDiff(createAgentEditVisualDiff) {
  const oneLine = createAgentEditVisualDiff('Alpha beta を維持', 'Alpha gamma を維持')
  assertVisualDiffContent(oneLine, 'Alpha beta を維持', 'Alpha gamma を維持')
  assert.equal(oneLine.beforeLines[0].changed, true)
  assert.equal(oneLine.afterLines[0].changed, true)
  assert.ok(oneLine.beforeLines[0].segments.some((segment) => segment.changed))
  assert.ok(oneLine.beforeLines[0].segments.some((segment) => !segment.changed))
  assert.ok(oneLine.afterLines[0].segments.some((segment) => segment.changed))

  const japanese = createAgentEditVisualDiff(
    'この文章は少し冗長な表現です。',
    'この文章は簡潔な表現です。',
  )
  assertVisualDiffContent(japanese, 'この文章は少し冗長な表現です。', 'この文章は簡潔な表現です。')
  assert.ok(japanese.beforeLines[0].segments.some((segment) => segment.changed))
  assert.ok(japanese.beforeLines[0].segments.some((segment) => !segment.changed))

  const multiLine = createAgentEditVisualDiff(
    '共通の先頭\n削除する行\n共通の末尾',
    '共通の先頭\n追加する行\n共通の末尾',
  )
  assertVisualDiffContent(
    multiLine,
    '共通の先頭\n削除する行\n共通の末尾',
    '共通の先頭\n追加する行\n共通の末尾',
  )
  assert.deepEqual(multiLine.beforeLines.map((line) => line.changed), [false, true, false])
  assert.deepEqual(multiLine.afterLines.map((line) => line.changed), [false, true, false])

  const insertedLine = createAgentEditVisualDiff('先頭\n末尾', '先頭\n追加行\n末尾')
  assertVisualDiffContent(insertedLine, '先頭\n末尾', '先頭\n追加行\n末尾')
  assert.deepEqual(insertedLine.beforeLines.map((line) => line.placeholder), [false, true, false])
  assert.deepEqual(insertedLine.afterLines.map((line) => line.placeholder), [false, false, false])
  assert.deepEqual(
    insertedLine.beforeLines.filter((line) => !line.placeholder).map((line) => line.changed),
    [false, false],
  )
  assert.deepEqual(insertedLine.afterLines.map((line) => line.changed), [false, true, false])
  assert.equal(insertedLine.beforeLines[2].text, '末尾')
  assert.equal(insertedLine.afterLines[2].text, '末尾')
  assert.deepEqual(
    insertedLine.beforeLines.map((line) => line.rowNumber),
    insertedLine.afterLines.map((line) => line.rowNumber),
  )

  const removedLine = createAgentEditVisualDiff('先頭\n削除行\n末尾', '先頭\n末尾')
  assertVisualDiffContent(removedLine, '先頭\n削除行\n末尾', '先頭\n末尾')
  assert.deepEqual(removedLine.beforeLines.map((line) => line.placeholder), [false, false, false])
  assert.deepEqual(removedLine.afterLines.map((line) => line.placeholder), [false, true, false])
  assert.equal(removedLine.beforeLines[2].text, '末尾')
  assert.equal(removedLine.afterLines[2].text, '末尾')

  const deletion = createAgentEditVisualDiff('削除対象', '')
  assertVisualDiffContent(deletion, '削除対象', '')
  assert.equal(deletion.beforeLines[0].changed, true)
  assert.equal(deletion.afterLines[0].changed, true)
  assert.deepEqual(deletion.afterLines[0].segments, [])

  const trailingNewline = createAgentEditVisualDiff('変更前\n', '変更後\n')
  assertVisualDiffContent(trailingNewline, '変更前\n', '変更後\n')
  assert.deepEqual(trailingNewline.beforeLines.map((line) => line.changed), [true, false])
  assert.deepEqual(trailingNewline.afterLines.map((line) => line.changed), [true, false])

  const markup = createAgentEditVisualDiff(
    '<script>alert("before")</script>😀',
    '<b onclick="alert(1)">after</b>🧑‍💻',
  )
  assertVisualDiffContent(
    markup,
    '<script>alert("before")</script>😀',
    '<b onclick="alert(1)">after</b>🧑‍💻',
  )

  const longBefore = Array.from({ length: 140 }, (_, index) => `変更前-${index}`).join('\n')
  const longAfter = Array.from({ length: 140 }, (_, index) => `変更後-${index}`).join('\n')
  assertVisualDiffContent(
    createAgentEditVisualDiff(longBefore, longAfter),
    longBefore,
    longAfter,
  )

  const sharedSuffix = Array.from({ length: 140 }, (_, index) => `共通-${index}`).join('\n')
  const longAffixDiff = createAgentEditVisualDiff(
    `変更前だけの先頭\n${sharedSuffix}`,
    `変更後だけの先頭\n${sharedSuffix}`,
  )
  assertVisualDiffContent(
    longAffixDiff,
    `変更前だけの先頭\n${sharedSuffix}`,
    `変更後だけの先頭\n${sharedSuffix}`,
  )
  assert.equal(longAffixDiff.beforeLines[0].changed, true)
  assert.ok(longAffixDiff.beforeLines.slice(1).every((line) => !line.changed))

  const longTokenBefore = Array.from({ length: 140 }, (_, index) => `before${index}`).join(' ')
  const longTokenAfter = Array.from({ length: 140 }, (_, index) => `after${index}`).join(' ')
  assertVisualDiffContent(
    createAgentEditVisualDiff(longTokenBefore, longTokenAfter),
    longTokenBefore,
    longTokenAfter,
  )
}

function assertVisualDiffContent(diff, before, after) {
  assert.equal(diff.beforeLines.length, diff.afterLines.length)
  assert.deepEqual(
    diff.beforeLines.map((line) => line.rowNumber),
    diff.afterLines.map((line) => line.rowNumber),
  )
  assert.equal(renderDiffLines(diff.beforeLines), before)
  assert.equal(renderDiffLines(diff.afterLines), after)
}

function renderDiffLines(lines) {
  return lines
    .filter((line) => !line.placeholder)
    .map((line) => line.segments.map((segment) => segment.text).join(''))
    .join('\n')
}

function testEditorHighlightUtilities(
  createAgentEditorTextHighlight,
  findChangedTopLevelBlockRange,
) {
  const content = '前文\n<script>alert("x")</script>😀\n後文'
  const start = content.indexOf('<script>')
  const end = content.indexOf('\n後文')
  const textHighlight = createAgentEditorTextHighlight(content, { start, end })
  assert.equal(
    `${textHighlight.prefix}${textHighlight.highlighted}${textHighlight.suffix}`,
    content,
  )
  assert.equal(textHighlight.highlighted, '<script>alert("x")</script>😀')
  assert.equal(textHighlight.isDeletion, false)

  const deletion = createAgentEditorTextHighlight('前後', { start: 1, end: 1 })
  assert.deepEqual(deletion, {
    prefix: '前',
    highlighted: '',
    suffix: '後',
    isDeletion: true,
  })

  const paragraph = (text) => ({ type: 'paragraph', content: [{ type: 'text', text }] })
  assert.deepEqual(
    findChangedTopLevelBlockRange(
      { content: [paragraph('先頭'), paragraph('変更前'), paragraph('末尾')] },
      { content: [paragraph('先頭'), paragraph('変更後'), paragraph('末尾')] },
    ),
    { startIndex: 1, endIndex: 2, usesDeletionAnchor: false },
  )
  assert.deepEqual(
    findChangedTopLevelBlockRange(
      { content: [paragraph('先頭'), paragraph('削除'), paragraph('末尾')] },
      { content: [paragraph('先頭'), paragraph('末尾')] },
    ),
    { startIndex: 1, endIndex: 2, usesDeletionAnchor: true },
  )
  assert.equal(
    findChangedTopLevelBlockRange(
      { content: [paragraph('同じ')] },
      { content: [paragraph('同じ')] },
    ),
    null,
  )
}

async function testAgentSavePublishesHighlight() {
  setActivePinia(createPinia())
  const mockNotes = await import(pathToFileURL(mockNotesFile).href)
  const { useNoteStore } = await import(pathToFileURL(noteStoreOutFile).href)
  mockNotes.calls.updateNote.length = 0
  const noteStore = useNoteStore()
  const originalNote = {
    id: 'note-1',
    notebookId: null,
    title: '対象ノート',
    content: '前文\n変更前\n後文',
    isFavorite: false,
    isPinned: false,
    isTrashed: false,
    revision: 3,
    createdAt: '2026-08-23T00:00:00Z',
    updatedAt: '2026-08-23T00:00:00Z',
  }
  noteStore.activeNote = originalNote
  noteStore.summaries = [originalNote]

  const deferredUpdate = mockNotes.deferUpdate()
  const proposal = {
    targetNoteID: originalNote.id,
    targetTitle: originalNote.title,
    baseRevision: originalNote.revision,
    reason: '簡潔化するため',
    before: '変更前',
    after: 'Agent変更後',
    changedFields: ['content'],
  }
  const applyPromise = noteStore.applyAgentEditProposal(proposal)
  await waitFor(() => mockNotes.calls.updateNote.length === 1)
  deferredUpdate.resolve({
    ...originalNote,
    content: '前文\nAgent変更後\n後文',
    revision: 4,
    updatedAt: '2026-08-23T00:01:00Z',
  })

  assert.equal(await applyPromise, 'applied')
  assert.deepEqual(noteStore.agentEditorHighlight, {
    id: 1,
    noteId: originalNote.id,
    revision: 4,
    start: 3,
    end: 11,
    beforeMarkdown: originalNote.content,
    changeKind: 'replace',
  })

  assert.equal(await noteStore.applyAgentEditProposal(proposal), 'conflict')
  assert.equal(noteStore.agentEditorHighlight, null, 'a new failed apply must clear the old marker')
  assert.equal(mockNotes.calls.updateNote.length, 1)
}

async function testAgentSaveRetainsConcurrentDraft() {
  setActivePinia(createPinia())
  const mockNotes = await import(pathToFileURL(mockNotesFile).href)
  const { useNoteStore } = await import(pathToFileURL(noteStoreOutFile).href)
  mockNotes.calls.updateNote.length = 0
  const noteStore = useNoteStore()
  const originalNote = {
    id: 'note-1',
    notebookId: null,
    title: '対象ノート',
    content: '前文\n変更前\n後文',
    isFavorite: false,
    isPinned: false,
    isTrashed: false,
    revision: 3,
    createdAt: '2026-08-23T00:00:00Z',
    updatedAt: '2026-08-23T00:00:00Z',
  }
  noteStore.activeNote = originalNote
  noteStore.summaries = [originalNote]

  const deferredUpdate = mockNotes.deferUpdate()
  const applyPromise = noteStore.applyAgentEditProposal({
    targetNoteID: originalNote.id,
    targetTitle: originalNote.title,
    baseRevision: originalNote.revision,
    reason: '簡潔化するため',
    before: '変更前',
    after: 'Agent変更後',
    changedFields: ['content'],
  })
  await waitFor(() => mockNotes.calls.updateNote.length === 1)

  noteStore.scheduleDraft(originalNote.id, originalNote.title, '保存中に入力したローカル下書き')
  const draftFlush = noteStore.flushPendingDraft()
  await Promise.resolve()

  deferredUpdate.resolve({
    ...originalNote,
    content: '前文\nAgent変更後\n後文',
    revision: 4,
    updatedAt: '2026-08-23T00:01:00Z',
  })

  assert.equal(await applyPromise, 'applied-with-draft-conflict')
  assert.equal(await draftFlush, true)
  assert.equal(mockNotes.calls.updateNote.length, 1, 'the canceled draft must not reach the note API')
  assert.equal(noteStore.activeNote.content, '前文\nAgent変更後\n後文')
  assert.equal(noteStore.activeNote.revision, 4)
  assert.equal(noteStore.getDraft(originalNote.id)?.status, 'conflicted')
  assert.equal(
    noteStore.getDraft(originalNote.id)?.content,
    '保存中に入力したローカル下書き',
  )
  assert.deepEqual(noteStore.getDraft(originalNote.id)?.conflict, {
    code: 'NOTE_REVISION_CONFLICT',
    noteId: originalNote.id,
    expectedRevision: 3,
    actualRevision: 4,
  })
  assert.equal(noteStore.agentEditorHighlight, null)
}

async function testAgentSaveFailureDoesNotPublishHighlight() {
  setActivePinia(createPinia())
  const mockNotes = await import(pathToFileURL(mockNotesFile).href)
  const { useNoteStore } = await import(pathToFileURL(noteStoreOutFile).href)
  mockNotes.calls.updateNote.length = 0
  const noteStore = useNoteStore()
  const originalNote = {
    id: 'note-failure',
    notebookId: null,
    title: '失敗対象',
    content: '変更前',
    isFavorite: false,
    isPinned: false,
    isTrashed: false,
    revision: 2,
    createdAt: '2026-08-23T00:00:00Z',
    updatedAt: '2026-08-23T00:00:00Z',
  }
  noteStore.activeNote = originalNote
  noteStore.summaries = [originalNote]

  const deferredUpdate = mockNotes.deferUpdate()
  const applyPromise = noteStore.applyAgentEditProposal({
    targetNoteID: originalNote.id,
    targetTitle: originalNote.title,
    baseRevision: originalNote.revision,
    reason: '失敗確認',
    before: '変更前',
    after: '変更後',
    changedFields: ['content'],
  })
  await waitFor(() => mockNotes.calls.updateNote.length === 1)
  deferredUpdate.reject(new Error('save failed'))

  assert.equal(await applyPromise, 'save-failure')
  assert.equal(noteStore.agentEditorHighlight, null)
  assert.equal(noteStore.activeNote.content, originalNote.content)
}

async function waitFor(predicate) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (predicate()) return
    await Promise.resolve()
  }
  assert.fail('timed out waiting for the deferred note update')
}
