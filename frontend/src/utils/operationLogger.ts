export type OperationLogContext = {
  operationId?: string
  noteId?: string
  stage: string
  errorCategory: string
}

type OperationFailureReporter = (context: {
  stage: string
  errorCategory: string
}) => Promise<unknown>

let operationFailureReporter: OperationFailureReporter | null = null

const KNOWN_STAGES = new Set([
  'app-close',
  'note-editor.ai-summary-copy',
  'note-editor.agent-highlight',
  'note-editor.flush-before-lock',
  'note-editor.image-paste',
  'note-editor.markdown-to-rich',
  'note-editor.table-copy',
  'note-editor.attachments-export',
  'note-export.markdown-to-html',
  'note-export.pdf-render',
  'wails.toggle-always-on-top',
])

const KNOWN_ERROR_CATEGORIES = new Set([
  'flush-or-close',
  'parse-failed',
  'render-failed',
  'rich-content-snapshot-failed',
  'runtime',
  'paste-failed',
])

export function setOperationFailureReporter(reporter: OperationFailureReporter | null) {
  operationFailureReporter = reporter
}

// Keep diagnostics deliberately metadata-only. Never pass the original Error,
// Markdown body, title, or request payload to the logger.
export function logOperationFailure(context: OperationLogContext) {
  const stage = KNOWN_STAGES.has(context.stage) ? context.stage : 'unknown-stage'
  const errorCategory = KNOWN_ERROR_CATEGORIES.has(context.errorCategory)
    ? context.errorCategory
    : 'unknown-category'
  console.error('operation failed', {
    ...(context.operationId ? { operationId: context.operationId } : {}),
    ...(context.noteId ? { noteId: context.noteId } : {}),
    stage,
    errorCategory,
  })
  if (operationFailureReporter) {
    void operationFailureReporter({ stage, errorCategory }).catch(() => {
      // Diagnostics are best-effort and must not create a second failure.
    })
  }
}
