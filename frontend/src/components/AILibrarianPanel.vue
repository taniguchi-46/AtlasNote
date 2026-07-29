<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    class="ai-librarian-panel"
    aria-label="AI司書"
    aria-live="polite"
  >
    <div class="ai-librarian-heading">
      <strong>AI司書</strong>
      <span v-if="librarianStore.isGenerating">生成中…</span>
      <span v-else-if="librarianStore.state === 'canceled'">キャンセル済み</span>
    </div>

    <div class="ai-librarian-actions" role="group" aria-label="AI司書の操作">
      <button
        v-for="item in operations"
        :key="item.value"
        class="ai-librarian-icon-button"
        type="button"
        :title="item.label"
        :aria-label="item.label"
        :disabled="librarianStore.isGenerating || noteStore.isSaving"
        @click="startOperation(item.value)"
      >
        <component :is="item.icon" :size="15" aria-hidden="true" />
      </button>
    </div>

    <p v-if="librarianStore.state === 'error' && librarianStore.error" class="ai-librarian-error" role="alert">
      {{ librarianStore.error.message }}
    </p>
    <p v-else-if="librarianStore.state === 'empty'" class="ai-librarian-status">
      候補はありません。
    </p>
    <p v-else-if="librarianStore.state === 'canceled'" class="ai-librarian-status">
      部分応答は破棄され、ノートへ適用されていません。
    </p>
    <p v-else-if="librarianStore.state === 'stale'" class="ai-librarian-warning" role="status">
      ノートが更新されたため、候補は古くなっています。再試行してください。
    </p>

    <div v-if="librarianStore.isGenerating" class="ai-librarian-progress">
      <p class="ai-librarian-status">
        {{ librarianStore.state === 'canceling' ? 'キャンセル中…' : '候補を生成しています。' }}
      </p>
      <pre v-if="librarianStore.partialText" class="ai-librarian-partial">{{ librarianStore.partialText }}</pre>
      <button
        v-if="librarianStore.state !== 'canceling'"
        class="ai-librarian-icon-button"
        type="button"
        title="生成をキャンセル"
        aria-label="生成をキャンセル"
        @click="librarianStore.cancel()"
      >
        <XIcon :size="15" aria-hidden="true" />
      </button>
    </div>

    <template v-if="librarianStore.result && librarianStore.result.candidates.length > 0">
      <p v-if="librarianStore.result.quality === 'low'" class="ai-librarian-warning" role="status">
        関連度が低い候補を含みます。内容を確認してから採用してください。
      </p>
      <ul class="ai-librarian-candidates">
        <li v-for="candidate in librarianStore.result.candidates" :key="candidateKey(candidate)" class="ai-librarian-candidate">
          <div class="ai-librarian-candidate-body">
            <strong>{{ candidateLabel(candidate) }}</strong>
            <span class="ai-librarian-score">{{ Math.round(candidate.score * 100) }}%</span>
            <p v-if="candidate.reason" class="ai-librarian-reason">{{ candidate.reason }}</p>
          </div>
          <div class="ai-librarian-candidate-actions">
            <button
              v-if="canAdopt(candidate)"
              class="ai-librarian-icon-button"
              type="button"
              title="候補を採用"
              aria-label="候補を採用"
              :disabled="noteStore.isSaving"
              @click="adopt(candidate)"
            >
              <CheckIcon :size="15" aria-hidden="true" />
            </button>
            <span v-else-if="librarianStore.operation === 'duplicate'" class="ai-librarian-readonly">
              確認のみ
            </span>
            <span v-else-if="candidate.newTag" class="ai-librarian-readonly">
              タグを手動作成後に再試行
            </span>
            <button
              class="ai-librarian-icon-button"
              type="button"
              title="候補を破棄"
              aria-label="候補を破棄"
              @click="librarianStore.removeCandidate(candidate)"
            >
              <XIcon :size="15" aria-hidden="true" />
            </button>
          </div>
        </li>
      </ul>
    </template>

    <div v-if="librarianStore.state === 'success' || librarianStore.state === 'empty' || librarianStore.state === 'error' || librarianStore.state === 'stale'" class="ai-librarian-footer">
      <button
        class="ai-librarian-icon-button"
        type="button"
        title="現在の操作を再試行"
        aria-label="現在の操作を再試行"
        @click="retryCurrentOperation"
      >
        <RotateCcwIcon :size="15" aria-hidden="true" />
      </button>
      <button
        class="ai-librarian-icon-button"
        type="button"
        title="候補を閉じる"
        aria-label="候補を閉じる"
        @click="librarianStore.discard"
      >
        <XIcon :size="15" aria-hidden="true" />
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import type { Component } from 'vue'
import {
  CheckIcon,
  CopyIcon,
  FolderTreeIcon,
  LinkIcon,
  RotateCcwIcon,
  SparklesIcon,
  TagsIcon,
  XIcon,
} from '@lucide/vue'
import type { note } from '../../wailsjs/go/models'
import { listBacklinks } from '../api/noteLinks'
import { searchNotes } from '../api/search'
import { createNoteLinkHref, createNoteLinkMarkdown } from '../utils/noteLink'
import { useAIStore } from '../stores/useAIStore'
import { useAILibrarianStore } from '../stores/useAILibrarianStore'
import { useNoteStore } from '../stores/useNoteStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useSettingsStore } from '../stores/useSettingsStore'
import { useTagStore } from '../stores/useTagStore'
import type { LibrarianCandidate, LibrarianCandidateContext, LibrarianOperation } from '../api/ai'

const noteStore = useNoteStore()
const tagStore = useTagStore()
const notebookStore = useNotebookStore()
const aiStore = useAIStore()
const librarianStore = useAILibrarianStore()
const settingsStore = useSettingsStore()
const candidatePool = ref<LibrarianCandidateContext[]>([])
const lastOperation = ref<LibrarianOperation | null>(null)
let activeNoteID: string | null = null

const operations: Array<{ value: LibrarianOperation; label: string; icon: Component }> = [
  { value: 'title', label: 'タイトル候補', icon: SparklesIcon },
  { value: 'tags', label: 'タグ候補', icon: TagsIcon },
  { value: 'classification', label: '分類候補', icon: FolderTreeIcon },
  { value: 'related', label: '関連メモ', icon: LinkIcon },
  { value: 'duplicate', label: '重複候補', icon: CopyIcon },
]

function candidateKey(candidate: LibrarianCandidate) {
  return candidate.noteID ?? candidate.notebookID ?? candidate.name ?? candidate.value ?? ''
}

function candidateLabel(candidate: LibrarianCandidate) {
  if (candidate.value) return candidate.value
  if (candidate.name) return candidate.name
  if (candidate.notebookID) {
    return notebookStore.notebooks.find((notebook) => notebook.id === candidate.notebookID)?.name
      ?? candidate.notebookID
  }
  if (candidate.noteID) {
    return candidatePool.value.find((item) => item.noteID === candidate.noteID)?.title ?? candidate.noteID
  }
  return '候補'
}

function canAdopt(candidate: LibrarianCandidate) {
  return librarianStore.state !== 'stale' && librarianStore.operation !== 'duplicate' && !candidate.newTag
}

function normalizeSearchTerm(value: string) {
  return value.trim().slice(0, 200)
}

function addCandidate(target: Map<string, LibrarianCandidateContext>, item: LibrarianCandidateContext) {
  if (!item.noteID || item.noteID === noteStore.activeNote?.id || target.has(item.noteID)) return
  target.set(item.noteID, item)
}

async function buildCandidatePool(note: note.Note) {
  const candidates = new Map<string, LibrarianCandidateContext>()
  const terms = [
    note.title,
    ...tagStore.activeNoteTags.map((tag) => tag.name),
  ]
    .map(normalizeSearchTerm)
    .filter((term, index, all) => term !== '' && all.indexOf(term) === index)

  for (const query of terms) {
    try {
      const result = await searchNotes({
        query,
        scope: 'all',
        includeTrashed: false,
        page: 1,
        pageSize: 20,
      } as note.SearchInput)
      if (result.error) continue
      for (const item of result.items ?? []) {
        addCandidate(candidates, {
          noteID: item.note.id,
          title: item.note.title,
          snippet: item.snippet,
        })
      }
    } catch {
      // Candidate discovery is best-effort; the local editor remains usable.
    }
    if (candidates.size >= 20) break
  }

  try {
    const backlinks = await listBacklinks({ noteId: note.id, page: 1, pageSize: 20 })
    for (const item of backlinks.items ?? []) {
      addCandidate(candidates, { noteID: item.id, title: item.title })
    }
  } catch {
    // Search candidates are still valid when backlink discovery fails.
  }

  return [...candidates.values()].slice(0, 20)
}

async function startOperation(requestedOperation: LibrarianOperation) {
  const selectedNote = noteStore.activeNote
  if (!selectedNote) return
  if (!aiStore.configuredSetting) {
    settingsStore.openSettings('ai')
    librarianStore.setApplyError('AI_CONFIGURATION_UNAVAILABLE')
    return
  }
  if (noteStore.activeDraft?.status === 'conflicted' || noteStore.activeDraft?.status === 'failed') {
    librarianStore.setApplyError('AI_DRAFT_NOT_SAVED')
    return
  }

  const noteID = selectedNote.id
  try {
    if (!await noteStore.flushPendingDraft()) {
      librarianStore.setApplyError('AI_DRAFT_NOT_SAVED')
      return
    }
  } catch {
    librarianStore.setApplyError('AI_DRAFT_NOT_SAVED')
    return
  }
  const currentNote = noteStore.activeNote
  if (!currentNote || currentNote.id !== noteID || noteStore.activeDraft) {
    librarianStore.setApplyError('AI_DRAFT_NOT_SAVED')
    return
  }

  if (!tagStore.activeNoteTagsReady || tagStore.activeNoteId !== noteID) {
    await tagStore.loadNoteTags(noteID)
  }
  if (notebookStore.notebooks.length === 0) {
    await notebookStore.fetchNotebooks()
  }

  let pool: LibrarianCandidateContext[] = []
  if (requestedOperation === 'related' || requestedOperation === 'duplicate') {
    pool = await buildCandidatePool(currentNote)
    if (pool.length === 0) {
      candidatePool.value = []
      lastOperation.value = requestedOperation
      librarianStore.setEmpty(currentNote.id, currentNote.revision, requestedOperation)
      return
    }
  }

  const label = operations.find((item) => item.value === requestedOperation)?.label ?? '候補'
  if (!window.confirm(`現在のノート「${currentNote.title || '無題'}」をAIへ送信して${label}を生成します。\n\n生成結果は保存されず、採用した候補だけが既存の保存処理へ渡されます。`)) {
    return
  }

  candidatePool.value = pool
  lastOperation.value = requestedOperation
  await librarianStore.start({
    operation: requestedOperation,
    noteID: currentNote.id,
    baseRevision: currentNote.revision,
    title: currentNote.title,
    content: currentNote.content,
    candidateCount: 5,
    candidates: pool,
    existingTags: tagStore.tags.map((tag) => ({ id: tag.id, name: tag.name })),
    notebooks: notebookStore.notebooks.map((notebook) => ({ id: notebook.id, name: notebook.name })),
  })
}

async function adopt(candidate: LibrarianCandidate) {
  const currentNote = noteStore.activeNote
  if (!currentNote || currentNote.id !== librarianStore.targetNoteID || currentNote.revision !== librarianStore.baseRevision || noteStore.activeDraft) {
    librarianStore.setApplyError('AI_REVISION_CONFLICT')
    return
  }

  try {
    switch (librarianStore.operation) {
      case 'title':
        if (!candidate.value || !await noteStore.saveNote(currentNote.id, { title: candidate.value })) throw new Error('AI_PROVIDER_UNAVAILABLE')
        break
      case 'classification':
        if (!candidate.notebookID) throw new Error('AI_INPUT_INVALID')
        await noteStore.moveNotesToNotebook([currentNote.id], candidate.notebookID)
        break
      case 'tags': {
        if (!candidate.name) throw new Error('AI_INPUT_INVALID')
        const currentTags = tagStore.activeNoteTags
        const normalized = normalizeTagName(candidate.name)
        const tag = tagStore.tags.find((item) => normalizeTagName(item.name) === normalized)
        if (!tag) {
          librarianStore.setApplyError('TAG_STATE_CONFLICT')
          return
        }
        const expectedTagIDs = currentTags.map((item) => item.id)
        const tagIDs = expectedTagIDs.includes(tag.id) ? expectedTagIDs : [...expectedTagIDs, tag.id]
        await tagStore.setTagsForNoteWithExpectedRevision(currentNote.id, tagIDs, currentNote.revision, expectedTagIDs)
        break
      }
      case 'related': {
        if (!candidate.noteID) throw new Error('AI_INPUT_INVALID')
        const targetHref = createNoteLinkHref(candidate.noteID)
        if (currentNote.content.includes(targetHref)) break
        const title = candidateLabel(candidate)
        const link = createNoteLinkMarkdown(title, candidate.noteID)
        const content = currentNote.content.trimEnd() + `\n\n- ${link}\n`
        if (!await noteStore.saveNote(currentNote.id, { content })) throw new Error('AI_PROVIDER_UNAVAILABLE')
        break
      }
      case 'duplicate':
        return
    }
    librarianStore.removeCandidate(candidate)
  } catch (cause) {
    const code = cause && typeof cause === 'object' && 'code' in cause && typeof cause.code === 'string'
      ? cause.code
      : ''
    librarianStore.setApplyError(code === 'TAG_STATE_CONFLICT' ? 'TAG_STATE_CONFLICT' : 'AI_REVISION_CONFLICT')
  }
}

function normalizeTagName(value: string) {
  return value.normalize('NFC').trim().replace(/\s+/gu, ' ').toLocaleLowerCase()
}

async function retryCurrentOperation() {
  if (lastOperation.value) await startOperation(lastOperation.value)
}

watch(
  () => [noteStore.activeNote?.id ?? null, noteStore.activeNote?.revision ?? null] as const,
  ([noteID, revision]) => {
    if (!noteID) {
      activeNoteID = null
      librarianStore.discardForNote(null)
      return
    }

    const noteChanged = activeNoteID !== noteID
    activeNoteID = noteID
    if (noteChanged) {
      librarianStore.discardForNote(noteID)
      return
    }
    if (typeof revision === 'number') librarianStore.markStaleForRevision(noteID, revision)
  },
  { immediate: true },
)

defineExpose({ startOperation })

onBeforeUnmount(() => {
  librarianStore.discardForNote(null)
})
</script>

<style scoped>
.ai-librarian-panel {
  border: 1px solid var(--border-color, #d8dee9);
  border-radius: 8px;
  margin: 10px 14px 0;
  padding: 10px;
  background: var(--panel-background, #fff);
}

.ai-librarian-heading,
.ai-librarian-actions,
.ai-librarian-footer,
.ai-librarian-candidate-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-librarian-heading {
  justify-content: space-between;
  margin-bottom: 8px;
}

.ai-librarian-actions {
  flex-wrap: wrap;
}

.ai-librarian-icon-button {
  display: inline-grid;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
  border: 1px solid var(--border-color, #cbd5e1);
  border-radius: 5px;
  background: transparent;
  cursor: pointer;
}

.ai-librarian-actions button:disabled,
.ai-librarian-candidate-actions button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.ai-librarian-status,
.ai-librarian-error,
.ai-librarian-warning,
.ai-librarian-reason,
.ai-librarian-readonly {
  margin: 8px 0;
  font-size: 12px;
}

.ai-librarian-error {
  color: #b42318;
}

.ai-librarian-warning {
  color: #8a5a00;
}

.ai-librarian-partial,
.ai-librarian-candidate {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.ai-librarian-partial {
  margin: 8px 0;
  padding: 8px;
  background: var(--muted-background, #f8fafc);
  font-size: 11px;
}

.ai-librarian-candidates {
  display: grid;
  gap: 6px;
  margin: 10px 0;
  padding: 0;
  list-style: none;
}

.ai-librarian-candidate {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  border-top: 1px solid var(--border-color, #e2e8f0);
  padding-top: 8px;
}

.ai-librarian-candidate-body {
  min-width: 0;
}

.ai-librarian-score {
  margin-left: 8px;
  color: #64748b;
  font-size: 11px;
}

.ai-librarian-footer {
  justify-content: flex-end;
  margin-top: 8px;
}
</style>
