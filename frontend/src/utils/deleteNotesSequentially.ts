export class NoteDeleteError extends Error {
  constructor(
    message: string,
    public readonly deletedIds: string[],
  ) {
    super(message)
    this.name = 'NoteDeleteError'
  }
}

export async function deleteNotesSequentially(
  ids: string[],
  deleteOne: (id: string) => Promise<void>,
): Promise<string[]> {
  const deletedIds: string[] = []

  for (const id of ids) {
    try {
      await deleteOne(id)
      deletedIds.push(id)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : 'ノートの削除に失敗しました'
      throw new NoteDeleteError(message, deletedIds)
    }
  }

  return deletedIds
}
