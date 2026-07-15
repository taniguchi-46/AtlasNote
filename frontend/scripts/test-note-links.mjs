import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'noteLink.ts')
const outDir = path.join(rootDir, '.tmp', 'note-links-test')
const outFile = path.join(outDir, 'noteLink.mjs')

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

  const {
    createNoteLinkHref,
    createNoteLinkMarkdown,
    findNoteLinkTargetAt,
    isNoteID,
    parseNoteLinkHref,
  } = await import(pathToFileUrl(outFile))

  const noteId = '0123456789abcdef0123456789abcdef'
  assert.equal(isNoteID(noteId), true)
  assert.equal(isNoteID(noteId.toUpperCase()), false)
  assert.equal(createNoteLinkHref(noteId), `atlasnote://note/${noteId}`)
  assert.equal(parseNoteLinkHref(`atlasnote://note/${noteId}`), noteId)
  assert.equal(parseNoteLinkHref('https://example.com'), null)
  assert.equal(parseNoteLinkHref(`atlasnote://note/${noteId.slice(0, -1)}`), null)
  assert.throws(() => createNoteLinkHref('not-a-note-id'))
  assert.equal(
    createNoteLinkMarkdown('A ] note\\name\nsecond', noteId),
    `[A \\] note\\\\name second](atlasnote://note/${noteId})`,
  )

  const markdownLink = `[Target](atlasnote://note/${noteId})`
  assert.equal(findNoteLinkTargetAt(markdownLink, 2), noteId)
  assert.equal(findNoteLinkTargetAt(markdownLink, markdownLink.indexOf(noteId) + 4), noteId)
  assert.equal(findNoteLinkTargetAt(markdownLink, markdownLink.length), noteId)
  assert.equal(findNoteLinkTargetAt(markdownLink, markdownLink.length + 1), null)

  const ignoredLinks = [
    `![Image](atlasnote://note/${noteId})`,
    `\`[Inline](atlasnote://note/${noteId})\``,
    `\\[Escaped](atlasnote://note/${noteId})`,
    '```',
    `[Fenced](atlasnote://note/${noteId})`,
    '```',
    '[External](https://example.com)',
    `[AfterFence](atlasnote://note/${noteId})`,
  ].join('\n')
  for (const label of ['Image', 'Inline', 'Escaped', 'Fenced']) {
    assert.equal(findNoteLinkTargetAt(ignoredLinks, ignoredLinks.indexOf(label) + 1), null, label)
  }
  assert.equal(findNoteLinkTargetAt(ignoredLinks, ignoredLinks.indexOf('AfterFence') + 1), noteId)

  console.log('note link utility tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
