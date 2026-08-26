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

      <span class="notebook-name">
        {{ node.name }}
        <LockKeyholeIcon v-if="node.locked" class="notebook-lock-icon" :size="13" aria-label="ロック中" />
        <LockIcon v-else-if="node.protected" class="notebook-lock-icon" :size="13" aria-label="保護中" />
      </span>

      <div class="notebook-actions" @click.stop>
        <button class="notebook-action-btn" type="button" title="子ノートブックを追加" @click="openChildCreateModal">
          <PlusIcon :size="12" />
        </button>
        <PopoverRoot v-model:open="isEditPopoverOpen">
          <PopoverTrigger as-child>
            <button class="notebook-action-btn" type="button" title="編集・ロック設定" @click="openEditor">
              <Edit2Icon :size="12" />
            </button>
          </PopoverTrigger>
          <PopoverPortal>
            <PopoverContent
              class="notebook-edit-popover"
              side="right"
              align="start"
              :side-offset="6"
              @click.stop
            >
              <form class="notebook-edit-form" @submit.prevent="saveNotebook">
                <label :for="`notebook-name-${node.id}`">名前</label>
                <input :id="`notebook-name-${node.id}`" ref="inputRef" v-model="editName" type="text" maxlength="100" />
                <ContentLockControls
                  ref="lockControlsRef"
                  :target="{ type: 'notebook', id: node.id }"
                  :target-label="node.name"
                  defer-save
                  @changed="refreshAfterLockChange"
                />
                <p v-if="editorError" class="notebook-edit-error" role="alert">{{ editorError }}</p>
                <div class="notebook-edit-actions">
                  <button type="button" :disabled="isSavingEditor" @click="cancelEditor">キャンセル</button>
                  <button class="save-notebook-button" type="submit" :disabled="isSavingEditor">
                    {{ isSavingEditor ? '保存中...' : '保存' }}
                  </button>
                </div>
              </form>
            </PopoverContent>
          </PopoverPortal>
        </PopoverRoot>
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
import { Edit2Icon, LockIcon, LockKeyholeIcon, PlusIcon, Trash2Icon } from '@lucide/vue'
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
import ContentLockControls from './ContentLockControls.vue'

const props = defineProps<{
  node: NotebookNode
}>()

const notebookStore = useNotebookStore()
const appStore = useAppStore()
const noteStore = useNoteStore()

const isEditPopoverOpen = ref(false)
// Keep the existing drag contract: editing, including the new popover form,
// disables notebook hierarchy drag and drop.
const isEditing = computed(() => isEditPopoverOpen.value)
const editName = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
type ContentLockEditorHandle = {
  save: () => Promise<boolean>
  reset: () => void
}
const lockControlsRef = ref<ContentLockEditorHandle | null>(null)
const isSavingEditor = ref(false)
const editorError = ref('')
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
  if (isEditPopoverOpen.value) return
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

async function selectNotebook() {
  if (!(await notebookStore.selectNotebook(props.node.id))) return
  appStore.setSidebarSection('notes')
  await noteStore.fetchNotes([], null)
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

function openEditor() {
  lockControlsRef.value?.reset()
  editorError.value = ''
  editName.value = props.node.name
  isEditPopoverOpen.value = true
  nextTick(() => {
    inputRef.value?.focus()
    inputRef.value?.select()
  })
}

async function saveNotebook() {
  if (isSavingEditor.value) return
  if (!lockControlsRef.value) {
    editorError.value = 'ロック設定の読み込みが完了していません。編集画面を開き直してください。'
    return
  }
  editorError.value = ''
  isSavingEditor.value = true
  try {
    if (!(await lockControlsRef.value.save())) return

    const trimmed = editName.value.trim()
    if (trimmed && trimmed !== props.node.name) {
      await notebookStore.renameNotebook(props.node.id, trimmed)
    }
    isEditPopoverOpen.value = false
  } finally {
    isSavingEditor.value = false
  }
}

function cancelEditor() {
  if (isSavingEditor.value) return
  lockControlsRef.value?.reset()
  editorError.value = ''
  isEditPopoverOpen.value = false
}

async function refreshAfterLockChange() {
  await notebookStore.fetchNotebooks()
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

.notebook-name {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.notebook-lock-icon {
  flex: 0 0 auto;
  color: var(--brand-primary);
}

/* PopoverContent is teleported outside this component, so its root must not
   rely on Vue's scoped attribute to receive the opaque surface styles. */
:global(.notebook-edit-popover) {
  z-index: 1200;
  display: grid;
  gap: 12px;
  width: min(340px, calc(100vw - 28px));
  padding: 12px;
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  background-color: var(--bg-sidebar);
  opacity: 1;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.32);
}

.notebook-edit-form {
  display: grid;
  gap: 7px;
}

.notebook-edit-form label {
  color: var(--text-secondary);
  font-size: 12px;
}

.notebook-edit-form input {
  min-width: 0;
  padding: 7px 8px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
}

.notebook-edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.notebook-edit-error {
  margin: 0;
  color: var(--color-danger, #c0392b);
  font-size: 12px;
  line-height: 1.45;
}

.notebook-edit-actions button {
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-editor);
  color: var(--text-primary);
  cursor: pointer;
  font-size: 12px;
}

.notebook-edit-actions .save-notebook-button {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: white;
}

</style>
