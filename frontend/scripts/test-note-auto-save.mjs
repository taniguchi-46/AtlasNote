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
  await testStaleRevisionIsNotApplied()
  await testFailedDraftSurvivesSwitchAndRetry()
  await testFlushWaitsForInFlightSave()
  await testCancelDropsPendingSave()
  console.log('note auto-save tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function testSwitchKeepsSaveTargetFixed() {
  const saves = []
  const appliedSummaries = []
  const feedback = []
  let activeNote = { id: 'note-a', content: 'A before edit' }
  let activeRevision = 1
  const latestRevision = new Map([['note-a', 2]])
  const saveDeferred = deferred()
  const timers = fakeTimers()

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => {
      saves.push(snapshot)
      return saveDeferred.promise
    },
    shouldApply: (snapshot) => latestRevision.get(snapshot.noteId) === snapshot.revision,
    isCurrent: (snapshot) =>
      activeNote.id === snapshot.noteId && activeRevision === snapshot.revision,
    applyResult: (snapshot, result) => {
      appliedSummaries.push(result)
      if (activeNote.id === snapshot.noteId && activeRevision === snapshot.revision) {
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
    revision: 2,
  })

  const flushPromise = autoSave.flush()
  activeNote = { id: 'note-b', content: 'B content' }
  activeRevision = 3
  saveDeferred.resolve({ id: 'note-a', content: 'A edited content' })
  await flushPromise

  assert.deepEqual(saves, [{
    noteId: 'note-a',
    title: 'A title',
    content: 'A edited content',
    revision: 2,
  }])
  assert.equal(activeNote.id, 'note-b')
  assert.equal(activeNote.content, 'B content')
  assert.deepEqual(appliedSummaries, [{ id: 'note-a', content: 'A edited content' }])
  assert.deepEqual(feedback, [])
}

async function testStaleRevisionIsNotApplied() {
  const deferredByRevision = new Map([[1, deferred()], [2, deferred()]])
  const applied = []
  const latestRevision = new Map([['note-a', 1]])
  const timers = fakeTimers()

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => deferredByRevision.get(snapshot.revision).promise,
    shouldApply: (snapshot) => latestRevision.get(snapshot.noteId) === snapshot.revision,
    isCurrent: (snapshot) => latestRevision.get(snapshot.noteId) === snapshot.revision,
    applyResult: (_snapshot, result) => applied.push(result),
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({ noteId: 'note-a', title: 'old', content: 'old', revision: 1 })
  const oldSave = autoSave.flush()
  latestRevision.set('note-a', 2)
  autoSave.schedule({ noteId: 'note-a', title: 'new', content: 'new', revision: 2 })
  const newSave = autoSave.flush()

  deferredByRevision.get(2).resolve({ id: 'note-a', content: 'new' })
  deferredByRevision.get(1).resolve({ id: 'note-a', content: 'old' })
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

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', revision: 1 })
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
    revision: 2,
  }
  drafts.set(snapshot.noteId, snapshot)

  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: async (pendingSnapshot) => {
      attempts.push(pendingSnapshot)
      return results.shift()
    },
    shouldApply: (pendingSnapshot) =>
      drafts.get(pendingSnapshot.noteId)?.revision === pendingSnapshot.revision,
    isCurrent: (pendingSnapshot) =>
      drafts.get(pendingSnapshot.noteId)?.revision === pendingSnapshot.revision,
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
  autoSave.schedule(drafts.get(activeNoteId))
  assert.equal(await autoSave.flush(), true)
  assert.equal(drafts.has('note-a'), false)
  assert.equal(attempts.length, 2)
}

async function testFlushWaitsForInFlightSave() {
  const deferredByRevision = new Map([[1, deferred()], [2, deferred()]])
  const timers = fakeTimers()
  const autoSave = createNoteAutoSave({
    delayMs: 1000,
    save: (snapshot) => deferredByRevision.get(snapshot.revision).promise,
    shouldApply: () => true,
    isCurrent: () => true,
    applyResult: () => {},
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  })

  autoSave.schedule({ noteId: 'note-a', title: 'A', content: 'A', revision: 1 })
  timers.run()
  autoSave.schedule({ noteId: 'note-a', title: 'B', content: 'B', revision: 2 })
  const flushPromise = autoSave.flush()
  let flushFinished = false
  void flushPromise.then(() => {
    flushFinished = true
  })
  await Promise.resolve()
  assert.equal(flushFinished, false)

  deferredByRevision.get(2).resolve({ id: 'note-a' })
  await Promise.resolve()
  assert.equal(flushFinished, false)

  deferredByRevision.get(1).resolve({ id: 'note-a' })
  assert.equal(await flushPromise, true)
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
