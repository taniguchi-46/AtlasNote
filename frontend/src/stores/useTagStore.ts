import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import type { note } from '../../wailsjs/go/models'
import {
	TagApiError,
	createTag as createTagRequest,
	deleteTag as deleteTagRequest,
	listNoteTags,
	listTags,
	setNoteTags as setNoteTagsRequest,
	updateTag as updateTagRequest,
} from '../api/tags'
import { useNotificationStore, type NotificationAction } from './useNotificationStore'

type TagErrorContext = {
	code: string
	action?: NotificationAction
}

export const useTagStore = defineStore('tags', () => {
	const tags = ref<note.Tag[]>([])
	const activeNoteTags = ref<note.Tag[]>([])
	const activeNoteId = ref<string | null>(null)
	const activeNoteTagsReady = ref(false)
	const isLoading = ref(false)
	const isMutating = ref(false)
	const error = ref<string | null>(null)
	const notificationStore = useNotificationStore()
	const errorContext = ref<TagErrorContext | null>(null)
	let noteTagsRequestVersion = 0

	watch(error, (message) => {
		if (!message) {
			notificationStore.dismissBySource('tags')
			return
		}

		const context = errorContext.value ?? { code: 'TAG_OPERATION_FAILED' }
		notificationStore.notify(message, {
			kind: 'error',
			source: 'tags',
			code: context.code,
			retryable: Boolean(context.action),
			action: context.action,
			dedupeKey: `tags:${context.code}`,
		})
	}, { flush: 'sync' })

	function publishError(cause: unknown, fallbackCode: string, fallbackMessage: string, action?: NotificationAction) {
		const tagError = cause instanceof TagApiError ? cause : null
		errorContext.value = {
			code: tagError?.code ?? fallbackCode,
			action: tagError?.retryable === false ? undefined : action,
		}
		error.value = tagError?.message ?? (cause instanceof Error ? cause.message : fallbackMessage)
	}

	function sortTags(items: note.Tag[]) {
		return [...items].sort((left, right) => left.name.localeCompare(right.name, 'ja'))
	}

	async function fetchTags() {
		isLoading.value = true
		error.value = null
		try {
			tags.value = await listTags()
		} catch (cause) {
			publishError(cause, 'TAG_LIST_FAILED', 'タグの読み込みに失敗しました。', {
				label: '再試行',
				run: () => fetchTags(),
			})
		} finally {
			isLoading.value = false
		}
	}

	async function loadNoteTags(noteId: string) {
		const requestVersion = ++noteTagsRequestVersion
		activeNoteId.value = noteId
		activeNoteTags.value = []
		activeNoteTagsReady.value = false
		isLoading.value = true
		error.value = null
		try {
			const loadedTags = await listNoteTags(noteId)
			if (requestVersion === noteTagsRequestVersion) {
				activeNoteTags.value = loadedTags
				activeNoteTagsReady.value = true
			}
		} catch (cause) {
			if (requestVersion === noteTagsRequestVersion) {
				publishError(cause, 'NOTE_TAG_LIST_FAILED', 'ノートのタグ読み込みに失敗しました。', {
					label: '再試行',
					run: () => loadNoteTags(noteId),
				})
			}
		} finally {
			if (requestVersion === noteTagsRequestVersion) {
				isLoading.value = false
			}
		}
	}

	function clearActiveNoteTags() {
		noteTagsRequestVersion += 1
		activeNoteId.value = null
		activeNoteTags.value = []
		activeNoteTagsReady.value = false
	}

	function applyActiveNoteTags(updatedTags: note.Tag[]) {
		noteTagsRequestVersion += 1
		activeNoteTags.value = updatedTags
		activeNoteTagsReady.value = true
		isLoading.value = false
	}

	async function createTag(name: string) {
		isMutating.value = true
		error.value = null
		try {
			const created = await createTagRequest({ name })
			tags.value = sortTags([...tags.value, created])
			return created
		} catch (cause) {
			publishError(cause, 'TAG_CREATE_FAILED', 'タグの作成に失敗しました。', {
				label: '再試行',
				run: () => createTag(name),
			})
			throw cause
		} finally {
			isMutating.value = false
		}
	}

	async function renameTag(id: string, name: string) {
		isMutating.value = true
		error.value = null
		try {
			const updated = await updateTagRequest(id, { name })
			tags.value = sortTags(tags.value.map((tag) => tag.id === id ? updated : tag))
			applyActiveNoteTags(activeNoteTags.value.map((tag) => tag.id === id ? updated : tag))
			return updated
		} catch (cause) {
			publishError(cause, 'TAG_UPDATE_FAILED', 'タグの更新に失敗しました。', {
				label: '再試行',
				run: () => renameTag(id, name),
			})
			throw cause
		} finally {
			isMutating.value = false
		}
	}

	async function removeTag(id: string) {
		isMutating.value = true
		error.value = null
		try {
			await deleteTagRequest(id)
			tags.value = tags.value.filter((tag) => tag.id !== id)
			applyActiveNoteTags(activeNoteTags.value.filter((tag) => tag.id !== id))
		} catch (cause) {
			publishError(cause, 'TAG_DELETE_FAILED', 'タグの削除に失敗しました。', {
				label: '再試行',
				run: () => removeTag(id),
			})
			throw cause
		} finally {
			isMutating.value = false
		}
	}

	async function setTagsForNote(noteId: string, tagIds: string[]) {
		isMutating.value = true
		error.value = null
		try {
			const updatedTags = await setNoteTagsRequest(noteId, { tagIds })
			if (activeNoteId.value === noteId) {
				applyActiveNoteTags(updatedTags)
			}
			return updatedTags
		} catch (cause) {
			publishError(cause, 'NOTE_TAG_SET_FAILED', 'ノートのタグ更新に失敗しました。', {
				label: '再試行',
				run: () => setTagsForNote(noteId, tagIds),
			})
			throw cause
		} finally {
			isMutating.value = false
		}
	}

	async function attachTagToNote(noteId: string, tagId: string) {
		const currentTags = activeNoteId.value === noteId
			? activeNoteTags.value
			: await listNoteTags(noteId)
		if (currentTags.some((tag) => tag.id === tagId)) return currentTags

		return setTagsForNote(noteId, [...currentTags.map((tag) => tag.id), tagId])
	}

	async function detachTagFromNote(noteId: string, tagId: string) {
		const currentTags = activeNoteId.value === noteId
			? activeNoteTags.value
			: await listNoteTags(noteId)
		return setTagsForNote(noteId, currentTags.filter((tag) => tag.id !== tagId).map((tag) => tag.id))
	}

	return {
		tags,
		activeNoteTags,
		activeNoteId,
		activeNoteTagsReady,
		isLoading,
		isMutating,
		error,
		fetchTags,
		loadNoteTags,
		clearActiveNoteTags,
		createTag,
		renameTag,
		removeTag,
		setTagsForNote,
		attachTagToNote,
		detachTagFromNote,
	}
})
