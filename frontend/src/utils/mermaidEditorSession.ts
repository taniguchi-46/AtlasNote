export type MermaidEditorInputFlusher = () => boolean | void
export type MermaidEditorInputLocker = (locked: boolean) => boolean | void

export type MermaidEditorSessionStorage = {
  noteId?: string | null
  generation?: number
  mermaidEditorFlushers?: Set<MermaidEditorInputFlusher>
  mermaidEditorInputLockers?: Set<MermaidEditorInputLocker>
}

export function flushMermaidEditorInputs(
  storage: MermaidEditorSessionStorage | null | undefined,
) {
  const flushers = storage?.mermaidEditorFlushers
  if (!flushers) return true

  for (const flush of flushers) {
    try {
      if (flush() === false) return false
    } catch {
      return false
    }
  }

  return true
}

export function setMermaidEditorInputsLocked(
  storage: MermaidEditorSessionStorage | null | undefined,
  locked: boolean,
) {
  const lockers = storage?.mermaidEditorInputLockers
  if (!lockers) return true

  for (const setLocked of lockers) {
    try {
      if (setLocked(locked) === false) return false
    } catch {
      return false
    }
  }

  return true
}
