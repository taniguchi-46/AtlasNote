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
    <div v-if="locationStore.error" class="location-settings-error" role="alert">
        <p>{{ locationStore.error.message }}</p>
        <dl>
          <div v-if="locationStore.error.reason"><dt>理由</dt><dd>{{ locationStore.error.reason }}</dd></div>
          <div v-if="locationStore.error.stage"><dt>検証段階</dt><dd>{{ locationStore.error.stage }}</dd></div>
          <div v-if="locationStore.error.role"><dt>対象</dt><dd>{{ locationStore.error.role }}</dd></div>
          <div v-if="locationStore.error.osErrorNumber"><dt>OSエラー番号</dt><dd>{{ locationStore.error.osErrorNumber }}</dd></div>
          <div><dt>コード</dt><dd>{{ locationStore.error.code }}</dd></div>
        <div v-if="locationStore.error.diagnosticId"><dt>診断 ID</dt><dd>{{ locationStore.error.diagnosticId }}</dd></div>
        <div v-if="locationStore.error.action"><dt>対応</dt><dd>{{ locationStore.error.action }}</dd></div>
      </dl>
    </div>
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

    <section class="location-diagnostics" aria-labelledby="location-diagnostics-title">
      <div class="location-diagnostics-heading">
        <div>
          <h4 id="location-diagnostics-title">診断情報</h4>
          <p class="setting-help">保存場所の確認に関する安全な情報だけを表示します。</p>
        </div>
        <button type="button" :disabled="locationStore.isDiagnosticsLoading" @click="copyDiagnostics">
          {{ locationStore.isDiagnosticsLoading ? '取得中…' : '診断情報をコピー' }}
        </button>
      </div>
      <p v-if="copyMessage" class="setting-help" role="status">{{ copyMessage }}</p>
      <ul v-if="locationStore.diagnostics.length" class="location-diagnostics-list">
        <li v-for="event in locationStore.diagnostics.slice().reverse().slice(0, 5)" :key="event.diagnosticId">
          <code>{{ event.diagnosticId }}</code>
          <span>{{ event.reason || event.code }}</span>
          <small>{{ event.timestamp }}</small>
        </li>
      </ul>
      <p v-else class="setting-help">記録された診断情報はありません。</p>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'

const locationStore = useStorageLocationStore()
const copyMessage = ref('')
const hasPendingChange = computed(() => Boolean(
  locationStore.status?.pendingDataRoot || locationStore.status?.pendingBackupRoot,
))

async function choose(kind: 'data' | 'backup') {
  await locationStore.choose(kind)
  void locationStore.loadDiagnostics()
}

async function apply() {
  await locationStore.apply()
  void locationStore.loadDiagnostics()
}

async function copyDiagnostics() {
  copyMessage.value = ''
  if (!locationStore.diagnosticReport) await locationStore.loadDiagnostics()
  if (!locationStore.diagnosticReport) {
    copyMessage.value = '診断情報を取得できませんでした。'
    return
  }
  try {
    const copied = await ClipboardSetText(locationStore.diagnosticReport)
    copyMessage.value = copied ? '診断情報をクリップボードにコピーしました。' : 'クリップボードへのコピーに失敗しました。'
  } catch {
    copyMessage.value = 'クリップボードへのコピーに失敗しました。'
  }
}

onMounted(() => {
  void locationStore.initialize()
  void locationStore.loadDiagnostics()
})
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
.location-settings-error p { margin: 0; }
.location-settings-error dl { display: grid; gap: 4px; margin: 8px 0 0; font-size: 0.82rem; }
.location-settings-error dl div { display: flex; gap: 8px; }
.location-settings-error dt { font-weight: 700; }
.location-settings-error dd { margin: 0; }
.location-diagnostics { display: grid; gap: 8px; margin-top: 8px; padding-top: 12px; border-top: 1px solid var(--color-border, #dfe3eb); }
.location-diagnostics-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.location-diagnostics-heading h4 { margin: 0; }
.location-diagnostics-heading button { padding: 7px 12px; border: 1px solid var(--color-border, #dfe3eb); border-radius: 8px; background: var(--color-surface, #fff); cursor: pointer; }
.location-diagnostics-list { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; font-size: 0.78rem; }
.location-diagnostics-list li { display: grid; grid-template-columns: auto 1fr auto; gap: 8px; align-items: center; }
.location-diagnostics-list small { color: var(--color-text-muted, #667085); }
@media (max-width: 600px) { .location-settings-row { align-items: flex-start; flex-direction: column; } }
</style>
