import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'deleteNotesSequentially.ts')
const notificationPath = path.join(rootDir, 'src', 'components', 'NotificationCenter.vue')
const outDir = path.join(rootDir, '.tmp', 'note-delete-test')
const outFile = path.join(outDir, 'deleteNotesSequentially.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

const { deleteNotesSequentially, NoteDeleteError } = await import(pathToFileURL(outFile).href)

try {
  await testAllDeletesSucceed()
  await testFailureReportsOnlyCompletedDeletes()
  await testDeleteErrorIsRenderedAsAnAlert()
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

async function testAllDeletesSucceed() {
  const calls = []
  const deletedIds = await deleteNotesSequentially(['a', 'b'], async (id) => {
    calls.push(id)
  })

  assert.deepEqual(calls, ['a', 'b'])
  assert.deepEqual(deletedIds, ['a', 'b'])
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
      assert.deepEqual(error.deletedIds, ['a'])
      return true
    },
  )
  assert.deepEqual(calls, ['a', 'b'])
}
