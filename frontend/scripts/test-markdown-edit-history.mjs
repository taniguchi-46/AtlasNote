import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'markdownEditHistory.ts')
const outDir = path.join(rootDir, '.tmp', 'markdown-edit-history-test')
const outFile = path.join(outDir, 'markdownEditHistory.mjs')

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
  const { createMarkdownEditHistory } = await import(pathToFileURL(outFile).href)

  let time = 0
  const initial = { content: '', selectionStart: 0, selectionEnd: 0 }
  const history = createMarkdownEditHistory(initial, { now: () => time, groupDelayMs: 500 })

  const first = { content: 'a', selectionStart: 1, selectionEnd: 1 }
  history.record(initial, first, { group: 'insert-text' })
  time += 100
  const second = { content: 'ab', selectionStart: 2, selectionEnd: 2 }
  history.record(first, second, { group: 'insert-text' })
  assert.deepEqual(history.undo(), initial, 'nearby typing must undo as one group')
  assert.deepEqual(history.redo(), second, 'redo must restore grouped typing and selection')

  time += 1000
  const formatted = { content: '**ab**', selectionStart: 2, selectionEnd: 4 }
  history.record(second, formatted, { group: 'command', forceNewGroup: true })
  assert.deepEqual(history.undo(), second, 'toolbar changes must have their own undo step')

  const replacement = { content: 'replacement', selectionStart: 11, selectionEnd: 11 }
  history.record(second, replacement, { group: 'input', forceNewGroup: true })
  assert.equal(history.redo(), null, 'editing after undo must clear the redo branch')

  history.reset({ content: 'secret', selectionStart: 6, selectionEnd: 6 })
  assert.equal(history.undo(), null, 'reset must remove prior snapshots')
  assert.equal(history.redo(), null, 'reset must remove redo snapshots')

  const clamped = createMarkdownEditHistory({
    content: 'abc',
    selectionStart: -10,
    selectionEnd: 99,
  }).current()
  assert.deepEqual(clamped, { content: 'abc', selectionStart: 0, selectionEnd: 3 })

  console.log('Markdown edit history tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
