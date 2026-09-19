import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { JSDOM } from 'jsdom'
import ts from 'typescript'

const outDir = path.join(process.cwd(), '.tmp', 'code-block-test')
const dom = new JSDOM('<!doctype html><html><body></body></html>')
for (const key of ['window', 'document', 'Element', 'HTMLElement', 'Node', 'Text', 'DOMParser', 'MutationObserver']) {
  globalThis[key] = dom.window[key]
}
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator })
globalThis.getSelection = dom.window.getSelection.bind(dom.window)

await mkdir(outDir, { recursive: true })
let editor
try {
  for (const name of ['imageResize', 'tiptapMarkdownSerializer']) {
    const source = await readFile(path.join('src', 'utils', `${name}.ts`), 'utf8')
    const compiled = ts.transpileModule(source, {
      compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
    })
    await writeFile(path.join(outDir, `${name}.mjs`),
      compiled.outputText.replace("'./imageResize'", "'./imageResize.mjs'"))
  }
  const { serializeTiptapJsonToMarkdown, restoreSerializedEmptyParagraphs } = await import(
    pathToFileURL(path.join(outDir, 'tiptapMarkdownSerializer.mjs'))
  )
  const { Editor } = await import('@tiptap/core')
  const { StarterKit } = await import('@tiptap/starter-kit')
  const { CodeBlockLowlight } = await import('@tiptap/extension-code-block-lowlight')
  const { createLowlight, common } = await import('lowlight')
  const { Markdown } = await import('tiptap-markdown')
  const { DOMParser } = await import('@tiptap/pm/model')
  const { history, closeHistory, undo, redo } = await import('@tiptap/pm/history')
  editor = new Editor({
    element: document.body.appendChild(document.createElement('div')),
    extensions: [
      StarterKit.configure({ codeBlock: false, undoRedo: false }),
      Markdown.configure({ html: false, linkify: true }),
      CodeBlockLowlight.configure({ lowlight: createLowlight(common) }),
    ],
  })
  editor.registerPlugin(history())
  function load(markdown) {
    const container = document.createElement('div')
    container.innerHTML = editor.storage.markdown.parser.parse(markdown)
    restoreSerializedEmptyParagraphs(container)
    editor.commands.setContent(DOMParser.fromSchema(editor.schema).parse(container).toJSON(), { emitUpdate: false })
  }
  const sources = [
    '```mermaid\ngraph TD\n  A[開始] --> B[終了]\n```\n\n後置段落',
    '```javascript\nconst value = "<script>"\n```\n\n後置段落',
    '前置段落\n\n&nbsp;\n\n&nbsp;\n\n後置段落',
  ]
  for (const source of sources) {
    load(source)
    assert.equal(serializeTiptapJsonToMarkdown(editor.getJSON()), source,
      'code source and empty paragraphs survive the Rich/Markdown round-trip')
  }
  load(sources[0])
  assert.equal(editor.state.doc.firstChild.attrs.language, 'mermaid')
  assert.equal(editor.view.dom.querySelector('pre code').textContent, 'graph TD\n  A[開始] --> B[終了]')
  assert.equal(editor.view.dom.querySelector('svg, img, textarea'), null,
    'Mermaid source is displayed as ordinary editable code')
  editor.commands.setTextSelection(1)
  editor.view.dispatch(closeHistory(editor.state.tr))
  editor.commands.insertContent('%% 編集\n')
  assert.match(serializeTiptapJsonToMarkdown(editor.getJSON()), /%% 編集/)
  assert.equal(undo(editor.state, editor.view.dispatch), true)
  assert.equal(serializeTiptapJsonToMarkdown(editor.getJSON()), sources[0])
  assert.equal(redo(editor.state, editor.view.dispatch), true)
  assert.match(serializeTiptapJsonToMarkdown(editor.getJSON()), /%% 編集/)
  console.log('code block round-trip and history tests passed')
} finally {
  editor?.destroy()
  dom.window.close()
  await rm(outDir, { recursive: true, force: true })
}
