import type { JSONContent } from '@tiptap/core'

import { serializeTiptapJsonToMarkdown } from './tiptapMarkdownSerializer'

export type TableClipboardPayload = {
  markdown: string
  plainText: string
  html?: string
}

type ClipboardWriter = Pick<Clipboard, 'write' | 'writeText'>
type ClipboardItemConstructor = new (
  items: Record<string, Blob | Promise<Blob>>,
) => ClipboardItem

export function createTableClipboardPayload(markdown: string, html?: string): TableClipboardPayload {
  return {
    markdown,
    plainText: markdown,
    ...(html ? { html } : {}),
  }
}

export function createTiptapTableClipboardPayload(table: JSONContent): TableClipboardPayload {
  const markdown = serializeTiptapJsonToMarkdown({
    type: 'doc',
    content: [table],
  })

  return createTableClipboardPayload(markdown)
}

export async function writeTableClipboard(
  payload: TableClipboardPayload,
  clipboard: Clipboard | undefined = typeof navigator === 'undefined' ? undefined : navigator.clipboard,
  clipboardItemConstructor?: ClipboardItemConstructor,
) {
  if (!clipboard) throw new Error('Clipboard API is unavailable')

  const itemConstructor = clipboardItemConstructor ?? getClipboardItemConstructor()

  if (payload.html) {
    if (typeof clipboard.write !== 'function' || !itemConstructor) {
      throw new Error('Rich clipboard API is unavailable')
    }

    const clipboardData: Record<string, Blob> = {
      'text/plain': new Blob([payload.plainText], { type: 'text/plain' }),
      'text/html': new Blob([payload.html], { type: 'text/html' }),
    }
    const item = new itemConstructor(clipboardData)
    await clipboard.write([item])
    return
  }

  if (typeof clipboard.writeText === 'function') {
    try {
      await clipboard.writeText(payload.plainText)
      return
    } catch (error) {
      if (typeof clipboard.write !== 'function' || !itemConstructor) throw error
    }
  }

  if (typeof clipboard.write === 'function' && itemConstructor) {
    const item = new itemConstructor({
      'text/plain': new Blob([payload.plainText], { type: 'text/plain' }),
    })
    await clipboard.write([item])
    return
  }

  throw new Error('Clipboard API is unavailable')
}

function getClipboardItemConstructor(): ClipboardItemConstructor | undefined {
  if (typeof globalThis.ClipboardItem !== 'function') return undefined
  return globalThis.ClipboardItem as ClipboardItemConstructor
}
