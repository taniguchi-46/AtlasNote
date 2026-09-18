export type ContentLockEditorBridge = {
  flushEditorInput: () => boolean
  setContentLockPending?: (pending: boolean) => boolean
}

export type ContentLockPreparation = {
  ok: true
  finish: () => void
}

export type ContentLockBeforeLockResult = boolean | ContentLockPreparation

export type ContentLockBeforeLockOptions = {
  automatic?: boolean
}

export function createContentLockBeforeLock(
  getEditor: () => ContentLockEditorBridge | null,
  flushAllDirtyNotes: (options?: ContentLockBeforeLockOptions) => Promise<boolean>,
) {
  return async (options: ContentLockBeforeLockOptions = {}): Promise<ContentLockBeforeLockResult> => {
    const editor = getEditor()
    let inputLockPending = false

    const releaseInputLock = () => {
      if (!inputLockPending) return
      inputLockPending = false
      try {
        editor?.setContentLockPending?.(false)
      } catch {
        // The editor may already be unmounted while a failed lock is being
        // unwound. The save/lock result remains the source of truth.
      }
    }

    try {
      if (editor && !editor.flushEditorInput()) return false

      if (editor?.setContentLockPending) {
        inputLockPending = true
        if (!editor.setContentLockPending(true)) {
          releaseInputLock()
          return false
        }
      }

      if (!(await flushAllDirtyNotes(options))) {
        releaseInputLock()
        return false
      }

      if (!inputLockPending) return true

      return {
        ok: true,
        finish: releaseInputLock,
      }
    } catch (error) {
      releaseInputLock()
      throw error
    }
  }
}
