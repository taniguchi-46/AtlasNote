import {
  ConfigureAIProvider,
  DeleteAIProviderCredential,
  DeleteAllAICredentials,
  GetAISettings,
  TestAIConnection,
} from '../../wailsjs/go/main/App'

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

type AIWailsBridge = {
  ListAIModels(input: ListModelsInput): Promise<ModelListResponse>
  GenerateAISummary(input: GenerateSummaryInput): Promise<SummaryResponse>
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

export function deleteAIProviderCredential(providerID: AIProviderID): Promise<ProviderSettings[]> {
  return DeleteAIProviderCredential(providerID) as Promise<ProviderSettings[]>
}

export function deleteAllAICredentials(): Promise<ProviderSettings[]> {
  return DeleteAllAICredentials() as Promise<ProviderSettings[]>
}
