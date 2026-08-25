import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { DEFAULT_NOTEBOOK_ICON, isKnownNotebookIcon } from '../utils/notebookIcons'

export type EditorFirstLineStyle = 'heading1' | 'heading2' | 'heading3' | 'paragraph'
export type AIWorkspacePlacement = 'right' | 'bottom'
export type AIAgentEditPermission = 'review-required' | 'auto-update'
export type SettingsTab = 'theme' | 'general' | 'editor' | 'sync' | 'ai' | 'storage-spaces'

export const SIDEBAR_WIDTH_MIN = 180
export const SIDEBAR_WIDTH_MAX = 360
export const NOTE_LIST_WIDTH_MIN = 220
export const NOTE_LIST_WIDTH_MAX = 480
export const EDITOR_WIDTH_MIN = 360
export const AI_WORKSPACE_RIGHT_WIDTH_MIN = 300
export const AI_WORKSPACE_RIGHT_WIDTH_MAX = 960
export const AI_WORKSPACE_BOTTOM_HEIGHT_MIN = 180
export const AI_WORKSPACE_BOTTOM_HEIGHT_MAX = 760

const FONT_SIZE_OPTIONS = [12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 26] as const
const FIRST_LINE_STYLE_OPTIONS: EditorFirstLineStyle[] = ['heading1', 'heading2', 'heading3', 'paragraph']
const AI_WORKSPACE_PLACEMENT_OPTIONS = ['right', 'bottom'] as const
const AI_AGENT_EDIT_PERMISSION_OPTIONS = ['review-required', 'auto-update'] as const

function readNumberOption<T extends readonly number[]>(key: string, fallback: T[number], options: T) {
  const value = Number(localStorage.getItem(key))
  return options.includes(value as T[number]) ? value as T[number] : fallback
}

function readNumberInRange(key: string, fallback: number, min: number, max: number) {
  const value = Number(localStorage.getItem(key))
  return Number.isFinite(value) && value >= min && value <= max ? value : fallback
}

function readClampedNumberInRange(key: string, fallback: number, min: number, max: number) {
  const rawValue = localStorage.getItem(key)
  if (rawValue === null || rawValue.trim() === '') return fallback
  const value = Number(rawValue)
  if (!Number.isFinite(value)) return fallback
  return Math.min(max, Math.max(min, Math.round(value)))
}

function readStringOption<T extends readonly string[]>(key: string, fallback: T[number], options: T) {
  const value = localStorage.getItem(key)
  return value && options.includes(value as T[number]) ? value as T[number] : fallback
}

export const useSettingsStore = defineStore('settings', () => {
  const isSettingsOpen = ref(false)
  const requestedTab = ref<SettingsTab>('theme')

  // Layout Settings
  const sidebarWidth = ref(
    readNumberInRange('atlas-sidebar-width', 220, SIDEBAR_WIDTH_MIN, SIDEBAR_WIDTH_MAX),
  )
  const noteListWidth = ref(
    readNumberInRange('atlas-note-list-width', 280, NOTE_LIST_WIDTH_MIN, NOTE_LIST_WIDTH_MAX),
  )
  const aiWorkspacePlacement = ref<AIWorkspacePlacement>(
    readStringOption('atlas-ai-workspace-placement', 'right', AI_WORKSPACE_PLACEMENT_OPTIONS),
  )
  const aiAgentEditPermission = ref<AIAgentEditPermission>(
    readStringOption(
      'atlas-ai-agent-edit-permission',
      'review-required',
      AI_AGENT_EDIT_PERMISSION_OPTIONS,
    ),
  )
  const aiWorkspaceRightWidth = ref(
    readClampedNumberInRange(
      'atlas-ai-workspace-right-width',
      480,
      AI_WORKSPACE_RIGHT_WIDTH_MIN,
      AI_WORKSPACE_RIGHT_WIDTH_MAX,
    ),
  )
  const aiWorkspaceBottomHeight = ref(
    readNumberInRange(
      'atlas-ai-workspace-bottom-height',
      360,
      AI_WORKSPACE_BOTTOM_HEIGHT_MIN,
      AI_WORKSPACE_BOTTOM_HEIGHT_MAX,
    ),
  )
  
  // Editor Settings
  const fontFamily = ref(localStorage.getItem('atlas-font-family') ?? 'Meiryo')
  const editorFontSize = ref(readNumberOption('atlas-editor-font-size', 14, FONT_SIZE_OPTIONS))
  const editorFirstLineStyle = ref(
    readStringOption('atlas-editor-first-line-style', 'heading2', FIRST_LINE_STYLE_OPTIONS),
  )
  const editorLineLength = ref(readNumberInRange('atlas-editor-line-length', 760, 520, 1200))
  const editorLineHeight = ref(readNumberInRange('atlas-editor-line-height', 1.8, 1.2, 2.4))
  const editorParagraphSpacing = ref(readNumberInRange('atlas-editor-paragraph-spacing', 1, 0, 2))
  const savedDefaultNotebookIcon = localStorage.getItem('atlas-default-notebook-icon') ?? DEFAULT_NOTEBOOK_ICON
  const defaultNotebookIcon = ref(
    isKnownNotebookIcon(savedDefaultNotebookIcon) ? savedDefaultNotebookIcon : DEFAULT_NOTEBOOK_ICON,
  )

  watch(sidebarWidth, (newSidebarWidth) => {
    localStorage.setItem('atlas-sidebar-width', String(newSidebarWidth))
  }, { immediate: true })

  watch(noteListWidth, (newNoteListWidth) => {
    localStorage.setItem('atlas-note-list-width', String(newNoteListWidth))
  }, { immediate: true })

  watch(aiWorkspacePlacement, (newPlacement) => {
    localStorage.setItem('atlas-ai-workspace-placement', newPlacement)
  }, { immediate: true })

  watch(aiAgentEditPermission, (newPermission) => {
    localStorage.setItem('atlas-ai-agent-edit-permission', newPermission)
  }, { immediate: true })

  watch(aiWorkspaceRightWidth, (newWidth) => {
    localStorage.setItem('atlas-ai-workspace-right-width', String(newWidth))
  }, { immediate: true })

  watch(aiWorkspaceBottomHeight, (newHeight) => {
    localStorage.setItem('atlas-ai-workspace-bottom-height', String(newHeight))
  }, { immediate: true })
  
  watch(fontFamily, (newFont) => {
    localStorage.setItem('atlas-font-family', newFont)
    document.documentElement.style.setProperty('--font-family-base', newFont)
    document.documentElement.style.setProperty('--editor-font-family', newFont)
  }, { immediate: true })

  watch(editorFontSize, (newFontSize) => {
    localStorage.setItem('atlas-editor-font-size', String(newFontSize))
    document.documentElement.style.setProperty('--editor-font-size', `${newFontSize}px`)
  }, { immediate: true })

  watch(editorLineLength, (newLineLength) => {
    localStorage.setItem('atlas-editor-line-length', String(newLineLength))
    document.documentElement.style.setProperty('--editor-line-max-width', `${newLineLength}px`)
  }, { immediate: true })

  watch(editorLineHeight, (newLineHeight) => {
    localStorage.setItem('atlas-editor-line-height', String(newLineHeight))
    document.documentElement.style.setProperty('--editor-line-height', String(newLineHeight))
  }, { immediate: true })

  watch(editorParagraphSpacing, (newParagraphSpacing) => {
    localStorage.setItem('atlas-editor-paragraph-spacing', String(newParagraphSpacing))
    document.documentElement.style.setProperty('--editor-paragraph-spacing', `${newParagraphSpacing}em`)
  }, { immediate: true })

  watch(editorFirstLineStyle, (newFirstLineStyle) => {
    localStorage.setItem('atlas-editor-first-line-style', newFirstLineStyle)
  }, { immediate: true })

  watch(defaultNotebookIcon, (newDefaultNotebookIcon) => {
    const icon = isKnownNotebookIcon(newDefaultNotebookIcon)
      ? newDefaultNotebookIcon
      : DEFAULT_NOTEBOOK_ICON
    if (icon !== newDefaultNotebookIcon) {
      defaultNotebookIcon.value = icon
      return
    }
    localStorage.setItem('atlas-default-notebook-icon', icon)
  }, { immediate: true })
  
  function openSettings(tab?: SettingsTab) {
    if (tab) requestedTab.value = tab
    isSettingsOpen.value = true
  }
  
  function closeSettings() {
    isSettingsOpen.value = false
  }

  function setSidebarWidth(width: number) {
    sidebarWidth.value = Math.min(SIDEBAR_WIDTH_MAX, Math.max(SIDEBAR_WIDTH_MIN, Math.round(width)))
  }

  function setNoteListWidth(width: number) {
    noteListWidth.value = Math.min(NOTE_LIST_WIDTH_MAX, Math.max(NOTE_LIST_WIDTH_MIN, Math.round(width)))
  }

  function setAIWorkspaceRightWidth(width: number) {
    aiWorkspaceRightWidth.value = Math.min(
      AI_WORKSPACE_RIGHT_WIDTH_MAX,
      Math.max(AI_WORKSPACE_RIGHT_WIDTH_MIN, Math.round(width)),
    )
  }

  function setAIWorkspaceBottomHeight(height: number) {
    aiWorkspaceBottomHeight.value = Math.min(
      AI_WORKSPACE_BOTTOM_HEIGHT_MAX,
      Math.max(AI_WORKSPACE_BOTTOM_HEIGHT_MIN, Math.round(height)),
    )
  }

  return {
    isSettingsOpen,
    requestedTab,
    sidebarWidth,
    noteListWidth,
    aiWorkspacePlacement,
    aiAgentEditPermission,
    aiWorkspaceRightWidth,
    aiWorkspaceBottomHeight,
    fontFamily,
    editorFontSize,
    editorFirstLineStyle,
    editorLineLength,
    editorLineHeight,
    editorParagraphSpacing,
    defaultNotebookIcon,
    openSettings,
    closeSettings,
    setSidebarWidth,
    setNoteListWidth,
    setAIWorkspaceRightWidth,
    setAIWorkspaceBottomHeight,
  }
})
