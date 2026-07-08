import type { JSONContent } from '@tiptap/core'

export function serializeTiptapJsonToMarkdown(node?: JSONContent): string {
  return serializeNode(node).trimEnd()
}

function serializeNode(node?: JSONContent): string {
  if (!node) return ''

  switch (node.type) {
    case 'doc':
      return serializeBlockChildren(node.content)
    case 'paragraph':
      return serializeInlineChildren(node.content)
    case 'heading':
      return serializeHeading(node)
    case 'blockquote':
      return serializeBlockquote(node)
    case 'bulletList':
      return serializeList(node, '-')
    case 'orderedList':
      return serializeOrderedList(node)
    case 'listItem':
      return serializeListItem(node)
    case 'taskList':
      return serializeTaskList(node)
    case 'taskItem':
      return serializeTaskItem(node)
    case 'codeBlock':
      return serializeCodeBlock(node)
    case 'horizontalRule':
      return '---'
    case 'image':
      return serializeImage(node)
    case 'table':
      return serializeTable(node)
    case 'tableRow':
      return serializeTableRow(node)
    case 'tableCell':
    case 'tableHeader':
      return serializeTableCell(node)
    case 'hardBreak':
      return '\n'
    case 'text':
      return serializeText(node)
    default:
      return serializeBlockChildren(node.content)
  }
}

function serializeBlockChildren(content?: JSONContent[]) {
  return (content ?? [])
    .map((child) => serializeNode(child))
    .filter((text) => text.length > 0)
    .join('\n\n')
}

function serializeInlineChildren(content?: JSONContent[]) {
  return (content ?? []).map((child) => serializeNode(child)).join('')
}

function serializeHeading(node: JSONContent) {
  const level = clampHeadingLevel(node.attrs?.level)
  const text = serializeInlineChildren(node.content)
  return `${'#'.repeat(level)} ${text}`.trimEnd()
}

function clampHeadingLevel(level: unknown) {
  if (typeof level !== 'number') return 1
  return Math.min(Math.max(level, 1), 6)
}

function serializeBlockquote(node: JSONContent) {
  return serializeBlockChildren(node.content)
    .split('\n')
    .map((line) => `> ${line}`)
    .join('\n')
}

function serializeList(node: JSONContent, marker: string) {
  return (node.content ?? [])
    .map((item) => serializeListItem(item, marker))
    .join('\n')
}

function serializeOrderedList(node: JSONContent) {
  const start = typeof node.attrs?.start === 'number' ? node.attrs.start : 1
  return (node.content ?? [])
    .map((item, index) => serializeListItem(item, `${start + index}.`))
    .join('\n')
}

function serializeTaskList(node: JSONContent) {
  return (node.content ?? []).map((item) => serializeTaskItem(item)).join('\n')
}

function serializeTaskItem(node: JSONContent) {
  const checked = node.attrs?.checked ? 'x' : ' '
  return serializeListItem(node, `- [${checked}]`)
}

function serializeListItem(node: JSONContent, marker = '-') {
  const parts = (node.content ?? [])
    .map((child) => serializeNode(child))
    .filter((text) => text.length > 0)
  const [first = '', ...rest] = parts
  const lines = [`${marker} ${first}`]

  rest.forEach((part) => {
    lines.push(...part.split('\n').map((line) => `  ${line}`))
  })

  return lines.join('\n')
}

function serializeCodeBlock(node: JSONContent) {
  const language = typeof node.attrs?.language === 'string' ? node.attrs.language : ''
  const code = serializeInlineChildren(node.content)
  return `\`\`\`${language}\n${code}\n\`\`\``
}

function serializeImage(node: JSONContent) {
  const src = typeof node.attrs?.src === 'string' ? node.attrs.src : ''
  const alt = typeof node.attrs?.alt === 'string' ? node.attrs.alt : ''
  return src ? `![${escapeBracketText(alt)}](${src})` : ''
}

function serializeTable(node: JSONContent) {
  const rows = (node.content ?? []).map((row) =>
    (row.content ?? []).map((cell) => serializeTableCell(cell)),
  )

  if (rows.length === 0) return ''

  const [header, ...body] = rows
  const separator = header.map(() => '---')

  return [header, separator, ...body]
    .map((row) => `| ${row.join(' | ')} |`)
    .join('\n')
}

function serializeTableRow(node: JSONContent) {
  return (node.content ?? [])
    .map((cell) => serializeTableCell(cell))
    .join(' | ')
}

function serializeTableCell(node: JSONContent) {
  return serializeBlockChildren(node.content)
    .replace(/\n+/g, '<br>')
    .replace(/\|/g, '\\|')
}

function serializeText(node: JSONContent) {
  let text = node.text ?? ''

  for (const mark of node.marks ?? []) {
    switch (mark.type) {
      case 'code':
        text = `\`${text}\``
        break
      case 'bold':
        text = `**${text}**`
        break
      case 'italic':
        text = `*${text}*`
        break
      case 'strike':
        text = `~~${text}~~`
        break
      case 'link': {
        const href = typeof mark.attrs?.href === 'string' ? mark.attrs.href : ''
        text = href ? `[${text}](${href})` : text
        break
      }
    }
  }

  return text
}

function escapeBracketText(text: string) {
  return text.replace(/\]/g, '\\]')
}
