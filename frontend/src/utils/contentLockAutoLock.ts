import type { ContentLock, ContentLockTarget } from '../api/contentLocks'

const MAX_TIMER_DELAY_MS = 2_147_483_647

type TimerHandle = ReturnType<typeof setTimeout>

export type ContentLockAutoLockSnapshot = {
  minutes: number
  locks: ContentLock[]
  unlockedAt: Record<string, number>
}

export type ContentLockAutoLockOptions = {
  now?: () => number
  setTimer?: (callback: () => void, delayMs: number) => TimerHandle
  clearTimer?: (timer: TimerHandle) => void
  retryDelayMs?: number
}

type ScheduledLock = {
  target: ContentLockTarget
  unlockedAt: number
}

function targetKey(target: ContentLockTarget) {
  return `${target.type}:${target.id}`
}

// The scheduler deliberately knows nothing about keyboard, mouse, focus, or
// editor activity. A deadline is always measured from the successful unlock.
export function createContentLockAutoLock(
  onDue: (targets: ContentLockTarget[]) => Promise<boolean>,
  options: ContentLockAutoLockOptions = {},
) {
  const now = options.now ?? (() => Date.now())
  const setTimer = options.setTimer ?? ((callback, delayMs) => setTimeout(callback, delayMs))
  const clearTimer = options.clearTimer ?? ((timer) => clearTimeout(timer))
  const retryDelayMs = options.retryDelayMs ?? 10_000

  let minutes = 0
  let scheduledLocks = new Map<string, ScheduledLock>()
  let timer: TimerHandle | null = null
  let retryAt: number | null = null
  let isLocking = false
  const completedTargets = new Set<string>()

  function cancelTimer() {
    if (timer === null) return
    clearTimer(timer)
    timer = null
  }

  function deadline(lock: ScheduledLock) {
    return lock.unlockedAt + minutes * 60_000
  }

  function arm() {
    cancelTimer()
    if (minutes <= 0 || isLocking) return

    const candidates = Array.from(scheduledLocks.entries())
      .filter(([key]) => !completedTargets.has(key))
    if (candidates.length === 0) return

    const nextDeadline = Math.min(...candidates.map(([, lock]) => deadline(lock)))
    const nextAt = retryAt ?? nextDeadline
    const delayMs = Math.min(MAX_TIMER_DELAY_MS, Math.max(0, nextAt - now()))
    timer = setTimer(() => { void check() }, delayMs)
  }

  async function check() {
    cancelTimer()
    if (minutes <= 0 || isLocking) return

    const currentTime = now()
    if (retryAt !== null && currentTime < retryAt) {
      arm()
      return
    }
    retryAt = null
    const due = Array.from(scheduledLocks.entries())
      .filter(([key, lock]) => !completedTargets.has(key) && currentTime >= deadline(lock))
      .map(([, lock]) => ({ ...lock.target }))
    if (due.length === 0) {
      arm()
      return
    }

    isLocking = true
    let completed = false
    try {
      completed = await onDue(due)
    } catch {
      completed = false
    } finally {
      isLocking = false
    }

    if (completed) {
      for (const target of due) completedTargets.add(targetKey(target))
    } else {
      // A failed save must not discard an in-memory key. Keep the original
      // deadline and retry shortly without treating user activity as an unlock.
      retryAt = now() + retryDelayMs
    }
    arm()
  }

  function update(snapshot: ContentLockAutoLockSnapshot) {
    minutes = snapshot.minutes
    retryAt = null
    const previousLocks = scheduledLocks
    const next = new Map<string, ScheduledLock>()
    for (const lock of snapshot.locks) {
      if (!lock.unlocked) continue
      const target: ContentLockTarget = { type: lock.targetType, id: lock.targetId }
      const unlockedAt = snapshot.unlockedAt[targetKey(target)]
      if (!Number.isFinite(unlockedAt)) continue
      next.set(targetKey(target), { target, unlockedAt })
    }
    scheduledLocks = next
    for (const [key, lock] of scheduledLocks) {
      if (previousLocks.get(key)?.unlockedAt !== lock.unlockedAt) {
        completedTargets.delete(key)
      }
    }
    for (const key of completedTargets) {
      if (!scheduledLocks.has(key)) completedTargets.delete(key)
    }
    arm()
  }

  function dispose() {
    cancelTimer()
    scheduledLocks.clear()
    completedTargets.clear()
    retryAt = null
  }

  return { update, check, dispose }
}
