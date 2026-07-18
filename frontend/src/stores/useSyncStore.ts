import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  configureSync,
  disconnectSync,
  executeSyncRecovery,
  getSyncStatus,
  listSyncConflicts,
  prepareSyncRecovery,
  quitForSyncRecovery,
  resolveSyncConflict,
  syncNow,
  testSyncConfiguration,
  type SyncConfigurationTestResult,
  type SyncConnectionInput,
  type SyncConflict,
  type SyncIntervalSeconds,
  type SyncResult,
  type SyncRecoveryAction,
  type SyncRecoveryPreview,
  type SyncRecoveryResult,
  type SyncSetupMode,
  type SyncStatus,
  type SyncStatusResult,
} from '../api/sync'
import { useNotificationStore } from './useNotificationStore'

export type SyncSettingsDraft = {
  webDAVURL: string
  username: string
  password: string
  syncIntervalSeconds: SyncIntervalSeconds
  allowInsecureHTTP: boolean
  customTLSCertificates: string
  ignoreTLSErrors: boolean
  proxyEnabled: boolean
  proxyURL: string
  proxyTimeoutSeconds: number
  failSafe: boolean
  setupMode: SyncSetupMode
}

type SyncRunOptions = {
  initializeRemote?: boolean
  importRemote?: boolean
  forceRetry?: boolean
  suppressFailureNotification?: boolean
}

function emptyDraft(): SyncSettingsDraft {
  return {
    webDAVURL: '',
    username: '',
    password: '',
    syncIntervalSeconds: 0,
    allowInsecureHTTP: false,
    customTLSCertificates: '',
    ignoreTLSErrors: false,
    proxyEnabled: false,
    proxyURL: '',
    proxyTimeoutSeconds: 1,
    failSafe: true,
    setupMode: '',
  }
}

const statusLabels: Record<SyncStatus, string> = {
  disabled: '未設定',
  idle: '待機中',
  pending: '同期待ち',
  syncing: '同期中',
  synced: '同期済み',
  offline: 'オフライン',
  failed: '失敗',
  conflict: '競合あり',
  'auth-required': '認証が必要',
}

const retryDelays = [15_000, 60_000, 300_000] as const

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : typeof error === 'string' ? error : ''
}

function configurationErrorMessage(error: unknown): string {
  const value = errorText(error).toLowerCase()
  if (value.includes('401') && value.includes('server requested digest authentication')) {
    return 'Digest認証で拒否されました。ユーザー名とパスワード、またはWebDAVサーバー側のアクセス権を確認してください。'
  }
  if (value.includes('401') && value.includes('propfind') && value.includes('basic credentials were sent')) {
    return 'Basic認証情報は送信済みですが、同期先がPROPFINDを401で拒否しました。WebDAVサーバー側のユーザー権限または認証プロキシ設定を確認してください。'
  }
  if (value.includes('401') && value.includes('propfind')) {
    return 'WebDAVの確認要求（PROPFIND）が拒否されました。資格情報が正しくても、URL末尾の「/」またはサーバーのWebDAV認証設定が原因になることがあります。'
  }
  if (value.includes('401') || value.includes('authentication')) return '認証に失敗しました。ユーザー名とパスワードを確認してください。'
  if (value.includes('403') || value.includes('permission')) return '同期先へのアクセス権限がありません。WebDAV側の権限を確認してください。'
  if (value.includes('deadline') || value.includes('timeout')) return '同期先への接続がタイムアウトしました。URL、ネットワーク、プロキシを確認してください。'
  if (value.includes('certificate') || value.includes('tls')) return 'TLS証明書を検証できませんでした。証明書設定を確認してください。'
  if (value.includes('proxy')) return 'プロキシ設定が正しくありません。HTTPまたはHTTPSのURLを確認してください。'
  if (value.includes('format') || value.includes('etag')) return '同期先のAtlas Note保管庫を検証できませんでした。'
  if (value.includes('credentials')) return '保存済みの認証情報を取得できません。パスワードを再入力してください。'
  if (value.includes('endpoint')) return 'WebDAV URLが正しくありません。HTTPを使う場合は詳細設定で明示的に許可してください。'
  return '同期設定を確認できませんでした。入力内容と接続先を確認してください。'
}

function syncErrorMessage(error: unknown): string {
  const value = errorText(error).toLowerCase()
  if (value.includes('401') || value.includes('authentication')) return configurationErrorMessage(error)
  if (value.includes('mkcol')) {
    if (value.includes('405') || value.includes('501')) {
      return '同期先はフォルダ作成（MKCOL）に対応していません。WebDAVサーバーで書込み操作を有効にしてください。'
    }
    return '同期先が同期用フォルダの作成（MKCOL）を拒否しました。ユーザーの書込み権限とWebDAVサーバー設定を確認してください。'
  }
  if (value.includes('403') || value.includes('permission')) return configurationErrorMessage(error)
  if (value.includes('put')) {
    return '同期先が同期データの書込み（PUT）を拒否しました。ユーザーの書込み権限とWebDAVサーバー設定を確認してください。'
  }
  if (value.includes('strong etag') || value.includes('etag')) {
    return '同期先が安全なETagを返さないため、競合を防止する同期を開始できません。WebDAVサーバーのETag設定を確認してください。'
  }
  if (value.includes('remote vault') || value.includes('format')) {
    return '同期先の初期化データを検証できませんでした。失敗後に作成されたAtlas Note用ファイルがあれば削除してから、初期化をやり直してください。'
  }
  if (value.includes('deadline') || value.includes('timeout')) return configurationErrorMessage(error)
  return '同期に失敗しました。同期先の書込み権限とWebDAV設定を確認してください。'
}

export const useSyncStore = defineStore('sync', () => {
  const status = ref<SyncStatus>('disabled')
  const statusResult = ref<SyncStatusResult | null>(null)
  const conflicts = ref<SyncConflict[]>([])
  const isBusy = ref(false)
  const draft = ref<SyncSettingsDraft>(emptyDraft())
  const configurationTest = ref<SyncConfigurationTestResult | null>(null)
  const configurationTestError = ref('')
  const syncError = ref('')
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let retryAttempt = 0
  let retryOptions: SyncRunOptions = {}
  let beforeSync: (() => Promise<unknown>) | null = null

  const notificationStore = useNotificationStore()
  const statusLabel = computed(() => statusLabels[status.value])
  const syncIntervalSeconds = computed(() => statusResult.value?.connection?.syncIntervalSeconds ?? 0)
  const autoSync = computed(() => syncIntervalSeconds.value > 0)
  const targetChanged = computed(() => {
    const connection = statusResult.value?.connection
    if (!connection) return draft.value.webDAVURL.trim() !== '' || draft.value.username.trim() !== ''
    return connection.webDAVURL !== draft.value.webDAVURL.trim() || connection.username !== draft.value.username.trim()
  })

  function applyStatus(result: SyncStatusResult) {
    statusResult.value = result
    status.value = result.status
  }

  function resetDraft() {
    const connection = statusResult.value?.connection
    if (!connection) {
      draft.value = emptyDraft()
    } else {
      draft.value = {
        webDAVURL: connection.webDAVURL,
        username: connection.username,
        password: '',
        syncIntervalSeconds: connection.syncIntervalSeconds,
        allowInsecureHTTP: connection.allowInsecureHTTP,
        customTLSCertificates: connection.customTLSCertificates,
        ignoreTLSErrors: connection.ignoreTLSErrors,
        proxyEnabled: connection.proxyEnabled,
        proxyURL: connection.proxyURL,
        proxyTimeoutSeconds: connection.proxyTimeoutSeconds,
        failSafe: connection.failSafe,
        setupMode: 'update',
      }
    }
    configurationTest.value = null
    configurationTestError.value = ''
  }

  function discardDraft() {
    resetDraft()
  }

  async function refresh(loadDraft = false) {
    const result = await getSyncStatus()
    applyStatus(result)
    conflicts.value = result.conflictCount > 0 ? await listSyncConflicts() : []
    if (loadDraft) resetDraft()
    return result
  }

  async function initialize() {
    try {
      const result = await refresh(true)
      if ((result.connection?.syncIntervalSeconds ?? 0) > 0 && result.status !== 'auth-required' && result.status !== 'failed') {
        startPolling(result.connection!.syncIntervalSeconds)
        void runSync().catch(() => {})
      }
    } catch (error) {
      status.value = 'failed'
      notificationStore.notify('同期状態を取得できませんでした', {
        kind: 'warning', source: 'sync', code: 'SYNC_STATUS_FAILED',
      })
      throw error
    }
  }

  function buildInput(): SyncConnectionInput {
    let setupMode = draft.value.setupMode
    if (statusResult.value?.connection && !targetChanged.value && (setupMode === '' || setupMode === 'update')) setupMode = 'update'
    return {
      webDAVURL: draft.value.webDAVURL.trim(),
      username: draft.value.username.trim(),
      password: draft.value.password,
      syncIntervalSeconds: draft.value.syncIntervalSeconds,
      allowInsecureHTTP: draft.value.allowInsecureHTTP,
      customTLSCertificates: draft.value.customTLSCertificates.trim(),
      ignoreTLSErrors: draft.value.ignoreTLSErrors,
      proxyEnabled: draft.value.proxyEnabled,
      proxyURL: draft.value.proxyURL.trim(),
      proxyTimeoutSeconds: draft.value.proxyTimeoutSeconds,
      failSafe: draft.value.failSafe,
      setupMode,
      initializeRemote: setupMode === 'initialize',
    }
  }

  async function checkConfiguration() {
    if (isBusy.value) return null
    isBusy.value = true
    configurationTest.value = null
    configurationTestError.value = ''
    try {
      const result = await testSyncConfiguration(buildInput())
      configurationTest.value = result
      return result
    } catch (error) {
      configurationTestError.value = configurationErrorMessage(error)
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function configure() {
    if (isBusy.value) return null
    const input = buildInput()
    isBusy.value = true
    syncError.value = ''
    let saved = false
    try {
      if (input.setupMode !== 'update' && beforeSync) await beforeSync()
      const result = await configureSync(input)
      applyStatus(result)
      saved = true
      if (input.setupMode !== 'update') clearRetryState()
      if (result.message) {
        notificationStore.notify('パスワードはこのセッション内だけで保持されます', {
          kind: 'warning', source: 'sync', code: 'SYNC_CREDENTIAL_SESSION_ONLY',
        })
      } else {
        notificationStore.notify('同期設定を保存しました', {
          kind: 'success', source: 'sync', code: 'SYNC_CONFIGURED',
        })
      }
      if ((result.connection?.syncIntervalSeconds ?? 0) > 0) startPolling(result.connection!.syncIntervalSeconds)
      else stopPolling()
      if (input.setupMode === 'initialize' || input.setupMode === 'import') {
        await runSync({
          initializeRemote: input.setupMode === 'initialize',
          importRemote: input.setupMode === 'import',
          forceRetry: true,
          suppressFailureNotification: true,
        }, true)
      } else {
        await refresh(false)
      }
      resetDraft()
      return result
    } catch (error) {
      const message = syncError.value || syncErrorMessage(error)
      syncError.value = message
      notificationStore.notify(saved ? `同期設定は保存されましたが、初回同期に失敗しました: ${message}` : `同期設定を保存できませんでした: ${message}`, {
        kind: 'error', source: 'sync', code: saved ? 'SYNC_INITIAL_SYNC_FAILED' : 'SYNC_CONFIGURE_FAILED', retryable: true,
      })
      throw error
    } finally {
      if (saved) draft.value.password = ''
      isBusy.value = false
    }
  }

  async function runSync(options: SyncRunOptions = {}, allowBusy = false) {
    if (isBusy.value && !allowBusy) return null
    if (options.forceRetry) clearRetryState()
    isBusy.value = true
    status.value = 'syncing'
    try {
      if (beforeSync) await beforeSync()
      const result = await syncNow({
        initializeRemote: options.initializeRemote ?? false,
        importRemote: options.importRemote ?? false,
        forceRetry: options.forceRetry ?? false,
      })
      syncError.value = ''
      applySyncResult(result)
      const refreshed = await refresh(false)
      const interval = refreshed.connection?.syncIntervalSeconds ?? 0
      if (interval > 0 && refreshed.status !== 'auth-required' && refreshed.status !== 'failed') startPolling(interval)
      if (result.status === 'pending' && result.remaining > 0) scheduleRetry(options)
      else if (result.status !== 'offline') clearRetryState()
      return result
    } catch (error) {
      const message = syncErrorMessage(error)
      syncError.value = message
      try {
        await refresh(false)
      } catch (_) {
        status.value = 'failed'
      }
      const currentStatus = status.value as SyncStatus
      if (currentStatus === 'offline') {
        const scheduled = scheduleRetry(options)
        if (!scheduled && !retryTimer && retryAttempt >= retryDelays.length) {
          status.value = 'failed'
          stopPolling()
        }
      } else if (currentStatus === 'auth-required' || currentStatus === 'failed') {
        clearRetryState()
        stopPolling()
      }
      if (!options.suppressFailureNotification) {
        notificationStore.notify(`同期に失敗しました: ${message}`, {
          kind: 'error', source: 'sync', code: 'SYNC_FAILED', retryable: true,
        })
      }
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function resolveConflict(conflictId: string, choice: 'local' | 'remote' | 'both') {
    if (isBusy.value) return
    isBusy.value = true
    try {
      await resolveSyncConflict(conflictId, choice)
      const result = await refresh(false)
      if (result.status === 'pending') scheduleAutoSync()
      notificationStore.notify('競合を解決しました', {
        kind: 'success', source: 'sync', code: 'SYNC_CONFLICT_RESOLVED',
      })
    } catch (error) {
      notificationStore.notify('同期競合の解決に失敗しました', {
        kind: 'error', source: 'sync', code: 'SYNC_CONFLICT_RESOLVE_FAILED', retryable: true,
      })
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function disconnect() {
    if (isBusy.value) return
    isBusy.value = true
    try {
      await disconnectSync()
      await refresh(true)
      syncError.value = ''
      clearRetryState()
      stopPolling()
      notificationStore.notify('同期設定を解除しました', {
        kind: 'success', source: 'sync', code: 'SYNC_DISCONNECTED',
      })
    } catch (error) {
      notificationStore.notify('同期設定の解除に失敗しました', {
        kind: 'error', source: 'sync', code: 'SYNC_DISCONNECT_FAILED', retryable: true,
      })
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function prepareRecovery(action: SyncRecoveryAction): Promise<SyncRecoveryPreview | null> {
    if (isBusy.value) return null
    isBusy.value = true
    try {
      if (beforeSync) await beforeSync()
      return await prepareSyncRecovery(action)
    } catch (error) {
      notificationStore.notify('同期復旧の事前確認に失敗しました', {
        kind: 'error', source: 'sync', code: 'SYNC_RECOVERY_PREPARE_FAILED', retryable: true,
      })
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function executeRecovery(token: string): Promise<SyncRecoveryResult | null> {
    if (isBusy.value) return null
    isBusy.value = true
    try {
      const result = await executeSyncRecovery(token)
      if (!result.restartRequired) {
        clearRetryState()
        await refresh(true)
        notificationStore.notify('同期先の再アップロードが完了しました', {
          kind: 'success', source: 'sync', code: 'SYNC_RECOVERY_REUPLOAD_COMPLETED',
        })
      }
      return result
    } catch (error) {
      notificationStore.notify('同期復旧を実行できませんでした', {
        kind: 'error', source: 'sync', code: 'SYNC_RECOVERY_FAILED', retryable: true,
      })
      throw error
    } finally {
      isBusy.value = false
    }
  }

  async function quitForRecovery() {
    await quitForSyncRecovery()
  }

  function applySyncResult(result: SyncResult) {
    status.value = result.status
    if (result.conflicts > 0) {
      notificationStore.notify('同期競合があります', {
        kind: 'warning', source: 'sync', code: 'SYNC_CONFLICT',
      })
    } else if (result.uploaded > 0 || result.downloaded > 0) {
      notificationStore.notify('同期が完了しました', {
        kind: 'success', source: 'sync', code: 'SYNC_COMPLETED',
      })
    }
  }

  function scheduleAutoSync() {
    if (!autoSync.value || !statusResult.value?.connection) return
    status.value = 'pending'
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      debounceTimer = null
      void runSync()
    }, 5000)
  }

  function startPolling(seconds = syncIntervalSeconds.value) {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = null
    if (seconds <= 0) return
    pollTimer = setInterval(() => {
      if (!isBusy.value) void runSync()
    }, seconds * 1000)
  }

  function scheduleRetry(options: SyncRunOptions = {}): boolean {
    if (retryTimer || retryAttempt >= retryDelays.length || !statusResult.value?.connection) return false
    retryOptions = {
      initializeRemote: options.initializeRemote ?? false,
      importRemote: options.importRemote ?? false,
      forceRetry: false,
    }
    const delay = retryDelays[retryAttempt]
    retryAttempt += 1
    const execute = () => {
      retryTimer = null
      if (isBusy.value) {
        retryTimer = setTimeout(execute, 1000)
        return
      }
      void runSync(retryOptions).catch(() => {})
    }
    retryTimer = setTimeout(execute, delay)
    return true
  }

  function clearRetryState() {
    if (retryTimer) clearTimeout(retryTimer)
    retryTimer = null
    retryAttempt = 0
    retryOptions = {}
  }

  function stopPolling() {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = null
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = null
  }

  function dispose() {
    stopPolling()
    clearRetryState()
    beforeSync = null
  }

  function setBeforeSync(callback: (() => Promise<unknown>) | null) {
    beforeSync = callback
  }

  return {
    status,
    statusLabel,
    statusResult,
    conflicts,
    isBusy,
    draft,
    configurationTest,
    configurationTestError,
    syncError,
    syncIntervalSeconds,
    autoSync,
    targetChanged,
    refresh,
    initialize,
    resetDraft,
    discardDraft,
    checkConfiguration,
    configure,
    runSync,
    resolveConflict,
    disconnect,
    prepareRecovery,
    executeRecovery,
    quitForRecovery,
    scheduleAutoSync,
    startPolling,
    stopPolling,
    dispose,
    setBeforeSync,
  }
})
