// Types retained for saved legacy records and the existing note proposal contract.
// AtlasNote no longer calls provider generation APIs from the frontend.
export type AIProviderID = 'openrouter' | 'gemini'
export type AssistantKind = 'qa' | 'brainstorm'
export type AIConversationMessage = { role: 'user' | 'assistant'; content: string }
export type AIRecordStatus = 'saved' | 'stale' | 'orphaned'
export type AIHistorySource = { noteID: string; inputRevision: number }

export type AgentEditProposal = {
  targetNoteID: string
  targetTitle: string
  baseRevision: number
  reason: string
  before: string
  after: string
  affectedFields: string[]
}

export type AIHistory = {
  id: string
  kind: AssistantKind
  title: string
  providerID: AIProviderID
  modelID: string
  status: AIRecordStatus
  messages?: AIConversationMessage[]
  sources: AIHistorySource[]
  createdAt: string
  updatedAt: string
}

export type ArtifactKind = 'prompt' | 'prompt-improvement' | 'readme' | 'document' | 'blog' | 'requirements' | 'summary'
export type AIArtifact = {
  id: string
  kind: ArtifactKind
  title: string
  providerID: AIProviderID
  modelID: string
  content: string
  status: AIRecordStatus
  sources: AIHistorySource[]
  createdAt: string
  updatedAt: string
}
