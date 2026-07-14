import type { JSONContent } from '@tiptap/core'

export function serializeTiptapJsonToMarkdown(node?: JSONContent): string {
  return serializeNode(node).trimEnd()
}

type SerializeContext = {
  tableCell?: boolean
}

function serializeNode(node?: JSONContent, context: SerializeContext = {}): string {
  if (!node) return ''

  switch (node.type) {
    case 'doc':
      return serializeBlockChildren(node.content)
    case 'paragraph':
      return serializeInlineChildren(node.content, context)
    case 'heading':
      return serializeHeading(node, context)
    case 'blockquote':
      return serializeBlockquote(node, context)
    case 'bulletList':
      return serializeList(node, '-', context)
    case 'orderedList':
      return serializeOrderedList(node, context)
    case 'listItem':
      return serializeListItem(node, '-', context)
    case 'taskList':
      return serializeTaskList(node, context)
    case 'taskItem':
      return serializeTaskItem(node, context)
    case 'codeBlock':
      return serializeCodeBlock(node, context)
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
      return serializeText(node, context)
    default:
      return serializeBlockChildren(node.content, context)
  }
}

function serializeBlockChildren(content?: JSONContent[], context: SerializeContext = {}) {
  return (content ?? [])
    .map((child) => serializeNode(child, context))
    .filter((text) => text.length > 0)
    .join('\n\n')
}

function serializeInlineChildren(content?: JSONContent[], context: SerializeContext = {}) {
  return (content ?? []).map((child) => serializeNode(child, context)).join('')
}

function serializeHeading(node: JSONContent, context: SerializeContext = {}) {
  const level = clampHeadingLevel(node.attrs?.level)
  const text = serializeInlineChildren(node.content, context)
  return `${'#'.repeat(level)} ${text}`.trimEnd()
}

function clampHeadingLevel(level: unknown) {
  if (typeof level !== 'number') return 1
  return Math.min(Math.max(level, 1), 6)
}

function serializeBlockquote(node: JSONContent, context: SerializeContext = {}) {
  return serializeBlockChildren(node.content, context)
    .split('\n')
    .map((line) => `> ${line}`)
    .join('\n')
}

function serializeList(node: JSONContent, marker: string, context: SerializeContext = {}) {
  return (node.content ?? [])
    .map((item) => serializeListItem(item, marker, context))
    .join('\n')
}

function serializeOrderedList(node: JSONContent, context: SerializeContext = {}) {
  const start = typeof node.attrs?.start === 'number' ? node.attrs.start : 1
  return (node.content ?? [])
    .map((item, index) => serializeListItem(item, `${start + index}.`, context))
    .join('\n')
}

function serializeTaskList(node: JSONContent, context: SerializeContext = {}) {
  return (node.content ?? []).map((item) => serializeTaskItem(item, context)).join('\n')
}

function serializeTaskItem(node: JSONContent, context: SerializeContext = {}) {
  const checked = node.attrs?.checked ? 'x' : ' '
  return serializeListItem(node, `- [${checked}]`, context)
}

function serializeListItem(node: JSONContent, marker = '-', context: SerializeContext = {}) {
  const parts = (node.content ?? [])
    .map((child) => serializeNode(child, context))
    .filter((text) => text.length > 0)
  const [first = '', ...rest] = parts
  const lines = [`${marker} ${first}`]

  rest.forEach((part) => {
    lines.push(...part.split('\n').map((line) => `  ${line}`))
  })

  return lines.join('\n')
}

function serializeCodeBlock(node: JSONContent, context: SerializeContext = {}) {
  const language = typeof node.attrs?.language === 'string' ? node.attrs.language : ''
  const code = serializeInlineChildren(node.content, context)
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
  return serializeBlockChildren(node.content, { tableCell: true })
    .replace(/\n/g, '&#10;')
}

function serializeText(node: JSONContent, context: SerializeContext = {}) {
  let text = node.text ?? ''

  if (context.tableCell) {
    text = escapeTableCellText(text, node.marks ?? [])
  }

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
        const safeHref = context.tableCell ? escapeTableCellLinkHref(href) : href
        text = safeHref ? `[${text}](${safeHref})` : text
        break
      }
    }
  }

  return text
}

function escapeTableCellText(text: string, marks: JSONContent['marks'] = []) {
  const hasCodeMark = marks.some((mark) => mark.type === 'code')
  const separated = escapeTableCellSeparators(text, hasCodeMark)

  return hasCodeMark
    ? separated
    : separated.replace(/([`*_\[\]~])/g, '\\$1')
}

function escapeTableCellSeparators(text: string, preserveBackslashes: boolean) {
  let result = ''

  for (let index = 0; index < text.length;) {
    if (text[index] !== '\\') {
      result += text[index] === '|' ? '\\|' : text[index]
      index += 1
      continue
    }

    const start = index
    while (index < text.length && text[index] === '\\') {
      index += 1
    }

    const slashCount = index - start
    const nextCharacter = text[index]
    if (nextCharacter === '|') {
      result += '\\'.repeat(slashCount + 1)
      result += '|'
      index += 1
      continue
    }

    result += '\\'.repeat(preserveBackslashes ? slashCount : slashCount * 2)
  }

  return result
}

function escapeTableCellLinkHref(href: string) {
  return href
    .replace(/\\/g, '\\\\')
    .replace(/\|/g, '\\|')
}

function escapeBracketText(text: string) {
  return text.replace(/\]/g, '\\]')
}
