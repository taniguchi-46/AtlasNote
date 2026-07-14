import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type Theme = 'light' | 'dark'
export type SidebarSection = 'notes' | 'recent' | 'uncategorized' | 'favorites' | 'pinned' | 'trash'

export const NOTE_SORT_OPTIONS = [
  { value: '', label: '既定' },
  { value: 'updatedAt:desc', label: '更新日時（新しい順）' },
  { value: 'updatedAt:asc', label: '更新日時（古い順）' },
  { value: 'createdAt:desc', label: '作成日時（新しい順）' },
  { value: 'createdAt:asc', label: '作成日時（古い順）' },
  { value: 'title:asc', label: 'タイトル（昇順）' },
  { value: 'title:desc', label: 'タイトル（降順）' },
] as const

export type NoteSortOption = typeof NOTE_SORT_OPTIONS[number]['value']

export function parseNoteSortOption(value: NoteSortOption) {
  if (!value) return null
  const [sortBy, sortDirection] = value.split(':')
  return { sortBy, sortDirection }
}

export const useAppStore = defineStore('app', () => {
  const theme = ref<Theme>(
    (localStorage.getItem('atlas-theme') as Theme | null) ?? 'dark'
  )
  const sidebarSection = ref<SidebarSection>('notes')
  const sortOption = ref<NoteSortOption>('')

  watch(theme, (newTheme) => {
    localStorage.setItem('atlas-theme', newTheme)
    document.documentElement.setAttribute('data-theme', newTheme)
  }, { immediate: true })

  function setTheme(t: Theme) {
    theme.value = t
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  function setSidebarSection(s: SidebarSection) {
    sidebarSection.value = s
  }

  function setSortOption(value: NoteSortOption) {
    sortOption.value = value
  }

  return {
    theme,
    sidebarSection,
    sortOption,
    setTheme,
    toggleTheme,
    setSidebarSection,
    setSortOption,
  }
})
