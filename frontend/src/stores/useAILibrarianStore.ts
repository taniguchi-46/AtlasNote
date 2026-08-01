import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  cancelAILibrarian,
  onAILibrarianUpdate,
  startAILibrarian,
  type AIProviderID,
  type LibrarianCandidate,
  type LibrarianCandidateContext,
  type LibrarianEvent,
  type LibrarianInput,
  type LibrarianNotebookContext,
  type LibrarianOperation,
  type LibrarianResult,
  type LibrarianTagContext,
} from '../api/ai'
import { useAIStore } from './useAIStore'

export type LibrarianState =
  | 'idle'
  | 'generating'
  | 'partial'
  | 'canceling'
  | 'success'
  | 'empty'
  | 'error'
  | 'canceled'
  | 'stale'

export type LibrarianError = {
  code: string
  message: string
  retryAfterSeconds?: number
}

export type LibrarianStartSource = {
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

const safeMessages: Record<string, string> = {
  AI_PROVIDER_UNSUPPORTED: '選択した AI プロバイダーは利用できません。',
  AI_CONFIGURATION_UNAVAILABLE: 'AI 設定を確認してください。',
  AI_CREDENTIAL_UNAVAILABLE: 'AI 認証情報を利用できません。API Key を再入力してください。',
  AI_REAUTHENTICATION_REQUIRED: 'AI 認証情報の再入力が必要です。',
  AI_AUTH_FAILED: 'AI 認証に失敗しました。API Key を確認してください。',
  AI_PROVIDER_CONFIGURATION_REQUIRED: 'AI プロバイダーのプロジェクト設定、利用可能な地域、または無料枠の利用状況を確認してください。',
  AI_MODEL_UNAVAILABLE: '選択したモデルは利用できません。',
  AI_MODEL_CAPABILITY_UNAVAILABLE: '選択したモデルは、AI司書に必要な構造化応答へ対応していません。別のモデルを選択してください。',
  AI_OUTPUT_LIMIT: 'モデルの出力上限に達したため、候補を完了できませんでした。別のモデルで再試行してください。',
  AI_CONTENT_BLOCKED: '入力または応答が AI プロバイダーの安全基準によりブロックされました。内容を見直して再試行してください。',
  AI_INPUT_TOO_LARGE: '送信するノートまたは候補情報が上限を超えています。候補数を減らして再試行してください。',
  AI_INPUT_INVALID: 'AI司書への入力が無効です。',
  AI_RATE_LIMITED: 'AI プロバイダーの利用上限に達しました。時間をおいて再試行してください。',
  AI_TIMEOUT: 'AI プロバイダーが時間内に応答しませんでした。',
  AI_NETWORK_UNAVAILABLE: 'ネットワークに接続できません。',
  AI_PROVIDER_UNAVAILABLE: 'AI プロバイダーを現在利用できません。',
  AI_BUSY: '別のAI処理を実行中です。完了してから再試行してください。',
  AI_INVALID_RESPONSE: 'AI プロバイダーから有効な候補を受け取れませんでした。',
  AI_CANCELLED: 'AI司書の生成をキャンセルしました。',
  AI_NOTE_UNAVAILABLE: 'このノートはAI司書の対象にできません。',
  AI_DRAFT_NOT_SAVED: '未保存の変更を保存できないため、AI司書を開始しません。',
  AI_REVISION_CONFLICT: 'ノートが更新されたため、候補を適用できません。',
  AI_CONTEXT_CHANGED: '確認後にノートが更新されました。もう一度候補を生成してください。',
  TAG_STATE_CONFLICT: 'タグが更新されたため、候補を適用できません。',
}

function createError(code: string, retryAfterSeconds?: number): LibrarianError {
  const safeCode = safeMessages[code] ? code : 'AI_PROVIDER_UNAVAILABLE'
  return {
    code: safeCode,
    message: safeMessages[safeCode],
    ...(typeof retryAfterSeconds === 'number' && retryAfterSeconds > 0
      ? { retryAfterSeconds: Math.floor(retryAfterSeconds) }
      : {}),
  }
}

function errorFromUnknown(error: unknown): LibrarianError {
  if (error && typeof error === 'object' && 'code' in error && typeof error.code === 'string') {
    const retryAfter = 'retryAfterSeconds' in error && typeof error.retryAfterSeconds === 'number'
      ? error.retryAfterSeconds
      : undefined
    return createError(error.code, retryAfter)
  }
  const message = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
  const code = Object.keys(safeMessages).find((candidate) => message.includes(candidate))
  return createError(code ?? 'AI_PROVIDER_UNAVAILABLE')
}

function candidateKey(candidate: LibrarianCandidate) {
  return candidate.noteID ?? candidate.notebookID ?? candidate.name ?? candidate.value ?? ''
}

export const useAILibrarianStore = defineStore('ai-librarian', () => {
  const aiStore = useAIStore()
  const state = ref<LibrarianState>('idle')
  const targetNoteID = ref<string | null>(null)
  const baseRevision = ref<number | null>(null)
  const operation = ref<LibrarianOperation | null>(null)
  const partialText = ref('')
  const result = ref<LibrarianResult | null>(null)
  const error = ref<LibrarianError | null>(null)
  const stale = ref(false)
  const isGenerating = computed(() => (
    state.value === 'generating' || state.value === 'partial' || state.value === 'canceling'
  ))

  let activeRequestID: string | null = null
  let unsubscribe: (() => void) | null = null
  let pendingEvents: LibrarianEvent[] = []

  function stopListening() {
    unsubscribe?.()
    unsubscribe = null
  }

  function clear() {
    stopListening()
    activeRequestID = null
    pendingEvents = []
    stale.value = false
    targetNoteID.value = null
    baseRevision.value = null
    operation.value = null
    partialText.value = ''
    result.value = null
    error.value = null
    state.value = 'idle'
  }

  function handleEvent(event: LibrarianEvent) {
    if (stale.value) return
    if (event.noteID !== targetNoteID.value || event.baseRevision !== baseRevision.value) return
    if (event.operation !== operation.value) return
    if (!activeRequestID) {
      pendingEvents.push(event)
      return
    }
    if (event.requestID !== activeRequestID) return

    if (event.phase === 'partial') {
      partialText.value += event.partialText ?? ''
      state.value = 'partial'
      return
    }
    if (event.phase === 'completed') {
      result.value = event.result ?? { operation: event.operation, quality: 'empty', candidates: [] }
      state.value = result.value.quality === 'empty' ? 'empty' : 'success'
      stopListening()
      activeRequestID = null
      return
    }
    if (event.phase === 'canceled') {
      error.value = createError('AI_CANCELLED')
      state.value = 'canceled'
      stopListening()
      activeRequestID = null
      return
    }

    error.value = createError(event.error?.code ?? 'AI_PROVIDER_UNAVAILABLE', event.error?.retryAfterSeconds)
    state.value = 'error'
    stopListening()
    activeRequestID = null
  }

  async function start(source: LibrarianStartSource) {
    if (isGenerating.value) {
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
    if (!aiStore.isLibrarianReady) {
      error.value = createError('AI_MODEL_CAPABILITY_UNAVAILABLE')
      state.value = 'error'
      return false
    }
    clear()
    targetNoteID.value = source.noteID
    baseRevision.value = source.baseRevision
    operation.value = source.operation
    state.value = 'generating'
    unsubscribe = onAILibrarianUpdate(handleEvent)

    const input: LibrarianInput = {
      providerID: setting.providerID as AIProviderID,
      modelID: setting.modelID,
      operation: source.operation,
      noteID: source.noteID,
      baseRevision: source.baseRevision,
      title: source.title,
      content: source.content,
      candidateCount: source.candidateCount,
      ...(source.candidates ? { candidates: source.candidates } : {}),
      ...(source.existingTags ? { existingTags: source.existingTags } : {}),
      ...(source.notebooks ? { notebooks: source.notebooks } : {}),
    }

    try {
      const response = await startAILibrarian(input)
      if (response.error) {
        error.value = createError(response.error.code, response.error.retryAfterSeconds)
        state.value = 'error'
        pendingEvents = []
        stopListening()
        return false
      }
      if (!response.requestID) {
        error.value = createError('AI_PROVIDER_UNAVAILABLE')
        state.value = 'error'
        pendingEvents = []
        stopListening()
        return false
      }
      activeRequestID = response.requestID
      const queuedEvents = pendingEvents
      pendingEvents = []
      queuedEvents.forEach(handleEvent)
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      pendingEvents = []
      stopListening()
      return false
    }
  }

  async function cancel() {
    if (!activeRequestID) return false
    state.value = 'canceling'
    try {
      await cancelAILibrarian(activeRequestID)
      return true
    } catch (cause) {
      error.value = errorFromUnknown(cause)
      state.value = 'error'
      stopListening()
      activeRequestID = null
      return false
    }
  }

  function discard() {
    const requestID = activeRequestID
    activeRequestID = null
    stopListening()
    if (requestID) void cancelAILibrarian(requestID).catch(() => {})
    clear()
  }

  function discardForNote(noteID: string | null) {
    if (targetNoteID.value && targetNoteID.value !== noteID) discard()
    if (result.value && result.value.operation && targetNoteID.value !== noteID) clear()
  }

  function markStaleForRevision(noteID: string, revision: number) {
    if (targetNoteID.value !== noteID || baseRevision.value === null || baseRevision.value === revision) return
    if (!result.value && !isGenerating.value) return
    const requestID = activeRequestID
    activeRequestID = null
    pendingEvents = []
    stopListening()
    stale.value = true
    state.value = 'stale'
    if (requestID) void cancelAILibrarian(requestID).catch(() => {})
  }

  function removeCandidate(candidate: LibrarianCandidate) {
    if (!result.value) return
    const key = candidateKey(candidate)
    result.value = {
      ...result.value,
      candidates: result.value.candidates.filter((item) => candidateKey(item) !== key),
    }
    if (result.value.candidates.length === 0) state.value = 'empty'
  }

  function setEmpty(noteID: string, revision: number, requestedOperation: LibrarianOperation) {
    discard()
    targetNoteID.value = noteID
    baseRevision.value = revision
    operation.value = requestedOperation
    result.value = { operation: requestedOperation, quality: 'empty', candidates: [] }
    state.value = 'empty'
  }

  function setApplyError(code: string) {
    error.value = createError(code)
    state.value = 'error'
  }

  return {
    state,
    targetNoteID,
    baseRevision,
    operation,
    partialText,
    result,
    error,
    stale,
    isGenerating,
    start,
    cancel,
    discard,
    discardForNote,
    markStaleForRevision,
    removeCandidate,
    setEmpty,
    setApplyError,
  }
})
