import type { note } from '../../wailsjs/go/models'
import { SearchNotes } from '../../wailsjs/go/main/App'

export type { note }

export function searchNotes(input: note.SearchInput): Promise<note.SearchResult> {
	return SearchNotes(input)
}
