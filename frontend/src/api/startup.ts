import {
  DeleteMissingNote,
  GetStartupStatus,
  ReinspectRecovery,
} from '../../wailsjs/go/main/App'
import type { StorageSpace } from './storageSpaces'
import type { StorageLocationStatus } from './storageLocations'

export type MissingNoteDiagnostic = {
  id: string
  title: string
  filePath: string
}

export type StartupStatus = {
  phase?: 'initializing' | 'setup-required' | 'storage-recovery' | 'ready' | 'locked' | 'error' | string
  setupRequired?: boolean
  ready: boolean
  locked?: boolean
  degraded: boolean
  message?: string
  dataDir?: string
  missingNotes: MissingNoteDiagnostic[]
  syncRecoveryBackup?: string
  backupRestoreSafetyBackupId?: string
  activeStorageSpace?: StorageSpace
  storageLocations?: StorageLocationStatus
  storageLocationError?: {
    code: string
    message: string
  }
}

export function getStartupStatus(): Promise<StartupStatus> {
  return GetStartupStatus()
}

export function reinspectRecovery(): Promise<StartupStatus> {
  return ReinspectRecovery()
}

export function deleteMissingNote(id: string): Promise<StartupStatus> {
  return DeleteMissingNote(id)
}
