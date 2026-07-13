import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'noteAutoSave.ts')
const outDir = path.join(rootDir, '.tmp', 'note-auto-save-test')
const outFile = path.join(outDir, 'noteAutoSave.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
    importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

const { createNoteAutoSave } = await import(pathToFileUrl(outFile))

try {
  await testSwitchKeepsSaveTargetFixed()
  await testStaleDraftVersionIsNotApplied()
  await testFailedDraftSurvivesSwitchAndRetry()
  await testFlushWaitsForInFlightSave()
  await testCancelDropsPendingSave()
  await testDifferentNotesSaveConcurrently()
  await testFlushTargetsOneNoteLane()
  await testFailedLaneWaitsForManualRetry()
  await testThrownSaveIsHandled()
  console.log('note auto-save tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function testSwitchKeepsSaveTargetFixed() {
  const saves = []
  const appliedSummaries = []
  const feedback = []
  let activeNote = { id: 'note-a', content: 'A before edit' }
  let activeDraftVersion = 1
  const latestDraftVersion = new Map([['note-a', 2]])
  const saveDeferred = deferred()
  const timers = fakeTimers()

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => {
      saves.push(snapshot)
      return saveDeferred.promise
    },
    shouldApply: (snapshot) => latestDraftVersion.get(snapshot.noteId) === snapshot.draftVersion,
    isCurrent: (snapshot) =>
      activeNote.id === snapshot.noteId && activeDraftVersion === snapshot.draftVersion,
    applyResult: (snapshot, result) => {
      appliedSummaries.push(result)
      if (activeNote.id === snapshot.noteId && activeDraftVersion === snapshot.draftVersion) {
        activeNote = result
      }
    },
    onSaved: () => feedback.push('saved'),
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({
    noteId: 'note-a',
    title: 'A title',
    content: 'A edited content',
    draftVersion: 2,
  })

  const flushPromise = autoSave.flush()
  activeNote = { id: 'note-b', content: 'B content' }
  activeDraftVersion = 3
  saveDeferred.resolve({ id: 'note-a', content: 'A edited content' })
  await flushPromise

  assert.deepEqual(saves, [{
    noteId: 'note-a',
    title: 'A title',
    content: 'A edited content',
    draftVersion: 2,
  }])
  assert.equal(activeNote.id, 'note-b')
  assert.equal(activeNote.content, 'B content')
  assert.deepEqual(appliedSummaries, [{ id: 'note-a', content: 'A edited content' }])
  assert.deepEqual(feedback, [])
}

async function testStaleDraftVersionIsNotApplied() {
  const deferredByDraftVersion = new Map([[1, deferred()], [2, deferred()]])
  const applied = []
  const latestDraftVersion = new Map([['note-a', 1]])
  const timers = fakeTimers()

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => deferredByDraftVersion.get(snapshot.draftVersion).promise,
    shouldApply: (snapshot) => latestDraftVersion.get(snapshot.noteId) === snapshot.draftVersion,
    isCurrent: (snapshot) => latestDraftVersion.get(snapshot.noteId) === snapshot.draftVersion,
    applyResult: (_snapshot, result) => applied.push(result),
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({ noteId: 'note-a', title: 'old', content: 'old', draftVersion: 1 })
  const oldSave = autoSave.flush()
  latestDraftVersion.set('note-a', 2)
  autoSave.schedule({ noteId: 'note-a', title: 'new', content: 'new', draftVersion: 2 })
  const newSave = autoSave.flush()

  deferredByDraftVersion.get(2).resolve({ id: 'note-a', content: 'new' })
  deferredByDraftVersion.get(1).resolve({ id: 'note-a', content: 'old' })
  await Promise.all([newSave, oldSave])

  assert.deepEqual(applied, [{ id: 'note-a', content: 'new' }])
}

async function testCancelDropsPendingSave() {
  const saves = []
  const timers = fakeTimers()
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: async (snapshot) => {
      saves.push(snapshot)
      return snapshot
    },
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', draftVersion: 1 })
  autoSave.cancel()
  timers.run()
  await Promise.resolve()

  assert.deepEqual(saves, [])
}

async function testFailedDraftSurvivesSwitchAndRetry() {
  const drafts = new Map()
  const attempts = []
  const results = [null, { id: 'note-a', content: 'A edited content' }]
  let activeNoteId = 'note-a'
  const timers = fakeTimers()
  const snapshot = {
    noteId: 'note-a',
    title: 'A title',
    content: 'A edited content',
    draftVersion: 2,
  }
  drafts.set(snapshot.noteId, snapshot)

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: async (pendingSnapshot) => {
      attempts.push(pendingSnapshot)
      return results.shift()
    },
    shouldApply: (pendingSnapshot) =>
      drafts.get(pendingSnapshot.noteId)?.draftVersion === pendingSnapshot.draftVersion,
    isCurrent: (pendingSnapshot) =>
      drafts.get(pendingSnapshot.noteId)?.draftVersion === pendingSnapshot.draftVersion,
    applyResult: () => {},
    onSaved: (pendingSnapshot) => drafts.delete(pendingSnapshot.noteId),
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule(snapshot)
  assert.equal(await autoSave.flush(), false)
  activeNoteId = 'note-b'

  assert.equal(activeNoteId, 'note-b')
  assert.deepEqual(drafts.get('note-a'), snapshot)

  activeNoteId = 'note-a'
  autoSave.retry(drafts.get(activeNoteId))
  assert.equal(await autoSave.flush(), true)
  assert.equal(drafts.has('note-a'), false)
  assert.equal(attempts.length, 2)
}

async function testFlushWaitsForInFlightSave() {
  const deferredByDraftVersion = new Map([[1, deferred()], [2, deferred()]])
  const timers = fakeTimers()
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => deferredByDraftVersion.get(snapshot.draftVersion).promise,
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', draftVersion: 1 })
  timers.run()
  autoSave.schedule({ noteId: 'note-a', title: 'B', content: 'B', draftVersion: 2 })
  const flushPromise = autoSave.flush()
  let flushFinished = false
  void flushPromise.then(() => {
    flushFinished = true
  })
  await Promise.resolve()
  assert.equal(flushFinished, false)

  deferredByDraftVersion.get(2).resolve({ id: 'note-a' })
  await Promise.resolve()
  assert.equal(flushFinished, false)

  deferredByDraftVersion.get(1).resolve({ id: 'note-a' })
  assert.equal(await flushPromise, true)
}

async function testDifferentNotesSaveConcurrently() {
  const saves = []
  const deferredByNoteId = new Map([['note-a', deferred()], ['note-b', deferred()]])
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => {
      saves.push(snapshot.noteId)
      return deferredByNoteId.get(snapshot.noteId).promise
    },
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', draftVersion: 1 })
  autoSave.schedule({ noteId: 'note-b', title: 'B', content: 'B', draftVersion: 1 })
  const flushPromise = autoSave.flush()
  await Promise.resolve()

  assert.deepEqual(new Set(saves), new Set(['note-a', 'note-b']))
  deferredByNoteId.get('note-a').resolve({ id: 'note-a' })
  deferredByNoteId.get('note-b').resolve({ id: 'note-b' })
  assert.equal(await flushPromise, true)
}

async function testFlushTargetsOneNoteLane() {
  const saves = []
  const noteA = deferred()
  const noteB = deferred()
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => {
      saves.push(snapshot.noteId)
      return snapshot.noteId === 'note-a' ? noteA.promise : noteB.promise
    },
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', draftVersion: 1 })
  autoSave.schedule({ noteId: 'note-b', title: 'B', content: 'B', draftVersion: 1 })
  const noteAFlush = autoSave.flush('note-a')
  await Promise.resolve()

  assert.deepEqual(saves, ['note-a'])
  noteA.resolve({ id: 'note-a' })
  assert.equal(await noteAFlush, true)
  assert.deepEqual(saves, ['note-a'])

  const noteBFlush = autoSave.flush('note-b')
  noteB.resolve({ id: 'note-b' })
  assert.equal(await noteBFlush, true)
}

async function testFailedLaneWaitsForManualRetry() {
  const attempts = []
  const firstAttempt = deferred()
  const retryAttempt = deferred()
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => {
      attempts.push(snapshot.draftVersion)
      return attempts.length === 1 ? firstAttempt.promise : retryAttempt.promise
    },
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
  })
  const first = { noteId: 'note-a', title: 'A1', content: 'A1', draftVersion: 1 }
  const latest = { noteId: 'note-a', title: 'A2', content: 'A2', draftVersion: 2 }

  autoSave.schedule(first)
  const failedFlush = autoSave.flush('note-a')
  autoSave.schedule(latest)
  firstAttempt.resolve(null)
  assert.equal(await failedFlush, false)
  assert.deepEqual(attempts, [1])
  assert.equal(await autoSave.flush('note-a'), false)

  autoSave.retry(latest)
  const retryFlush = autoSave.flush('note-a')
  retryAttempt.resolve({ id: 'note-a' })
  assert.equal(await retryFlush, true)
  assert.deepEqual(attempts, [1, 2])
}

async function testThrownSaveIsHandled() {
  let failedCount = 0
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: async () => {
      throw new Error('unexpected save failure')
    },
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
    onFailed: () => {
      failedCount += 1
    },
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', draftVersion: 1 })
  assert.equal(await autoSave.flush('note-a'), false)
  assert.equal(failedCount, 1)
  assert.equal(await autoSave.flush('note-a'), false)
}

function fakeTimers() {
  let callback = null
  return {
    setTimer(nextCallback) {
      callback = nextCallback
      return 1
    },
    clearTimer() {
      callback = null
    },
    run() {
      const pendingCallback = callback
      callback = null
      pendingCallback?.()
    },
  }
}

function deferred() {
  let resolve
  const promise = new Promise((nextResolve) => {
    resolve = nextResolve
  })
  return { promise, resolve }
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
