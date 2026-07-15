import type { note } from '../../wailsjs/go/models'
import { ListBacklinks } from '../../wailsjs/go/main/App'
import { searchNotes } from './search'

export type { note }

export function listBacklinks(input: note.BacklinkListInput): Promise<note.BacklinkListResult> {
  return ListBacklinks(input)
}

export function searchNoteLinkTargets(query: string): Promise<note.SearchResult> {
  return searchNotes({
    query,
    scope: 'title',
    includeTrashed: false,
    page: 1,
    pageSize: 20,
  } as note.SearchInput)
}
