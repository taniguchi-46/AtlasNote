import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { listBacklinks, searchNoteLinkTargets } from '../api/noteLinks'
import { createLatestRequestGuard } from '../utils/latestRequestGuard'
import { useNotificationStore } from './useNotificationStore'

const BACKLINK_PAGE_SIZE = 20
const LINK_TARGET_PAGE_SIZE = 20

export const useNoteLinkStore = defineStore('noteLinks', () => {
  const backlinks = ref<note.Summary[]>([])
  const backlinkPage = ref(0)
  const backlinkTotal = ref(0)
  const backlinkHasNext = ref(false)
  const backlinkNoteId = ref<string | null>(null)
  const isLoadingBacklinks = ref(false)
  const backlinkError = ref<string | null>(null)

  const targetQuery = ref('')
  const targetItems = ref<note.SearchItem[]>([])
  const isSearchingTargets = ref(false)
  const targetError = ref<string | null>(null)

  const backlinkRequests = createLatestRequestGuard()
  const targetRequests = createLatestRequestGuard()
  const notificationStore = useNotificationStore()

  async function loadBacklinks(noteId: string) {
    const isLatestRequest = backlinkRequests.begin()
    backlinkNoteId.value = noteId
    backlinkPage.value = 0
    backlinkTotal.value = 0
    backlinkHasNext.value = false
    backlinks.value = []
    backlinkError.value = null
    isLoadingBacklinks.value = true

    try {
      const result = await listBacklinks({
        noteId,
        page: 1,
        pageSize: BACKLINK_PAGE_SIZE,
      } as note.BacklinkListInput)
      if (!isLatestRequest()) return

      backlinkPage.value = result.page
      backlinkTotal.value = result.total
      backlinkHasNext.value = result.hasNext
      backlinks.value = result.items ?? []
      notificationStore.dismissBySource('note-links')
    } catch (cause) {
      if (!isLatestRequest()) return
      backlinkError.value = cause instanceof Error ? cause.message : 'バックリンクの読み込みに失敗しました'
      notificationStore.notify(backlinkError.value, {
        kind: 'error',
        source: 'note-links',
        code: 'BACKLINK_LOAD_FAILED',
        retryable: true,
        action: { label: '再試行', run: () => loadBacklinks(noteId) },
        dedupeKey: 'note-links:BACKLINK_LOAD_FAILED',
      })
    } finally {
      if (isLatestRequest()) isLoadingBacklinks.value = false
    }
  }

  async function loadNextBacklinks() {
    const noteId = backlinkNoteId.value
    if (!noteId || !backlinkHasNext.value || isLoadingBacklinks.value) return

    const isLatestRequest = backlinkRequests.begin()
    isLoadingBacklinks.value = true
    backlinkError.value = null
    try {
      const result = await listBacklinks({
        noteId,
        page: backlinkPage.value + 1,
        pageSize: BACKLINK_PAGE_SIZE,
      } as note.BacklinkListInput)
      if (!isLatestRequest()) return

      backlinks.value = [...backlinks.value, ...(result.items ?? [])]
      backlinkPage.value = result.page
      backlinkTotal.value = result.total
      backlinkHasNext.value = result.hasNext
    } catch (cause) {
      if (!isLatestRequest()) return
      backlinkError.value = cause instanceof Error ? cause.message : 'バックリンクの追加読み込みに失敗しました'
      notificationStore.notify(backlinkError.value, {
        kind: 'error',
        source: 'note-links',
        code: 'BACKLINK_LOAD_MORE_FAILED',
        retryable: true,
        action: { label: '再試行', run: () => loadNextBacklinks() },
        dedupeKey: 'note-links:BACKLINK_LOAD_MORE_FAILED',
      })
    } finally {
      if (isLatestRequest()) isLoadingBacklinks.value = false
    }
  }

  async function searchTargets(query: string) {
    targetQuery.value = query
    const isLatestRequest = targetRequests.begin()
    targetItems.value = []
    targetError.value = null

    if (!query.trim()) {
      isSearchingTargets.value = false
      return
    }

    isSearchingTargets.value = true
    try {
      const result = await searchNoteLinkTargets(query)
      if (!isLatestRequest()) return
      if (result.error) {
        throw new Error(result.error.message)
      }
      targetItems.value = result.items ?? []
    } catch (cause) {
      if (!isLatestRequest()) return
      targetError.value = cause instanceof Error ? cause.message : 'リンク先ノートの検索に失敗しました'
      notificationStore.notify(targetError.value, {
        kind: 'error',
        source: 'note-links',
        code: 'NOTE_LINK_TARGET_SEARCH_FAILED',
        retryable: true,
        action: { label: '再試行', run: () => searchTargets(query) },
        dedupeKey: 'note-links:NOTE_LINK_TARGET_SEARCH_FAILED',
      })
    } finally {
      if (isLatestRequest()) isSearchingTargets.value = false
    }
  }

  function clearTargetSearch() {
    targetRequests.begin()
    targetQuery.value = ''
    targetItems.value = []
    targetError.value = null
    isSearchingTargets.value = false
  }

  return {
    backlinks,
    backlinkPage,
    backlinkTotal,
    backlinkHasNext,
    backlinkNoteId,
    isLoadingBacklinks,
    backlinkError,
    targetQuery,
    targetItems,
    isSearchingTargets,
    targetError,
    loadBacklinks,
    loadNextBacklinks,
    searchTargets,
    clearTargetSearch,
    targetPageSize: LINK_TARGET_PAGE_SIZE,
  }
})
