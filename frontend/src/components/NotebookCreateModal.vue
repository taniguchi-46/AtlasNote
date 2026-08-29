<template>
  <DialogRoot :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="notebook-create-overlay" />
      <DialogContent as-child @open-auto-focus="handleOpenAutoFocus">
        <form class="notebook-create-modal" @submit.prevent="submit">
      <header class="notebook-create-header">
        <DialogTitle as="h2">ノートブックを作成</DialogTitle>
        <VisuallyHidden>
          <DialogDescription>ノートブック名とアイコンを指定して作成します</DialogDescription>
        </VisuallyHidden>
        <DialogClose as-child>
          <button class="icon-btn" type="button" title="閉じる">
            <XIcon :size="18" />
          </button>
        </DialogClose>
      </header>

      <div class="notebook-create-body">
        <label class="notebook-create-field">
          <span>ノートブック名</span>
          <input
            ref="nameInputRef"
            v-model="name"
            class="notebook-create-input"
            type="text"
            placeholder="ノートブック名"
          />
        </label>

        <div class="notebook-create-field">
          <span>アイコン</span>
          <NotebookIconPicker v-model="selectedIcon" />
        </div>

        <p v-if="error" class="notebook-create-error">{{ error }}</p>
      </div>

      <footer class="notebook-create-footer">
        <DialogClose as-child>
          <button class="secondary-btn" type="button">キャンセル</button>
        </DialogClose>
        <button class="primary-btn" type="submit">作成</button>
      </footer>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { XIcon } from '@lucide/vue'
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  VisuallyHidden,
} from 'reka-ui'
import NotebookIconPicker from './NotebookIconPicker.vue'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useSettingsStore } from '../stores/useSettingsStore'
import { isKnownNotebookIcon } from '../utils/notebookIcons'

const props = defineProps<{
  open: boolean
  parentId?: string | null
}>()

const emit = defineEmits<{
  close: []
}>()

const notebookStore = useNotebookStore()
const settingsStore = useSettingsStore()
const nameInputRef = ref<HTMLInputElement | null>(null)
const name = ref('')
const selectedIcon = ref(settingsStore.defaultNotebookIcon)
const error = ref('')

watch(() => props.open, (open) => {
  if (!open) return

  name.value = ''
  selectedIcon.value = settingsStore.defaultNotebookIcon
  error.value = ''
})

function handleOpenChange(open: boolean) {
  if (!open) close()
}

function handleOpenAutoFocus(event: Event) {
  event.preventDefault()
  nextTick(() => {
    nameInputRef.value?.focus()
  })
}

async function submit() {
  const trimmed = name.value.trim()
  if (!trimmed) {
    error.value = 'ノートブック名を入力してください'
    return
  }

  const icon = isKnownNotebookIcon(selectedIcon.value)
    ? selectedIcon.value
    : settingsStore.defaultNotebookIcon

  try {
    await notebookStore.newNotebook(trimmed, props.parentId ?? null, icon)
    close()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'ノートブックの作成に失敗しました'
  }
}

function close() {
  emit('close')
}
</script>

<style scoped>
.notebook-create-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.5);
}

.notebook-create-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1101;
  transform: translate(-50%, -50%);
  width: min(420px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.35);
  overflow: hidden;
}

.notebook-create-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}

.notebook-create-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.notebook-create-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 18px;
  overflow-y: auto;
}

.notebook-create-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.notebook-create-field span {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.notebook-create-input {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
}

.notebook-create-input:focus {
  border-color: var(--brand-primary);
}

.notebook-create-error {
  margin: 0;
  color: var(--color-danger);
  font-size: 12px;
}

.notebook-create-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}

.primary-btn,
.secondary-btn {
  min-width: 88px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-top: 0;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
}

.primary-btn {
  background: var(--brand-primary);
  color: #fff;
}

.primary-btn:hover {
  background: var(--brand-hover);
}

.secondary-btn {
  border: 1px solid var(--border);
  background: var(--bg-input);
  color: var(--text-primary);
}

.secondary-btn:hover {
  background: var(--bg-hover);
}
</style>
