import { NoteBatchError, runSequentially } from './noteBatch'

export class NoteDeleteError extends NoteBatchError {
  public readonly deletedIds: string[]

  constructor(message: string, deletedIds: string[], failedId: string) {
    super(message, deletedIds, failedId)
    this.name = 'NoteDeleteError'
    this.deletedIds = deletedIds
  }
}

export async function deleteNotesSequentially(
  ids: string[],
  deleteOne: (id: string) => Promise<void>,
): Promise<string[]> {
  try {
    return await runSequentially(ids, deleteOne)
  } catch (cause) {
    if (cause instanceof NoteBatchError) {
      throw new NoteDeleteError(cause.message, cause.completedIds, cause.failedId)
    }
    throw cause
  }
}
