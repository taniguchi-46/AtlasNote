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

        <!-- Context actions -->
        <div class="note-item-actions">
          <button
            v-if="!note.isTrashed"
            class="action-btn"
            type="button"
            :title="note.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加'"
            @click.stop="noteStore.toggleFavorite(note.id)"
          >
            <StarIcon :size="14" :class="{ filled: note.isFavorite }" />
          </button>
          <button
            v-if="!note.isTrashed"
            class="action-btn"
            type="button"
            :title="note.isPinned ? 'ピン留めを外す' : 'ピン留め'"
            @click.stop="noteStore.togglePinned(note.id)"
          >
            <PinIcon :size="14" :class="{ filled: note.isPinned }" />
          </button>
          <button
            v-if="!note.isTrashed"
            class="action-btn danger"
            type="button"
            title="ゴミ箱へ移動"
            @click.stop="noteStore.trashNote(note.id)"
          >
            <Trash2Icon :size="14" />
          </button>
          <template v-else>
            <button
              class="action-btn"
              type="button"
              title="復元"
              @click.stop="noteStore.restoreNote(note.id)"
            >
              <RotateCcwIcon :size="14" />
            </button>
            <button
              class="action-btn danger"
              type="button"
              title="完全に削除"
              @click.stop="noteStore.permanentlyDeleteNote(note.id)"
            >
              <Trash2Icon :size="14" />
            </button>
          </template>
        </div>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { FileTextIcon, StarIcon, PinIcon, Trash2Icon, RotateCcwIcon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'

const noteStore = useNoteStore()
const appStore = useAppStore()

const sectionTitle = computed(() => {
  switch (appStore.sidebarSection) {
    case 'favorites': return 'お気に入り'
    case 'pinned': return 'ピン留め'
    case 'trash': return 'ゴミ箱'
    default: return 'すべてのノート'
  }
})

const displayedNotes = computed(() => {
  switch (appStore.sidebarSection) {
    case 'favorites': return noteStore.favoriteNotes
    case 'pinned': return noteStore.pinnedNotes
    case 'trash': return noteStore.trashedNotes
    default: return noteStore.activeNotes
  }
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
