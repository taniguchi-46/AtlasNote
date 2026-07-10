<template>
  <div class="app-root" :data-theme="appStore.theme">
    <AppTopBar 
      :is-always-on-top="isAlwaysOnTop"
      @sync="handleSync"
      @search="handleSearch"
      @new-note="noteStore.newNote()"
      @toggle-always-on-top="handleToggleAlwaysOnTop"
      @open-settings="handleOpenSettings"
    />

    <!-- Startup error banner -->
    <div
      v-if="startupStatus && !startupStatus.ready"
      class="startup-banner"
      role="alert"
    >
      <span>⚠ 起動エラー: {{ startupStatus.message }}</span>
      <span class="startup-datadir">{{ startupStatus.dataDir }}</span>
    </div>

    <!-- 3-pane shell -->
    <div
      ref="appShellRef"
      class="app-shell"
      :style="{
        gridTemplateColumns: `${settingsStore.sidebarWidth}px ${settingsStore.noteListWidth}px minmax(0, 1fr)`,
      }"
    >
      <AppSidebar />
      <NoteList />
      <NoteEditor />

      <button
        class="pane-resizer"
        :class="{ 'is-resizing': activeResize === 'sidebar' }"
        :style="{ left: `${settingsStore.sidebarWidth}px` }"
        type="button"
        role="separator"
        aria-label="サイドバーの幅を調整"
        aria-orientation="vertical"
        :aria-valuemin="SIDEBAR_WIDTH_MIN"
        :aria-valuemax="SIDEBAR_WIDTH_MAX"
        :aria-valuenow="settingsStore.sidebarWidth"
        @keydown="handleResizerKeydown('sidebar', $event)"
        @pointerdown="startResize('sidebar', $event)"
        @pointermove="handleResize"
        @pointerup="finishResize"
        @pointercancel="finishResize"
      />
      <button
        class="pane-resizer"
        :class="{ 'is-resizing': activeResize === 'noteList' }"
        :style="{ left: `${settingsStore.sidebarWidth + settingsStore.noteListWidth}px` }"
        type="button"
        role="separator"
        aria-label="ノート一覧の幅を調整"
        aria-orientation="vertical"
        :aria-valuemin="NOTE_LIST_WIDTH_MIN"
        :aria-valuemax="NOTE_LIST_WIDTH_MAX"
        :aria-valuenow="settingsStore.noteListWidth"
        @keydown="handleResizerKeydown('noteList', $event)"
        @pointerdown="startResize('noteList', $event)"
        @pointermove="handleResize"
        @pointerup="finishResize"
        @pointercancel="finishResize"
      />
    </div>

    <!-- Modals -->
    <SettingsModal />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watchEffect } from 'vue'
import AppTopBar from './components/AppTopBar.vue'
import AppSidebar from './components/AppSidebar.vue'
import NoteList from './components/NoteList.vue'
import NoteEditor from './components/NoteEditor.vue'
import SettingsModal from './components/SettingsModal.vue'
import { getStartupStatus, type StartupStatus } from './api/startup'
import { ToggleAlwaysOnTop } from '../wailsjs/go/main/App'
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'
import {
  EDITOR_WIDTH_MIN,
  NOTE_LIST_WIDTH_MAX,
  NOTE_LIST_WIDTH_MIN,
  SIDEBAR_WIDTH_MAX,
  SIDEBAR_WIDTH_MIN,
  useSettingsStore,
} from './stores/useSettingsStore'

type ResizablePane = 'sidebar' | 'noteList'

const noteStore = useNoteStore()
const appStore = useAppStore()
const settingsStore = useSettingsStore()
const startupStatus = ref<StartupStatus | null>(null)
const isAlwaysOnTop = ref(localStorage.getItem('atlas-always-on-top') === 'true')
const appShellRef = ref<HTMLElement | null>(null)
const activeResize = ref<ResizablePane | null>(null)
let resizeObserver: ResizeObserver | null = null
let resizeStartX = 0
let resizeStartWidth = 0

// Apply font family globally
watchEffect(() => {
  document.documentElement.style.setProperty('--font-family-base', settingsStore.fontFamily)
})

// Placeholder handlers for TopBar actions
function handleSync() {
  console.log('Sync clicked')
}

function handleSearch(query: string) {
  console.log('Search query:', query)
}

async function handleToggleAlwaysOnTop() {
  isAlwaysOnTop.value = !isAlwaysOnTop.value
  localStorage.setItem('atlas-always-on-top', String(isAlwaysOnTop.value))
  try {
    await ToggleAlwaysOnTop(isAlwaysOnTop.value)
  } catch (e) {
    console.error('Wails ToggleAlwaysOnTop failed:', e)
  }
}

function handleOpenSettings() {
  settingsStore.openSettings()
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

function resizePane(pane: ResizablePane, requestedWidth: number) {
  const shellWidth = appShellRef.value?.clientWidth
  if (!shellWidth) return

  if (pane === 'sidebar') {
    const maxWidth = Math.min(
      SIDEBAR_WIDTH_MAX,
      shellWidth - settingsStore.noteListWidth - EDITOR_WIDTH_MIN,
    )
    settingsStore.setSidebarWidth(clamp(requestedWidth, SIDEBAR_WIDTH_MIN, maxWidth))
    return
  }

  const maxWidth = Math.min(
    NOTE_LIST_WIDTH_MAX,
    shellWidth - settingsStore.sidebarWidth - EDITOR_WIDTH_MIN,
  )
  settingsStore.setNoteListWidth(clamp(requestedWidth, NOTE_LIST_WIDTH_MIN, maxWidth))
}

function normalizePaneWidths() {
  const shellWidth = appShellRef.value?.clientWidth
  if (!shellWidth) return

  let sidebarWidth = settingsStore.sidebarWidth
  let noteListWidth = settingsStore.noteListWidth
  let overflow = sidebarWidth + noteListWidth + EDITOR_WIDTH_MIN - shellWidth

  if (overflow > 0) {
    const noteListReduction = Math.min(overflow, noteListWidth - NOTE_LIST_WIDTH_MIN)
    noteListWidth -= noteListReduction
    overflow -= noteListReduction
  }

  if (overflow > 0) {
    sidebarWidth -= Math.min(overflow, sidebarWidth - SIDEBAR_WIDTH_MIN)
  }

  settingsStore.setSidebarWidth(sidebarWidth)
  settingsStore.setNoteListWidth(noteListWidth)
}

function startResize(pane: ResizablePane, event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return

  activeResize.value = pane
  resizeStartX = event.clientX
  resizeStartWidth = pane === 'sidebar'
    ? settingsStore.sidebarWidth
    : settingsStore.noteListWidth

  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  document.body.classList.add('is-pane-resizing')
}

function handleResize(event: PointerEvent) {
  if (!activeResize.value) return
  resizePane(activeResize.value, resizeStartWidth + event.clientX - resizeStartX)
}

function finishResize(event: PointerEvent) {
  if (!activeResize.value) return

  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) {
    target.releasePointerCapture(event.pointerId)
  }
  activeResize.value = null
  document.body.classList.remove('is-pane-resizing')
}

function handleResizerKeydown(pane: ResizablePane, event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return

  event.preventDefault()
  const currentWidth = pane === 'sidebar'
    ? settingsStore.sidebarWidth
    : settingsStore.noteListWidth
  resizePane(pane, currentWidth + (event.key === 'ArrowLeft' ? -10 : 10))
}

onMounted(async () => {
  resizeObserver = new ResizeObserver(normalizePaneWidths)
  if (appShellRef.value) {
    resizeObserver.observe(appShellRef.value)
    normalizePaneWidths()
  }

  try {
    startupStatus.value = await getStartupStatus()
    if (startupStatus.value.ready) {
      await noteStore.fetchNotes()
    }
  } catch (_) {
    // Network or Wails not available (dev browser mode)
    await noteStore.fetchNotes().catch(() => {})
  }

  // Apply initial always-on-top status
  try {
    await ToggleAlwaysOnTop(isAlwaysOnTop.value)
  } catch (e) {
    console.error('Wails ToggleAlwaysOnTop failed:', e)
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  document.body.classList.remove('is-pane-resizing')
})
</script>
