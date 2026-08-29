import * as WailsApp from '../../wailsjs/go/main/App'

export type NoteExportFormat = 'html' | 'pdf'

export type NoteExportInput = {
  noteId: string
  expectedRevision: number
  title: string
  markdown: string
  format: NoteExportFormat
  htmlFragment?: string
  pdfBase64?: string
  allowPlaintextProtected: boolean
}

export type NoteExportError = {
  code: string
  message: string
  retryable: boolean
}

export type NoteExportResult = {
  cancelled: boolean
  exportedName?: string
  error?: NoteExportError
}

type ExportNoteMethod = (input: NoteExportInput) => Promise<NoteExportResult>

const exportNoteMethod = (WailsApp as unknown as { ExportNote: ExportNoteMethod }).ExportNote

export function exportNote(input: NoteExportInput): Promise<NoteExportResult> {
  return exportNoteMethod(input)
}
