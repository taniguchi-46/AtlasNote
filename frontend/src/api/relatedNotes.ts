import { RelatedNotes } from '../../wailsjs/go/app/App'
import type { note } from '../../wailsjs/go/models'

export function relatedNotes(input: note.RelatedNoteInput): Promise<note.RelatedNoteResult> {
  return RelatedNotes(input)
}
