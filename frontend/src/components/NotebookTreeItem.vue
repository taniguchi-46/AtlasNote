<template>
  <div class="notebook-tree-item">
    <div
      class="notebook-row"
      :class="{ 'is-active': notebookStore.activeNotebookId === node.id }"
      @click="selectNotebook"
    >
      <div class="icon-wrapper" @click.stop="toggleIconPicker">
        <img :src="currentIcon.src" :alt="currentIcon.label" class="notebook-icon" />

        <div v-if="isIconPickerOpen" class="icon-picker" @click.stop>
          <NotebookIconPicker
            :model-value="node.icon"
            @update:model-value="selectIcon"
          />
        </div>
      </div>

      <input
        v-if="isEditing"
        ref="inputRef"
        v-model="editName"
        class="notebook-rename-input"
        type="text"
        @blur="saveRename"
        @keydown.enter="saveRename"
      />
      <span v-else class="notebook-name">{{ node.name }}</span>

      <div class="notebook-actions" @click.stop>
        <button class="notebook-action-btn" type="button" title="子ノートブックを追加" @click="openChildCreateModal">
          <PlusIcon :size="12" />
        </button>
        <button class="notebook-action-btn" type="button" title="名前を変更" @click="startRename">
          <Edit2Icon :size="12" />
        </button>
        <button
          class="notebook-action-btn danger"
          type="button"
          title="削除"
          @click="openDeleteModal"
        >
          <Trash2Icon :size="12" />
        </button>
      </div>
    </div>

    <div v-if="node.children && node.children.length > 0" class="notebook-children">
      <NotebookTreeItem
        v-for="child in node.children"
        :key="child.id"
        :node="child"
      />
    </div>

    <NotebookCreateModal
      :open="isChildCreateModalOpen"
      :parent-id="node.id"
      @close="isChildCreateModalOpen = false"
    />

    <NotebookDeleteModal
      :open="isDeleteModalOpen"
      :notebook-name="node.name"
      :is-deleting="isDeleting"
      :error="deleteError"
      @cancel="closeDeleteModal"
      @confirm="deleteSelf"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { PlusIcon, Edit2Icon, Trash2Icon } from '@lucide/vue'
import { useNotebookStore, type NotebookNode } from '../stores/useNotebookStore'
import { useAppStore } from '../stores/useAppStore'
import type { NotebookDeleteMode } from '../api/notebooks'
import NotebookCreateModal from './NotebookCreateModal.vue'
import NotebookDeleteModal from './NotebookDeleteModal.vue'
import NotebookIconPicker from './NotebookIconPicker.vue'
import { resolveNotebookIcon } from '../utils/notebookIcons'

const props = defineProps<{
  node: NotebookNode
}>()

const notebookStore = useNotebookStore()
const appStore = useAppStore()

const isEditing = ref(false)
const editName = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const isChildCreateModalOpen = ref(false)
const isDeleteModalOpen = ref(false)
const isDeleting = ref(false)
const deleteError = ref('')
const isIconPickerOpen = ref(false)

const currentIcon = computed(() => resolveNotebookIcon(props.node.icon))

function toggleIconPicker() {
  isIconPickerOpen.value = !isIconPickerOpen.value
}

async function selectIcon(iconName: string) {
  await notebookStore.updateNotebookIcon(props.node.id, iconName)
  isIconPickerOpen.value = false
}

function selectNotebook() {
  notebookStore.activeNotebookId = props.node.id
  appStore.setSidebarSection('notes')
}

function openChildCreateModal() {
  isChildCreateModalOpen.value = true
}

function openDeleteModal() {
  deleteError.value = ''
  isDeleteModalOpen.value = true
}

function closeDeleteModal() {
  if (isDeleting.value) return
  isDeleteModalOpen.value = false
  deleteError.value = ''
}

function startRename() {
  editName.value = props.node.name
  isEditing.value = true
  nextTick(() => {
    inputRef.value?.focus()
  })
}

function saveRename() {
  if (!isEditing.value) return
  isEditing.value = false
  const trimmed = editName.value.trim()
  if (trimmed && trimmed !== props.node.name) {
    notebookStore.renameNotebook(props.node.id, trimmed)
  }
}

async function deleteSelf(mode: NotebookDeleteMode) {
  isDeleting.value = true
  deleteError.value = ''
  try {
    await notebookStore.removeNotebook(props.node.id, mode)
    isDeleteModalOpen.value = false
  } catch (e) {
    deleteError.value = e instanceof Error ? e.message : 'ノートブックの削除に失敗しました'
  } finally {
    isDeleting.value = false
  }
}
</script>

<style scoped>
.icon-wrapper {
  position: relative;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
}

.notebook-icon {
  width: 30px;
  height: 30px;
  border-radius: 6px;
  object-fit: cover;
}

.icon-picker {
  position: absolute;
  top: 28px;
  left: 0;
  z-index: 100;
  width: 260px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.32);
}
</style>
