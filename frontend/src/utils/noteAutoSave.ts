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
      return
    }

    if (options.shouldApply(snapshot)) {
      options.applyResult(snapshot, result)
    }
    if (options.isCurrent(snapshot)) {
      options.onSaved?.(snapshot)
    }
  }

  async function runPending() {
    cancelTimer()
    const snapshot = pendingSnapshot
    pendingSnapshot = null
    if (!snapshot) return

    await saveSnapshot(snapshot)
  }

  function schedule(snapshot: NoteSaveSnapshot) {
    cancelTimer()
    pendingSnapshot = snapshot
    timer = setTimer(() => {
      void runPending()
    }, options.delayMs)
  }

  function flush() {
    return runPending()
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
