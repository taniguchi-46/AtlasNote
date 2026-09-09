import type { AIChatTimelineEntry, AIChatTool } from '../stores/useAIChatStore'
import type { LibrarianState } from '../stores/useAILibrarianStore'

type TimelineStatus = 'success' | 'error'
type TimelineUpdate = { content: string; status: TimelineStatus }

type CompleteLibrarianTraceInput = {
  traceID: string | null
  state: LibrarianState
  errorMessage?: string
  updateTimelineEntry: (traceID: string, update: TimelineUpdate) => void
}

export function completeLibrarianTimelineTrace({
  traceID,
  state,
  errorMessage,
  updateTimelineEntry,
}: CompleteLibrarianTraceInput) {
  if (!traceID || state === 'generating' || state === 'partial' || state === 'canceling') {
    return false
  }

  const successful = state === 'success' || state === 'empty'
  updateTimelineEntry(traceID, {
    content: successful
      ? state === 'empty' ? '候補は見つかりませんでした。' : '候補を生成しました。内容を確認してください。'
      : errorMessage ?? '候補生成を完了できませんでした。',
    status: successful ? 'success' : 'error',
  })
  return true
}

type RecordAssistantFailureInput = {
  errorMessage: string
  tool: AIChatTool | null
  agentProposalEntryID: string | null
  traceID: string | null
  removeTimelineEntry: (entryID: string) => void
  appendError: (content: string, tool?: AIChatTool) => string
  updateTimelineEntry: (traceID: string, update: TimelineUpdate) => void
}

export function recordAssistantTimelineFailure({
  errorMessage,
  tool,
  agentProposalEntryID,
  traceID,
  removeTimelineEntry,
  appendError,
  updateTimelineEntry,
}: RecordAssistantFailureInput) {
  if (agentProposalEntryID) removeTimelineEntry(agentProposalEntryID)
  appendError(errorMessage, tool ?? undefined)
  if (traceID) {
    updateTimelineEntry(traceID, {
      content: errorMessage,
      status: 'error',
    })
  }
}

type CleanupInterruptedComposerSubmissionInput = {
  timeline: readonly AIChatTimelineEntry[]
  userEntryID: string
  agentProposalEntryID: string | null
  traceID: string | null
  removeTimelineEntry: (entryID: string) => void
  clearResultForTrace: (traceID: string) => void
}

export function cleanupInterruptedComposerSubmission({
  timeline,
  userEntryID,
  agentProposalEntryID,
  traceID,
  removeTimelineEntry,
  clearResultForTrace,
}: CleanupInterruptedComposerSubmissionInput) {
  const userEntry = timeline.find((entry) => entry.id === userEntryID)
  if (userEntry) removeTimelineEntry(userEntryID)

  const agentProposalEntry = agentProposalEntryID
    ? timeline.find((entry) => entry.id === agentProposalEntryID)
    : undefined
  if (
    agentProposalEntryID
    && agentProposalEntry?.kind === 'agent-proposal'
    && agentProposalEntry.proposalState === 'generating'
  ) {
    removeTimelineEntry(agentProposalEntryID)
  }

  const traceEntry = traceID
    ? timeline.find((entry) => entry.id === traceID)
    : undefined
  if (traceID && traceEntry?.kind === 'tool-trace' && traceEntry.status === 'pending') {
    removeTimelineEntry(traceID)
    clearResultForTrace(traceID)
  }
}
