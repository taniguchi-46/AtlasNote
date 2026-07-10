import type { note } from '../../wailsjs/go/models'
import {
	CreateNotebook,
	ListNotebooks,
	UpdateNotebook,
	DeleteNotebook,
} from '../../wailsjs/go/main/App'

export type NotebookDeleteMode = 'trashNotes' | 'keepNotes'

export function createNotebook(input: note.NotebookCreateInput): Promise<note.Notebook> {
	return CreateNotebook(input)
}

export function listNotebooks(): Promise<note.Notebook[]> {
	return ListNotebooks()
}

export function updateNotebook(id: string, input: note.NotebookUpdateInput): Promise<note.Notebook> {
	return UpdateNotebook(id, input)
}

export function deleteNotebook(id: string, mode: NotebookDeleteMode): Promise<void> {
	return DeleteNotebook(id, { mode })
}
