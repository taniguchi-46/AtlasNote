import {
  CancelBackupRestore,
  CreateAutomaticBackup,
  ExecuteBackupRestore,
  GetBackupStatus,
  ListBackups,
  PreviewBackupRestore,
} from '../../wailsjs/go/main/App'

export type BackupError = {
  code: string
  message: string
}

export type BackupSummary = {
  id: string
  kind: string
  createdAt: string
  sizeBytes: number
  fileCount: number
  restorable: boolean
  errorMessage?: string
}

export type BackupListResult = {
  backups: BackupSummary[]
  error?: BackupError
}

export type BackupStatusResult = {
  automaticEnabled: boolean
  automaticDue: boolean
  lastAutomaticAt?: string
  backupCount: number
  pendingRestore: boolean
  error?: BackupError
}

export type AutomaticBackupResult = {
  created: boolean
  skipped: boolean
  backup?: BackupSummary
  error?: BackupError
}

export type BackupRestorePreview = {
  token: string
  backupId: string
  createdAt: string
  sizeBytes: number
  fileCount: number
  message: string
}

export type BackupRestorePreviewResult = {
  preview?: BackupRestorePreview
  error?: BackupError
}

export type BackupRestoreResult = {
  backupId?: string
  restartRequired: boolean
  restoreSafetyBackupId?: string
  canceled: boolean
  message?: string
  error?: BackupError
}

export function getBackupStatus(): Promise<BackupStatusResult> {
  return GetBackupStatus()
}

export function listBackups(): Promise<BackupListResult> {
  return ListBackups()
}

export function createAutomaticBackup(): Promise<AutomaticBackupResult> {
  return CreateAutomaticBackup()
}

export function previewBackupRestore(backupId: string): Promise<BackupRestorePreviewResult> {
  return PreviewBackupRestore(backupId)
}

export function executeBackupRestore(token: string): Promise<BackupRestoreResult> {
  return ExecuteBackupRestore({ token })
}

export function cancelBackupRestore(): Promise<BackupRestoreResult> {
  return CancelBackupRestore()
}
