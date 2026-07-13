<template>
  <section class="tag-manager" aria-label="タグ管理">
    <div class="tag-manager-header">
      <span>タグ</span>
      <button
        class="add-tag-btn"
        type="button"
        title="タグを追加"
        aria-label="タグを追加"
        :disabled="tagStore.isMutating"
        @click="isCreateModalOpen = true"
      >
        <PlusIcon :size="14" />
      </button>
    </div>

    <p v-if="tagStore.tags.length === 0" class="tag-empty">
      タグはまだありません。
    </p>

    <ul v-else class="tag-list">
      <li v-for="tag in tagStore.tags" :key="tag.id" class="tag-list-item">
        <template v-if="editingTagID === tag.id">
          <input
            v-model="editingName"
            class="tag-rename-input"
            type="text"
            :aria-label="`${tag.name} の新しい名前`"
            :disabled="tagStore.isMutating"
            @keydown.enter.prevent="saveRename(tag.id)"
            @keydown.esc="cancelRename"
          />
          <div class="tag-list-actions">
            <button type="button" :disabled="tagStore.isMutating" title="名前を保存" @click="saveRename(tag.id)">
              <CheckIcon :size="14" />
            </button>
            <button type="button" :disabled="tagStore.isMutating" title="名前の変更をやめる" @click="cancelRename">
              <XIcon :size="14" />
            </button>
          </div>
        </template>
        <template v-else>
          <button
            class="tag-list-name"
            :class="{ 'is-active': noteStore.activeTagId === tag.id }"
            type="button"
            :title="`${tag.name}のノートを表示`"
            @click="selectTag(tag.id)"
          >
            <TagIcon :size="13" aria-hidden="true" />
            <span class="tag-list-name-text">{{ tag.name }}</span>
          </button>
          <div class="tag-list-actions">
            <button type="button" :disabled="tagStore.isMutating" title="タグ名を変更" @click="startRename(tag)">
              <PencilIcon :size="13" />
            </button>
            <button class="danger" type="button" :disabled="tagStore.isMutating" title="タグを削除" @click="deleteTag(tag)">
              <Trash2Icon :size="13" />
            </button>
          </div>
        </template>
      </li>
    </ul>
  </section>

  <TagCreateModal :open="isCreateModalOpen" @close="isCreateModalOpen = false" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { CheckIcon, PencilIcon, PlusIcon, TagIcon, Trash2Icon, XIcon } from '@lucide/vue'
import type { note } from '../../wailsjs/go/models'
import TagCreateModal from './TagCreateModal.vue'
import { useTagStore } from '../stores/useTagStore'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useSearchStore } from '../stores/useSearchStore'

const tagStore = useTagStore()
const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const searchStore = useSearchStore()
const isCreateModalOpen = ref(false)
const editingTagID = ref<string | null>(null)
const editingName = ref('')

async function selectTag(tagID: string) {
  appStore.setSidebarSection('notes')
  notebookStore.activeNotebookId = null
  searchStore.clear()
  await noteStore.fetchNotes([], tagID)
}

function startRename(tag: note.Tag) {
  editingTagID.value = tag.id
  editingName.value = tag.name
}

function cancelRename() {
  editingTagID.value = null
  editingName.value = ''
}

async function saveRename(id: string) {
  if (!editingName.value.trim()) return

  try {
    await tagStore.renameTag(id, editingName.value)
    cancelRename()
  } catch (_) {
    // Keep the entered value so the user can correct it after a validation error.
  }
}

async function deleteTag(tag: note.Tag) {
  const confirmed = window.confirm(
    `タグ「${tag.name}」を削除しますか？ノートとの関連は外れますが、ノート自体は削除されません。`,
  )
  if (!confirmed) return

  const wasActive = noteStore.activeTagId === tag.id
  try {
    await tagStore.removeTag(tag.id)
    if (wasActive) {
      noteStore.clearTagFilter()
      await noteStore.fetchNotes([], null)
    }
  } catch (_) {
    // The tag store reports the operation error through the notification center.
  }
}
</script>

<style scoped>
.tag-manager {
  display: flex;
  flex: 0 1 auto;
  flex-direction: column;
  min-height: 0;
  padding: 12px 10px 0;
  border-top: 1px solid var(--border);
}

.tag-manager-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 7px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.tag-rename-input {
  min-width: 0;
  height: 27px;
  padding: 4px 7px;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
}

.add-tag-btn,
.tag-list-actions button {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 27px;
  height: 27px;
  padding: 0;
  color: var(--text-secondary);
  background: transparent;
  border: 0;
  border-radius: 4px;
  cursor: pointer;
}

.add-tag-btn:hover:not(:disabled),
.tag-list-actions button:hover:not(:disabled) {
  background: var(--bg-hover);
}

.tag-list-actions button.danger:hover:not(:disabled) {
  color: var(--color-danger);
}

.add-tag-btn:disabled,
.tag-list-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.tag-empty {
  margin: 8px 2px;
  color: var(--text-secondary);
  font-size: 12px;
}

.tag-list {
  display: flex;
  flex-direction: column;
  gap: 1px;
  max-height: 160px;
  padding: 5px 0;
  margin: 0;
  overflow-y: auto;
  list-style: none;
}

.tag-list-item {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 1px;
}

.tag-list-actions {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 1px;
  margin-left: auto;
}

.tag-list-name {
  display: flex;
  flex: 1 1 0;
  align-items: center;
  min-width: 0;
  gap: 6px;
  width: auto;
  height: auto;
  padding: 5px 4px;
  overflow: hidden;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 4px;
  cursor: pointer;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.tag-list-name > svg {
  flex: 0 0 auto;
}

.tag-list-name-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag-list-name:hover {
  background: var(--bg-hover);
}

.tag-list-name.is-active {
  color: var(--text-active);
  background: var(--bg-active);
}

.tag-rename-input {
  flex: 1 1 0;
  min-width: 0;
}

.tag-rename-input:focus {
  outline: 2px solid color-mix(in srgb, var(--color-primary) 35%, transparent);
  outline-offset: -1px;
}
</style>
