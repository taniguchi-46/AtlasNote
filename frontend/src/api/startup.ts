import {
  DeleteMissingNote,
  GetStartupStatus,
  ReinspectRecovery,
} from '../../wailsjs/go/main/App'
import type { StorageSpace } from './storageSpaces'

export type MissingNoteDiagnostic = {
  id: string
  title: string
  filePath: string
}

export type StartupStatus = {
  ready: boolean
  locked?: boolean
  degraded: boolean
  message?: string
  dataDir?: string
  missingNotes: MissingNoteDiagnostic[]
  syncRecoveryBackup?: string
  backupRestoreSafetyBackupId?: string
  activeStorageSpace?: StorageSpace
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
