import { defineStore } from 'pinia'
import { ref } from 'vue'

export type SupportTab = 'organize' | 'ai' | 'changes'

export const useSupportWorkspaceStore = defineStore('supportWorkspace', () => {
  const isOpen = ref(false)
  const isMinimized = ref(false)
  const activeTab = ref<SupportTab>('organize')
  const isFloating = ref(false)
  const position = ref({ left: 48, top: 64, width: 480, height: 650 })
  const focusAIRequest = ref(0)
  const lockVersion = ref(0)

  function open(tab: SupportTab) {
    activeTab.value = tab
    isOpen.value = true
    isMinimized.value = false
    if (tab === 'ai') focusAIRequest.value += 1
  }

  function minimize() {
    isMinimized.value = true
  }

  function restore() {
    isOpen.value = true
    isMinimized.value = false
    if (activeTab.value === 'ai') focusAIRequest.value += 1
  }

  function toggleAI() {
    if (isOpen.value && !isMinimized.value && activeTab.value === 'ai') {
      minimize()
    } else {
      open('ai')
    }
  }

  function toggleFloating() {
    isFloating.value = !isFloating.value
  }

  function setPosition(next: Partial<typeof position.value>) {
    position.value = { ...position.value, ...next }
  }

  function invalidateForLock() { lockVersion.value += 1 }

  return { isOpen, isMinimized, activeTab, isFloating, position, focusAIRequest, lockVersion, open, minimize, restore, toggleAI, toggleFloating, setPosition, invalidateForLock }
})
