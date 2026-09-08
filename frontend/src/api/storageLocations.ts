import {
  ApplyStorageLocations,
  CancelStorageLocationSelection,
  CancelPendingStorageLocationMigration,
  GetStorageLocationDiagnostics,
  GetStorageLocationStatus,
  RetryPendingStorageLocationMigration,
  SelectStorageLocation,
} from '../../wailsjs/go/main/App'

export type StorageLocationStatus = {
  dataRoot?: string
  backupRoot?: string
  source?: string
  environmentOverride: boolean
  setupRequired: boolean
  recoveryRequired: boolean
  pendingRestart: boolean
  pendingDataRoot?: string
  pendingBackupRoot?: string
  pendingMigration: boolean
  pendingMigrationAction?: string
  pendingSelection: boolean
  dataRootChangeAllowed: boolean
}

export type StorageLocationError = {
  code: string
  message: string
  reason?: string
  stage?: string
  role?: string
  osErrorNumber?: number
  diagnosticId?: string
  action?: string
}

export type StorageLocationDiagnostic = {
  schema: number
  timestamp: string
  diagnosticId: string
  operation: string
  phase?: string
  role?: string
  code: string
  reason?: string
  stage?: string
  osErrorNumber?: number
  os?: string
  arch?: string
  appVersion?: string
  vcsRevision?: string
}

export type StorageLocationDiagnosticsResult = {
  events: StorageLocationDiagnostic[]
  report: string
}

export type StorageLocationStatusResult = {
  status?: StorageLocationStatus
  error?: StorageLocationError
}

export type StorageLocationSelectionResult = {
  kind: string
  path?: string
  probe?: {
    path: string
    kind: string
    exists: boolean
    hasAtlasData: boolean
    hasBackups: boolean
    writable: boolean
  }
  status?: StorageLocationStatus
  error?: StorageLocationError
  canceled: boolean
}

export type StorageLocationMutationResult = {
  status?: StorageLocationStatus
  restartRequired: boolean
  error?: StorageLocationError
}

export function getStorageLocationStatus(): Promise<StorageLocationStatusResult> {
  return GetStorageLocationStatus()
}

export function selectStorageLocation(kind: 'data' | 'backup'): Promise<StorageLocationSelectionResult> {
  return SelectStorageLocation(kind)
}

export function applyStorageLocations(): Promise<StorageLocationMutationResult> {
  return ApplyStorageLocations()
}

export function cancelStorageLocationSelection(): Promise<StorageLocationStatusResult> {
  return CancelStorageLocationSelection()
}

export function cancelPendingStorageLocationMigration(): Promise<StorageLocationMutationResult> {
  return CancelPendingStorageLocationMigration()
}

export function retryPendingStorageLocationMigration(): Promise<StorageLocationMutationResult> {
  return RetryPendingStorageLocationMigration()
}

export function getStorageLocationDiagnostics(): Promise<StorageLocationDiagnosticsResult> {
  return GetStorageLocationDiagnostics()
}
