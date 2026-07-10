<template>
  <section class="note-list-pane" aria-label="ノート一覧">
    <div class="note-list-action-bar">
      <button
        id="btn-new-note"
        class="note-list-new-note-btn"
        type="button"
        :disabled="noteStore.isSaving"
        @click="createNewNote"
      >
        <PlusIcon :size="15" />
        <span>新しいノート</span>
      </button>
    </div>

    <!-- Header -->
    <div class="note-list-header">
      <h2 class="note-list-title">{{ sectionTitle }}</h2>
      <button
        v-if="isTrashSection"
        class="empty-trash-btn"
        type="button"
        :disabled="noteStore.isSaving || noteStore.trashedNotes.length === 0"
        @click="emptyTrash"
      >
        ゴミ箱を空にする
      </button>
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
        :class="{
          'is-active': noteStore.activeNote?.id === note.id,
          'is-selected': selectedNoteIds.has(note.id),
        }"
        role="listitem"
        @contextmenu.prevent="showContextMenu($event, note)"
      >
        <button
          :id="`note-item-${note.id}`"
          class="note-item-btn"
          type="button"
          @click="handleNoteClick($event, note)"
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
      @click.stop
    >
      <div v-if="contextMenu.targetIds.length > 1" class="context-menu-label">
        {{ contextMenu.targetIds.length }}件を選択中
      </div>
      <template v-if="!contextMenu.isTrashed">
        <template v-if="contextMenu.targetIds.length === 1">
          <button class="context-menu-item" @click="handleContextAction('favorite')">
            <StarIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isFavorite }" />
            {{ contextMenu.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加' }}
          </button>
          <button class="context-menu-item" @click="handleContextAction('pin')">
            <PinIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isPinned }" />
            {{ contextMenu.isPinned ? 'ピン留めを外す' : 'ピン留めする' }}
          </button>
          <div class="context-menu-divider"></div>
        </template>
        <div class="context-menu-submenu">
          <button class="context-menu-item" type="button">
            <FolderInputIcon :size="14" class="mr-2" />
            ノートブックへ移動
            <ChevronRightIcon :size="14" class="context-menu-chevron" />
          </button>
          <div class="context-submenu-panel">
            <button
              class="context-menu-item"
              type="button"
              @click="handleMoveToNotebook(null)"
            >
              未分類
            </button>
            <button
              v-for="notebook in notebookOptions"
              :key="notebook.id"
              class="context-menu-item"
              type="button"
              @click="handleMoveToNotebook(notebook.id)"
            >
              {{ notebook.label }}
            </button>
          </div>
        </div>
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
import {
  ChevronRightIcon,
  FileTextIcon,
  FolderInputIcon,
  PlusIcon,
  StarIcon,
  PinIcon,
  Trash2Icon,
  RotateCcwIcon,
} from '@lucide/vue'
import type { note } from '../../wailsjs/go/models'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const selectedNoteIds = ref<Set<string>>(new Set())
const lastSelectedNoteId = ref<string | null>(null)

const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  noteId: '',
  targetIds: [] as string[],
  isTrashed: false,
  isFavorite: false,
  isPinned: false,
})

function handleNoteClick(event: MouseEvent, note: note.Summary) {
  if (event.shiftKey) {
    toggleNoteSelection(note.id)
    return
  }

  selectedNoteIds.value = new Set()
  lastSelectedNoteId.value = note.id
  noteStore.selectNote(note.id)
}

function toggleNoteSelection(noteId: string) {
  const nextSelectedIds = new Set(selectedNoteIds.value)

  if (nextSelectedIds.has(noteId)) {
    nextSelectedIds.delete(noteId)
    selectedNoteIds.value = nextSelectedIds
    return
  }

  nextSelectedIds.add(noteId)
  selectedNoteIds.value = nextSelectedIds
  lastSelectedNoteId.value = noteId
}

function showContextMenu(event: MouseEvent, note: note.Summary) {
  const displayedIds = new Set(displayedNotes.value.map(n => n.id))
  const targetIds = selectedNoteIds.value.has(note.id)
    ? Array.from(selectedNoteIds.value).filter(id => displayedIds.has(id))
    : [note.id]

  if (!selectedNoteIds.value.has(note.id)) {
    selectedNoteIds.value = new Set([note.id])
    lastSelectedNoteId.value = note.id
  }

  contextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
    noteId: note.id,
    targetIds,
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

async function handleContextAction(action: 'favorite' | 'pin' | 'trash' | 'restore' | 'delete') {
  const id = contextMenu.value.noteId
  if (!id) return
  const targetIds = contextMenu.value.targetIds.length > 0 ? contextMenu.value.targetIds : [id]
  
  switch (action) {
    case 'favorite':
      await noteStore.toggleFavorite(id)
      break
    case 'pin':
      await noteStore.togglePinned(id)
      break
    case 'trash':
      await noteStore.trashNotes(targetIds)
      clearSelectedNotes(targetIds)
      break
    case 'restore':
      await noteStore.restoreNotes(targetIds)
      clearSelectedNotes(targetIds)
      break
    case 'delete':
      await noteStore.permanentlyDeleteNotes(targetIds)
      clearSelectedNotes(targetIds)
      break
  }
  closeContextMenu()
}

async function handleMoveToNotebook(notebookId: string | null) {
  const targetIds = contextMenu.value.targetIds
  if (targetIds.length === 0) return

  await noteStore.moveNotesToNotebook(targetIds, notebookId)
  clearSelectedNotes(targetIds)
  closeContextMenu()
}

function clearSelectedNotes(ids: string[]) {
  const idSet = new Set(ids)
  selectedNoteIds.value = new Set(Array.from(selectedNoteIds.value).filter(id => !idSet.has(id)))
  if (lastSelectedNoteId.value && idSet.has(lastSelectedNoteId.value)) {
    lastSelectedNoteId.value = null
  }
}

async function emptyTrash() {
  const count = noteStore.trashedNotes.length
  if (count === 0) return

  const confirmed = window.confirm(
    `ゴミ箱内の${count}件のノートを完全に削除します。この操作は元に戻せません。`,
  )
  if (!confirmed) return

  await noteStore.emptyTrash()
  selectedNoteIds.value = new Set()
  lastSelectedNoteId.value = null
}

function createNewNote() {
  noteStore.newNote('新しいノート', '', notebookStore.activeNotebookId)
}

const isTrashSection = computed(() =>
  appStore.sidebarSection === 'trash' && !notebookStore.activeNotebookId
)

const sectionTitle = computed(() => {
  if (notebookStore.activeNotebookId) {
    const nb = notebookStore.notebooks.find(n => n.id === notebookStore.activeNotebookId)
    return nb ? nb.name : 'すべてのノート'
  }
  switch (appStore.sidebarSection) {
    case 'uncategorized': return '未分類'
    case 'favorites': return 'お気に入り'
    case 'pinned': return 'ピン留め'
    case 'trash': return 'ゴミ箱'
    default: return 'すべてのノート'
  }
})

const displayedNotes = computed(() => {
  let list: note.Summary[] = []
  switch (appStore.sidebarSection) {
    case 'uncategorized': list = noteStore.activeNotes.filter(n => !n.notebookId); break
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

const notebookOptions = computed(() => {
  const depthById = new Map<string, number>()
  const getDepth = (notebook: note.Notebook): number => {
    if (!notebook.parentId) return 0
    if (depthById.has(notebook.id)) return depthById.get(notebook.id) ?? 0

    const parent = notebookStore.notebooks.find(n => n.id === notebook.parentId)
    const depth = parent ? getDepth(parent) + 1 : 0
    depthById.set(notebook.id, depth)
    return depth
  }

  return notebookStore.notebooks.map(notebook => ({
    id: notebook.id,
    label: `${'  '.repeat(getDepth(notebook))}${notebook.name}`,
  }))
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
.note-list-action-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.note-list-new-note-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 6px;
  background: var(--brand-primary);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  transition: background 0.15s, opacity 0.12s;
}

.note-list-new-note-btn:hover:not(:disabled) {
  background: var(--brand-hover);
}

.note-list-new-note-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

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

.empty-trash-btn {
  flex-shrink: 0;
  padding: 4px 8px;
  border-radius: 6px;
  color: var(--color-danger);
  font-size: 12px;
  font-weight: 600;
  transition: background 0.12s, opacity 0.12s;
}

.empty-trash-btn:hover:not(:disabled) {
  background: rgba(248, 81, 73, 0.1);
}

.empty-trash-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
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

.note-item.is-selected {
  background: var(--bg-active);
  outline: 1px solid var(--border-strong);
}

.note-item.is-selected .note-item-title {
  color: var(--text-active);
}

.context-menu-label {
  padding: 6px 12px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.context-menu-submenu {
  position: relative;
}

.context-menu-submenu:hover .context-submenu-panel,
.context-menu-submenu:focus-within .context-submenu-panel {
  display: block;
}

.context-menu-chevron {
  margin-left: auto;
  color: var(--text-muted);
}

.context-submenu-panel {
  display: none;
  position: absolute;
  top: -4px;
  left: 100%;
  z-index: 10000;
  min-width: 180px;
  max-height: 260px;
  overflow-y: auto;
  padding: 4px 0;
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}
</style>
