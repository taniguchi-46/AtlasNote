<template>
  <aside class="sidebar" aria-label="サイドバー">
    <button
      id="btn-new-notebook"
      class="new-note-btn"
      type="button"
      @click="openRootCreateModal"
    >
      <span>ノートブック</span>
      <PlusIcon :size="16" class="new-note-btn-icon" />
    </button>

    <nav class="sidebar-nav" aria-label="メインナビゲーション">
      <button
        v-for="item in navItems"
        :key="item.section"
        :id="`nav-${item.section}`"
        class="nav-item"
        :class="{ 'is-active': appStore.sidebarSection === item.section && !notebookStore.activeNotebookId }"
        type="button"
        @click="handleNavClick(item.section)"
        @contextmenu.prevent="showTrashContextMenu($event, item.section)"
      >
        <component :is="item.icon" :size="16" class="nav-icon" />
        <span>{{ item.label }}</span>
        <span v-if="item.count > 0" class="nav-badge">{{ item.count }}</span>
      </button>
    </nav>

    <div
      v-if="trashContextMenu.visible"
      class="sidebar-context-menu"
      :style="{ top: `${trashContextMenu.y}px`, left: `${trashContextMenu.x}px` }"
      @click.stop
    >
      <button
        class="sidebar-context-menu-item danger"
        type="button"
        :disabled="noteStore.isSaving || noteStore.trashedNotes.length === 0"
        @click="emptyTrashFromContextMenu"
      >
        <Trash2Icon :size="14" class="mr-2" />
        ゴミ箱を空にする
      </button>
    </div>

    <div class="sidebar-notebooks-section">
      <div class="notebooks-header">
        <span>ノートブック</span>
        <button class="add-notebook-btn" type="button" title="ノートブックを追加" @click="openRootCreateModal">
          <PlusIcon :size="14" />
        </button>
      </div>
      <div class="notebooks-tree">
        <NotebookTreeItem
          v-for="node in notebookStore.notebookTree"
          :key="node.id"
          :node="node"
        />
      </div>
    </div>

    <div class="sidebar-spacer" />

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

    <NotebookCreateModal
      :open="isRootCreateModalOpen"
      @close="isRootCreateModalOpen = false"
    />
  </aside>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { PlusIcon, FileTextIcon, StarIcon, PinIcon, Trash2Icon, SunIcon, MoonIcon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import NotebookTreeItem from './NotebookTreeItem.vue'
import NotebookCreateModal from './NotebookCreateModal.vue'
import type { SidebarSection } from '../stores/useAppStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const isRootCreateModalOpen = ref(false)
const trashContextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
})

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

function showTrashContextMenu(event: MouseEvent, section: SidebarSection) {
  if (section !== 'trash') {
    closeTrashContextMenu()
    return
  }

  trashContextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
  }
  document.addEventListener('click', closeTrashContextMenu)
}

function closeTrashContextMenu() {
  trashContextMenu.value.visible = false
  document.removeEventListener('click', closeTrashContextMenu)
}

async function emptyTrashFromContextMenu() {
  const count = noteStore.trashedNotes.length
  if (count === 0) return

  const confirmed = window.confirm(
    `ゴミ箱内の${count}件のノートを完全に削除します。この操作は元に戻せません。`,
  )
  if (!confirmed) {
    closeTrashContextMenu()
    return
  }

  await noteStore.emptyTrash()
  closeTrashContextMenu()
}

function openRootCreateModal() {
  isRootCreateModalOpen.value = true
}

onBeforeUnmount(() => {
  document.removeEventListener('click', closeTrashContextMenu)
})
</script>

<style scoped>
.sidebar-context-menu {
  position: fixed;
  z-index: 9999;
  min-width: 180px;
  padding: 4px 0;
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.sidebar-context-menu-item {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 8px 12px;
  background: none;
  border: none;
  color: var(--text-primary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.sidebar-context-menu-item:hover:not(:disabled) {
  background-color: var(--bg-hover);
}

.sidebar-context-menu-item.danger {
  color: var(--color-danger);
}

.sidebar-context-menu-item.danger:hover:not(:disabled) {
  background-color: rgba(248, 81, 73, 0.1);
}

.sidebar-context-menu-item:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.mr-2 {
  margin-right: 8px;
}

.new-note-btn-icon {
  margin-left: auto;
  flex-shrink: 0;
}
</style>
