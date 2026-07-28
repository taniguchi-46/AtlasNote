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
}

export type ConfigureProviderInput = {
  providerID: AIProviderID
  apiKey: string
  modelID: string
}

export type TestConnectionInput = {
  providerID: AIProviderID
  apiKey: string
}

export type ConnectionTestResult = {
  success: boolean
}

export type ListModelsInput = TestConnectionInput

export type ModelInfo = {
  id: string
  displayName: string
  supportsSummary: boolean
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

type AIWailsBridge = {
  ListAIModels(input: ListModelsInput): Promise<ModelListResponse>
  GenerateAISummary(input: GenerateSummaryInput): Promise<SummaryResponse>
  StartAILibrarian(input: LibrarianInput): Promise<LibrarianStartResponse>
  CancelAILibrarian(requestID: string): Promise<LibrarianCancelResponse>
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

export function generateAISummary(input: GenerateSummaryInput): Promise<SummaryResponse> {
  return getAIWailsBridge().GenerateAISummary(input)
}

export function startAILibrarian(input: LibrarianInput): Promise<LibrarianStartResponse> {
  return getAIWailsBridge().StartAILibrarian(input)
}

export function cancelAILibrarian(requestID: string): Promise<LibrarianCancelResponse> {
  return getAIWailsBridge().CancelAILibrarian(requestID)
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
