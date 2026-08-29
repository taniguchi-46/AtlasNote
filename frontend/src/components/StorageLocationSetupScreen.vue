<template>
  <main class="storage-location-setup" aria-labelledby="storage-location-title">
    <section class="storage-location-card">
      <p class="storage-location-eyebrow">Atlas Note</p>
      <h1 id="storage-location-title">保存場所を設定</h1>
      <p class="storage-location-lead">
        ノートとバックアップを保存するフォルダを選択してください。既存のAtlas Noteフォルダを選ぶと、そのデータを引き継ぎます。
      </p>

      <div class="storage-location-fields">
        <div class="storage-location-field">
          <div>
            <strong>保存領域</strong>
            <p>SQLiteデータベースとMarkdownノートを保存します。</p>
            <code v-if="dataRoot">{{ dataRoot }}</code>
          </div>
          <button type="button" :disabled="isBusy" @click="choose('data')">フォルダを選択</button>
        </div>

        <div class="storage-location-field">
          <div>
            <strong>バックアップ</strong>
            <p>自動バックアップの保存先です。保存領域とは別のドライブも選べます。</p>
            <code v-if="backupRoot">{{ backupRoot }}</code>
          </div>
          <button type="button" :disabled="isBusy" @click="choose('backup')">フォルダを選択</button>
        </div>
      </div>

      <p v-if="message" class="storage-location-message" :class="{ error: hasError }" role="alert">{{ message }}</p>
      <div class="storage-location-actions">
        <button type="button" class="primary" :disabled="isBusy" @click="apply">
          {{ isBusy ? '確認中…' : 'この設定で開始' }}
        </button>
      </div>
      <p class="storage-location-note">フォルダの移動中に元のデータは削除されません。</p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { StorageLocationStatus } from '../api/storageLocations'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'

const props = defineProps<{ initialStatus?: StorageLocationStatus }>()
const emit = defineEmits<{ completed: [] }>()

const locationStore = useStorageLocationStore()
const status = computed(() => locationStore.status ?? props.initialStatus ?? null)
const isBusy = computed(() => locationStore.isBusy)
const localMessage = ref('')
const message = computed(() => localMessage.value || locationStore.error?.message || '')
const hasError = computed(() => Boolean(locationStore.error))

const dataRoot = computed(() => status.value?.pendingDataRoot || status.value?.dataRoot || '')
const backupRoot = computed(() => status.value?.pendingBackupRoot || status.value?.backupRoot || '')

async function choose(kind: 'data' | 'backup') {
  if (isBusy.value) return
  localMessage.value = ''
  const selected = await locationStore.choose(kind)
  if (selected) localMessage.value = '選択したフォルダを確認しました。'
}

async function apply() {
  if (isBusy.value) return
  localMessage.value = ''
  const applied = await locationStore.apply()
  if (applied) emit('completed')
}

onMounted(() => {
  void locationStore.initialize()
})

</script>

<style scoped>
.storage-location-setup {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 32px;
  background: var(--color-bg, #f6f7fb);
}

.storage-location-card {
  width: min(680px, 100%);
  padding: 36px;
  border: 1px solid var(--color-border, #dfe3eb);
  border-radius: 18px;
  background: var(--color-surface, #fff);
  box-shadow: 0 18px 48px rgb(21 31 52 / 10%);
}

.storage-location-eyebrow {
  margin: 0 0 8px;
  color: var(--color-accent, #4f46e5);
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

h1 { margin: 0; font-size: 1.7rem; }
.storage-location-lead { margin: 14px 0 26px; color: var(--color-text-muted, #667085); line-height: 1.7; }
.storage-location-fields { display: grid; gap: 12px; }
.storage-location-field { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px; border: 1px solid var(--color-border, #dfe3eb); border-radius: 12px; }
.storage-location-field p { margin: 5px 0; color: var(--color-text-muted, #667085); font-size: 0.9rem; }
.storage-location-field code { display: block; overflow-wrap: anywhere; color: var(--color-text-muted, #667085); font-size: 0.78rem; }
button { flex: 0 0 auto; padding: 8px 14px; border: 1px solid var(--color-border, #dfe3eb); border-radius: 8px; background: var(--color-surface, #fff); cursor: pointer; }
button:disabled { cursor: wait; opacity: 0.55; }
.storage-location-actions { display: flex; justify-content: flex-end; margin-top: 24px; }
button.primary { border-color: var(--color-accent, #4f46e5); background: var(--color-accent, #4f46e5); color: #fff; font-weight: 700; }
.storage-location-message { margin: 16px 0 0; color: #166534; font-size: 0.9rem; }
.storage-location-message.error { color: #b42318; }
.storage-location-note { margin: 18px 0 0; color: var(--color-text-muted, #667085); font-size: 0.8rem; }
@media (max-width: 600px) {
  .storage-location-setup { padding: 16px; }
  .storage-location-card { padding: 24px; }
  .storage-location-field { align-items: flex-start; flex-direction: column; }
}
</style>
