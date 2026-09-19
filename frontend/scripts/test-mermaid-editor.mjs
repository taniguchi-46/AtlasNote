import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import { JSDOM } from 'jsdom'
import { compileScript, parse } from '@vue/compiler-sfc'
import ts from 'typescript'

const rootDir = process.cwd()
const outDir = path.join(rootDir, '.tmp', 'mermaid-editor-test')
const clipboardOut = path.join(outDir, 'mermaidClipboard.mjs')
const serializerOut = path.join(outDir, 'tiptapMarkdownSerializer.mjs')
const imageResizeOut = path.join(outDir, 'imageResize.mjs')
const sessionOut = path.join(outDir, 'mermaidEditorSession.mjs')
const beforeLockOut = path.join(outDir, 'contentLockBeforeLock.mjs')
const pasteOut = path.join(outDir, 'mermaidPaste.mjs')
const dialogOut = path.join(outDir, 'MermaidEditDialog.mjs')
const visualEditorOut = path.join(outDir, 'MermaidVisualEditor.mjs')
const visualEditorModelOut = path.join(outDir, 'mermaidVisualEditorModel.mjs')
const nodeViewOut = path.join(outDir, 'MermaidCodeBlockView.mjs')
const nodeViewProductionOut = path.join(outDir, 'MermaidCodeBlockView.production.mjs')
const insertionOut = path.join(outDir, 'mermaidInsertion.mjs')
let restoreEmptyParagraphs = () => {}
const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'https://atlasnote.test/',
})

for (const key of [
  'window',
  'document',
  'Element',
  'HTMLElement',
  'Document',
  'SVGElement',
  'Node',
  'Text',
  'DOMParser',
  'Range',
  'MutationObserver',
  'Event',
]) {
  globalThis[key] = dom.window[key]
}
globalThis.navigator = dom.window.navigator
globalThis.getSelection = dom.window.getSelection.bind(dom.window)
globalThis.requestAnimationFrame = (callback) => setTimeout(callback, 0)
globalThis.cancelAnimationFrame = (handle) => clearTimeout(handle)

await mkdir(outDir, { recursive: true })

try {
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'mermaidClipboard.ts'), clipboardOut)
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'imageResize.ts'), imageResizeOut)
  await compileTypeScript(
    path.join(rootDir, 'src', 'utils', 'tiptapMarkdownSerializer.ts'),
    serializerOut,
    [["from './imageResize'", "from './imageResize.mjs'"]],
  )
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'mermaidEditorSession.ts'), sessionOut)
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'contentLockBeforeLock.ts'), beforeLockOut)
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'mermaidInsertion.ts'), insertionOut)
  await compileTypeScript(
    path.join(rootDir, 'src', 'utils', 'mermaidPaste.ts'),
    pasteOut,
    [["from './mermaidClipboard'", "from './mermaidClipboard.mjs'"]],
  )
  await compileTypeScript(
    path.join(rootDir, 'src', 'utils', 'mermaidVisualEditor.ts'),
    visualEditorModelOut,
  )
  await compileVueComponent(
    path.join(rootDir, 'src', 'components', 'MermaidVisualEditor.vue'),
    visualEditorOut,
    [["from '../utils/mermaidVisualEditor'", "from './mermaidVisualEditorModel.mjs'"]],
  )
  await compileVueComponent(
    path.join(rootDir, 'src', 'components', 'MermaidEditDialog.vue'),
    dialogOut,
    [
      ["from 'reka-ui'", "from './component-mocks.mjs'"],
      ["from '../utils/mermaidRenderer'", "from './component-mocks.mjs'"],
      ["from './MermaidVisualEditor.vue'", "from './MermaidVisualEditor.mjs'"],
    ],
  )
  await compileVueComponent(
    path.join(rootDir, 'src', 'components', 'MermaidCodeBlockView.vue'),
    nodeViewOut,
    [
      ["from '@tiptap/vue-3'", "from './component-mocks.mjs'"],
      ["from '@tiptap/pm/model'", "from './component-mocks.mjs'"],
      ["from '../stores/useAppStore'", "from './component-mocks.mjs'"],
      ["from '../stores/useNoteStore'", "from './component-mocks.mjs'"],
      ["from '../utils/mermaidRenderer'", "from './component-mocks.mjs'"],
      ["from './MermaidEditDialog.vue'", "from './MermaidEditDialog.mjs'"],
    ],
  )
  await compileVueComponent(
    path.join(rootDir, 'src', 'components', 'MermaidCodeBlockView.vue'),
    nodeViewProductionOut,
    [
      ["from '../stores/useAppStore'", "from './component-mocks.mjs'"],
      ["from '../stores/useNoteStore'", "from './component-mocks.mjs'"],
      ["from '../utils/mermaidRenderer'", "from './component-mocks.mjs'"],
      ["from './MermaidEditDialog.vue'", "from './MermaidEditDialog.mjs'"],
    ],
  )
  await writeFile(path.join(outDir, 'component-mocks.mjs'), componentMocksSource(), 'utf8')

  const { createMermaidFence, readMermaidClipboardPayload } = await import(pathToFileURL(clipboardOut))
  const {
    restoreSerializedEmptyParagraphs,
    serializeTiptapJsonToMarkdown,
  } = await import(pathToFileURL(serializerOut))
  restoreEmptyParagraphs = restoreSerializedEmptyParagraphs
  const { flushMermaidEditorInputs, setMermaidEditorInputsLocked } = await import(pathToFileURL(sessionOut))
  const { createContentLockBeforeLock } = await import(pathToFileURL(beforeLockOut))
  const { insertMermaidCodeBlock } = await import(pathToFileURL(insertionOut))
  const { handleMermaidPaste } = await import(pathToFileURL(pasteOut))
  const Dialog = (await import(pathToFileURL(dialogOut))).default
  const NodeView = (await import(pathToFileURL(nodeViewOut))).default
  const ProductionNodeView = (await import(pathToFileURL(nodeViewProductionOut))).default
  const componentMocks = await import(pathToFileURL(path.join(outDir, 'component-mocks.mjs')))
  const { Editor } = await import('@tiptap/core')
  const StarterKit = (await import('@tiptap/starter-kit')).default
  const { CodeBlockLowlight } = await import('@tiptap/extension-code-block-lowlight')
  const { common, createLowlight } = await import('lowlight')
  const { Markdown } = await import('tiptap-markdown')
  const { DOMParser: ProseMirrorDOMParser } = await import('@tiptap/pm/model')
  const { NodeSelection } = await import('@tiptap/pm/state')
  const { history, undo } = await import('@tiptap/pm/history')
  const { Editor: VueEditor, EditorContent, VueNodeViewRenderer } = await import('@tiptap/vue-3')

  const editor = createEditor(Editor, StarterKit, CodeBlockLowlight, createLowlight, common, Markdown)

  const MermaidInsertionCodeBlock = CodeBlockLowlight.extend({
    addStorage() {
      return {
        ...this.parent?.(),
        openMermaidEditorOnSelect: false,
      }
    },
  })
  const insertionEditor = new Editor({
    extensions: [
      StarterKit.configure({ codeBlock: false, undoRedo: false }),
      MermaidInsertionCodeBlock.configure({ lowlight: createLowlight(common) }),
    ],
    content: {
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'before' }] },
        { type: 'codeBlock', attrs: { language: 'mermaid' }, content: [{ type: 'text', text: 'flowchart TD\n  A --> B' }] },
        { type: 'paragraph', content: [{ type: 'text', text: 'middle' }] },
        { type: 'codeBlock', attrs: { language: 'mermaid' }, content: [{ type: 'text', text: 'flowchart TD\n  C --> D' }] },
        { type: 'paragraph', content: [{ type: 'text', text: 'after' }] },
      ],
    },
  })
  insertionEditor.registerPlugin(history({ depth: 20, newGroupDelay: 500 }))
  let middleParagraphPosition = -1
  insertionEditor.state.doc.descendants((node, position) => {
    if (node.type.name === 'paragraph' && node.textContent === 'middle') middleParagraphPosition = position + 1
  })
  assert.notEqual(middleParagraphPosition, -1)
  insertionEditor.commands.setTextSelection(middleParagraphPosition)
  const insertedSource = 'flowchart TD\n  X --> Y'
  assert.equal(insertMermaidCodeBlock(insertionEditor, insertedSource), true)
  const insertedMermaidPositions = []
  insertionEditor.state.doc.descendants((node, position) => {
    if (node.type.name === 'codeBlock' && String(node.attrs.language ?? '').toLowerCase() === 'mermaid') {
      insertedMermaidPositions.push(position)
    }
  })
  assert.equal(insertedMermaidPositions.length, 3)
  assert.ok(insertionEditor.state.selection instanceof NodeSelection)
  assert.equal(insertionEditor.state.selection.node.textContent, insertedSource)
  assert.equal(insertionEditor.state.selection.from, insertedMermaidPositions[1],
    'inserting before the last Mermaid block must select the inserted block')
  assert.equal(insertionEditor.storage.codeBlock.openMermaidEditorOnSelect, true,
    'the inserted block must request the Mermaid edit dialog')
  assert.equal(undo(insertionEditor.state, insertionEditor.view.dispatch), true,
    'the insertion must remain one undoable transaction')
  assert.equal(insertionEditor.state.doc.childCount, 5)
  insertionEditor.destroy()

  const mixedMarkdown = [
    '前置段落',
    '',
    '```mermaid',
    'flowchart TD',
    '  A[開始] --> B[終了]',
    '```',
    '',
    '後置段落',
  ].join('\n')
  const mixedDocument = createMarkdownDocument(editor, mixedMarkdown, ProseMirrorDOMParser)
  assert.deepEqual(
    Array.from({ length: mixedDocument.childCount }, (_, index) => mixedDocument.child(index).type.name),
    ['paragraph', 'codeBlock', 'paragraph'],
    'fenced Mermaid paste parsing keeps surrounding blocks',
  )
  assert.equal(mixedDocument.child(1).attrs.language, 'mermaid')
  assert.equal(mixedDocument.child(1).textContent, 'flowchart TD\n  A[開始] --> B[終了]')

  const emptyParagraphMarkdown = '前置段落\n\n&nbsp;\n\n&nbsp;\n\n後置段落'
  const emptyParagraphDocument = createMarkdownDocument(
    editor,
    emptyParagraphMarkdown,
    ProseMirrorDOMParser,
  )
  assert.deepEqual(
    Array.from({ length: emptyParagraphDocument.childCount }, (_, index) => ({
      type: emptyParagraphDocument.child(index).type.name,
      childCount: emptyParagraphDocument.child(index).childCount,
    })),
    [
      { type: 'paragraph', childCount: 1 },
      { type: 'paragraph', childCount: 0 },
      { type: 'paragraph', childCount: 0 },
      { type: 'paragraph', childCount: 1 },
    ],
    'serialized empty paragraphs are restored as empty Rich paragraphs',
  )
  assert.equal(
    serializeTiptapJsonToMarkdown(emptyParagraphDocument.toJSON()),
    emptyParagraphMarkdown,
    'empty paragraphs survive Markdown to Rich to Markdown round-trip',
  )

  editor.commands.setContent(mixedDocument.toJSON(), { emitUpdate: false })
  const mixedSerialized = serializeTiptapJsonToMarkdown(editor.getJSON())
  assert.equal(mixedSerialized, mixedMarkdown)

  const sourceClipboard = clipboardData({
    'text/plain': 'flowchart LR\n  A --> B',
  })
  assert.deepEqual(readMermaidClipboardPayload(sourceClipboard), {
    kind: 'source',
    source: 'flowchart LR\n  A --> B',
  })
  editor.commands.setContent({ type: 'doc', content: [{ type: 'paragraph' }] }, { emitUpdate: false })
  editor.commands.setTextSelection(1)
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: sourceClipboard },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), true, 'the product Mermaid paste handler accepts raw Mermaid source')
  assert.equal(editor.state.doc.firstChild.type.name, 'codeBlock')
  assert.equal(editor.state.doc.firstChild.attrs.language, 'mermaid')
  assert.equal(editor.state.doc.firstChild.textContent, 'flowchart LR\n  A --> B')
  assert.ok(!(editor.state.selection instanceof NodeSelection), 'raw Mermaid paste leaves the node selection')
  assert.equal(editor.state.selection.$from.parent.type.name, 'paragraph')
  const pastedSource = editor.state.doc.firstChild.textContent
  editor.view.dom.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  assert.equal(editor.state.doc.firstChild.textContent, pastedSource, 'Enter must not edit hidden Mermaid source')

  editor.commands.setContent({
    type: 'doc',
    content: [{ type: 'paragraph', content: [{ type: 'text', text: '前置' }] }],
  }, { emitUpdate: false })
  editor.commands.setTextSelection(3)
  const fencedPaste = clipboardData({
    'text/plain': '```mermaid\nflowchart TD\n  A --> B\n```',
  })
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: fencedPaste },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), true, 'the product Mermaid paste handler accepts fenced Markdown')
  assert.match(serializeTiptapJsonToMarkdown(editor.getJSON()), /```mermaid\nflowchart TD\n  A --> B\n```/)
  assert.match(editor.state.doc.textContent, /前置/)

  editor.commands.setContent({
    type: 'doc',
    content: [{ type: 'paragraph', content: [{ type: 'text', text: '前AA後' }] }],
  }, { emitUpdate: false })
  editor.commands.setTextSelection({ from: 2, to: 4 })
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: fencedPaste },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), true, 'a fenced Mermaid paste must replace a selected text range')
  const selectedPastePosition = findMermaidCodeBlockPosition(editor)
  const selectedPasteSource = editor.state.doc.nodeAt(selectedPastePosition).textContent
  const selectedPasteMarkdown = serializeTiptapJsonToMarkdown(editor.getJSON())
  assert.match(selectedPasteMarkdown, /前/)
  assert.match(selectedPasteMarkdown, /後/)
  assert.equal(editor.state.selection.$from.parent.type.name, 'paragraph',
    'a selected fenced Mermaid paste must place the cursor outside hidden source')
  editor.view.dom.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  assert.equal(editor.state.doc.nodeAt(selectedPastePosition).textContent, selectedPasteSource,
    'Enter after a selected fenced paste must not edit hidden source')

  editor.commands.setContent({ type: 'doc', content: [{ type: 'paragraph' }] }, { emitUpdate: false })
  editor.commands.setTextSelection(1)
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: fencedPaste },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), true, 'a standalone fenced Mermaid block must be accepted')
  const standaloneMermaidPosition = findMermaidCodeBlockPosition(editor)
  const standaloneMermaidSource = editor.state.doc.nodeAt(standaloneMermaidPosition).textContent
  assert.equal(editor.state.selection.$from.parent.type.name, 'paragraph',
    'standalone fenced Mermaid paste must place the cursor outside hidden source')
  editor.view.dom.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  assert.equal(editor.state.doc.nodeAt(standaloneMermaidPosition).textContent, standaloneMermaidSource,
    'Enter after standalone fenced paste must not edit hidden Mermaid source')

  const trailingMermaidMarkdown = [
    '前置段落',
    '',
    '```typescript',
    'const value = 1',
    '```',
    '',
    '```mermaid',
    'flowchart TD',
    '  A --> B',
    '```',
  ].join('\n')
  editor.commands.setContent({ type: 'doc', content: [{ type: 'paragraph' }] }, { emitUpdate: false })
  editor.commands.setTextSelection(1)
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: clipboardData({ 'text/plain': trailingMermaidMarkdown }) },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), true, 'multiple blocks ending in Mermaid must be accepted')
  const trailingMermaidPosition = findMermaidCodeBlockPosition(editor)
  const trailingMermaidSource = editor.state.doc.nodeAt(trailingMermaidPosition).textContent
  assert.equal(editor.state.selection.$from.parent.type.name, 'paragraph',
    'a Mermaid block at the end of a multi-block paste must not retain selection in source')
  editor.view.dom.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
  assert.equal(editor.state.doc.nodeAt(trailingMermaidPosition).textContent, trailingMermaidSource,
    'Enter after a multi-block Mermaid paste must not edit hidden source')

  editor.commands.setContent({
    type: 'doc',
    content: [{
      type: 'codeBlock',
      attrs: { language: 'typescript' },
      content: [{ type: 'text', text: 'const value = 1' }],
    }],
  }, { emitUpdate: false })
  editor.commands.setTextSelection(2)
  assert.equal(handleMermaidPaste({
    editor,
    view: editor.view,
    event: { clipboardData: sourceClipboard },
    parseMarkdown: (markdown) => createMarkdownDocument(editor, markdown, ProseMirrorDOMParser),
  }), false, 'Mermaid paste must not replace an ordinary code block')
  assert.equal(editor.state.doc.firstChild.textContent, 'const value = 1')

  const originalSource = 'flowchart TD\n  A --> B'
  const changedSource = 'flowchart TD\n  A --> C'
  editor.commands.setContent({
    type: 'doc',
    content: [{ type: 'codeBlock', attrs: { language: 'mermaid' }, content: [{ type: 'text', text: originalSource }] }],
  }, { emitUpdate: false })
  editor.registerPlugin(history({ depth: 20, newGroupDelay: 500 }))
  const mermaidPosition = findCodeBlockPosition(editor)
  const currentNode = editor.state.doc.nodeAt(mermaidPosition)
  editor.view.dispatch(editor.state.tr.replaceWith(
    mermaidPosition + 1,
    mermaidPosition + currentNode.nodeSize - 1,
    editor.schema.text(changedSource),
  ))
  assert.equal(editor.state.doc.firstChild.textContent, changedSource)
  assert.equal(undo(editor.state, editor.view.dispatch), true)
  assert.equal(editor.state.doc.firstChild.textContent, originalSource)

  const copiedSource = editor.state.doc.firstChild.textContent
  assert.equal(createMermaidFence(copiedSource), `\`\`\`mermaid\n${copiedSource}\n\`\`\``)
  assert.equal(createMermaidFence('flowchart TD\n  A --> B\n``\`'), '````mermaid\nflowchart TD\n  A --> B\n```\n````')

  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/plain': 'これは通常のテキストです',
  })), null)
  assert.deepEqual(readMermaidClipboardPayload(clipboardData({
    'text/html': '<pre><code class="language-mermaid">flowchart TD\n  A --&gt; B</code></pre>',
  })), {
    kind: 'source',
    source: 'flowchart TD\n  A --> B',
  })
  const sourceWithWhitespace = ' \r\nflowchart TD\r\n  A --> B\r\n '
  assert.deepEqual(readMermaidClipboardPayload(clipboardData({
    'text/plain': sourceWithWhitespace,
  })), {
    kind: 'source',
    source: sourceWithWhitespace,
  })
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<pre><code class="language-mermaid">flowchart TD</code></pre><pre><code class="language-mermaid">flowchart LR</code></pre>',
    'text/plain': 'flowchart TD\nflowchart LR',
  })), null, 'multiple HTML Mermaid blocks must fall back instead of being joined')
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<p>前置</p><pre><code class="language-mermaid">flowchart TD</code></pre><p>後置</p>',
    'text/plain': '前置\nflowchart TD\n後置',
  })), null, 'HTML surrounding text must not be dropped')
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<div class="mermaid">flowchart TD\n  A --&gt; B</div>',
    'text/plain': 'flowchart TD\n  A --> B',
  })), null)
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<pre><code class="language-typescript">flowchart TD</code></pre>',
    'text/plain': 'flowchart TD',
  })), null)
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<pre><code>flowchart TD</code></pre>',
    'text/plain': 'flowchart TD',
  })), null, 'unclassified HTML code must remain normal code')
  assert.equal(readMermaidClipboardPayload(clipboardData({
    'text/html': '<p>前置</p><p>flowchart TD</p>',
    'text/plain': '前置\nflowchart TD',
  })), null, 'HTML surrounding text must not be dropped during raw fallback')
  assert.deepEqual(readMermaidClipboardPayload(clipboardData({
    'text/html': '<p>flowchart TD</p>',
    'text/plain': '```mermaid\nflowchart TD\n```',
  })), {
    kind: 'markdown',
    markdown: '```mermaid\nflowchart TD\n```',
  })

  await testProductMermaidComponents(
    Dialog,
    NodeView,
    ProductionNodeView,
    componentMocks,
    dom.window,
    {
      Editor,
      VueEditor,
      EditorContent,
      StarterKit,
      CodeBlockLowlight,
      VueNodeViewRenderer,
      common,
      createLowlight,
      flushMermaidEditorInputs,
      setMermaidEditorInputsLocked,
      createContentLockBeforeLock,
    },
  )

  editor.destroy()
  console.log('Mermaid editor integration tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
  dom.window.close()
}

async function compileTypeScript(sourcePath, outputPath, replacements = []) {
  const source = await readFile(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
      importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
    },
    fileName: sourcePath,
  })
  const output = replacements.reduce(
    (text, [from, to]) => text.replaceAll(from, to),
    compiled.outputText,
  )
  await writeFile(outputPath, output, 'utf8')
}

async function compileVueComponent(sourcePath, outputPath, replacements = []) {
  const source = await readFile(sourcePath, 'utf8')
  const { descriptor, errors } = parse(source)
  if (errors.length > 0) throw new Error(`failed to parse ${sourcePath}`)

  const compiled = compileScript(descriptor, {
    id: path.basename(sourcePath),
    inlineTemplate: true,
  }).content
  const output = replacements.reduce(
    (text, [from, to]) => text.replaceAll(from, to),
    compiled,
  )
  await writeFile(outputPath, ts.transpileModule(output, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
      importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
    },
    fileName: sourcePath,
  }).outputText, 'utf8')
}

function componentMocksSource() {
  return `
import { h, reactive } from 'vue'

export const nodeViewProps = { node: Object, editor: Object, extension: Object, getPos: Function, selected: Boolean }
export const Fragment = { empty: {} }
export const store = reactive({ theme: 'light' })
export const useAppStore = () => store
export const deletionPreparing = reactive(new Set())
export const useNoteStore = () => ({
  isNoteDeletionPreparing: (noteId) => deletionPreparing.has(noteId),
})
export const pendingRenders = []
export const dialogEvents = { escape: null, outside: null }

export function renderMermaidDiagram(source, options) {
  return new Promise((resolve, reject) => pendingRenders.push({ source, options, resolve, reject }))
}

export function resolveRender(index, result = { ok: true, svg: '<svg/>', altText: 'Mermaid図' }) {
  const pending = pendingRenders[index]
  if (!pending) throw new Error('pending Mermaid render was not found')
  pending.resolve(result)
}

export const NodeViewWrapper = {
  inheritAttrs: false,
  props: { as: { type: String, default: 'div' } },
  setup(props, { attrs, slots }) {
    return () => h(props.as, attrs, slots.default?.())
  },
}

export const NodeViewContent = {
  inheritAttrs: false,
  props: { as: { type: String, default: 'div' } },
  setup(props, { attrs }) {
    return () => h(props.as, attrs)
  },
}

export const DialogRoot = {
  props: { open: Boolean },
  setup(props, { slots }) {
    return () => props.open ? h('div', slots.default?.()) : null
  },
}
export const DialogPortal = { setup(_, { slots }) { return () => h('div', slots.default?.()) } }
export const DialogOverlay = { setup(_, { slots }) { return () => h('div', slots.default?.()) } }
export const DialogTitle = { setup(_, { slots }) { return () => h('h2', slots.default?.()) } }
export const DialogDescription = { setup(_, { slots }) { return () => h('p', slots.default?.()) } }
export const DialogContent = {
  setup(_, { slots, emit }) {
    dialogEvents.escape = () => emit('escapeKeyDown', { preventDefault() {} })
    dialogEvents.outside = () => emit('interactOutside', { preventDefault() {} })
    return () => h('div', slots.default?.())
  },
}
`
}

async function testProductMermaidComponents(
  Dialog,
  NodeView,
  ProductionNodeView,
  mocks,
  browserWindow,
  integration,
) {
  const { createApp, h, nextTick, reactive } = await import('vue')
  const previousCreateObjectUrl = URL.createObjectURL
  const previousRevokeObjectUrl = URL.revokeObjectURL
  const created = []
  const revoked = []
  URL.createObjectURL = () => {
    const url = `blob:product-mermaid-${created.length}`
    created.push(url)
    return url
  }
  URL.revokeObjectURL = (url) => revoked.push(url)

  try {
    const { default: Canvas } = await import(pathToFileURL(visualEditorOut))
    await testCanvasDragRelease(Canvas, { createApp, h, nextTick, reactive })
    await testSequenceCanvasOperations(Canvas, { createApp, h, nextTick, reactive })
    await testCanvasRegressionOperations(Canvas, { createApp, h, nextTick, reactive })
    await testDialogVisualFlush(Dialog, { createApp, h, nextTick, reactive })
    await testActualMermaidDialog(Dialog, mocks, { createApp, h, nextTick, reactive })
    await testActualMermaidNodeView(NodeView, mocks, {
      createApp,
      h,
      nextTick,
      reactive,
      ...integration,
    })
    await testActualTiptapMermaidNodeView(ProductionNodeView, mocks, {
      createApp,
      h,
      nextTick,
      ...integration,
    })
  } finally {
    if (previousCreateObjectUrl) URL.createObjectURL = previousCreateObjectUrl
    else delete URL.createObjectURL
    if (previousRevokeObjectUrl) URL.revokeObjectURL = previousRevokeObjectUrl
    else delete URL.revokeObjectURL
    browserWindow.document.body.replaceChildren()
  }
  assert.ok(created.length > 0, 'product Mermaid components should create preview Blob URLs')
  assert.ok(revoked.length > 0, 'product Mermaid components should revoke preview Blob URLs')
}

async function testCanvasDragRelease(Canvas, { createApp, h, nextTick, reactive }) {
  const host = document.createElement('div')
  document.body.append(host)
  const props = reactive({ source: 'flowchart TD\n  A[Start]\n  B[End]', disabled: false })
  const app = createApp({ render: () => h(Canvas, props) })
  app.mount(host)
  try {
    const node = host.querySelector('.mermaid-visual-editor-node')
    const target = host.querySelectorAll('.mermaid-visual-editor-node')[1]
    for (const stop of ['release', 'cancel', 'blur', 'disabled', 'buttons']) {
      props.disabled = false
      await nextTick()
      dispatchPointer(node, 'pointerdown', { clientX: 100, clientY: 100, buttons: 1 })
      const before = node.style.left
      dispatchPointer(window, 'pointermove', { clientX: 120, clientY: 100, buttons: 1 })
      await nextTick()
      assert.notEqual(node.style.left, before, 'held pointer must move the node')
      if (stop === 'release') dispatchPointer(node, 'pointerup')
      if (stop === 'cancel') dispatchPointer(node, 'pointercancel')
      if (stop === 'blur') window.dispatchEvent(new Event('blur'))
      if (stop === 'disabled') props.disabled = true
      if (stop === 'buttons') dispatchPointer(window, 'pointermove', { clientX: 180, buttons: 0 })
      await nextTick()
      const stopped = node.style.left
      dispatchPointer(window, 'pointermove', { clientX: 200, clientY: 100, buttons: 1 })
      await nextTick()
      assert.equal(node.style.left, stopped, `${stop} must end node movement`)
    }
    props.disabled = false
    await nextTick()
    const point = node.querySelector('.mermaid-visual-editor-connection-point')
    dispatchPointer(point, 'pointerdown', { buttons: 1 })
    await nextTick()
    assert.ok(host.querySelector('.mermaid-visual-editor-connection-preview'))
    dispatchPointer(target, 'pointerup')
    await nextTick()
    assert.equal(host.querySelector('.mermaid-visual-editor-connection-preview'), null)
    assert.equal(host.querySelectorAll('.mermaid-visual-editor-edges g').length, 1, 'release on target must still create a connection')
    dispatchPointer(point, 'pointerdown', { buttons: 1 })
    dispatchPointer(node, 'pointerup')
    await nextTick()
    assert.equal(host.querySelector('.mermaid-visual-editor-connection-preview'), null, 'release on source must cancel connection')
    dispatchPointer(node, 'pointerdown', { buttons: 1 })
    const beforeUnmount = node.style.left
    app.unmount()
    dispatchPointer(window, 'pointermove', { clientX: 250, buttons: 1 })
    await nextTick()
    assert.equal(node.style.left, beforeUnmount)
  } finally {
    if (host.firstChild) app.unmount()
    host.remove()
  }
}

async function testSequenceCanvasOperations(Canvas, { createApp, h, nextTick, reactive }) {
  const host = document.createElement('div')
  document.body.append(host)
  const props = reactive({
    source: 'sequenceDiagram\n  participant A as 利用者\n  participant B as システム',
    disabled: false,
  })
  const updates = []
  const app = createApp({
    render: () => h(Canvas, {
      source: props.source,
      disabled: props.disabled,
      'onUpdate:source': (source) => {
        updates.push(source)
        props.source = source
      },
    }),
  })
  app.mount(host)
  try {
    await nextTick()
    assert.equal(host.querySelectorAll('.mermaid-visual-editor-node').length, 2,
      'sequence messages must not be rendered as participant cards')
    const connectButton = host.querySelector('button[aria-label="接続を作成"]')
    connectButton?.click()
    const nodes = host.querySelectorAll('.mermaid-visual-editor-node')
    nodes[0]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    nodes[1]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    await nextTick()
    assert.equal(host.querySelectorAll('.mermaid-visual-editor-node').length, 2,
      'connecting sequence participants must not add a third participant')
    assert.equal(host.querySelectorAll('.mermaid-visual-editor-edge').length, 1,
      'connecting sequence participants must add one message edge')
    assert.equal((updates.at(-1) ?? '').split('\n').filter((line) => line.trim().startsWith('participant ')).length, 2,
      'sequence connection source must retain exactly two participants')

    const propertyInput = [...host.querySelectorAll('input')].find((input) => input.value === 'A' || input.value === '利用者')
    const beforeEdges = host.querySelectorAll('.mermaid-visual-editor-edge').length
    propertyInput?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Backspace', bubbles: true }))
    await nextTick()
    assert.equal(host.querySelectorAll('.mermaid-visual-editor-edge').length, beforeEdges,
      'Backspace in a property input must not delete a canvas element')
  } finally {
    app.unmount()
    host.remove()
  }
}

async function testCanvasRegressionOperations(Canvas, { createApp, h, nextTick, reactive }) {
  async function mountCanvas(source) {
    const host = document.createElement('div')
    document.body.append(host)
    const props = reactive({ source, disabled: false })
    const updates = []
    const app = createApp({
      render: () => h(Canvas, {
        source: props.source,
        disabled: props.disabled,
        'onUpdate:source': (nextSource) => {
          updates.push(nextSource)
          props.source = nextSource
        },
      }),
    })
    app.mount(host)
    await nextTick()
    return { host, props, updates, app }
  }

  const orphaned = await mountCanvas('flowchart TD\n  A[開始] --> B[終了]')
  try {
    const firstNode = orphaned.host.querySelector('.mermaid-visual-editor-node')
    firstNode?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    orphaned.host.querySelector('.mermaid-visual-editor-property-actions .danger')?.click()
    await nextTick()
    assert.equal(orphaned.props.source, 'flowchart TD\n  B[終了]',
      'deleting a flow node must promote an orphaned derived endpoint into source')
  } finally {
    orphaned.app.unmount()
    orphaned.host.remove()
  }

  const deletedEdge = await mountCanvas('flowchart TD\n  A[開始] --> B[終了]')
  try {
    deletedEdge.host.querySelector('.mermaid-visual-editor-edge')?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    deletedEdge.host.querySelector('.mermaid-visual-editor-property-actions .danger')?.click()
    await nextTick()
    assert.doesNotMatch(deletedEdge.props.source, /-->/,
      'deleting only a flow connection must remove the edge from source')
    assert.match(deletedEdge.props.source, /A\[開始\]/)
    assert.match(deletedEdge.props.source, /B\[終了\]/)
  } finally {
    deletedEdge.app.unmount()
    deletedEdge.host.remove()
  }

  const connectedShapes = await mountCanvas('flowchart TD\n  A{判断}\n  B([終了])')
  try {
    connectedShapes.host.querySelector('button[aria-label="接続を作成"]')?.click()
    const nodes = connectedShapes.host.querySelectorAll('.mermaid-visual-editor-node')
    nodes[0]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    nodes[1]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    await nextTick()
    assert.match(connectedShapes.props.source, /A --> B/,
      'a new flow connection must reference existing node identifiers')
    assert.doesNotMatch(connectedShapes.props.source, /A\{判断\} --> B/,
      'a new flow connection must not redefine endpoint shapes')
  } finally {
    connectedShapes.app.unmount()
    connectedShapes.host.remove()
  }

  const collision = await mountCanvas('flowchart TD\n  A[開始]\n  B[終了]')
  try {
    collision.host.querySelector('.mermaid-visual-editor-node')?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    const identifier = [...collision.host.querySelectorAll('input')].find((input) => input.value === 'A')
    setInputValue(identifier, 'B', browserInputEvent())
    await nextTick()
    assert.equal(collision.props.source, 'flowchart TD\n  A[開始]\n  B[終了]',
      'identifier collision must reject the draft without changing source')
    assert.match(collision.host.querySelector('[role="alert"]')?.textContent ?? '', /同じ種類/)
  } finally {
    collision.app.unmount()
    collision.host.remove()
  }

  const sequenceDefaults = await mountCanvas('sequenceDiagram\n  participant P1 as 利用者\n  participant P2 as システム')
  try {
    sequenceDefaults.host.querySelector('button[aria-label="メッセージ"]')?.click()
    await nextTick()
    assert.equal(sequenceDefaults.props.source.split('\n').filter((line) => line.trim().startsWith('participant ')).length, 2,
      'adding a sequence message must not add implicit participants')
    assert.match(sequenceDefaults.props.source, /P1->>P2:/,
      'new sequence messages must use existing participants as defaults')
  } finally {
    sequenceDefaults.app.unmount()
    sequenceDefaults.host.remove()
  }

  const multiSelect = await mountCanvas('flowchart TD\n  A[開始]\n  B[終了]')
  try {
    const nodes = multiSelect.host.querySelectorAll('.mermaid-visual-editor-node')
    nodes[0]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    dispatchPointer(nodes[1], 'pointerdown', { ctrlKey: true, pointerId: 41 })
    dispatchPointer(nodes[1], 'pointerup', { pointerId: 41 })
    nodes[1]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, ctrlKey: true }))
    await nextTick()
    assert.equal(multiSelect.host.querySelectorAll('.mermaid-visual-editor-node.is-selected').length, 2,
      'Ctrl/Cmd-click must add a node once instead of toggling twice')
  } finally {
    multiSelect.app.unmount()
    multiSelect.host.remove()
  }

  const noteReferences = await mountCanvas('sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  Note over A,B: 共有')
  try {
    noteReferences.host.querySelector('.mermaid-visual-editor-node')?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    const identifier = [...noteReferences.host.querySelectorAll('input')].find((input) => input.value === 'A')
    setInputValue(identifier, 'C', browserInputEvent())
    await nextTick()
    assert.match(noteReferences.props.source, /participant C as 利用者/)
    assert.match(noteReferences.props.source, /Note over C,B: 共有/,
      'renaming a participant must update every target in a multi-reference Note')
  } finally {
    noteReferences.app.unmount()
    noteReferences.host.remove()
  }

  const blocks = await mountCanvas('sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  alt 条件\n    A->>B: 一つ目\n  else 別条件\n    B-->>A: 二つ目\n  end')
  try {
    assert.equal(blocks.host.querySelectorAll('.mermaid-visual-editor-sequence-block').length, 1,
      'sequence branches must render an enclosing frame')
    const control = [...blocks.host.querySelectorAll('.mermaid-visual-editor-node')].find((node) => node.textContent?.includes('条件'))
    control?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    blocks.host.querySelector('.mermaid-visual-editor-property-actions .danger')?.click()
    await nextTick()
    assert.doesNotMatch(blocks.props.source, /\b(?:alt|else|end)\b/,
      'deleting a sequence block must remove its start, members, and end together')
  } finally {
    blocks.app.unmount()
    blocks.host.remove()
  }

  const duplicatedBlock = await mountCanvas('sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  loop 反復\n    A->>B: 一つ目\n  end')
  try {
    const control = [...duplicatedBlock.host.querySelectorAll('.mermaid-visual-editor-node')].find((node) => node.textContent?.includes('反復'))
    control?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    duplicatedBlock.host.querySelector('button[aria-label="複製"]')?.click()
    await nextTick()
    assert.equal((duplicatedBlock.props.source.match(/^  loop /gm) ?? []).length, 2,
      'duplicating a sequence loop must duplicate its block unit')
    assert.equal((duplicatedBlock.props.source.match(/^  end$/gm) ?? []).length, 2,
      'duplicating a sequence loop must retain the matching end')
  } finally {
    duplicatedBlock.app.unmount()
    duplicatedBlock.host.remove()
  }

  const blockContent = await mountCanvas('sequenceDiagram\n  participant A as 利用者\n  participant B as システム')
  try {
    blockContent.host.querySelector('button[aria-label="分岐"]')?.click()
    blockContent.host.querySelector('button[aria-label="メッセージ"]')?.click()
    await nextTick()
    const source = blockContent.props.source
    assert.ok(source.indexOf('  alt ') < source.indexOf('  A->>B:') && source.indexOf('  A->>B:') < source.indexOf('  end'),
      'a message added after a block start must be inserted before its end')
  } finally {
    blockContent.app.unmount()
    blockContent.host.remove()
  }

  const history = await mountCanvas('sequenceDiagram\n  participant A as Alice\n  participant B as システム')
  try {
    history.host.querySelector('.mermaid-visual-editor-node')?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    const alias = [...history.host.querySelectorAll('input')].find((input) => input.value === 'Alice')
    setInputValue(alias, 'Changed', browserInputEvent())
    await nextTick()
    history.props.source = `${history.props.source}\n  participant C as 追加`
    await nextTick()
    history.host.querySelector('button[aria-label="元に戻す"]')?.click()
    await nextTick()
    assert.match(history.props.source, /participant A as Changed/)
    assert.doesNotMatch(history.props.source, /participant C as 追加/,
      'undo after a source edit must preserve the preceding GUI edit')
    history.host.querySelector('button[aria-label="やり直す"]')?.click()
    await nextTick()
    assert.match(history.props.source, /participant C as 追加/,
      'redo must restore the external source edit')
    history.props.source = 'これはMermaidではありません'
    await nextTick()
    assert.ok(history.host.querySelector('.mermaid-visual-editor-fallback'),
      'an invalid source must keep the source editor available')
    history.host.querySelector('button[aria-label="元に戻す"]')?.click()
    await nextTick()
    assert.match(history.props.source, /participant C as 追加/,
      'undo must recover from an invalid source')
    history.host.querySelector('button[aria-label="やり直す"]')?.click()
    await nextTick()
    assert.equal(history.props.source, 'これはMermaidではありません',
      'redo must restore the invalid source state without losing its history')
  } finally {
    history.app.unmount()
    history.host.remove()
  }

  const messageOrder = await mountCanvas('sequenceDiagram\n  participant A as 利用者\n  participant B as システム\n  A->>B: 一つ目\n  B-->>A: 二つ目')
  try {
    const edges = messageOrder.host.querySelectorAll('.mermaid-visual-editor-edge')
    edges[0]?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    await nextTick()
    const messageInput = [...messageOrder.host.querySelectorAll('input')].find((input) => input.value === '一つ目')
    setInputValue(messageInput, '編集済み', browserInputEvent())
    await nextTick()
    assert.match(messageOrder.props.source, /編集済み/,
      'selecting a sequence message must expose direct property editing')
    dispatchPointer(edges[1], 'pointerdown', { clientY: 180, pointerId: 51 })
    dispatchPointer(window, 'pointermove', { clientY: 90, buttons: 1, pointerId: 51 })
    dispatchPointer(window, 'pointerup', { clientY: 90, pointerId: 51 })
    await nextTick()
    assert.ok(messageOrder.props.source.indexOf('B-->>A: 二つ目') < messageOrder.props.source.indexOf('A->>B: 編集済み'),
      'dragging a sequence message must reorder the source lines')
  } finally {
    messageOrder.app.unmount()
    messageOrder.host.remove()
  }

  const branchOrder = await mountCanvas('sequenceDiagram\n  participant A\n  participant B\n  alt success\n    A->>B: successOnly\n  else failure\n    A->>B: failureOnly\n  end')
  try {
    const branchSource = branchOrder.props.source
    const edge = branchOrder.host.querySelector('.mermaid-visual-editor-edge')
    dispatchPointer(edge, 'pointerdown', { clientY: 100, pointerId: 61 })
    dispatchPointer(window, 'pointermove', { clientY: 158, buttons: 1, pointerId: 61 })
    dispatchPointer(window, 'pointerup', { clientY: 158, pointerId: 61 })
    await nextTick()
    assert.equal(branchOrder.props.source, branchSource,
      'dragging a message must not cross an else branch boundary')
  } finally {
    branchOrder.app.unmount()
    branchOrder.host.remove()
  }

  const repeatedDrag = await mountCanvas('sequenceDiagram\n  participant A\n  participant B\n  A->>B: first\n  A->>B: second\n  A->>B: third\n  A->>B: fourth')
  try {
    const edges = repeatedDrag.host.querySelectorAll('.mermaid-visual-editor-edge')
    dispatchPointer(edges[0], 'pointerdown', { clientY: 100, pointerId: 62 })
    dispatchPointer(window, 'pointermove', { clientY: 158, buttons: 1, pointerId: 62 })
    await nextTick()
    dispatchPointer(window, 'pointermove', { clientY: 159, buttons: 1, pointerId: 62 })
    await nextTick()
    dispatchPointer(window, 'pointerup', { clientY: 159, pointerId: 62 })
    await nextTick()
    assert.deepEqual(sequenceMessageLabels(repeatedDrag.props.source), ['second', 'first', 'third', 'fourth'],
      'one row plus a small additional pointermove must move the message exactly one row')
    repeatedDrag.host.querySelector('button[aria-label="元に戻す"]')?.click()
    await nextTick()
    assert.deepEqual(sequenceMessageLabels(repeatedDrag.props.source), ['first', 'second', 'third', 'fourth'],
      'one message drag must be one undo operation')
    repeatedDrag.host.querySelector('button[aria-label="やり直す"]')?.click()
    await nextTick()
    assert.deepEqual(sequenceMessageLabels(repeatedDrag.props.source), ['second', 'first', 'third', 'fourth'],
      'redo must restore the completed one-row message drag')
  } finally {
    repeatedDrag.app.unmount()
    repeatedDrag.host.remove()
  }

  const zoomedDrag = await mountCanvas('sequenceDiagram\n  participant A\n  participant B\n  A->>B: first\n  A->>B: second\n  A->>B: third\n  A->>B: fourth')
  try {
    for (let index = 0; index < 5; index += 1) zoomedDrag.host.querySelector('button[aria-label="拡大"]')?.click()
    await nextTick()
    const edge = zoomedDrag.host.querySelector('.mermaid-visual-editor-edge')
    dispatchPointer(edge, 'pointerdown', { clientY: 100, pointerId: 63 })
    dispatchPointer(window, 'pointermove', { clientY: 187, buttons: 1, pointerId: 63 })
    dispatchPointer(window, 'pointerup', { clientY: 187, pointerId: 63 })
    await nextTick()
    assert.deepEqual(sequenceMessageLabels(zoomedDrag.props.source), ['second', 'first', 'third', 'fourth'],
      'message drag distance must be converted from zoomed screen coordinates')
  } finally {
    zoomedDrag.app.unmount()
    zoomedDrag.host.remove()
  }

  const rawBlockSource = 'sequenceDiagram\n  participant A\n  participant B\n  loop repeat\n    activate B\n    alt ready\n      A->>B: request\n    else skip\n      A-->>B: skipped\n    end\n    deactivate B\n  end'
  const duplicatedRawBlock = await mountCanvas(rawBlockSource)
  try {
    const control = [...duplicatedRawBlock.host.querySelectorAll('.mermaid-visual-editor-node')]
      .find((node) => node.textContent?.includes('repeat'))
    control?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    duplicatedRawBlock.host.querySelector('button[aria-label="複製"]')?.click()
    await nextTick()
    assert.equal((duplicatedRawBlock.props.source.match(/^    activate B$/gm) ?? []).length, 2,
      'duplicating a block must retain unsupported activate lines')
    assert.equal((duplicatedRawBlock.props.source.match(/^    deactivate B$/gm) ?? []).length, 2,
      'duplicating a block must retain unsupported deactivate lines')
    assert.equal((duplicatedRawBlock.props.source.match(/^[ \t]+alt ready$/gm) ?? []).length, 2,
      'duplicating a block must retain nested block controls')
  } finally {
    duplicatedRawBlock.app.unmount()
    duplicatedRawBlock.host.remove()
  }

  const copiedRawBlock = await mountCanvas(rawBlockSource)
  try {
    const control = [...copiedRawBlock.host.querySelectorAll('.mermaid-visual-editor-node')]
      .find((node) => node.textContent?.includes('repeat'))
    control?.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true }))
    const editor = copiedRawBlock.host.querySelector('.mermaid-visual-editor')
    editor?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'c', ctrlKey: true, bubbles: true }))
    editor?.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'v', ctrlKey: true, bubbles: true }))
    await nextTick()
    assert.equal((copiedRawBlock.props.source.match(/^    activate B$/gm) ?? []).length, 2,
      'copy and paste of a block must retain unsupported activate lines')
    assert.equal((copiedRawBlock.props.source.match(/^    deactivate B$/gm) ?? []).length, 2,
      'copy and paste of a block must retain unsupported deactivate lines')
  } finally {
    copiedRawBlock.app.unmount()
    copiedRawBlock.host.remove()
  }
}

async function testDialogVisualFlush(Dialog, { createApp, h, nextTick, reactive }) {
  const source = 'sequenceDiagram\n  participant A as 利用者\n  participant B as システム'
  const props = reactive({ open: false, source, theme: 'light' })
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({
    render: () => h(Dialog, {
      open: props.open,
      source: props.source,
      theme: props.theme,
      'onUpdate:open': (open) => { props.open = open },
      'onUpdate:source': (nextSource) => { props.source = nextSource },
    }),
  })
  app.mount(host)
  try {
    props.open = true
    await nextTick()
    host.querySelector('.mermaid-edit-dialog-source-toggle')?.click()
    await nextTick()
    const firstNode = host.querySelector('.mermaid-visual-editor-node')
    firstNode?.dispatchEvent(new dom.window.MouseEvent('dblclick', { bubbles: true }))
    await nextTick()
    const inlineInput = host.querySelector('.mermaid-visual-editor-inline-input')
    assert.ok(inlineInput, 'sequence participant must support visual inline editing')
    inlineInput.dispatchEvent(new Event('compositionstart', { bubbles: true }))
    inlineInput.value = 'Renamed'
    host.querySelector('.mermaid-edit-dialog-actions button:first-child')?.click()
    await nextTick()
    assert.match(props.source, /participant A as Renamed/,
      'flushing visual IME input with the source panel open must keep the visual rename')
    assert.doesNotMatch(props.source, /participant A as 利用者/,
      'a stale source textarea DOM value must not overwrite a newer visual rename')
  } finally {
    app.unmount()
    host.remove()
  }
}

async function testActualMermaidDialog(Dialog, mocks, vue) {
  const { createApp, h, nextTick, reactive } = vue
  const source = 'flowchart TD\n  A --> B'
  const props = reactive({ open: false, source, theme: 'light' })
  const updates = []
  let saves = 0
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({
    render: () => h(Dialog, {
      open: props.open,
      source: props.source,
      theme: props.theme,
      'onUpdate:open': (open) => {
        props.open = open
      },
      'onUpdate:source': (nextSource) => {
        updates.push(nextSource)
        props.source = nextSource
      },
      onSave: () => {
        saves += 1
      },
    }),
  })
  app.mount(host)

  try {
    props.open = true
    await nextTick()
    const initialRender = await waitForRender(mocks, source)
    mocks.resolveRender(initialRender, { ok: true, svg: '<svg/>', altText: 'initial' })
    await waitFor(() => host.querySelector('.mermaid-edit-dialog-render-preview') !== null)
    assert.match(host.querySelector('.mermaid-edit-dialog-render-status')?.textContent ?? '', /構文を確認/)
    host.querySelector('.mermaid-edit-dialog-source-toggle')?.click()
    await nextTick()
    const textarea = () => host.querySelector('textarea')
    assert.equal(textarea()?.value, source, 'the actual dialog opens with the node source')
    const externalSource = 'flowchart TD\n  A --> external'
    const updatesBeforeExternalSource = updates.length
    props.source = externalSource
    await nextTick()
    assert.equal(textarea()?.value, externalSource, 'external node updates must replace the dialog source')
    assert.equal(updates.length, updatesBeforeExternalSource, 'external source sync must not emit the stale textarea value')
    const externalRender = await waitForRender(mocks, externalSource)
    mocks.resolveRender(externalRender, { ok: true, svg: '<svg/>', altText: 'external' })
    const edited = 'flowchart LR\n  A --> C'
    setInputValue(textarea(), edited, browserInputEvent())
    await nextTick()
    assert.equal(props.source, edited, 'dialog input must immediately update its source model')
    assert.equal(updates.at(-1), edited)

    const composition = ' \nflowchart TD\n  A --> D\n '
    const updateCountBeforeComposition = updates.length
    textarea().dispatchEvent(new Event('compositionstart', { bubbles: true }))
    setInputValue(textarea(), composition, browserInputEvent())
    await nextTick()
    assert.equal(updates.length, updateCountBeforeComposition, 'IME input must wait for compositionend')
    textarea().dispatchEvent(new Event('compositionend', { bubbles: true }))
    await nextTick()
    assert.equal(props.source, composition, 'compositionend must commit the final source')
    props.theme = 'dark'
    await nextTick()
    const darkRender = await waitForRender(mocks, composition, 'dark')
    assert.equal(mocks.pendingRenders[darkRender].options.theme, 'dark', 'theme changes must rerender the latest source')
    mocks.resolveRender(darkRender, { ok: true, svg: '<svg/>', altText: 'dark' })

    host.querySelector('.mermaid-edit-dialog-actions button:first-child')?.click()
    await nextTick()
    assert.equal(props.open, false, 'close must close the actual dialog')
    assert.equal(props.source, composition, 'close must preserve the latest source')
    assert.equal(saves, 0, 'close must not be treated as a save button click')

    props.open = true
    await nextTick()
    host.querySelector('.mermaid-edit-dialog-source-toggle')?.click()
    await nextTick()
    assert.equal(textarea()?.value, composition, 'reopening must use the committed source')
    host.querySelector('.mermaid-edit-dialog-actions button:last-child')?.click()
    await nextTick()
    assert.equal(props.open, false)
    assert.equal(saves, 1, 'save must close after the source is already committed')
  } finally {
    app.unmount()
    host.remove()
  }
}

async function testActualMermaidNodeView(NodeView, mocks, vue) {
  const {
    createApp,
    h,
    nextTick,
    reactive,
    flushMermaidEditorInputs,
    setMermaidEditorInputsLocked,
    createContentLockBeforeLock,
  } = vue
  let currentNode = createFakeMermaidNode('flowchart TD\n  A --> B')
  let dispatchCount = 0
  const editor = {
    isDestroyed: false,
    isEditable: true,
    setEditable(editable) {
      this.isEditable = editable
    },
    state: {
      doc: { nodeAt: () => currentNode },
      tr: null,
    },
    schema: {
      text(value) {
        return { textContent: value }
      },
    },
    view: {
      dispatch(transaction) {
        currentNode = createFakeMermaidNode(transaction.nextSource)
        props.node = currentNode
        dispatchCount += 1
      },
    },
  }
  editor.state.tr = {
    replaceWith(_from, _to, content) {
      return { nextSource: content.textContent ?? '' }
    },
  }
  const mermaidEditorFlushers = new Set()
  const mermaidEditorInputLockers = new Set()
  const props = reactive({
    node: currentNode,
    editor,
    extension: {
      storage: {
        noteId: 'product-note',
        generation: 1,
        mermaidEditorFlushers,
        mermaidEditorInputLockers,
      },
    },
    getPos: () => 0,
  })
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(NodeView, props) })
  app.mount(host)

  try {
    const openDiagramEditor = async () => {
      await waitFor(() => host.querySelector('.mermaid-code-block-preview') !== null)
      host.querySelector('.mermaid-code-block-icon-button')?.click()
      await nextTick()
      host.querySelector('.mermaid-edit-dialog-source-toggle')?.click()
      await nextTick()
      assert.equal(host.querySelectorAll('textarea').length, 1, 'the edit button must open one edit dialog')
    }
    const initialRender = await waitForRender(mocks, currentNode.textContent)
    mocks.resolveRender(initialRender, { ok: true, svg: '<svg/>', altText: 'node' })
    await waitFor(() => host.querySelector('.mermaid-code-block-image') !== null)
    await nextTick()
    const hiddenSource = host.querySelector('.mermaid-code-block-source')
    assert.equal(hiddenSource?.getAttribute('contenteditable'), 'false')
    assert.equal(hiddenSource?.getAttribute('tabindex'), '-1')
    assert.equal(hiddenSource?.getAttribute('aria-hidden'), 'true')

    const zoomLabel = () => Number.parseInt(host.querySelector('.mermaid-code-block-zoom')?.textContent ?? '', 10)
    const image = host.querySelector('.mermaid-code-block-image')
    const handle = host.querySelector('.visual-resize-handle')
    assert.ok(image)
    assert.ok(handle)
    mockImageLayout(image, 100)
    assert.match(handle.getAttribute('title') ?? '', /上下/)
    const initialZoom = zoomLabel()
    dispatchPointer(handle, 'pointerdown', { clientX: 100, clientY: 100, pointerId: 21 })
    dispatchPointer(handle, 'pointermove', { clientX: 900, clientY: 100, pointerId: 21 })
    await nextTick()
    assert.equal(zoomLabel(), initialZoom, 'horizontal movement must not change NodeView Mermaid zoom')
    dispatchPointer(handle, 'pointermove', { clientX: 100, clientY: 140, pointerId: 21 })
    await nextTick()
    assert.ok(zoomLabel() > initialZoom, 'NodeView downward drag must increase Mermaid zoom')
    dispatchPointer(handle, 'pointerup', { clientY: 140, pointerId: 21 })
    const zoomBeforeUpwardDrag = zoomLabel()
    dispatchPointer(handle, 'pointerdown', { clientY: 100, pointerId: 22 })
    dispatchPointer(handle, 'pointermove', { clientX: 900, clientY: 50, pointerId: 22 })
    await nextTick()
    assert.ok(zoomLabel() < zoomBeforeUpwardDrag, 'NodeView upward drag must decrease Mermaid zoom')
    dispatchPointer(handle, 'pointercancel', { clientY: 50, pointerId: 22 })
    dispatchPointer(handle, 'pointerdown', { clientY: 100, pointerId: 23 })
    dispatchPointer(handle, 'pointermove', { clientY: -10000, pointerId: 23 })
    await nextTick()
    assert.equal(zoomLabel(), 10, 'NodeView Mermaid zoom must clamp at 10%')
    dispatchPointer(handle, 'pointerup', { clientY: -10000, pointerId: 23 })
    dispatchPointer(handle, 'pointerdown', { clientY: 100, pointerId: 24 })
    dispatchPointer(handle, 'pointermove', { clientY: 10000, pointerId: 24 })
    await nextTick()
    assert.equal(zoomLabel(), 200, 'NodeView Mermaid zoom must clamp at 200%')
    dispatchPointer(handle, 'pointerup', { clientY: 10000, pointerId: 24 })

    await openDiagramEditor()
    host.querySelector('.mermaid-edit-dialog-actions button').click()
    await nextTick()
    await openDiagramEditor()
    await nextTick()
    const textarea = () => host.querySelector('textarea')
    assert.equal(textarea()?.value, currentNode.textContent)

    const firstEdit = 'flowchart LR\n  A --> C'
    setInputValue(textarea(), firstEdit, browserInputEvent())
    await nextTick()
    assert.equal(currentNode.textContent, firstEdit, 'actual NodeView handler must dispatch source edits')
    assert.equal(dispatchCount, 1)

    const secondEdit = 'flowchart LR\n  A --> D'
    setInputValue(textarea(), secondEdit, browserInputEvent())
    await nextTick()
    assert.equal(currentNode.textContent, secondEdit, 'the edit session must survive its own node update')
    assert.equal(dispatchCount, 2)

    host.querySelector('.mermaid-edit-dialog-actions button').click()
    await nextTick()
    assert.equal(host.querySelector('textarea'), null, 'the actual dialog close button must close the session')
    assert.equal(currentNode.textContent, secondEdit)

    await openDiagramEditor()
    await nextTick()
    assert.equal(host.querySelector('textarea')?.value, secondEdit, 'reopening must retain NodeView edits')
    host.querySelector('.mermaid-edit-dialog-actions button').click()
    await nextTick()
    assert.equal(host.querySelector('textarea'), null)

    props.editor.isEditable = true
    mocks.deletionPreparing.delete('product-note')
    await openDiagramEditor()
    await nextTick()
    const composition = 'flowchart TD\n  A --> IME'
    const editingTextarea = host.querySelector('textarea')
    editingTextarea.dispatchEvent(new Event('compositionstart', { bubbles: true }))
    setInputValue(editingTextarea, composition, browserInputEvent())
    await nextTick()

    const beforeLockEvents = []
    let persistedSource = ''
    let releaseDraftFlush
    let draftFlushStarted = false
    const draftFlush = new Promise((resolve) => { releaseDraftFlush = resolve })
    const beforeLock = createContentLockBeforeLock(
      () => ({
        flushEditorInput: () => {
          beforeLockEvents.push('editor-input')
          return flushMermaidEditorInputs({ mermaidEditorFlushers })
        },
        setContentLockPending: (pending) => {
          const locked = setMermaidEditorInputsLocked({ mermaidEditorInputLockers }, pending)
          if (!locked) return false
          editor.setEditable(!pending)
          return true
        },
      }),
      async () => {
        beforeLockEvents.push('draft-flush')
        draftFlushStarted = true
        await draftFlush
        persistedSource = currentNode.textContent
        return true
      },
    )
    const beforeLockPromise = beforeLock()
    await waitFor(() => draftFlushStarted)
    await nextTick()
    assert.equal(editingTextarea.hasAttribute('readonly'), true,
      'the actual Mermaid dialog must block input while the draft flush is pending')
    const additionalComposition = `${composition}\n  C --> E`
    setInputValue(editingTextarea, additionalComposition, browserInputEvent())
    await nextTick()
    assert.equal(currentNode.textContent, composition,
      'input arriving after the initial flush must not change the protected document')
    assert.equal(persistedSource, '', 'the held draft flush must not complete before release')
    releaseDraftFlush()
    const preparation = await beforeLockPromise
    assert.equal(preparation.ok, true, 'before-lock must return a lock preparation after saving')
    assert.deepEqual(beforeLockEvents, ['editor-input', 'draft-flush'])
    assert.equal(persistedSource, composition, 'the draft flush must observe the IME-composed Mermaid source')
    assert.equal(editingTextarea.hasAttribute('readonly'), true,
      'the lock preparation must remain protected until the lock result is reflected')
    const compositionEnd = new Event('compositionend', { bubbles: true })
    editingTextarea.dispatchEvent(compositionEnd)
    assert.equal(mermaidEditorInputLockers.size, 1)
    preparation.finish()
    await nextTick()
    assert.equal(editingTextarea.hasAttribute('readonly'), false,
      'a successful lock refresh must release the editor input guard')
    assert.equal(editor.isEditable, true)

    const failedBeforeLock = createContentLockBeforeLock(
      () => ({
        flushEditorInput: () => flushMermaidEditorInputs({ mermaidEditorFlushers }),
        setContentLockPending: (pending) => {
          const locked = setMermaidEditorInputsLocked({ mermaidEditorInputLockers }, pending)
          if (!locked) return false
          editor.setEditable(!pending)
          return true
        },
      }),
      async () => false,
    )
    assert.equal(await failedBeforeLock(), false, 'a failed draft flush must defer the lock')
    await nextTick()
    assert.equal(editingTextarea.hasAttribute('readonly'), false,
      'a failed draft flush must restore editor input')
    assert.equal(editor.isEditable, true)

    mocks.deletionPreparing.add('product-note')
    props.editor.isEditable = false
    await nextTick()
    assert.equal(host.querySelector('textarea'), null, 'read-only transition must close the dialog')
    assert.equal(currentNode.textContent, composition, 'read-only transition must preserve IME input')
    mocks.deletionPreparing.delete('product-note')
  } finally {
    app.unmount()
    host.remove()
  }
}

function createFakeMermaidNode(textContent) {
  return {
    attrs: { language: 'mermaid' },
    textContent,
    nodeSize: textContent.length + 2,
    type: { name: 'codeBlock' },
    eq(other) {
      return other?.type?.name === 'codeBlock'
        && String(other.attrs?.language ?? '').toLowerCase() === 'mermaid'
        && other.textContent === textContent
    },
  }
}

async function testActualTiptapMermaidNodeView(NodeView, mocks, vue) {
  const {
    createApp,
    h,
    nextTick,
    VueEditor,
    EditorContent,
    StarterKit,
    CodeBlockLowlight,
    VueNodeViewRenderer,
    common,
    createLowlight,
  } = vue
  const source = 'flowchart TD\n  A --> B'
  const mermaidCodeBlock = CodeBlockLowlight.extend({
    addStorage() {
      return {
        ...this.parent?.(),
        noteId: 'product-note',
        generation: 1,
        mermaidEditorFlushers: new Set(),
        mermaidEditorInputLockers: new Set(),
      }
    },
    addNodeView() {
      return VueNodeViewRenderer(NodeView)
    },
  })
  const editor = new VueEditor({
    extensions: [
      StarterKit.configure({ codeBlock: false, undoRedo: false }),
      mermaidCodeBlock.configure({ lowlight: createLowlight(common) }),
    ],
    content: {
      type: 'doc',
      content: [
        {
          type: 'codeBlock',
          attrs: { language: 'mermaid' },
          content: [{ type: 'text', text: source }],
        },
        { type: 'paragraph' },
      ],
    },
  })
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(EditorContent, { editor }) })
  app.mount(host)

  try {
    await waitFor(() => host.querySelector('.mermaid-code-block-preview') !== null)
    const initialRender = await waitForRender(mocks, source)
    mocks.resolveRender(initialRender, { ok: true, svg: '<svg/>', altText: 'integrated node' })
    await waitFor(() => host.querySelector('.mermaid-code-block-image') !== null)
    await nextTick()

    const preview = () => host.querySelector('.mermaid-code-block-preview')
    const closeDialog = async () => {
      host.querySelector('.mermaid-edit-dialog-actions button')?.click()
      await nextTick()
    }

    host.querySelector('.mermaid-code-block-icon-button')?.click()
    await nextTick()
    host.querySelector('.mermaid-edit-dialog-source-toggle')?.click()
    await nextTick()
    assert.equal(host.querySelectorAll('textarea').length, 1,
      'the actual Vue Editor NodeView must open editing from the edit button')
    await closeDialog()

    assert.equal(editor.state.doc.firstChild.textContent, source,
      'opening the edit dialog must not change the document')
  } finally {
    app.unmount()
    editor.destroy()
    host.remove()
  }
}

function setInputValue(element, value, event) {
  if (!element) throw new Error('textarea was not rendered')
  element.value = value
  element.dispatchEvent(event)
}

function browserInputEvent() {
  return new Event('input', { bubbles: true })
}

async function waitForRender(mocks, source, theme) {
  const firstNewIndex = mocks.pendingRenders.length
  const matches = (pending, index) => index >= firstNewIndex
    && pending.source === source
    && (theme === undefined || pending.options.theme === theme)
  await waitFor(() => mocks.pendingRenders.some((pending, index) => matches(pending, index)))
  return mocks.pendingRenders.findLastIndex((pending, index) => matches(pending, index))
}

async function waitFor(predicate) {
  const deadline = Date.now() + 1500
  while (Date.now() < deadline) {
    if (predicate()) return
    await new Promise((resolve) => setTimeout(resolve, 10))
  }
  throw new Error('condition was not reached')
}

function createEditor(Editor, StarterKit, CodeBlockLowlight, createLowlight, common, Markdown) {
  return new Editor({
    extensions: [
      StarterKit.configure({ codeBlock: false, undoRedo: false }),
      Markdown.configure({ html: false, linkify: true }),
      CodeBlockLowlight.configure({ lowlight: createLowlight(common) }),
    ],
    content: { type: 'doc', content: [{ type: 'paragraph' }] },
  })
}

function createMarkdownDocument(editor, markdown, ProseMirrorDOMParser) {
  const html = editor.storage.markdown.parser.parse(markdown)
  const container = document.createElement('div')
  container.innerHTML = html
  restoreEmptyParagraphs(container)
  return ProseMirrorDOMParser.fromSchema(editor.schema).parse(container)
}

function clipboardData(entries) {
  return {
    types: Object.keys(entries),
    getData(type) {
      return entries[type] ?? ''
    },
  }
}

function dispatchPointer(target, type, options = {}) {
  const event = new dom.window.MouseEvent(type, {
    bubbles: true,
    cancelable: true,
    button: 0,
    clientX: 0,
    clientY: 0,
    ...options,
  })
  Object.defineProperty(event, 'pointerId', { value: options.pointerId ?? 1 })
  target.dispatchEvent(event)
  return event
}

function sequenceMessageLabels(source) {
  return source.split('\n')
    .filter((line) => line.includes(':') && !/^\s*(?:participant|actor|Note)\b/.test(line))
    .map((line) => line.slice(line.indexOf(':') + 1).trim())
}

function mockImageLayout(image, height) {
  image.getBoundingClientRect = () => ({
    x: 0,
    y: 0,
    top: 0,
    left: 0,
    right: 200,
    bottom: height,
    width: 200,
    height,
    toJSON() {
      return this
    },
  })
}

function findCodeBlockPosition(editor) {
  let position = -1
  editor.state.doc.descendants((node, nodePosition) => {
    if (position === -1 && node.type.name === 'codeBlock') position = nodePosition
  })
  assert.notEqual(position, -1)
  return position
}

function findMermaidCodeBlockPosition(editor) {
  let position = -1
  editor.state.doc.descendants((node, nodePosition) => {
    if (
      node.type.name === 'codeBlock'
      && String(node.attrs.language ?? '').toLowerCase() === 'mermaid'
    ) position = nodePosition
  })
  assert.notEqual(position, -1)
  return position
}

function pathToFileUrl(filePath) {
  return `file:///${filePath.replace(/\\/g, '/')}`
}
