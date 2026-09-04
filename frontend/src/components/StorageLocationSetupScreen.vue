<template>
  <main class="storage-location-setup" aria-labelledby="storage-location-title">
    <section class="storage-location-card">
      <p class="storage-location-eyebrow">Atlas Note</p>
      <h1 id="storage-location-title">{{ isRecovery ? '保存場所を確認' : '保存場所を設定' }}</h1>
      <p class="storage-location-lead">
        {{ isRecovery
          ? '現在の保存場所を利用できません。元のデータは変更されていないため、利用可能なフォルダを選択してください。'
          : 'ノートとバックアップを保存するフォルダを選択してください。既存のAtlas Noteフォルダを選ぶと、そのデータを引き継ぎます。' }}
      </p>

      <div class="storage-location-fields">
        <div class="storage-location-field">
          <div>
            <strong>保存領域</strong>
            <span v-if="isAutomaticDataRoot" class="storage-location-badge">自動設定済み</span>
            <p>SQLiteデータベースとMarkdownノートを保存します。</p>
            <code v-if="dataRoot">{{ dataRoot }}</code>
          </div>
          <button type="button" :disabled="isBusy" @click="choose('data')">フォルダを選択</button>
        </div>

        <div class="storage-location-field">
          <div>
            <strong>バックアップ</strong>
            <span v-if="isAutomaticBackupRoot" class="storage-location-badge">自動設定済み</span>
            <p>自動バックアップの保存先です。保存領域とは別のドライブも選べます。</p>
            <code v-if="backupRoot">{{ backupRoot }}</code>
          </div>
          <button type="button" :disabled="isBusy" @click="choose('backup')">フォルダを選択</button>
        </div>
      </div>

      <p v-if="message" class="storage-location-message" :class="{ error: hasError }" role="alert" aria-live="polite">{{ message }}</p>
      <div class="storage-location-actions">
        <button type="button" class="primary" :disabled="isBusy" @click="apply">
          {{ isBusy ? '確認中…' : isRecovery ? '保存場所を変更して再起動' : 'この設定で開始' }}
        </button>
      </div>
      <p class="storage-location-note">
        {{ isRecovery ? '元の保存場所とデータは削除・移動されません。' : 'フォルダの移動中に元のデータは削除されません。' }}
      </p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { StorageLocationError, StorageLocationStatus } from '../api/storageLocations'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'

const props = defineProps<{
  initialStatus?: StorageLocationStatus
  initialError?: StorageLocationError
  mode?: 'setup' | 'recovery'
}>()
const emit = defineEmits<{ completed: [] }>()

const locationStore = useStorageLocationStore()
const status = computed(() => locationStore.status ?? props.initialStatus ?? null)
const isRecovery = computed(() => props.mode === 'recovery')
const isBusy = computed(() => locationStore.isBusy)
const localMessage = ref('')
const initialErrorCleared = ref(false)
const message = computed(() => localMessage.value || locationStore.error?.message || (!initialErrorCleared.value ? props.initialError?.message : '') || '')
const hasError = computed(() => Boolean(locationStore.error || (!initialErrorCleared.value && props.initialError)))

const dataRoot = computed(() => status.value?.pendingDataRoot || status.value?.dataRoot || '')
const backupRoot = computed(() => status.value?.pendingBackupRoot || status.value?.backupRoot || '')
const isAutomaticDataRoot = computed(() => status.value?.source === 'default' && !status.value?.pendingDataRoot)
const isAutomaticBackupRoot = computed(() => isAutomaticDataRoot.value && !status.value?.pendingBackupRoot)

async function choose(kind: 'data' | 'backup') {
  if (isBusy.value) return
  localMessage.value = ''
  const selected = await locationStore.choose(kind)
  if (selected) {
    initialErrorCleared.value = true
    localMessage.value = '選択したフォルダを確認しました。'
  }
}

async function apply() {
  if (isBusy.value) return
  localMessage.value = ''
  const applied = await locationStore.apply()
  if (applied) {
    initialErrorCleared.value = true
    emit('completed')
  }
}

onMounted(() => {
  void locationStore.initialize()
})

</script>

<style scoped>
.storage-location-setup {
  min-height: 100%;
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  display: grid;
  place-items: center;
  padding: 32px;
  background: var(--bg-app);
}

.storage-location-card {
  width: min(680px, 100%);
  padding: 36px;
  border: 1px solid var(--border-strong);
  border-radius: 18px;
  background: var(--bg-sidebar);
  box-shadow: 0 18px 48px rgb(0 0 0 / 18%);
}

.storage-location-eyebrow {
  margin: 0 0 8px;
  color: var(--text-active);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

h1 { margin: 0; font-size: 1.7rem; color: var(--text-primary); }
.storage-location-lead { margin: 14px 0 26px; color: var(--text-secondary); line-height: 1.7; }
.storage-location-fields { display: grid; gap: 12px; }
.storage-location-field { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 12px; background: var(--bg-input); }
.storage-location-field strong { color: var(--text-primary); }
.storage-location-field p { margin: 5px 0; color: var(--text-secondary); font-size: 0.9rem; }
.storage-location-field code { display: block; overflow-wrap: anywhere; color: var(--text-secondary); font-size: 0.78rem; }
.storage-location-badge { display: inline-block; margin-left: 8px; padding: 2px 7px; border-radius: 999px; background: var(--bg-active); color: var(--text-active); font-size: 0.72rem; font-weight: 600; }
button { flex: 0 0 auto; padding: 8px 14px; border: 1px solid var(--border-strong); border-radius: 8px; background: var(--bg-sidebar); color: var(--text-primary); cursor: pointer; }
button:disabled { cursor: wait; opacity: 0.55; }
button:focus-visible { outline: 2px solid var(--text-active); outline-offset: 2px; }
.storage-location-actions { display: flex; justify-content: flex-end; margin-top: 24px; }
button.primary { border-color: var(--brand-primary); background: var(--brand-primary); color: #fff; font-weight: 700; }
button.primary:hover:not(:disabled) { background: var(--brand-hover); }
.storage-location-message { margin: 16px 0 0; color: var(--color-success); font-size: 0.9rem; }
.storage-location-message.error { color: var(--color-danger); }
.storage-location-note { margin: 18px 0 0; color: var(--text-secondary); font-size: 0.8rem; }
@media (max-width: 600px) {
  .storage-location-setup { padding: 16px; }
  .storage-location-card { padding: 24px; }
  .storage-location-field { align-items: flex-start; flex-direction: column; }
}
</style>
