import { ImportNotes } from '../../wailsjs/go/main/App'

export type NoteImportTitleMode = 'auto' | 'filename' | 'heading' | 'metadata'

export type NoteImportInput = {
  notebookId?: string
  newNotebookName?: string
  titleMode?: NoteImportTitleMode
}

export type NoteImportError = {
  code: string
  message: string
  retryable: boolean
}

export type ImportedNote = {
  sourceName: string
  noteId: string
  title: string
}

export type NoteImportFailure = {
  sourceName: string
  code: string
  message: string
}

export type NoteImportResult = {
  cancelled: boolean
  imported: ImportedNote[]
  failures: NoteImportFailure[]
  createdNotebook?: {
    id: string
    name: string
  }
  error?: NoteImportError
}

export function importNotes(input: NoteImportInput): Promise<NoteImportResult> {
  return ImportNotes(input) as Promise<NoteImportResult>
}
