import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { JSDOM } from 'jsdom'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const apiPath = path.join(rootDir, 'src', 'api', 'noteExport.ts')
const storePath = path.join(rootDir, 'src', 'stores', 'useNoteExportStore.ts')
const documentPath = path.join(rootDir, 'src', 'utils', 'noteExportDocument.ts')
const noteLinkPath = path.join(rootDir, 'src', 'utils', 'noteLink.ts')
const editorPath = path.join(rootDir, 'src', 'components', 'NoteEditor.vue')
const appPath = path.join(rootDir, 'src', 'App.vue')
const switchGuardPath = path.join(rootDir, 'src', 'services', 'storageSpaceSwitch.ts')
const outDir = path.join(rootDir, '.tmp', 'note-export-test')
const storeOutFile = path.join(outDir, 'useNoteExportStore.mjs')
const documentOutFile = path.join(outDir, 'noteExportDocument.mjs')
const pdfMakeModuleUrl = pathToFileURL(path.join(rootDir, 'node_modules', 'pdfmake', 'build', 'pdfmake.js')).href
const regularFontPath = path.join(rootDir, 'src', 'assets', 'fonts', 'NotoSansJP-Regular.otf')
const boldFontPath = path.join(rootDir, 'src', 'assets', 'fonts', 'NotoSansJP-Bold.otf')
const originalFetch = globalThis.fetch

await mkdir(outDir, { recursive: true })

const compile = (source) =>
  ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
  }).outputText

const storeSource = (await readFile(storePath, 'utf8')).replace(
  "from '../api/noteExport'",
  "from './mock-note-export.mjs'",
)
const documentSource = (await readFile(documentPath, 'utf8'))
  .replace("from './noteLink'", "from './noteLink.mjs'")
  .replaceAll("import('pdfmake/build/pdfmake')", `import(${JSON.stringify(pdfMakeModuleUrl)})`)
  .replace("import('../assets/fonts/NotoSansJP-Regular.otf?url')", "import('./regular-font.mjs')")
  .replace("import('../assets/fonts/NotoSansJP-Bold.otf?url')", "import('./bold-font.mjs')")
  .replace("import('../assets/fonts/OFL.txt?url')", "import('./font-license.mjs')")
  .replace("import('../assets/licenses/pdfmake-MIT.txt?url')", "import('./pdfmake-license.mjs')")
await Promise.all([
  writeFile(storeOutFile, compile(storeSource), 'utf8'),
  writeFile(documentOutFile, compile(documentSource), 'utf8'),
  writeFile(path.join(outDir, 'noteLink.mjs'), compile(await readFile(noteLinkPath, 'utf8')), 'utf8'),
  writeFile(path.join(outDir, 'regular-font.mjs'), "export default 'atlasnote-test:regular'\n", 'utf8'),
  writeFile(path.join(outDir, 'bold-font.mjs'), "export default 'atlasnote-test:bold'\n", 'utf8'),
  writeFile(path.join(outDir, 'font-license.mjs'), "export default 'atlasnote-test:font-license'\n", 'utf8'),
  writeFile(path.join(outDir, 'pdfmake-license.mjs'), "export default 'atlasnote-test:pdfmake-license'\n", 'utf8'),
  writeFile(
    path.join(outDir, 'mock-note-export.mjs'),
    `
export const calls = []
let nextResult = { cancelled: false, exportedName: 'note.html' }
let nextError = null
let pendingResolver = null
let deferNext = false
export function setNextResult(result) { nextResult = structuredClone(result); nextError = null }
export function setNextError(error) { nextError = error; deferNext = false }
export function deferResult() { deferNext = true; nextError = null }
export function resolveDeferred(result) {
  if (!pendingResolver) throw new Error('no deferred export')
  const resolve = pendingResolver
  pendingResolver = null
  deferNext = false
  resolve(structuredClone(result))
}
export async function exportNote(input) {
  calls.push(structuredClone(input))
  if (nextError) throw nextError
  if (deferNext) return new Promise((resolve) => { pendingResolver = resolve })
  return structuredClone(nextResult)
}
`,
    'utf8',
  ),
])

try {
  const [apiSource, utilitySource, editorSource, appSource, switchGuardSource] = await Promise.all([
    readFile(apiPath, 'utf8'),
    readFile(documentPath, 'utf8'),
    readFile(editorPath, 'utf8'),
    readFile(appPath, 'utf8'),
    readFile(switchGuardPath, 'utf8'),
  ])

  assert.match(apiSource, /WailsApp[\s\S]*ExportNote/, 'the API wrapper must call Wails ExportNote')
  for (const field of [
    'noteId',
    'expectedRevision',
    'title',
    'markdown',
    'format',
    'htmlFragment',
    'pdfBase64',
    'allowPlaintextProtected',
  ]) {
    assert.match(apiSource, new RegExp(`\\b${field}\\b`), `the API input must include ${field}`)
  }
  assert.match(apiSource, /cancelled/, 'the API result must distinguish picker cancellation')
  assert.match(apiSource, /exportedName/, 'the API result must expose only the exported basename')
  assert.match(apiSource, /retryable/, 'the API error must retain retryability')

  assert.match(utilitySource, /setUrlAccessPolicy\(\(\) => false\)/, 'PDF URL access must be denied')
  assert.match(utilitySource, /NotoSansJP-Regular\.otf\?url/, 'the regular Japanese font must be bundled')
  assert.match(utilitySource, /NotoSansJP-Bold\.otf\?url/, 'the bold Japanese font must be bundled')
  assert.match(utilitySource, /OFL\.txt\?url/, 'the font license must be bundled')
  assert.match(utilitySource, /pdfmake-MIT\.txt\?url/, 'the pdfmake license must be bundled')
  assert.doesNotMatch(utilitySource, /\.innerHTML\s*=/, 'untrusted HTML must never be executed')
  assert.doesNotMatch(utilitySource, /\beval\s*\(/, 'untrusted HTML must never be evaluated')
  assert.doesNotMatch(editorSource, /wailsjs/, 'the editor component must not call Wails directly')
  assert.match(editorSource, /useNoteExportStore/, 'the editor must use the export store')
  assert.match(editorSource, /HTMLとしてエクスポート/, 'the editor must expose HTML export')
  assert.match(editorSource, /PDFとしてエクスポート/, 'the editor must expose PDF export')
  assert.match(editorSource, /runPrepared/, 'lock, flush, rendering, and Wails export must share one busy lifecycle')
  assert.match(editorSource, /requestAccess\(/, 'export must use the common content-lock access gate')
  assert.match(editorSource, /flushPendingDraft\(\)/, 'dirty content must be flushed before export')
  assert.match(editorSource, /createPdfBase64FromHtml/, 'PDF export must generate a direct PDF payload')
  assert.match(editorSource, /allowPlaintextProtected/, 'protected plaintext export must require confirmation')
  assert.match(editorSource, /expectedRevision: current\.revision/, 'the persisted revision must be sent to the backend')
  assert.match(editorSource, /markdown: current\.content/, 'the persisted Markdown snapshot must be sent to the backend')
  assert.match(appSource, /noteExportStore\.isBusy/, 'storage-space switching must observe export preparation')
  assert.match(switchGuardSource, /isExportBusy/, 'the switch guard must inspect export activity')
  assert.match(switchGuardSource, /STORAGE_SPACE_EXPORT_BUSY/, 'the switch guard needs an export-specific outcome')

  setActivePinia(createPinia())
  const mock = await import(pathToFileURL(path.join(outDir, 'mock-note-export.mjs')).href)
  const { useNoteExportStore } = await import(pathToFileURL(storeOutFile).href)
  const store = useNoteExportStore()
  const htmlInput = {
    noteId: 'note-1',
    expectedRevision: 7,
    title: 'Export',
    markdown: '# Export',
    format: 'html',
    htmlFragment: '<h1>Export</h1>',
    allowPlaintextProtected: false,
  }

  mock.setNextResult({ cancelled: false, exportedName: 'Export.html' })
  const completed = await store.run(htmlInput)
  assert.equal(completed.exportedName, 'Export.html')
  assert.equal(store.error, null)
  assert.equal(store.lastResult.exportedName, 'Export.html')
  assert.deepEqual(mock.calls.at(-1), htmlInput)

  mock.setNextResult({ cancelled: true })
  const cancelled = await store.run({ ...htmlInput, title: 'Cancel' })
  assert.equal(cancelled.cancelled, true)
  assert.equal(store.error, null)

  mock.setNextResult({
    cancelled: false,
    error: { code: 'NOTE_EXPORT_STALE', message: '更新されています。', retryable: true },
  })
  const failed = await store.run({ ...htmlInput, expectedRevision: 8 })
  assert.equal(failed.error.code, 'NOTE_EXPORT_STALE')
  assert.equal(store.error.code, 'NOTE_EXPORT_STALE')

  const previousResult = store.lastResult
  const previousError = store.error
  let resolvePreparation
  const preparing = store.runPrepared(
    () => new Promise((resolve) => { resolvePreparation = resolve }),
  )
  assert.equal(store.isBusy, true, 'preparation must be covered by the busy state')
  assert.equal(
    await store.run(htmlInput),
    null,
    'duplicate requests must be ignored while lock, flush, or PDF preparation is running',
  )
  assert.equal(store.lastResult, previousResult, 'preparation must not clear the previous result')
  assert.equal(store.error, previousError, 'preparation must not clear the previous error')
  resolvePreparation(null)
  assert.equal(await preparing, null)
  assert.equal(store.isBusy, false)
  assert.equal(store.lastResult, previousResult, 'cancelled preparation must retain the previous result')
  assert.equal(store.error, previousError, 'cancelled preparation must retain the previous error')

  mock.deferResult()
  const exporting = store.run(htmlInput)
  await Promise.resolve()
  assert.equal(store.isBusy, true)
  assert.equal(await store.run(htmlInput), null, 'duplicate Wails calls must be ignored')
  mock.resolveDeferred({ cancelled: false, exportedName: 'deferred.html' })
  await exporting
  assert.equal(store.isBusy, false)

  mock.setNextError(new Error('Wails unavailable'))
  const unavailable = await store.run(htmlInput)
  assert.equal(unavailable.error.code, 'NOTE_EXPORT_UNAVAILABLE')
  assert.equal(store.error.code, 'NOTE_EXPORT_UNAVAILABLE')
  assert.equal(store.lastResult.error.code, 'NOTE_EXPORT_UNAVAILABLE')

  const preparationFailure = await store.runPrepared(async () => {
    throw new Error('PDF rendering failed')
  })
  assert.equal(preparationFailure.error.code, 'NOTE_EXPORT_UNAVAILABLE')
  assert.equal(store.error.code, 'NOTE_EXPORT_UNAVAILABLE')
  store.reset()
  assert.equal(store.error, null)
  assert.equal(store.lastResult, null)

  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  globalThis.DOMParser = dom.window.DOMParser
  globalThis.URL = dom.window.URL
  const { createPdfBase64FromHtml, createPdfDocumentDefinition } = await import(
    pathToFileURL(documentOutFile).href
  )
  const noteId = 'a'.repeat(32)
  const documentDefinition = createPdfDocumentDefinition(
    `
      <script>script payload</script>
      <style>style payload</style>
      <h1>日本語の見出し</h1>
      <p>本文 <strong>太字</strong> <em>斜体</em> <del>取消</del> <code>code()</code><br>改行</p>
      <p>
        <a href="https://example.com/safe">外部リンク</a>
        <a href="https://user:password@example.com/secret">認証情報付きリンク</a>
        <a href="mailto:test@example.com">メール</a>
        <a href="tel:+81000000000">電話</a>
        <a href="#section-1">ページ内</a>
        <a href="atlasnote://note/${noteId}">ノート</a>
        <a href="atlasnote://note/invalid">不正ノート</a>
        <a href="javascript:alert(1)">危険リンク</a>
        <a href="data:text/html,attack">dataリンク</a>
        <a href="file:///secret">fileリンク</a>
        <img src="https://example.com/tracker.png" alt="画像の代替文字">
      </p>
      <blockquote><p>引用</p></blockquote>
      <ul>
        <li>箇条書き</li>
        <li data-checked="true"><input type="checkbox" checked>完了タスク</li>
        <li data-checked="false"><input type="checkbox">未完了タスク</li>
      </ul>
      <ol start="3"><li>番号付き</li></ol>
      <ul><li><p>リスト内段落</p><pre><code>nested code</code></pre>
        <table><tbody><tr><td>nested table cell</td></tr></tbody></table>
        <blockquote><p>nested quote</p></blockquote><ol><li>nested ordered item</li></ol>
        <p>リスト末尾</p></li></ul>
      <pre><code>const value = 1;\n  return value</code></pre>
      <hr>
      <table>
        <thead><tr><th>列A</th><th>列B</th></tr></thead>
        <tbody><tr><td>値1</td><td>値2</td></tr></tbody>
      </table>
      <svg><text>svg payload</text></svg>
      <canvas>canvas payload</canvas>
      <iframe>iframe payload</iframe>
      <div hidden>hidden payload</div>
    `,
    '制御\u0000文字を含む題名',
  )
  const serialized = JSON.stringify(documentDefinition)

  assert.equal(documentDefinition.pageSize, 'A4')
  assert.equal(documentDefinition.pageOrientation, 'portrait')
  assert.deepEqual(documentDefinition.pageMargins, [48, 48, 48, 48])
  assert.equal(documentDefinition.defaultStyle.font, 'NotoSansJP')
  assert.equal(documentDefinition.info.title, '制御 文字を含む題名')
  assert.match(serialized, /日本語の見出し/)
  assert.match(serialized, /太字/)
  assert.match(serialized, /箇条書き/)
  assert.match(serialized, /\[x\]/)
  assert.match(serialized, /\[ \]/)
  assert.match(serialized, /const value = 1/)
  for (const nestedContent of [
    'リスト内段落',
    'nested code',
    'nested table cell',
    'nested quote',
    'nested ordered item',
    'リスト末尾',
  ]) {
    assert.match(serialized, new RegExp(nestedContent), `list item must preserve ${nestedContent}`)
  }
  assert.ok(
    ['リスト内段落', 'nested code', 'nested table cell', 'nested quote', 'nested ordered item', 'リスト末尾']
      .map((value) => serialized.indexOf(value))
      .every((value, index, positions) => value >= 0 && (index === 0 || value > positions[index - 1])),
    'list item block content must preserve source order',
  )
  assert.match(serialized, /"table"/)
  assert.match(serialized, /https:\/\/example\.com\/safe/)
  assert.match(serialized, new RegExp(`atlasnote://note/${noteId}`))
  assert.match(serialized, /画像の代替文字/, 'image alt text must be retained')
  assert.doesNotMatch(serialized, /tracker\.png/, 'image resources must not be embedded')
  assert.doesNotMatch(serialized, /javascript:|data:text|file:\/\//i)
  assert.doesNotMatch(serialized, /user:password/i)
  assert.doesNotMatch(serialized, /atlasnote:\/\/note\/invalid/)
  assert.doesNotMatch(serialized, /script payload|style payload|svg payload|canvas payload|iframe payload/)
  assert.doesNotMatch(serialized, /hidden payload/)
  assert.doesNotMatch(serialized, /"image"|"svg"/, 'PDF content must not contain image or SVG nodes')

  const emptyDocument = createPdfDocumentDefinition('', '')
  assert.equal(emptyDocument.info.title, '無題')
  assert.equal(emptyDocument.content.length, 1)
  assert.deepEqual(emptyDocument.content[0].text, '')

  const requestedAssets = []
  globalThis.fetch = async (url, options) => {
    requestedAssets.push(String(url))
    assert.deepEqual(options, { credentials: 'same-origin', redirect: 'error' })
    const assetPath = url === 'atlasnote-test:regular'
      ? regularFontPath
      : url === 'atlasnote-test:bold'
        ? boldFontPath
        : null
    if (assetPath === null) throw new Error(`unexpected PDF asset request: ${url}`)
    const content = await readFile(assetPath)
    return {
      ok: true,
      async arrayBuffer() {
        return content.buffer.slice(content.byteOffset, content.byteOffset + content.byteLength)
      },
    }
  }
  const pdfBase64 = await createPdfBase64FromHtml(
    '<h1>日本語PDF</h1><p>本文 <strong>太字</strong></p><ul><li data-checked="true">完了</li></ul>',
    'PDF通常経路',
  )
  const pdfBytes = Buffer.from(pdfBase64, 'base64')
  assert.equal(pdfBytes.subarray(0, 5).toString('ascii'), '%PDF-')
  assert.match(pdfBytes.subarray(-32).toString('ascii'), /%%EOF\s*$/)
  assert.ok(pdfBytes.length > 1_000, 'the generated PDF must contain embedded font data')
  assert.deepEqual(requestedAssets.sort(), ['atlasnote-test:bold', 'atlasnote-test:regular'])

  console.log('note export tests passed')
} finally {
  globalThis.fetch = originalFetch
  await rm(outDir, { recursive: true, force: true })
}
