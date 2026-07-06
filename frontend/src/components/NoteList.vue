<template>
  <section class="note-list-pane" aria-label="ノート一覧">
    <!-- Header -->
    <div class="note-list-header">
      <h2 class="note-list-title">{{ sectionTitle }}</h2>
      <span class="note-list-count">{{ displayedNotes.length }}</span>
    </div>

    <!-- Loading -->
    <div v-if="noteStore.isLoading && displayedNotes.length === 0" class="note-list-empty">
      <div class="spinner" aria-label="読み込み中..." />
    </div>

    <!-- Empty state -->
    <div v-else-if="displayedNotes.length === 0" class="note-list-empty">
      <FileTextIcon :size="32" class="empty-icon" />
      <p class="empty-label">ノートはありません</p>
    </div>

    <!-- Note items -->
    <ul v-else class="note-list" role="list">
      <li
        v-for="note in displayedNotes"
        :key="note.id"
        class="note-item"
        :class="{ 'is-active': noteStore.activeNote?.id === note.id }"
        role="listitem"
        @contextmenu.prevent="showContextMenu($event, note)"
      >
        <button
          :id="`note-item-${note.id}`"
          class="note-item-btn"
          type="button"
          @click="noteStore.selectNote(note.id)"
        >
          <!-- Icons row -->
          <div class="note-item-meta">
            <PinIcon v-if="note.isPinned" :size="12" class="meta-icon pinned" />
            <StarIcon v-if="note.isFavorite" :size="12" class="meta-icon favorite" />
            <span class="note-item-date">{{ formatDate(note.updatedAt) }}</span>
          </div>
          <p class="note-item-title">{{ note.title || '(無題)' }}</p>
        </button>
      </li>
    </ul>

    <!-- Context Menu -->
    <div 
      v-if="contextMenu.visible" 
      class="context-menu" 
      :style="{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }"
    >
      <template v-if="!contextMenu.isTrashed">
        <button class="context-menu-item" @click="handleContextAction('favorite')">
          <StarIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isFavorite }" />
          {{ contextMenu.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加' }}
        </button>
        <button class="context-menu-item" @click="handleContextAction('pin')">
          <PinIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isPinned }" />
          {{ contextMenu.isPinned ? 'ピン留めを外す' : 'ピン留めする' }}
        </button>
        <div class="context-menu-divider"></div>
        <button class="context-menu-item danger" @click="handleContextAction('trash')">
          <Trash2Icon :size="14" class="mr-2" />
          ゴミ箱へ移動
        </button>
      </template>
      <template v-else>
        <button class="context-menu-item" @click="handleContextAction('restore')">
          <RotateCcwIcon :size="14" class="mr-2" />
          元に戻す
        </button>
        <button class="context-menu-item danger" @click="handleContextAction('delete')">
          <Trash2Icon :size="14" class="mr-2" />
          完全に削除
        </button>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { FileTextIcon, StarIcon, PinIcon, Trash2Icon, RotateCcwIcon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()

const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  noteId: '',
  isTrashed: false,
  isFavorite: false,
  isPinned: false,
})

function showContextMenu(event: MouseEvent, note: any) {
  contextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
    noteId: note.id,
    isTrashed: note.isTrashed,
    isFavorite: note.isFavorite,
    isPinned: note.isPinned,
  }
  
  // Close menu when clicking elsewhere
  document.addEventListener('click', closeContextMenu)
}

function closeContextMenu() {
  contextMenu.value.visible = false
  document.removeEventListener('click', closeContextMenu)
}

function handleContextAction(action: 'favorite' | 'pin' | 'trash' | 'restore' | 'delete') {
  const id = contextMenu.value.noteId
  if (!id) return
  
  switch (action) {
    case 'favorite':
      noteStore.toggleFavorite(id)
      break
    case 'pin':
      noteStore.togglePinned(id)
      break
    case 'trash':
      noteStore.trashNote(id)
      break
    case 'restore':
      noteStore.restoreNote(id)
      break
    case 'delete':
      noteStore.permanentlyDeleteNote(id)
      break
  }
  closeContextMenu()
}

const sectionTitle = computed(() => {
  if (notebookStore.activeNotebookId) {
    const nb = notebookStore.notebooks.find(n => n.id === notebookStore.activeNotebookId)
    return nb ? nb.name : 'すべてのノート'
  }
  switch (appStore.sidebarSection) {
    case 'favorites': return 'お気に入り'
    case 'pinned': return 'ピン留め'
    case 'trash': return 'ゴミ箱'
    default: return 'すべてのノート'
  }
})

const displayedNotes = computed(() => {
  let list = []
  switch (appStore.sidebarSection) {
    case 'favorites': list = noteStore.favoriteNotes; break
    case 'pinned': list = noteStore.pinnedNotes; break
    case 'trash': list = noteStore.trashedNotes; break
    default: list = noteStore.activeNotes; break
  }
  if (notebookStore.activeNotebookId) {
    list = list.filter(n => n.notebookId === notebookStore.activeNotebookId)
  }
  return list
})

function formatDate(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  if (diffDays === 0) return d.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' })
  if (diffDays < 7) return `${diffDays}日前`
  return d.toLocaleDateString('ja-JP', { month: 'short', day: 'numeric' })
}
</script>

<style scoped>
.context-menu {
  position: fixed;
  z-index: 9999;
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  padding: 4px 0;
  min-width: 160px;
}

.context-menu-item {
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

.context-menu-item:hover {
  background-color: var(--bg-hover);
}

.context-menu-item.danger {
  color: var(--color-danger);
}

.context-menu-item.danger:hover {
  background-color: rgba(248, 81, 73, 0.1);
}

.context-menu-divider {
  height: 1px;
  background-color: var(--border);
  margin: 4px 0;
}

.mr-2 {
  margin-right: 8px;
}

.filled {
  fill: currentColor;
}
</style>
