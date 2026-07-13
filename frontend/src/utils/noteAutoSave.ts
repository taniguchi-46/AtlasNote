export type NoteSaveSnapshot = {
  noteId: string
  title: string
  content: string
  draftVersion: number
}

type TimerHandle = ReturnType<typeof setTimeout>

type NoteAutoSaveOptions<Result> = {
  delayMs: number
  save: (snapshot: NoteSaveSnapshot) => Promise<Result | null>
  shouldApply: (snapshot: NoteSaveSnapshot) => boolean
  isCurrent: (snapshot: NoteSaveSnapshot) => boolean
  applyResult: (snapshot: NoteSaveSnapshot, result: Result) => void
  onSaved?: (snapshot: NoteSaveSnapshot) => void
  onFailed?: (snapshot: NoteSaveSnapshot) => void
  execute?: (noteId: string, operation: () => Promise<boolean>) => Promise<boolean>
  setTimer?: (callback: () => void, delayMs: number) => TimerHandle
  clearTimer?: (timer: TimerHandle) => void
}

type SaveLane = {
  timer: TimerHandle | null
  pendingSnapshot: NoteSaveSnapshot | null
  inFlightSave: Promise<boolean> | null
  blocked: boolean
}

export function createNoteAutoSave<Result>(options: NoteAutoSaveOptions<Result>) {
  const setTimer = options.setTimer ?? ((callback, delayMs) => setTimeout(callback, delayMs))
  const clearTimer = options.clearTimer ?? ((timer) => clearTimeout(timer))
  const lanes = new Map<string, SaveLane>()

  function getOrCreateLane(noteId: string) {
    const existing = lanes.get(noteId)
    if (existing) return existing

    const lane: SaveLane = {
      timer: null,
      pendingSnapshot: null,
      inFlightSave: null,
      blocked: false,
    }
    lanes.set(noteId, lane)
    return lane
  }

  function cancelTimer(lane: SaveLane) {
    if (lane.timer === null) return

    clearTimer(lane.timer)
    lane.timer = null
  }

  function deleteIdleLane(noteId: string, lane: SaveLane) {
    if (!lane.blocked && !lane.pendingSnapshot && !lane.inFlightSave && lane.timer === null) {
      lanes.delete(noteId)
    }
  }

  function notifyFailed(snapshot: NoteSaveSnapshot) {
    if (!options.isCurrent(snapshot)) return
    try {
      options.onFailed?.(snapshot)
    } catch {
      // A failure callback must not create a second unhandled rejection.
    }
  }

  async function saveSnapshot(snapshot: NoteSaveSnapshot) {
    const result = await options.save(snapshot)
    if (result === null) {
      notifyFailed(snapshot)
      return false
    }

    if (options.shouldApply(snapshot)) {
      options.applyResult(snapshot, result)
    }
    if (options.isCurrent(snapshot)) {
      options.onSaved?.(snapshot)
    }
    return true
  }

  async function runPending(noteId: string) {
    const lane = lanes.get(noteId)
    if (!lane) return true

    cancelTimer(lane)
    if (lane.blocked) return false

    const snapshot = lane.pendingSnapshot
    lane.pendingSnapshot = null
    if (!snapshot) {
      return lane.inFlightSave ? await lane.inFlightSave : true
    }

    const previousSave = lane.inFlightSave
    const save = (async () => {
      if (previousSave) {
        const previousSucceeded = await previousSave
        if (!previousSucceeded) {
          notifyFailed(snapshot)
          return false
        }
      }
      const operation = () => saveSnapshot(snapshot)
      return options.execute
        ? options.execute(snapshot.noteId, operation)
        : operation()
    })()
    lane.inFlightSave = save
    let succeeded = false
    try {
      succeeded = await save
    } catch {
      // A coordinator callback must not leak a rejection into a timer or
      // window-close fire-and-forget path. Treat unexpected failures like a
      // normal failed save so the lane can be retried explicitly.
      notifyFailed(snapshot)
      succeeded = false
    }
    try {
      if (!succeeded) {
        lane.blocked = true
        cancelTimer(lane)
      }
      return succeeded
    } finally {
      if (lane.inFlightSave === save) {
        lane.inFlightSave = null
      }
      deleteIdleLane(noteId, lane)
    }
  }

  function scheduleSnapshot(snapshot: NoteSaveSnapshot, resume: boolean) {
    const lane = getOrCreateLane(snapshot.noteId)
    cancelTimer(lane)
    lane.pendingSnapshot = snapshot
    if (resume) lane.blocked = false
    if (lane.blocked) return

    lane.timer = setTimer(() => {
      void runPending(snapshot.noteId)
    }, options.delayMs)
  }

  function schedule(snapshot: NoteSaveSnapshot) {
    scheduleSnapshot(snapshot, false)
  }

  function retry(snapshot: NoteSaveSnapshot) {
    scheduleSnapshot(snapshot, true)
  }

  async function flushLane(noteId: string) {
    let succeeded = true
    while (true) {
      const lane = lanes.get(noteId)
      if (!lane) return succeeded
      if (lane.blocked) return false
      if (!lane.pendingSnapshot && !lane.inFlightSave) {
        deleteIdleLane(noteId, lane)
        return succeeded
      }

      const result = lane.pendingSnapshot
        ? await runPending(noteId)
        : await lane.inFlightSave!
      succeeded = result && succeeded
    }
  }

  async function flush(noteId?: string) {
    if (noteId) return flushLane(noteId)

    let succeeded = true
    while (lanes.size > 0) {
      const activeNoteIds = [...lanes.entries()]
        .filter(([, lane]) => !lane.blocked && (lane.pendingSnapshot || lane.inFlightSave))
        .map(([activeNoteId]) => activeNoteId)
      if (activeNoteIds.length === 0) break

      const results = await Promise.all(activeNoteIds.map(flushLane))
      succeeded = results.every(Boolean) && succeeded
    }
    return succeeded && [...lanes.values()].every((lane) => !lane.blocked)
  }

  function cancel(noteId?: string) {
    const targetLanes = noteId
      ? [...lanes.entries()].filter(([laneNoteId]) => laneNoteId === noteId)
      : [...lanes.entries()]

    for (const [laneNoteId, lane] of targetLanes) {
      cancelTimer(lane)
      lane.pendingSnapshot = null
      lane.blocked = false
      deleteIdleLane(laneNoteId, lane)
    }
  }

  return {
    schedule,
    retry,
    flush,
    cancel,
  }
}
