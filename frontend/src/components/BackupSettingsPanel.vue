<template>
  <section class="backup-settings">
    <h3>バックアップ</h3>
    <p class="backup-description">
      現在の保存空間を自動でバックアップします。復元前には現在のデータも安全用バックアップとして保存されます。
    </p>

    <div class="backup-toggle-card">
      <label class="backup-toggle-label">
        <input v-model="backupStore.automaticEnabled" type="checkbox" />
        <span>
          <strong>自動バックアップを有効にする</strong>
          <small>保存済みのSQLiteデータとMarkdown本文を24時間ごとに保存します。</small>
        </span>
      </label>
      <p class="backup-help">保存先は「設定 > 保存場所」から変更できます。変更後は次回起動時に適用されます。</p>
    </div>

    <div class="backup-heading-row">
      <h4>保存済みバックアップ</h4>
      <button type="button" :disabled="backupStore.isBusy" @click="backupStore.initialize">
        再読み込み
      </button>
    </div>

    <p v-if="backupStore.error" class="backup-error" role="alert">
      {{ backupStore.error.message }}
    </p>
    <p v-if="backupStore.isLoading" class="backup-status" role="status">
      バックアップを検証しています…
    </p>
    <p v-else-if="backupStore.backups.length === 0" class="backup-status">
      まだバックアップはありません。自動バックアップが有効なら、次回の保存タイミングで作成されます。
    </p>
    <ul v-else class="backup-list" aria-label="保存済みバックアップ">
      <li v-for="backup in backupStore.backups" :key="backup.id" class="backup-entry">
        <div class="backup-entry-info">
          <strong>{{ backup.kind === 'restore-safety' ? '復元前の安全用' : '自動バックアップ' }}</strong>
          <span>{{ formatDate(backup.createdAt) }} · {{ formatBytes(backup.sizeBytes) }}</span>
          <small v-if="!backup.restorable" class="backup-entry-invalid">
            検証に失敗したため復元できません
          </small>
        </div>
        <button
          class="primary-button backup-restore-button"
          type="button"
          :disabled="!backup.restorable || backupStore.isBusy"
          @click="requestRestore(backup)"
        >
          復元
        </button>
      </li>
    </ul>

    <p v-if="backupStore.status?.lastAutomaticAt" class="backup-last-run">
      最終自動バックアップ: {{ formatDate(backupStore.status.lastAutomaticAt) }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import type { BackupSummary } from '../api/backups'
import { useBackupStore } from '../stores/useBackupStore'

const backupStore = useBackupStore()

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '日時不明'
  return date.toLocaleString('ja-JP')
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value < 0) return 'サイズ不明'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

async function requestRestore(backup: BackupSummary) {
  const preview = await backupStore.previewRestore(backup.id)
  if (!preview) return
  const confirmed = window.confirm(
    `${preview.message}\n\n作成日時: ${formatDate(preview.createdAt)}\nサイズ: ${formatBytes(preview.sizeBytes)}\n\n現在の保存空間を置き換えて再起動します。続行しますか？`,
  )
  if (!confirmed) return
  await backupStore.restore(preview)
}

onMounted(() => {
  void backupStore.initialize()
})
</script>

<style scoped>
.backup-settings {
  color: var(--text-primary);
}

.backup-description {
  margin: -10px 0 22px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.backup-toggle-card {
  margin-bottom: 24px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-input);
}

.backup-toggle-label {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  cursor: pointer;
}

.backup-toggle-label input {
  margin-top: 3px;
}

.backup-toggle-label span {
  display: grid;
  gap: 4px;
}

.backup-toggle-label small,
.backup-help,
.backup-entry-info span,
.backup-entry-invalid,
.backup-last-run {
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.backup-help {
  margin: 12px 0 0 28px;
}

.backup-heading-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.backup-heading-row h4 {
  margin: 0;
  font-size: 13px;
}

.backup-heading-row button {
  border: 0;
  background: transparent;
  color: var(--brand-primary);
  cursor: pointer;
}

.backup-heading-row button:disabled {
  cursor: default;
  opacity: 0.55;
}

.backup-error {
  color: var(--danger, #c24141);
  font-size: 13px;
}

.backup-status {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.backup-list {
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.backup-entry {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 13px 14px;
  background: var(--bg-input);
}

.backup-entry + .backup-entry {
  border-top: 1px solid var(--border);
}

.backup-entry-info {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.backup-entry-invalid {
  color: var(--danger, #c24141);
}

.backup-restore-button {
  flex: 0 0 auto;
}

.backup-last-run {
  margin-top: 12px;
}
</style>
