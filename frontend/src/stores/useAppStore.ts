import { defineStore } from 'pinia'
import { ref } from 'vue'

export type Theme = 'light' | 'dark'
export type SidebarSection = 'notes' | 'favorites' | 'pinned' | 'trash'

export const useAppStore = defineStore('app', () => {
  const theme = ref<Theme>(
    (localStorage.getItem('atlas-theme') as Theme | null) ?? 'dark'
  )
  const sidebarSection = ref<SidebarSection>('notes')

  function setTheme(t: Theme) {
    theme.value = t
    localStorage.setItem('atlas-theme', t)
    document.documentElement.setAttribute('data-theme', t)
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  function setSidebarSection(s: SidebarSection) {
    sidebarSection.value = s
  }

  // Apply initial theme
  document.documentElement.setAttribute('data-theme', theme.value)

  return { theme, sidebarSection, setTheme, toggleTheme, setSidebarSection }
})
