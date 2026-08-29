import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import MarkdownIt from 'markdown-it'
import ts from 'typescript'

const rootDir = process.cwd()
const outDir = path.join(rootDir, '.tmp', 'table-copy-test')
const serializerPath = path.join(rootDir, 'src', 'utils', 'tiptapMarkdownSerializer.ts')
const clipboardPath = path.join(rootDir, 'src', 'utils', 'tableClipboard.ts')
const serializerOut = path.join(outDir, 'tiptapMarkdownSerializer.mjs')
const clipboardOut = path.join(outDir, 'tableClipboard.mjs')

await mkdir(outDir, { recursive: true })

try {
  await compile(serializerPath, serializerOut)
  await compile(clipboardPath, clipboardOut, [
    ["from './tiptapMarkdownSerializer'", "from './tiptapMarkdownSerializer.mjs'"],
  ])

  const {
    createTableClipboardPayload,
    createTiptapTableClipboardPayload,
    writeTableClipboard,
  } = await import(pathToFileUrl(clipboardOut))

  const table = createTable([
    tableRow([tableHeader('Name'), tableHeader('Value')]),
    tableRow([
      tableCell(''),
      tableCellContent([
        paragraph([
          text('bold | value', [{ type: 'bold' }]),
          { type: 'hardBreak' },
          text('next'),
        ]),
      ]),
    ]),
  ])
  const expectedMarkdown = '| Name | Value |\n| --- | --- |\n|  | **bold \\| value**&#10;next |'
  const payload = createTiptapTableClipboardPayload(table)

  assert.deepEqual(payload, {
    markdown: expectedMarkdown,
    plainText: expectedMarkdown,
  }, 'Rich table copy creates canonical Markdown text')
  assert.equal(payload.plainText.includes('\n\n'), false, 'table rows use a single line break')

  const markdown = new MarkdownIt({ html: false, linkify: true })
  const rendered = markdown.render(payload.markdown)
  assert.match(rendered, /<table>/, 'copied Markdown remains a table')
  assert.match(rendered, /<strong>bold \| value<\/strong>/, 'inline formatting and escaped pipes survive parsing')
  assert.match(rendered, /next/, 'cell line breaks remain in the copied representation')

  const formattedTable = createTable([
    tableRow([tableHeader('Style'), tableHeader('Value')]),
    tableRow([
      tableCellContent([
        paragraph([
          text('italic', [{ type: 'italic' }]),
          text(' '),
          text('strike', [{ type: 'strike' }]),
          text(' '),
          text('code', [{ type: 'code' }]),
          text(' '),
          text('link', [{ type: 'link', attrs: { href: 'https://example.com/?a=1|2' } }]),
        ]),
      ]),
      tableCell('ok'),
    ]),
  ])
  assert.equal(
    createTiptapTableClipboardPayload(formattedTable).markdown,
    '| Style | Value |\n| --- | --- |\n| *italic* ~~strike~~ `code` [link](https://example.com/?a=1\\|2) | ok |',
    'supported inline decorations survive table serialization',
  )

  const rawMarkdown = '| A | B |\n| :--- | ---: |\n| 1 | 2 |'
  assert.deepEqual(createTableClipboardPayload(rawMarkdown), {
    markdown: rawMarkdown,
    plainText: rawMarkdown,
  }, 'Markdown mode keeps the original table source')

  const writes = []
  class FakeClipboardItem {
    constructor(items) {
      this.items = items
    }
  }

  await writeTableClipboard(payload, {
    async write(items) {
      writes.push(items)
    },
    async writeText() {
      throw new Error('writeText should not be used when ClipboardItem is available')
    },
  }, FakeClipboardItem)

  assert.equal(writes.length, 1, 'plain-text ClipboardItem fallback writes one clipboard item')
  assert.equal(await writes[0][0].items['text/plain'].text(), expectedMarkdown)
  assert.deepEqual(
    Object.keys(writes[0][0].items),
    ['text/plain'],
    'Markdown copy avoids non-standard ClipboardItem MIME types',
  )

  const richPayload = createTableClipboardPayload(expectedMarkdown, '<table><tbody><tr><td>Value</td></tr></tbody></table>')
  await writeTableClipboard(richPayload, {
    async write(items) {
      writes.push(items)
    },
  }, FakeClipboardItem)
  assert.equal(writes.length, 2, 'rich table copy writes a second clipboard item')
  assert.equal(
    await writes[1][0].items['text/plain'].text(),
    expectedMarkdown,
    'rich table copy keeps Markdown in the plain-text clipboard representation',
  )
  assert.equal(
    await writes[1][0].items['text/html'].text(),
    richPayload.html,
    'rich table copy preserves an HTML representation for rich paste',
  )
  assert.deepEqual(
    Object.keys(writes[1][0].items).sort(),
    ['text/html', 'text/plain'],
    'rich table copy uses only standard HTML and plain-text clipboard formats',
  )

  let richFallbackText = null
  await assert.rejects(
    writeTableClipboard(richPayload, {
      async write() {
        throw new Error('clipboard write failed')
      },
      async writeText(value) {
        richFallbackText = value
      },
    }, FakeClipboardItem),
    /clipboard write failed/,
    'rich copy surfaces ClipboardItem failures instead of silently degrading to text',
  )
  assert.equal(richFallbackText, null, 'rich copy never falls back to Markdown-only text')

  await assert.rejects(
    writeTableClipboard(richPayload, {
      async writeText() {},
    }, FakeClipboardItem),
    /Rich clipboard API is unavailable/,
    'rich copy requires ClipboardItem support instead of degrading to plain text',
  )

  let fallbackText = null
  await writeTableClipboard(payload, {
    async writeText(value) {
      fallbackText = value
    },
  })
  assert.equal(fallbackText, expectedMarkdown, 'text/plain fallback keeps Markdown intact')
  await assert.rejects(
    writeTableClipboard(payload, {}),
    /Clipboard API is unavailable/,
    'missing Clipboard API is reported to the caller',
  )
} finally {
  await rm(outDir, { recursive: true, force: true })
}

async function compile(sourcePath, outputPath, replacements = []) {
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
      importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
    },
  })
  const output = replacements.reduce(
    (text, [from, to]) => text.replaceAll(from, to),
    compiled.outputText,
  )
  await writeFile(outputPath, output, 'utf8')
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}

function createTable(content) {
  return { type: 'table', content }
}

function paragraph(content) {
  return { type: 'paragraph', content }
}

function text(value, marks) {
  return { type: 'text', text: value, ...(marks ? { marks } : {}) }
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

function tableCellContent(content) {
  return { type: 'tableCell', content }
}
