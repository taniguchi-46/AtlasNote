<template>
  <div class="app-root" :data-theme="appStore.theme">
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppSidebar from './components/AppSidebar.vue'
import NoteList from './components/NoteList.vue'
import NoteEditor from './components/NoteEditor.vue'
import { getStartupStatus, type StartupStatus } from './api/startup'
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const startupStatus = ref<StartupStatus | null>(null)

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
