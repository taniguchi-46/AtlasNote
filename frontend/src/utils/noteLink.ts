const NOTE_ID_LENGTH = 32
const NOTE_ID_PATTERN = /^[0-9a-f]{32}$/
const NOTE_LINK_PREFIX = 'atlasnote://note/'

export function isNoteID(value: string) {
  return NOTE_ID_PATTERN.test(value)
}

export function createNoteLinkHref(noteId: string) {
  if (!isNoteID(noteId)) {
    throw new Error('ノートリンクのIDが不正です')
  }
  return `${NOTE_LINK_PREFIX}${noteId}`
}

export function parseNoteLinkHref(href: string) {
  if (!href.startsWith(NOTE_LINK_PREFIX)) return null

  const noteId = href.slice(NOTE_LINK_PREFIX.length)
  return isNoteID(noteId) ? noteId : null
}

export function createNoteLinkMarkdown(title: string, noteId: string) {
  const label = title
    .replace(/[\r\n]+/g, ' ')
    .replace(/\\/g, '\\\\')
    .replace(/[\[\]]/g, '\\$&')
  return `[${label}](${createNoteLinkHref(noteId)})`
}

export function findNoteLinkTargetAt(markdown: string, position: number) {
  if (position < 0 || position > markdown.length) return null

  let lineStart = 0
  let inFence: '`' | '~' | null = null

  for (const line of markdown.split('\n')) {
    const marker = fenceMarker(line.replace(/^[ \t]+/, ''))
    if (marker) {
      if (inFence === null) {
        inFence = marker
      } else if (inFence === marker) {
        inFence = null
      }
      lineStart += line.length + 1
      continue
    }

    const lineEnd = lineStart + line.length
    if (inFence === null && position >= lineStart && position <= lineEnd) {
      const targetID = findLineNoteLinkTargetAt(line, position - lineStart)
      if (targetID) return targetID
    }

    lineStart = lineEnd + 1
  }

  return null
}

function findLineNoteLinkTargetAt(line: string, position: number) {
  let inlineCodeTicks = 0

  for (let index = 0; index < line.length;) {
    if (line[index] === '`') {
      const tickCount = countRun(line, index, '`')
      if (inlineCodeTicks === 0) {
        inlineCodeTicks = tickCount
      } else if (inlineCodeTicks === tickCount) {
        inlineCodeTicks = 0
      }
      index += tickCount
      continue
    }
    if (inlineCodeTicks !== 0) {
      index += 1
      continue
    }
    if (line[index] !== '[' || isEscaped(line, index) || (index > 0 && line[index - 1] === '!')) {
      index += 1
      continue
    }

    const closingBracket = findClosingBracket(line, index + 1)
    if (closingBracket < 0 || line[closingBracket + 1] !== '(') {
      index += 1
      continue
    }

    const hrefStart = closingBracket + 2
    const hrefEnd = hrefStart + NOTE_LINK_PREFIX.length + NOTE_ID_LENGTH
    if (hrefEnd >= line.length || line[hrefEnd] !== ')') {
      index += 1
      continue
    }

    const targetID = parseNoteLinkHref(line.slice(hrefStart, hrefEnd))
    const linkEnd = hrefEnd + 1
    if (targetID && position >= index && position <= linkEnd) return targetID

    index = linkEnd
  }

  return null
}

function fenceMarker(line: string): '`' | '~' | null {
  if (line.startsWith('```')) return '`'
  if (line.startsWith('~~~')) return '~'
  return null
}

function findClosingBracket(line: string, start: number) {
  for (let index = start; index < line.length; index += 1) {
    if (line[index] === ']' && !isEscaped(line, index)) return index
  }
  return -1
}

function countRun(value: string, start: number, character: string) {
  let count = 0
  while (value[start + count] === character) count += 1
  return count
}

function isEscaped(value: string, index: number) {
  let backslashes = 0
  while (index > 0 && value[index - 1] === '\\') {
    backslashes += 1
    index -= 1
  }
  return backslashes % 2 === 1
}
