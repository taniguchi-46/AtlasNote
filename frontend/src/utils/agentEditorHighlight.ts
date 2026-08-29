export type AgentEditorTextHighlight = {
  prefix: string
  highlighted: string
  suffix: string
  isDeletion: boolean
}

export type AgentEditorBlockRange = {
  startIndex: number
  endIndex: number
  usesDeletionAnchor: boolean
}

type EditorDocumentLike = {
  content?: unknown[]
}

export function createAgentEditorTextHighlight(
  content: string,
  range: { start: number; end: number },
): AgentEditorTextHighlight {
  const start = clampOffset(range.start, content.length)
  const end = Math.max(start, clampOffset(range.end, content.length))

  return {
    prefix: content.slice(0, start),
    highlighted: content.slice(start, end),
    suffix: content.slice(end),
    isDeletion: start === end,
  }
}

export function findChangedTopLevelBlockRange(
  beforeDocument: EditorDocumentLike,
  afterDocument: EditorDocumentLike,
): AgentEditorBlockRange | null {
  const beforeNodes = beforeDocument.content ?? []
  const afterNodes = afterDocument.content ?? []
  let startIndex = 0

  while (
    startIndex < beforeNodes.length
    && startIndex < afterNodes.length
    && areEditorNodesEqual(beforeNodes[startIndex], afterNodes[startIndex])
  ) {
    startIndex += 1
  }

  let beforeEndIndex = beforeNodes.length
  let afterEndIndex = afterNodes.length
  while (
    beforeEndIndex > startIndex
    && afterEndIndex > startIndex
    && areEditorNodesEqual(beforeNodes[beforeEndIndex - 1], afterNodes[afterEndIndex - 1])
  ) {
    beforeEndIndex -= 1
    afterEndIndex -= 1
  }

  if (beforeEndIndex === startIndex && afterEndIndex === startIndex) return null
  if (afterEndIndex > startIndex) {
    return {
      startIndex,
      endIndex: afterEndIndex,
      usesDeletionAnchor: false,
    }
  }

  if (afterNodes.length === 0) return null
  const anchorIndex = Math.min(startIndex, afterNodes.length - 1)
  return {
    startIndex: anchorIndex,
    endIndex: anchorIndex + 1,
    usesDeletionAnchor: true,
  }
}

function clampOffset(value: number, length: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(Math.max(Math.trunc(value), 0), length)
}

function areEditorNodesEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right)
}
