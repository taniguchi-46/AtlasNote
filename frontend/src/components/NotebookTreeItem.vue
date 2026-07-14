<template>
  <div class="notebook-tree-item">
    <div
      class="notebook-row"
      :class="{
        'is-active': notebookStore.activeNotebookId === node.id,
        'is-dragging': notebookStore.draggedNotebookId === node.id,
        'is-drop-target': isDropTarget,
      }"
      :draggable="!isEditing"
      @click="selectNotebook"
      @dragstart="handleDragStart"
      @dragend="handleDragEnd"
      @dragover.stop="handleDragOver"
      @dragleave.stop="handleDragLeave"
      @drop.stop.prevent="handleDrop"
    >
      <PopoverRoot v-model:open="isIconPickerOpen">
        <PopoverTrigger as-child>
          <button class="icon-wrapper" type="button" title="アイコンを変更" @click.stop>
            <img :src="currentIcon.src" :alt="currentIcon.label" class="notebook-icon" />
          </button>
        </PopoverTrigger>
        <PopoverPortal>
          <PopoverContent
            class="icon-picker"
            side="right"
            align="start"
            :side-offset="6"
            @click.stop
          >
            <NotebookIconPicker
              :model-value="node.icon"
              @update:model-value="selectIcon"
            />
          </PopoverContent>
        </PopoverPortal>
      </PopoverRoot>

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
import { Edit2Icon, PlusIcon, Trash2Icon } from '@lucide/vue'
import {
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'
import { useNotebookStore, type NotebookNode } from '../stores/useNotebookStore'
import { useAppStore } from '../stores/useAppStore'
import { useNoteStore } from '../stores/useNoteStore'
import type { NotebookDeleteMode } from '../api/notebooks'
import NotebookCreateModal from './NotebookCreateModal.vue'
import NotebookDeleteModal from './NotebookDeleteModal.vue'
import NotebookIconPicker from './NotebookIconPicker.vue'
import { resolveNotebookIcon } from '../utils/notebookIcons'
import { wouldCreateNotebookCycle } from '../utils/notebookHierarchy'

const props = defineProps<{
  node: NotebookNode
}>()

const notebookStore = useNotebookStore()
const appStore = useAppStore()
const noteStore = useNoteStore()

const isEditing = ref(false)
const editName = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const isChildCreateModalOpen = ref(false)
const isDeleteModalOpen = ref(false)
const isDeleting = ref(false)
const deleteError = ref('')
const isIconPickerOpen = ref(false)
const isDropTarget = ref(false)

const currentIcon = computed(() => resolveNotebookIcon(props.node.icon))
const canAcceptDrop = computed(() => {
  const draggedId = notebookStore.draggedNotebookId
  return Boolean(
    draggedId
      && draggedId !== props.node.id
      && !wouldCreateNotebookCycle(notebookStore.notebooks, draggedId, props.node.id),
  )
})

function handleDragStart(event: DragEvent) {
  if (isEditing.value) return
  notebookStore.beginNotebookDrag(props.node.id)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', props.node.id)
  }
}

function handleDragEnd() {
  isDropTarget.value = false
  notebookStore.endNotebookDrag()
}

function handleDragOver(event: DragEvent) {
  if (!canAcceptDrop.value) {
    isDropTarget.value = false
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'none'
    return
  }
  isDropTarget.value = true
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function handleDragLeave() {
  isDropTarget.value = false
}

function handleDrop() {
  isDropTarget.value = false
  if (!canAcceptDrop.value) return
  const draggedId = notebookStore.draggedNotebookId
  if (!draggedId) return
  void notebookStore.moveNotebook(draggedId, props.node.id)
  notebookStore.endNotebookDrag()
}

async function selectIcon(iconName: string) {
  await notebookStore.updateNotebookIcon(props.node.id, iconName)
  isIconPickerOpen.value = false
}

function selectNotebook() {
  notebookStore.activeNotebookId = props.node.id
  appStore.setSidebarSection('notes')
  void noteStore.fetchNotes([], null)
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
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.notebook-icon {
  width: 30px;
  height: 30px;
  border-radius: 6px;
  object-fit: cover;
}

.icon-picker {
  z-index: 1200;
  width: 260px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.32);
}

</style>
