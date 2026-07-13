export type OperationLogContext = {
  operationId?: string
  noteId?: string
  stage: string
  errorCategory: string
}

// Keep diagnostics deliberately metadata-only. Never pass the original Error,
// Markdown body, title, or request payload to the logger.
export function logOperationFailure(context: OperationLogContext) {
  console.error('operation failed', {
    ...(context.operationId ? { operationId: context.operationId } : {}),
    ...(context.noteId ? { noteId: context.noteId } : {}),
    stage: context.stage,
    errorCategory: context.errorCategory,
  })
}
