import { NoteBatchError, runSequentially } from './noteBatch'

export class NoteUpdateError extends NoteBatchError {
  public readonly updatedIds: string[]

  constructor(message: string, updatedIds: string[], failedId: string) {
    super(message, updatedIds, failedId)
    this.name = 'NoteUpdateError'
    this.updatedIds = updatedIds
  }
}

export async function updateNotesSequentially(
  ids: string[],
  updateOne: (id: string) => Promise<void>,
): Promise<string[]> {
  try {
    return await runSequentially(ids, updateOne)
  } catch (cause) {
    if (cause instanceof NoteBatchError) {
      throw new NoteUpdateError(cause.message, cause.completedIds, cause.failedId)
    }
    throw cause
  }
}
