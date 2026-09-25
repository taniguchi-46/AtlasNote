import {
  AnalyzeOrganization,
  ApplyOrganizationCandidates,
  DiscardOrganizationAnalysis,
  InvalidateOrganizationAnalyses,
} from '../../wailsjs/go/app/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type OrganizationCandidate = {
  id: string
  kind: string
  noteId?: string
  noteTitle?: string
  relatedId?: string
  relatedTitle?: string
  notebookId?: string
  relatedRevision?: number
  relatedContentHash?: string
  tagId?: string
  spaceId: string
  baseRevision?: number
  before: Record<string, unknown>
  proposed?: Record<string, unknown>
  reason: string
  applicable: boolean
}

export type OrganizationAnalysis = {
  sessionId: string
  spaceId: string
  notebookId?: string
  noteId?: string
  scope: OrganizationScope
  startedAt: string
  candidates: OrganizationCandidate[]
  analyzedNotes: number
  skippedLocked: number
  skippedTrash: number
}

export type OrganizationApplyStatus = 'applied' | 'applied-with-draft-conflict' | 'conflict' | 'save-failure' | 'stale' | 'not-executed' | 'not-applicable'

export type OrganizationApplyResult = {
  candidateId: string
  status: OrganizationApplyStatus
  message?: string
}

export type OrganizationScope = 'space' | 'notebook' | 'descendants' | 'note'
export type OrganizationAnalysisInput = { scope: OrganizationScope; notebookId?: string; noteId?: string; requestId?: string }
export type OrganizationProgress = { requestId: string; phase: 'reading' | 'proposing'; processedNotes: number; totalNotes: number }
export type OrganizationApplyInput = { sessionId: string; candidateIds: string[] }

export function analyzeOrganization(input: OrganizationAnalysisInput): Promise<OrganizationAnalysis> {
  return AnalyzeOrganization(input) as Promise<OrganizationAnalysis>
}

export function onOrganizationProgress(listener: (event: OrganizationProgress) => void): () => void {
  return EventsOn('organization:progress', (event: OrganizationProgress) => listener(event))
}

export function applyOrganizationCandidates(input: OrganizationApplyInput): Promise<OrganizationApplyResult[]> {
  return ApplyOrganizationCandidates(input) as Promise<OrganizationApplyResult[]>
}

export function discardOrganizationAnalysis(sessionId: string): Promise<void> {
  return DiscardOrganizationAnalysis(sessionId)
}

export function invalidateOrganizationAnalyses(): Promise<void> {
  return InvalidateOrganizationAnalyses()
}
