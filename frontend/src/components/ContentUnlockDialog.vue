<template>
  <DialogRoot :open="Boolean(lockStore.accessRequest)" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="content-unlock-dialog-overlay" />
      <DialogContent
        class="content-unlock-dialog-content"
        @escape-key-down.prevent="cancel"
        @interact-outside.prevent
      >
        <DialogTitle as="h2">ロックを解除</DialogTitle>
        <DialogDescription class="content-unlock-description">
          <template v-if="request && currentLock">
            「{{ request.targetLabel }}」を開くには、{{ targetTypeLabel(currentLock.targetType) }}「{{ currentLock.targetName }}」のパスフレーズが必要です。
          </template>
        </DialogDescription>

        <p v-if="request && request.requiredLocks.length > 1" class="content-unlock-progress">
          {{ request.requiredLocks.length }}件のロック解除が必要です。解除できたものから順に確認します。
        </p>

        <form class="content-unlock-form" @submit.prevent="submit">
          <label for="content-unlock-passphrase">
            パスフレーズ
            <input
              id="content-unlock-passphrase"
              ref="passphraseInput"
              v-model="passphrase"
              type="password"
              autocomplete="current-password"
              minlength="8"
              required
              :disabled="lockStore.isBusy"
            />
          </label>
          <p v-if="validationError" class="content-unlock-error" role="alert">{{ validationError }}</p>
          <div class="content-unlock-actions">
            <button type="button" :disabled="lockStore.isBusy" @click="cancel">キャンセル</button>
            <button class="content-unlock-submit" type="submit" :disabled="lockStore.isBusy || !currentLock">
              {{ lockStore.isBusy ? '解除中…' : '解除して開く' }}
            </button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import type { ContentLockTargetType } from '../api/contentLocks'
import { useContentLockStore } from '../stores/useContentLockStore'

const lockStore = useContentLockStore()
const passphrase = ref('')
const validationError = ref('')
const passphraseInput = ref<HTMLInputElement | null>(null)

const request = computed(() => lockStore.accessRequest)
const currentLock = computed(() => request.value?.requiredLocks[0] ?? null)

watch(request, async (nextRequest) => {
  passphrase.value = ''
  validationError.value = ''
  if (!nextRequest) return
  await nextTick()
  passphraseInput.value?.focus()
})

function targetTypeLabel(type: ContentLockTargetType) {
  return ({ space: '保存空間', notebook: 'ノートブック', note: 'ノート' })[type]
}

async function submit() {
  if (!passphrase.value.trim()) {
    validationError.value = 'パスフレーズを入力してください。'
    return
  }
  validationError.value = ''
  const result = await lockStore.unlockAccess(passphrase.value)
  passphrase.value = ''
  if (result.error) validationError.value = result.error.message
}

function cancel() {
  passphrase.value = ''
  validationError.value = ''
  lockStore.cancelAccessRequest()
}

function handleOpenChange(open: boolean) {
  if (!open) cancel()
}
</script>

<style scoped>
:global(.content-unlock-dialog-overlay) {
  position: fixed;
  inset: 0;
  z-index: 1600;
  background: rgba(0, 0, 0, 0.58);
}

:global(.content-unlock-dialog-content) {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1601;
  width: min(420px, calc(100vw - 32px));
  transform: translate(-50%, -50%);
  padding: 22px;
  border: 1px solid var(--border-strong, var(--border));
  border-radius: 10px;
  background-color: var(--bg-editor);
  color: var(--text-primary);
  opacity: 1;
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.42);
  outline: none;
}

:global(.content-unlock-dialog-content h2) {
  margin: 0;
  font-size: 18px;
}

.content-unlock-description,
.content-unlock-progress {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.content-unlock-progress {
  color: var(--brand-primary);
}

.content-unlock-form {
  display: grid;
  gap: 12px;
  margin-top: 20px;
}

.content-unlock-form label {
  display: grid;
  gap: 7px;
  color: var(--text-secondary);
  font-size: 13px;
}

.content-unlock-form input {
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font: inherit;
}

.content-unlock-form input:focus-visible {
  border-color: var(--brand-primary);
  outline: 2px solid color-mix(in srgb, var(--brand-primary) 28%, transparent);
}

.content-unlock-error {
  margin: 0;
  color: var(--color-danger, #c0392b);
  font-size: 12px;
  line-height: 1.45;
}

.content-unlock-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.content-unlock-actions button {
  min-height: 34px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.content-unlock-actions button:disabled {
  cursor: wait;
  opacity: 0.6;
}

.content-unlock-actions .content-unlock-submit {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: #fff;
}
</style>
