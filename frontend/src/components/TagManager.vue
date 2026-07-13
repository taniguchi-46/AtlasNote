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

    <input
      v-model="query"
      class="tag-search-input"
      type="search"
      placeholder="タグを検索"
      aria-label="タグを検索"
    />

    <p v-if="filteredTags.length === 0" class="tag-empty">
      {{ tagStore.tags.length === 0 ? 'タグはまだありません。' : '一致するタグはありません。' }}
    </p>

    <ul v-else class="tag-list">
      <li v-for="tag in filteredTags" :key="tag.id" class="tag-list-item">
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
          <button type="button" :disabled="tagStore.isMutating" title="名前を保存" @click="saveRename(tag.id)">
            <CheckIcon :size="14" />
          </button>
          <button type="button" :disabled="tagStore.isMutating" title="名前の変更をやめる" @click="cancelRename">
            <XIcon :size="14" />
          </button>
        </template>
        <template v-else>
          <span class="tag-list-name" :title="tag.name">
            <TagIcon :size="13" aria-hidden="true" />
            {{ tag.name }}
          </span>
          <button type="button" :disabled="tagStore.isMutating" title="タグ名を変更" @click="startRename(tag)">
            <PencilIcon :size="13" />
          </button>
          <button class="danger" type="button" :disabled="tagStore.isMutating" title="タグを削除" @click="deleteTag(tag)">
            <Trash2Icon :size="13" />
          </button>
        </template>
      </li>
    </ul>
  </section>

  <TagCreateModal :open="isCreateModalOpen" @close="isCreateModalOpen = false" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckIcon, PencilIcon, PlusIcon, TagIcon, Trash2Icon, XIcon } from '@lucide/vue'
import type { note } from '../../wailsjs/go/models'
import TagCreateModal from './TagCreateModal.vue'
import { useTagStore } from '../stores/useTagStore'

const tagStore = useTagStore()
const isCreateModalOpen = ref(false)
const query = ref('')
const editingTagID = ref<string | null>(null)
const editingName = ref('')

const filteredTags = computed(() => {
  const normalizedQuery = query.value.normalize('NFC').trim().toLocaleLowerCase()
  if (!normalizedQuery) return tagStore.tags

  return tagStore.tags.filter((tag) => (
    tag.name.normalize('NFC').toLocaleLowerCase().includes(normalizedQuery)
  ))
})

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

  try {
    await tagStore.removeTag(tag.id)
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

.tag-search-input,
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

.tag-search-input {
  width: 100%;
  margin-top: 0;
}

.add-tag-btn,
.tag-list-item button {
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
.tag-list-item button:hover:not(:disabled) {
  background: var(--bg-hover);
}

.tag-list-item button.danger:hover:not(:disabled) {
  color: var(--color-danger);
}

.add-tag-btn:disabled,
.tag-list-item button:disabled {
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

.tag-list-name {
  display: flex;
  flex: 1;
  align-items: center;
  min-width: 0;
  gap: 6px;
  padding: 5px 4px;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 12px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.tag-rename-input {
  flex: 1;
}

.tag-search-input:focus,
.tag-rename-input:focus {
  outline: 2px solid color-mix(in srgb, var(--color-primary) 35%, transparent);
  outline-offset: -1px;
}
</style>
