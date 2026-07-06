import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useSettingsStore = defineStore('settings', () => {
  const isSettingsOpen = ref(false)
  
  // App Settings
  const fontFamily = ref('Meiryo')
  
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
