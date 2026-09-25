import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  analyzeOrganization,
  applyOrganizationCandidates,
  discardOrganizationAnalysis,
  invalidateOrganizationAnalyses,
  onOrganizationProgress,
  type OrganizationAnalysis,
  type OrganizationAnalysisInput,
  type OrganizationApplyResult,
  type OrganizationCandidate,
  type OrganizationProgress,
} from '../api/organization'
import { useNoteStore } from './useNoteStore'
import { useTagStore } from './useTagStore'
import { useSupportWorkspaceStore } from './useSupportWorkspaceStore'

export type OrganizationView = 'tasks' | 'organize'
export type OrganizationMiniSource = 'ai' | 'related'

export type OrganizationSessionState = {
  analysis: OrganizationAnalysis | null
  selectedCandidateIds: string[]
  outcomes: Record<string, OrganizationApplyResult>
  isAnalyzing: boolean
  progress: OrganizationProgress | null
  error: string
}

const RETRYABLE_STATUSES = new Set(['save-failure', 'not-executed'])

function sessionKeyFor(input: OrganizationAnalysisInput) {
  if (input.scope === 'note') return `note:${input.noteId ?? ''}`
  if (input.scope === 'notebook' || input.scope === 'descendants') {
    return `notebook:${input.notebookId ?? ''}:${input.scope}`
  }
  return 'space'
}

function emptySession(): OrganizationSessionState {
  return {
    analysis: null,
    selectedCandidateIds: [],
    outcomes: {},
    isAnalyzing: false,
    progress: null,
    error: '',
  }
}

export const useOrganizationStore = defineStore('organization', () => {
  const activeView = ref<OrganizationView>('tasks')
  const sessions = ref<Record<string, OrganizationSessionState>>({})
  const centerSessionKey = ref('space')
  const lastCollectionScopeKey = ref('space')
  const isApplying = ref(false)
  const miniOpen = ref(false)
  const miniSessionKey = ref<string | null>(null)
  const miniNoteId = ref<string | null>(null)
  const miniSource = ref<OrganizationMiniSource | null>(null)

  const centerSession = computed(() => sessions.value[centerSessionKey.value] ?? null)
  const analysis = computed(() => centerSession.value?.analysis ?? null)
  const candidates = computed(() => analysis.value?.candidates ?? [])
  const selectedCandidateIds = computed(() => centerSession.value?.selectedCandidateIds ?? [])
  const outcomes = computed(() => centerSession.value?.outcomes ?? {})
  const isAnalyzing = computed(() => centerSession.value?.isAnalyzing ?? false)
  const progress = computed(() => centerSession.value?.progress ?? null)
  const error = computed(() => centerSession.value?.error ?? '')
  const pendingCount = computed(() => candidates.value.filter((candidate) => {
    const outcome = outcomes.value[candidate.id]
    return !outcome || RETRYABLE_STATUSES.has(outcome.status)
  }).length)

  const requestVersions = new Map<string, number>()
  let lockGeneration = 0

  function getSession(key: string) {
    return sessions.value[key] ?? null
  }

  function ensureSession(key: string) {
    if (!sessions.value[key]) sessions.value[key] = emptySession()
    return sessions.value[key]
  }

  function noteSessionKey(noteId: string) {
    return sessionKeyFor({ scope: 'note', noteId })
  }

  function getSessionForNote(noteId: string) {
    return getSession(noteSessionKey(noteId))
  }

  function isSessionAnalyzing(key: string) {
    return getSession(key)?.isAnalyzing ?? false
  }

  function canApply(candidateId: string, key = centerSessionKey.value) {
    const status = getSession(key)?.outcomes[candidateId]?.status
    return !status || RETRYABLE_STATUSES.has(status)
  }

  function open(view: OrganizationView = 'tasks') {
    activeView.value = view
    useSupportWorkspaceStore().open('organize')
  }

  function showScope(key: string) {
    centerSessionKey.value = key
    if (!key.startsWith('note:')) lastCollectionScopeKey.value = key
    activeView.value = 'organize'
    useSupportWorkspaceStore().open('organize')
  }

  function openNote(noteId: string): Promise<boolean> {
    const key = noteSessionKey(noteId)
    showScope(key)
    const session = ensureSession(key)
    return session.analysis || session.isAnalyzing
      ? Promise.resolve(true)
      : analyzeIntoSession({ scope: 'note', noteId }, key)
  }

  function openMini(noteId: string, source: OrganizationMiniSource): Promise<boolean> {
    const key = noteSessionKey(noteId)
    const session = ensureSession(key)
    miniNoteId.value = noteId
    miniSessionKey.value = key
    miniSource.value = source
    miniOpen.value = true
    if (!session.analysis && !session.isAnalyzing) {
      return analyzeIntoSession({ scope: 'note', noteId }, key)
    }
    return Promise.resolve(true)
  }

  function closeMini() {
    miniOpen.value = false
  }

  async function analyze(input: OrganizationAnalysisInput) {
    const key = sessionKeyFor(input)
    centerSessionKey.value = key
    if (input.scope !== 'note') lastCollectionScopeKey.value = key
    return analyzeIntoSession(input, key)
  }

  async function analyzeIntoSession(input: OrganizationAnalysisInput, key: string) {
    const session = ensureSession(key)
    const requestVersion = (requestVersions.get(key) ?? 0) + 1
    requestVersions.set(key, requestVersion)
    const requestLockGeneration = lockGeneration
    const requestId = crypto.randomUUID()
    session.isAnalyzing = true
    session.progress = null
    session.error = ''
    const stopProgress = onOrganizationProgress((event) => {
      if (event.requestId !== requestId || requestLockGeneration !== lockGeneration || requestVersions.get(key) !== requestVersion) return
      session.progress = event
    })
    try {
      const next = await analyzeOrganization({ ...input, requestId })
      if (
        requestLockGeneration !== lockGeneration
        || requestVersions.get(key) !== requestVersion
      ) {
        void discardOrganizationAnalysis(next.sessionId).catch(() => {})
        return false
      }
      session.analysis = next
      session.selectedCandidateIds = []
      session.outcomes = {}
      return true
    } catch {
      if (requestLockGeneration === lockGeneration && requestVersions.get(key) === requestVersion) {
        session.error = 'ノートの解析に失敗しました。既存の候補は保持されています。'
      }
      return false
    } finally {
      stopProgress()
      if (requestLockGeneration === lockGeneration && requestVersions.get(key) === requestVersion) {
        session.isAnalyzing = false
        session.progress = null
      }
    }
  }

  async function clearForLock() {
    lockGeneration += 1
    sessions.value = {}
    centerSessionKey.value = 'space'
    lastCollectionScopeKey.value = 'space'
    miniOpen.value = false
    miniSessionKey.value = null
    miniNoteId.value = null
    miniSource.value = null
    try {
      await invalidateOrganizationAnalyses()
    } catch {
      // The backend still checks lock state before every application.
    }
  }

  function toggleCandidate(id: string, key = centerSessionKey.value) {
    const session = getSession(key)
    if (!session || !canApply(id, key)) return
    session.selectedCandidateIds = session.selectedCandidateIds.includes(id)
      ? session.selectedCandidateIds.filter((candidateId) => candidateId !== id)
      : [...session.selectedCandidateIds, id]
  }

  function selectApplicable(key = centerSessionKey.value, visibleIds?: readonly string[]) {
    const session = getSession(key)
    if (!session?.analysis) return
    const visible = visibleIds ? new Set(visibleIds) : null
    const selectedVisible = session.analysis.candidates
      .filter((candidate) => (!visible || visible.has(candidate.id)) && candidate.applicable && canApply(candidate.id, key))
      .map((candidate) => candidate.id)
    session.selectedCandidateIds = visible
      ? [...session.selectedCandidateIds.filter((id) => !visible.has(id)), ...selectedVisible]
      : selectedVisible
  }

  async function applySelected(key = centerSessionKey.value, visibleIds?: readonly string[]) {
    const session = getSession(key)
    if (isApplying.value || session?.isAnalyzing || !session?.analysis || session.selectedCandidateIds.length === 0) return
    const visible = visibleIds ? new Set(visibleIds) : null
    isApplying.value = true
    try {
      const eligible = session.selectedCandidateIds
        .map((id) => session.analysis?.candidates.find((candidate) => candidate.id === id))
        .filter((candidate): candidate is OrganizationCandidate => !!candidate && (!visible || visible.has(candidate.id)) && candidate.applicable && canApply(candidate.id, key))
      const grouped = new Map<string, OrganizationCandidate[]>()
      for (const candidate of eligible) {
        if (!candidate.noteId) continue
        grouped.set(candidate.noteId, [...(grouped.get(candidate.noteId) ?? []), candidate])
      }
      const sessionId = session.analysis.sessionId
      const requestLockGeneration = lockGeneration
      for (const [noteId, group] of grouped) {
        await applyGroup(key, sessionId, noteId, group, requestLockGeneration)
      }
      if (requestLockGeneration === lockGeneration && getSession(key)?.analysis?.sessionId === sessionId) {
        session.selectedCandidateIds = session.selectedCandidateIds.filter((id) => {
          if (visible && !visible.has(id)) return true
          const status = session.outcomes[id]?.status
          return status !== undefined && RETRYABLE_STATUSES.has(status)
        })
      }
    } finally {
      isApplying.value = false
    }
  }

  async function applyCandidate(id: string, key = centerSessionKey.value) {
    const session = getSession(key)
    if (isApplying.value || session?.isAnalyzing || !session?.analysis) return
    const candidate = session.analysis.candidates.find((item) => item.id === id)
    if (!candidate || !candidate.applicable || !canApply(id, key)) return
    isApplying.value = true
    const sessionId = session.analysis.sessionId
    const requestLockGeneration = lockGeneration
    try {
      await applyGroup(key, sessionId, candidate.noteId ?? '', [candidate], requestLockGeneration)
      if (requestLockGeneration !== lockGeneration || getSession(key)?.analysis?.sessionId !== sessionId) return
      if (!RETRYABLE_STATUSES.has(session.outcomes[id]?.status ?? '')) {
        session.selectedCandidateIds = session.selectedCandidateIds.filter((candidateId) => candidateId !== id)
      }
    } catch {
      if (requestLockGeneration !== lockGeneration || getSession(key)?.analysis?.sessionId !== sessionId) return
      session.outcomes = {
        ...session.outcomes,
        [id]: { candidateId: id, status: 'save-failure', message: '保存に失敗しました。候補は保持されています。' },
      }
    } finally {
      isApplying.value = false
    }
  }

  async function applyGroup(
    key: string,
    sessionId: string,
    noteId: string,
    group: OrganizationCandidate[],
    requestLockGeneration: number,
  ) {
    if (!noteId || group.length === 0) return
    const isCurrent = () => requestLockGeneration === lockGeneration && getSession(key)?.analysis?.sessionId === sessionId
    let results: OrganizationApplyResult[] | null = null
    try {
      results = await useNoteStore().runOrganizationOperation(noteId, () => (
        applyOrganizationCandidates({ sessionId, candidateIds: group.map((candidate) => candidate.id) })
      ))
    } catch {
      if (!isCurrent()) return
      results = group.map((candidate) => ({
        candidateId: candidate.id,
        status: 'save-failure',
        message: '保存に失敗しました。候補は保持されています。',
      }))
    }
    if (!isCurrent()) return
    const session = getSession(key)
    if (!session) return
    const resultById = new Map((results ?? []).map((result) => [result.candidateId, result]))
    for (const candidate of group) {
      session.outcomes = {
        ...session.outcomes,
        [candidate.id]: resultById.get(candidate.id) ?? {
          candidateId: candidate.id,
          status: 'save-failure',
          message: '適用結果を取得できませんでした。候補を保持しています。',
        },
      }
    }
    if (group.some((candidate) => candidate.kind === 'tag-assignment' && session.outcomes[candidate.id]?.status === 'applied')) {
      try {
        await useTagStore().refreshNoteTagsIfActive(noteId)
      } catch {
        // The backend change succeeded; a later view refresh can restore the tag list.
      }
    }
  }

  return {
    activeView,
    sessions,
    centerSessionKey,
    lastCollectionScopeKey,
    analysis,
    candidates,
    pendingCount,
    selectedCandidateIds,
    outcomes,
    isAnalyzing,
    progress,
    isApplying,
    miniOpen,
    miniSessionKey,
    miniNoteId,
    miniSource,
    error,
    open,
    showScope,
    openNote,
    openMini,
    closeMini,
    analyze,
    clearForLock,
    getSession,
    getSessionForNote,
    noteSessionKey,
    isSessionAnalyzing,
    canApply,
    toggleCandidate,
    selectApplicable,
    applyCandidate,
    applySelected,
  }
})
