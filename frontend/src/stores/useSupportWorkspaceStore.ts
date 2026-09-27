import { defineStore } from 'pinia'
import { ref } from 'vue'

export type SupportTab = 'changes' | 'terminal'

export const useSupportWorkspaceStore = defineStore('supportWorkspace', () => {
  const isOpen = ref(false)
  const isMinimized = ref(false)
  const activeTab = ref<SupportTab>('terminal')
  const isFloating = ref(false)
  const position = ref({ left: 48, top: 64, width: 480, height: 650 })

  function open(tab: SupportTab) {
    activeTab.value = tab
    isOpen.value = true
    isMinimized.value = false
  }

  function minimize() {
    isMinimized.value = true
  }

  function restore() {
    isOpen.value = true
    isMinimized.value = false
  }

  function toggleFloating() {
    isFloating.value = !isFloating.value
  }

  function setPosition(next: Partial<typeof position.value>) {
    position.value = { ...position.value, ...next }
  }

  return { isOpen, isMinimized, activeTab, isFloating, position, open, minimize, restore, toggleFloating, setPosition }
})
