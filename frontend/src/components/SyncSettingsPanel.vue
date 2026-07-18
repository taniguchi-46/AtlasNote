<template>
  <section class="sync-settings">
    <h3>同期</h3>

    <button type="button" class="secondary-button" @click="wizardOpen = !wizardOpen">
      同期ウィザードを開く…
    </button>
    <div v-if="wizardOpen || syncStore.targetChanged" class="wizard-box">
      <p>新しい同期先では、ローカルをアップロードするか、空のローカルへ既存データを取り込むかを明示してください。</p>
      <label for="sync-setup-mode">初回設定方式</label>
      <select id="sync-setup-mode" v-model="syncStore.draft.setupMode">
        <option value="">選択してください</option>
        <option value="initialize">ローカルデータで新しい同期先を初期化</option>
        <option value="import">既存の同期先を空のローカルへ取り込む</option>
        <option v-if="syncStore.statusResult?.connection" value="reconnect">同じ保管庫へ再接続</option>
      </select>
    </div>

    <div class="setting-group">
      <label for="sync-target">同期先</label>
      <select id="sync-target" value="webdav" disabled>
        <option value="webdav">WebDAV</option>
      </select>
    </div>

    <div class="setting-group wide-field">
      <label for="sync-webdav-url">WebDAV URL</label>
      <input id="sync-webdav-url" v-model.trim="syncStore.draft.webDAVURL" type="url" autocomplete="url" placeholder="https://dav.example.com/atlasnote" />
      <p class="field-help">
        注意: この場所を変更する際は、同期する前に新しい場所へ必要なデータがあることを確認してください。設定ミスによる消失を防ぐため、初回設定方式の選択が必要です。
      </p>
    </div>

    <p v-if="syncStore.targetChanged && syncStore.statusResult?.connection" class="sync-warning" role="alert">
      WebDAV URLまたはユーザー名が変更されています。保存前に同期ウィザードで方式を選択し、パスワードを再入力してください。
    </p>

    <div class="setting-group wide-field">
      <label for="sync-username">WebDAV ユーザー名</label>
      <input id="sync-username" v-model.trim="syncStore.draft.username" type="text" autocomplete="username" />
    </div>
    <div class="setting-group wide-field">
      <label for="sync-password">WebDAV パスワード</label>
      <input id="sync-password" v-model="syncStore.draft.password" type="password" autocomplete="new-password" />
      <p class="field-help">入力したパスワードはOSの資格情報ストアへ保存します。利用できない場合は現在のセッション内だけで保持します。</p>
    </div>

    <div class="setting-group">
      <label for="sync-interval">同期間隔</label>
      <select id="sync-interval" v-model.number="syncStore.draft.syncIntervalSeconds">
        <option :value="0">無効</option>
        <option :value="300">5分</option>
        <option :value="600">10分</option>
        <option :value="1800">30分</option>
        <option :value="3600">1時間</option>
        <option :value="43200">12時間</option>
        <option :value="86400">24時間</option>
      </select>
    </div>

    <button type="button" class="secondary-button" :disabled="syncStore.isBusy" @click="handleCheck">
      同期の設定を確認する
    </button>
    <p v-if="syncStore.configurationTest" class="check-result success" role="status">
      {{ syncStore.configurationTest.remoteInitialized ? '成功です！同期先のAtlas Note保管庫を確認しました。' : '接続に成功しました。同期先はまだ初期化されていません。' }}
    </p>
    <p v-else-if="syncStore.configurationTestError" class="check-result error" role="alert">
      {{ syncStore.configurationTestError }}
    </p>

    <button type="button" class="advanced-toggle" :aria-expanded="advancedOpen" @click="advancedOpen = !advancedOpen">
      <span aria-hidden="true">{{ advancedOpen ? '▾' : '›' }}</span>
      詳細設定を{{ advancedOpen ? '隠す' : '表示' }}
    </button>

    <div v-if="advancedOpen" class="advanced-settings">
      <label class="sync-check">
        <input v-model="syncStore.draft.allowInsecureHTTP" type="checkbox" />
        HTTP接続を許可する
      </label>
      <p v-if="syncStore.draft.allowInsecureHTTP" class="sync-warning" role="alert">
        HTTPではユーザー名、パスワード、同期データが暗号化されません。信頼できる閉域ネットワークでのみ使用してください。
      </p>

      <div class="setting-group wide-field">
        <label for="sync-certificates">TLS証明書のカスタマイズ</label>
        <input id="sync-certificates" v-model.trim="syncStore.draft.customTLSCertificates" type="text" placeholder="C:\certs, C:\custom.pem" />
        <p class="field-help">証明書ディレクトリまたは証明書ファイルをコンマ区切りで指定します。</p>
      </div>
      <label class="sync-check danger-option">
        <input v-model="syncStore.draft.ignoreTLSErrors" type="checkbox" />
        TLS証明書のエラーを無視
      </label>
      <p v-if="syncStore.draft.ignoreTLSErrors" class="sync-warning" role="alert">
        なりすましを検出できなくなります。自己署名証明書は、できるだけ上の証明書設定で信頼してください。
      </p>

      <label class="sync-check">
        <input v-model="syncStore.draft.proxyEnabled" type="checkbox" />
        プロキシの有効化
      </label>
      <div class="setting-group wide-field">
        <label for="sync-proxy-url">プロキシURL</label>
        <input id="sync-proxy-url" v-model.trim="syncStore.draft.proxyURL" type="url" :disabled="!syncStore.draft.proxyEnabled" placeholder="http://proxy.example.com:80" />
      </div>
      <div class="setting-group compact-field">
        <label for="sync-proxy-timeout">プロキシのタイムアウト（秒）</label>
        <input id="sync-proxy-timeout" v-model.number="syncStore.draft.proxyTimeoutSeconds" type="number" min="1" max="60" :disabled="!syncStore.draft.proxyEnabled" />
      </div>

      <label class="sync-check">
        <input v-model="syncStore.draft.failSafe" type="checkbox" />
        フェイルセーフ
      </label>
      <p class="field-help">同期先が空の場合、設定ミスや障害の可能性を考慮してローカルデータを消去しません。</p>

      <div class="recovery-actions">
        <button type="button" :disabled="syncStore.isBusy || !syncStore.statusResult?.connection" @click="handleRecovery('reupload-local')">ローカルデータを同期先に再アップロードする</button>
        <p class="field-help">ローカルデータを正として同期先を再構築します。</p>
        <button type="button" :disabled="syncStore.isBusy || !syncStore.statusResult?.connection" @click="handleRecovery('redownload-remote')">ローカルデータを削除して同期先から再ダウンロードする</button>
        <p class="field-help">同期先を別領域へ完全に検証してから、再起動時にバックアップ付きで置換します。</p>
      </div>
    </div>

    <div class="runtime-section">
      <div class="runtime-heading">
        <p class="sync-status" role="status">
          状態: {{ syncStore.statusLabel }} / 未送信: {{ syncStore.statusResult?.outboxCount ?? 0 }} / 競合: {{ syncStore.statusResult?.conflictCount ?? 0 }}
        </p>
        <button type="button" :disabled="syncStore.isBusy || !syncStore.statusResult?.connection" @click="handleSync">今すぐ同期</button>
      </div>
      <p v-if="syncStore.syncError" class="check-result error" role="alert">
        {{ syncStore.syncError }}
      </p>
      <ul v-if="syncStore.conflicts.length > 0" class="sync-conflicts">
        <li v-for="conflict in syncStore.conflicts" :key="conflict.id">
          <span>{{ conflict.entityType }}: {{ conflict.entityKey }}</span>
          <span class="sync-conflict-actions">
            <button type="button" :disabled="syncStore.isBusy" @click="handleResolve(conflict.id, 'local')">ローカルを採用</button>
            <button type="button" :disabled="syncStore.isBusy" @click="handleResolve(conflict.id, 'remote')">リモートを採用</button>
            <button v-if="conflict.entityType === 'note'" type="button" :disabled="syncStore.isBusy" @click="handleResolve(conflict.id, 'both')">両方保持</button>
          </span>
        </li>
      </ul>
      <button type="button" class="danger-link" :disabled="syncStore.isBusy || !syncStore.statusResult?.connection" @click="handleDisconnect">同期設定を解除</button>
    </div>

    <footer class="sync-footer">
      <button type="button" @click="handleBack">戻る</button>
      <span class="footer-spacer" />
      <button type="button" :disabled="syncStore.isBusy" @click="handleApply">適用</button>
      <button type="button" class="primary-button" :disabled="syncStore.isBusy" @click="handleOK">OK</button>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useSettingsStore } from '../stores/useSettingsStore'
import { useSyncStore } from '../stores/useSyncStore'
import type { SyncRecoveryAction } from '../api/sync'

const settingsStore = useSettingsStore()
const syncStore = useSyncStore()
const advancedOpen = ref(false)
const wizardOpen = ref(false)

async function handleCheck() {
  try {
    await syncStore.checkConfiguration()
  } catch (_) {
    // The inline result contains the user-facing failure.
  }
}

async function handleApply(): Promise<boolean> {
  try {
    await syncStore.configure()
    return true
  } catch (_) {
    return false
  }
}

async function handleOK() {
  if (await handleApply()) settingsStore.closeSettings()
}

function handleBack() {
  syncStore.discardDraft()
  settingsStore.closeSettings()
}

async function handleSync() {
  try {
    await syncStore.runSync({ forceRetry: true })
  } catch (_) {
    // The store has already emitted a user-facing notification.
  }
}

async function handleDisconnect() {
  if (!window.confirm('同期設定を解除しますか？ ローカルデータは削除されません。')) return
  try {
    await syncStore.disconnect()
  } catch (_) {
    // The store has already emitted a user-facing notification.
  }
}

async function handleRecovery(action: SyncRecoveryAction) {
  try {
    const preview = await syncStore.prepareRecovery(action)
    if (!preview) return
    const counts = `\nローカル項目: ${preview.localItems}\n同期先項目: ${preview.remoteItems}`
    if (!window.confirm(`${preview.message}${counts}\n\nこの操作を実行しますか？`)) return
    const result = await syncStore.executeRecovery(preview.token)
    if (result?.restartRequired) {
      window.alert(`${result.message}\nバックアップ先: ${result.backupPath ?? '次回起動時に作成'}`)
      await syncStore.quitForRecovery()
    }
  } catch (_) {
    // The store has already emitted a user-facing notification.
  }
}

async function handleResolve(conflictId: string, choice: 'local' | 'remote' | 'both') {
  const confirmations = {
    local: 'ローカル版を採用し、同期先の版を置き換えます。続行しますか？',
    remote: '同期先の版を採用し、現在のローカル版を置き換えます。続行しますか？',
    both: 'ローカル版を採用し、同期先の版を別ノートとして保持します。続行しますか？',
  }
  if (!window.confirm(confirmations[choice])) return
  try {
    await syncStore.resolveConflict(conflictId, choice)
  } catch (_) {
    // The store has already emitted a user-facing notification.
  }
}
</script>

<style scoped>
.sync-settings { color: var(--text-primary); padding-bottom: 64px; }
.setting-group { margin: 20px 0; }
.setting-group label, .wizard-box label { display: block; margin-bottom: 7px; font-size: 14px; font-weight: 600; }
.wide-field input { width: min(100%, 460px); }
.compact-field input { width: 90px; }
input, select, button { font: inherit; }
input, select { padding: 7px 9px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-input); color: var(--text-primary); }
button { padding: 7px 11px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-input); color: var(--text-primary); cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: 0.55; }
.primary-button { background: var(--brand-primary); color: white; border-color: var(--brand-primary); }
.secondary-button { margin-bottom: 12px; }
.wizard-box, .advanced-settings, .runtime-section { border: 1px solid var(--border); border-radius: 6px; padding: 14px; margin: 10px 0 18px; }
.wizard-box p, .field-help, .sync-status { color: var(--text-secondary); font-size: 13px; line-height: 1.5; }
.field-help { margin: 6px 0; max-width: 600px; }
.sync-warning, .check-result.error { color: var(--danger-color, #b42318); font-size: 13px; line-height: 1.5; }
.check-result.success { color: var(--success-color, #18794e); font-size: 13px; }
.advanced-toggle { display: flex; gap: 8px; margin-top: 18px; }
.sync-check { display: flex; gap: 8px; align-items: flex-start; margin: 12px 0; font-size: 13px; color: var(--text-secondary); }
.danger-option { color: var(--danger-color, #b42318); }
.recovery-actions { border-top: 1px solid var(--border); margin-top: 18px; padding-top: 16px; }
.recovery-actions button { display: block; margin-top: 12px; }
.runtime-heading { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.sync-conflicts { display: grid; gap: 8px; padding: 0; list-style: none; }
.sync-conflicts li { display: flex; justify-content: space-between; gap: 10px; padding: 9px; border: 1px solid var(--border); border-radius: 4px; font-size: 13px; }
.sync-conflict-actions { display: flex; flex-wrap: wrap; gap: 6px; }
.danger-link { margin-top: 10px; color: var(--danger-color, #b42318); }
.sync-footer { position: sticky; bottom: -24px; display: flex; gap: 9px; margin: 28px -24px -64px; padding: 12px 24px; border-top: 1px solid var(--border); background: var(--bg-editor); }
.footer-spacer { flex: 1; }
@media (max-width: 720px) {
  .runtime-heading, .sync-conflicts li { align-items: stretch; flex-direction: column; }
  .sync-footer { bottom: -24px; }
}
</style>
