import type { note } from '../../wailsjs/go/models'
import {
  CreateNote,
  ListNotes,
  ListNotesPage,
  GetNote,
  UpdateNote,
  DeleteNote,
} from '../../wailsjs/go/main/App'

export type { note }

export class NoteRevisionConflictError extends Error {
  readonly code: string
  readonly noteId: string
  readonly expectedRevision: number
  readonly actualRevision: number

  constructor(conflict: note.RevisionConflict) {
    super('ノートが別の更新によって変更されています')
    this.name = 'NoteRevisionConflictError'
    this.code = conflict.code
    this.noteId = conflict.noteId
    this.expectedRevision = conflict.expectedRevision
    this.actualRevision = conflict.actualRevision
  }
}

export function createNote(input: note.CreateInput): Promise<note.Note> {
  return CreateNote(input)
}

export function listNotes(): Promise<note.Summary[]> {
  return ListNotes()
}

export function listNotesPage(input: note.NoteListInput): Promise<note.NoteListResult> {
  return ListNotesPage(input)
}

export function getNote(id: string): Promise<note.Note> {
  return GetNote(id)
}

export async function updateNote(id: string, input: note.UpdateInput): Promise<note.Note> {
  const result = await UpdateNote(id, input)
  if (result.conflict) throw new NoteRevisionConflictError(result.conflict)
  if (!result.note) throw new Error('ノート更新APIから結果が返されませんでした')
  return result.note
}

export async function deleteNote(id: string, expectedRevision: number): Promise<void> {
  const result = await DeleteNote(id, { expectedRevision })
  if (result.conflict) throw new NoteRevisionConflictError(result.conflict)
  if (!result.deleted) throw new Error('ノート削除APIから結果が返されませんでした')
}
