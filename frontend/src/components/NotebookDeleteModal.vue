<template>
  <div v-if="open" class="notebook-delete-overlay" @click.self="cancel">
    <form class="notebook-delete-modal" @submit.prevent="confirm">
      <header class="notebook-delete-header">
        <h2>ノートブックを削除</h2>
        <button class="icon-btn" type="button" title="閉じる" :disabled="isDeleting" @click="cancel">
          <XIcon :size="18" />
        </button>
      </header>

      <div class="notebook-delete-body">
        <p class="notebook-delete-message">
          「{{ notebookName }}」を削除します。ノートの扱いを選択してください。
        </p>

        <label class="notebook-delete-option">
          <input
            v-model="selectedMode"
            type="radio"
            value="trashNotes"
            :disabled="isDeleting"
          />
          <span>ノートブックを削除（ノートをゴミ箱に移動）</span>
        </label>

        <label class="notebook-delete-option">
          <input
            v-model="selectedMode"
            type="radio"
            value="keepNotes"
            :disabled="isDeleting"
          />
          <span>ノートブックを削除（ノートを削除しない）</span>
        </label>

        <p v-if="error" class="notebook-delete-error">{{ error }}</p>
      </div>

      <footer class="notebook-delete-footer">
        <button class="secondary-btn" type="button" :disabled="isDeleting" @click="cancel">
          キャンセル
        </button>
        <button class="danger-btn" type="submit" :disabled="isDeleting">
          削除
        </button>
      </footer>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { XIcon } from '@lucide/vue'
import type { NotebookDeleteMode } from '../api/notebooks'

const props = defineProps<{
  open: boolean
  notebookName: string
  isDeleting?: boolean
  error?: string
}>()

const emit = defineEmits<{
  cancel: []
  confirm: [mode: NotebookDeleteMode]
}>()

const selectedMode = ref<NotebookDeleteMode>('trashNotes')

watch(() => props.open, (open) => {
  if (open) {
    selectedMode.value = 'trashNotes'
  }
})

function cancel() {
  if (props.isDeleting) return
  emit('cancel')
}

function confirm() {
  emit('confirm', selectedMode.value)
}
</script>

<style scoped>
.notebook-delete-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
}

.notebook-delete-modal {
  width: min(460px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.35);
  overflow: hidden;
}

.notebook-delete-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}

.notebook-delete-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.notebook-delete-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
}

.notebook-delete-message {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.notebook-delete-option {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 13px;
  line-height: 1.5;
  cursor: pointer;
}

.notebook-delete-option:hover {
  background: var(--bg-hover);
}

.notebook-delete-option input {
  width: 15px;
  height: 15px;
  margin-top: 2px;
  flex-shrink: 0;
  accent-color: var(--brand-primary);
}

.notebook-delete-error {
  margin: 0;
  color: var(--color-danger);
  font-size: 12px;
}

.notebook-delete-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}

.secondary-btn,
.danger-btn {
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

.secondary-btn {
  border-color: var(--border);
  background: var(--bg-input);
  color: var(--text-primary);
}

.secondary-btn:hover:not(:disabled) {
  background: var(--bg-hover);
}

.danger-btn {
  background: var(--color-danger);
  color: #fff;
}

.danger-btn:hover:not(:disabled) {
  filter: brightness(0.92);
}

.secondary-btn:disabled,
.danger-btn:disabled,
.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
