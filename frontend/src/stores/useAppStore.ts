import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type Theme = 'light' | 'dark'
export type SidebarSection = 'notes' | 'uncategorized' | 'favorites' | 'pinned' | 'trash'

export const useAppStore = defineStore('app', () => {
  const theme = ref<Theme>(
    (localStorage.getItem('atlas-theme') as Theme | null) ?? 'dark'
  )
  const sidebarSection = ref<SidebarSection>('notes')

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

  return { theme, sidebarSection, setTheme, toggleTheme, setSidebarSection }
})
