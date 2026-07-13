<template>
  <section class="note-list-pane" aria-label="ノート一覧">
    <!-- Header -->
    <div class="note-list-header">
      <h2 class="note-list-title">{{ sectionTitle }}</h2>
      <span class="note-list-count">{{ displayedCount }}</span>
      <button
        v-if="!isTrashSection"
        id="btn-new-note"
        class="note-list-new-note-btn"
        type="button"
        :disabled="noteStore.isSaving"
        @click="createNewNote"
      >
        <span>新規</span>
        <PlusIcon :size="15" />
      </button>
      <button
        v-if="isTrashSection"
        class="empty-trash-btn"
        type="button"
        :disabled="noteStore.isSaving || noteStore.trashedNotes.length === 0"
        @click="emptyTrash"
      >
        ゴミ箱を空にする
      </button>
    </div>

    <p v-if="noteStore.error" class="note-list-error" role="alert">
      {{ noteStore.error }}
    </p>
    <p v-if="searchStore.error" class="note-list-error" role="alert">
      {{ searchStore.error }}
    </p>

    <!-- Loading -->
    <div v-if="(noteStore.isLoading || searchStore.isSearching) && displayedNotes.length === 0" class="note-list-empty">
      <div class="spinner" aria-label="読み込み中..." />
    </div>

    <!-- Empty state -->
    <div v-else-if="displayedNotes.length === 0" class="note-list-empty">
      <FileTextIcon :size="32" class="empty-icon" />
      <p class="empty-label">{{ searchStore.isActive ? '検索結果がありません' : 'ノートはありません' }}</p>
    </div>

    <p v-if="searchStore.isActive && searchStore.hasNext" class="note-list-search-limit">
      検索結果が多いため、追加の結果を読み込めます。
    </p>
    <button
      v-if="searchStore.isActive && searchStore.hasNext"
      class="note-list-search-more"
      type="button"
      :disabled="searchStore.isSearching"
      @click="searchStore.nextPage()"
    >
      {{ searchStore.isSearching ? '読み込み中...' : '次の検索結果を読み込む' }}
    </button>

    <!-- Note items -->
    <ul v-if="displayedNotes.length > 0" class="note-list" role="list">
      <ContextMenuRoot
        v-for="note in displayedNotes"
        :key="note.id"
        @update:open="handleContextMenuOpen($event, note)"
      >
        <ContextMenuTrigger as-child>
          <li
            class="note-item"
            :class="{
              'is-active': noteStore.activeNote?.id === note.id,
              'is-selected': selectedNoteIds.has(note.id),
            }"
            role="listitem"
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
              <p v-if="searchSnippet(note.id)" class="note-item-snippet">
                {{ searchSnippet(note.id) }}
              </p>
            </button>
          </li>
        </ContextMenuTrigger>

        <ContextMenuPortal>
          <ContextMenuContent class="context-menu" :data-theme="appStore.theme">
            <ContextMenuLabel v-if="contextMenu.targetIds.length > 1" class="context-menu-label">
              {{ contextMenu.targetIds.length }}件を選択中
            </ContextMenuLabel>
            <template v-if="!contextMenu.isTrashed">
              <template v-if="contextMenu.targetIds.length === 1">
                <ContextMenuItem class="context-menu-item" @select="handleContextAction('favorite')">
                  <StarIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isFavorite }" />
                  {{ contextMenu.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加' }}
                </ContextMenuItem>
                <ContextMenuItem class="context-menu-item" @select="handleContextAction('pin')">
                  <PinIcon :size="14" class="mr-2" :class="{ filled: contextMenu.isPinned }" />
                  {{ contextMenu.isPinned ? 'ピン留めを外す' : 'ピン留めする' }}
                </ContextMenuItem>
                <ContextMenuSeparator class="context-menu-divider" />
              </template>

              <ContextMenuSub>
                <ContextMenuSubTrigger class="context-menu-item">
                  <FolderInputIcon :size="14" class="mr-2" />
                  ノートブックへ移動
                  <ChevronRightIcon :size="14" class="context-menu-chevron" />
                </ContextMenuSubTrigger>
                <ContextMenuSubContent
                  class="context-submenu-panel"
                  :data-theme="appStore.theme"
                  :side-offset="4"
                  :align-offset="-4"
                >
                  <ContextMenuItem class="context-menu-item" @select="handleMoveToNotebook(null)">
                    未分類
                  </ContextMenuItem>
                  <ContextMenuItem
                    v-for="notebook in notebookOptions"
                    :key="notebook.id"
                    class="context-menu-item"
                    @select="handleMoveToNotebook(notebook.id)"
                  >
                    {{ notebook.label }}
                  </ContextMenuItem>
                </ContextMenuSubContent>
              </ContextMenuSub>

              <ContextMenuSeparator class="context-menu-divider" />
              <ContextMenuItem class="context-menu-item danger" @select="handleContextAction('trash')">
                <Trash2Icon :size="14" class="mr-2" />
                ゴミ箱へ移動
              </ContextMenuItem>
            </template>
            <template v-else>
              <ContextMenuItem class="context-menu-item" @select="handleContextAction('restore')">
                <RotateCcwIcon :size="14" class="mr-2" />
                元に戻す
              </ContextMenuItem>
              <ContextMenuItem class="context-menu-item danger" @select="handleContextAction('delete')">
                <Trash2Icon :size="14" class="mr-2" />
                完全に削除
              </ContextMenuItem>
            </template>
          </ContextMenuContent>
        </ContextMenuPortal>
      </ContextMenuRoot>
    </ul>
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
import {
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from 'reka-ui'
import type { note } from '../../wailsjs/go/models'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useSearchStore } from '../stores/useSearchStore'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const searchStore = useSearchStore()
const selectedNoteIds = ref<Set<string>>(new Set())
const lastSelectedNoteId = ref<string | null>(null)

const contextMenu = ref({
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

function prepareContextMenu(note: note.Summary) {
  const displayedIds = new Set(displayedNotes.value.map(n => n.id))
  const targetIds = selectedNoteIds.value.has(note.id)
    ? Array.from(selectedNoteIds.value).filter(id => displayedIds.has(id))
    : [note.id]

  if (!selectedNoteIds.value.has(note.id)) {
    selectedNoteIds.value = new Set([note.id])
    lastSelectedNoteId.value = note.id
  }

  contextMenu.value = {
    noteId: note.id,
    targetIds,
    isTrashed: note.isTrashed,
    isFavorite: note.isFavorite,
    isPinned: note.isPinned,
  }
}

function handleContextMenuOpen(open: boolean, note: note.Summary) {
  if (open) prepareContextMenu(note)
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
  if (searchStore.isActive) await searchStore.refresh()
}

async function handleMoveToNotebook(notebookId: string | null) {
  const targetIds = contextMenu.value.targetIds
  if (targetIds.length === 0) return

  await noteStore.moveNotesToNotebook(targetIds, notebookId)
  if (searchStore.isActive) await searchStore.refresh()
  clearSelectedNotes(targetIds)
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
  if (searchStore.isActive) await searchStore.refresh()
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
  if (searchStore.isActive) return '検索結果'
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
  if (searchStore.isActive) {
    let list = searchStore.items.map(item => item.note)
    switch (appStore.sidebarSection) {
      case 'uncategorized': list = list.filter(n => !n.notebookId); break
      case 'favorites': list = list.filter(n => n.isFavorite && !n.isTrashed); break
      case 'pinned': list = list.filter(n => n.isPinned && !n.isTrashed); break
      case 'trash': list = list.filter(n => n.isTrashed); break
      default: list = list.filter(n => !n.isTrashed); break
    }
    if (notebookStore.activeNotebookId) {
      list = list.filter(n => n.notebookId === notebookStore.activeNotebookId)
    }
    return list
  }

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

const displayedCount = computed(() => {
  if (!searchStore.isActive) return displayedNotes.value.length
  if (['uncategorized', 'favorites', 'pinned'].includes(appStore.sidebarSection)) {
    return displayedNotes.value.length
  }
  return searchStore.total
})
const searchItemsById = computed(() => new Map(searchStore.items.map(item => [item.note.id, item])))

function searchSnippet(noteId: string) {
  return (searchItemsById.value.get(noteId)?.snippet ?? '').replace(/<\/?mark>/g, '')
}

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
.note-list-new-note-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 36px;
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

:global(.context-menu) {
  z-index: 1300;
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  padding: 4px 0;
  min-width: 160px;
  outline: none;
}

.empty-trash-btn {
  display: inline-flex;
  align-items: center;
  height: 36px;
  flex-shrink: 0;
  padding: 0 12px;
  border-radius: 6px;
  color: var(--color-danger);
  font-size: 13px;
  font-weight: 700;
  transition: background 0.12s, opacity 0.12s;
}

.empty-trash-btn:hover:not(:disabled) {
  background: rgba(248, 81, 73, 0.1);
}

.empty-trash-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.note-list-error {
  margin: 8px 12px 0;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--color-danger) 45%, transparent);
  border-radius: 6px;
  color: var(--color-danger);
  font-size: 12px;
}

.note-list-search-limit {
  margin: 8px 12px 0;
  color: var(--text-muted);
  font-size: 11px;
}

.note-list-search-more {
  align-self: center;
  margin: 8px 12px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.note-list-search-more:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.note-list-search-more:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.note-item-snippet {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 11px;
  line-height: 1.35;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:global(.context-menu-item) {
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

:global(.context-menu-item[data-highlighted]) {
  background-color: var(--bg-hover);
  outline: none;
}

:global(.context-menu-item.danger) {
  color: var(--color-danger);
}

:global(.context-menu-item.danger[data-highlighted]) {
  background-color: rgba(248, 81, 73, 0.1);
}

:global(.context-menu-divider) {
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

:global(.context-menu-label) {
  padding: 6px 12px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
}

:global(.context-menu-chevron) {
  margin-left: auto;
  color: var(--text-muted);
}

:global(.context-submenu-panel) {
  z-index: 1301;
  min-width: 180px;
  max-height: 260px;
  overflow-y: auto;
  padding: 4px 0;
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  outline: none;
}
</style>
