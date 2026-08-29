<template>
  <section class="storage-location-settings">
    <h3>保存場所</h3>
    <p class="setting-help">ノートの保存領域とバックアップ保存領域を個別に指定できます。</p>

    <div v-if="locationStore.status" class="location-settings-list">
      <div class="location-settings-row">
        <div>
          <strong>保存領域</strong>
          <code>{{ locationStore.status.pendingDataRoot || locationStore.status.dataRoot || '未選択' }}</code>
        </div>
        <button type="button" :disabled="locationStore.isBusy || !locationStore.status.dataRootChangeAllowed" @click="choose('data')">
          フォルダを選択
        </button>
      </div>
      <div class="location-settings-row">
        <div>
          <strong>バックアップ</strong>
          <code>{{ locationStore.status.pendingBackupRoot || locationStore.status.backupRoot || '未選択' }}</code>
        </div>
        <button type="button" :disabled="locationStore.isBusy || !locationStore.status.dataRootChangeAllowed" @click="choose('backup')">
          フォルダを選択
        </button>
      </div>
    </div>

    <p v-if="locationStore.status?.environmentOverride" class="setting-help">
      ATLAS_NOTE_DATA_DIR によって保存領域が固定されています。
    </p>
    <p v-if="locationStore.status?.pendingRestart" class="setting-help">変更は次回起動時に適用されます。</p>
    <p v-if="locationStore.error" class="location-settings-error" role="alert">{{ locationStore.error.message }}</p>
    <div class="location-settings-actions">
      <button
        type="button"
        class="primary"
        :disabled="locationStore.isBusy || !hasPendingChange"
        @click="apply"
      >
        {{ locationStore.isOperating ? '適用中…' : '保存して再起動' }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'

const locationStore = useStorageLocationStore()
const hasPendingChange = computed(() => Boolean(
  locationStore.status?.pendingDataRoot || locationStore.status?.pendingBackupRoot,
))

function choose(kind: 'data' | 'backup') {
  void locationStore.choose(kind)
}

function apply() {
  void locationStore.apply()
}
</script>

<style scoped>
.storage-location-settings { display: grid; gap: 12px; }
.location-settings-list { display: grid; gap: 10px; }
.location-settings-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px; border: 1px solid var(--color-border, #dfe3eb); border-radius: 10px; }
.location-settings-row strong { display: block; }
.location-settings-row code { display: block; max-width: 420px; margin-top: 4px; overflow-wrap: anywhere; color: var(--color-text-muted, #667085); font-size: 0.78rem; }
.location-settings-row button, .location-settings-actions button { padding: 7px 12px; border: 1px solid var(--color-border, #dfe3eb); border-radius: 8px; background: var(--color-surface, #fff); cursor: pointer; }
.location-settings-row button:disabled, .location-settings-actions button:disabled { cursor: not-allowed; opacity: 0.5; }
.location-settings-actions { display: flex; justify-content: flex-end; }
.location-settings-actions .primary { border-color: var(--color-accent, #4f46e5); background: var(--color-accent, #4f46e5); color: #fff; }
.location-settings-error { color: #b42318; }
@media (max-width: 600px) { .location-settings-row { align-items: flex-start; flex-direction: column; } }
</style>
