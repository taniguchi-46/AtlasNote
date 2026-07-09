import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { DEFAULT_NOTEBOOK_ICON, isKnownNotebookIcon } from '../utils/notebookIcons'

export type EditorFirstLineStyle = 'heading1' | 'heading2' | 'heading3' | 'paragraph'

const FONT_SIZE_OPTIONS = [12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 26] as const
const FIRST_LINE_STYLE_OPTIONS: EditorFirstLineStyle[] = ['heading1', 'heading2', 'heading3', 'paragraph']

function readNumberOption<T extends readonly number[]>(key: string, fallback: T[number], options: T) {
  const value = Number(localStorage.getItem(key))
  return options.includes(value as T[number]) ? value as T[number] : fallback
}

function readNumberInRange(key: string, fallback: number, min: number, max: number) {
  const value = Number(localStorage.getItem(key))
  return Number.isFinite(value) && value >= min && value <= max ? value : fallback
}

function readStringOption<T extends readonly string[]>(key: string, fallback: T[number], options: T) {
  const value = localStorage.getItem(key)
  return value && options.includes(value as T[number]) ? value as T[number] : fallback
}

export const useSettingsStore = defineStore('settings', () => {
  const isSettingsOpen = ref(false)
  
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
  
  function openSettings() {
    isSettingsOpen.value = true
  }
  
  function closeSettings() {
    isSettingsOpen.value = false
  }

  return {
    isSettingsOpen,
    fontFamily,
    editorFontSize,
    editorFirstLineStyle,
    editorLineLength,
    editorLineHeight,
    editorParagraphSpacing,
    defaultNotebookIcon,
    openSettings,
    closeSettings
  }
})
