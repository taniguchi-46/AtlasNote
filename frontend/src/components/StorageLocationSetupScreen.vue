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

      <div v-if="message" class="storage-location-message" :class="{ error: hasError }" role="alert" aria-live="polite">
        <p>{{ message }}</p>
        <dl v-if="currentError">
          <div v-if="currentError.reason"><dt>理由</dt><dd>{{ currentError.reason }}</dd></div>
          <div v-if="currentError.stage"><dt>検証段階</dt><dd>{{ currentError.stage }}</dd></div>
          <div v-if="currentError.role"><dt>対象</dt><dd>{{ currentError.role }}</dd></div>
          <div v-if="currentError.osErrorNumber"><dt>OSエラー番号</dt><dd>{{ currentError.osErrorNumber }}</dd></div>
          <div><dt>コード</dt><dd>{{ currentError.code }}</dd></div>
          <div v-if="currentError.diagnosticId"><dt>診断 ID</dt><dd>{{ currentError.diagnosticId }}</dd></div>
          <div v-if="currentError.action"><dt>対応</dt><dd>{{ currentError.action }}</dd></div>
        </dl>
      </div>
      <div class="storage-location-actions">
        <button type="button" class="primary" :disabled="isBusy" @click="apply">
          {{ isBusy ? '確認中…' : isRecovery ? '別の保存場所で開始' : 'この設定で開始' }}
        </button>
      </div>
      <p class="storage-location-note">
        {{ isRecovery ? '元の保存場所とデータは削除・移動されません。別の空フォルダで開始する場合、元データは自動移行されません。' : 'フォルダの移動中に元のデータは削除されません。' }}
      </p>
      <div v-if="isRecovery" class="storage-location-recovery-actions">
        <button type="button" :disabled="isBusy || !hasPendingMigration" @click="cancelPendingMigration">
          元の保存場所に戻す
        </button>
        <button type="button" :disabled="isBusy || !hasPendingMigration" @click="retryPendingMigration">
          同じ移行を再試行する
        </button>
      </div>
      <p v-if="isRecovery" class="storage-location-recovery-help">
        「別の保存場所で開始」は、選択した既存のAtlas Noteデータを開くか、空のフォルダで新しく開始します。
      </p>
      <div class="storage-location-exit-actions">
        <button type="button" :disabled="isBusy" @click="openInstalledApps">Windowsの「インストールされているアプリ」を開く</button>
        <button type="button" :disabled="isBusy" @click="exitApplication">終了</button>
      </div>
      <section class="storage-location-diagnostics" aria-labelledby="storage-location-diagnostics-title">
        <div class="storage-location-diagnostics-heading">
          <div>
            <h2 id="storage-location-diagnostics-title">診断情報</h2>
            <p>保存場所の確認に関する安全な情報だけを表示します。</p>
          </div>
          <button type="button" :disabled="locationStore.isDiagnosticsLoading" @click="copyDiagnostics">
            {{ locationStore.isDiagnosticsLoading ? '取得中…' : '診断情報をコピー' }}
          </button>
        </div>
        <p v-if="copyMessage" class="storage-location-diagnostics-message" role="status">{{ copyMessage }}</p>
        <ul v-if="locationStore.diagnostics.length" class="storage-location-diagnostics-list">
          <li v-for="event in locationStore.diagnostics.slice().reverse().slice(0, 5)" :key="event.diagnosticId">
            <code>{{ event.diagnosticId }}</code>
            <span>{{ event.reason || event.code }}</span>
            <small>{{ event.timestamp }}</small>
          </li>
        </ul>
        <p v-else>記録された診断情報はありません。</p>
      </section>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { exitApplication, openInstalledApps } from '../api/startup'
import type { StorageLocationError, StorageLocationStatus } from '../api/storageLocations'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

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
const copyMessage = ref('')
const currentError = computed(() => locationStore.error || (!initialErrorCleared.value ? props.initialError : undefined))
const message = computed(() => localMessage.value || currentError.value?.message || '')
const hasError = computed(() => Boolean(currentError.value))
const hasPendingMigration = computed(() => Boolean(status.value?.pendingMigration))

const dataRoot = computed(() => status.value?.pendingDataRoot || status.value?.dataRoot || '')
const backupRoot = computed(() => status.value?.pendingBackupRoot || status.value?.backupRoot || '')
const isAutomaticDataRoot = computed(() => status.value?.source === 'default' && !status.value?.pendingDataRoot)
const isAutomaticBackupRoot = computed(() => isAutomaticDataRoot.value && !status.value?.pendingBackupRoot)

async function choose(kind: 'data' | 'backup') {
  if (isBusy.value) return
  localMessage.value = ''
  const selected = await locationStore.choose(kind)
  void locationStore.loadDiagnostics()
  if (selected) {
    initialErrorCleared.value = true
    localMessage.value = '選択したフォルダを確認しました。'
  }
}

async function apply() {
  if (isBusy.value) return
  localMessage.value = ''
  const applied = await locationStore.apply()
  void locationStore.loadDiagnostics()
  if (applied) {
    initialErrorCleared.value = true
    emit('completed')
  }
}

async function cancelPendingMigration() {
  if (isBusy.value) return
  localMessage.value = ''
  const completed = await locationStore.cancelPendingMigration()
  void locationStore.loadDiagnostics()
  if (completed) localMessage.value = '元の保存場所へ戻す設定を保存しました。'
}

async function retryPendingMigration() {
  if (isBusy.value) return
  localMessage.value = ''
  const completed = await locationStore.retryPendingMigration()
  void locationStore.loadDiagnostics()
  if (completed) localMessage.value = '同じ移行を再試行する設定を保存しました。'
}

onMounted(() => {
  void locationStore.initialize()
  void locationStore.loadDiagnostics()
})

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
.storage-location-message p { margin: 0; }
.storage-location-message.error { color: var(--color-danger); }
.storage-location-message dl { display: grid; gap: 4px; margin: 8px 0 0; font-size: 0.82rem; }
.storage-location-message dl div { display: flex; gap: 8px; }
.storage-location-message dt { font-weight: 700; }
.storage-location-message dd { margin: 0; }
.storage-location-note { margin: 18px 0 0; color: var(--text-secondary); font-size: 0.8rem; }
.storage-location-recovery-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.storage-location-recovery-help { margin: 12px 0 0; color: var(--text-secondary); font-size: 0.84rem; line-height: 1.6; }
.storage-location-exit-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 24px; }
.storage-location-diagnostics { display: grid; gap: 8px; margin-top: 24px; padding-top: 16px; border-top: 1px solid var(--border); color: var(--text-secondary); font-size: 0.8rem; }
.storage-location-diagnostics-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.storage-location-diagnostics-heading h2 { margin: 0; color: var(--text-primary); font-size: 1rem; }
.storage-location-diagnostics-heading p, .storage-location-diagnostics > p { margin: 4px 0 0; }
.storage-location-diagnostics-heading button { flex: 0 0 auto; }
.storage-location-diagnostics-list { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.storage-location-diagnostics-list li { display: grid; grid-template-columns: auto 1fr auto; gap: 8px; align-items: center; }
.storage-location-diagnostics-list small { color: var(--text-secondary); }
.storage-location-diagnostics-message { color: var(--text-active) !important; }
@media (max-width: 600px) {
  .storage-location-setup { padding: 16px; }
  .storage-location-card { padding: 24px; }
  .storage-location-field { align-items: flex-start; flex-direction: column; }
}
</style>
