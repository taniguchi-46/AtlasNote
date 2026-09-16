import {
  GetDiagnostics,
  RecordOperationFailure,
  SaveDiagnostics,
} from '../../wailsjs/go/main/App'
import type {
  StorageLocationDiagnostic,
  StorageLocationDiagnosticsResult,
} from './storageLocations'

export type OperationFailureInput = {
  operation: 'frontend'
  phase: 'runtime'
  role: 'frontend'
  stage: string
  errorCategory: string
}

export type DiagnosticsSaveResult = {
  saved: boolean
  cancelled: boolean
  savedName?: string
  error?: string
}

export function getDiagnostics(): Promise<StorageLocationDiagnosticsResult> {
  return GetDiagnostics()
}

export function recordOperationFailure(input: OperationFailureInput): Promise<StorageLocationDiagnostic> {
  return RecordOperationFailure(input)
}

export function saveDiagnostics(): Promise<DiagnosticsSaveResult> {
  return SaveDiagnostics()
}
