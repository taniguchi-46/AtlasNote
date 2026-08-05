import type { AgentEditProposal } from '../api/ai'

export type AgentEditPatchResult =
  | { status: 'ok'; content: string }
  | { status: 'conflict'; reason: 'not-found' | 'ambiguous' | 'invalid-empty-target' | 'unchanged' }

// applyAgentEditHunk deliberately accepts only one exact, unique replacement.
// This keeps a stale or ambiguous AI proposal from replacing an unrelated part
// of the active note body.
export function applyAgentEditHunk(
  content: string,
  proposal: Pick<AgentEditProposal, 'before' | 'after'>,
): AgentEditPatchResult {
  if (proposal.before === proposal.after) return { status: 'conflict', reason: 'unchanged' }
  if (proposal.before === '') return { status: 'conflict', reason: 'invalid-empty-target' }

  const first = content.indexOf(proposal.before)
  if (first < 0) return { status: 'conflict', reason: 'not-found' }
  if (content.indexOf(proposal.before, first + 1) >= 0) {
    return { status: 'conflict', reason: 'ambiguous' }
  }
  return {
    status: 'ok',
    content: `${content.slice(0, first)}${proposal.after}${content.slice(first + proposal.before.length)}`,
  }
}
