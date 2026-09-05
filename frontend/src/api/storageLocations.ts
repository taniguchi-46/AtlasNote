import {
  ApplyStorageLocations,
  CancelStorageLocationSelection,
  CancelPendingStorageLocationMigration,
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
