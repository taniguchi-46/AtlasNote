import type { AgentEditProposal } from '../api/ai'

export type AgentEditPatchResult =
  | { status: 'ok'; content: string; range: { start: number; end: number } }
  | { status: 'conflict'; reason: 'not-found' | 'ambiguous' | 'invalid-empty-target' | 'unchanged' }

export type AgentEditDiffSegment = {
  text: string
  changed: boolean
}

export type AgentEditDiffLine = {
  rowNumber: number
  lineNumber: number | null
  text: string
  changed: boolean
  placeholder: boolean
  segments: AgentEditDiffSegment[]
}

export type AgentEditVisualDiff = {
  beforeLines: AgentEditDiffLine[]
  afterLines: AgentEditDiffLine[]
}

type LinePair = {
  beforeIndex?: number
  afterIndex?: number
}

type SegmenterLike = {
  segment: (input: string) => Iterable<{ segment: string }>
}

type SegmenterConstructor = new (
  locales?: string | string[],
  options?: { granularity?: 'grapheme' | 'word' },
) => SegmenterLike

const MAX_LINE_LCS_CELLS = 16_384
const MAX_TOKEN_LCS_CELLS = 16_384
const segmenterConstructor = (Intl as unknown as { Segmenter?: SegmenterConstructor }).Segmenter
const wordSegmenter = createSegmenter('word')
const graphemeSegmenter = createSegmenter('grapheme')

// applyAgentEditHunk deliberately accepts only one exact, unique replacement.
// This keeps a stale or ambiguous AI proposal from replacing an unrelated part
// of the active note body.
export function applyAgentEditHunk(
  content: string,
  proposal: Pick<AgentEditProposal, 'before' | 'after'>,
): AgentEditPatchResult {
  if (proposal.before === proposal.after) return { status: 'conflict', reason: 'unchanged' }
  if (proposal.before === '') return { status: 'conflict', reason: 'invalid-empty-target' }

  const first = content.indexOf(proposal.before)
  if (first < 0) return { status: 'conflict', reason: 'not-found' }
  if (content.indexOf(proposal.before, first + 1) >= 0) {
    return { status: 'conflict', reason: 'ambiguous' }
  }
  return {
    status: 'ok',
    content: `${content.slice(0, first)}${proposal.after}${content.slice(first + proposal.before.length)}`,
    range: {
      start: first,
      end: first + proposal.after.length,
    },
  }
}

// This visual diff is deliberately separate from applyAgentEditHunk. It never
// changes note content and bounds its LCS work so a valid 16 KiB proposal
// cannot stall the UI. Large inputs fall back to prefix/suffix highlighting.
export function createAgentEditVisualDiff(
  before: string,
  after: string,
): AgentEditVisualDiff {
  const beforeTexts = splitProposalLines(before)
  const afterTexts = splitProposalLines(after)
  const beforeLines: AgentEditDiffLine[] = []
  const afterLines: AgentEditDiffLine[] = []

  for (const [rowIndex, pair] of alignLines(beforeTexts, afterTexts).entries()) {
    const beforeLine = pair.beforeIndex === undefined
      ? createPlaceholderLine(rowIndex)
      : createChangedLine(beforeTexts[pair.beforeIndex] ?? '', pair.beforeIndex, rowIndex)
    const afterLine = pair.afterIndex === undefined
      ? createPlaceholderLine(rowIndex)
      : createChangedLine(afterTexts[pair.afterIndex] ?? '', pair.afterIndex, rowIndex)
    beforeLines.push(beforeLine)
    afterLines.push(afterLine)

    if (beforeLine.placeholder || afterLine.placeholder) {
      continue
    }

    if (beforeLine.text === afterLine.text) {
      beforeLine.changed = false
      afterLine.changed = false
      beforeLine.segments = createSingleSegment(beforeLine.text, false)
      afterLine.segments = createSingleSegment(afterLine.text, false)
      continue
    }

    const segments = createTextSegments(beforeLine.text, afterLine.text)
    beforeLine.segments = segments.before
    afterLine.segments = segments.after
  }

  return { beforeLines, afterLines }
}

function createSegmenter(granularity: 'grapheme' | 'word'): SegmenterLike | undefined {
  if (!segmenterConstructor) return undefined
  try {
    return new segmenterConstructor('ja', { granularity })
  } catch {
    return undefined
  }
}

function splitProposalLines(value: string): string[] {
  return value.split(/\r\n|\n|\r/)
}

function createChangedLine(text: string, lineIndex: number, rowIndex: number): AgentEditDiffLine {
  return {
    rowNumber: rowIndex + 1,
    lineNumber: lineIndex + 1,
    text,
    changed: true,
    placeholder: false,
    segments: createSingleSegment(text, true),
  }
}

function createPlaceholderLine(rowIndex: number): AgentEditDiffLine {
  return {
    rowNumber: rowIndex + 1,
    lineNumber: null,
    text: '',
    changed: false,
    placeholder: true,
    segments: [],
  }
}

function createSingleSegment(text: string, changed: boolean): AgentEditDiffSegment[] {
  return text === '' ? [] : [{ text, changed }]
}

function alignLines(beforeLines: string[], afterLines: string[]): LinePair[] {
  const matches = beforeLines.length * afterLines.length <= MAX_LINE_LCS_CELLS
    ? findLCSMatches(beforeLines, afterLines)
    : findAffixMatches(beforeLines, afterLines)
  const pairs: LinePair[] = []
  let beforeStart = 0
  let afterStart = 0

  for (const [beforeMatch, afterMatch] of [...matches, [beforeLines.length, afterLines.length] as const]) {
    const beforeCount = beforeMatch - beforeStart
    const afterCount = afterMatch - afterStart
    const pairedCount = Math.min(beforeCount, afterCount)

    for (let offset = 0; offset < pairedCount; offset += 1) {
      pairs.push({
        beforeIndex: beforeStart + offset,
        afterIndex: afterStart + offset,
      })
    }
    for (let offset = pairedCount; offset < beforeCount; offset += 1) {
      pairs.push({ beforeIndex: beforeStart + offset })
    }
    for (let offset = pairedCount; offset < afterCount; offset += 1) {
      pairs.push({ afterIndex: afterStart + offset })
    }

    if (beforeMatch < beforeLines.length && afterMatch < afterLines.length) {
      pairs.push({ beforeIndex: beforeMatch, afterIndex: afterMatch })
    }
    beforeStart = beforeMatch + 1
    afterStart = afterMatch + 1
  }

  return pairs
}

function findLCSMatches(left: string[], right: string[]): Array<readonly [number, number]> {
  const matrix = Array.from(
    { length: left.length + 1 },
    () => new Uint16Array(right.length + 1),
  )

  for (let leftIndex = left.length - 1; leftIndex >= 0; leftIndex -= 1) {
    for (let rightIndex = right.length - 1; rightIndex >= 0; rightIndex -= 1) {
      matrix[leftIndex][rightIndex] = left[leftIndex] === right[rightIndex]
        ? matrix[leftIndex + 1][rightIndex + 1] + 1
        : Math.max(matrix[leftIndex + 1][rightIndex], matrix[leftIndex][rightIndex + 1])
    }
  }

  const matches: Array<readonly [number, number]> = []
  let leftIndex = 0
  let rightIndex = 0
  while (leftIndex < left.length && rightIndex < right.length) {
    if (left[leftIndex] === right[rightIndex]) {
      matches.push([leftIndex, rightIndex])
      leftIndex += 1
      rightIndex += 1
    } else if (matrix[leftIndex + 1][rightIndex] >= matrix[leftIndex][rightIndex + 1]) {
      leftIndex += 1
    } else {
      rightIndex += 1
    }
  }
  return matches
}

function findAffixMatches(left: string[], right: string[]): Array<readonly [number, number]> {
  const matches: Array<readonly [number, number]> = []
  let prefixLength = 0
  while (
    prefixLength < left.length
    && prefixLength < right.length
    && left[prefixLength] === right[prefixLength]
  ) {
    matches.push([prefixLength, prefixLength])
    prefixLength += 1
  }

  const suffixMatches: Array<readonly [number, number]> = []
  let leftIndex = left.length - 1
  let rightIndex = right.length - 1
  while (
    leftIndex >= prefixLength
    && rightIndex >= prefixLength
    && left[leftIndex] === right[rightIndex]
  ) {
    suffixMatches.push([leftIndex, rightIndex])
    leftIndex -= 1
    rightIndex -= 1
  }
  return matches.concat(suffixMatches.reverse())
}

function createTextSegments(
  before: string,
  after: string,
): { before: AgentEditDiffSegment[]; after: AgentEditDiffSegment[] } {
  const beforeTokens = segmentWords(before)
  const afterTokens = segmentWords(after)
  if (
    beforeTokens.length === 0
    || afterTokens.length === 0
    || beforeTokens.length * afterTokens.length > MAX_TOKEN_LCS_CELLS
  ) {
    return createAffixSegments(before, after)
  }

  const beforeChanged = beforeTokens.map(() => true)
  const afterChanged = afterTokens.map(() => true)
  for (const [beforeIndex, afterIndex] of findLCSMatches(beforeTokens, afterTokens)) {
    beforeChanged[beforeIndex] = false
    afterChanged[afterIndex] = false
  }
  return {
    before: mergeSegments(beforeTokens, beforeChanged),
    after: mergeSegments(afterTokens, afterChanged),
  }
}

function segmentWords(value: string): string[] {
  if (value === '') return []
  if (wordSegmenter) {
    return Array.from(wordSegmenter.segment(value), ({ segment }) => segment)
  }
  return value.match(/\s+|[A-Za-z0-9_]+|./gu) ?? []
}

function segmentGraphemes(value: string): string[] {
  if (value === '') return []
  if (graphemeSegmenter) {
    return Array.from(graphemeSegmenter.segment(value), ({ segment }) => segment)
  }
  return Array.from(value)
}

function createAffixSegments(
  before: string,
  after: string,
): { before: AgentEditDiffSegment[]; after: AgentEditDiffSegment[] } {
  const beforeParts = segmentGraphemes(before)
  const afterParts = segmentGraphemes(after)
  let prefixLength = 0
  while (
    prefixLength < beforeParts.length
    && prefixLength < afterParts.length
    && beforeParts[prefixLength] === afterParts[prefixLength]
  ) {
    prefixLength += 1
  }

  let suffixLength = 0
  while (
    suffixLength < beforeParts.length - prefixLength
    && suffixLength < afterParts.length - prefixLength
    && beforeParts[beforeParts.length - suffixLength - 1] === afterParts[afterParts.length - suffixLength - 1]
  ) {
    suffixLength += 1
  }

  return {
    before: mergeAffixParts(beforeParts, prefixLength, suffixLength),
    after: mergeAffixParts(afterParts, prefixLength, suffixLength),
  }
}

function mergeAffixParts(
  parts: string[],
  prefixLength: number,
  suffixLength: number,
): AgentEditDiffSegment[] {
  const changedEnd = parts.length - suffixLength
  const segments: AgentEditDiffSegment[] = []
  appendSegment(segments, parts.slice(0, prefixLength).join(''), false)
  appendSegment(segments, parts.slice(prefixLength, changedEnd).join(''), true)
  appendSegment(segments, parts.slice(changedEnd).join(''), false)
  return segments
}

function mergeSegments(tokens: string[], changed: boolean[]): AgentEditDiffSegment[] {
  const segments: AgentEditDiffSegment[] = []
  for (let index = 0; index < tokens.length; index += 1) {
    appendSegment(segments, tokens[index], changed[index])
  }
  return segments
}

function appendSegment(
  segments: AgentEditDiffSegment[],
  text: string,
  changed: boolean,
): void {
  if (text === '') return
  const previous = segments[segments.length - 1]
  if (previous?.changed === changed) {
    previous.text += text
  } else {
    segments.push({ text, changed })
  }
}
