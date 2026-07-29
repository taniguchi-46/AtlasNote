<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    class="ai-summary-panel"
    aria-label="AI要約"
    aria-live="polite"
  >
    <div class="ai-summary-heading">
      <strong>AI要約</strong>
      <span v-if="aiStore.summaryState === 'generating'">生成中…</span>
    </div>

    <p class="ai-summary-privacy">
      現在のノート本文だけを送信します。生成結果は保存・同期されません。
    </p>

    <div v-if="aiStore.summaryState === 'idle'" class="ai-summary-actions">
      <button
        class="ai-summary-action ai-summary-generate"
        type="button"
        title="要約を生成"
        aria-label="要約を生成"
        @click="handleAISummary"
      >
        <SparklesIcon :size="16" aria-hidden="true" />
      </button>
    </div>

    <p v-else-if="aiStore.summaryState === 'confirming'" class="ai-summary-status">
      要約の送信を確認しています。
    </p>
    <template v-else-if="aiStore.summaryError">
      <p class="ai-summary-error" role="alert">
        {{ aiStore.summaryError.message }}
      </p>
      <div class="ai-summary-actions">
        <button
          class="ai-summary-action"
          type="button"
          title="要約を再試行"
          aria-label="要約を再試行"
          @click="handleAISummary"
        >
          <RotateCcwIcon :size="15" aria-hidden="true" />
        </button>
        <button
          class="ai-summary-action"
          type="button"
          title="要約を閉じる"
          aria-label="要約を閉じる"
          @click="aiStore.discardSummary"
        >
          <XIcon :size="15" aria-hidden="true" />
        </button>
      </div>
    </template>
    <template v-else-if="visibleAISummary">
      <p v-if="isAISummaryStale" class="ai-summary-warning" role="status">
        ノートが更新されたため、この要約は現在の本文より古い可能性があります。コピーはできます。
      </p>
      <pre class="ai-summary-result">{{ visibleAISummary.text }}</pre>
      <div class="ai-summary-actions">
        <button
          class="ai-summary-action"
          type="button"
          title="要約をコピー"
          aria-label="要約をコピー"
          @click="handleCopyAISummary"
        >
          <CopyIcon :size="15" aria-hidden="true" />
        </button>
        <button
          class="ai-summary-action"
          type="button"
          title="要約を破棄"
          aria-label="要約を破棄"
          @click="aiStore.discardSummary"
        >
          <XIcon :size="15" aria-hidden="true" />
        </button>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { CopyIcon, RotateCcwIcon, SparklesIcon, XIcon } from '@lucide/vue'
import { useAIStore } from '../stores/useAIStore'
import { useNoteStore, type NoteDraft } from '../stores/useNoteStore'
import { useNotificationStore } from '../stores/useNotificationStore'
import { useSettingsStore } from '../stores/useSettingsStore'
import { createTableClipboardPayload, writeTableClipboard } from '../utils/tableClipboard'
import { logOperationFailure } from '../utils/operationLogger'

const aiStore = useAIStore()
const noteStore = useNoteStore()
const notificationStore = useNotificationStore()
const settingsStore = useSettingsStore()
let activeNoteID: string | null = null

const visibleAISummary = computed(() => {
  const note = noteStore.activeNote
  const result = aiStore.summary
  if (!note || !result || result.noteID !== note.id) return null
  return result
})

const isAISummaryStale = computed(() => (
  Boolean(visibleAISummary.value && noteStore.activeNote)
  && visibleAISummary.value!.baseRevision !== noteStore.activeNote!.revision
))

watch(
  () => noteStore.activeNote,
  (note) => {
    if (!note) {
      activeNoteID = null
      aiStore.discardSummaryForActiveNote(null)
      return
    }

    const noteChanged = activeNoteID !== note.id
    activeNoteID = note.id
    if (noteChanged) aiStore.discardSummaryForActiveNote(note.id)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  aiStore.discardSummaryForActiveNote(null)
})

async function handleAISummary() {
  const selectedNote = noteStore.activeNote
  if (!selectedNote) return

  if (!aiStore.isSummaryReady) {
    aiStore.setSummaryPreconditionError('AI_SUMMARY_NOT_READY', selectedNote.id)
    settingsStore.openSettings('ai')
    return
  }
  if (selectedNote.isTrashed) {
    aiStore.setSummaryPreconditionError('AI_NOTE_UNAVAILABLE', selectedNote.id)
    return
  }
  if (noteStore.activeDraft?.status === 'conflicted' || noteStore.activeDraft?.status === 'failed') {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', selectedNote.id)
    return
  }

  const noteID = selectedNote.id
  let saved = false
  try {
    saved = await noteStore.flushPendingDraft()
  } catch {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', noteID)
    return
  }
  const currentNote = noteStore.activeNote
  const currentDraft = noteStore.activeDraft
  if (!saved || !currentNote || currentNote.id !== noteID || currentDraft) {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', noteID)
    return
  }
  if (currentNote.isTrashed) {
    aiStore.setSummaryPreconditionError('AI_NOTE_UNAVAILABLE', noteID)
    return
  }

  if (!aiStore.beginSummary({
    noteID,
    content: currentNote.content,
    baseRevision: currentNote.revision,
  })) {
    if (!aiStore.isSummaryReady) settingsStore.openSettings('ai')
    return
  }

  const snapshot = aiStore.pendingSummary
  if (!snapshot) return
  const confirmed = window.confirm(
    `次の内容を AI に送信して要約します。\n\nプロバイダー: ${snapshot.providerID}\nモデル: ${snapshot.modelID}\n送信内容: 現在のノート本文のみ\n指示: 次のメモを、事実を補わずに簡潔に要約してください。\n\n生成結果はノートに保存・同期されません。`,
  )
  if (!confirmed) {
    aiStore.cancelSummaryConfirmation()
    return
  }

  const confirmationNote = noteStore.activeNote
  const confirmationDraft = noteStore.activeDraft as NoteDraft | null
  await aiStore.confirmSummary({
    noteID: confirmationNote?.id ?? null,
    content: confirmationDraft?.content ?? confirmationNote?.content ?? null,
    revision: confirmationNote?.revision ?? null,
    hasPendingDraft: Boolean(confirmationDraft),
  })
}

async function handleCopyAISummary() {
  const result = visibleAISummary.value
  if (!result) return

  try {
    await writeTableClipboard(createTableClipboardPayload(result.text))
    notificationStore.notify('要約をクリップボードにコピーしました。', {
      kind: 'success',
      source: 'ai',
      code: 'AI_SUMMARY_COPIED',
      dedupeKey: 'ai:summary-copied',
    })
  } catch (error) {
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.ai-summary-copy',
      errorCategory: getClipboardErrorCategory(error),
    })
    notificationStore.notify('要約をコピーできませんでした。', {
      kind: 'error',
      source: 'ai',
      code: 'AI_SUMMARY_COPY_FAILED',
      dedupeKey: 'ai:summary-copy-failed',
    })
  }
}

function getClipboardErrorCategory(error: unknown) {
  const errorName = error instanceof DOMException || error instanceof Error
    ? error.name
    : ''

  switch (errorName) {
    case 'DataError':
      return 'clipboard-data-error'
    case 'NotAllowedError':
      return 'clipboard-not-allowed'
    case 'NotSupportedError':
      return 'clipboard-not-supported'
    default:
      return 'clipboard-write-failed'
  }
}

defineExpose({ startSummary: handleAISummary })
</script>

<style scoped>
.ai-summary-panel {
  display: grid;
  gap: 10px;
  padding: 14px;
}

.ai-summary-heading,
.ai-summary-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-summary-heading {
  justify-content: space-between;
}

.ai-summary-heading span,
.ai-summary-status,
.ai-summary-privacy {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
}

.ai-summary-privacy {
  line-height: 1.5;
}

.ai-summary-error {
  margin: 0;
  color: var(--color-danger);
}

.ai-summary-warning {
  margin: 0;
  color: var(--color-warning);
}

.ai-summary-result {
  margin: 0;
  padding: 10px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font: inherit;
  line-height: 1.6;
}

.ai-summary-actions {
  justify-content: flex-start;
}

.ai-summary-action {
  display: inline-grid;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.ai-summary-generate {
  background: var(--brand-primary) !important;
  color: #fff !important;
}
</style>
