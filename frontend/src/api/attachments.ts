import * as WailsApp from '../../wailsjs/go/main/App'

export type NoteAttachment = {
  id: string
  noteId: string
  kind: string
  mimeType: string
  name: string
  size: number
  sha256: string
  width: number
  height: number
  createdAt: string
  reference: string
}

export type SaveNoteAttachmentInput = {
  noteId: string
  kind: 'image'
  mimeType: 'image/png' | 'image/jpeg'
  name: string
  data: string
}

export type GetNoteAttachmentInput = {
  noteId: string
  attachmentId: string
}

export type NoteAttachmentData = {
  attachment: NoteAttachment
  data: string
}

export type SaveNoteAttachmentsResult = {
  saved: boolean
  cancelled: boolean
  savedName?: string
  error?: string
}

type SaveNoteAttachmentMethod = (input: SaveNoteAttachmentInput) => Promise<NoteAttachment>
type GetNoteAttachmentMethod = (input: GetNoteAttachmentInput) => Promise<NoteAttachmentData>
type SaveNoteAttachmentsMethod = (
  noteId: string,
  title: string,
  expectedRevision: number,
  allowPlaintextProtected: boolean,
) => Promise<SaveNoteAttachmentsResult>

const saveNoteAttachmentMethod = (WailsApp as unknown as {
  SaveNoteAttachment: SaveNoteAttachmentMethod
}).SaveNoteAttachment
const getNoteAttachmentMethod = (WailsApp as unknown as {
  GetNoteAttachment: GetNoteAttachmentMethod
}).GetNoteAttachment
const saveNoteAttachmentsMethod = (WailsApp as unknown as {
  SaveNoteAttachments: SaveNoteAttachmentsMethod
}).SaveNoteAttachments

export function saveNoteAttachment(input: SaveNoteAttachmentInput) {
  return saveNoteAttachmentMethod(input)
}

export function getNoteAttachment(input: GetNoteAttachmentInput) {
  return getNoteAttachmentMethod(input)
}

export function saveNoteAttachments(
  noteId: string,
  title: string,
  expectedRevision: number,
  allowPlaintextProtected: boolean,
) {
  return saveNoteAttachmentsMethod(noteId, title, expectedRevision, allowPlaintextProtected)
}
