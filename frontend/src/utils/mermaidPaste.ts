import type { Editor } from '@tiptap/core'
import { Slice, type Node as ProseMirrorNode } from '@tiptap/pm/model'
import { NodeSelection, TextSelection, type Selection, type Transaction } from '@tiptap/pm/state'
import type { EditorView } from '@tiptap/pm/view'
import { readMermaidClipboardPayload } from './mermaidClipboard'

type MermaidPasteOptions = {
  editor: Pick<Editor, 'schema'>
  view: EditorView
  event: ClipboardEvent
  parseMarkdown: (markdown: string) => ProseMirrorNode
  onError?: (error: unknown) => void
}

export function handleMermaidPaste({
  editor,
  view,
  event,
  parseMarkdown,
  onError,
}: MermaidPasteOptions) {
  const payload = readMermaidClipboardPayload(event.clipboardData)
  if (!payload || findRichCodeBlockNode(view.state.selection)) return false

  try {
    if (payload.kind === 'source') {
      const codeBlock = editor.schema.nodes.codeBlock.create(
        { language: 'mermaid' },
        payload.source ? editor.schema.text(payload.source) : undefined,
      )
      const transaction = view.state.tr.replaceSelectionWith(codeBlock)
      const insertedCodeBlock = findRichCodeBlockRange(transaction.selection)
      if (!insertedCodeBlock || insertedCodeBlock.node.type.name !== 'codeBlock') return false

      moveSelectionAfterCodeBlock(transaction, insertedCodeBlock)
      view.dispatch(transaction.scrollIntoView())
      return true
    }

    const parsedDocument = parseMarkdown(payload.markdown)
    const transaction = view.state.tr.replaceSelection(
      new Slice(parsedDocument.content, 0, 0),
    )
    const insertedCodeBlock = findRichCodeBlockRange(transaction.selection)
    if (insertedCodeBlock && isMermaidCodeBlock(insertedCodeBlock.node)) {
      // Markdown parsing can leave the cursor inside a trailing Mermaid
      // codeBlock just like raw-source insertion. Keep that cursor outside the
      // hidden NodeViewContent so Enter edits a paragraph instead of the source.
      moveSelectionAfterCodeBlock(transaction, insertedCodeBlock)
    }
    view.dispatch(transaction.scrollIntoView())
    return true
  } catch (error) {
    onError?.(error)
    return false
  }
}

function moveSelectionAfterCodeBlock(
  transaction: Transaction,
  codeBlockRange: { to: number },
) {
  // NodeViewContent is intentionally non-editable for Mermaid. Move the
  // cursor to a following text block so Enter/arrow navigation cannot place
  // a text cursor into the hidden source content.
  const afterCodeBlock = codeBlockRange.to
  const nextNode = transaction.doc.nodeAt(afterCodeBlock)
  if (!nextNode || !nextNode.isTextblock || nextNode.type.name === 'codeBlock') {
    transaction.insert(afterCodeBlock, transaction.doc.type.schema.nodes.paragraph.create())
    transaction.setSelection(TextSelection.create(transaction.doc, afterCodeBlock + 1))
  } else {
    transaction.setSelection(TextSelection.near(
      transaction.doc.resolve(afterCodeBlock + 1),
    ))
  }
}

function findRichCodeBlockNode(selection: Selection): ProseMirrorNode | null {
  return findRichCodeBlockRange(selection)?.node ?? null
}

function isMermaidCodeBlock(node: ProseMirrorNode) {
  return node.type.name === 'codeBlock'
    && String(node.attrs.language ?? '').trim().toLowerCase() === 'mermaid'
}

function findRichCodeBlockRange(selection: Selection) {
  if (selection instanceof NodeSelection) {
    return selection.node.type.name === 'codeBlock'
      ? { node: selection.node, from: selection.from, to: selection.to }
      : null
  }

  const { $from } = selection
  for (let depth = $from.depth; depth > 0; depth -= 1) {
    const node = $from.node(depth)
    if (node.type.name === 'codeBlock') {
      return { node, from: $from.before(depth), to: $from.after(depth) }
    }
  }

  return null
}
