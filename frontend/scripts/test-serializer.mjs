import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const serializerPath = path.join(rootDir, 'src', 'utils', 'tiptapMarkdownSerializer.ts')
const outDir = path.join(rootDir, '.tmp', 'serializer-test')
const outFile = path.join(outDir, 'tiptapMarkdownSerializer.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(serializerPath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
    importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

const { serializeTiptapJsonToMarkdown } = await import(pathToFileUrl(outFile))

const cases = [
  {
    name: 'heading level 2',
    input: doc(heading(2, [text('aa')])),
    expected: '## aa',
  },
  {
    name: 'inline marks',
    input: doc(paragraph([
      text('bold', [{ type: 'bold' }]),
      text(' and '),
      text('link', [{ type: 'link', attrs: { href: 'https://example.com' } }]),
    ])),
    expected: '**bold** and [link](https://example.com)',
  },
  {
    name: 'bullet and ordered lists',
    input: doc([
      bulletList([listItem(paragraph([text('one')]))]),
      orderedList([listItem(paragraph([text('two')]))], 3),
    ]),
    expected: '- one\n\n3. two',
  },
  {
    name: 'task list',
    input: doc(taskList([
      taskItem(false, paragraph([text('todo')])),
      taskItem(true, paragraph([text('done')])),
    ])),
    expected: '- [ ] todo\n- [x] done',
  },
  {
    name: 'blockquote',
    input: doc(blockquote(paragraph([text('quoted')]))),
    expected: '> quoted',
  },
  {
    name: 'code block',
    input: doc(codeBlock('ts', 'const a = 1')),
    expected: '```ts\nconst a = 1\n```',
  },
  {
    name: 'table',
    input: doc(table([
      tableRow([tableHeader('A'), tableHeader('B')]),
      tableRow([tableCell('1'), tableCell('2')]),
    ])),
    expected: '| A | B |\n| --- | --- |\n| 1 | 2 |',
  },
  {
    name: 'image',
    input: doc({ type: 'image', attrs: { src: 'note-assets/a.png', alt: 'a]b' } }),
    expected: '![a\\]b](note-assets/a.png)',
  },
]

for (const testCase of cases) {
  assert.equal(serializeTiptapJsonToMarkdown(testCase.input), testCase.expected, testCase.name)
}

await rm(outDir, { recursive: true, force: true })

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}

function doc(content) {
  return {
    type: 'doc',
    content: Array.isArray(content) ? content : [content],
  }
}

function paragraph(content) {
  return { type: 'paragraph', content }
}

function heading(level, content) {
  return { type: 'heading', attrs: { level }, content }
}

function text(value, marks) {
  return { type: 'text', text: value, ...(marks ? { marks } : {}) }
}

function bulletList(content) {
  return { type: 'bulletList', content }
}

function orderedList(content, start) {
  return { type: 'orderedList', attrs: { start }, content }
}

function listItem(...content) {
  return { type: 'listItem', content }
}

function taskList(content) {
  return { type: 'taskList', content }
}

function taskItem(checked, ...content) {
  return { type: 'taskItem', attrs: { checked }, content }
}

function blockquote(...content) {
  return { type: 'blockquote', content }
}

function codeBlock(language, value) {
  return {
    type: 'codeBlock',
    attrs: { language },
    content: [text(value)],
  }
}

function table(content) {
  return { type: 'table', content }
}

function tableRow(content) {
  return { type: 'tableRow', content }
}

function tableHeader(value) {
  return { type: 'tableHeader', content: [paragraph([text(value)])] }
}

function tableCell(value) {
  return { type: 'tableCell', content: [paragraph([text(value)])] }
}
