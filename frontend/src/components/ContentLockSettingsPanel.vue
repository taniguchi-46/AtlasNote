<template>
  <section class="content-lock-settings">
    <h3>ロック</h3>
    <p class="content-lock-description">
      保存空間・ノートブック・ノートごとに本文を暗号化して保護します。保護されたノートはAI機能では利用できません。
    </p>

    <section class="content-lock-auto-lock" aria-labelledby="content-lock-auto-lock-title">
      <h4 id="content-lock-auto-lock-title">自動ロック</h4>
      <label for="content-lock-auto-lock-minutes">ロック解除後</label>
      <select id="content-lock-auto-lock-minutes" v-model.number="settingsStore.contentLockAutoLockMinutes">
        <option v-for="minutes in autoLockMinuteOptions" :key="minutes" :value="minutes">
          {{ autoLockLabel(minutes) }}
        </option>
      </select>
      <p>解除した時点から固定時間で再びロックします。ノートの操作では時間を延長しません。</p>
    </section>

    <div class="content-lock-heading">
      <h4>現在設定されているロック</h4>
      <button type="button" :disabled="lockStore.isLoading || lockStore.isBusy" @click="refresh">
        再読み込み
      </button>
    </div>

    <p v-if="lockStore.error" class="content-lock-settings-error" role="alert">{{ lockStore.error.message }}</p>
    <p v-if="lockStore.isLoading" class="content-lock-settings-status" role="status">ロック設定を読み込んでいます…</p>
    <p v-else-if="lockStore.locks.length === 0" class="content-lock-settings-status">
      ロックはまだ設定されていません。保存空間は「保存空間」、ノートブック・ノートは各項目の編集メニューから設定できます。
    </p>
    <ul v-else class="content-lock-list" aria-label="現在設定されているロック">
      <li v-for="lock in lockStore.locks" :key="lock.id" class="content-lock-card">
        <header>
          <div>
            <span class="content-lock-target-type">{{ targetTypeLabel(lock.targetType) }}</span>
            <strong>{{ lock.targetName }}</strong>
          </div>
          <span :class="['content-lock-badge', { 'is-locked': !lock.unlocked }]">
            {{ lock.unlocked ? '解除中' : 'ロック中' }}
          </span>
        </header>
        <ContentLockControls
          :target="{ type: lock.targetType, id: lock.targetId }"
          :target-label="lock.targetName"
          @changed="refresh"
        />
      </li>
    </ul>

    <p class="content-lock-sync-note">
      既存の同期先がある保存空間では、先に同期を切断してからロックを設定してください。保護済み本文は現在の同期形式では送信されません。
    </p>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import type { ContentLockTargetType } from '../api/contentLocks'
import { useContentLockStore } from '../stores/useContentLockStore'
import {
  CONTENT_LOCK_AUTO_LOCK_MINUTE_OPTIONS,
  type ContentLockAutoLockMinutes,
  useSettingsStore,
} from '../stores/useSettingsStore'
import ContentLockControls from './ContentLockControls.vue'

const lockStore = useContentLockStore()
const settingsStore = useSettingsStore()
const autoLockMinuteOptions = CONTENT_LOCK_AUTO_LOCK_MINUTE_OPTIONS

function targetTypeLabel(type: ContentLockTargetType) {
  return ({ space: '保存空間', notebook: 'ノートブック', note: 'ノート' })[type]
}

function autoLockLabel(minutes: ContentLockAutoLockMinutes) {
  return minutes === 0 ? 'アプリ終了時のみ（既定）' : `${minutes}分`
}

async function refresh() {
  await lockStore.refresh()
}

onMounted(() => {
  void refresh()
})
</script>

<style scoped>
.content-lock-settings {
  color: var(--text-primary);
}

.content-lock-description {
  margin: -10px 0 22px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.content-lock-auto-lock {
  margin: 0 0 24px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-input);
}

.content-lock-auto-lock h4 {
  margin: 0 0 10px;
  font-size: 13px;
}

.content-lock-auto-lock label {
  display: block;
  margin-bottom: 6px;
  color: var(--text-secondary);
  font-size: 12px;
}

.content-lock-auto-lock select {
  width: min(260px, 100%);
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-editor);
  color: var(--text-primary);
  font: inherit;
  font-size: 13px;
}

.content-lock-auto-lock p {
  margin: 9px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.content-lock-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.content-lock-heading h4 {
  margin: 0;
  font-size: 13px;
}

.content-lock-heading button {
  border: 0;
  background: transparent;
  color: var(--brand-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.content-lock-heading button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.content-lock-list {
  display: grid;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.content-lock-card {
  padding: 13px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-input);
}

.content-lock-card header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}

.content-lock-card header > div {
  min-width: 0;
}

.content-lock-card strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.content-lock-target-type {
  display: block;
  margin-bottom: 2px;
  color: var(--text-secondary);
  font-size: 11px;
}

.content-lock-badge {
  flex: 0 0 auto;
  padding: 3px 7px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand-primary) 12%, var(--bg-editor));
  color: var(--brand-primary);
  font-size: 11px;
}

.content-lock-badge.is-locked {
  background: color-mix(in srgb, var(--color-danger, #c0392b) 12%, var(--bg-editor));
  color: var(--color-danger, #c0392b);
}

.content-lock-settings-status,
.content-lock-settings-error,
.content-lock-sync-note {
  font-size: 12px;
  line-height: 1.55;
}

.content-lock-settings-status {
  color: var(--text-secondary);
}

.content-lock-settings-error {
  color: var(--color-danger, #c0392b);
}

.content-lock-sync-note {
  margin: 20px 0 0;
  padding-top: 14px;
  border-top: 1px solid var(--border);
  color: var(--text-secondary);
}
</style>
