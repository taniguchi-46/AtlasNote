import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useAILibrarianStore.ts')
const outDir = path.join(rootDir, '.tmp', 'ai-librarian-test')
const outFile = path.join(outDir, 'useAILibrarianStore.mjs')

await mkdir(outDir, { recursive: true })

const source = (await readFile(sourcePath, 'utf8'))
  .replace("from '../api/ai'", "from './mock-ai-librarian.mjs'")
  .replace("from './useAIStore'", "from './mock-ai-store.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-ai-store.mjs'), `
export function useAIStore() {
  return {
    configuredSetting: { providerID: 'openrouter', modelID: 'openai/gpt-test' },
    isLibrarianReady: true,
  }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-ai-librarian.mjs'), `
export const calls = { start: [], cancel: [] }
let listener
let pendingStart

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
  return { canceled: true }
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

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-ai-librarian.mjs')).href)
  const { useAILibrarianStore } = await import(pathToFileURL(outFile).href)
  const store = useAILibrarianStore()

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
  assert.equal(await store.start(secondInput), true)
  mock.emit(event({ noteID: 'note-1', baseRevision: 4, partialText: 'ignored' }))
  assert.equal(store.partialText, '', 'late events for another note must be ignored')
  mock.emit(event({ noteID: 'note-3', baseRevision: 8, partialText: 'accepted' }))
  assert.equal(store.partialText, 'accepted')

  assert.equal(await store.cancel(), true)
  assert.deepEqual(mock.calls.cancel, ['request-1'])
  assert.equal(store.state, 'canceling')
  mock.emit(event({ noteID: 'note-3', baseRevision: 8, phase: 'canceled', sequence: 2 }))
  assert.equal(store.state, 'canceled')
  assert.equal(store.result, null, 'cancellation must not retain a result')

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
