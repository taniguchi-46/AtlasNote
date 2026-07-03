import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { listNotes, getNote, createNote, updateNote, deleteNote } from '../api/notes'

export const useNoteStore = defineStore('notes', () => {
  // State
  const summaries = ref<note.Summary[]>([])
  const activeNote = ref<note.Note | null>(null)
  const isLoading = ref(false)
  const isSaving = ref(false)
  const error = ref<string | null>(null)

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
      summaries.value = await listNotes()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
    } finally {
      isLoading.value = false
    }
  }

  async function selectNote(id: string) {
    isLoading.value = true
    error.value = null
    try {
      activeNote.value = await getNote(id)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
    } finally {
      isLoading.value = false
    }
  }

  async function newNote(title = '新しいノート', content = '') {
    isSaving.value = true
    error.value = null
    try {
      const created = await createNote({ title, content })
      summaries.value.unshift({
        id: created.id,
        notebookId: created.notebookId,
        title: created.title,
        isFavorite: created.isFavorite,
        isPinned: created.isPinned,
        isTrashed: created.isTrashed,
        createdAt: created.createdAt,
        updatedAt: created.updatedAt,
      } as note.Summary)
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
      activeNote.value = updated
      const idx = summaries.value.findIndex((n: note.Summary) => n.id === id)
      if (idx !== -1) {
        summaries.value[idx] = {
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
    permanentlyDeleteNote,
    toggleFavorite,
    togglePinned,
  }
})
