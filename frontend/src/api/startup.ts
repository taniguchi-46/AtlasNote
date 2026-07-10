import {
  DeleteMissingNote,
  GetStartupStatus,
  ReinspectRecovery,
} from '../../wailsjs/go/main/App'

export type MissingNoteDiagnostic = {
  id: string
  title: string
  filePath: string
}

export type StartupStatus = {
  ready: boolean
  degraded: boolean
  message?: string
  dataDir?: string
  missingNotes: MissingNoteDiagnostic[]
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
