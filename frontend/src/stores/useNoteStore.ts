import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { listNotes, getNote, createNote, updateNote, deleteNote } from '../api/notes'
import { createLatestRequestGuard } from '../utils/latestRequestGuard'
import { createNoteAutoSave, type NoteSaveSnapshot } from '../utils/noteAutoSave'
import { deleteNotesSequentially, NoteDeleteError } from '../utils/deleteNotesSequentially'
import { useSettingsStore, type EditorFirstLineStyle } from './useSettingsStore'

const DEFAULT_NOTE_TITLE = '新しいノート'

export type NoteDraft = NoteSaveSnapshot & {
  status: 'dirty' | 'saving' | 'failed'
  error: string | null
}

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
  const drafts = ref<Record<string, NoteDraft>>({})
  const saveFeedbackVersion = ref(0)
  const lastSavedNoteId = ref<string | null>(null)
  let nextRevision = 0
  const noteSelectionRequests = createLatestRequestGuard()

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
  const activeDraft = computed(() => {
    const id = activeNote.value?.id
    return id ? drafts.value[id] ?? null : null
  })
  const hasDirtyNotes = computed(() => Object.keys(drafts.value).length > 0)

  function getDraft(noteId: string) {
    return drafts.value[noteId] ?? null
  }

  function replaceDraft(noteId: string, draft: NoteDraft | null) {
    const nextDrafts = { ...drafts.value }
    if (draft) {
      nextDrafts[noteId] = draft
    } else {
      delete nextDrafts[noteId]
    }
    // Vueのリアクティビティを確実にするため、オブジェクトのプロパティを直接変更するのではなく、
    // 新しいオブジェクトを丸ごと代入して状態を更新する。
    drafts.value = nextDrafts
  }

  // Actions
  async function fetchNotes(excludedIds: string[] = []) {
    isLoading.value = true
    error.value = null
    try {
      const excluded = new Set(excludedIds)
      summaries.value = ((await listNotes()) ?? []).filter((note) => !excluded.has(note.id))
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
    } finally {
      isLoading.value = false
    }
  }

  async function selectNote(id: string) {
    // ノートを連続で高速に切り替えた際、過去のリクエストのレスポンスが遅延して到着し、
    // 表示すべき最新のノートが古いノートで上書きされてしまう競合（レースコンディション）を防ぐ。
    // begin() で取得した isLatestRequest() が false を返す場合は処理を中断する。
    const isLatestRequest = noteSelectionRequests.begin()
    await flushPendingDraft()
    if (!isLatestRequest()) return

    isLoading.value = true
    error.value = null
    autoTitleNoteId.value = null
    try {
      const selectedNote = await getNote(id)
      if (isLatestRequest()) {
        activeNote.value = selectedNote
      }
    } catch (e) {
      if (isLatestRequest()) {
        error.value = e instanceof Error ? e.message : 'ノートの読み込みに失敗しました'
      }
    } finally {
      if (isLatestRequest()) {
        isLoading.value = false
      }
    }
  }

  async function newNote(title = DEFAULT_NOTE_TITLE, content = '', notebookId: string | null = null) {
    await flushPendingDraft()
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

  function applyPersistedNote(updated: note.Note, applyToActiveNote = true) {
    if (applyToActiveNote && activeNote.value?.id === updated.id) {
      activeNote.value = updated
    }
    const idx = summaries.value.findIndex((n: note.Summary) => n.id === updated.id)
    if (idx !== -1) {
      summaries.value[idx] = toSummary(updated)
    }
  }

  async function persistNote(id: string, input: note.UpdateInput) {
    isSaving.value = true
    error.value = null
    try {
      return await updateNote(id, input)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの保存に失敗しました'
      return null
    } finally {
      isSaving.value = false
    }
  }

  async function persistDraftSnapshot(snapshot: NoteSaveSnapshot) {
    const current = getDraft(snapshot.noteId)
    if (current?.revision === snapshot.revision) {
      replaceDraft(snapshot.noteId, { ...current, status: 'saving', error: null })
    }

    return persistNote(snapshot.noteId, {
      title: snapshot.title,
      content: snapshot.content,
    })
  }

  const autoSave = createNoteAutoSave<note.Note>({
    delayMs: 1000,
    save: persistDraftSnapshot,
    shouldApply: (snapshot) => getDraft(snapshot.noteId)?.revision === snapshot.revision,
    isCurrent: (snapshot) => getDraft(snapshot.noteId)?.revision === snapshot.revision,
    applyResult: (snapshot, updated) => {
      const applyToActiveNote = activeNote.value?.id === snapshot.noteId
      applyPersistedNote(updated, applyToActiveNote)
    },
    onSaved: (snapshot) => {
      if (getDraft(snapshot.noteId)?.revision !== snapshot.revision) return

      replaceDraft(snapshot.noteId, null)
      lastSavedNoteId.value = snapshot.noteId
      saveFeedbackVersion.value += 1
    },
    onFailed: (snapshot) => {
      const current = getDraft(snapshot.noteId)
      if (current?.revision !== snapshot.revision) return

      replaceDraft(snapshot.noteId, {
        ...current,
        status: 'failed',
        error: error.value ?? 'ノートの保存に失敗しました',
      })
    },
  })

  function scheduleDraft(noteId: string, title: string, content: string) {
    // ユーザーの入力ごとに毎回バックエンドAPI（DBおよびファイルシステム）へ保存リクエストを送ると、
    // 通信量やディスクI/Oが過剰になりパフォーマンスが低下する。
    // そのため、入力を一旦ドラフト（dirty状態）としてメモリ上に保持し、
    // autoSave によって一定時間（delayMs）経過後にまとめてバックエンドへ書き込む（デバウンス処理）。
    const snapshot: NoteSaveSnapshot = {
      noteId,
      title,
      content,
      revision: ++nextRevision,
    }
    replaceDraft(noteId, { ...snapshot, status: 'dirty', error: null })
    autoSave.schedule(snapshot)
    return snapshot
  }

  async function flushPendingDraft() {
    return autoSave.flush()
  }

  async function retryDraftSave(noteId: string) {
    const draft = getDraft(noteId)
    if (!draft) return true

    replaceDraft(noteId, { ...draft, status: 'dirty', error: null })
    autoSave.schedule(draft)
    await autoSave.flush()
    return getDraft(noteId) === null
  }

  async function flushAllDirtyNotes() {
    await autoSave.flush()

    for (const draft of Object.values(drafts.value)) {
      const current = getDraft(draft.noteId)
      if (!current || current.revision !== draft.revision) continue

      autoSave.schedule(current)
      await autoSave.flush()
    }

    return Object.keys(drafts.value).length === 0
  }

  function discardDraft(noteId: string) {
    replaceDraft(noteId, null)
  }

  function discardAllDrafts() {
    autoSave.cancel()
    drafts.value = {}
  }

  async function saveNote(id: string, input: note.UpdateInput) {
    const updated = await persistNote(id, input)
    if (!updated) return false

    applyPersistedNote(updated)
    return true
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
      discardDraft(id)
      summaries.value = summaries.value.filter((n: note.Summary) => n.id !== id)
      if (activeNote.value?.id === id) activeNote.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'ノートの削除に失敗しました'
      throw e
    }
  }

  async function permanentlyDeleteNotes(ids: string[]) {
    if (ids.length === 0) return

    isSaving.value = true
    error.value = null
    let deletedIds: string[] = []
    try {
      deletedIds = await deleteNotesSequentially(ids, deleteNote)
    } catch (e) {
      if (e instanceof NoteDeleteError) deletedIds = e.deletedIds
      error.value = e instanceof Error ? e.message : 'ノートの一括削除に失敗しました'
      throw e
    } finally {
      const idSet = new Set(deletedIds)
      deletedIds.forEach(discardDraft)
      summaries.value = summaries.value.filter((n: note.Summary) => !idSet.has(n.id))
      if (activeNote.value && idSet.has(activeNote.value.id)) {
        activeNote.value = null
      }
      isSaving.value = false
    }
  }

  async function emptyTrash() {
    const ids = trashedNotes.value.map((n: note.Summary) => n.id)
    await permanentlyDeleteNotes(ids)
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
    drafts,
    activeDraft,
    hasDirtyNotes,
    saveFeedbackVersion,
    lastSavedNoteId,
    pinnedNotes,
    favoriteNotes,
    trashedNotes,
    activeNotes,
    fetchNotes,
    selectNote,
    newNote,
    persistNote,
    applyPersistedNote,
    getDraft,
    scheduleDraft,
    flushPendingDraft,
    flushAllDirtyNotes,
    retryDraftSave,
    discardDraft,
    discardAllDrafts,
    saveNote,
    trashNote,
    restoreNote,
    trashNotes,
    restoreNotes,
    moveNotesToNotebook,
    permanentlyDeleteNote,
    permanentlyDeleteNotes,
    emptyTrash,
    toggleFavorite,
    togglePinned,
  }
})
