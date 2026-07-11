import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const outDir = path.join(rootDir, '.tmp', 'note-operation-queue-test')
await mkdir(outDir, { recursive: true })

for (const sourceName of ['noteOperationQueue', 'requestCounter']) {
  const source = await readFile(path.join(rootDir, 'src', 'utils', `${sourceName}.ts`), 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  })
  await writeFile(path.join(outDir, `${sourceName}.mjs`), compiled.outputText, 'utf8')
}

const { createNoteOperationQueue } = await import(pathToFileUrl(path.join(outDir, 'noteOperationQueue.mjs')))
const { createRequestCounter } = await import(pathToFileUrl(path.join(outDir, 'requestCounter.mjs')))

try {
  await testSameNoteRunsSequentially()
  await testDifferentNotesRunConcurrently()
  await testRejectedOperationDoesNotBlockLane()
  await testTargetAndGlobalFlush()
  testRequestCounterTracksOverlappingRequests()
  console.log('note operation queue tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function testSameNoteRunsSequentially() {
  const queue = createNoteOperationQueue()
  const first = deferred()
  const events = []

  const firstResult = queue.enqueue('note-a', async () => {
    events.push('first:start')
    await first.promise
    events.push('first:end')
    return 1
  })
  const secondResult = queue.enqueue('note-a', async () => {
    events.push('second:start')
    return 2
  })
  await Promise.resolve()
  await Promise.resolve()
  assert.deepEqual(events, ['first:start'])

  first.resolve()
  assert.deepEqual(await Promise.all([firstResult, secondResult]), [1, 2])
  assert.deepEqual(events, ['first:start', 'first:end', 'second:start'])
}

async function testDifferentNotesRunConcurrently() {
  const queue = createNoteOperationQueue()
  const noteA = deferred()
  const noteB = deferred()
  const started = []

  const operations = [
    queue.enqueue('note-a', async () => { started.push('note-a'); await noteA.promise }),
    queue.enqueue('note-b', async () => { started.push('note-b'); await noteB.promise }),
  ]
  await Promise.resolve()
  await Promise.resolve()
  assert.deepEqual(new Set(started), new Set(['note-a', 'note-b']))

  noteA.resolve()
  noteB.resolve()
  await Promise.all(operations)
}

async function testRejectedOperationDoesNotBlockLane() {
  const queue = createNoteOperationQueue()
  const expected = new Error('expected failure')
  const failed = queue.enqueue('note-a', async () => { throw expected })
  const recovered = queue.enqueue('note-a', async () => 'recovered')

  await assert.rejects(failed, expected)
  assert.equal(await recovered, 'recovered')
}

async function testTargetAndGlobalFlush() {
  const queue = createNoteOperationQueue()
  const noteA = deferred()
  const noteB = deferred()
  void queue.enqueue('note-a', () => noteA.promise)
  void queue.enqueue('note-b', () => noteB.promise)

  const noteAFlush = queue.flush('note-a')
  let noteAFlushed = false
  void noteAFlush.then(() => { noteAFlushed = true })
  noteA.resolve()
  await noteAFlush
  assert.equal(noteAFlushed, true)

  let allFlushed = false
  const allFlush = queue.flush().then(() => { allFlushed = true })
  await Promise.resolve()
  assert.equal(allFlushed, false)
  noteB.resolve()
  await allFlush
  assert.equal(allFlushed, true)
}

function testRequestCounterTracksOverlappingRequests() {
  const counts = []
  const counter = createRequestCounter((count) => counts.push(count))
  const endFirst = counter.begin()
  const endSecond = counter.begin()
  assert.equal(counter.getCount(), 2)

  endFirst()
  assert.equal(counter.getCount(), 1)
  endFirst()
  assert.equal(counter.getCount(), 1)
  endSecond()
  assert.equal(counter.getCount(), 0)
  assert.deepEqual(counts, [1, 2, 1, 0])
}

function deferred() {
  let resolve
  let reject
  const promise = new Promise((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  })
  return { promise, resolve, reject }
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
