<template>
  <section class="ai-settings">
    <h3>AI</h3>
    <p class="field-help">
      API Key は表示・再表示されず、この画面を閉じると入力中の値も破棄されます。接続確認とモデル取得では保存されません。
    </p>

    <div class="setting-group">
      <label for="ai-provider">プロバイダー</label>
      <select id="ai-provider" v-model="aiStore.draft.providerID" :disabled="aiStore.isSettingsBusy">
        <option value="openrouter">OpenRouter</option>
        <option value="gemini">Google Gemini</option>
      </select>
      <p class="field-help">
        認証情報: {{ credentialStatusLabel(aiStore.activeProviderSetting?.credentialStatus) }}
      </p>
    </div>

    <div class="setting-group wide-field">
      <label for="ai-api-key">API Key</label>
      <input
        id="ai-api-key"
        v-model="aiStore.draft.apiKey"
        type="password"
        autocomplete="new-password"
        spellcheck="false"
        :disabled="aiStore.isSettingsBusy"
      />
      <p class="field-help">
        保存済みの API Key は表示しません。接続確認後にモデル一覧を手動で更新し、適用または OK でのみ保存します。
      </p>
    </div>

    <div class="connection-actions">
      <button type="button" :disabled="!aiStore.canCheckConnection" @click="handleCheckConnection">
        {{ aiStore.isSettingsBusy ? '確認中…' : '接続を確認' }}
      </button>
      <button type="button" :disabled="!aiStore.canRefreshModels" @click="handleRefreshModels">
        モデル一覧を更新
      </button>
    </div>
    <p v-if="aiStore.connectionState === 'success'" class="check-result success" role="status">
      接続を確認しました。モデル一覧を手動で更新してください。
    </p>
    <p v-else-if="aiStore.connectionError" class="check-result error" role="alert">
      {{ aiStore.connectionError.message }}
    </p>
    <p v-if="aiStore.modelsError" class="check-result error" role="alert">
      {{ aiStore.modelsError.message }}
    </p>

    <div class="setting-group wide-field">
      <label for="ai-model">要約モデル</label>
      <select
        id="ai-model"
        v-model="aiStore.draft.modelID"
        :disabled="aiStore.isSettingsBusy || aiStore.models.length === 0"
      >
        <option value="">モデルを選択してください</option>
        <option v-for="model in aiStore.models" :key="model.id" :value="model.id" :disabled="!model.available">
          {{ model.displayName || model.id }}{{ model.available ? '' : '（利用不可）' }}
        </option>
      </select>
      <p v-if="aiStore.modelsRetrievedAt" class="field-help">
        最終取得: {{ formatDate(aiStore.modelsRetrievedAt) }}。自動更新・自動選択は行いません。
      </p>
      <p
        v-if="aiStore.models.length > 0 && aiStore.draft.modelID && !aiStore.selectedModelAvailable"
        class="warning"
        role="alert"
      >
        現在のモデルは利用できません。別のモデルを選択してから適用してください。
      </p>
    </div>

    <details v-if="aiStore.models.length > 0" class="model-metadata">
      <summary>モデル詳細を表示（{{ aiStore.models.length }}件）</summary>
      <ul class="model-metadata-list" aria-label="モデル情報">
        <li v-for="model in aiStore.models" :key="`${model.id}-metadata`">
          <strong>{{ model.displayName || model.id }}</strong>
          <span>入力上限: {{ formatLimit(model.inputTokenLimit) }}</span>
          <span>出力上限: {{ formatLimit(model.outputTokenLimit) }}</span>
        </li>
      </ul>
    </details>

    <p v-if="aiStore.settingsError" class="check-result error" role="alert">
      {{ aiStore.settingsError.message }}
    </p>

    <section class="credential-actions" aria-label="AI 認証情報の削除">
      <p class="field-help">削除操作は確認後に実行します。認証情報の状態はキーを表示せずに示します。</p>
      <button
        type="button"
        class="danger-link"
        :disabled="aiStore.isSettingsBusy || !canDeleteProvider"
        @click="handleDeleteProvider"
      >
        このプロバイダーの認証情報を削除
      </button>
      <button
        type="button"
        class="danger-link"
        :disabled="aiStore.isSettingsBusy || !aiStore.hasConfiguredProvider"
        @click="handleDeleteAll"
      >
        すべての AI 認証情報を削除
      </button>
    </section>

    <footer class="ai-footer">
      <button type="button" :disabled="aiStore.isSettingsBusy" @click="handleBack">戻る</button>
      <span class="footer-spacer" />
      <button type="button" :disabled="!aiStore.canApply" @click="handleApply">適用</button>
      <button type="button" class="primary-button" :disabled="!aiStore.canApply" @click="handleOK">OK</button>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { CredentialStatus } from '../api/ai'
import { useAIStore } from '../stores/useAIStore'
import { useSettingsStore } from '../stores/useSettingsStore'

const aiStore = useAIStore()
const settingsStore = useSettingsStore()

const canDeleteProvider = computed(() => {
  const status = aiStore.activeProviderSetting?.credentialStatus
  return status === 'persistent' || status === 'session-only' || status === 'reauthentication-required'
})

const credentialStatusLabels: Record<CredentialStatus, string> = {
  'not-configured': '未設定',
  persistent: 'この端末に保存済み',
  'session-only': 'このセッションだけで利用',
  'reauthentication-required': '再入力が必要',
}

function credentialStatusLabel(status: CredentialStatus | undefined) {
  return status ? credentialStatusLabels[status] : '未設定'
}

function formatLimit(value: number | undefined) {
  return value ? `${value.toLocaleString()} tokens` : '不明'
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '不明' : date.toLocaleString('ja-JP')
}

async function handleCheckConnection() {
  await aiStore.checkConnection()
}

async function handleRefreshModels() {
  await aiStore.refreshModels()
}

async function handleApply(): Promise<boolean> {
  return aiStore.applyConfiguration()
}

async function handleOK() {
  if (await handleApply()) settingsStore.closeSettings()
}

function handleBack() {
  aiStore.discardDraft()
  settingsStore.closeSettings()
}

async function handleDeleteProvider() {
  if (!window.confirm('現在選択しているプロバイダーの AI 認証情報を削除しますか？')) return
  await aiStore.deleteProvider(aiStore.draft.providerID)
}

async function handleDeleteAll() {
  if (!window.confirm('すべての AI 認証情報を削除しますか？この操作は元に戻せません。')) return
  await aiStore.deleteAllProviders()
}
</script>

<style scoped>
.ai-settings { color: var(--text-primary); padding-bottom: 64px; }
.setting-group { margin: 20px 0; }
.setting-group label { display: block; margin-bottom: 7px; font-size: 14px; font-weight: 600; }
.wide-field input, .wide-field select { width: min(100%, 460px); }
input, select, button { font: inherit; }
input, select { padding: 7px 9px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-input); color: var(--text-primary); }
button { padding: 7px 11px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-input); color: var(--text-primary); cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: 0.55; }
.primary-button { background: var(--brand-primary); color: white; border-color: var(--brand-primary); }
.connection-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.field-help { color: var(--text-secondary); font-size: 13px; line-height: 1.5; margin: 6px 0; max-width: 600px; }
.check-result, .warning { font-size: 13px; line-height: 1.5; }
.check-result.success { color: var(--success-color, #18794e); }
.check-result.error, .warning { color: var(--danger-color, #b42318); }
.model-metadata { margin: 16px 0; }
.model-metadata summary { cursor: pointer; font-size: 13px; font-weight: 600; }
.model-metadata-list { display: grid; gap: 8px; margin: 8px 0 0; padding: 0; list-style: none; }
.model-metadata-list li { display: grid; gap: 3px; padding: 9px; border: 1px solid var(--border); border-radius: 4px; font-size: 13px; }
.model-metadata-list span { color: var(--text-secondary); }
.credential-actions { display: grid; gap: 8px; margin-top: 24px; padding-top: 16px; border-top: 1px solid var(--border); }
.danger-link { color: var(--danger-color, #b42318); justify-self: start; }
.ai-footer { position: sticky; bottom: -24px; display: flex; gap: 9px; margin: 28px -24px -64px; padding: 12px 24px; border-top: 1px solid var(--border); background: var(--bg-editor); }
.footer-spacer { flex: 1; }
</style>
