import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { listNotebooks, createNotebook, updateNotebook, deleteNotebook, type NotebookDeleteMode } from '../api/notebooks'
import { DEFAULT_NOTEBOOK_ICON } from '../utils/notebookIcons'
import { useNoteStore } from './useNoteStore'

export interface NotebookNode extends note.Notebook {
	children: NotebookNode[]
}

export const useNotebookStore = defineStore('notebooks', () => {
	const notebooks = ref<note.Notebook[]>([])
	const activeNotebookId = ref<string | null>(null)
	const isLoading = ref(false)
	const error = ref<string | null>(null)

	const notebookTree = computed(() => {
		const map = new Map<string, NotebookNode>()
		const roots: NotebookNode[] = []

		// First pass: create nodes
		notebooks.value.forEach(nb => {
			map.set(nb.id, { ...nb, children: [] })
		})

		// Second pass: associate children
		map.forEach(node => {
			if (node.parentId) {
				const parent = map.get(node.parentId)
				if (parent) {
					parent.children.push(node)
				} else {
					roots.push(node)
				}
			} else {
				roots.push(node)
			}
		})

		return roots
	})

	async function fetchNotebooks() {
		isLoading.value = true
		error.value = null
		try {
			notebooks.value = (await listNotebooks()) ?? []
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックの読み込みに失敗しました'
		} finally {
			isLoading.value = false
		}
	}

	async function newNotebook(name: string, parentId: string | null = null, icon = DEFAULT_NOTEBOOK_ICON) {
		error.value = null
		try {
			const nb = await createNotebook({
				name,
				icon,
				...(parentId ? { parentId } : {}),
			})
			if (!notebooks.value) {
				notebooks.value = []
			}
			notebooks.value.push(nb)
			return nb
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックの作成に失敗しました'
			throw e
		}
	}

	async function renameNotebook(id: string, name: string) {
		error.value = null
		try {
			const updated = await updateNotebook(id, { name })
			const idx = notebooks.value.findIndex(n => n.id === id)
			if (idx !== -1) {
				notebooks.value[idx] = updated
			}
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックの更新に失敗しました'
		}
	}

	async function moveNotebook(id: string, parentId: string | null) {
		error.value = null
		try {
			const updated = await updateNotebook(
				id,
				parentId ? { parentId } : ({ clearParent: true } as note.NotebookUpdateInput)
			)
			const idx = notebooks.value.findIndex(n => n.id === id)
			if (idx !== -1) {
				notebooks.value[idx] = updated
			}
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックの移動に失敗しました'
		}
	}

	async function removeNotebook(id: string, mode: NotebookDeleteMode) {
		error.value = null
		try {
			await deleteNotebook(id, mode)
			const idsToRemove = mode === 'trashNotes'
				? collectNotebookDescendantIds(id)
				: new Set<string>([id])
			notebooks.value = notebooks.value
				.filter(n => !idsToRemove.has(n.id))
				.map(n => n.parentId === id ? { ...n, parentId: undefined } : n)
			if (activeNotebookId.value && idsToRemove.has(activeNotebookId.value)) {
				activeNotebookId.value = null
			}
			await useNoteStore().fetchNotes()
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックの削除に失敗しました'
			throw e
		}
	}

	function collectNotebookDescendantIds(id: string): Set<string> {
		const ids = new Set<string>([id])
		let changed = true

		while (changed) {
			changed = false
			notebooks.value.forEach(nb => {
				if (nb.parentId && ids.has(nb.parentId) && !ids.has(nb.id)) {
					ids.add(nb.id)
					changed = true
				}
			})
		}

		return ids
	}

	async function updateNotebookIcon(id: string, icon: string) {
		error.value = null
		try {
			const updated = await updateNotebook(id, { icon })
			const index = notebooks.value.findIndex(n => n.id === id)
			if (index !== -1) {
				notebooks.value[index] = updated
			}
		} catch (e) {
			error.value = e instanceof Error ? e.message : 'ノートブックアイコンの更新に失敗しました'
		}
	}

	return {
		notebooks,
		activeNotebookId,
		isLoading,
		error,
		notebookTree,
		fetchNotebooks,
		newNotebook,
		renameNotebook,
		moveNotebook,
		removeNotebook,
		updateNotebookIcon,
	}
})
