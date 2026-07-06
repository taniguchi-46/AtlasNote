<template>
  <div class="app-root" :data-theme="appStore.theme">
    <AppTopBar 
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
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'
import { useSettingsStore } from './stores/useSettingsStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const settingsStore = useSettingsStore()
const startupStatus = ref<StartupStatus | null>(null)

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

function handleToggleAlwaysOnTop() {
  console.log('Toggle always on top clicked')
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
})
</script>
