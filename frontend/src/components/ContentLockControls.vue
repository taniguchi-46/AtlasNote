<template>
  <section
    class="content-lock-controls"
    :aria-label="`${targetLabel}のロック設定`"
    :aria-busy="targetStatusLoading"
  >
    <div v-if="targetStatusError" class="content-lock-status-error" role="alert">
      <p class="content-lock-error">{{ targetStatusError.message }}</p>
      <button type="button" :disabled="targetStatusLoading" @click="refreshStatus">再試行</button>
    </div>

    <template v-if="deferSave">
      <div class="content-lock-editor-toggle">
        <div>
          <strong>ロック</strong>
          <p>{{ deferredStateDescription }}</p>
        </div>
        <button
          class="content-lock-toggle"
          type="button"
          role="switch"
          :aria-checked="deferredLockEnabled"
          :disabled="lockStore.isBusy || targetStatusLoading || Boolean(targetStatusError) || !status"
          @click="toggleDeferredLock"
        >
          <span class="content-lock-toggle-indicator" aria-hidden="true" />
          {{ deferredLockEnabled ? '設定済み' : '未設定' }}
        </button>
      </div>

      <p v-if="operationError" class="content-lock-error" role="alert">{{ operationError.message }}</p>

      <div v-if="mode === 'enable' || mode === 'disable'" class="content-lock-form">
        <template v-if="mode === 'enable'">
          <label>
            パスフレーズ
            <input v-model="passphrase" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
          <label>
            パスフレーズ（確認）
            <input v-model="confirmation" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
          <p class="content-lock-help">本文だけを暗号化します。名前・タイトルは表示されます。</p>
          <label v-if="requiresAIRecordDeletion" class="content-lock-confirm">
            <input v-model="deleteAIRecords" type="checkbox" :disabled="lockStore.isBusy" />
            関連するAI履歴・成果物（{{ aiRecordCount }}件）を削除することを確認しました
          </label>
        </template>

        <template v-else>
          <label>
            現在のパスフレーズ
            <input v-model="passphrase" type="password" autocomplete="current-password" required :disabled="lockStore.isBusy" />
          </label>
          <p class="content-lock-help">
            無効化すると、対象本文を現在のロック構成に応じて再保存します。
          </p>
        </template>
      </div>
    </template>

    <template v-else>
      <div class="content-lock-state">
        <LockKeyholeIcon v-if="status?.locked" :size="16" aria-hidden="true" />
        <LockIcon v-else-if="status?.protected" :size="16" aria-hidden="true" />
        <UnlockIcon v-else :size="16" aria-hidden="true" />
        <div>
          <strong>{{ stateTitle }}</strong>
          <p>{{ stateDescription }}</p>
        </div>
      </div>

      <p v-if="operationError" class="content-lock-error" role="alert">{{ operationError.message }}</p>

      <div v-if="mode === 'idle'" class="content-lock-actions">
        <button
          v-if="!status?.explicitLock"
          class="content-lock-primary"
          type="button"
          :disabled="lockStore.isBusy"
          @click="startEnable"
        >
          <LockIcon :size="15" aria-hidden="true" />
          {{ status?.protected ? 'この対象にもロックを設定' : 'ロックを設定' }}
        </button>
        <template v-else>
          <button
            v-if="status.locked && sessionUnlockAvailable"
            class="content-lock-primary"
            type="button"
            :disabled="lockStore.isBusy"
            @click="startOperation('unlock')"
          >
            <UnlockIcon :size="15" aria-hidden="true" />
            ロックを解除
          </button>
          <button
            v-if="!status.locked && showLockNow"
            type="button"
            :disabled="lockStore.isBusy"
            @click="handleLockNow"
          >
            <LockKeyholeIcon :size="15" aria-hidden="true" />
            今すぐロック
          </button>
          <button type="button" :disabled="lockStore.isBusy" @click="startOperation('change')">
            パスフレーズを変更
          </button>
          <button class="danger-button" type="button" :disabled="lockStore.isBusy" @click="startOperation('disable')">
            ロックを無効化
          </button>
        </template>
      </div>

      <form v-else class="content-lock-form" @submit.prevent="submit">
        <template v-if="mode === 'enable'">
          <label>
            パスフレーズ
            <input v-model="passphrase" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
          <label>
            パスフレーズ（確認）
            <input v-model="confirmation" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
          <p class="content-lock-help">本文だけを暗号化します。名前・タイトルは表示されます。</p>
          <label v-if="requiresAIRecordDeletion" class="content-lock-confirm">
            <input v-model="deleteAIRecords" type="checkbox" :disabled="lockStore.isBusy" />
            関連するAI履歴・成果物（{{ aiRecordCount }}件）を削除することを確認しました
          </label>
        </template>

        <template v-else-if="mode === 'unlock' || mode === 'disable'">
          <label>
            パスフレーズ
            <input v-model="passphrase" type="password" autocomplete="current-password" required :disabled="lockStore.isBusy" />
          </label>
          <p v-if="mode === 'disable'" class="content-lock-help">
            無効化すると、対象本文を現在のロック構成に応じて再保存します。
          </p>
        </template>

        <template v-else-if="mode === 'change'">
          <label>
            現在のパスフレーズ
            <input v-model="currentPassphrase" type="password" autocomplete="current-password" required :disabled="lockStore.isBusy" />
          </label>
          <label>
            新しいパスフレーズ
            <input v-model="passphrase" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
          <label>
            新しいパスフレーズ（確認）
            <input v-model="confirmation" type="password" minlength="8" autocomplete="new-password" required :disabled="lockStore.isBusy" />
          </label>
        </template>

        <div class="content-lock-form-actions">
          <button type="button" :disabled="lockStore.isBusy" @click="reset">キャンセル</button>
          <button class="content-lock-primary" type="submit" :disabled="lockStore.isBusy">
            {{ submitLabel }}
          </button>
        </div>
      </form>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { LockIcon, LockKeyholeIcon, UnlockIcon } from '@lucide/vue'
import type { ContentLockError, ContentLockTarget } from '../api/contentLocks'
import { useContentLockStore } from '../stores/useContentLockStore'

type Mode = 'idle' | 'enable' | 'unlock' | 'change' | 'disable'

const props = withDefaults(defineProps<{
  target: ContentLockTarget
  targetLabel: string
  sessionUnlockAvailable?: boolean
  showLockNow?: boolean
  deferSave?: boolean
}>(), {
  sessionUnlockAvailable: true,
  showLockNow: true,
  deferSave: false,
})

const emit = defineEmits<{
  changed: []
  locked: [target: ContentLockTarget]
}>()

const lockStore = useContentLockStore()
const mode = ref<Mode>('idle')
const passphrase = ref('')
const confirmation = ref('')
const currentPassphrase = ref('')
const operationError = ref<ContentLockError | null>(null)
const requiresAIRecordDeletion = ref(false)
const aiRecordCount = ref(0)
const deleteAIRecords = ref(false)

const key = computed(() => `${props.target.type}:${props.target.id}`)
const status = computed(() => lockStore.statuses[key.value] ?? null)
const targetStatusLoading = computed(() => Boolean(lockStore.statusLoading[key.value]))
const targetStatusError = computed(() => lockStore.statusErrors[key.value] ?? null)
const stateTitle = computed(() => {
  if (status.value?.locked) return 'ロック中'
  if (status.value?.protected) return status.value.explicitLock ? 'ロック解除中' : '上位ロックにより保護中'
  return 'ロック未設定'
})
const stateDescription = computed(() => {
  if (status.value?.locked) return props.sessionUnlockAvailable
    ? '本文を開くにはパスフレーズが必要です。'
    : 'この保存空間を開いた後にパスフレーズで解除できます。'
  if (status.value?.protected && !status.value.explicitLock) return '親ノートブックまたは保存空間のロックが適用されています。'
  if (status.value?.protected) return 'このセッションでは本文を利用できます。'
  return '本文は通常どおり保存されています。'
})
const deferredLockEnabled = computed(() => (
  mode.value === 'enable'
  || (mode.value !== 'disable' && Boolean(status.value?.explicitLock))
))
const deferredStateDescription = computed(() => {
  if (targetStatusError.value) return 'ロック状態を取得できませんでした。再試行してください。'
  if (!status.value) return 'ロック状態を確認しています。'
  if (mode.value === 'enable') return 'パスフレーズを入力し、下の保存でロックを有効にします。'
  if (mode.value === 'disable') return '現在のパスフレーズを入力し、下の保存でロックを無効にします。'
  if (status.value.explicitLock) {
    return status.value.locked
      ? 'ロック中です。解除やパスフレーズ変更は「設定 ＞ ロック」で行えます。'
      : 'この対象のロックは設定済みです。'
  }
  if (status.value.protected) return '上位ロックにより本文が保護されています。'
  return 'トグルを有効にすると、パスフレーズ欄が表示されます。'
})
const submitLabel = computed(() => ({
  enable: 'ロックを設定',
  unlock: 'ロックを解除',
  change: '変更を保存',
  disable: 'ロックを無効化',
  idle: '保存',
}[mode.value]))

watch(
  key,
  () => {
    reset()
    void refreshStatus()
  },
  { immediate: true },
)

async function refreshStatus() {
  operationError.value = null
  await lockStore.refreshTarget({ type: props.target.type, id: props.target.id })
}

function startEnable() {
  reset()
  mode.value = 'enable'
}

function startOperation(next: Exclude<Mode, 'idle' | 'enable'>) {
  reset()
  mode.value = next
}

function toggleDeferredLock() {
  if (!props.deferSave || lockStore.isBusy || !status.value) return
  if (deferredLockEnabled.value) {
    if (mode.value === 'enable') {
      reset()
      return
    }
    if (status.value.explicitLock) startOperation('disable')
    return
  }
  if (mode.value === 'disable') {
    reset()
    return
  }
  startEnable()
}

function reset() {
  mode.value = 'idle'
  passphrase.value = ''
  confirmation.value = ''
  currentPassphrase.value = ''
  operationError.value = null
  requiresAIRecordDeletion.value = false
  aiRecordCount.value = 0
  deleteAIRecords.value = false
}

function setResultError(error?: ContentLockError) {
  operationError.value = error ?? {
    code: 'CONTENT_LOCK_UNAVAILABLE',
    message: 'ロックを利用できませんでした。データは変更していません。',
  }
  if (operationError.value.code === 'CONTENT_LOCK_AI_RECORDS_CONFIRMATION_REQUIRED') {
    requiresAIRecordDeletion.value = true
    aiRecordCount.value = operationError.value.aiRecordCount ?? 0
  }
}

async function submit(): Promise<boolean> {
  operationError.value = null
  if (props.deferSave && (targetStatusLoading.value || targetStatusError.value || !status.value)) {
    if (!targetStatusError.value) {
      operationError.value = {
        code: 'CONTENT_LOCK_UNAVAILABLE',
        message: 'ロック状態の確認が完了していません。確認後にもう一度保存してください。',
      }
    }
    return false
  }
  if (mode.value === 'idle') return true
  if ((mode.value === 'enable' || mode.value === 'change') && passphrase.value !== confirmation.value) {
    operationError.value = { code: 'CONTENT_LOCK_CONFIRMATION', message: '確認用パスフレーズが一致しません。' }
    return false
  }
  if (mode.value === 'enable' && requiresAIRecordDeletion.value && !deleteAIRecords.value) {
    operationError.value = { code: 'CONTENT_LOCK_AI_RECORDS_CONFIRMATION_REQUIRED', message: 'AI履歴・成果物の削除確認が必要です。' }
    return false
  }

  const result = mode.value === 'enable'
    ? await lockStore.enable(props.target, passphrase.value, deleteAIRecords.value)
    : mode.value === 'unlock'
      ? await lockStore.unlock(props.target, passphrase.value)
      : mode.value === 'change'
        ? await lockStore.changePassphrase(props.target, currentPassphrase.value, passphrase.value)
        : await lockStore.disable(props.target, passphrase.value)
  if (result.error) {
    setResultError(result.error)
    return false
  }
  reset()
  emit('changed')
  return true
}

async function handleLockNow() {
  if (!window.confirm(`「${props.targetLabel}」を今すぐロックします。未保存の変更は先に保存されます。続行しますか？`)) return
  operationError.value = null
  const result = await lockStore.lockNow(props.target)
  if (result.error) {
    setResultError(result.error)
    return
  }
  emit('locked', props.target)
  emit('changed')
}

defineExpose({
  save: submit,
  reset,
})
</script>

<style scoped>
.content-lock-controls {
  display: grid;
  gap: 12px;
  color: var(--text-primary);
}

.content-lock-state {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--bg-input);
}

.content-lock-state > svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--brand-primary);
}

.content-lock-state strong,
.content-lock-state p {
  display: block;
  margin: 0;
}

.content-lock-state strong {
  font-size: 13px;
}

.content-lock-editor-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--bg-input);
}

.content-lock-editor-toggle strong,
.content-lock-editor-toggle p {
  display: block;
  margin: 0;
}

.content-lock-editor-toggle strong {
  font-size: 13px;
}

.content-lock-editor-toggle p {
  margin-top: 3px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.content-lock-state p,
.content-lock-help {
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.content-lock-state p {
  margin-top: 3px;
}

.content-lock-actions,
.content-lock-form-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

button {
  min-height: 30px;
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-editor);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

button:hover:not(:disabled) {
  background: var(--bg-hover);
}

button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.content-lock-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: white;
}

.content-lock-primary:hover:not(:disabled) {
  background: var(--brand-primary-hover, var(--brand-primary));
}

.content-lock-toggle {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 6px;
  min-width: 74px;
}

.content-lock-toggle[aria-checked='true'] {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: white;
}

.content-lock-toggle-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.danger-button {
  color: var(--color-danger, #c0392b);
}

.content-lock-form {
  display: grid;
  gap: 10px;
}

.content-lock-form label {
  display: grid;
  gap: 5px;
  font-size: 12px;
  font-weight: 600;
}

.content-lock-form input[type='password'] {
  min-width: 0;
  box-sizing: border-box;
  width: 100%;
  padding: 7px 8px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
}

.content-lock-confirm {
  grid-template-columns: auto 1fr;
  align-items: start;
  font-weight: 400 !important;
  line-height: 1.45;
}

.content-lock-confirm input {
  margin-top: 2px;
}

.content-lock-error {
  margin: 0;
  color: var(--color-danger, #c0392b);
  font-size: 12px;
  line-height: 1.45;
}

.content-lock-status-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--color-danger, #c0392b) 45%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--color-danger, #c0392b) 8%, transparent);
}

.content-lock-status-error button {
  flex: 0 0 auto;
}

.content-lock-help {
  margin: 0;
}
</style>
