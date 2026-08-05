import {
  ConfigureAIProvider,
  DeleteAIProviderCredential,
  DeleteAllAICredentials,
  GetAISettings,
  TestAIConnection,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type AIProviderID = 'openrouter' | 'gemini'

export type CredentialStatus =
  | 'not-configured'
  | 'persistent'
  | 'session-only'
  | 'reauthentication-required'

export type ProviderSettings = {
  providerID: AIProviderID
  modelID: string
  credentialStatus: CredentialStatus
  isSelected: boolean
}

export type ConfigureProviderInput = {
  providerID: AIProviderID
  apiKey: string
  modelID: string
}

export type TestConnectionInput = {
  providerID: AIProviderID
  apiKey: string
  useStoredCredential: boolean
}

export type ConnectionTestResult = {
  success: boolean
}

export type ListModelsInput = TestConnectionInput

export type UpdateProviderModelInput = {
  providerID: AIProviderID
  modelID: string
}

export type TestGenerationInput = TestConnectionInput & {
  modelID: string
}

export type ModelInfo = {
  id: string
  displayName: string
  supportsSummary: boolean
  supportsTextGeneration: boolean
  supportsStructuredOutput: boolean
  supportsStreaming: boolean
  supportsLibrarian: boolean
  inputTokenLimit?: number
  outputTokenLimit?: number
  available: boolean
}

export type SafeAIError = {
  code: string
  retryAfterSeconds?: number
}

export type ModelListResponse = {
  models: ModelInfo[]
  retrievedAt?: string
  error?: SafeAIError
}

export type GenerateSummaryInput = {
  providerID: AIProviderID
  modelID: string
  content: string
}

export type SummaryResponse = {
  text?: string
  error?: SafeAIError
}

export type LibrarianOperation = 'title' | 'tags' | 'classification' | 'related' | 'duplicate'

export type LibrarianCandidateContext = {
  noteID: string
  title: string
  snippet?: string
}

export type LibrarianTagContext = {
  id: string
  name: string
}

export type LibrarianNotebookContext = {
  id: string
  name: string
}

export type LibrarianInput = {
  providerID: AIProviderID
  modelID: string
  operation: LibrarianOperation
  noteID: string
  baseRevision: number
  title: string
  content: string
  candidateCount: number
  candidates?: LibrarianCandidateContext[]
  existingTags?: LibrarianTagContext[]
  notebooks?: LibrarianNotebookContext[]
}

export type LibrarianCandidate = {
  value?: string
  name?: string
  notebookID?: string
  noteID?: string
  score: number
  reason?: string
  newTag?: boolean
}

export type LibrarianResult = {
  operation: LibrarianOperation
  quality: 'normal' | 'low' | 'empty'
  candidates: LibrarianCandidate[]
}

export type LibrarianStartResponse = {
  requestID?: string
  error?: SafeAIError
}

export type LibrarianCancelResponse = {
  canceled: boolean
  error?: SafeAIError
}

export type LibrarianEvent = {
  requestID: string
  noteID: string
  baseRevision: number
  operation: LibrarianOperation
  phase: 'partial' | 'completed' | 'failed' | 'canceled'
  sequence: number
  partialText?: string
  result?: LibrarianResult
  error?: SafeAIError
}

export type AssistantKind = 'qa' | 'brainstorm'
export type AIChatMode = 'ask' | 'agent'

export type AIConversationMessage = {
  role: 'user' | 'assistant'
  content: string
}

export type AIWebCitation = {
  url: string
  title?: string
}

export type AIContextInput = {
  noteIDs?: string[]
  searchQuery?: string
  includeBacklinks?: boolean
}

export type AIContextSource = {
  noteID: string
  title: string
  revision: number
  snippet?: string
  contentByte: number
}

export type AIContextResponse = {
  sources: AIContextSource[]
  error?: SafeAIError
}

export type AssistantInput = {
  providerID: AIProviderID
  modelID: string
  kind: AssistantKind
  mode?: AIChatMode
  question: string
  messages?: AIConversationMessage[]
  noteIDs?: string[]
  searchQuery?: string
  includeBacklinks?: boolean
  webSearch?: boolean
  expectedSources?: AIHistorySource[]
  agentTarget?: AgentEditTarget
}

export type AssistantResult = {
  providerID: AIProviderID
  modelID: string
  kind: AssistantKind
  mode?: AIChatMode
  messages: AIConversationMessage[]
  sources: AIContextSource[]
  citations?: AIWebCitation[]
  webSearchRequests?: number
  proposal?: AgentEditProposal
}

export type AgentEditTarget = {
  noteID: string
  baseRevision: number
}

export type AgentEditProposal = {
  targetNoteID: string
  targetTitle: string
  baseRevision: number
  reason: string
  before: string
  after: string
  affectedFields: string[]
}

export type AssistantResponse = {
  result?: AssistantResult
  error?: SafeAIError
}

export type AIHistorySource = {
  noteID: string
  inputRevision: number
}

export type AIRecordStatus = 'saved' | 'stale' | 'orphaned'

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

export type SaveAIHistoryInput = {
  id?: string
  kind: AssistantKind
  title: string
  providerID: AIProviderID
  modelID: string
  messages: AIConversationMessage[]
  sources: AIHistorySource[]
}

export type AIHistoryResponse = {
  history?: AIHistory
  error?: SafeAIError
}

export type AIHistoryListResponse = {
  items: AIHistory[]
  error?: SafeAIError
}

export type WritingKind = 'prompt' | 'prompt-improvement' | 'readme' | 'document' | 'blog' | 'requirements'

export type WritingInput = {
  providerID: AIProviderID
  modelID: string
  kind: WritingKind
  instruction: string
  noteIDs?: string[]
  searchQuery?: string
  includeBacklinks?: boolean
  expectedSources?: AIHistorySource[]
}

export type WritingResult = {
  providerID: AIProviderID
  modelID: string
  kind: WritingKind
  content: string
  sources: AIContextSource[]
}

export type WritingResponse = {
  result?: WritingResult
  error?: SafeAIError
}

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

export type SaveAIArtifactInput = {
  id?: string
  kind: ArtifactKind
  title: string
  providerID: AIProviderID
  modelID: string
  content: string
  sources: AIHistorySource[]
}

export type ArtifactKind = WritingKind | 'summary'

export type AIArtifactResponse = {
  artifact?: AIArtifact
  error?: SafeAIError
}

export type AIArtifactListResponse = {
  items: AIArtifact[]
  error?: SafeAIError
}

export type AIDeleteResponse = {
  deleted: boolean
  error?: SafeAIError
}

type AIWailsBridge = {
  ListAIModels(input: ListModelsInput): Promise<ModelListResponse>
  TestAIGeneration(input: TestGenerationInput): Promise<ConnectionTestResult>
  UpdateAIProviderModel(input: UpdateProviderModelInput): Promise<ProviderSettings[]>
  GenerateAISummary(input: GenerateSummaryInput): Promise<SummaryResponse>
  StartAILibrarian(input: LibrarianInput): Promise<LibrarianStartResponse>
  CancelAILibrarian(requestID: string): Promise<LibrarianCancelResponse>
  PrepareAIContext(input: AIContextInput): Promise<AIContextResponse>
  RunAIAssistant(input: AssistantInput): Promise<AssistantResponse>
  SaveAIHistory(input: SaveAIHistoryInput): Promise<AIHistoryResponse>
  ListAIHistories(): Promise<AIHistoryListResponse>
  GetAIHistory(id: string): Promise<AIHistoryResponse>
  DeleteAIHistory(id: string): Promise<AIDeleteResponse>
  DeleteAllAIHistories(): Promise<AIDeleteResponse>
  RunAIWriting(input: WritingInput): Promise<WritingResponse>
  SaveAIArtifact(input: SaveAIArtifactInput): Promise<AIArtifactResponse>
  ListAIArtifacts(): Promise<AIArtifactListResponse>
  GetAIArtifact(id: string): Promise<AIArtifactResponse>
  DeleteAIArtifact(id: string): Promise<AIDeleteResponse>
  DeleteAllAIArtifacts(): Promise<AIDeleteResponse>
  DeleteAllAIWritingArtifacts(): Promise<AIDeleteResponse>
}

type WailsWindow = Window & typeof globalThis & {
  go?: {
    main?: {
      App?: AIWailsBridge
    }
  }
}

function getAIWailsBridge(): AIWailsBridge {
  const bridge = (window as WailsWindow).go?.main?.App
  if (!bridge) throw new Error('AI_BACKEND_UNAVAILABLE')
  return bridge
}

export function getAISettings(): Promise<ProviderSettings[]> {
  return GetAISettings() as Promise<ProviderSettings[]>
}

export function configureAIProvider(input: ConfigureProviderInput): Promise<ProviderSettings[]> {
  return ConfigureAIProvider(
    input as unknown as Parameters<typeof ConfigureAIProvider>[0],
  ) as Promise<ProviderSettings[]>
}

export function testAIConnection(input: TestConnectionInput): Promise<ConnectionTestResult> {
  return TestAIConnection(
    input as unknown as Parameters<typeof TestAIConnection>[0],
  ) as Promise<ConnectionTestResult>
}

export function listAIModels(input: ListModelsInput): Promise<ModelListResponse> {
  return getAIWailsBridge().ListAIModels(input)
}

export function testAIGeneration(input: TestGenerationInput): Promise<ConnectionTestResult> {
  return getAIWailsBridge().TestAIGeneration(input)
}

export function updateAIProviderModel(input: UpdateProviderModelInput): Promise<ProviderSettings[]> {
  return getAIWailsBridge().UpdateAIProviderModel(input)
}

export function generateAISummary(input: GenerateSummaryInput): Promise<SummaryResponse> {
  return getAIWailsBridge().GenerateAISummary(input)
}

export function startAILibrarian(input: LibrarianInput): Promise<LibrarianStartResponse> {
  return getAIWailsBridge().StartAILibrarian(input)
}

export function cancelAILibrarian(requestID: string): Promise<LibrarianCancelResponse> {
  return getAIWailsBridge().CancelAILibrarian(requestID)
}

export function prepareAIContext(input: AIContextInput): Promise<AIContextResponse> {
  return getAIWailsBridge().PrepareAIContext(input)
}

export function runAIAssistant(input: AssistantInput): Promise<AssistantResponse> {
  return getAIWailsBridge().RunAIAssistant(input)
}

export function saveAIHistory(input: SaveAIHistoryInput): Promise<AIHistoryResponse> {
  return getAIWailsBridge().SaveAIHistory(input)
}

export function listAIHistories(): Promise<AIHistoryListResponse> {
  return getAIWailsBridge().ListAIHistories()
}

export function getAIHistory(id: string): Promise<AIHistoryResponse> {
  return getAIWailsBridge().GetAIHistory(id)
}

export function deleteAIHistory(id: string): Promise<AIDeleteResponse> {
  return getAIWailsBridge().DeleteAIHistory(id)
}

export function deleteAllAIHistories(): Promise<AIDeleteResponse> {
  return getAIWailsBridge().DeleteAllAIHistories()
}

export function runAIWriting(input: WritingInput): Promise<WritingResponse> {
  return getAIWailsBridge().RunAIWriting(input)
}

export function saveAIArtifact(input: SaveAIArtifactInput): Promise<AIArtifactResponse> {
  return getAIWailsBridge().SaveAIArtifact(input)
}

export function listAIArtifacts(): Promise<AIArtifactListResponse> {
  return getAIWailsBridge().ListAIArtifacts()
}

export function getAIArtifact(id: string): Promise<AIArtifactResponse> {
  return getAIWailsBridge().GetAIArtifact(id)
}

export function deleteAIArtifact(id: string): Promise<AIDeleteResponse> {
  return getAIWailsBridge().DeleteAIArtifact(id)
}

export function deleteAllAIArtifacts(): Promise<AIDeleteResponse> {
  return getAIWailsBridge().DeleteAllAIArtifacts()
}

export function deleteAllAIWritingArtifacts(): Promise<AIDeleteResponse> {
  return getAIWailsBridge().DeleteAllAIWritingArtifacts()
}

export function onAILibrarianUpdate(listener: (event: LibrarianEvent) => void): () => void {
  return EventsOn('ai:librarian:update', (event: LibrarianEvent) => listener(event))
}

export function deleteAIProviderCredential(providerID: AIProviderID): Promise<ProviderSettings[]> {
  return DeleteAIProviderCredential(providerID) as Promise<ProviderSettings[]>
}

export function deleteAllAICredentials(): Promise<ProviderSettings[]> {
  return DeleteAllAICredentials() as Promise<ProviderSettings[]>
}
