import { getNoteAttachment } from '../api/attachments'
import { parseAttachmentReference } from './attachmentReference'

export async function createAttachmentObjectURL(reference: string, expectedNoteId?: string): Promise<string | null> {
  const parsed = parseAttachmentReference(reference)
  if (!parsed) return null
  if (expectedNoteId && parsed.noteId !== expectedNoteId) return null
  const result = await getNoteAttachment(parsed)
  const binary = atob(result.data)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return URL.createObjectURL(new Blob([bytes], { type: result.attachment.mimeType }))
}
