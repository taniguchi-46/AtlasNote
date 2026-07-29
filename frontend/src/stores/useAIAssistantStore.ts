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
  type AIConversationMessage,
  type AIHistory,
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
  question: string
  noteIDs: string[]
  searchQuery: string
  includeBacklinks: boolean
  messages: AIConversationMessage[]
}

const safeMessages: Record<string, string> = {
  AI_PROVIDER_UNSUPPORTED: '選択した AI プロバイダーは利用できません。',
  AI_CONFIGURATION_UNAVAILABLE: 'AI 設定を確認してください。',
  AI_CREDENTIAL_UNAVAILABLE: 'AI 認証情報を利用できません。API Keyを再入力してください。',
  AI_MODEL_UNAVAILABLE: '選択したモデルは利用できません。モデルを再選択してください。',
  AI_REAUTHENTICATION_REQUIRED: 'AI 認証情報の再入力が必要です。',
  AI_AUTH_FAILED: 'AI 認証に失敗しました。API Key を確認してください。',
  AI_INPUT_INVALID: 'AIアシスタントへの入力が無効です。',
  AI_INPUT_TOO_LARGE: '送信する質問または参照資料が大きすぎます。対象を減らして再試行してください。',
  AI_RATE_LIMITED: 'AI プロバイダーの利用上限に達しました。時間をおいて再試行してください。',
  AI_TIMEOUT: 'AI プロバイダーが時間内に応答しませんでした。',
  AI_NETWORK_UNAVAILABLE: 'ネットワークに接続できません。',
  AI_PROVIDER_UNAVAILABLE: 'AI プロバイダーを現在利用できません。',
  AI_BUSY: '別のAI処理を実行中です。完了してから再試行してください。',
  AI_INVALID_RESPONSE: 'AI プロバイダーから有効な回答を受け取れませんでした。',
  AI_HISTORY_NOT_FOUND: 'AI履歴が見つかりません。',
  AI_CANCELLED: 'AI処理をキャンセルしました。',
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

function sourceKey(input: Pick<AssistantRequest, 'kind' | 'question' | 'noteIDs' | 'searchQuery' | 'includeBacklinks'>) {
  return JSON.stringify({
    kind: input.kind,
    question: input.question,
    noteIDs: input.noteIDs,
    searchQuery: input.searchQuery,
    includeBacklinks: input.includeBacklinks,
  })
}

export const useAIAssistantStore = defineStore('ai-assistant', () => {
  const aiStore = useAIStore()
  const state = ref<AssistantState>('idle')
  const error = ref<AssistantError | null>(null)
  const messages = ref<AIConversationMessage[]>([])
  const sources = ref<AIContextSource[]>([])
  const contextSources = ref<AIContextSource[]>([])
  const histories = ref<AIHistory[]>([])
  const selectedHistoryID = ref<string | null>(null)
  const activeRequest = ref<AssistantRequest | null>(null)
  const isBusy = computed(() => state.value === 'loading-context' || state.value === 'generating')
  let preparedContextKey = ''

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
    clearError()
    state.value = 'loading-context'
    try {
      const response = await prepareAIContext({
        noteIDs: input.noteIDs,
        searchQuery: input.searchQuery,
        includeBacklinks: input.includeBacklinks,
      })
      if (response.error) {
        error.value = createError(response.error.code, response.error.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      contextSources.value = response.sources
      preparedContextKey = sourceKey(input)
      state.value = messages.value.length > 0 ? 'success' : 'idle'
      return true
    } catch (cause) {
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
      messages: input.messages ? [...input.messages] : [...messages.value],
      providerID: setting.providerID,
      modelID: setting.modelID,
    }
    const key = sourceKey(request)
    if (preparedContextKey !== key) {
      if (!await previewContext(request)) return false
    }
    clearError()
    state.value = 'generating'
    activeRequest.value = request
    try {
      const response = await runAIAssistant({
        providerID: request.providerID,
        modelID: request.modelID,
        kind: request.kind,
        question: request.question,
        messages: request.messages,
        noteIDs: request.noteIDs,
        searchQuery: request.searchQuery,
        includeBacklinks: request.includeBacklinks,
      })
      if (response.error || !response.result) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      messages.value = response.result.messages
      sources.value = response.result.sources
      contextSources.value = response.result.sources
      selectedHistoryID.value = null
      state.value = 'success'
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function save(title: string) {
    const request = activeRequest.value
    const setting = aiStore.configuredSetting
    if (!request || !setting || (state.value === 'stale' || state.value === 'orphaned') || messages.value.length < 2) {
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
        question: [...(history.messages ?? [])].reverse().find((message) => message.role === 'user')?.content ?? '',
        noteIDs: history.sources.map((source) => source.noteID),
        searchQuery: '',
        includeBacklinks: false,
        messages: [...messages.value],
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
    state.value = 'idle'
    error.value = null
    messages.value = []
    sources.value = []
    contextSources.value = []
    activeRequest.value = null
    selectedHistoryID.value = null
    preparedContextKey = ''
  }

  function markStaleForRevision(noteID: string, revision: number) {
    const source = sources.value.find((item) => item.noteID === noteID)
    if (!source || source.revision === revision || messages.value.length === 0) return
    state.value = 'stale'
  }

  return {
    state,
    error,
    messages,
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
  }
})
