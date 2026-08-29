export class NoteBatchError extends Error {
  constructor(
    message: string,
    public readonly completedIds: string[],
    public readonly failedId: string,
  ) {
    super(message)
    this.name = 'NoteBatchError'
  }
}

export async function runSequentially(
  ids: string[],
  operation: (id: string) => Promise<void>,
): Promise<string[]> {
  const completedIds: string[] = []

  for (const id of ids) {
    try {
      await operation(id)
      completedIds.push(id)
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '一括操作に失敗しました'
      throw new NoteBatchError(message, completedIds, id)
    }
  }

  return completedIds
}
