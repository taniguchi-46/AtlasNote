import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export const useSettingsStore = defineStore('settings', () => {
  const isSettingsOpen = ref(false)
  
  // App Settings
  const fontFamily = ref(localStorage.getItem('atlas-font-family') ?? 'Meiryo')
  
  watch(fontFamily, (newFont) => {
    localStorage.setItem('atlas-font-family', newFont)
    document.documentElement.style.setProperty('--font-family-base', newFont)
  }, { immediate: true })
  
  function openSettings() {
    isSettingsOpen.value = true
  }
  
  function closeSettings() {
    isSettingsOpen.value = false
  }

  return {
    isSettingsOpen,
    fontFamily,
    openSettings,
    closeSettings
  }
})
