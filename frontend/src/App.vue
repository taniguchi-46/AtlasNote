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
    <div class="app-shell">
      <AppSidebar />
      <NoteList />
      <NoteEditor />
    </div>

    <!-- Modals -->
    <SettingsModal />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watchEffect } from 'vue'
import AppTopBar from './components/AppTopBar.vue'
import AppSidebar from './components/AppSidebar.vue'
import NoteList from './components/NoteList.vue'
import NoteEditor from './components/NoteEditor.vue'
import SettingsModal from './components/SettingsModal.vue'
import { getStartupStatus, type StartupStatus } from './api/startup'
import { ToggleAlwaysOnTop } from '../wailsjs/go/main/App'
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'
import { useSettingsStore } from './stores/useSettingsStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const settingsStore = useSettingsStore()
const startupStatus = ref<StartupStatus | null>(null)
const isAlwaysOnTop = ref(localStorage.getItem('atlas-always-on-top') === 'true')

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

onMounted(async () => {
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
</script>
