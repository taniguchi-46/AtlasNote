import type { Editor } from '@tiptap/core'
import { NodeSelection, TextSelection } from '@tiptap/pm/state'
import type { Node as ProseMirrorNode } from '@tiptap/pm/model'

type MermaidEditorStorage = {
  codeBlock?: { openMermaidEditorOnSelect?: boolean }
}

function isMermaidCodeBlock(node: ProseMirrorNode) {
  return node.type.name === 'codeBlock'
    && String(node.attrs.language ?? '').trim().toLowerCase() === 'mermaid'
}

export function insertMermaidCodeBlock(
  editor: Editor,
  source: string,
  replaceRange?: { from: number; to: number },
) {
  const storage = (editor.storage as MermaidEditorStorage).codeBlock
  if (storage) storage.openMermaidEditorOnSelect = true

  let insertedPosition: number | null = null
  const inserted = editor.chain()
    .focus()
    .command(({ tr }) => {
      if (replaceRange) {
        tr.setSelection(TextSelection.create(tr.doc, replaceRange.from, replaceRange.to))
        tr.deleteSelection()
      }
      // `insertContentAt` adjusts block insertion at the start of a text block
      // by one position. Calculate the same position before insertion so the
      // new node can be selected without scanning the whole document.
      const { from, to } = tr.selection
      insertedPosition = from
      if (from === to) {
        const { parent } = tr.doc.resolve(from)
        const isEmptyTextBlock = parent.isTextblock && !parent.type.spec.code && !parent.childCount
        if (isEmptyTextBlock) insertedPosition = Math.max(0, from - 1)
      }

      const $from = tr.doc.resolve(insertedPosition)
      const fromNode = $from.node()
      if (
        $from.parentOffset === 0
        && (fromNode.isText || fromNode.isTextblock)
        && fromNode.content.size > 0
      ) {
        insertedPosition = Math.max(0, insertedPosition - 1)
      }
      return true
    })
    .insertContent([
      {
        type: 'codeBlock',
        attrs: { language: 'mermaid' },
        content: [{ type: 'text', text: source }],
      },
      { type: 'paragraph' },
    ])
    .command(({ tr }) => {
      if (insertedPosition === null || editor.isDestroyed) {
        if (storage) storage.openMermaidEditorOnSelect = false
        return true
      }

      const insertedNode = tr.doc.nodeAt(insertedPosition)
      if (!insertedNode || !isMermaidCodeBlock(insertedNode)) {
        if (storage) storage.openMermaidEditorOnSelect = false
        return true
      }

      tr.setSelection(NodeSelection.create(tr.doc, insertedPosition))
      return true
    })
    .run()

  if (!inserted && storage) storage.openMermaidEditorOnSelect = false
  return inserted
}
