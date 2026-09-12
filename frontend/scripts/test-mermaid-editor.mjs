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
const sessionOut = path.join(outDir, 'mermaidEditorSession.mjs')
const beforeLockOut = path.join(outDir, 'contentLockBeforeLock.mjs')
const pasteOut = path.join(outDir, 'mermaidPaste.mjs')
const dialogOut = path.join(outDir, 'MermaidEditDialog.mjs')
const nodeViewOut = path.join(outDir, 'MermaidCodeBlockView.mjs')
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
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'tiptapMarkdownSerializer.ts'), serializerOut)
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'mermaidEditorSession.ts'), sessionOut)
  await compileTypeScript(path.join(rootDir, 'src', 'utils', 'contentLockBeforeLock.ts'), beforeLockOut)
  await compileTypeScript(
    path.join(rootDir, 'src', 'utils', 'mermaidPaste.ts'),
    pasteOut,
    [["from './mermaidClipboard'", "from './mermaidClipboard.mjs'"]],
  )
  await compileVueComponent(
    path.join(rootDir, 'src', 'components', 'MermaidEditDialog.vue'),
    dialogOut,
    [
      ["from 'reka-ui'", "from './component-mocks.mjs'"],
      ["from '../utils/mermaidRenderer'", "from './component-mocks.mjs'"],
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
  await writeFile(path.join(outDir, 'component-mocks.mjs'), componentMocksSource(), 'utf8')

  const { createMermaidFence, readMermaidClipboardPayload } = await import(pathToFileURL(clipboardOut))
  const { serializeTiptapJsonToMarkdown } = await import(pathToFileURL(serializerOut))
  const { flushMermaidEditorInputs, setMermaidEditorInputsLocked } = await import(pathToFileURL(sessionOut))
  const { createContentLockBeforeLock } = await import(pathToFileURL(beforeLockOut))
  const { handleMermaidPaste } = await import(pathToFileURL(pasteOut))
  const Dialog = (await import(pathToFileURL(dialogOut))).default
  const NodeView = (await import(pathToFileURL(nodeViewOut))).default
  const componentMocks = await import(pathToFileURL(path.join(outDir, 'component-mocks.mjs')))
  const { Editor } = await import('@tiptap/core')
  const StarterKit = (await import('@tiptap/starter-kit')).default
  const { CodeBlockLowlight } = await import('@tiptap/extension-code-block-lowlight')
  const { common, createLowlight } = await import('lowlight')
  const { Markdown } = await import('tiptap-markdown')
  const { DOMParser: ProseMirrorDOMParser } = await import('@tiptap/pm/model')
  const { NodeSelection } = await import('@tiptap/pm/state')
  const { history, undo } = await import('@tiptap/pm/history')

  const editor = createEditor(Editor, StarterKit, CodeBlockLowlight, createLowlight, common, Markdown)

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
    componentMocks,
    dom.window,
    { flushMermaidEditorInputs, setMermaidEditorInputsLocked, createContentLockBeforeLock },
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

export const nodeViewProps = { node: Object, editor: Object, extension: Object, getPos: Function }
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

async function testProductMermaidComponents(Dialog, NodeView, mocks, browserWindow, integration) {
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
    await testActualMermaidDialog(Dialog, mocks, { createApp, h, nextTick, reactive })
    await testActualMermaidNodeView(NodeView, mocks, {
      createApp,
      h,
      nextTick,
      reactive,
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
    const textarea = () => host.querySelector('textarea')
    assert.equal(textarea()?.value, source, 'the actual dialog opens with the node source')
    const initialRender = await waitForRender(mocks, source)
    mocks.resolveRender(initialRender, { ok: true, svg: '<svg/>', altText: 'initial' })
    await nextTick()

    const externalSource = 'flowchart TD\n  A --> external'
    const updatesBeforeExternalSource = updates.length
    props.source = externalSource
    await nextTick()
    assert.equal(textarea()?.value, externalSource, 'external node updates must replace the dialog source')
    assert.equal(updates.length, updatesBeforeExternalSource, 'external source sync must not emit the stale textarea value')
    const externalRender = await waitForRender(mocks, externalSource)
    mocks.resolveRender(externalRender, { ok: true, svg: '<svg/>', altText: 'external' })
    await nextTick()

    const edited = 'flowchart LR\n  A --> C'
    setInputValue(textarea(), edited, browserInputEvent())
    await nextTick()
    assert.equal(props.source, edited, 'dialog input must immediately update its source model')
    assert.equal(updates.at(-1), edited)

    props.theme = 'dark'
    const darkRender = await waitForRender(mocks, edited, 'dark')
    assert.equal(mocks.pendingRenders[darkRender].source, edited, 'theme changes must not restore old source')
    assert.equal(mocks.pendingRenders[darkRender].options.theme, 'dark')
    mocks.resolveRender(darkRender, { ok: true, svg: '<svg/>', altText: 'dark' })

    const composition = ' \nflowchart TD\n  A --> D\n '
    const updateCountBeforeComposition = updates.length
    textarea().dispatchEvent(new Event('compositionstart', { bubbles: true }))
    setInputValue(textarea(), composition, browserInputEvent())
    await nextTick()
    assert.equal(updates.length, updateCountBeforeComposition, 'IME input must wait for compositionend')
    textarea().dispatchEvent(new Event('compositionend', { bubbles: true }))
    await nextTick()
    assert.equal(props.source, composition, 'compositionend must commit the final source')

    host.querySelectorAll('button')[0].click()
    await nextTick()
    assert.equal(props.open, false, 'close must close the actual dialog')
    assert.equal(props.source, composition, 'close must preserve the latest source')
    assert.equal(saves, 0, 'close must not be treated as a save button click')

    props.open = true
    await nextTick()
    assert.equal(textarea()?.value, composition, 'reopening must use the committed source')
    host.querySelectorAll('button')[1].click()
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
    const initialRender = await waitForRender(mocks, currentNode.textContent)
    mocks.resolveRender(initialRender, { ok: true, svg: '<svg/>', altText: 'node' })
    await nextTick()
    const hiddenSource = host.querySelector('.mermaid-code-block-source')
    assert.equal(hiddenSource?.getAttribute('contenteditable'), 'false')
    assert.equal(hiddenSource?.getAttribute('tabindex'), '-1')
    assert.equal(hiddenSource?.getAttribute('aria-hidden'), 'true')

    host.querySelector('.mermaid-code-block-edit').click()
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

    host.querySelectorAll('button').item(1).click()
    await nextTick()
    assert.equal(host.querySelector('textarea'), null, 'the actual dialog close button must close the session')
    assert.equal(currentNode.textContent, secondEdit)

    host.querySelector('.mermaid-code-block-edit').click()
    await nextTick()
    assert.equal(host.querySelector('textarea')?.value, secondEdit, 'reopening must retain NodeView edits')
    host.querySelectorAll('button').item(2).click()
    await nextTick()
    assert.equal(host.querySelector('textarea'), null)

    props.editor.isEditable = true
    mocks.deletionPreparing.delete('product-note')
    host.querySelector('.mermaid-code-block-edit').click()
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

function setInputValue(element, value, event) {
  if (!element) throw new Error('textarea was not rendered')
  element.value = value
  element.dispatchEvent(event)
}

function browserInputEvent() {
  return new Event('input', { bubbles: true })
}

async function waitForRender(mocks, source, theme) {
  await waitFor(() => mocks.pendingRenders.some((pending) =>
    pending.source === source && (theme === undefined || pending.options.theme === theme),
  ))
  return mocks.pendingRenders.findIndex((pending) =>
    pending.source === source && (theme === undefined || pending.options.theme === theme),
  )
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
