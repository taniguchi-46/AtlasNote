import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  deleteAIArtifact,
  deleteAllAIArtifacts,
  getAIArtifact,
  listAIArtifacts,
  prepareAIContext,
  runAIWriting,
  saveAIArtifact,
  type AIArtifact,
  type AIContextSource,
  type WritingKind,
  type WritingInput,
} from '../api/ai'
import { useAIStore } from './useAIStore'

export type WritingState = 'idle' | 'loading-context' | 'generating' | 'success' | 'error' | 'stale' | 'orphaned'

export type WritingError = {
  code: string
  message: string
  retryAfterSeconds?: number
}

const safeMessages: Record<string, string> = {
  AI_CONFIGURATION_UNAVAILABLE: 'AI 設定を確認してください。',
  AI_CREDENTIAL_UNAVAILABLE: 'AI 認証情報を利用できません。API Keyを再入力してください。',
  AI_REAUTHENTICATION_REQUIRED: 'AI 認証情報の再入力が必要です。',
  AI_MODEL_UNAVAILABLE: '選択したモデルは利用できません。モデルを再選択してください。',
  AI_INPUT_INVALID: 'AIライティングへの入力が無効です。',
  AI_INPUT_TOO_LARGE: '送信する目的または参照資料が大きすぎます。対象を減らして再試行してください。',
  AI_PROVIDER_UNAVAILABLE: 'AI プロバイダーを現在利用できません。',
  AI_BUSY: '別のAI処理を実行中です。完了してから再試行してください。',
  AI_INVALID_RESPONSE: 'AI プロバイダーから有効な文章を受け取れませんでした。',
  AI_TIMEOUT: 'AI プロバイダーが時間内に応答しませんでした。',
  AI_NETWORK_UNAVAILABLE: 'ネットワークに接続できません。',
  AI_RATE_LIMITED: 'AI プロバイダーの利用上限に達しました。時間をおいて再試行してください。',
  AI_ARTIFACT_NOT_FOUND: 'AI成果物が見つかりません。',
}

function createError(code: string, retryAfterSeconds?: number): WritingError {
  const safeCode = safeMessages[code] ? code : 'AI_PROVIDER_UNAVAILABLE'
  return {
    code: safeCode,
    message: safeMessages[safeCode],
    ...(typeof retryAfterSeconds === 'number' && retryAfterSeconds > 0
      ? { retryAfterSeconds: Math.floor(retryAfterSeconds) }
      : {}),
  }
}

function errorFromUnknown(error: unknown): WritingError {
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

function contextKey(input: Pick<WritingInput, 'noteIDs' | 'searchQuery' | 'includeBacklinks'>) {
  return JSON.stringify({
    noteIDs: input.noteIDs,
    searchQuery: input.searchQuery,
    includeBacklinks: input.includeBacklinks,
  })
}

export const useAIWritingStore = defineStore('ai-writing', () => {
  const aiStore = useAIStore()
  const state = ref<WritingState>('idle')
  const error = ref<WritingError | null>(null)
  const content = ref('')
  const sources = ref<AIContextSource[]>([])
  const contextSources = ref<AIContextSource[]>([])
  const artifacts = ref<AIArtifact[]>([])
  const selectedArtifactID = ref<string | null>(null)
  const activeRequest = ref<WritingInput | null>(null)
  const isBusy = computed(() => state.value === 'loading-context' || state.value === 'generating')
  let preparedContextKey = ''

  async function refreshArtifacts() {
    try {
      const response = await listAIArtifacts()
      if (response.error) {
        error.value = createError(response.error.code, response.error.retryAfterSeconds)
        return false
      }
      artifacts.value = response.items
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  async function previewContext(input: Pick<WritingInput, 'noteIDs' | 'searchQuery' | 'includeBacklinks'>) {
    state.value = 'loading-context'
    error.value = null
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
      preparedContextKey = contextKey({
        noteIDs: input.noteIDs,
        searchQuery: input.searchQuery,
        includeBacklinks: input.includeBacklinks,
      })
      state.value = content.value ? 'success' : 'idle'
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function generate(input: WritingInput) {
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
    const normalized: WritingInput = {
      ...input,
      instruction: input.instruction.trim(),
      searchQuery: input.searchQuery?.trim() ?? '',
      noteIDs: input.noteIDs ?? [],
      includeBacklinks: input.includeBacklinks ?? false,
      providerID: setting.providerID,
      modelID: setting.modelID,
    }
    const preparedKey = contextKey({
      noteIDs: normalized.noteIDs,
      searchQuery: normalized.searchQuery,
      includeBacklinks: normalized.includeBacklinks,
    })
    if (preparedContextKey !== preparedKey && !await previewContext(normalized)) return false
    error.value = null
    state.value = 'generating'
    activeRequest.value = normalized
    selectedArtifactID.value = null
    try {
      const response = await runAIWriting(normalized)
      if (response.error || !response.result) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      content.value = response.result.content
      sources.value = response.result.sources
      contextSources.value = response.result.sources
      state.value = 'success'
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  function updateContent(value: string) {
    content.value = value
  }

  async function save(title: string) {
    const request = activeRequest.value
    if (!request || (state.value === 'stale' || state.value === 'orphaned') || content.value.trim() === '') {
      error.value = createError('AI_INPUT_INVALID')
      state.value = 'error'
      return false
    }
    try {
      const response = await saveAIArtifact({
        kind: request.kind,
        title: title.trim(),
        providerID: request.providerID,
        modelID: request.modelID,
        content: content.value,
        sources: sourceRefs(sources.value),
        ...(selectedArtifactID.value ? { id: selectedArtifactID.value } : {}),
      })
      if (response.error || !response.artifact) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      selectedArtifactID.value = response.artifact.id
      await refreshArtifacts()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function loadArtifact(id: string) {
    try {
      const response = await getAIArtifact(id)
      if (response.error || !response.artifact) {
        error.value = createError(response.error?.code ?? 'AI_ARTIFACT_NOT_FOUND', response.error?.retryAfterSeconds)
        state.value = 'error'
        return false
      }
      const artifact = response.artifact
      content.value = artifact.content
      sources.value = artifact.sources.map((source) => ({
        noteID: source.noteID,
        title: source.noteID,
        revision: source.inputRevision,
        contentByte: 0,
      }))
      contextSources.value = sources.value
      activeRequest.value = {
        providerID: artifact.providerID,
        modelID: artifact.modelID,
        kind: artifact.kind,
        instruction: '',
        noteIDs: artifact.sources.map((source) => source.noteID),
        searchQuery: '',
        includeBacklinks: false,
      }
      selectedArtifactID.value = artifact.id
      state.value = artifact.status === 'saved'
        ? 'success'
        : artifact.status === 'stale'
          ? 'stale'
          : 'orphaned'
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      return false
    }
  }

  async function removeArtifact(id: string) {
    try {
      const response = await deleteAIArtifact(id)
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_ARTIFACT_NOT_FOUND', response.error?.retryAfterSeconds)
        return false
      }
      artifacts.value = artifacts.value.filter((artifact) => artifact.id !== id)
      if (selectedArtifactID.value === id) clear()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  async function removeAllArtifacts() {
    try {
      const response = await deleteAllAIArtifacts()
      if (response.error || !response.deleted) {
        error.value = createError(response.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', response.error?.retryAfterSeconds)
        return false
      }
      artifacts.value = []
      clear()
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      return false
    }
  }

  function clear() {
    state.value = 'idle'
    error.value = null
    content.value = ''
    sources.value = []
    contextSources.value = []
    activeRequest.value = null
    selectedArtifactID.value = null
    preparedContextKey = ''
  }

  function markStaleForRevision(noteID: string, revision: number) {
    const source = sources.value.find((item) => item.noteID === noteID)
    if (!source || source.revision === revision || content.value === '') return
    state.value = 'stale'
  }

  return {
    state,
    error,
    content,
    sources,
    contextSources,
    artifacts,
    selectedArtifactID,
    isBusy,
    refreshArtifacts,
    previewContext,
    generate,
    updateContent,
    save,
    loadArtifact,
    removeArtifact,
    removeAllArtifacts,
    clear,
    markStaleForRevision,
  }
})
