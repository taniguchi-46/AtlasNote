import type { AgentEditProposal } from '../api/ai'
import type { AIAgentEditPermission } from '../stores/useSettingsStore'

type AgentProposalResult = {
  content: string
  proposal: AgentEditProposal | null
}

type AgentProposalPermissionFlow = {
  prompt: string
  permission: AIAgentEditPermission
  entryID: string
  submitPrompt: (prompt: string, permission: AIAgentEditPermission) => Promise<boolean>
  readResult: () => AgentProposalResult
  resolveProposal: (
    entryID: string,
    content: string,
    proposal: AgentEditProposal | null,
  ) => void
  autoApply: (entryID: string) => Promise<void>
}

export async function runAgentProposalPermissionFlow({
  prompt,
  permission,
  entryID,
  submitPrompt,
  readResult,
  resolveProposal,
  autoApply,
}: AgentProposalPermissionFlow) {
  const submitted = await submitPrompt(prompt, permission)
  if (!submitted) return false

  const result = readResult()
  resolveProposal(entryID, result.content, result.proposal)
  if (result.proposal && permission === 'auto-update') {
    await autoApply(entryID)
  }
  return true
}
