<template>
  <aside class="sidebar" aria-label="サイドバー">
    <!-- Brand -->
    <div class="sidebar-brand">
      <div class="brand-icon">
        <span>A</span>
      </div>
      <span class="brand-name">Atlas Note</span>
    </div>

    <!-- New Note Button -->
    <button
      id="btn-new-note"
      class="new-note-btn"
      type="button"
      :disabled="noteStore.isSaving"
      @click="createNewNote"
    >
      <PlusIcon :size="16" />
      <span>新規ノート</span>
    </button>

    <!-- Nav Sections -->
    <nav class="sidebar-nav" aria-label="メインナビゲーション">
      <button
        v-for="item in navItems"
        :key="item.section"
        :id="`nav-${item.section}`"
        class="nav-item"
        :class="{ 'is-active': appStore.sidebarSection === item.section && !notebookStore.activeNotebookId }"
        type="button"
        @click="handleNavClick(item.section)"
      >
        <component :is="item.icon" :size="16" class="nav-icon" />
        <span>{{ item.label }}</span>
        <span v-if="item.count > 0" class="nav-badge">{{ item.count }}</span>
      </button>
    </nav>

    <!-- Notebooks Section -->
    <div class="sidebar-notebooks-section">
      <div class="notebooks-header">
        <span>ノートブック</span>
        <button class="add-notebook-btn" type="button" title="ノートブックを追加" @click="startAddRootNotebook">
          <PlusIcon :size="14" />
        </button>
      </div>
      <input
        v-if="isAddingRootNotebook"
        ref="rootNotebookInputRef"
        v-model="rootNotebookName"
        class="notebook-rename-input"
        type="text"
        placeholder="ノートブック名"
        @blur="saveRootNotebook"
        @keydown.enter="saveRootNotebook"
        @keydown.escape="cancelAddRootNotebook"
      />
      <div class="notebooks-tree">
        <NotebookTreeItem
          v-for="node in notebookStore.notebookTree"
          :key="node.id"
          :node="node"
        />
      </div>
    </div>

    <!-- Spacer -->
    <div class="sidebar-spacer" />

    <!-- Theme Toggle -->
    <button
      id="btn-theme-toggle"
      class="theme-toggle"
      type="button"
      :title="appStore.theme === 'dark' ? 'ライトテーマに切り替え' : 'ダークテーマに切り替え'"
      @click="appStore.toggleTheme()"
    >
      <SunIcon v-if="appStore.theme === 'dark'" :size="16" />
      <MoonIcon v-else :size="16" />
    </button>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { PlusIcon, FileTextIcon, StarIcon, PinIcon, Trash2Icon, SunIcon, MoonIcon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import NotebookTreeItem from './NotebookTreeItem.vue'
import type { SidebarSection } from '../stores/useAppStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const isAddingRootNotebook = ref(false)
const rootNotebookName = ref('')
const rootNotebookInputRef = ref<HTMLInputElement | null>(null)

onMounted(async () => {
  try {
    await notebookStore.fetchNotebooks()
  } catch (_) {}
})

const navItems = computed<Array<{
  section: SidebarSection
  label: string
  icon: unknown
  count: number
}>>(() => [
  {
    section: 'notes',
    label: 'すべてのノート',
    icon: FileTextIcon,
    count: noteStore.activeNotes.length,
  },
  {
    section: 'favorites',
    label: 'お気に入り',
    icon: StarIcon,
    count: noteStore.favoriteNotes.length,
  },
  {
    section: 'pinned',
    label: 'ピン留め',
    icon: PinIcon,
    count: noteStore.pinnedNotes.length,
  },
  {
    section: 'trash',
    label: 'ゴミ箱',
    icon: Trash2Icon,
    count: noteStore.trashedNotes.length,
  },
])

function handleNavClick(section: SidebarSection) {
  appStore.setSidebarSection(section)
  notebookStore.activeNotebookId = null
}

function createNewNote() {
  // If a notebook is selected, associate the new note with it
  noteStore.newNote('新しいノート', '', notebookStore.activeNotebookId)
}

function startAddRootNotebook() {
  isAddingRootNotebook.value = true
  rootNotebookName.value = ''
  nextTick(() => {
    rootNotebookInputRef.value?.focus()
  })
}

function saveRootNotebook() {
  if (!isAddingRootNotebook.value) return
  const name = rootNotebookName.value.trim()
  isAddingRootNotebook.value = false
  rootNotebookName.value = ''
  if (name) {
    notebookStore.newNotebook(name)
  }
}

function cancelAddRootNotebook() {
  isAddingRootNotebook.value = false
  rootNotebookName.value = ''
}
</script>
