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

type DeleteNotebookMethod = (
	id: string,
	input: note.NotebookDeleteInput,
) => Promise<note.NotebookDeleteResult>

const deleteNotebookRPC = DeleteNotebook as unknown as DeleteNotebookMethod

export class NotebookDeleteApiError extends Error {
	readonly code: string
	readonly retryable: boolean

	constructor(error: note.NotebookDeleteError) {
		super(error.message)
		this.name = 'NotebookDeleteApiError'
		this.code = error.code
		this.retryable = error.retryable
	}
}

export async function deleteNotebook(id: string, mode: NotebookDeleteMode): Promise<void> {
	const result = await deleteNotebookRPC(id, { mode })
	if (result.error) throw new NotebookDeleteApiError(result.error)
	if (!result.deleted) throw new Error('ノートブック削除APIから結果が返されませんでした')
}
