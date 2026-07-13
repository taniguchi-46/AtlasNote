import type { note } from '../../wailsjs/go/models'
import {
	CreateTag,
	DeleteTag,
	ListNoteTags,
	ListTags,
	SetNoteTags,
	UpdateTag,
} from '../../wailsjs/go/main/App'

export type { note }

export class TagApiError extends Error {
	readonly code: string
	readonly field?: string
	readonly retryable: boolean

	constructor(error: note.TagError) {
		super(error.message)
		this.name = 'TagApiError'
		this.code = error.code
		this.field = error.field
		this.retryable = error.retryable
	}
}

function throwIfTagError(error?: note.TagError) {
	if (error) throw new TagApiError(error)
}

export async function listTags(): Promise<note.Tag[]> {
	return (await ListTags()) ?? []
}

export async function listNoteTags(noteId: string): Promise<note.Tag[]> {
	const result = await ListNoteTags(noteId)
	throwIfTagError(result.error)
	return result.tags ?? []
}

export async function createTag(input: note.TagCreateInput): Promise<note.Tag> {
	const result = await CreateTag(input)
	throwIfTagError(result.error)
	if (!result.tag) throw new Error('タグ作成APIから結果が返されませんでした')
	return result.tag
}

export async function updateTag(id: string, input: note.TagUpdateInput): Promise<note.Tag> {
	const result = await UpdateTag(id, input)
	throwIfTagError(result.error)
	if (!result.tag) throw new Error('タグ更新APIから結果が返されませんでした')
	return result.tag
}

export async function deleteTag(id: string): Promise<void> {
	const result = await DeleteTag(id)
	throwIfTagError(result.error)
	if (!result.deleted) throw new Error('タグ削除APIから結果が返されませんでした')
}

export async function setNoteTags(noteId: string, input: note.SetNoteTagsInput): Promise<note.Tag[]> {
	const result = await SetNoteTags(noteId, input)
	throwIfTagError(result.error)
	return result.tags ?? []
}
