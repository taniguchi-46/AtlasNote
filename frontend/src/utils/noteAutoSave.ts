export type NoteSaveSnapshot = {
  noteId: string
  title: string
  content: string
  revision: number
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
  setTimer?: (callback: () => void, delayMs: number) => TimerHandle
  clearTimer?: (timer: TimerHandle) => void
}

export function createNoteAutoSave<Result>(options: NoteAutoSaveOptions<Result>) {
  const setTimer = options.setTimer ?? ((callback, delayMs) => setTimeout(callback, delayMs))
  const clearTimer = options.clearTimer ?? ((timer) => clearTimeout(timer))
  let timer: TimerHandle | null = null
  let pendingSnapshot: NoteSaveSnapshot | null = null
  let inFlightSave: Promise<boolean> | null = null

  function cancelTimer() {
    if (timer === null) return

    clearTimer(timer)
    timer = null
  }

  async function saveSnapshot(snapshot: NoteSaveSnapshot) {
    const result = await options.save(snapshot)
    if (result === null) {
      if (options.isCurrent(snapshot)) {
        options.onFailed?.(snapshot)
      }
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

  async function runPending() {
    cancelTimer()
    const snapshot = pendingSnapshot
    pendingSnapshot = null
    if (!snapshot) {
      return inFlightSave ? await inFlightSave : true
    }

    const previousSave = inFlightSave
    const save = (async () => {
      if (previousSave) {
        await previousSave
      }
      return saveSnapshot(snapshot)
    })()
    inFlightSave = save
    try {
      return await save
    } finally {
      if (inFlightSave === save) {
        inFlightSave = null
      }
    }
  }

  function schedule(snapshot: NoteSaveSnapshot) {
    cancelTimer()
    pendingSnapshot = snapshot
    timer = setTimer(() => {
      void runPending()
    }, options.delayMs)
  }

  async function flush() {
    let succeeded = true

    while (pendingSnapshot || inFlightSave) {
      const result = pendingSnapshot
        ? await runPending()
        : await inFlightSave!
      succeeded = result && succeeded
    }

    return succeeded
  }

  function cancel() {
    cancelTimer()
    pendingSnapshot = null
  }

  return {
    schedule,
    flush,
    cancel,
  }
}
