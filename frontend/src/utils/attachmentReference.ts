export type ManagedAttachmentReference = {
  noteId: string
  attachmentId: string
}

const attachmentReferencePattern = /^atlasnote-attachment:\/\/([A-Za-z0-9_-]+)\/([A-Za-z0-9_-]+)$/

export function createAttachmentReference(noteId: string, attachmentId: string) {
  return `atlasnote-attachment://${noteId}/${attachmentId}`
}

export function parseAttachmentReference(value: string): ManagedAttachmentReference | null {
  const match = attachmentReferencePattern.exec(value)
  if (!match) return null
  return { noteId: match[1], attachmentId: match[2] }
}

export function isManagedAttachmentReference(value: string) {
  return parseAttachmentReference(value) !== null
}
