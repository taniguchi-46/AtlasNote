export function createNoteOperationQueue() {
  const tails = new Map<string, Promise<void>>()

  async function enqueue<Result>(noteId: string, operation: () => Promise<Result>) {
    const previous = tails.get(noteId) ?? Promise.resolve()
    const result = previous.catch(() => {}).then(operation)
    const tail = result.then(() => {}, () => {})
    tails.set(noteId, tail)

    try {
      return await result
    } finally {
      if (tails.get(noteId) === tail) tails.delete(noteId)
    }
  }

  async function flush(noteId?: string) {
    if (noteId) {
      await tails.get(noteId)
      return
    }

    while (tails.size > 0) {
      await Promise.all([...tails.values()])
    }
  }

  return { enqueue, flush }
}
