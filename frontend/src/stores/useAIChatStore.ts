import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { note } from '../../wailsjs/go/models'
import { listNotes } from '../api/notes'
import type { AgentEditProposal, AIChatMode, AIWebCitation } from '../api/ai'

export type AIChatTool =
  | 'summary'
  | 'writing'
  | 'title'
  | 'tags'
  | 'classification'
  | 'related'
  | 'duplicate'
  | 'web-search'

export type AIChatContext =
  | {
      kind: 'active-note'
      id: string
      label: string
    }
  | {
      kind: 'note'
      id: string
      label: string
    }
  | {
      kind: 'notebook'
      id: string
      label: string
    }

export type AIExplicitChatContext = Exclude<AIChatContext, { kind: 'active-note' }>

export type AgentProposalState =
  | 'generating'
  | 'awaiting-review'
  | 'applying'
  | 'applied'
  | 'conflict'
  | 'save-failure'
  | 'discarded'

export type AIChatTimelineEntry = {
  id: string
  role: 'user' | 'assistant' | 'system' | 'tool'
  kind: 'message' | 'tool-trace' | 'error' | 'agent-proposal'
  content: string
  tool?: AIChatTool
  status?: 'pending' | 'success' | 'error'
  citations?: AIWebCitation[]
  proposal?: AgentEditProposal
  proposalState?: AgentProposalState
  createdAt: number
}

type ActiveNoteContextInput = {
  id: string
  title?: string
}

type ContextResolution = {
  noteIDs: string[]
  notebookOmissions: Record<string, number>
  notebookResolvedCounts: Record<string, number>
}

const MAX_CONTEXT_NOTE_IDS = 10

function contextKey(context: Pick<AIChatContext, 'kind' | 'id'>) {
  return `${context.kind}:${context.id}`
}

function compareCatalogNotes(left: note.Summary, right: note.Summary) {
  const updatedComparison = new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime()
  return updatedComparison || left.id.localeCompare(right.id)
}

function normalizeLabel(value: string | undefined, fallback: string) {
  return value?.trim() || fallback
}

export const useAIChatStore = defineStore('ai-chat', () => {
  const mode = ref<AIChatMode>('ask')
  const draft = ref('')
  const activeNoteContext = ref<AIChatContext | null>(null)
  const explicitContexts = ref<AIExplicitChatContext[]>([])
  const selectedTool = ref<AIChatTool | null>(null)
  const timeline = ref<AIChatTimelineEntry[]>([])
  const catalogNotes = ref<note.Summary[]>([])
  const isContextLoading = ref(false)
  const isContextCatalogReady = ref(false)
  const contextError = ref<string | null>(null)
  let nextTimelineID = 0

  const contexts = computed<AIChatContext[]>(() => [
    ...(activeNoteContext.value ? [activeNoteContext.value] : []),
    ...explicitContexts.value,
  ])

  const contextResolution = computed<ContextResolution>(() => {
    const noteIDs: string[] = []
    const seen = new Set<string>()
    const notebookOmissions: Record<string, number> = {}
    const notebookResolvedCounts: Record<string, number> = {}

    const addNoteID = (noteID: string) => {
      if (!noteID || seen.has(noteID) || noteIDs.length >= MAX_CONTEXT_NOTE_IDS) return false
      seen.add(noteID)
      noteIDs.push(noteID)
      return true
    }

    if (activeNoteContext.value) addNoteID(activeNoteContext.value.id)

    for (const context of explicitContexts.value) {
      if (context.kind !== 'note') continue
      addNoteID(context.id)
    }

    for (const context of explicitContexts.value) {
      if (context.kind !== 'notebook') continue
      let omitted = 0
      let resolved = 0
      const candidates = catalogNotes.value
        .filter((item) => !item.isTrashed && item.notebookId === context.id)
        .sort(compareCatalogNotes)

      for (const candidate of candidates) {
        if (seen.has(candidate.id)) continue
        if (addNoteID(candidate.id)) {
          resolved += 1
        } else {
          omitted += 1
        }
      }
      notebookOmissions[context.id] = omitted
      notebookResolvedCounts[context.id] = resolved
    }

    return { noteIDs, notebookOmissions, notebookResolvedCounts }
  })

  const resolvedNoteIDs = computed(() => contextResolution.value.noteIDs)
  const notebookOmissions = computed(() => contextResolution.value.notebookOmissions)
  const notebookResolvedCounts = computed(() => contextResolution.value.notebookResolvedCounts)

  function setMode(nextMode: AIChatMode) {
    mode.value = nextMode
  }

  function setDraft(value: string) {
    draft.value = value
  }

  function clearConversation(options: { keepContexts?: boolean } = {}) {
    draft.value = ''
    selectedTool.value = null
    timeline.value = []
    if (!options.keepContexts) explicitContexts.value = []
    contextError.value = null
  }

  function setActiveNoteContext(noteContext: ActiveNoteContextInput | null) {
    const previousID = activeNoteContext.value?.id ?? null
    const nextID = noteContext?.id ?? null
    if (previousID !== nextID) clearConversation()

    activeNoteContext.value = noteContext
      ? {
          kind: 'active-note',
          id: noteContext.id,
          label: normalizeLabel(noteContext.title, '無題のノート'),
        }
      : null

    if (nextID) {
      explicitContexts.value = explicitContexts.value.filter(
        (context) => context.kind !== 'note' || context.id !== nextID,
      )
    }
  }

  async function loadContextCatalog() {
    if (isContextLoading.value) return false
    isContextLoading.value = true
    contextError.value = null
    try {
      catalogNotes.value = ((await listNotes()) ?? [])
        .filter((item) => !item.isTrashed)
        .sort(compareCatalogNotes)
      isContextCatalogReady.value = true
      return true
    } catch {
      isContextCatalogReady.value = false
      contextError.value = 'ノート候補を読み込めませんでした。ローカル保存を確認して再試行してください。'
      return false
    } finally {
      isContextLoading.value = false
    }
  }

  function addNoteContext(noteID: string, title?: string) {
    const normalizedID = noteID.trim()
    if (!normalizedID || activeNoteContext.value?.id === normalizedID) return false
    if (explicitContexts.value.some((context) => context.kind === 'note' && context.id === normalizedID)) {
      return false
    }
    const explicitNoteCount = explicitContexts.value.filter(
      (context) => context.kind === 'note',
    ).length + (activeNoteContext.value ? 1 : 0)
    if (explicitNoteCount >= MAX_CONTEXT_NOTE_IDS) {
      contextError.value = `参照できるノートは、開いているノートを含めて最大${MAX_CONTEXT_NOTE_IDS}件です。`
      return false
    }

    const catalogNote = catalogNotes.value.find((item) => item.id === normalizedID)
    if (catalogNote?.isTrashed) return false
    explicitContexts.value = [
      ...explicitContexts.value,
      {
        kind: 'note',
        id: normalizedID,
        label: normalizeLabel(title ?? catalogNote?.title, '無題のノート'),
      },
    ]
    contextError.value = null
    return true
  }

  function addNotebookContext(notebookID: string, name?: string) {
    const normalizedID = notebookID.trim()
    if (!normalizedID) return false
    if (!isContextCatalogReady.value) {
      contextError.value = 'ノート一覧を読み込んでからノートブックを追加してください。'
      return false
    }
    if (explicitContexts.value.some((context) => context.kind === 'notebook' && context.id === normalizedID)) {
      return false
    }

    explicitContexts.value = [
      ...explicitContexts.value,
      {
        kind: 'notebook',
        id: normalizedID,
        label: normalizeLabel(name, '無題のノートブック'),
      },
    ]
    contextError.value = null
    return true
  }

  function removeContext(kind: 'note' | 'notebook', id: string) {
    explicitContexts.value = explicitContexts.value.filter(
      (context) => contextKey(context) !== `${kind}:${id}`,
    )
    contextError.value = null
  }

  function selectTool(tool: AIChatTool | null) {
    selectedTool.value = tool
  }

  function appendTimelineEntry(
    entry: Omit<AIChatTimelineEntry, 'id' | 'createdAt'>,
  ) {
    const created: AIChatTimelineEntry = {
      ...entry,
      id: `ai-chat-${++nextTimelineID}`,
      createdAt: Date.now(),
    }
    timeline.value = [...timeline.value, created]
    return created.id
  }

  function appendUserMessage(content: string) {
    return appendTimelineEntry({ role: 'user', kind: 'message', content })
  }

  function appendAssistantMessage(content: string, citations: AIWebCitation[] = []) {
    return appendTimelineEntry({
      role: 'assistant',
      kind: 'message',
      content,
      ...(citations.length > 0 ? { citations: [...citations] } : {}),
    })
  }

  function appendAgentProposalPlaceholder() {
    return appendTimelineEntry({
      role: 'assistant',
      kind: 'agent-proposal',
      content: 'ノート本文の変更提案を生成しています…',
      proposalState: 'generating',
    })
  }

  function resolveAgentProposal(id: string, content: string, proposal: AgentEditProposal | null) {
    if (!proposal) {
      updateTimelineEntry(id, {
        kind: 'message',
        content,
        proposal: undefined,
        proposalState: undefined,
      })
      return
    }
    updateTimelineEntry(id, {
      kind: 'agent-proposal',
      content,
      proposal,
      proposalState: 'awaiting-review',
    })
  }

  function setAgentProposalState(id: string, proposalState: AgentProposalState, content?: string) {
    updateTimelineEntry(id, {
      proposalState,
      ...(typeof content === 'string' ? { content } : {}),
    })
  }

  function discardAgentProposal(id: string) {
    const entry = timeline.value.find((item) => item.id === id)
    if (!entry || entry.kind !== 'agent-proposal' || entry.proposalState === 'applying') return
    updateTimelineEntry(id, {
      proposal: undefined,
      proposalState: 'discarded',
      content: '変更提案を破棄しました。ノートは変更されていません。',
    })
  }

  function markAgentProposalStale(noteID: string, revision: number) {
    timeline.value = timeline.value.map((entry) => {
      if (
        entry.kind !== 'agent-proposal'
        || !entry.proposal
        || entry.proposal.targetNoteID !== noteID
        || entry.proposal.baseRevision === revision
        || (entry.proposalState !== 'awaiting-review' && entry.proposalState !== 'save-failure')
      ) return entry
      return {
        ...entry,
        proposalState: 'conflict',
        content: '対象ノートが更新されたため、この変更提案は適用できません。内容を確認して再生成してください。',
      }
    })
  }

  function appendToolTrace(
    tool: AIChatTool,
    content: string,
    status: AIChatTimelineEntry['status'] = 'pending',
  ) {
    return appendTimelineEntry({
      role: 'tool',
      kind: 'tool-trace',
      content,
      tool,
      status,
    })
  }

  function appendError(content: string, tool?: AIChatTool) {
    return appendTimelineEntry({
      role: 'system',
      kind: 'error',
      content,
      status: 'error',
      ...(tool ? { tool } : {}),
    })
  }

  function updateTimelineEntry(
    id: string,
    update: Partial<Omit<AIChatTimelineEntry, 'id' | 'createdAt'>>,
  ) {
    timeline.value = timeline.value.map((entry) => (
      entry.id === id ? { ...entry, ...update } : entry
    ))
  }

  function removeTimelineEntry(id: string) {
    timeline.value = timeline.value.filter((entry) => entry.id !== id)
  }

  function replaceTimelineFromConversation(
    messages: Array<{ role: 'user' | 'assistant'; content: string }>,
    citations: AIWebCitation[] = [],
  ) {
    timeline.value = messages.map((message, index) => ({
      id: `ai-chat-${++nextTimelineID}`,
      role: message.role,
      kind: 'message',
      content: message.content,
      ...(message.role === 'assistant' && index === messages.length - 1 && citations.length > 0
        ? { citations: [...citations] }
        : {}),
      createdAt: Date.now() + index,
    }))
  }

  return {
    mode,
    draft,
    activeNoteContext,
    explicitContexts,
    selectedTool,
    timeline,
    catalogNotes,
    isContextLoading,
    isContextCatalogReady,
    contextError,
    contexts,
    resolvedNoteIDs,
    notebookOmissions,
    notebookResolvedCounts,
    setMode,
    setDraft,
    setActiveNoteContext,
    loadContextCatalog,
    addNoteContext,
    addNotebookContext,
    removeContext,
    selectTool,
    appendUserMessage,
    appendAssistantMessage,
    appendAgentProposalPlaceholder,
    appendToolTrace,
    appendError,
    updateTimelineEntry,
    resolveAgentProposal,
    setAgentProposalState,
    discardAgentProposal,
    markAgentProposalStale,
    removeTimelineEntry,
    replaceTimelineFromConversation,
    clearConversation,
  }
})
