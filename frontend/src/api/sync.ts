import {
  ConfigureSync,
  DisconnectSync,
  ExecuteSyncRecovery,
  GetSyncStatus,
  ListSyncConflicts,
  PrepareSyncRecovery,
  QuitForSyncRecovery,
  ResolveSyncConflict,
  SyncNow,
  TestSyncConfiguration,
} from '../../wailsjs/go/main/App'

export type SyncStatus =
  | 'disabled'
  | 'idle'
  | 'pending'
  | 'syncing'
  | 'synced'
  | 'offline'
  | 'failed'
  | 'conflict'
  | 'auth-required'

export type SyncConnection = {
  webDAVURL: string
  username: string
  lastSyncAt?: string
  hasLastSync: boolean
  status: SyncStatus
  syncIntervalSeconds: SyncIntervalSeconds
  allowInsecureHTTP: boolean
  customTLSCertificates: string
  ignoreTLSErrors: boolean
  proxyEnabled: boolean
  proxyURL: string
  proxyTimeoutSeconds: number
  failSafe: boolean
}

export type SyncIntervalSeconds = 0 | 300 | 600 | 1800 | 3600 | 43200 | 86400
export type SyncSetupMode = '' | 'initialize' | 'import' | 'reconnect' | 'update'

export type SyncStatusResult = {
  connection?: SyncConnection
  status: SyncStatus
  outboxCount: number
  conflictCount: number
  message?: string
}

export type SyncConnectionInput = {
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
  initializeRemote: boolean
}

export type SyncConfigurationTestResult = {
  success: boolean
  remoteInitialized: boolean
  message: string
}

export type SyncNowInput = {
  initializeRemote: boolean
  importRemote: boolean
  forceRetry: boolean
}

export type SyncResult = {
  status: SyncStatus
  uploaded: number
  downloaded: number
  conflicts: number
  remaining: number
  message?: string
}

export type SyncConflict = {
  id: string
  entityKey: string
  entityType: string
  conflictType: string
  createdAt?: string
}

export type SyncRecoveryAction = 'reupload-local' | 'redownload-remote'

export type SyncRecoveryPreview = {
  token: string
  action: SyncRecoveryAction
  localItems: number
  remoteItems: number
  message: string
}

export type SyncRecoveryResult = {
  action: SyncRecoveryAction
  restartRequired: boolean
  backupPath?: string
  message: string
}

export function getSyncStatus(): Promise<SyncStatusResult> {
  return GetSyncStatus() as Promise<SyncStatusResult>
}

export function listSyncConflicts(): Promise<SyncConflict[]> {
  return ListSyncConflicts() as Promise<SyncConflict[]>
}

export function configureSync(input: SyncConnectionInput): Promise<SyncStatusResult> {
  return ConfigureSync(input as unknown as Parameters<typeof ConfigureSync>[0]) as Promise<SyncStatusResult>
}

export function testSyncConfiguration(input: SyncConnectionInput): Promise<SyncConfigurationTestResult> {
  return TestSyncConfiguration(input as unknown as Parameters<typeof TestSyncConfiguration>[0]) as Promise<SyncConfigurationTestResult>
}

export function syncNow(input: SyncNowInput = { initializeRemote: false, importRemote: false, forceRetry: false }): Promise<SyncResult> {
  return SyncNow(input) as Promise<SyncResult>
}

export function resolveSyncConflict(conflictId: string, choice: 'local' | 'remote' | 'both'): Promise<void> {
  return ResolveSyncConflict({ conflictId, choice })
}

export function disconnectSync(): Promise<void> {
  return DisconnectSync()
}

export function prepareSyncRecovery(action: SyncRecoveryAction): Promise<SyncRecoveryPreview> {
  return PrepareSyncRecovery(action) as Promise<SyncRecoveryPreview>
}

export function executeSyncRecovery(token: string): Promise<SyncRecoveryResult> {
  return ExecuteSyncRecovery({ token }) as Promise<SyncRecoveryResult>
}

export function quitForSyncRecovery(): Promise<void> {
  return QuitForSyncRecovery()
}
