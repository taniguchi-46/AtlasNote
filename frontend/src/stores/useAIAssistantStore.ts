import { computed, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import {
  cancelAIAssistant,
  deleteAIHistory,
  deleteAllAIHistories,
  getAIHistory,
  listAIHistories,
  prepareAIContext,
  runAIAssistant,
  saveAIHistory,
  type AIContextSource,
  type AIChatMode,
  type AIConversationMessage,
  type AgentEditProposal,
  type AgentEditTarget,
  type AIHistory,
  type AIHistorySource,
  type AIWebCitation,
  type SaveAIHistoryInput,
  type AssistantInput,
  type AssistantKind,
} from '../api/ai'
import { useAIStore } from './useAIStore'

export type AssistantState = 'idle' | 'loading-context' | 'generating' | 'canceling' | 'success' | 'error' | 'stale' | 'orphaned'

export type AIHistorySaveState = 'idle' | 'saving' | 'saved' | 'failed'

export type AssistantError = {
  code: string
  message: string
  retryAfterSeconds?: number
}

export type AssistantRequest = {
  providerID: AssistantInput['providerID']
  modelID: string
  kind: AssistantKind
  mode?: AIChatMode
  question: string
  noteIDs: string[]
  searchQuery: string
  includeBacklinks: boolean
  webSearch?: boolean
  messages: AIConversationMessage[]
  expectedSources?: AIHistorySource[]
  agentTarget?: AgentEditTarget
}

const safeMessages: Record<string, string> = {
  AI_PROVIDER_UNSUPPORTED: '選択した AI プロバイダーは利用できません。',
  AI_CONFIGURATION_UNAVAILABLE: 'AI 設定を確認してください。',
  AI_CREDENTIAL_UNAVAILABLE: 'AI 認証情報を利用できません。API Keyを再入力してください。',
  AI_MODEL_UNAVAILABLE: '選択したモデルは利用できません。モデルを再選択してください。',
  AI_MODEL_CAPABILITY_UNAVAILABLE: '選択したモデルは、このAI機能に必要な応答形式へ対応していません。別のモデルを選択してください。',
  AI_REAUTHENTICATION_REQUIRED: 'AI 認証情報の再入力が必要です。',
  AI_AUTH_FAILED: 'AI 認証に失敗しました。API Key を確認してください。',
  AI_PROVIDER_CONFIGURATION_REQUIRED: 'AI プロバイダーのプロジェクト設定、利用可能な地域、または無料枠の利用状況を確認してください。',
  AI_INPUT_INVALID: 'AIアシスタントへの入力が無効です。',
  AI_INPUT_TOO_LARGE: '送信する質問または参照資料が大きすぎます。対象を減らして再試行してください。',
  AI_OUTPUT_LIMIT: 'モデルの出力上限に達したため、応答を完了できませんでした。別のモデルで再試行してください。',
  AI_CONTENT_BLOCKED: '入力または応答が AI プロバイダーの安全基準によりブロックされました。内容を見直して再試行してください。',
  AI_RATE_LIMITED: 'AI プロバイダーの利用上限に達しました。時間をおいて再試行してください。',
  AI_TIMEOUT: 'AI プロバイダーが時間内に応答しませんでした。',
  AI_NETWORK_UNAVAILABLE: 'ネットワークに接続できません。',
  AI_PROVIDER_UNAVAILABLE: 'AI プロバイダーを現在利用できません。',
  AI_BUSY: '別のAI処理を実行中です。完了してから再試行してください。',
  AI_INVALID_RESPONSE: 'AI プロバイダーから有効な回答を受け取れませんでした。',
  AI_HISTORY_NOT_FOUND: 'AI履歴が見つかりません。',
  AI_HISTORY_SAVE_PENDING: '以前のAI履歴保存が完了していません。先に保存を再試行してください。',
  AI_CANCELLED: 'AI処理をキャンセルしました。',
  AI_CONTEXT_CHANGED: '確認後に参照ノートが更新されました。参照を確認してからもう一度送信してください。',
  AI_DRAFT_NOT_SAVED: '未保存の変更を保存できないため、AIへ送信しません。',
  AI_NOTE_UNAVAILABLE: 'このノートはAIの参照対象にできません。',
}

function createError(code: string, retryAfterSeconds?: number): AssistantError {
  const safeCode = safeMessages[code] ? code : 'AI_PROVIDER_UNAVAILABLE'
  return {
    code: safeCode,
    message: safeMessages[safeCode],
    ...(typeof retryAfterSeconds === 'number' && retryAfterSeconds > 0
      ? { retryAfterSeconds: Math.floor(retryAfterSeconds) }
      : {}),
  }
}

function errorFromUnknown(error: unknown): AssistantError {
  if (error && typeof error === 'object' && 'code' in error && typeof error.code === 'string') {
    const retryAfter = 'retryAfterSeconds' in error && typeof error.retryAfterSeconds === 'number'
      ? error.retryAfterSeconds
      : undefined
    return createError(error.code, retryAfter)
  }
  return createError('AI_PROVIDER_UNAVAILABLE')
}

function sourceRefs(sources: AIContextSource[]) {
  return sources.map((source) => ({ noteID: source.noteID, inputRevision: source.revision }))
}

function mergeConversationSources(
  existing: AIContextSource[],
  incoming: AIContextSource[],
) {
  const result = [...existing]
  const seen = new Set(existing.map((source) => source.noteID))
  for (const source of incoming) {
    if (seen.has(source.noteID)) continue
    seen.add(source.noteID)
    result.push(source)
  }
  return result
}

function sourceKey(input: Pick<AssistantRequest, 'kind' | 'mode' | 'question' | 'noteIDs' | 'searchQuery' | 'includeBacklinks' | 'webSearch' | 'agentTarget'>) {
  return JSON.stringify({
    kind: input.kind,
    mode: input.mode ?? 'ask',
    question: input.question,
    noteIDs: input.noteIDs,
    searchQuery: input.searchQuery,
    includeBacklinks: input.includeBacklinks,
    webSearch: input.webSearch ?? false,
    agentTarget: input.agentTarget ?? null,
  })
}

let fallbackRequestSequence = 0

function createAssistantRequestID() {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return `assistant-${globalThis.crypto.randomUUID()}`
  }
  fallbackRequestSequence += 1
  return `assistant-${Date.now().toString(36)}-${fallbackRequestSequence.toString(36)}`
}

export const useAIAssistantStore = defineStore('ai-assistant', () => {
  const aiStore = useAIStore()
  const state = ref<AssistantState>('idle')
  const error = ref<AssistantError | null>(null)
  const messages = ref<AIConversationMessage[]>([])
  const citations = ref<AIWebCitation[]>([])
  const proposal = ref<AgentEditProposal | null>(null)
  const webSearchRequests = ref(0)
  const sources = ref<AIContextSource[]>([])
  const contextSources = ref<AIContextSource[]>([])
  const histories = ref<AIHistory[]>([])
  const selectedHistoryID = ref<string | null>(null)
  const historySaveState = ref<AIHistorySaveState>('idle')
  const historySaveError = ref<AssistantError | null>(null)
  const activeRequest = ref<AssistantRequest | null>(null)
  const historySaveInFlight = ref(false)
  const isBusy = computed(() => (
    state.value === 'loading-context'
    || state.value === 'generating'
    || state.value === 'canceling'
    || historySaveState.value === 'saving'
    || historySaveInFlight.value
  ))
  let preparedContextKey = ''
  let latestContextRequest = 0
  let latestGenerationRequest = 0
  let activeBackendRequestID: string | null = null
  let cancelRequestedRequestID: string | null = null
  let clearedBackendRequestID: string | null = null
  let staleBackendRequestID: string | null = null
  let completedMessages: AIConversationMessage[] = []
  let conversationGeneration = 0
  let lastHistoryTitle = ''
  let historySavePromise: Promise<boolean> | null = null
  const pendingHistorySave = shallowRef<{ payload: SaveAIHistoryInput; generation: number } | null>(null)
  const pendingHistorySaveFailed = ref(false)
  const hasHistorySaveFailure = computed(() => (
    historySaveState.value === 'failed' || pendingHistorySaveFailed.value
  ))

  function clearError() {
    error.value = null
  }

  async function refreshHistories() {
    try {
      const response = await listAIHistories()
      if (response.error) {
        error.value = createError(response.error.code, response.error.retryAfterSeconds)
        return false
      }
      histories.value = response.items
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  async function previewContext(input: Pick<AssistantRequest, 'noteIDs' | 'searchQuery' | 'includeBacklinks' | 'kind' | 'question'>) {
    const requestID = ++latestContextRequest
    clearError()
    state.value = 'loading-context'
    try {
      const response = await prepareAIContext({
        noteIDs: input.noteIDs,
        searchQuery: input.searchQuery,
        includeBacklinks: input.includeBacklinks,
      })
      if (requestID !== latestContextRequest) return false
      if (response.error) {
        error.value = createError(response.error.code, response.error.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      contextSources.value = response.sources
      preparedContextKey = sourceKey(input)
      state.value = completedMessages.length > 0 ? 'success' : 'idle'
      return true
    } catch (cause) {
      if (requestID !== latestContextRequest) return false
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function ask(input: Omit<AssistantRequest, 'messages' | 'providerID' | 'modelID'> & { messages?: AIConversationMessage[] }) {
    if (isBusy.value) {
      error.value = createError('AI_BUSY')
      state.value = 'error'
      return false
    }
    if (pendingHistorySave.value) {
      const saveError = createError('AI_HISTORY_SAVE_PENDING')
      historySaveError.value = saveError
      historySaveState.value = 'failed'
      error.value = saveError
      state.value = 'error'
      return false
    }
    const setting = aiStore.configuredSetting
    if (!setting || !setting.modelID.trim()) {
      error.value = createError('AI_CONFIGURATION_UNAVAILABLE')
      state.value = 'error'
      return false
    }
    const request: AssistantRequest = {
      ...input,
      question: input.question.trim(),
      searchQuery: input.searchQuery.trim(),
      messages: input.messages ? [...input.messages] : [...completedMessages],
      providerID: setting.providerID,
      modelID: setting.modelID,
    }
    const key = sourceKey(request)
    const requestID = ++latestGenerationRequest
    if (preparedContextKey !== key) {
      if (!await previewContext(request)) return false
    }
    if (requestID !== latestGenerationRequest) return false
    clearError()
    historySaveState.value = 'idle'
    historySaveError.value = null
    state.value = 'generating'
    proposal.value = null
    const backendRequestID = createAssistantRequestID()
    activeBackendRequestID = backendRequestID
    cancelRequestedRequestID = null
    clearedBackendRequestID = null
    staleBackendRequestID = null
    messages.value = [
      ...request.messages,
      { role: 'user', content: request.question },
    ]
    try {
      const response = await runAIAssistant({
        requestID: backendRequestID,
        providerID: request.providerID,
        modelID: request.modelID,
        kind: request.kind,
        mode: request.mode ?? 'ask',
        question: request.question,
        messages: request.messages,
        noteIDs: request.noteIDs,
        searchQuery: request.searchQuery,
        includeBacklinks: request.includeBacklinks,
        webSearch: request.webSearch ?? false,
        expectedSources: sourceRefs(contextSources.value),
        ...(request.agentTarget ? { agentTarget: request.agentTarget } : {}),
      })
      if (requestID !== latestGenerationRequest) return false
      if (response.error || !response.result) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      if (staleBackendRequestID === backendRequestID) {
        clearError()
        proposal.value = null
        state.value = 'stale'
        return false
      }
      clearError()
      messages.value = response.result.messages
      citations.value = response.result.citations ?? []
      proposal.value = response.result.proposal ?? null
      webSearchRequests.value = Math.max(0, response.result.webSearchRequests ?? 0)
      completedMessages = [...response.result.messages]
      sources.value = mergeConversationSources(sources.value, response.result.sources)
      contextSources.value = response.result.sources
      activeRequest.value = request
      state.value = 'success'
      return true
    } catch (cause) {
      if (requestID !== latestGenerationRequest) return false
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    } finally {
      if (activeBackendRequestID === backendRequestID) {
        const wasCleared = clearedBackendRequestID === backendRequestID
        activeBackendRequestID = null
        cancelRequestedRequestID = null
        clearedBackendRequestID = null
        staleBackendRequestID = null
        if (wasCleared) state.value = 'idle'
      }
    }
  }

  async function cancel() {
    if (state.value === 'loading-context') {
      latestContextRequest += 1
      latestGenerationRequest += 1
      preparedContextKey = ''
      error.value = createError('AI_CANCELLED')
      state.value = 'error'
      return true
    }
    if (state.value === 'canceling') return false
    if (state.value !== 'generating' || !activeBackendRequestID) return false

    const requestID = activeBackendRequestID
    cancelRequestedRequestID = requestID
    clearError()
    state.value = 'canceling'
    try {
      const response = await cancelAIAssistant(requestID)
      if (clearedBackendRequestID === requestID) {
        return response.canceled && !response.error
      }
      if (activeBackendRequestID !== requestID || state.value !== 'canceling') {
        return response.canceled && !response.error
      }
      if (response.error || !response.canceled) {
        cancelRequestedRequestID = null
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'generating'
        return false
      }
      return true
    } catch (cause) {
      if (clearedBackendRequestID === requestID) return false
      if (activeBackendRequestID !== requestID || state.value !== 'canceling') return false
      cancelRequestedRequestID = null
      error.value = errorFromUnknown(cause)
      state.value = 'generating'
      return false
    }
  }

  function defaultHistoryTitle() {
    const firstQuestion = messages.value.find((message) => message.role === 'user')?.content.trim()
    const firstLine = firstQuestion?.split(/\r?\n/, 1)[0]?.trim()
    if (!firstLine) return `AI会話 ${new Date().toLocaleString('ja-JP')}`
    return firstLine.length > 80 ? `${firstLine.slice(0, 80)}…` : firstLine
  }

  function recordHistorySaveFailure(saveError: AssistantError, saveGeneration: number, showError: boolean) {
    if (pendingHistorySave.value) pendingHistorySaveFailed.value = true
    historySaveError.value = saveError
    if (saveGeneration !== conversationGeneration) return
    historySaveState.value = 'failed'
    if (showError) error.value = saveError
  }

  async function persistHistory(
    title: string,
    showError: boolean,
    retained: { payload: SaveAIHistoryInput; generation: number } | null = null,
  ) {
    const request = activeRequest.value
    const setting = aiStore.configuredSetting
    const normalizedTitle = title.trim() || defaultHistoryTitle()
    const saveGeneration = retained?.generation ?? conversationGeneration
    const payload = retained?.payload ?? (
      request && setting && state.value === 'success' && messages.value.length >= 2 && messages.value.length % 2 === 0
        ? {
            kind: request.kind,
            title: normalizedTitle,
            providerID: request.providerID,
            modelID: request.modelID,
            messages: [...messages.value],
            sources: sourceRefs(sources.value),
            ...(selectedHistoryID.value ? { id: selectedHistoryID.value } : {}),
          }
        : null
    )

    if (!payload) {
      const saveError = createError('AI_INPUT_INVALID')
      recordHistorySaveFailure(saveError, saveGeneration, showError)
      return false
    }
    if (!retained && pendingHistorySave.value) {
      const saveError = createError('AI_HISTORY_SAVE_PENDING')
      recordHistorySaveFailure(saveError, conversationGeneration, showError)
      return false
    }

    const pending = retained ?? { payload, generation: saveGeneration }
    pendingHistorySave.value = pending
    pendingHistorySaveFailed.value = false
    if (saveGeneration === conversationGeneration) {
      lastHistoryTitle = payload.title
      historySaveState.value = 'saving'
      historySaveError.value = null
    }

    try {
      const response = await saveAIHistory(payload)
      if (response.error || !response.history) {
        const saveError = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        recordHistorySaveFailure(saveError, saveGeneration, showError)
        return false
      }
      if (pendingHistorySave.value?.payload === payload) {
        pendingHistorySave.value = null
        pendingHistorySaveFailed.value = false
      }
      if (saveGeneration !== conversationGeneration) return Boolean(retained)
      selectedHistoryID.value = response.history.id
      historySaveState.value = 'saved'
      await refreshHistories()
      return true
    } catch (cause) {
      const saveError = errorFromUnknown(cause)
      recordHistorySaveFailure(saveError, saveGeneration, showError)
      return false
    }
  }

  function startHistorySave(
    title: string,
    showError: boolean,
    retained: { payload: SaveAIHistoryInput; generation: number } | null = null,
  ) {
    if (historySavePromise) return historySavePromise
    historySaveInFlight.value = true
    const pending = persistHistory(title, showError, retained)
    historySavePromise = pending
    const finish = () => {
      if (historySavePromise !== pending) return
      historySavePromise = null
      historySaveInFlight.value = false
    }
    void pending.then(finish, finish)
    return pending
  }

  async function saveCompletedConversation(
    title = defaultHistoryTitle(),
    retained: { payload: SaveAIHistoryInput; generation: number } | null = null,
  ) {
    return startHistorySave(title, false, retained)
  }

  async function retryHistorySave() {
    if (historySavePromise) return historySavePromise
    if (pendingHistorySave.value) {
      return startHistorySave(pendingHistorySave.value.payload.title, true, pendingHistorySave.value)
    }
    if (historySaveState.value !== 'failed') return false
    return startHistorySave(lastHistoryTitle || defaultHistoryTitle(), true)
  }

  async function save(title: string) {
    if (historySavePromise) return historySavePromise
    if (pendingHistorySave.value) {
      const saveError = createError('AI_HISTORY_SAVE_PENDING')
      recordHistorySaveFailure(saveError, conversationGeneration, true)
      return false
    }
    return startHistorySave(title, true)
  }

  async function loadHistory(id: string) {
    if (isBusy.value) {
      error.value = createError('AI_BUSY')
      return false
    }
    const loadGeneration = ++conversationGeneration
    latestContextRequest += 1
    latestGenerationRequest += 1
    clearError()
    try {
      const response = await getAIHistory(id)
      if (response.error || !response.history) {
        error.value = createError(response.error?.code ?? 'AI_HISTORY_NOT_FOUND', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      if (loadGeneration !== conversationGeneration) return false
      const history = response.history
      messages.value = history.messages ?? []
      citations.value = []
      proposal.value = null
      webSearchRequests.value = 0
      completedMessages = [...messages.value]
      sources.value = history.sources.map((source) => ({
        noteID: source.noteID,
        title: source.noteID,
        revision: source.inputRevision,
        contentByte: 0,
      }))
      contextSources.value = sources.value
      selectedHistoryID.value = history.id
      historySaveState.value = 'saved'
      historySaveError.value = null
      lastHistoryTitle = history.title
      activeRequest.value = {
        providerID: history.providerID,
        modelID: history.modelID,
        kind: history.kind,
        mode: 'ask',
        question: [...(history.messages ?? [])].reverse().find((message) => message.role === 'user')?.content ?? '',
        noteIDs: history.sources.map((source) => source.noteID),
        searchQuery: '',
        includeBacklinks: false,
        webSearch: false,
        messages: [...messages.value],
        expectedSources: history.sources,
      }
      state.value = history.status === 'saved'
        ? 'success'
        : history.status === 'stale'
          ? 'stale'
          : 'orphaned'
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function removeHistory(id: string) {
    if (isBusy.value) {
      error.value = createError('AI_BUSY')
      return false
    }
    try {
      const response = await deleteAIHistory(id)
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_HISTORY_NOT_FOUND', response.error?.retryAfterSeconds)
        return false
      }
      histories.value = histories.value.filter((history) => history.id !== id)
      if (selectedHistoryID.value === id) clearConversation({ discardPendingHistory: true })
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  async function removeAllHistories() {
    if (isBusy.value) {
      error.value = createError('AI_BUSY')
      return false
    }
    try {
      const response = await deleteAllAIHistories()
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        return false
      }
      histories.value = []
      clearConversation({ discardPendingHistory: true })
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  function clearConversation(options: { discardPendingHistory?: boolean } = {}) {
    conversationGeneration += 1
    const requestID = activeBackendRequestID
    if (requestID) {
      clearedBackendRequestID = requestID
      cancelRequestedRequestID = requestID
      void cancelAIAssistant(requestID).catch(() => undefined)
    }
    latestContextRequest += 1
    latestGenerationRequest += 1
    state.value = requestID ? 'canceling' : 'idle'
    error.value = null
    messages.value = []
    citations.value = []
    proposal.value = null
    webSearchRequests.value = 0
    sources.value = []
    contextSources.value = []
    activeRequest.value = null
    selectedHistoryID.value = null
    historySaveState.value = 'idle'
    if (options.discardPendingHistory) {
      pendingHistorySave.value = null
      pendingHistorySaveFailed.value = false
      historySaveError.value = null
    } else if (!pendingHistorySaveFailed.value) {
      historySaveError.value = null
    }
    lastHistoryTitle = pendingHistorySave.value?.payload.title ?? ''
    preparedContextKey = ''
    completedMessages = []
  }

  function discardConversation() {
    clearConversation({ discardPendingHistory: true })
  }

  function markStaleForRevision(noteID: string, revision: number) {
    const source = sources.value.find((item) => item.noteID === noteID)
      ?? contextSources.value.find((item) => item.noteID === noteID)
    if (!source || source.revision === revision || messages.value.length === 0) return
    preparedContextKey = ''
    if (activeBackendRequestID) {
      staleBackendRequestID = activeBackendRequestID
      return
    }
    state.value = 'stale'
  }

  function setPreconditionError(code: 'AI_DRAFT_NOT_SAVED' | 'AI_NOTE_UNAVAILABLE') {
    error.value = createError(code)
    state.value = 'error'
  }

  return {
    state,
    error,
    messages,
    citations,
    proposal,
    webSearchRequests,
    sources,
    contextSources,
    histories,
    selectedHistoryID,
    historySaveState,
    historySaveError,
    hasHistorySaveFailure,
    isBusy,
    refreshHistories,
    previewContext,
    ask,
    cancel,
    save,
    saveCompletedConversation,
    retryHistorySave,
    loadHistory,
    removeHistory,
    removeAllHistories,
    clearConversation,
    discardConversation,
    markStaleForRevision,
    setPreconditionError,
  }
})
