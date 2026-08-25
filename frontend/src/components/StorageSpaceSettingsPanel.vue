<template>
  <section class="storage-space-settings">
    <h3>保存空間</h3>
    <p class="storage-space-description">
      ノートを保存空間ごとに分けて管理します。別の保存空間のノートは表示されません。
    </p>

    <div class="storage-space-heading-row">
      <h4>保存空間の一覧</h4>
      <button
        v-if="storageSpaceStore.error"
        class="text-button"
        type="button"
        :disabled="storageSpaceStore.isBusy"
        @click="storageSpaceStore.initialize"
      >
        再読み込み
      </button>
    </div>

    <p v-if="storageSpaceStore.error" class="storage-space-error" role="alert">
      {{ storageSpaceStore.error.message }}
    </p>
    <p v-if="storageSpaceStore.isLoading" class="storage-space-status" role="status">
      保存空間を読み込んでいます…
    </p>
    <ul v-else class="storage-space-list" aria-label="保存空間の一覧">
      <li v-for="space in storageSpaceStore.spaces" :key="space.id">
        <button
          class="storage-space-row"
          :class="{ 'is-active': space.active }"
          type="button"
          :disabled="space.active || storageSpaceStore.isBusy"
          :aria-current="space.active ? 'true' : undefined"
          @click="requestSwitch(space)"
        >
          <span class="storage-space-name">{{ space.name }}</span>
          <span v-if="space.active" class="active-space-badge">使用中</span>
          <CheckIcon v-if="space.active" class="active-space-check" :size="20" aria-hidden="true" />
        </button>
      </li>
    </ul>

    <button
      class="create-space-button"
      type="button"
      :disabled="storageSpaceStore.isBusy"
      @click="openCreateDialog"
    >
      <PlusIcon :size="18" aria-hidden="true" />
      新しい保存空間
    </button>

    <div class="restart-information">
      <RefreshCwIcon :size="18" aria-hidden="true" />
      <p>別の保存空間を選ぶと、変更を保存してAtlas Noteを自動的に再起動します。</p>
    </div>
    <p class="internal-storage-help">保存場所はAtlas Note内で管理されます。</p>

    <DialogRoot :open="createDialogOpen" @update:open="handleCreateDialogOpen">
      <DialogPortal>
        <DialogOverlay class="nested-dialog-overlay" />
        <DialogContent class="nested-dialog-content">
          <DialogTitle as="h3">新しい保存空間</DialogTitle>
          <DialogDescription class="nested-dialog-description">
            Atlas Noteの内部管理ディレクトリに新しい保存空間を作成します。
          </DialogDescription>
          <form @submit.prevent="handleCreate">
            <label for="storage-space-name">名前</label>
            <input
              id="storage-space-name"
              ref="createNameInput"
              v-model="createName"
              type="text"
              maxlength="80"
              autocomplete="off"
              :disabled="storageSpaceStore.isCreating"
            />
            <p v-if="createValidationError" class="storage-space-error" role="alert">
              {{ createValidationError }}
            </p>
            <div class="nested-dialog-actions">
              <button type="button" :disabled="storageSpaceStore.isCreating" @click="createDialogOpen = false">
                キャンセル
              </button>
              <button class="primary-button" type="submit" :disabled="storageSpaceStore.isCreating">
                {{ storageSpaceStore.isCreating ? '作成中…' : '作成' }}
              </button>
            </div>
          </form>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>

    <DialogRoot :open="pendingSpace !== null" @update:open="handleSwitchDialogOpen">
      <DialogPortal>
        <DialogOverlay class="nested-dialog-overlay" />
        <DialogContent class="nested-dialog-content">
          <DialogTitle as="h3">保存空間を切り替えますか？</DialogTitle>
          <DialogDescription class="nested-dialog-description">
            未保存の変更を保存した後、Atlas Noteを自動的に再起動して「{{ pendingSpace?.name }}」を開きます。
          </DialogDescription>
          <div class="nested-dialog-actions">
            <button type="button" :disabled="storageSpaceStore.isSwitching" @click="pendingSpace = null">
              キャンセル
            </button>
            <button class="primary-button" type="button" :disabled="storageSpaceStore.isSwitching" @click="handleSwitch">
              {{ storageSpaceStore.isSwitching ? '再起動しています…' : '保存して再起動' }}
            </button>
          </div>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </section>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { CheckIcon, PlusIcon, RefreshCwIcon } from '@lucide/vue'
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import type { StorageSpace } from '../api/storageSpaces'
import { useStorageSpaceStore } from '../stores/useStorageSpaceStore'

const storageSpaceStore = useStorageSpaceStore()
const createDialogOpen = ref(false)
const createName = ref('')
const createValidationError = ref('')
const createNameInput = ref<HTMLInputElement | null>(null)
const pendingSpace = ref<StorageSpace | null>(null)

function openCreateDialog() {
  storageSpaceStore.clearError()
  createName.value = ''
  createValidationError.value = ''
  createDialogOpen.value = true
  void nextTick(() => createNameInput.value?.focus())
}

function handleCreateDialogOpen(open: boolean) {
  if (!storageSpaceStore.isCreating) createDialogOpen.value = open
}

async function handleCreate() {
  const name = createName.value.trim()
  if (!name || Array.from(name).length > 80) {
    createValidationError.value = '名前は1〜80文字で入力してください。'
    return
  }
  createValidationError.value = ''
  const created = await storageSpaceStore.create(name)
  if (created) createDialogOpen.value = false
}

function requestSwitch(space: StorageSpace) {
  if (space.active || storageSpaceStore.isBusy) return
  storageSpaceStore.clearError()
  pendingSpace.value = space
}

function handleSwitchDialogOpen(open: boolean) {
  if (!open && !storageSpaceStore.isSwitching) pendingSpace.value = null
}

async function handleSwitch() {
  const target = pendingSpace.value
  if (!target) return
  const switched = await storageSpaceStore.switchTo(target.id)
  if (!switched) pendingSpace.value = null
}
</script>

<style scoped>
.storage-space-settings {
  color: var(--text-primary);
}

.storage-space-description {
  margin: -10px 0 22px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.storage-space-heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.storage-space-heading-row h4 {
  margin: 0;
  font-size: 13px;
}

.text-button,
.create-space-button {
  border: 0;
  background: transparent;
  color: var(--brand-primary);
  cursor: pointer;
}

.text-button:disabled,
.create-space-button:disabled {
  cursor: default;
  opacity: 0.55;
}

.storage-space-list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.storage-space-list li + li {
  border-top: 1px solid var(--border);
}

.storage-space-row {
  width: 100%;
  min-height: 54px;
  padding: 0 14px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 24px;
  align-items: center;
  gap: 10px;
  border: 0;
  background: var(--bg-input);
  color: var(--text-primary);
  text-align: left;
  cursor: pointer;
}

.storage-space-row:hover:not(:disabled),
.storage-space-row:focus-visible {
  background: var(--bg-hover);
  outline: 2px solid var(--brand-primary);
  outline-offset: -2px;
}

.storage-space-row.is-active {
  background: color-mix(in srgb, var(--brand-primary) 10%, var(--bg-input));
  cursor: default;
}

.storage-space-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 500;
}

.active-space-badge {
  padding: 3px 7px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--brand-primary) 18%, transparent);
  color: var(--brand-primary);
  font-size: 11px;
}

.active-space-check {
  color: var(--brand-primary);
}

.create-space-button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin: 14px 0 18px;
  padding: 4px 0;
  font-size: 14px;
}

.restart-information {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-sidebar);
  color: var(--text-secondary);
}

.restart-information svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--brand-primary);
}

.restart-information p,
.internal-storage-help {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
}

.internal-storage-help {
  margin-top: 12px;
  color: var(--text-secondary);
}

.storage-space-error,
.storage-space-status {
  margin: 10px 0;
  font-size: 13px;
}

.storage-space-error {
  color: var(--danger, #dc2626);
}

.storage-space-status {
  color: var(--text-secondary);
}

.nested-dialog-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.58);
}

.nested-dialog-content {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1101;
  width: min(430px, calc(100vw - 40px));
  padding: 24px;
  transform: translate(-50%, -50%);
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.38);
  color: var(--text-primary);
}

.nested-dialog-content h3 {
  margin: 0 0 10px;
  padding: 0;
  border: 0;
  font-size: 18px;
}

.nested-dialog-description {
  margin: 0 0 20px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.nested-dialog-content label {
  display: block;
  margin-bottom: 7px;
  font-size: 13px;
  font-weight: 600;
}

.nested-dialog-content input {
  box-sizing: border-box;
  width: 100%;
  padding: 9px 11px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 14px;
}

.nested-dialog-content input:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 1px;
}

.nested-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 22px;
}

.nested-dialog-actions button {
  min-width: 96px;
  padding: 8px 14px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.nested-dialog-actions button:disabled {
  cursor: default;
  opacity: 0.55;
}

.nested-dialog-actions .primary-button {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: white;
}
</style>
