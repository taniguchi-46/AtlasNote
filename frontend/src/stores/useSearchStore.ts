import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { note } from '../../wailsjs/go/models'
import { searchNotes } from '../api/search'
import { createLatestRequestGuard } from '../utils/latestRequestGuard'

export type SearchFilters = {
  notebookId: string | null
  includeTrashed: boolean
}

const DEFAULT_PAGE = 1
const DEFAULT_PAGE_SIZE = 100

function emptyResult(): note.SearchResult {
  return {
    items: [],
    page: DEFAULT_PAGE,
    pageSize: DEFAULT_PAGE_SIZE,
    total: 0,
    hasNext: false,
  } as unknown as note.SearchResult
}

export const useSearchStore = defineStore('search', () => {
  const query = ref('')
  const items = ref<note.SearchItem[]>([])
  const page = ref(DEFAULT_PAGE)
  const total = ref(0)
  const hasNext = ref(false)
  const isSearching = ref(false)
  const error = ref<string | null>(null)
  const errorCode = ref<string | null>(null)
  const filters = ref<SearchFilters>({ notebookId: null, includeTrashed: false })
  const requestGuard = createLatestRequestGuard()

  const isActive = computed(() => query.value.trim().length > 0)

  function clearResult() {
    const result = emptyResult()
    items.value = result.items
    total.value = result.total
    hasNext.value = result.hasNext
  }

  async function search(
    nextQuery: string,
    nextFilters: SearchFilters = filters.value,
    nextPage = DEFAULT_PAGE,
    append = false,
  ) {
    query.value = nextQuery
    filters.value = { ...nextFilters }
    const isLatestRequest = requestGuard.begin()
    error.value = null
    errorCode.value = null
    if (!append) {
      page.value = DEFAULT_PAGE
      clearResult()
    }

    if (!isActive.value) {
      page.value = DEFAULT_PAGE
      isSearching.value = false
      return
    }

    isSearching.value = true
    try {
      const result = await searchNotes({
        query: nextQuery,
        scope: 'all',
        ...(filters.value.notebookId ? { notebookId: filters.value.notebookId } : {}),
        includeTrashed: filters.value.includeTrashed,
        page: nextPage,
        pageSize: DEFAULT_PAGE_SIZE,
      } as note.SearchInput)
      if (!isLatestRequest()) return

      if (result.error) {
        error.value = result.error.message
        errorCode.value = result.error.code
        return
      }

      page.value = result.page
      items.value = append ? [...items.value, ...(result.items ?? [])] : (result.items ?? [])
      total.value = result.total
      hasNext.value = result.hasNext
    } catch (cause) {
      if (!isLatestRequest()) return
      error.value = cause instanceof Error ? cause.message : '検索に失敗しました。'
      errorCode.value = 'SEARCH_REQUEST_FAILED'
    } finally {
      if (isLatestRequest()) isSearching.value = false
    }
  }

  async function refresh() {
    if (!isActive.value) return
    await search(query.value, filters.value)
  }

  async function nextPage() {
    if (!isActive.value || !hasNext.value || isSearching.value) return
    await search(query.value, filters.value, page.value + 1, true)
  }

  function clear() {
    query.value = ''
    requestGuard.begin()
    page.value = DEFAULT_PAGE
    error.value = null
    errorCode.value = null
    isSearching.value = false
    clearResult()
  }

  return {
    query,
    items,
    page,
    pageSize: DEFAULT_PAGE_SIZE,
    total,
    hasNext,
    isSearching,
    error,
    errorCode,
    filters,
    isActive,
    search,
    refresh,
    nextPage,
    clear,
  }
})
