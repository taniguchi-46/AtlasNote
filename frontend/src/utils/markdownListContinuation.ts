import type { MarkdownEditSnapshot } from './markdownEditHistory'

type ListMatch = {
  indent: string
  marker: string
  markerSpacing: string
  checkbox: string | null
  checkboxSpacing: string
  content: string
}

const LIST_PATTERN = /^([ \t]*)([-*+]|\d+\.)([ \t]+)(?:((?:\[[ xX]\]))([ \t]*))?(.*)$/
const THEMATIC_BREAK_PATTERN = /^\s*(?:[-*_]\s*){3,}$/
const FENCE_PATTERN = /^\s*(`{3,}|~{3,})/

type MarkdownModifierEvent = {
  altKey?: boolean
  ctrlKey?: boolean
  metaKey?: boolean
  shiftKey?: boolean
  isComposing?: boolean
  getModifierState?: (keyArg: string) => boolean
}

function hasModifier(event: MarkdownModifierEvent) {
  return Boolean(
    event.altKey
    || event.ctrlKey
    || event.metaKey
    || event.shiftKey
    || event.getModifierState?.('Alt')
    || event.getModifierState?.('Control')
    || event.getModifierState?.('Meta')
    || event.getModifierState?.('Shift')
  )
}

/**
 * Carries the modifier decision from keydown to the following beforeinput.
 * InputEvent does not consistently expose the modifier flags, especially on
 * desktop browsers, so the state is deliberately one-event and reset by the
 * next keydown or beforeinput.
 */
export function createMarkdownLineBreakTracker() {
  let skipNextListContinuation = false

  function handleKeydown(event: MarkdownModifierEvent & { key?: string }) {
    skipNextListContinuation = Boolean(
      event.key === 'Enter'
      && !event.isComposing
      && hasModifier(event),
    )
  }

  function shouldSkipListContinuation(event: MarkdownModifierEvent & { inputType?: string }) {
    const skipFromKeydown = skipNextListContinuation
    skipNextListContinuation = false
    if (event.inputType !== 'insertLineBreak' && event.inputType !== 'insertParagraph') return false
    return skipFromKeydown || hasModifier(event)
  }

  function reset() {
    skipNextListContinuation = false
  }

  return { handleKeydown, shouldSkipListContinuation, reset }
}

function parseListLine(line: string): ListMatch | null {
  const match = line.match(LIST_PATTERN)
  if (!match) return null

  return {
    indent: match[1],
    marker: match[2],
    markerSpacing: match[3],
    checkbox: match[4] ?? null,
    checkboxSpacing: match[5] ?? '',
    content: match[6],
  }
}

function isInsideFencedCode(content: string, lineStart: number) {
  let fenceCharacter: '`' | '~' | null = null
  let fenceLength = 0

  for (const rawLine of content.slice(0, lineStart).split('\n')) {
    const line = rawLine.replace(/\r$/, '')
    const match = line.match(FENCE_PATTERN)
    if (!match) continue

    const character = match[1][0] as '`' | '~'
    if (!fenceCharacter) {
      fenceCharacter = character
      fenceLength = match[1].length
      continue
    }
    if (character === fenceCharacter && match[1].length >= fenceLength) {
      fenceCharacter = null
      fenceLength = 0
    }
  }

  return fenceCharacter !== null
}

function getNextNumberedMarker(marker: string) {
  const match = marker.match(/^(\d+)\.$/)
  if (!match) return marker

  const number = Number(match[1])
  return Number.isSafeInteger(number) ? `${number + 1}.` : marker
}

function getNewline(content: string, lineEnd: number) {
  if (lineEnd < content.length && content[lineEnd] === '\n') {
    return lineEnd > 0 && content[lineEnd - 1] === '\r' ? '\r\n' : '\n'
  }
  return content.includes('\r\n') ? '\r\n' : '\n'
}

/**
 * Returns the Markdown snapshot produced by pressing Enter in a list item.
 * The helper is intentionally side-effect free so keyboard and beforeinput
 * paths can share the same continuation rules.
 */
export function continueMarkdownList(snapshot: MarkdownEditSnapshot): MarkdownEditSnapshot | null {
  const contentLength = snapshot.content.length
  const selectionStart = Math.min(contentLength, Math.max(0, Math.round(snapshot.selectionStart)))
  const selectionEnd = Math.min(contentLength, Math.max(selectionStart, Math.round(snapshot.selectionEnd)))
  if (selectionStart !== selectionEnd) return null

  const lineStart = snapshot.content.lastIndexOf('\n', Math.max(selectionStart - 1, 0)) + 1
  const lineBreakIndex = snapshot.content.indexOf('\n', selectionStart)
  const lineEnd = lineBreakIndex === -1 ? contentLength : lineBreakIndex
  const lineTextEnd = lineEnd > lineStart && snapshot.content[lineEnd - 1] === '\r'
    ? lineEnd - 1
    : lineEnd
  const line = snapshot.content.slice(lineStart, lineTextEnd)

  if (isInsideFencedCode(snapshot.content, lineStart) || THEMATIC_BREAK_PATTERN.test(line)) return null

  const list = parseListLine(line)
  if (!list) return null

  const markerEnd = lineStart + list.indent.length + list.marker.length + list.markerSpacing.length
  const contentStart = markerEnd + (list.checkbox
    ? list.checkbox.length + list.checkboxSpacing.length
    : 0)
  if (selectionStart < contentStart) return null

  const newline = getNewline(snapshot.content, lineEnd)
  const nextMarker = getNextNumberedMarker(list.marker)
  const nextPrefix = `${list.indent}${nextMarker}${list.markerSpacing}${list.checkbox
    ? `[ ]${list.checkboxSpacing || ' '}`
    : ''}`

  if (list.content.trim() === '') {
    const existingBreak = lineBreakIndex === -1 ? '' : snapshot.content.slice(lineTextEnd, lineEnd + 1)
    const afterLine = lineBreakIndex === -1 ? '' : snapshot.content.slice(lineEnd + 1)
    const nextContent = `${snapshot.content.slice(0, lineStart)}${existingBreak || newline}${afterLine}`
    const nextSelection = lineStart + (existingBreak || newline).length
    return {
      content: nextContent,
      selectionStart: nextSelection,
      selectionEnd: nextSelection,
    }
  }

  const nextContent = `${snapshot.content.slice(0, selectionStart)}${newline}${nextPrefix}${snapshot.content.slice(selectionStart)}`
  const nextSelection = selectionStart + newline.length + nextPrefix.length
  return {
    content: nextContent,
    selectionStart: nextSelection,
    selectionEnd: nextSelection,
  }
}
