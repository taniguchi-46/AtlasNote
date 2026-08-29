import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'updateNotesSequentially.ts')
const dependencyPath = path.join(rootDir, 'src', 'utils', 'noteBatch.ts')
const outDir = path.join(rootDir, '.tmp', 'note-batch-test')

await mkdir(outDir, { recursive: true })

function compile(source, outputFile) {
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  return writeFile(outputFile, compiled.outputText, 'utf8')
}

try {
  await compile(await readFile(dependencyPath, 'utf8'), path.join(outDir, 'noteBatch.mjs'))
  const source = (await readFile(sourcePath, 'utf8'))
    .replace("from './noteBatch'", "from './noteBatch.mjs'")
  await compile(source, path.join(outDir, 'updateNotesSequentially.mjs'))

  const { updateNotesSequentially, NoteUpdateError } = await import(
    pathToFileURL(path.join(outDir, 'updateNotesSequentially.mjs')).href,
  )

  const calls = []
  await assert.rejects(
    updateNotesSequentially(['a', 'b', 'c'], async (id) => {
      calls.push(id)
      if (id === 'b') throw new Error('update failed')
    }),
    (error) => {
      assert.ok(error instanceof NoteUpdateError)
      assert.equal(error.message, 'update failed')
      assert.equal(error.failedId, 'b')
      assert.deepEqual(error.updatedIds, ['a'])
      return true
    },
  )
  assert.deepEqual(calls, ['a', 'b'])
  assert.deepEqual(
    await updateNotesSequentially(['a', 'b'], async () => {}),
    ['a', 'b'],
  )
  console.log('note batch tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
