import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'latestRequestGuard.ts')
const outDir = path.join(rootDir, '.tmp', 'note-selection-test')
const outFile = path.join(outDir, 'latestRequestGuard.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

const { createLatestRequestGuard } = await import(pathToFileUrl(outFile))

try {
  await testLatestSelectionWinsWhenResponsesFinishInReverseOrder()
  await testStaleSelectionErrorIsIgnored()
  console.log('note selection tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function testLatestSelectionWinsWhenResponsesFinishInReverseOrder() {
  const guard = createLatestRequestGuard()
  const noteA = deferred()
  const noteB = deferred()
  let activeNote = null
  let isLoading = false

  const select = async (request) => {
    const isLatestRequest = guard.begin()
    isLoading = true
    try {
      const selectedNote = await request.promise
      if (isLatestRequest()) activeNote = selectedNote
    } finally {
      if (isLatestRequest()) isLoading = false
    }
  }

  const selectingA = select(noteA)
  const selectingB = select(noteB)

  noteB.resolve({ id: 'note-b' })
  await selectingB
  assert.equal(activeNote.id, 'note-b')
  assert.equal(isLoading, false)

  noteA.resolve({ id: 'note-a' })
  await selectingA
  assert.equal(activeNote.id, 'note-b')
  assert.equal(isLoading, false)
}

async function testStaleSelectionErrorIsIgnored() {
  const guard = createLatestRequestGuard()
  const noteA = deferred()
  const noteB = deferred()
  let activeNote = null
  let error = null

  const select = async (request) => {
    const isLatestRequest = guard.begin()
    try {
      const selectedNote = await request.promise
      if (isLatestRequest()) activeNote = selectedNote
    } catch (cause) {
      if (isLatestRequest()) error = cause.message
    }
  }

  const selectingA = select(noteA)
  const selectingB = select(noteB)

  noteB.resolve({ id: 'note-b' })
  await selectingB
  noteA.reject(new Error('note A failed'))
  await selectingA

  assert.equal(activeNote.id, 'note-b')
  assert.equal(error, null)
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
