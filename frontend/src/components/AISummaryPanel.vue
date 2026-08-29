<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    class="ai-summary-panel"
    aria-label="AI要約"
    aria-live="polite"
  >
    <div v-if="!props.timeline" class="ai-summary-heading">
      <strong>AI要約</strong>
      <span v-if="aiStore.summaryState === 'generating'">生成中…</span>
    </div>

    <p v-if="!props.timeline" class="ai-summary-privacy">
      現在のノート本文だけを送信します。成功した要約はこの端末の履歴へ保存され、WebDAV同期はされません。
    </p>

    <p v-if="isActiveNoteProtected" class="ai-summary-privacy" role="status">
      保護されたノートはAI機能では利用できません。
    </p>

    <div v-if="!props.timeline && aiStore.summaryState === 'idle'" class="ai-summary-actions">
      <button
        class="ai-summary-action ai-summary-generate"
        type="button"
        title="要約を生成"
        aria-label="要約を生成"
        :disabled="isActiveNoteProtected"
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
      <p v-if="!isSummaryForActiveNote" class="ai-summary-warning" role="status">
        保存済みの要約を表示しています。参照元: {{ visibleAISummary.title || '無題のノート' }}
      </p>
      <p v-if="isAISummaryStale" class="ai-summary-warning" role="status">
        ノートが更新されたため、この要約は現在の本文より古い可能性があります。コピーはできます。
      </p>
      <AIMarkdownPreview class="ai-summary-result" :markdown="visibleAISummary.text" aria-label="AI要約のMarkdownプレビュー" />
      <p v-if="aiStore.summaryHistoryError" class="ai-summary-warning" role="status">
        {{ aiStore.summaryHistoryError.message }}
      </p>
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
          v-if="aiStore.summaryHistoryError"
          class="ai-summary-action"
          type="button"
          title="要約履歴を再保存"
          aria-label="要約履歴を再保存"
          @click="aiStore.retrySaveSummaryHistory"
        >
          <RotateCcwIcon :size="15" aria-hidden="true" />
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
import AIMarkdownPreview from './AIMarkdownPreview.vue'
import { createTableClipboardPayload, writeTableClipboard } from '../utils/tableClipboard'
import { logOperationFailure } from '../utils/operationLogger'

const props = withDefaults(defineProps<{ timeline?: boolean }>(), {
  timeline: false,
})
const aiStore = useAIStore()
const noteStore = useNoteStore()
const notificationStore = useNotificationStore()
const settingsStore = useSettingsStore()
let activeNoteID: string | null = null

const visibleAISummary = computed(() => {
  const result = aiStore.summary
  return result ?? null
})

const isSummaryForActiveNote = computed(() => (
  Boolean(visibleAISummary.value && noteStore.activeNote)
  && visibleAISummary.value!.noteID === noteStore.activeNote!.id
))

const isAISummaryStale = computed(() => (
  isSummaryForActiveNote.value
  && visibleAISummary.value!.baseRevision !== noteStore.activeNote!.revision
))

const isActiveNoteProtected = computed(() => Boolean(
  (noteStore.activeNote as { protected?: boolean } | null)?.protected,
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
  if (!selectedNote) return false
  if (isActiveNoteProtected.value) {
    aiStore.setSummaryPreconditionError('AI_CONTENT_PROTECTED', selectedNote.id)
    return false
  }

  if (!aiStore.isSummaryReady) {
    aiStore.setSummaryPreconditionError('AI_SUMMARY_NOT_READY', selectedNote.id)
    settingsStore.openSettings('ai')
    return false
  }
  if (selectedNote.isTrashed) {
    aiStore.setSummaryPreconditionError('AI_NOTE_UNAVAILABLE', selectedNote.id)
    return false
  }
  if (noteStore.activeDraft?.status === 'conflicted' || noteStore.activeDraft?.status === 'failed') {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', selectedNote.id)
    return false
  }

  const noteID = selectedNote.id
  let saved = false
  try {
    saved = await noteStore.flushPendingDraft()
  } catch {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', noteID)
    return false
  }
  const currentNote = noteStore.activeNote
  const currentDraft = noteStore.activeDraft
  if (!saved || !currentNote || currentNote.id !== noteID || currentDraft) {
    aiStore.setSummaryPreconditionError('AI_DRAFT_NOT_SAVED', noteID)
    return false
  }
  if (currentNote.isTrashed) {
    aiStore.setSummaryPreconditionError('AI_NOTE_UNAVAILABLE', noteID)
    return false
  }

  if (!aiStore.beginSummary({
    noteID,
    title: currentNote.title,
    content: currentNote.content,
    baseRevision: currentNote.revision,
  })) {
    if (!aiStore.isSummaryReady) settingsStore.openSettings('ai')
    return false
  }

  const snapshot = aiStore.pendingSummary
  if (!snapshot) return false
  const confirmed = window.confirm(
    `次の内容を AI に送信して要約します。\n\nプロバイダー: ${snapshot.providerID}\nモデル: ${snapshot.modelID}\n送信内容: 現在のノート本文のみ\n出力: Markdown形式の概要・要点・必要に応じた決定事項等\n\n成功した要約はこの端末の履歴へ保存されます。ノート本文とWebDAV同期は変更されません。`,
  )
  if (!confirmed) {
    aiStore.cancelSummaryConfirmation()
    return false
  }

  const confirmationNote = noteStore.activeNote
  const confirmationDraft = noteStore.activeDraft as NoteDraft | null
  return aiStore.confirmSummary({
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
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
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
