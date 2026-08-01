import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import {
  configureAIProvider,
  deleteAIProviderCredential,
  deleteAllAICredentials,
  deleteAIArtifact,
  generateAISummary,
  getAIArtifact,
  getAISettings,
  listAIArtifacts,
  listAIModels,
  saveAIArtifact,
  testAIGeneration,
  testAIConnection,
  updateAIProviderModel,
  type AIArtifact,
  type AIProviderID,
  type ModelInfo,
  type ProviderSettings,
} from '../api/ai'
import { useNotificationStore } from './useNotificationStore'

export type AISettingsDraft = {
  providerID: AIProviderID
  apiKey: string
  modelID: string
}

export type AIErrorState = {
  code: string
  message: string
  retryAfterSeconds?: number
}

type CredentialSource = 'draft' | 'stored'

export type SummarySource = {
  noteID: string
  title?: string
  content: string
  baseRevision: number
}

export type SummarySnapshot = SummarySource & {
  providerID: AIProviderID
  modelID: string
}

export type SummaryConfirmationContext = {
  noteID: string | null
  content: string | null
  revision: number | null
  hasPendingDraft: boolean
}

export type SummaryResult = {
  noteID: string
  title: string
  baseRevision: number
  providerID: AIProviderID
  modelID: string
  text: string
  artifactID?: string
}

export type SummaryState = 'idle' | 'confirming' | 'generating' | 'success' | 'error'

type ActiveSummaryRequest = {
  id: number
  snapshot: SummarySnapshot
  discarded: boolean
}

const providerIDs: AIProviderID[] = ['openrouter', 'gemini']

const safeMessages: Record<string, string> = {
  AI_PROVIDER_UNSUPPORTED: '選択した AI プロバイダーは利用できません。',
  AI_API_KEY_INVALID: 'API Key を確認してください。',
  AI_CONFIGURATION_UNAVAILABLE: 'AI 設定を確認してください。',
  AI_CREDENTIAL_UNAVAILABLE: 'AI 認証情報を利用できません。API Key を再入力してください。',
  AI_CREDENTIAL_CLEANUP_REQUIRED: 'AI 認証情報の更新後処理に失敗しました。設定を確認してください。',
  AI_REAUTHENTICATION_REQUIRED: 'AI 認証情報の再入力が必要です。',
  AI_AUTH_FAILED: 'AI 認証に失敗しました。API Key を確認してください。',
  AI_PROVIDER_CONFIGURATION_REQUIRED: 'AI プロバイダーのプロジェクト設定、利用可能な地域、または無料枠の利用状況を確認してください。',
  AI_MODEL_UNAVAILABLE: '選択したモデルは利用できません。モデルを再選択してください。',
  AI_MODEL_CAPABILITY_UNAVAILABLE: '選択したモデルは、このAI機能に必要な応答形式へ対応していません。別のモデルを選択してください。',
  AI_OUTPUT_LIMIT: 'モデルの出力上限に達したため、応答を完了できませんでした。別のモデルで再試行してください。',
  AI_CONTENT_BLOCKED: '入力または応答が AI プロバイダーの安全基準によりブロックされました。内容を見直して再試行してください。',
  AI_INPUT_TOO_LARGE: '本文が選択したモデルのコンテキスト上限を超えています。より長いコンテキストのモデルを選ぶか、本文を分けて再試行してください。',
  AI_INPUT_INVALID: '本文が空か無効なため、要約を送信しません。',
  AI_RATE_LIMITED: 'AI プロバイダーの利用上限に達しました。時間をおいてから再試行してください。',
  AI_TIMEOUT: 'AI プロバイダーが時間内に応答しませんでした。',
  AI_NETWORK_UNAVAILABLE: 'ネットワークに接続できません。接続を確認してください。',
  AI_PROVIDER_UNAVAILABLE: 'AI プロバイダーを現在利用できません。',
  AI_BUSY: '別の AI 要約を実行中です。完了してから再試行してください。',
  AI_INVALID_RESPONSE: 'AI プロバイダーから有効な応答を受け取れませんでした。',
  AI_SUMMARY_NOT_READY: 'AI の接続確認とモデル選択を完了してから要約してください。',
  AI_DRAFT_NOT_SAVED: '未保存の変更を保存できないため、要約を送信しません。',
  AI_NOTE_UNAVAILABLE: 'このノートは要約できません。',
  AI_HISTORY_SAVE_FAILED: '要約は生成されましたが、履歴を保存できませんでした。もう一度保存してください。',
  AI_ARTIFACT_NOT_FOUND: '要約履歴が見つかりません。',
}

function emptyDraft(providerID: AIProviderID = 'openrouter', modelID = ''): AISettingsDraft {
  return { providerID, apiKey: '', modelID }
}

function isConfiguredCredential(status: ProviderSettings['credentialStatus']) {
  return status === 'persistent' || status === 'session-only'
}

function isProviderID(value: string): value is AIProviderID {
  return providerIDs.includes(value as AIProviderID)
}

function normalizeSettings(value: ProviderSettings[]): ProviderSettings[] {
  return value.filter((setting) => isProviderID(setting.providerID))
}

function retryAfterSeconds(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return undefined
  return Math.min(86_400, Math.floor(value))
}

function safeErrorCode(error: unknown): string {
  if (error && typeof error === 'object' && 'code' in error && typeof error.code === 'string') {
    return safeMessages[error.code] ? error.code : 'AI_PROVIDER_UNAVAILABLE'
  }

  const value = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
  const code = Object.keys(safeMessages).find((candidate) => value.includes(candidate))
  return code ?? 'AI_PROVIDER_UNAVAILABLE'
}

function createSafeError(code: string, retryAfter?: unknown): AIErrorState {
  const safeCode = safeMessages[code] ? code : 'AI_PROVIDER_UNAVAILABLE'
  const retryAfterValue = retryAfterSeconds(retryAfter)
  if (safeCode === 'AI_RATE_LIMITED' && retryAfterValue) {
    return {
      code: safeCode,
      message: `${retryAfterValue} 秒後を目安に、もう一度要約を開始してください。`,
      retryAfterSeconds: retryAfterValue,
    }
  }

  return {
    code: safeCode,
    message: safeMessages[safeCode],
    ...(retryAfterValue ? { retryAfterSeconds: retryAfterValue } : {}),
  }
}

function errorFromUnknown(error: unknown): AIErrorState {
  const retryAfter = error && typeof error === 'object' && 'retryAfterSeconds' in error
    ? error.retryAfterSeconds
    : undefined
  return createSafeError(safeErrorCode(error), retryAfter)
}

export const useAIStore = defineStore('ai', () => {
  const notificationStore = useNotificationStore()
  const settings = ref<ProviderSettings[]>([])
  const draft = ref<AISettingsDraft>(emptyDraft())
  const models = ref<ModelInfo[]>([])
  const modelsRetrievedAt = ref<string | null>(null)
  const isSettingsBusy = ref(false)
  const connectionState = ref<'idle' | 'success' | 'error'>('idle')
  const connectionError = ref<AIErrorState | null>(null)
  const modelsError = ref<AIErrorState | null>(null)
  const settingsError = ref<AIErrorState | null>(null)
  const generationTestState = ref<'idle' | 'success' | 'error'>('idle')
  const generationTestError = ref<AIErrorState | null>(null)
  const summaryState = ref<SummaryState>('idle')
  const summaryTargetNoteID = ref<string | null>(null)
  const pendingSummary = ref<SummarySnapshot | null>(null)
  const summary = ref<SummaryResult | null>(null)
  const summaryError = ref<AIErrorState | null>(null)
  const summaryHistory = ref<AIArtifact[]>([])
  const summaryHistoryError = ref<AIErrorState | null>(null)
  const isGenerating = ref(false)

  let verifiedProviderID: AIProviderID | null = null
  let verifiedAPIKey = ''
  let verifiedCredentialSource: CredentialSource | null = null
  let listedProviderID: AIProviderID | null = null
  let nextSummaryRequestID = 0
  let activeSummaryRequest: ActiveSummaryRequest | null = null

  const configuredSetting = computed(() => {
    const isConfigured = (setting: ProviderSettings) => (
      isConfiguredCredential(setting.credentialStatus) && setting.modelID.trim() !== ''
    )
    return settings.value.find((setting) => (
      setting.providerID === draft.value.providerID && isConfigured(setting)
    )) ?? settings.value.find(isConfigured) ?? null
  })

  const activeProviderSetting = computed(() => settings.value.find(
    (setting) => setting.providerID === draft.value.providerID,
  ) ?? null)

  const selectedModel = computed(() => models.value.find(
    (model) => model.id === draft.value.modelID,
  ) ?? null)

  const selectedModelAvailable = computed(() => Boolean(
    selectedModel.value?.available && selectedModel.value.supportsSummary,
  ))

  const canUseStoredCredential = computed(() => Boolean(
    activeProviderSetting.value && isConfiguredCredential(activeProviderSetting.value.credentialStatus),
  ))

  const currentCredentialSource = computed<CredentialSource | null>(() => {
    if (draft.value.apiKey.trim() !== '') return 'draft'
    return canUseStoredCredential.value ? 'stored' : null
  })

  const isCurrentCredentialVerified = computed(() => (
    verifiedProviderID === draft.value.providerID
    && verifiedCredentialSource === currentCredentialSource.value
    && (verifiedCredentialSource !== 'draft' || verifiedAPIKey === draft.value.apiKey)
    && connectionState.value === 'success'
  ))

  const canCheckConnection = computed(() => (
    !isSettingsBusy.value && currentCredentialSource.value !== null
  ))

  const canRefreshModels = computed(() => (
    !isSettingsBusy.value
    && currentCredentialSource.value !== null
    && (currentCredentialSource.value === 'stored' || isCurrentCredentialVerified.value)
  ))

  const canApply = computed(() => (
    !isSettingsBusy.value
    && listedProviderID === draft.value.providerID
    && (currentCredentialSource.value === 'stored' || isCurrentCredentialVerified.value)
    && draft.value.modelID.trim() !== ''
    && selectedModelAvailable.value
  ))

  const canTestGeneration = computed(() => canApply.value && !isSettingsBusy.value)

  const isSummaryReady = computed(() => {
    const setting = configuredSetting.value
    if (!setting) return false
    const model = models.value.find((candidate) => candidate.id === setting.modelID)
    return !model || (model.available && model.supportsSummary)
  })

  const isLibrarianReady = computed(() => {
    const setting = configuredSetting.value
    if (!setting) return false
    const model = models.value.find((candidate) => candidate.id === setting.modelID)
    return !model || (model.available && model.supportsLibrarian)
  })

  const hasConfiguredProvider = computed(() => settings.value.some(
    (setting) => isConfiguredCredential(setting.credentialStatus),
  ))

  function clearVerification() {
    verifiedProviderID = null
    verifiedAPIKey = ''
    verifiedCredentialSource = null
    listedProviderID = null
    models.value = []
    modelsRetrievedAt.value = null
    connectionState.value = 'idle'
    connectionError.value = null
    modelsError.value = null
    generationTestState.value = 'idle'
    generationTestError.value = null
  }

  function matchesCurrentDraft(input: { providerID: AIProviderID; apiKey: string }) {
    return draft.value.providerID === input.providerID && draft.value.apiKey === input.apiKey
  }

  function credentialInputForDraft() {
    const apiKey = draft.value.apiKey
    return {
      providerID: draft.value.providerID,
      apiKey,
      useStoredCredential: apiKey.trim() === '',
    }
  }

  function matchesCurrentCredentialInput(input: { providerID: AIProviderID; apiKey: string; useStoredCredential: boolean }) {
    return matchesCurrentDraft(input) && input.useStoredCredential === (draft.value.apiKey.trim() === '')
  }

  function resetDraft() {
    const preferred = configuredSetting.value
      ?? settings.value.find((setting) => setting.modelID.trim() !== '')
      ?? settings.value[0]
    const providerID = preferred && isProviderID(preferred.providerID) ? preferred.providerID : 'openrouter'
    draft.value = emptyDraft(providerID, preferred?.modelID ?? '')
    if (verifiedProviderID && verifiedProviderID !== providerID) clearVerification()
    settingsError.value = null
  }

  function discardDraft() {
    resetDraft()
  }

  async function refreshSettings() {
    try {
      settings.value = normalizeSettings(await getAISettings())
      settingsError.value = null
      return settings.value
    } catch (error) {
      settingsError.value = errorFromUnknown(error)
      return null
    }
  }

  async function initialize() {
    await refreshSettings()
    clearVerification()
    resetDraft()
  }

  function setSummaryPreconditionError(
    code: 'AI_SUMMARY_NOT_READY' | 'AI_DRAFT_NOT_SAVED' | 'AI_NOTE_UNAVAILABLE' | 'AI_INPUT_TOO_LARGE' | 'AI_INPUT_INVALID' | 'AI_BUSY',
    noteID: string | null = null,
  ) {
    pendingSummary.value = null
    summary.value = null
    summaryTargetNoteID.value = noteID
    summaryError.value = createSafeError(code)
    summaryState.value = 'error'
  }

  async function checkConnection() {
    if (!canCheckConnection.value) {
      connectionError.value = createSafeError(
        draft.value.apiKey.trim() === '' ? 'AI_REAUTHENTICATION_REQUIRED' : 'AI_API_KEY_INVALID',
      )
      connectionState.value = 'error'
      return false
    }
    if (isSettingsBusy.value) return false

    isSettingsBusy.value = true
    connectionError.value = null
    connectionState.value = 'idle'
    const input = credentialInputForDraft()
    try {
      const result = await testAIConnection(input)
      if (!matchesCurrentCredentialInput(input)) return false
      if (!result.success) {
        connectionError.value = createSafeError('AI_PROVIDER_UNAVAILABLE')
        connectionState.value = 'error'
        return false
      }
      verifiedProviderID = input.providerID
      verifiedAPIKey = input.apiKey
      verifiedCredentialSource = input.useStoredCredential ? 'stored' : 'draft'
      listedProviderID = null
      models.value = []
      modelsRetrievedAt.value = null
      modelsError.value = null
      connectionState.value = 'success'
      return true
    } catch (error) {
      if (!matchesCurrentCredentialInput(input)) return false
      clearVerification()
      connectionError.value = errorFromUnknown(error)
      connectionState.value = 'error'
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  async function refreshModels() {
    if (!canRefreshModels.value || isSettingsBusy.value) {
      modelsError.value = createSafeError('AI_SUMMARY_NOT_READY')
      return false
    }

    isSettingsBusy.value = true
    modelsError.value = null
    const input = credentialInputForDraft()
    try {
      const response = await listAIModels(input)
      if (!matchesCurrentCredentialInput(input)) return false
      if (response.error) {
        modelsError.value = createSafeError(response.error.code, response.error.retryAfterSeconds)
        return false
      }
      models.value = response.models.filter((model) => model.supportsSummary)
      modelsRetrievedAt.value = response.retrievedAt ?? null
      listedProviderID = input.providerID
      return true
    } catch (error) {
      if (!matchesCurrentCredentialInput(input)) return false
      modelsError.value = errorFromUnknown(error)
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  async function applyConfiguration() {
    if (!canApply.value || isSettingsBusy.value) {
      settingsError.value = createSafeError('AI_SUMMARY_NOT_READY')
      return false
    }

    isSettingsBusy.value = true
    settingsError.value = null
    const input = {
      providerID: draft.value.providerID,
      apiKey: draft.value.apiKey,
      modelID: draft.value.modelID,
    }
    try {
      settings.value = normalizeSettings(input.apiKey.trim() === ''
        ? await updateAIProviderModel({ providerID: input.providerID, modelID: input.modelID })
        : await configureAIProvider(input))
      draft.value.apiKey = ''
      return true
    } catch (error) {
      settingsError.value = errorFromUnknown(error)
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  async function testGeneration() {
    if (!canTestGeneration.value || isSettingsBusy.value) {
      generationTestError.value = createSafeError('AI_SUMMARY_NOT_READY')
      generationTestState.value = 'error'
      return false
    }

    isSettingsBusy.value = true
    generationTestError.value = null
    generationTestState.value = 'idle'
    const credentialInput = credentialInputForDraft()
    const input = { ...credentialInput, modelID: draft.value.modelID }
    try {
      const result = await testAIGeneration(input)
      if (!matchesCurrentCredentialInput(input) || draft.value.modelID !== input.modelID) return false
      if (!result.success) {
        generationTestError.value = createSafeError('AI_PROVIDER_UNAVAILABLE')
        generationTestState.value = 'error'
        return false
      }
      generationTestState.value = 'success'
      return true
    } catch (error) {
      if (!matchesCurrentCredentialInput(input) || draft.value.modelID !== input.modelID) return false
      generationTestError.value = errorFromUnknown(error)
      generationTestState.value = 'error'
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  async function deleteProvider(providerID: AIProviderID) {
    if (isSettingsBusy.value) return false
    isSettingsBusy.value = true
    settingsError.value = null
    try {
      settings.value = normalizeSettings(await deleteAIProviderCredential(providerID))
      clearVerification()
      resetDraft()
      return true
    } catch (error) {
      settingsError.value = errorFromUnknown(error)
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  async function deleteAllProviders() {
    if (isSettingsBusy.value) return false
    isSettingsBusy.value = true
    settingsError.value = null
    try {
      settings.value = normalizeSettings(await deleteAllAICredentials())
      clearVerification()
      resetDraft()
      return true
    } catch (error) {
      settingsError.value = errorFromUnknown(error)
      return false
    } finally {
      isSettingsBusy.value = false
    }
  }

  function beginSummary(source: SummarySource) {
    if (isGenerating.value) {
      return false
    }
    const setting = configuredSetting.value
    if (!setting || !isSummaryReady.value) {
      setSummaryPreconditionError('AI_SUMMARY_NOT_READY', source.noteID)
      return false
    }
    if (!source.noteID || source.content.trim() === '') {
      setSummaryPreconditionError('AI_INPUT_INVALID', source.noteID)
      return false
    }
    pendingSummary.value = {
      ...source,
      providerID: setting.providerID,
      modelID: setting.modelID,
    }
    summaryTargetNoteID.value = source.noteID
    summary.value = null
    summaryError.value = null
    summaryState.value = 'confirming'
    return true
  }

  function cancelSummaryConfirmation() {
    if (summaryState.value !== 'confirming') return
    pendingSummary.value = null
    summaryTargetNoteID.value = null
    summaryState.value = 'idle'
  }

  async function confirmSummary(context: SummaryConfirmationContext) {
    const snapshot = pendingSummary.value
    const setting = configuredSetting.value
    if (!snapshot || !setting || !isSummaryReady.value || (
      setting.providerID !== snapshot.providerID || setting.modelID !== snapshot.modelID
    ) || (
      draft.value.providerID !== snapshot.providerID || draft.value.modelID !== snapshot.modelID
    )) {
      setSummaryPreconditionError('AI_SUMMARY_NOT_READY', snapshot?.noteID ?? null)
      return false
    }
    if (
      context.noteID !== snapshot.noteID
      || context.content !== snapshot.content
      || context.revision !== snapshot.baseRevision
      || context.hasPendingDraft
    ) {
      setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', snapshot.noteID)
      return false
    }
    if (isGenerating.value) {
      summaryError.value = createSafeError('AI_BUSY')
      summaryState.value = 'error'
      pendingSummary.value = null
      summaryTargetNoteID.value = snapshot.noteID
      return false
    }

    const request: ActiveSummaryRequest = {
      id: ++nextSummaryRequestID,
      snapshot,
      discarded: false,
    }
    activeSummaryRequest = request
    pendingSummary.value = null
    summary.value = null
    summaryError.value = null
    summaryState.value = 'generating'
    isGenerating.value = true

    try {
      const response = await generateAISummary({
        providerID: snapshot.providerID,
        modelID: snapshot.modelID,
        content: snapshot.content,
      })
      if (request.discarded) return false
      if (response.error) {
        summaryError.value = createSafeError(response.error.code, response.error.retryAfterSeconds)
        summaryState.value = 'error'
        return false
      }
      if (!response.text || response.text.trim() === '') {
        summaryError.value = createSafeError('AI_INVALID_RESPONSE')
        summaryState.value = 'error'
        return false
      }
      summary.value = {
        noteID: snapshot.noteID,
        title: snapshot.title ?? '',
        baseRevision: snapshot.baseRevision,
        providerID: snapshot.providerID,
        modelID: snapshot.modelID,
        text: response.text,
      }
      summaryState.value = 'success'
      await saveSummaryHistory(summary.value)
      return true
    } catch (error) {
      if (request.discarded) return false
      summaryError.value = errorFromUnknown(error)
      summaryState.value = 'error'
      return false
    } finally {
      if (activeSummaryRequest?.id === request.id) {
        activeSummaryRequest = null
        isGenerating.value = false
      }
      if (request.discarded) {
        pendingSummary.value = null
        summary.value = null
        summaryTargetNoteID.value = null
        summaryError.value = null
        summaryState.value = 'idle'
        notificationStore.notify('要約結果はノートを切り替えたため破棄しました。', {
          kind: 'info',
          source: 'ai',
          code: 'AI_SUMMARY_DISCARDED',
          dedupeKey: 'ai:summary-discarded',
        })
      }
    }
  }

  function discardSummary() {
    if (activeSummaryRequest) activeSummaryRequest.discarded = true
    pendingSummary.value = null
    summary.value = null
    summaryTargetNoteID.value = null
    summaryError.value = null
    summaryState.value = 'idle'
  }

  function discardSummaryForActiveNote(activeNoteID: string | null) {
    if (pendingSummary.value && pendingSummary.value.noteID !== activeNoteID) {
      pendingSummary.value = null
      summaryError.value = null
      if (!isGenerating.value) summaryState.value = 'idle'
    }
    if (activeSummaryRequest && activeSummaryRequest.snapshot.noteID !== activeNoteID) {
      activeSummaryRequest.discarded = true
      summaryError.value = null
      summaryState.value = 'idle'
    }
  }

  async function saveSummaryHistory(result: SummaryResult) {
    summaryHistoryError.value = null
    try {
      const response = await saveAIArtifact({
        kind: 'summary',
        title: `${result.title || '無題のノート'}の要約`,
        providerID: result.providerID,
        modelID: result.modelID,
        content: result.text,
        sources: [{ noteID: result.noteID, inputRevision: result.baseRevision }],
        ...(result.artifactID ? { id: result.artifactID } : {}),
      })
      if (response.error || !response.artifact) {
        summaryHistoryError.value = createSafeError(response.error?.code ?? 'AI_HISTORY_SAVE_FAILED', response.error?.retryAfterSeconds)
        return false
      }
      if (summary.value && summary.value.noteID === result.noteID && summary.value.text === result.text) {
        summary.value = { ...summary.value, artifactID: response.artifact.id }
      }
      summaryHistory.value = [
        response.artifact,
        ...summaryHistory.value.filter((item) => item.id !== response.artifact!.id),
      ]
      return true
    } catch (error) {
      summaryHistoryError.value = errorFromUnknown(error)
      if (summaryHistoryError.value.code === 'AI_PROVIDER_UNAVAILABLE') {
        summaryHistoryError.value = createSafeError('AI_HISTORY_SAVE_FAILED')
      }
      return false
    }
  }

  async function retrySaveSummaryHistory() {
    if (!summary.value) return false
    return saveSummaryHistory(summary.value)
  }

  async function refreshSummaryHistory() {
    try {
      const response = await listAIArtifacts()
      if (response.error) {
        summaryHistoryError.value = createSafeError(response.error.code, response.error.retryAfterSeconds)
        return false
      }
      summaryHistory.value = response.items.filter((item) => item.kind === 'summary')
      summaryHistoryError.value = null
      return true
    } catch (error) {
      summaryHistoryError.value = errorFromUnknown(error)
      return false
    }
  }

  async function loadSummaryHistory(id: string) {
    try {
      const response = await getAIArtifact(id)
      if (response.error || !response.artifact || response.artifact.kind !== 'summary') {
        summaryHistoryError.value = createSafeError(response.error?.code ?? 'AI_ARTIFACT_NOT_FOUND', response.error?.retryAfterSeconds)
        return false
      }
      const artifact = response.artifact
      const source = artifact.sources[0]
      if (!source) {
        summaryHistoryError.value = createSafeError('AI_ARTIFACT_NOT_FOUND')
        return false
      }
      summary.value = {
        noteID: source.noteID,
        title: artifact.title.replace(/の要約$/, ''),
        baseRevision: source.inputRevision,
        providerID: artifact.providerID,
        modelID: artifact.modelID,
        text: artifact.content,
        artifactID: artifact.id,
      }
      summaryTargetNoteID.value = source.noteID
      pendingSummary.value = null
      summaryError.value = null
      summaryHistoryError.value = null
      summaryState.value = 'success'
      return true
    } catch (error) {
      summaryHistoryError.value = errorFromUnknown(error)
      return false
    }
  }

  async function removeSummaryHistory(id: string) {
    try {
      const response = await deleteAIArtifact(id)
      if (response.error || !response.deleted) {
        summaryHistoryError.value = createSafeError(response.error?.code ?? 'AI_ARTIFACT_NOT_FOUND', response.error?.retryAfterSeconds)
        return false
      }
      summaryHistory.value = summaryHistory.value.filter((item) => item.id !== id)
      if (summary.value?.artifactID === id) discardSummary()
      return true
    } catch (error) {
      summaryHistoryError.value = errorFromUnknown(error)
      return false
    }
  }

  watch(
    () => draft.value.providerID,
    (providerID) => {
      const setting = settings.value.find((candidate) => candidate.providerID === providerID)
      if (setting && draft.value.modelID !== setting.modelID) {
        draft.value.modelID = setting.modelID
      }
      clearVerification()
    },
  )

  watch(
    () => draft.value.apiKey,
    (apiKey) => {
      const source = currentCredentialSource.value
      if (
        source !== verifiedCredentialSource
        || (source === 'draft' && (apiKey !== verifiedAPIKey || draft.value.providerID !== verifiedProviderID))
      ) {
        clearVerification()
      }
    },
  )

  watch(
    () => draft.value.modelID,
    () => {
      generationTestState.value = 'idle'
      generationTestError.value = null
    },
  )

  return {
    settings,
    draft,
    models,
    modelsRetrievedAt,
    isSettingsBusy,
    connectionState,
    connectionError,
    modelsError,
    settingsError,
    generationTestState,
    generationTestError,
    summaryState,
    summaryTargetNoteID,
    pendingSummary,
    summary,
    summaryError,
    summaryHistory,
    summaryHistoryError,
    isGenerating,
    configuredSetting,
    activeProviderSetting,
    selectedModel,
    selectedModelAvailable,
    canCheckConnection,
    canRefreshModels,
    canApply,
    canTestGeneration,
    isSummaryReady,
    isLibrarianReady,
    hasConfiguredProvider,
    resetDraft,
    discardDraft,
    refreshSettings,
    initialize,
    setSummaryPreconditionError,
    checkConnection,
    refreshModels,
    applyConfiguration,
    testGeneration,
    deleteProvider,
    deleteAllProviders,
    beginSummary,
    cancelSummaryConfirmation,
    confirmSummary,
    discardSummary,
    discardSummaryForActiveNote,
    retrySaveSummaryHistory,
    refreshSummaryHistory,
    loadSummaryHistory,
    removeSummaryHistory,
  }
})
