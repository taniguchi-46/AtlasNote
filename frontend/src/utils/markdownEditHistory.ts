export type MarkdownEditSnapshot = {
  content: string
  selectionStart: number
  selectionEnd: number
}

export type MarkdownEditHistoryOptions = {
  depth?: number
  groupDelayMs?: number
  now?: () => number
}

export type MarkdownEditRecordOptions = {
  group?: string
  forceNewGroup?: boolean
}

function normalizeSnapshot(snapshot: MarkdownEditSnapshot): MarkdownEditSnapshot {
  const contentLength = snapshot.content.length
  const selectionStart = Math.min(contentLength, Math.max(0, Math.round(snapshot.selectionStart)))
  const selectionEnd = Math.min(contentLength, Math.max(selectionStart, Math.round(snapshot.selectionEnd)))
  return {
    content: snapshot.content,
    selectionStart,
    selectionEnd,
  }
}

function cloneSnapshot(snapshot: MarkdownEditSnapshot) {
  return { ...snapshot }
}

export function createMarkdownEditHistory(
  initialSnapshot: MarkdownEditSnapshot,
  options: MarkdownEditHistoryOptions = {},
) {
  const depth = Math.max(1, Math.round(options.depth ?? 100))
  const groupDelayMs = Math.max(0, Math.round(options.groupDelayMs ?? 500))
  const now = options.now ?? Date.now
  let present = normalizeSnapshot(initialSnapshot)
  let past: MarkdownEditSnapshot[] = []
  let future: MarkdownEditSnapshot[] = []
  let lastGroup: string | null = null
  let lastRecordedAt = 0

  function reset(snapshot: MarkdownEditSnapshot) {
    present = normalizeSnapshot(snapshot)
    past = []
    future = []
    lastGroup = null
    lastRecordedAt = 0
  }

  function record(
    beforeSnapshot: MarkdownEditSnapshot,
    afterSnapshot: MarkdownEditSnapshot,
    recordOptions: MarkdownEditRecordOptions = {},
  ) {
    const before = normalizeSnapshot(beforeSnapshot)
    const after = normalizeSnapshot(afterSnapshot)
    if (before.content === after.content) {
      present = after
      return false
    }

    if (present.content !== before.content) {
      reset(before)
    } else {
      present = before
    }

    const recordedAt = now()
    const group = recordOptions.group ?? 'input'
    const canGroup = !recordOptions.forceNewGroup
      && future.length === 0
      && lastGroup === group
      && recordedAt - lastRecordedAt <= groupDelayMs

    if (!canGroup) {
      past.push(cloneSnapshot(present))
      if (past.length > depth) past = past.slice(-depth)
    }

    present = after
    future = []
    lastGroup = group
    lastRecordedAt = recordedAt
    return true
  }

  function undo() {
    const previous = past.pop()
    if (!previous) return null

    future.unshift(cloneSnapshot(present))
    present = previous
    lastGroup = null
    lastRecordedAt = 0
    return cloneSnapshot(present)
  }

  function redo() {
    const next = future.shift()
    if (!next) return null

    past.push(cloneSnapshot(present))
    if (past.length > depth) past = past.slice(-depth)
    present = next
    lastGroup = null
    lastRecordedAt = 0
    return cloneSnapshot(present)
  }

  function current() {
    return cloneSnapshot(present)
  }

  return {
    current,
    record,
    redo,
    reset,
    undo,
  }
}
