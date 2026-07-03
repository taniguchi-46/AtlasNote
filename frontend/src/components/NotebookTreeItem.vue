<template>
  <div class="notebook-tree-item">
    <div
      class="notebook-row"
      :class="{ 'is-active': notebookStore.activeNotebookId === node.id }"
      @click="selectNotebook"
    >
      <FolderIcon :size="14" class="notebook-icon" />
      
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

        <!-- Actions -->
      <div class="notebook-actions" @click.stop>
        <button class="notebook-action-btn" type="button" title="子ノートブックを追加" @click="startAddSubNotebook">
          <PlusIcon :size="12" />
        </button>
        <button class="notebook-action-btn" type="button" title="名前を変更" @click="startRename">
          <Edit2Icon :size="12" />
        </button>
        <button
          class="notebook-action-btn danger"
          type="button"
          :title="isConfirmingDelete ? 'もう一度押すと削除' : '削除'"
          @click="deleteSelf"
        >
          <Trash2Icon :size="12" />
        </button>
      </div>
    </div>

    <input
      v-if="isAddingChild"
      ref="childInputRef"
      v-model="childName"
      class="notebook-rename-input child-create-input"
      type="text"
      placeholder="子ノートブック名"
      @blur="saveSubNotebook"
      @keydown.enter="saveSubNotebook"
      @keydown.escape="cancelAddSubNotebook"
    />

    <!-- Children -->
    <div v-if="node.children && node.children.length > 0" class="notebook-children">
      <NotebookTreeItem
        v-for="child in node.children"
        :key="child.id"
        :node="child"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { FolderIcon, PlusIcon, Edit2Icon, Trash2Icon } from '@lucide/vue'
import { useNotebookStore, type NotebookNode } from '../stores/useNotebookStore'
import { useAppStore } from '../stores/useAppStore'

const props = defineProps<{
  node: NotebookNode
}>()

const notebookStore = useNotebookStore()
const appStore = useAppStore()

const isEditing = ref(false)
const editName = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const isAddingChild = ref(false)
const childName = ref('')
const childInputRef = ref<HTMLInputElement | null>(null)
const isConfirmingDelete = ref(false)

function selectNotebook() {
  notebookStore.activeNotebookId = props.node.id
  appStore.setSidebarSection('notes')
}

function startAddSubNotebook() {
  isAddingChild.value = true
  childName.value = ''
  nextTick(() => {
    childInputRef.value?.focus()
  })
}

function saveSubNotebook() {
  if (!isAddingChild.value) return
  const trimmed = childName.value.trim()
  isAddingChild.value = false
  childName.value = ''
  if (trimmed) {
    notebookStore.newNotebook(trimmed, props.node.id)
  }
}

function cancelAddSubNotebook() {
  isAddingChild.value = false
  childName.value = ''
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

function deleteSelf() {
  if (isConfirmingDelete.value) {
    notebookStore.removeNotebook(props.node.id)
    isConfirmingDelete.value = false
    return
  }
  isConfirmingDelete.value = true
  window.setTimeout(() => {
    isConfirmingDelete.value = false
  }, 3000)
}
</script>
