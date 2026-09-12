import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'deleteNotesSequentially.ts')
const dependencyPath = path.join(rootDir, 'src', 'utils', 'noteBatch.ts')
const noteStorePath = path.join(rootDir, 'src', 'stores', 'useNoteStore.ts')
const noteStoreUtilityNames = [
  'agentEditProposal',
  'deleteNotesSequentially',
  'latestRequestGuard',
  'noteAutoSave',
  'noteBatch',
  'noteOperationQueue',
  'requestCounter',
  'updateNotesSequentially',
]
const notificationPath = path.join(rootDir, 'src', 'components', 'NotificationCenter.vue')
const noteListPath = path.join(rootDir, 'src', 'components', 'NoteList.vue')
const outDir = path.join(rootDir, '.tmp', 'note-delete-test')
const outFile = path.join(outDir, 'deleteNotesSequentially.mjs')
const noteStoreOutFile = path.join(outDir, 'useNoteStore.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const dependencySource = await readFile(dependencyPath, 'utf8')
const dependencyCompiled = ts.transpileModule(dependencySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})
await writeFile(path.join(outDir, 'noteBatch.mjs'), dependencyCompiled.outputText, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText.replace("from './noteBatch'", "from './noteBatch.mjs'"), 'utf8')

const [noteStoreSource, ...noteStoreUtilitySources] = await Promise.all([
  readFile(noteStorePath, 'utf8'),
  ...noteStoreUtilityNames.map((name) => readFile(path.join(rootDir, 'src', 'utils', `${name}.ts`), 'utf8')),
])

for (let index = 0; index < noteStoreUtilityNames.length; index += 1) {
  const name = noteStoreUtilityNames[index]
  const utilitySource = noteStoreUtilitySources[index]
    .replaceAll("from './noteBatch'", "from './noteBatch.mjs'")
  const compiledUtility = ts.transpileModule(utilitySource, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: path.join(rootDir, 'src', 'utils', `${name}.ts`),
  })
  await writeFile(path.join(outDir, `${name}.mjs`), compiledUtility.outputText, 'utf8')
}

const compiledNoteStore = ts.transpileModule(
  noteStoreSource
    .replace("from '../api/notes'", "from './mock-notes.mjs'")
    .replace("from '../utils/latestRequestGuard'", "from './latestRequestGuard.mjs'")
    .replace("from '../utils/noteAutoSave'", "from './noteAutoSave.mjs'")
    .replace("from '../utils/noteOperationQueue'", "from './noteOperationQueue.mjs'")
    .replace("from '../utils/requestCounter'", "from './requestCounter.mjs'")
    .replace("from '../utils/deleteNotesSequentially'", "from './deleteNotesSequentially.mjs'")
    .replace("from '../utils/updateNotesSequentially'", "from './updateNotesSequentially.mjs'")
    .replace("from '../utils/agentEditProposal'", "from './agentEditProposal.mjs'")
    .replace("from './useSettingsStore'", "from './mock-note-stores.mjs'")
    .replace("from './useNotificationStore'", "from './mock-note-stores.mjs'")
    .replace("from './useAppStore'", "from './mock-note-stores.mjs'")
    .replace("from './useContentLockStore'", "from './mock-note-stores.mjs'"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: noteStorePath,
  },
)
await writeFile(noteStoreOutFile, compiledNoteStore.outputText, 'utf8')

await writeFile(path.join(outDir, 'mock-notes.mjs'), `
export const calls = { listNotesPage: [], getNote: [], updateNote: [], deleteNote: [] }
const notes = new Map()
const pendingListResponses = []
const pendingGetResponses = []
let deferListResponses = false
let deferGetResponses = false
let deferUpdateResponses = false
const pendingUpdateResponses = []
let nextUpdateFailure = null
const updateFailures = new Map()

export class NoteRevisionConflictError extends Error {
  constructor(conflict) {
    super('ノートが別の更新によって変更されています')
    this.name = 'NoteRevisionConflictError'
    Object.assign(this, conflict)
  }
}

export function resetBackend(seed = []) {
  notes.clear()
  for (const note of seed) notes.set(note.id, structuredClone(note))
  for (const key of Object.keys(calls)) calls[key].length = 0
  pendingListResponses.length = 0
  pendingGetResponses.length = 0
  pendingUpdateResponses.length = 0
  deferListResponses = false
  deferGetResponses = false
  deferUpdateResponses = false
  nextUpdateFailure = null
  updateFailures.clear()
}

export function snapshotNote(id) {
  const note = notes.get(id)
  return note ? structuredClone(note) : null
}

export function setDeferredListResponses(enabled) { deferListResponses = enabled }
export function setDeferredGetResponses(enabled) { deferGetResponses = enabled }
export function setDeferredUpdateResponses(enabled) { deferUpdateResponses = enabled }
export function setNextUpdateFailure(failure) { nextUpdateFailure = failure }
export function setUpdateFailure(id, failure) { updateFailures.set(id, failure) }

export function resolveListResponse(index, items) {
  const pending = pendingListResponses[index]
  if (!pending) throw new Error('pending list response was not found')
  pending.resolve({
    items: structuredClone(items),
    page: 1,
    pageSize: 100,
    total: items.length,
    hasNext: false,
  })
}

export function resolveGetResponse(index, item) {
  const pending = pendingGetResponses[index]
  if (!pending) throw new Error('pending note response was not found')
  pending.resolve(structuredClone(item))
}

export function resolveUpdateResponse(index) {
  const pending = pendingUpdateResponses[index]
  if (!pending) throw new Error('pending update response was not found')
  const current = notes.get(pending.id)
  if (!current) throw new Error('note not found')
  const updated = {
    ...current,
    ...(pending.input.title !== undefined ? { title: pending.input.title } : {}),
    ...(pending.input.content !== undefined ? { content: pending.input.content } : {}),
    ...(pending.input.isFavorite !== undefined ? { isFavorite: pending.input.isFavorite } : {}),
    ...(pending.input.isPinned !== undefined ? { isPinned: pending.input.isPinned } : {}),
    ...(pending.input.isTrashed !== undefined ? { isTrashed: pending.input.isTrashed } : {}),
    revision: current.revision + 1,
    updatedAt: new Date(Date.parse(current.updatedAt) + 1000).toISOString(),
  }
  notes.set(pending.id, updated)
  pendingUpdateResponses.splice(index, 1)
  pending.resolve(structuredClone(updated))
}

export async function listNotesPage(input) {
  calls.listNotesPage.push(structuredClone(input))
  if (deferListResponses) {
    return new Promise((resolve) => pendingListResponses.push({ resolve }))
  }
  const items = [...notes.values()].map(toSummary)
  return { items, page: input.page ?? 1, pageSize: input.pageSize ?? 100, total: items.length, hasNext: false }
}

export async function getNote(id) {
  calls.getNote.push(id)
  if (deferGetResponses) {
    return new Promise((resolve) => pendingGetResponses.push({ resolve }))
  }
  const item = notes.get(id)
  if (!item) throw new Error('note not found')
  return structuredClone(item)
}

export async function updateNote(id, input) {
  calls.updateNote.push({ id, input: structuredClone(input) })
  const current = notes.get(id)
  if (!current) throw new Error('note not found')
  const failure = updateFailures.has(id) ? updateFailures.get(id) : nextUpdateFailure
  if (failure) {
    if (updateFailures.has(id)) updateFailures.delete(id)
    else nextUpdateFailure = null
    if (failure === 'conflict') {
      throw new NoteRevisionConflictError({
        code: 'NOTE_REVISION_CONFLICT',
        noteId: id,
        expectedRevision: input.expectedRevision,
        actualRevision: current.revision,
      })
    }
    throw new Error('update failed')
  }
  if (input.expectedRevision !== current.revision) {
    throw new NoteRevisionConflictError({
      code: 'NOTE_REVISION_CONFLICT',
      noteId: id,
      expectedRevision: input.expectedRevision,
      actualRevision: current.revision,
    })
  }
  if (deferUpdateResponses) {
    return new Promise((resolve) => pendingUpdateResponses.push({ id, input, resolve }))
  }
  const updated = {
    ...current,
    ...(input.title !== undefined ? { title: input.title } : {}),
    ...(input.content !== undefined ? { content: input.content } : {}),
    ...(input.isFavorite !== undefined ? { isFavorite: input.isFavorite } : {}),
    ...(input.isPinned !== undefined ? { isPinned: input.isPinned } : {}),
    ...(input.isTrashed !== undefined ? { isTrashed: input.isTrashed } : {}),
    revision: current.revision + 1,
    updatedAt: new Date(Date.parse(current.updatedAt) + 1000).toISOString(),
  }
  notes.set(id, updated)
  return structuredClone(updated)
}

export async function deleteNote(id, expectedRevision) {
  calls.deleteNote.push({ id, expectedRevision })
  const current = notes.get(id)
  if (!current) throw new Error('note not found')
  if (expectedRevision !== current.revision) {
    throw new NoteRevisionConflictError({
      code: 'NOTE_REVISION_CONFLICT',
      noteId: id,
      expectedRevision,
      actualRevision: current.revision,
    })
  }
  notes.delete(id)
}

export async function listNotes() { return [...notes.values()].map(toSummary) }
export async function createNote() { throw new Error('unexpected note create') }

function toSummary(item) {
  const { content: _content, ...summary } = item
  return structuredClone(summary)
}
`, 'utf8')

await writeFile(path.join(outDir, 'mock-note-stores.mjs'), `
const settings = { editorFirstLineStyle: 'paragraph' }
const app = { sortOption: '', sidebarSection: 'notes' }
const notifications = { dismissBySource() {}, notify() {} }
let deferLockResponses = false
const pendingLockResponses = []
const lockCalls = []

export function resetStores() {
  deferLockResponses = false
  pendingLockResponses.length = 0
  lockCalls.length = 0
}

export const calls = { get refreshTarget() { return lockCalls } }

export function setDeferredLockResponses(enabled) { deferLockResponses = enabled }

export function resolveLockResponse(index, status) {
  const pending = pendingLockResponses[index]
  if (!pending) throw new Error('pending lock response was not found')
  pending.resolve(structuredClone(status))
}

export function useSettingsStore() { return settings }
export function useNotificationStore() { return notifications }
export function useAppStore() { return app }
export function parseNoteSortOption() { return null }

export function useContentLockStore() {
  return {
    async requestAccess() { return true },
    async refreshTarget(target) {
      lockCalls.push(structuredClone(target))
      if (deferLockResponses) {
        return new Promise((resolve) => pendingLockResponses.push({ resolve }))
      }
      return { protected: false, locked: false, explicitLock: false }
    },
  }
}
`, 'utf8')

const { deleteNotesSequentially, NoteDeleteError } = await import(pathToFileURL(outFile).href)

try {
  await testAllDeletesSucceed()
  await testFailureReportsOnlyCompletedDeletes()
  await testDeleteErrorIsRenderedAsAnAlert()
  await testDirectDeleteUsesExistingDeletionFlows()
  const noteMock = await import(pathToFileURL(path.join(outDir, 'mock-notes.mjs')).href)
  const storeMock = await import(pathToFileURL(path.join(outDir, 'mock-note-stores.mjs')).href)
  const { useNoteStore } = await import(pathToFileURL(noteStoreOutFile).href)
  await testActualStoreTrashThenEmptyTrash(noteMock, storeMock, useNoteStore)
  await testActualStoreDraftFlushBeforeDeletion(noteMock, storeMock, useNoteStore)
  await testActualStoreFailedDraftPreventsDeletion(noteMock, storeMock, useNoteStore)
  await testActualStoreKeepsDraftAddedAfterFlush(noteMock, storeMock, useNoteStore)
  await testActualStoreBatchDeleteKeepsPartialSuccess(noteMock, storeMock, useNoteStore)
  await testActualStoreLockResponseOrdering(noteMock, storeMock, useNoteStore)
  await testActualStoreSelectionAndListOrdering(noteMock, storeMock, useNoteStore)
  await testActualStoreDeletionDoesNotCancelOtherSelection(noteMock, storeMock, useNoteStore)
  console.log('note delete tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function testDeleteErrorIsRenderedAsAnAlert() {
  const notificationSource = await readFile(notificationPath, 'utf8')

  assert.match(notificationSource, /notificationStore\.notifications/)
  assert.match(notificationSource, /:role="notification\.kind === 'error' \? 'alert' : 'status'"/)
  assert.match(notificationSource, /\{\{ notification\.message \}\}/)
}

async function testDirectDeleteUsesExistingDeletionFlows() {
  const noteListSource = await readFile(noteListPath, 'utf8')

  assert.match(noteListSource, /if \(item\.isTrashed\)[\s\S]*?window\.confirm/, 'permanent deletion from the list must require confirmation')
  assert.match(noteListSource, /noteStore\.trashNotes\(\[item\.id\]\)/, 'normal notes must be moved to trash first')
  assert.match(noteListSource, /noteStore\.permanentlyDeleteNotes\(\[item\.id\]\)/, 'trash deletion must reuse the revision-safe batch path')
}

async function testAllDeletesSucceed() {
  const calls = []
  const ids = Array.from({ length: 68 }, (_, index) => `note-${index}`)
  const deletedIds = await deleteNotesSequentially(ids, async (id) => {
    calls.push(id)
  })

  assert.deepEqual(calls, ids)
  assert.deepEqual(deletedIds, ids)
}

async function testFailureReportsOnlyCompletedDeletes() {
  const calls = []

  await assert.rejects(
    deleteNotesSequentially(['a', 'b', 'c'], async (id) => {
      calls.push(id)
      if (id === 'b') throw new Error('commit markdown delete failed')
    }),
    (error) => {
      assert.ok(error instanceof NoteDeleteError)
      assert.equal(error.message, 'commit markdown delete failed')
      assert.equal(error.failedId, 'b')
      assert.deepEqual(error.deletedIds, ['a'])
      return true
    },
  )
  assert.deepEqual(calls, ['a', 'b'])
}

async function testActualStoreTrashThenEmptyTrash(noteMock, storeMock, useNoteStore) {
  const original = createTestNote('restored-note', 7, false)
  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  assert.equal(await store.selectNote(original.id), true)

  storeMock.setDeferredLockResponses(true)
  const pendingLockRefresh = store.refreshActiveNoteLockStatus()
  await waitFor(() => storeMock.calls.refreshTarget.length === 1)

  await store.trashNotes([original.id])
  assert.equal(store.activeNote, null)
  assert.equal(store.summaries[0].revision, original.revision + 1)
  assert.equal(store.summaries[0].isTrashed, true)

  storeMock.resolveLockResponse(0, { protected: false, locked: false, explicitLock: false })
  await pendingLockRefresh
  await store.emptyTrash()

  assert.deepEqual(noteMock.calls.deleteNote, [{ id: original.id, expectedRevision: original.revision + 1 }])
  assert.equal(noteMock.snapshotNote(original.id), null)
  assert.equal(store.summaries.some((summary) => summary.id === original.id), false)
  assert.equal(store.activeNote, null)
  storeMock.setDeferredLockResponses(false)
}

async function testActualStoreDraftFlushBeforeDeletion(noteMock, storeMock, useNoteStore) {
  const original = createTestNote('draft-delete-note', 4, false)
  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  assert.equal(await store.selectNote(original.id), true)
  store.scheduleDraft(original.id, 'draft title', 'draft content')

  assert.equal(await store.trashNote(original.id), true)
  assert.equal(store.activeNote, null)
  assert.equal(store.getDraft(original.id), null)
  assert.deepEqual(noteMock.calls.updateNote, [
    {
      id: original.id,
      input: {
        title: 'draft title',
        content: 'draft content',
        expectedRevision: original.revision,
      },
    },
    {
      id: original.id,
      input: {
        isTrashed: true,
        expectedRevision: original.revision + 1,
      },
    },
  ])
  assert.equal(noteMock.snapshotNote(original.id).isTrashed, true)

  const trashed = noteMock.snapshotNote(original.id)
  noteMock.resetBackend([{ ...trashed, revision: trashed.revision, isTrashed: true }])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const permanentStore = useNoteStore()
  await permanentStore.fetchNotes()
  assert.equal(await permanentStore.selectNote(original.id), true)
  permanentStore.scheduleDraft(original.id, 'permanent draft title', 'permanent draft content')

  await permanentStore.permanentlyDeleteNote(original.id)
  assert.equal(permanentStore.activeNote, null)
  assert.equal(permanentStore.getDraft(original.id), null)
  assert.deepEqual(noteMock.calls.updateNote, [
    {
      id: original.id,
      input: {
        title: 'permanent draft title',
        content: 'permanent draft content',
        expectedRevision: trashed.revision,
      },
    },
  ])
  assert.deepEqual(noteMock.calls.deleteNote, [
    { id: original.id, expectedRevision: trashed.revision + 1 },
  ])
  assert.equal(noteMock.snapshotNote(original.id), null)
}

async function testActualStoreFailedDraftPreventsDeletion(noteMock, storeMock, useNoteStore) {
  const original = createTestNote('failed-draft-delete-note', 2, false)
  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  assert.equal(await store.selectNote(original.id), true)
  store.scheduleDraft(original.id, 'unsaved title', 'unsaved content')
  noteMock.setNextUpdateFailure('error')

  assert.equal(await store.trashNote(original.id), false)
  assert.equal(noteMock.snapshotNote(original.id).isTrashed, false)
  assert.equal(store.activeNote.id, original.id)
  assert.equal(store.getDraft(original.id).status, 'failed')
  assert.equal(noteMock.calls.updateNote.length, 1)
  assert.equal(noteMock.calls.updateNote[0].input.isTrashed, undefined)
  assert.equal(noteMock.calls.deleteNote.length, 0)

  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const conflictStore = useNoteStore()
  await conflictStore.fetchNotes()
  assert.equal(await conflictStore.selectNote(original.id), true)
  conflictStore.scheduleDraft(original.id, 'conflicted title', 'conflicted content')
  noteMock.setNextUpdateFailure('conflict')

  assert.equal(await conflictStore.trashNote(original.id), false)
  assert.equal(noteMock.snapshotNote(original.id).isTrashed, false)
  assert.equal(conflictStore.activeNote.id, original.id)
  assert.equal(conflictStore.getDraft(original.id).status, 'conflicted')
  assert.equal(noteMock.calls.deleteNote.length, 0)
}

async function testActualStoreKeepsDraftAddedAfterFlush(noteMock, storeMock, useNoteStore) {
  const original = createTestNote('late-draft-delete-note', 3, false)
  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  assert.equal(await store.selectNote(original.id), true)

  noteMock.setDeferredUpdateResponses(true)
  const queuedUpdate = store.persistNote(original.id, { isPinned: true })
  await waitFor(() => noteMock.calls.updateNote.length === 1)

  const deletion = store.trashNote(original.id)
  await waitFor(() => store.isNoteDeletionPreparing(original.id))
  store.scheduleDraft(original.id, 'late title', 'late content')
  noteMock.resolveUpdateResponse(0)

  await queuedUpdate
  assert.equal(await deletion, false)
  assert.equal(noteMock.snapshotNote(original.id).isTrashed, false)
  assert.equal(noteMock.calls.updateNote.length, 1)
  assert.deepEqual(store.getDraft(original.id).content, 'late content')
  assert.equal(store.getDraft(original.id).title, 'late title')

  store.discardDraft(original.id)
  noteMock.setDeferredUpdateResponses(false)
}

async function testActualStoreBatchDeleteKeepsPartialSuccess(noteMock, storeMock, useNoteStore) {
  const firstNote = createTestNote('partial-delete-first', 1, false)
  const secondNote = createTestNote('partial-delete-second', 1, false)
  noteMock.resetBackend([firstNote, secondNote])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  noteMock.setUpdateFailure(secondNote.id, 'error')

  await assert.rejects(
    store.trashNotes([firstNote.id, secondNote.id]),
    /update failed/,
  )
  assert.equal(noteMock.snapshotNote(firstNote.id).isTrashed, true)
  assert.equal(noteMock.snapshotNote(secondNote.id).isTrashed, false)
  assert.equal(store.summaries.find((item) => item.id === firstNote.id).isTrashed, true)
  assert.equal(store.summaries.find((item) => item.id === secondNote.id).isTrashed, false)
}

async function testActualStoreLockResponseOrdering(noteMock, storeMock, useNoteStore) {
  const original = createTestNote('lock-order-note', 3, false)
  noteMock.resetBackend([original])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()
  store.activeNote = structuredClone(original)
  store.summaries = [toSummary(original)]

  storeMock.setDeferredLockResponses(true)
  const older = store.refreshActiveNoteLockStatus()
  const newer = store.refreshActiveNoteLockStatus()
  await waitFor(() => storeMock.calls.refreshTarget.length === 2)

  storeMock.resolveLockResponse(1, { protected: true, locked: false, explicitLock: true, source: 'note' })
  assert.equal(await newer, false)
  assert.equal(store.activeNote.protected, true)
  assert.equal(store.activeNote.locked, false)
  assert.equal(store.activeNote.revision, original.revision)

  storeMock.resolveLockResponse(0, { protected: false, locked: true, explicitLock: true, source: 'note' })
  assert.equal(await older, false)
  assert.equal(store.activeNote.id, original.id)
  assert.equal(store.activeNote.locked, false)
  assert.equal(store.activeNote.protected, true)

  const lockedRequest = store.refreshActiveNoteLockStatus()
  await waitFor(() => storeMock.calls.refreshTarget.length === 3)
  storeMock.resolveLockResponse(2, { protected: true, locked: true, explicitLock: true, source: 'note' })
  assert.equal(await lockedRequest, true)
  assert.equal(store.activeNote, null)
  storeMock.setDeferredLockResponses(false)
}

async function testActualStoreSelectionAndListOrdering(noteMock, storeMock, useNoteStore) {
  const firstNote = createTestNote('first-note', 1, false)
  const secondNote = createTestNote('second-note', 1, false)
  noteMock.resetBackend([firstNote, secondNote])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  noteMock.setDeferredListResponses(true)
  const olderList = store.fetchNotes()
  const newerList = store.fetchNotes()
  await waitFor(() => noteMock.calls.listNotesPage.length === 2)
  noteMock.resolveListResponse(1, [toSummary(secondNote)])
  await newerList
  noteMock.resolveListResponse(0, [toSummary(firstNote)])
  await olderList
  assert.deepEqual(store.summaries.map((summary) => summary.id), [secondNote.id])
  noteMock.setDeferredListResponses(false)

  noteMock.setDeferredGetResponses(true)
  const olderSelection = store.selectNote(firstNote.id)
  await waitFor(() => noteMock.calls.getNote.length === 1)
  const newerSelection = store.selectNote(secondNote.id)
  await waitFor(() => noteMock.calls.getNote.length === 2)
  noteMock.resolveGetResponse(1, secondNote)
  assert.equal(await newerSelection, true)
  noteMock.resolveGetResponse(0, firstNote)
  assert.equal(await olderSelection, false)
  assert.equal(store.activeNote.id, secondNote.id)
  noteMock.setDeferredGetResponses(false)
}

async function testActualStoreDeletionDoesNotCancelOtherSelection(noteMock, storeMock, useNoteStore) {
  const firstNote = createTestNote('delete-selection-note', 1, false)
  const secondNote = createTestNote('surviving-selection-note', 1, false)
  noteMock.resetBackend([firstNote, secondNote])
  storeMock.resetStores()
  setActivePinia(createPinia())
  const store = useNoteStore()

  await store.fetchNotes()
  noteMock.setDeferredGetResponses(true)
  const deletingSelection = store.selectNote(firstNote.id)
  await waitFor(() => noteMock.calls.getNote.length === 1)
  const survivingSelection = store.selectNote(secondNote.id)
  await waitFor(() => noteMock.calls.getNote.length === 2)

  await store.trashNotes([firstNote.id])
  assert.equal(store.isNoteDeletionPreparing(firstNote.id), false)
  assert.equal(store.summaries.find((summary) => summary.id === firstNote.id).isTrashed, true)

  noteMock.resolveGetResponse(1, secondNote)
  assert.equal(await survivingSelection, true)
  noteMock.resolveGetResponse(0, firstNote)
  assert.equal(await deletingSelection, false)
  assert.equal(store.activeNote.id, secondNote.id)
  noteMock.setDeferredGetResponses(false)
}

function createTestNote(id, revision, isTrashed) {
  return {
    id,
    notebookId: null,
    title: id,
    content: `${id} content`,
    isFavorite: false,
    isPinned: false,
    isTrashed,
    revision,
    createdAt: '2026-09-06T00:00:00.000Z',
    updatedAt: '2026-09-06T00:00:00.000Z',
  }
}

function toSummary(item) {
  const { content: _content, ...summary } = item
  return structuredClone(summary)
}

async function waitFor(predicate) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (predicate()) return
    await Promise.resolve()
  }
  throw new Error('condition was not reached')
}
