import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'operationLogger.ts')
const outDir = path.join(rootDir, '.tmp', 'operation-logger-test')
const outFile = path.join(outDir, 'operationLogger.mjs')

await mkdir(outDir, { recursive: true })

try {
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(outFile, compiled.outputText, 'utf8')

  const { logOperationFailure } = await import(pathToFileURL(outFile).href)
  const calls = []
  const originalError = console.error
  console.error = (...args) => calls.push(args)
  try {
    logOperationFailure({
      operationId: 'operation-1',
      noteId: 'note-1',
      stage: 'save',
      errorCategory: 'io',
      body: 'must not be logged',
    })
  } finally {
    console.error = originalError
  }

  assert.deepEqual(calls, [[
    'operation failed',
    { operationId: 'operation-1', noteId: 'note-1', stage: 'save', errorCategory: 'io' },
  ]])
  console.log('operation logger tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
