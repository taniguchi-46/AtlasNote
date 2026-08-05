import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
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
  type AssistantInput,
  type AssistantKind,
} from '../api/ai'
import { useAIStore } from './useAIStore'

export type AssistantState = 'idle' | 'loading-context' | 'generating' | 'success' | 'error' | 'stale' | 'orphaned'

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
  const activeRequest = ref<AssistantRequest | null>(null)
  const isBusy = computed(() => state.value === 'loading-context' || state.value === 'generating')
  let preparedContextKey = ''
  let latestContextRequest = 0
  let latestGenerationRequest = 0
  let completedMessages: AIConversationMessage[] = []

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
    state.value = 'generating'
    proposal.value = null
    messages.value = [
      ...request.messages,
      { role: 'user', content: request.question },
    ]
    try {
      const response = await runAIAssistant({
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
      messages.value = response.result.messages
      citations.value = response.result.citations ?? []
      proposal.value = response.result.proposal ?? null
      webSearchRequests.value = Math.max(0, response.result.webSearchRequests ?? 0)
      completedMessages = [...response.result.messages]
      sources.value = mergeConversationSources(sources.value, response.result.sources)
      contextSources.value = response.result.sources
      selectedHistoryID.value = null
      activeRequest.value = request
      state.value = 'success'
      return true
    } catch (cause) {
      if (requestID !== latestGenerationRequest) return false
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function save(title: string) {
    const request = activeRequest.value
    const setting = aiStore.configuredSetting
    if (!request || !setting || state.value !== 'success' || messages.value.length < 2) {
      error.value = createError('AI_INPUT_INVALID')
      state.value = 'error'
      return false
    }
    try {
      const response = await saveAIHistory({
        kind: request.kind,
        title: title.trim(),
        providerID: request.providerID,
        modelID: request.modelID,
        messages: messages.value,
        sources: sourceRefs(sources.value),
        ...(selectedHistoryID.value ? { id: selectedHistoryID.value } : {}),
      })
      if (response.error || !response.history) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      selectedHistoryID.value = response.history.id
      await refreshHistories()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function loadHistory(id: string) {
    if (isBusy.value) {
      error.value = createError('AI_BUSY')
      return false
    }
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
    try {
      const response = await deleteAIHistory(id)
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_HISTORY_NOT_FOUND', response.error?.retryAfterSeconds)
        return false
      }
      histories.value = histories.value.filter((history) => history.id !== id)
      if (selectedHistoryID.value === id) clearConversation()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  async function removeAllHistories() {
    try {
      const response = await deleteAllAIHistories()
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        return false
      }
      histories.value = []
      clearConversation()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  function clearConversation() {
    latestContextRequest += 1
    latestGenerationRequest += 1
    state.value = 'idle'
    error.value = null
    messages.value = []
    citations.value = []
    proposal.value = null
    webSearchRequests.value = 0
    sources.value = []
    contextSources.value = []
    activeRequest.value = null
    selectedHistoryID.value = null
    preparedContextKey = ''
    completedMessages = []
  }

  function markStaleForRevision(noteID: string, revision: number) {
    const source = sources.value.find((item) => item.noteID === noteID)
    if (!source || source.revision === revision || messages.value.length === 0) return
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
    isBusy,
    refreshHistories,
    previewContext,
    ask,
    save,
    loadHistory,
    removeHistory,
    removeAllHistories,
    clearConversation,
    markStaleForRevision,
    setPreconditionError,
  }
})
