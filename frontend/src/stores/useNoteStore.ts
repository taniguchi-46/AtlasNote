import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { listNotes, getNote, createNote, updateNote, deleteNote } from '../api/notes'
import { useSettingsStore, type EditorFirstLineStyle } from './useSettingsStore'

const DEFAULT_NOTE_TITLE = '新しいノート'

function createInitialNoteContent(firstLineStyle: EditorFirstLineStyle) {
  const markers: Record<EditorFirstLineStyle, string> = {
    heading1: '# ',
    heading2: '## ',
    heading3: '### ',
    paragraph: '',
  }

  return markers[firstLineStyle]
}

function toSummary(updated: note.Note): note.Summary {
  return {
    id: updated.id,
    notebookId: updated.notebookId,
    title: updated.title,
    isFavorite: updated.isFavorite,
    isPinned: updated.isPinned,
    isTrashed: updated.isTrashed,
    createdAt: updated.createdAt,
    updatedAt: updated.updatedAt,
  } as note.Summary
}

export const useNoteStore = defineStore('notes', () => {
  // State
  const summaries = ref<note.Summary[]>([])
  const activeNote = ref<note.Note | null>(null)
  const isLoading = ref(false)
  const isSaving = ref(false)
  const error = ref<string | null>(null)
  const autoTitleNoteId = ref<string | null>(null)

  // Computed
  const pinnedNotes = computed(() =>
    summaries.value.filter((n: note.Summary) => n.isPinned && !n.isTrashed)
  )
  const favoriteNotes = computed(() =>
    summaries.value.filter((n: note.Summary) => n.isFavorite && !n.isTrashed)
  )
  const trashedNotes = computed(() =>
    summaries.value.filter((n: note.Summary) => n.isTrashed)
  )
  const activeNotes = computed(() =>
    summaries.value.filter((n: note.Summary) => !n.isTrashed)
  )

  // Actions
  async function fetchNotes() {
    isLoading.value = true
    error.value = null
    try {
      summaries.value = (await listNotes()) ?? []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
    } finally {
      isLoading.value = false
    }
  }

  async function selectNote(id: string) {
    isLoading.value = true
    error.value = null
    autoTitleNoteId.value = null
    try {
      activeNote.value = await getNote(id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
    } finally {
      isLoading.value = false
    }
  }

  async function newNote(title = DEFAULT_NOTE_TITLE, content = '', notebookId: string | null = null) {
    isSaving.value = true
    error.value = null
    try {
      const settingsStore = useSettingsStore()
      const initialTitle = title.trim() || DEFAULT_NOTE_TITLE
      const shouldCreateInitialContent = !content.trim()
      const initialContent = shouldCreateInitialContent
        ? createInitialNoteContent(settingsStore.editorFirstLineStyle)
        : content
      const created = await createNote({
        title: initialTitle,
        content: initialContent,
        ...(notebookId ? { notebookId } : {}),
      })
      if (!summaries.value) {
        summaries.value = []
      }
      summaries.value.unshift(toSummary(created))
      autoTitleNoteId.value = shouldCreateInitialContent ? created.id : null
      activeNote.value = created
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの作成に失敗しました'
    } finally {
      isSaving.value = false
    }
  }

  async function saveNote(id: string, input: note.UpdateInput) {
    isSaving.value = true
    error.value = null
    try {
      const updated = await updateNote(id, input)
      if (activeNote.value?.id === id) {
        activeNote.value = updated
      }
      const idx = summaries.value.findIndex((n: note.Summary) => n.id === id)
      if (idx !== -1) {
        summaries.value[idx] = toSummary(updated)
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの保存に失敗しました'
    } finally {
      isSaving.value = false
    }
  }

  async function trashNote(id: string) {
    await saveNote(id, { isTrashed: true })
  }

  async function restoreNote(id: string) {
    await saveNote(id, { isTrashed: false })
  }

  async function updateNotes(ids: string[], input: note.UpdateInput) {
    if (ids.length === 0) return

    isSaving.value = true
    error.value = null
    try {
      for (const id of ids) {
        const updated = await updateNote(id, input)
        if (activeNote.value?.id === id) {
          activeNote.value = updated
        }
        const idx = summaries.value.findIndex((n: note.Summary) => n.id === id)
        if (idx !== -1) {
          summaries.value[idx] = toSummary(updated)
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの一括更新に失敗しました'
      throw e
    } finally {
      isSaving.value = false
    }
  }

  async function trashNotes(ids: string[]) {
    await updateNotes(ids, { isTrashed: true })
  }

  async function restoreNotes(ids: string[]) {
    await updateNotes(ids, { isTrashed: false })
  }

  async function moveNotesToNotebook(ids: string[], notebookId: string | null) {
    await updateNotes(
      ids,
      notebookId ? { notebookId } : ({ clearNotebook: true } as note.UpdateInput),
    )
  }

  async function permanentlyDeleteNote(id: string) {
    error.value = null
    try {
      await deleteNote(id)
      summaries.value = summaries.value.filter((n: note.Summary) => n.id !== id)
      if (activeNote.value?.id === id) activeNote.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの削除に失敗しました'
    }
  }

  async function permanentlyDeleteNotes(ids: string[]) {
    if (ids.length === 0) return

    isSaving.value = true
    error.value = null
    try {
      for (const id of ids) {
        await deleteNote(id)
      }
      const idSet = new Set(ids)
      summaries.value = summaries.value.filter((n: note.Summary) => !idSet.has(n.id))
      if (activeNote.value && idSet.has(activeNote.value.id)) {
        activeNote.value = null
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの一括削除に失敗しました'
      throw e
    } finally {
      isSaving.value = false
    }
  }

  async function toggleFavorite(id: string) {
    const n = summaries.value.find((s: note.Summary) => s.id === id)
    if (!n) return
    await saveNote(id, { isFavorite: !n.isFavorite })
  }

  async function togglePinned(id: string) {
    const n = summaries.value.find((s: note.Summary) => s.id === id)
    if (!n) return
    await saveNote(id, { isPinned: !n.isPinned })
  }

  return {
    summaries,
    activeNote,
    isLoading,
    isSaving,
    error,
    autoTitleNoteId,
    pinnedNotes,
    favoriteNotes,
    trashedNotes,
    activeNotes,
    fetchNotes,
    selectNote,
    newNote,
    saveNote,
    trashNote,
    restoreNote,
    trashNotes,
    restoreNotes,
    moveNotesToNotebook,
    permanentlyDeleteNote,
    permanentlyDeleteNotes,
    toggleFavorite,
    togglePinned,
  }
})
