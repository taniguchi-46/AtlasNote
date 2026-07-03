<template>
  <section class="editor-pane" aria-label="エディタ">
    <!-- Empty state -->
    <div v-if="!noteStore.activeNote" class="editor-empty">
      <div class="editor-empty-icon">
        <FileTextIcon :size="48" />
      </div>
      <p class="editor-empty-title">ノートを選択してください</p>
      <p class="editor-empty-sub">左のリストからノートを選ぶか、新規ノートを作成してください</p>
      <button
        id="btn-new-note-editor"
        class="primary-btn"
        type="button"
        @click="noteStore.newNote()"
      >
        新規ノート
      </button>
    </div>

    <!-- Editor -->
    <template v-else>
      <!-- Toolbar -->
      <div class="editor-toolbar">
        <input
          id="note-title-input"
          v-model="localTitle"
          class="title-input"
          type="text"
          placeholder="タイトル"
          @blur="handleTitleSave"
          @keydown.enter="handleTitleSave"
        />
        <div class="toolbar-actions">
          <span v-if="noteStore.isSaving" class="saving-indicator">保存中…</span>
          <span v-else-if="savedMessage" class="saved-indicator">保存済み</span>
          <button
            class="icon-btn"
            type="button"
            :title="noteStore.activeNote.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加'"
            @click="noteStore.toggleFavorite(noteStore.activeNote.id)"
          >
            <StarIcon :size="18" :class="{ filled: noteStore.activeNote.isFavorite }" />
          </button>
          <button
            class="icon-btn"
            type="button"
            :title="noteStore.activeNote.isPinned ? 'ピン留めを外す' : 'ピン留め'"
            @click="noteStore.togglePinned(noteStore.activeNote.id)"
          >
            <PinIcon :size="18" :class="{ filled: noteStore.activeNote.isPinned }" />
          </button>
          <button
            class="icon-btn danger"
            type="button"
            title="ゴミ箱へ移動"
            @click="noteStore.trashNote(noteStore.activeNote.id)"
          >
            <Trash2Icon :size="18" />
          </button>
        </div>
      </div>

      <!-- Text area (placeholder for Tiptap) -->
      <div class="editor-body">
        <textarea
          id="note-content-textarea"
          v-model="localContent"
          class="content-textarea"
          placeholder="ここにMarkdownで書き始めましょう…"
          spellcheck="false"
          @input="scheduleAutoSave"
        />
      </div>

      <!-- Status bar -->
      <div class="editor-statusbar">
        <span>{{ charCount }} 文字</span>
        <span>更新: {{ formatDate(noteStore.activeNote.updatedAt) }}</span>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { FileTextIcon, StarIcon, PinIcon, Trash2Icon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'

const noteStore = useNoteStore()

const localTitle = ref('')
const localContent = ref('')
const savedMessage = ref(false)
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null

watch(() => noteStore.activeNote, (note) => {
  if (note) {
    localTitle.value = note.title
    localContent.value = note.content
  }
}, { immediate: true })

const charCount = computed(() => localContent.value.length)

function handleTitleSave() {
  if (!noteStore.activeNote) return
  if (localTitle.value === noteStore.activeNote.title) return
  noteStore.saveNote(noteStore.activeNote.id, { title: localTitle.value })
    .then(() => showSaved())
}

function scheduleAutoSave() {
  if (autoSaveTimer) clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(() => {
    if (!noteStore.activeNote) return
    noteStore.saveNote(noteStore.activeNote.id, {
      title: localTitle.value,
      content: localContent.value,
    }).then(() => showSaved())
  }, 1000)
}

function showSaved() {
  savedMessage.value = true
  setTimeout(() => { savedMessage.value = false }, 2000)
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('ja-JP', {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}
</script>
