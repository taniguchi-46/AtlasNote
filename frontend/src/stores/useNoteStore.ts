import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { note } from '../../wailsjs/go/models'
import {
  listNotes,
  getNote,
  createNote,
  updateNote,
  deleteNote,
  NoteRevisionConflictError,
} from '../api/notes'
import { createLatestRequestGuard } from '../utils/latestRequestGuard'
import { createNoteAutoSave, type NoteSaveSnapshot } from '../utils/noteAutoSave'
import { createNoteOperationQueue } from '../utils/noteOperationQueue'
import { createRequestCounter } from '../utils/requestCounter'
import { deleteNotesSequentially, NoteDeleteError } from '../utils/deleteNotesSequentially'
import { updateNotesSequentially } from '../utils/updateNotesSequentially'
import { useSettingsStore, type EditorFirstLineStyle } from './useSettingsStore'
import { useNotificationStore, type NotificationAction } from './useNotificationStore'

const DEFAULT_NOTE_TITLE = '新しいノート'
const CONFLICT_COPY_SUFFIX = ' (競合コピー)'
const MAX_NOTE_TITLE_LENGTH = 200

export type NoteDraft = NoteSaveSnapshot & {
  status: 'dirty' | 'saving' | 'failed' | 'conflicted'
  error: string | null
  conflict: {
    code: string
    noteId: string
    expectedRevision: number
    actualRevision: number
  } | null
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

function createConflictCopyTitle(title: string) {
  const suffixLength = Array.from(CONFLICT_COPY_SUFFIX).length
  const baseTitle = Array.from(title.trim() || DEFAULT_NOTE_TITLE)
    .slice(0, MAX_NOTE_TITLE_LENGTH - suffixLength)
    .join('')
  return `${baseTitle}${CONFLICT_COPY_SUFFIX}`
}

function toSummary(updated: note.Note): note.Summary {
  return {
    id: updated.id,
    notebookId: updated.notebookId,
    title: updated.title,
    isFavorite: updated.isFavorite,
    isPinned: updated.isPinned,
    isTrashed: updated.isTrashed,
    revision: updated.revision,
    createdAt: updated.createdAt,
    updatedAt: updated.updatedAt,
  } as note.Summary
}

type NoteErrorContext = {
  code: string
  action?: NotificationAction
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
  let nextDraftVersion = 0
  const noteSelectionRequests = createLatestRequestGuard()
  const noteOperations = createNoteOperationQueue()
  const notificationStore = useNotificationStore()
  const errorContext = ref<NoteErrorContext | null>(null)
  const savingRequests = createRequestCounter((count) => {
    isSaving.value = count > 0
  })

  watch(error, (message) => {
    if (!message) {
      notificationStore.dismissBySource('notes')
      return
    }

    const context = errorContext.value ?? { code: 'NOTE_OPERATION_FAILED' }
    notificationStore.notify(message, {
      kind: 'error',
      source: 'notes',
      code: context.code,
      retryable: Boolean(context.action),
      action: context.action,
      dedupeKey: `notes:${context.code}`,
    })
  }, { flush: 'sync' })

  function setErrorContext(context: NoteErrorContext) {
    errorContext.value = context
  }

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
      setErrorContext({
        code: 'NOTE_LIST_FAILED',
        action: { label: '再試行', run: () => fetchNotes(excludedIds) },
      })
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
        setErrorContext({
          code: 'NOTE_LOAD_FAILED',
          action: { label: '再試行', run: () => selectNote(id) },
        })
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
    const endSaving = savingRequests.begin()
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
      setErrorContext({
        code: 'NOTE_CREATE_FAILED',
        action: {
          label: '再試行',
          run: () => newNote(title, content, notebookId),
        },
      })
      error.value = e instanceof Error ? e.message : 'ノートの作成に失敗しました'
    } finally {
      endSaving()
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

  function getPersistedRevision(noteId: string) {
    if (activeNote.value?.id === noteId) return activeNote.value.revision
    return summaries.value.find((note) => note.id === noteId)?.revision ?? null
  }

  function requirePersistedRevision(noteId: string) {
    const revision = getPersistedRevision(noteId)
    if (typeof revision !== 'number' || revision < 1) {
      throw new Error('ノートのrevisionを取得できません。再読み込みしてください')
    }
    return revision
  }

  async function persistNoteNow(
    id: string,
    input: note.UpdateInput,
    onFailure?: (failure: unknown) => void,
  ) {
    const endSaving = savingRequests.begin()
    error.value = null
    try {
      return await updateNote(id, {
        ...input,
        expectedRevision: requirePersistedRevision(id),
      })
    } catch (e) {
      setErrorContext({
        code: e instanceof NoteRevisionConflictError ? e.code : 'NOTE_SAVE_FAILED',
      })
      onFailure?.(e)
      error.value = e instanceof Error ? e.message : 'ノートの保存に失敗しました'
      return null
    } finally {
      endSaving()
    }
  }

  async function persistDraftSnapshot(snapshot: NoteSaveSnapshot) {
    const current = getDraft(snapshot.noteId)
    if (current?.draftVersion === snapshot.draftVersion) {
      replaceDraft(snapshot.noteId, { ...current, status: 'saving', error: null, conflict: null })
    }

    return persistNoteNow(snapshot.noteId, {
      title: snapshot.title,
      content: snapshot.content,
    }, (failure) => {
      const failedDraft = getDraft(snapshot.noteId)
      if (
        failedDraft?.draftVersion !== snapshot.draftVersion
        || !(failure instanceof NoteRevisionConflictError)
      ) return

      replaceDraft(snapshot.noteId, {
        ...failedDraft,
        status: 'conflicted',
        error: failure.message,
        conflict: {
          code: failure.code,
          noteId: failure.noteId,
          expectedRevision: failure.expectedRevision,
          actualRevision: failure.actualRevision,
        },
      })
    })
  }

  const autoSave = createNoteAutoSave<note.Note>({
    delayMs: 1000,
    save: persistDraftSnapshot,
    execute: noteOperations.enqueue,
    shouldApply: (snapshot) => getDraft(snapshot.noteId)?.draftVersion === snapshot.draftVersion,
    isCurrent: (snapshot) => getDraft(snapshot.noteId)?.draftVersion === snapshot.draftVersion,
    applyResult: (snapshot, updated) => {
      const applyToActiveNote = activeNote.value?.id === snapshot.noteId
      applyPersistedNote(updated, applyToActiveNote)
    },
    onSaved: (snapshot) => {
      if (getDraft(snapshot.noteId)?.draftVersion !== snapshot.draftVersion) return

      replaceDraft(snapshot.noteId, null)
      lastSavedNoteId.value = snapshot.noteId
      saveFeedbackVersion.value += 1
    },
    onFailed: (snapshot) => {
      const current = getDraft(snapshot.noteId)
      if (current?.draftVersion !== snapshot.draftVersion) return
      if (current.status === 'conflicted') return

      replaceDraft(snapshot.noteId, {
        ...current,
        status: 'failed',
        error: error.value ?? 'ノートの保存に失敗しました',
        conflict: null,
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
      draftVersion: ++nextDraftVersion,
    }
    const current = getDraft(noteId)
    if (current?.status === 'conflicted') {
      replaceDraft(noteId, {
        ...snapshot,
        status: 'conflicted',
        error: current.error,
        conflict: current.conflict,
      })
      return snapshot
    }
    if (current?.status === 'failed') {
      replaceDraft(noteId, {
        ...snapshot,
        status: 'failed',
        error: current.error,
        conflict: null,
      })
      return snapshot
    }

    replaceDraft(noteId, { ...snapshot, status: 'dirty', error: null, conflict: null })
    autoSave.schedule(snapshot)
    return snapshot
  }

  async function flushPendingDraft() {
    const noteId = activeNote.value?.id
    return noteId ? autoSave.flush(noteId) : true
  }

  async function retryDraftSave(noteId: string) {
    const draft = getDraft(noteId)
    if (!draft) return true
    if (draft.status === 'conflicted') return false

    replaceDraft(noteId, { ...draft, status: 'dirty', error: null, conflict: null })
    autoSave.retry(draft)
    await autoSave.flush(noteId)
    return getDraft(noteId) === null
  }

  async function flushAllDirtyNotes() {
    await autoSave.flush()

    for (const draft of Object.values(drafts.value)) {
      const current = getDraft(draft.noteId)
      if (!current || current.draftVersion !== draft.draftVersion) continue
      if (current.status === 'conflicted') continue

      autoSave.retry(current)
      await autoSave.flush(current.noteId)
    }

    return Object.keys(drafts.value).length === 0
  }

  function discardDraft(noteId: string) {
    replaceDraft(noteId, null)
  }

  async function reloadConflictedNote(noteId: string) {
    const draft = getDraft(noteId)
    if (draft?.status !== 'conflicted') return null

    const capturedDraftVersion = draft.draftVersion
    isLoading.value = true
    error.value = null
    try {
      const latestNote = await getNote(noteId)
      const current = getDraft(noteId)
      if (current?.status !== 'conflicted' || current.draftVersion !== capturedDraftVersion) {
        return null
      }

      applyPersistedNote(latestNote)
      replaceDraft(noteId, null)
      return latestNote
    } catch (e) {
      setErrorContext({
        code: 'NOTE_CONFLICT_RELOAD_FAILED',
        action: { label: '再試行', run: () => reloadConflictedNote(noteId) },
      })
      error.value = e instanceof Error ? e.message : 'ノートの再読み込みに失敗しました'
      return null
    } finally {
      isLoading.value = false
    }
  }

  async function copyConflictedDraft(noteId: string) {
    const draft = getDraft(noteId)
    if (draft?.status !== 'conflicted') return null

    const capturedDraftVersion = draft.draftVersion
    const sourceNote = activeNote.value?.id === noteId
      ? activeNote.value
      : summaries.value.find((summary) => summary.id === noteId)

    const endSaving = savingRequests.begin()
    error.value = null
    try {
      const created = await createNote({
        title: createConflictCopyTitle(draft.title),
        content: draft.content,
        ...(sourceNote?.notebookId ? { notebookId: sourceNote.notebookId } : {}),
      })
      summaries.value.unshift(toSummary(created))
      autoTitleNoteId.value = null
      activeNote.value = created

      const current = getDraft(noteId)
      if (current?.status === 'conflicted' && current.draftVersion === capturedDraftVersion) {
        replaceDraft(noteId, null)
      }
      return created
    } catch (e) {
      setErrorContext({
        code: 'NOTE_CONFLICT_COPY_FAILED',
        action: { label: '再試行', run: () => copyConflictedDraft(noteId) },
      })
      error.value = e instanceof Error ? e.message : '競合下書きのコピー保存に失敗しました'
      return null
    } finally {
      endSaving()
    }
  }

  async function persistNote(id: string, input: note.UpdateInput) {
    return noteOperations.enqueue(id, async () => {
      const updated = await persistNoteNow(id, input)
      if (updated) applyPersistedNote(updated)
      return updated
    })
  }

  function discardAllDrafts() {
    autoSave.cancel()
    drafts.value = {}
  }

  async function saveNote(id: string, input: note.UpdateInput) {
    const updated = await persistNote(id, input)
    if (!updated) return false
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

    const endSaving = savingRequests.begin()
    error.value = null
    try {
      return await updateNotesSequentially(ids, async (id) => {
        await noteOperations.enqueue(id, async () => {
          const updated = await updateNote(id, {
            ...input,
            expectedRevision: requirePersistedRevision(id),
          })
          applyPersistedNote(updated)
        })
      })
    } catch (e) {
      setErrorContext({ code: 'NOTES_BATCH_UPDATE_FAILED' })
      error.value = e instanceof Error ? e.message : 'ノートの一括更新に失敗しました'
      throw e
    } finally {
      endSaving()
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
      await noteOperations.enqueue(
        id,
        () => deleteNote(id, requirePersistedRevision(id)),
      )
      discardDraft(id)
      summaries.value = summaries.value.filter((n: note.Summary) => n.id !== id)
      if (activeNote.value?.id === id) activeNote.value = null
    } catch (e) {
      const isRevisionConflict = e instanceof NoteRevisionConflictError
      setErrorContext({
        code: isRevisionConflict ? e.code : 'NOTE_DELETE_FAILED',
        action: isRevisionConflict
          ? undefined
          : { label: '再試行', run: () => permanentlyDeleteNote(id) },
      })
      error.value = e instanceof Error ? e.message : 'ノートの削除に失敗しました'
      throw e
    }
  }

  async function permanentlyDeleteNotes(ids: string[]) {
    if (ids.length === 0) return

    const endSaving = savingRequests.begin()
    error.value = null
    let deletedIds: string[] = []
    try {
      deletedIds = await deleteNotesSequentially(
        ids,
        (id) => noteOperations.enqueue(
          id,
          () => deleteNote(id, requirePersistedRevision(id)),
        ),
      )
    } catch (e) {
      setErrorContext({ code: 'NOTES_BATCH_DELETE_FAILED' })
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
      endSaving()
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
    reloadConflictedNote,
    copyConflictedDraft,
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
