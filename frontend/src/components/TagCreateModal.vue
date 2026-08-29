<template>
  <DialogRoot :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="tag-create-overlay" />
      <DialogContent as-child @open-auto-focus="handleOpenAutoFocus">
        <form class="tag-create-modal" @submit.prevent="submit">
          <header class="tag-create-header">
            <DialogTitle as="h2">タグを追加</DialogTitle>
            <VisuallyHidden>
              <DialogDescription>タグ名を入力して追加します</DialogDescription>
            </VisuallyHidden>
            <DialogClose as-child>
              <button
                class="icon-btn"
                type="button"
                title="閉じる"
                aria-label="閉じる"
                :disabled="tagStore.isMutating"
              >
                <XIcon :size="18" />
              </button>
            </DialogClose>
          </header>

          <div class="tag-create-body">
            <label class="tag-create-field">
              <span>タグ名</span>
              <input
                ref="nameInputRef"
                v-model="name"
                class="tag-create-input"
                type="text"
                placeholder="タグ名"
                :disabled="tagStore.isMutating"
                :aria-describedby="error ? 'tag-create-error' : undefined"
              />
            </label>

            <p v-if="error" id="tag-create-error" class="tag-create-error" role="alert">
              {{ error }}
            </p>
          </div>

          <footer class="tag-create-footer">
            <DialogClose as-child>
              <button class="secondary-btn" type="button" :disabled="tagStore.isMutating">
                キャンセル
              </button>
            </DialogClose>
            <button class="primary-btn" type="submit" :disabled="tagStore.isMutating">
              {{ tagStore.isMutating ? '追加中…' : '追加' }}
            </button>
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
import { useTagStore } from '../stores/useTagStore'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const tagStore = useTagStore()
const nameInputRef = ref<HTMLInputElement | null>(null)
const name = ref('')
const error = ref('')

watch(() => props.open, (open) => {
  if (!open) return

  name.value = ''
  error.value = ''
})

function handleOpenChange(open: boolean) {
  if (!open && !tagStore.isMutating) close()
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
    error.value = 'タグ名を入力してください'
    return
  }

  error.value = ''
  try {
    await tagStore.createTag(trimmed)
    close()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'タグの追加に失敗しました'
  }
}

function close() {
  emit('close')
}
</script>

<style scoped>
.tag-create-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.5);
}

.tag-create-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1101;
  display: flex;
  flex-direction: column;
  width: min(420px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  overflow: hidden;
  background: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.35);
  transform: translate(-50%, -50%);
}

.tag-create-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}

.tag-create-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.tag-create-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 18px;
  overflow-y: auto;
}

.tag-create-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tag-create-field span {
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.tag-create-input {
  width: 100%;
  padding: 8px 10px;
  color: var(--text-primary);
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 6px;
}

.tag-create-input:focus {
  border-color: var(--brand-primary);
}

.tag-create-error {
  margin: 0;
  color: var(--color-danger);
  font-size: 12px;
}

.tag-create-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}

.primary-btn,
.secondary-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 88px;
  height: 34px;
  margin-top: 0;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
}

.primary-btn {
  color: #fff;
  background: var(--brand-primary);
}

.primary-btn:hover:not(:disabled) {
  background: var(--brand-hover);
}

.secondary-btn {
  color: var(--text-primary);
  background: var(--bg-input);
  border-color: var(--border);
}

.secondary-btn:hover:not(:disabled) {
  background: var(--bg-hover);
}

.primary-btn:disabled,
.secondary-btn:disabled,
.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
