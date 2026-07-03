import type { note } from '../../wailsjs/go/models'
import {
  CreateNote,
  ListNotes,
  GetNote,
  UpdateNote,
  DeleteNote,
} from '../../wailsjs/go/main/App'

export type { note }

export function createNote(input: note.CreateInput): Promise<note.Note> {
  return CreateNote(input)
}

export function listNotes(): Promise<note.Summary[]> {
  return ListNotes()
}

export function getNote(id: string): Promise<note.Note> {
  return GetNote(id)
}

export function updateNote(id: string, input: note.UpdateInput): Promise<note.Note> {
  return UpdateNote(id, input)
}

export function deleteNote(id: string): Promise<void> {
  return DeleteNote(id)
}
