<template>
  <section class="editor-pane" aria-label="エディタ">
    <div v-if="!noteStore.activeNote" class="editor-empty">
      <div class="editor-empty-icon">
        <FileTextIcon :size="48" />
      </div>
      <p class="editor-empty-title">ノートを選択してください</p>
      <p class="editor-empty-sub">
        左のリストからノートを選ぶか、新しいノートを作成してください
      </p>
      <button
        id="btn-new-note-editor"
        class="primary-btn"
        type="button"
        @click="noteStore.newNote()"
      >
        新しいノート
      </button>
    </div>

    <template v-else>
      <div class="editor-toolbar">
        <div class="title-field">
          <input
            id="note-title-input"
            v-model="localTitle"
            class="title-input"
            :class="{ 'is-waiting-title': isWaitingForFirstLineTitle }"
            type="text"
            placeholder="タイトル"
            @input="handleTitleInput"
            @blur="handleTitleSave"
            @keydown.enter="handleTitleSave"
          />
          <div
            v-if="isWaitingForFirstLineTitle"
            class="title-loading"
            aria-hidden="true"
          >
            <span>新しいノートを作成中</span>
            <span class="loading-dots">
              <span>.</span>
              <span>.</span>
              <span>.</span>
            </span>
          </div>
        </div>

        <div class="toolbar-actions">
          <span v-if="noteStore.isSaving" class="saving-indicator">保存中...</span>
          <div
            v-else-if="saveConflicted"
            class="save-conflict-indicator"
            role="status"
            :title="conflictDetail"
          >
            <span>保存競合・下書き保持中</span>
            <button
              type="button"
              :disabled="noteStore.isLoading"
              @click="handleReloadConflict"
            >
              {{ noteStore.isLoading ? '再読込中...' : '最新版を再読込' }}
            </button>
            <button type="button" @click="handleCopyConflict">コピー保存</button>
          </div>
          <div
            v-else-if="saveFailed"
            class="save-error-indicator"
            role="status"
          >
            <span>保存失敗</span>
            <button type="button" @click="handleRetrySave">再試行</button>
            <button type="button" @click="handleDiscardDraft">破棄</button>
          </div>
          <span v-else-if="savedMessage" class="saved-indicator">保存済み</span>

          <div class="mode-segment" role="group" aria-label="エディタモード切り替え">
            <button
              class="mode-segment-btn"
              :class="{ 'is-active': editMode === 'wysiwyg' }"
              type="button"
              title="リッチテキストモード"
              aria-label="リッチテキストモード"
              :aria-pressed="editMode === 'wysiwyg'"
              @click="setEditMode('wysiwyg')"
            >
              <SquarePenIcon :size="17" />
            </button>
            <button
              class="mode-segment-btn"
              :class="{ 'is-active': editMode === 'markdown' }"
              type="button"
              title="Markdownモード"
              aria-label="Markdownモード"
              :aria-pressed="editMode === 'markdown'"
              @click="setEditMode('markdown')"
            >
              <SquareMIcon :size="17" />
            </button>
          </div>

          <button
            class="ai-summary-button"
            type="button"
            :disabled="aiStore.isGenerating"
            title="AIで要約"
            @click="handleAISummary"
          >
            <SparklesIcon :size="16" />
            <span>AIで要約</span>
          </button>

          <button
            class="icon-btn"
            type="button"
            :title="noteStore.activeNote.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加'"
            @click="noteStore.toggleFavorite(noteStore.activeNote.id)"
          >
            <StarIcon :size="18" :class="{ filled: noteStore.activeNote.isFavorite }" />
          </button>
          <button
            class="icon-btn"
            type="button"
            :title="noteStore.activeNote.isPinned ? 'ピン留めを外す' : 'ピン留め'"
            @click="noteStore.togglePinned(noteStore.activeNote.id)"
          >
            <PinIcon :size="18" :class="{ filled: noteStore.activeNote.isPinned }" />
          </button>
          <button
            class="icon-btn danger"
            type="button"
            title="ゴミ箱へ移動"
            @click="noteStore.trashNote(noteStore.activeNote.id)"
          >
            <Trash2Icon :size="18" />
          </button>
        </div>
      </div>

      <div class="editor-format-bar" @mousedown.prevent>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('bold') }"
          type="button"
          title="太字"
          @click="toggleBold"
        >
          <BoldIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('italic') }"
          type="button"
          title="斜体"
          @click="toggleItalic"
        >
          <ItalicIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('strike') }"
          type="button"
          title="取り消し線"
          @click="toggleStrike"
        >
          <StrikethroughIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('code') }"
          type="button"
          title="インラインコード"
          @click="toggleInlineCode"
        >
          <CodeIcon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('heading', { level: 1 }) }"
          type="button"
          title="見出し1"
          @click="toggleHeading(1)"
        >
          <Heading1Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('heading', { level: 2 }) }"
          type="button"
          title="見出し2"
          @click="toggleHeading(2)"
        >
          <Heading2Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('heading', { level: 3 }) }"
          type="button"
          title="見出し3"
          @click="toggleHeading(3)"
        >
          <Heading3Icon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('bulletList') }"
          type="button"
          title="箇条書きリスト"
          @click="toggleBulletList"
        >
          <ListIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('orderedList') }"
          type="button"
          title="番号付きリスト"
          @click="toggleOrderedList"
        >
          <ListOrderedIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('taskList') }"
          type="button"
          title="タスクリスト"
          @click="toggleTaskList"
        >
          <CheckSquareIcon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('blockquote') }"
          type="button"
          title="引用"
          @click="toggleBlockquote"
        >
          <QuoteIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('codeBlock') }"
          type="button"
          title="コードブロック"
          @click="toggleCodeBlock"
        >
          <TerminalIcon :size="15" />
        </button>

        <NoteLinkPopover
          v-if="noteStore.activeNote"
          :note-id="noteStore.activeNote.id"
          :disabled="noteStore.activeNote.isTrashed"
          @opened="rememberRichSelection"
          @select="insertNoteLink"
        />

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('table') }"
          type="button"
          title="表を挿入"
          @click="insertTable"
        >
          <Table2Icon :size="15" />
        </button>

        <template v-if="isTableActionVisible">
          <button
            class="format-btn"
            type="button"
            title="表をコピー"
            aria-label="表をコピー"
            @click="copyCurrentTable"
          >
            <ClipboardCopyIcon :size="15" />
          </button>
          <button
            class="format-btn"
            type="button"
            title="下に行を追加"
            @click="addTableRow"
          >
            <TableRowsSplitIcon :size="15" />
          </button>
          <button
            class="format-btn"
            type="button"
            title="右に列を追加"
            @click="addTableColumn"
          >
            <TableColumnsSplitIcon :size="15" />
          </button>
          <button
            class="format-btn danger"
            type="button"
            title="現在の行を削除"
            @click="deleteTableRow"
          >
            <Rows3Icon :size="15" />
          </button>
          <button
            class="format-btn danger"
            type="button"
            title="現在の列を削除"
            @click="deleteTableColumn"
          >
            <Columns3Icon :size="15" />
          </button>
          <button
            class="format-btn danger"
            type="button"
            title="表を削除"
            @click="deleteTable"
          >
            <Trash2Icon :size="15" />
          </button>
        </template>
      </div>

      <section
        v-if="isAISummaryVisible"
        class="ai-summary-panel"
        aria-live="polite"
        aria-label="AI要約"
      >
        <div class="ai-summary-heading">
          <strong>AI要約</strong>
          <span v-if="aiStore.summaryState === 'generating'">生成中…</span>
        </div>
        <p v-if="aiStore.summaryState === 'confirming'" class="ai-summary-status">
          要約の送信を確認しています。
        </p>
        <p v-else-if="aiStore.summaryError" class="ai-summary-error" role="alert">
          {{ aiStore.summaryError.message }}
        </p>
        <template v-else-if="visibleAISummary">
          <p v-if="isAISummaryStale" class="ai-summary-warning" role="status">
            ノートが更新されたため、この要約は現在の本文より古い可能性があります。コピーはできます。
          </p>
          <pre class="ai-summary-result">{{ visibleAISummary.text }}</pre>
          <div class="ai-summary-actions">
            <button type="button" @click="handleCopyAISummary">コピー</button>
            <button type="button" @click="aiStore.discardSummary">破棄</button>
          </div>
        </template>
        <div v-else-if="aiStore.summaryState === 'error'" class="ai-summary-actions">
          <button type="button" @click="handleAISummary">再試行</button>
          <button type="button" @click="aiStore.discardSummary">閉じる</button>
        </div>
      </section>

      <div class="editor-body">
        <EditorContent v-if="editMode === 'wysiwyg'" :editor="editor" class="prose-editor" />
        <textarea
          v-else
          ref="markdownTextarea"
          v-model="localMarkdown"
          class="markdown-textarea"
          placeholder="ここにMarkdownで内容を入力してください..."
          title="Ctrl / Cmd + クリックでノートリンクを開く"
          @input="handleMarkdownInput"
          @click="handleMarkdownClick"
          @keyup="updateMarkdownSelection"
          @select="updateMarkdownSelection"
        />
      </div>

      <div class="editor-statusbar">
        <div class="editor-statusbar-left">
          <NoteTagAddPopover
            :note-id="noteStore.activeNote.id"
            :disabled="noteStore.activeNote.isTrashed"
          />
          <NoteTags
            :note-id="noteStore.activeNote.id"
            :disabled="noteStore.activeNote.isTrashed"
          />
          <NoteBacklinks :note-id="noteStore.activeNote.id" />
          <span>{{ charCount }} 文字</span>
        </div>
        <span>更新: {{ formatDate(noteStore.activeNote.updatedAt) }}</span>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { JSONContent } from '@tiptap/core'
import {
  BoldIcon,
  CheckSquareIcon,
  ClipboardCopyIcon,
  CodeIcon,
  Columns3Icon,
  FileTextIcon,
  Heading1Icon,
  Heading2Icon,
  Heading3Icon,
  ItalicIcon,
  ListIcon,
  ListOrderedIcon,
  PinIcon,
  QuoteIcon,
  Rows3Icon,
  SparklesIcon,
  SquareMIcon,
  SquarePenIcon,
  StarIcon,
  StrikethroughIcon,
  Table2Icon,
  TableColumnsSplitIcon,
  TableRowsSplitIcon,
  TerminalIcon,
  Trash2Icon,
} from '@lucide/vue'
import { Editor, EditorContent } from '@tiptap/vue-3'
import {
  DOMParser as ProseMirrorDOMParser,
  DOMSerializer as ProseMirrorDOMSerializer,
} from '@tiptap/pm/model'
import type { Selection } from '@tiptap/pm/state'
import StarterKit from '@tiptap/starter-kit'
import { Markdown } from 'tiptap-markdown'
import { Placeholder } from '@tiptap/extension-placeholder'
import { Link } from '@tiptap/extension-link'
import { Image } from '@tiptap/extension-image'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableHeader } from '@tiptap/extension-table-header'
import { TableCell } from '@tiptap/extension-table-cell'
import { TaskList } from '@tiptap/extension-task-list'
import { TaskItem } from '@tiptap/extension-task-item'
import { CodeBlockLowlight } from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import { useNoteStore, type NoteDraft } from '../stores/useNoteStore'
import { useNotificationStore } from '../stores/useNotificationStore'
import { useSettingsStore } from '../stores/useSettingsStore'
import { useAIStore } from '../stores/useAIStore'
import NoteTags from './NoteTags.vue'
import NoteTagAddPopover from './NoteTagAddPopover.vue'
import NoteLinkPopover from './NoteLinkPopover.vue'
import NoteBacklinks from './NoteBacklinks.vue'
import { RICH_MARKDOWN_OPTIONS } from '../utils/markdownSecurity'
import { logOperationFailure } from '../utils/operationLogger'
import {
  createTableClipboardPayload,
  createTiptapTableClipboardPayload,
  writeTableClipboard,
} from '../utils/tableClipboard'
import { serializeTiptapJsonToMarkdown } from '../utils/tiptapMarkdownSerializer'
import {
  createNoteLinkHref,
  createNoteLinkMarkdown,
  findNoteLinkTargetAt,
  parseNoteLinkHref,
} from '../utils/noteLink'

const CustomTableCell = TableCell.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const CustomTableHeader = TableHeader.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const lowlight = createLowlight(common)
const noteStore = useNoteStore()
const notificationStore = useNotificationStore()
const settingsStore = useSettingsStore()
const aiStore = useAIStore()

const localTitle = ref('')
const savedMessage = ref(false)
const saveConflicted = computed(() => noteStore.activeDraft?.status === 'conflicted')
const conflictDetail = computed(() => {
  const conflict = noteStore.activeDraft?.conflict
  if (!conflict) return '他の更新と競合したため、ローカルの下書きを保持しています'

  return `保存元 revision ${conflict.expectedRevision} / 最新 revision ${conflict.actualRevision}`
})
const saveFailed = computed(() => noteStore.activeDraft?.status === 'failed')
const editMode = ref<'wysiwyg' | 'markdown'>('markdown')
const localMarkdown = ref('')
const markdownTextarea = ref<HTMLTextAreaElement | null>(null)
const isApplyingContent = ref(false)
const isRichDirty = ref(false)
const editorStateVersion = ref(0)
const markdownSelectionVersion = ref(0)
let lastMarkdownSelection = { start: 0, end: 0 }
let savedMessageTimer: ReturnType<typeof setTimeout> | null = null
let activeNoteId: string | null = null
let savedRichSelection: { from: number; to: number } | null = null

const editor = new Editor({
  extensions: [
    StarterKit.configure({
      codeBlock: false,
      link: false,
    }),
    Markdown.configure(RICH_MARKDOWN_OPTIONS),
    Placeholder.configure({
      emptyNodeClass: 'is-empty',
      showOnlyCurrent: true,
      placeholder: 'ここに内容を入力してください...',
    }),
    Link.configure({
      openOnClick: false,
      protocols: ['atlasnote'],
      isAllowedUri: (url, context) => {
        if (url.startsWith('atlasnote:')) {
          return parseNoteLinkHref(url) !== null
        }

        return context.defaultValidate(url)
      },
    }),
    Image,
    Table.configure({
      resizable: true,
    }),
    TableRow,
    CustomTableHeader,
    CustomTableCell,
    TaskList,
    TaskItem.configure({
      nested: true,
    }),
    CodeBlockLowlight.configure({
      lowlight,
    }),
  ],
  editorProps: {
    clipboardTextSerializer(_content, view) {
      const table = findRichTableNode(view.state.selection)
      return table ? createTiptapTableClipboardPayload(table).plainText : ''
    },
    handleClick(_view, _pos, event) {
      const target = event.target
      if (!(target instanceof Element)) return false

      const href = target.closest('a')?.getAttribute('href') ?? ''
      const noteId = parseNoteLinkHref(href)
      if (!noteId) return false

      void noteStore.selectNote(noteId)
      return true
    },
  },
  onSelectionUpdate() {
    editorStateVersion.value += 1
  },
  onUpdate({ editor }) {
    editorStateVersion.value += 1

    if (editMode.value !== 'wysiwyg') return
    if (isApplyingContent.value) return

    const markdown = serializeTiptapJsonToMarkdown(editor.getJSON())
    isRichDirty.value = true

    if (localMarkdown.value !== markdown) {
      localMarkdown.value = markdown
      updateAutoTitleFromMarkdown(markdown)
      scheduleAutoSave(markdown)
    }
  },
})

watch(
  () => noteStore.activeNote,
  (note) => {
    if (!note) {
      activeNoteId = null
      savedRichSelection = null
      aiStore.discardSummaryForActiveNote(null)
      return
    }

    const noteChanged = activeNoteId !== note.id
    activeNoteId = note.id
    const draft = noteStore.getDraft(note.id)
    const editableContent = draft?.content ?? note.content
    localTitle.value =
      draft?.title ?? (noteStore.autoTitleNoteId === note.id && extractTitleFromFirstMarkdownLine(editableContent) === ''
        ? ''
        : note.title)

    if (noteChanged) {
      aiStore.discardSummaryForActiveNote(note.id)
      savedRichSelection = null
      resetSaveFeedback()
      localMarkdown.value = editableContent
      isRichDirty.value = false
      if (editMode.value === 'wysiwyg') {
        if (!setEditorFromMarkdown(editableContent)) {
          editMode.value = 'markdown'
        }
      }
      return
    }

    if (editMode.value === 'markdown') {
      return
    }

    if (!isRichDirty.value && localMarkdown.value !== note.content) {
      localMarkdown.value = note.content
      if (!setEditorFromMarkdown(note.content)) {
        editMode.value = 'markdown'
      }
    }
  },
  { immediate: true },
)

watch(
  () => noteStore.saveFeedbackVersion,
  () => {
    if (noteStore.lastSavedNoteId === noteStore.activeNote?.id) {
      showSaved()
    }
  },
)

onBeforeUnmount(() => {
  aiStore.discardSummaryForActiveNote(null)
  void noteStore.flushPendingDraft()
  if (savedMessageTimer) {
    clearTimeout(savedMessageTimer)
  }
  editor.destroy()
})

const charCount = computed(() => {
  return localMarkdown.value.length
})

const visibleAISummary = computed(() => {
  const note = noteStore.activeNote
  const result = aiStore.summary
  if (!note || !result || result.noteID !== note.id) return null
  return result
})

const isAISummaryVisible = computed(() => (
  aiStore.summaryState !== 'idle'
  && aiStore.summaryTargetNoteID === noteStore.activeNote?.id
))

const isAISummaryStale = computed(() => (
  Boolean(visibleAISummary.value && noteStore.activeNote)
  && visibleAISummary.value!.baseRevision !== noteStore.activeNote!.revision
))

const isTableActive = computed(() => {
  editorStateVersion.value
  return editMode.value === 'wysiwyg' && editor.isActive('table')
})

const isTableActionVisible = computed(() => {
  if (editMode.value === 'wysiwyg') return isTableActive.value

  markdownSelectionVersion.value
  return findMarkdownTableRange() !== null
})

const isWaitingForFirstLineTitle = computed(() => {
  if (!noteStore.activeNote) return false
  if (noteStore.autoTitleNoteId !== noteStore.activeNote.id) return false

  return extractTitleFromFirstMarkdownLine(localMarkdown.value) === ''
})

function handleTitleSave() {
  if (!noteStore.activeNote) return
  if (isWaitingForFirstLineTitle.value && localTitle.value.trim() === '') return
  const draft = noteStore.getDraft(noteStore.activeNote.id)
  if (localTitle.value === (draft?.title ?? noteStore.activeNote.title)) {
    if (draft) {
      void noteStore.flushPendingDraft()
    }
    return
  }

  scheduleAutoSave(localMarkdown.value)
  void noteStore.flushPendingDraft()
}

function handleTitleInput() {
  disableAutoTitleFromContent()
  scheduleAutoSave(localMarkdown.value)
}

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

async function handleRetrySave() {
  const noteId = noteStore.activeNote?.id
  if (!noteId) return

  await noteStore.retryDraftSave(noteId)
}

async function handleReloadConflict() {
  const note = noteStore.activeNote
  if (!note) return
  if (!window.confirm('ローカルの下書きを破棄して、最新の保存内容を再読み込みますか？')) return

  const latestNote = await noteStore.reloadConflictedNote(note.id)
  if (!latestNote) return

  localTitle.value = latestNote.title
  localMarkdown.value = latestNote.content
  isRichDirty.value = false
  if (editMode.value === 'wysiwyg' && !setEditorFromMarkdown(latestNote.content)) {
    editMode.value = 'markdown'
  }
  resetSaveFeedback()
}

async function handleCopyConflict() {
  const noteId = noteStore.activeNote?.id
  if (!noteId) return

  await noteStore.copyConflictedDraft(noteId)
}

function handleDiscardDraft() {
  const note = noteStore.activeNote
  if (!note) return
  if (!window.confirm('未保存の変更を破棄して、最後に保存した内容へ戻しますか？')) return

  noteStore.discardDraft(note.id)
  localTitle.value = note.title
  localMarkdown.value = note.content
  isRichDirty.value = false
  if (editMode.value === 'wysiwyg' && !setEditorFromMarkdown(note.content)) {
    editMode.value = 'markdown'
  }
  resetSaveFeedback()
}

function disableAutoTitleFromContent() {
  if (!noteStore.activeNote) return
  if (noteStore.autoTitleNoteId !== noteStore.activeNote.id) return

  noteStore.autoTitleNoteId = null
}

function setEditMode(mode: 'wysiwyg' | 'markdown') {
  if (editMode.value === mode) return

  if (mode === 'markdown') {
    applyRichEditorToMarkdown()
    editMode.value = 'markdown'
    return
  }

  scheduleAutoSave(localMarkdown.value)
  if (setEditorFromMarkdown(localMarkdown.value)) {
    editMode.value = mode
  }
}

function setEditorFromMarkdown(markdown: string): boolean {
  isApplyingContent.value = true
  try {
    const html = parseMarkdownToRichHtml(markdown)
    const content = parseRichHtmlToJson(html)
    ;(editor.commands as any).setContent(content, {
      emitUpdate: false,
    })
    isRichDirty.value = false
    return true
  } catch {
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.markdown-to-rich',
      errorCategory: 'parse-failed',
    })
    return false
  } finally {
    isApplyingContent.value = false
  }
}

function applyRichEditorToMarkdown() {
  if (!isRichDirty.value) return

  const markdown = serializeTiptapJsonToMarkdown(editor.getJSON())
  if (localMarkdown.value !== markdown) {
    localMarkdown.value = markdown
    scheduleAutoSave(markdown)
  }
  isRichDirty.value = false
}

function parseMarkdownToRichHtml(markdown: string): string {
  return (editor.storage as any).markdown.parser.parse(markdown)
}

function parseRichHtmlToJson(html: string) {
  const container = document.createElement('div')
  container.innerHTML = html
  preserveSoftBreaks(container)
  normalizeTableCells(container)
  return ProseMirrorDOMParser.fromSchema(editor.schema).parse(container).toJSON()
}

function preserveSoftBreaks(container: HTMLElement) {
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT)
  const textNodes: Text[] = []

  while (walker.nextNode()) {
    const node = walker.currentNode as Text
    if (!node.textContent?.includes('\n')) continue
    if (node.textContent.trim().length === 0) continue
    if (hasAncestor(node, ['pre', 'code'])) continue

    textNodes.push(node)
  }

  textNodes.forEach((node) => {
    const parts = node.textContent?.split('\n') ?? []
    const fragment = document.createDocumentFragment()

    parts.forEach((part, index) => {
      if (index > 0) fragment.appendChild(document.createElement('br'))
      if (part.length > 0) fragment.appendChild(document.createTextNode(part))
    })

    node.replaceWith(fragment)
  })
}

function hasAncestor(node: Node, tagNames: string[]) {
  let current = node.parentElement

  while (current) {
    if (tagNames.includes(current.tagName.toLowerCase())) return true
    current = current.parentElement
  }

  return false
}

function normalizeTableCells(container: HTMLElement) {
  container.querySelectorAll('td, th').forEach((cell) => {
    if (hasBlockChild(cell)) return

    const paragraph = document.createElement('p')
    while (cell.firstChild) {
      paragraph.appendChild(cell.firstChild)
    }
    cell.appendChild(paragraph)
  })
}

function hasBlockChild(cell: Element) {
  return Array.from(cell.children).some((child) =>
    ['p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'blockquote', 'pre', 'hr'].includes(
      child.tagName.toLowerCase(),
    ),
  )
}

function toggleBold() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleBold().run()
    return
  }

  toggleMarkdownInlineWrap('**')
}

function toggleItalic() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleItalic().run()
    return
  }

  toggleMarkdownInlineWrap('*')
}

function toggleStrike() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleStrike().run()
    return
  }

  toggleMarkdownInlineWrap('~~')
}

function toggleInlineCode() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleCode().run()
    return
  }

  toggleMarkdownInlineWrap('`')
}

function toggleHeading(level: 1 | 2 | 3) {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleHeading({ level }).run()
    return
  }

  toggleMarkdownLinePrefix(`${'#'.repeat(level)} `, /^#{1,6}\s+/)
}

function toggleBulletList() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleBulletList().run()
    return
  }

  toggleMarkdownLinePrefix('- ', /^\s*[-*+]\s+/)
}

function toggleOrderedList() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleOrderedList().run()
    return
  }

  toggleMarkdownLinePrefix('1. ', /^\s*\d+\.\s+/)
}

function toggleTaskList() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleTaskList().run()
    return
  }

  toggleMarkdownLinePrefix('- [ ] ', /^\s*[-*+]\s+\[[ xX]\]\s+/)
}

function toggleBlockquote() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleBlockquote().run()
    return
  }

  toggleMarkdownLinePrefix('> ', /^\s*>\s?/)
}

function toggleCodeBlock() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().toggleCodeBlock().run()
    return
  }

  toggleMarkdownCodeBlock()
}

function rememberRichSelection() {
  if (editMode.value !== 'wysiwyg') return

  const { from, to } = editor.state.selection
  savedRichSelection = { from, to }
}

function insertNoteLink(target: { id: string; title: string }) {
  const href = createNoteLinkHref(target.id)

  if (editMode.value === 'markdown') {
    const selection = getMarkdownSelection()
    if (!selection) return

    const markdown = createNoteLinkMarkdown(target.title, target.id)
    replaceMarkdownRange(
      selection.start,
      selection.end,
      markdown,
      selection.start + markdown.length,
      selection.start + markdown.length,
    )
    return
  }

  const selection = savedRichSelection ?? {
    from: editor.state.selection.from,
    to: editor.state.selection.to,
  }
  savedRichSelection = null

  const chain = editor.chain().focus().setTextSelection(selection)
  if (selection.from === selection.to) {
    chain
      .insertContent({
        type: 'text',
        text: target.title,
        marks: [{ type: 'link', attrs: { href } }],
      })
      .run()
    return
  }

  chain.setLink({ href }).run()
}

function insertTable() {
  if (editMode.value === 'markdown') {
    insertMarkdownTable()
    return
  }

  editor
    .chain()
    .focus()
    .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
    .run()
}

function addTableRow() {
  if (editMode.value === 'markdown') {
    editMarkdownTable('addRow')
    return
  }

  editor.chain().focus().addRowAfter().run()
}

function addTableColumn() {
  if (editMode.value === 'markdown') {
    editMarkdownTable('addColumn')
    return
  }

  editor.chain().focus().addColumnAfter().run()
}

function deleteTableRow() {
  if (editMode.value === 'markdown') {
    editMarkdownTable('deleteRow')
    return
  }

  editor.chain().focus().deleteRow().run()
}

function deleteTableColumn() {
  if (editMode.value === 'markdown') {
    editMarkdownTable('deleteColumn')
    return
  }

  editor.chain().focus().deleteColumn().run()
}

function deleteTable() {
  if (editMode.value === 'markdown') {
    editMarkdownTable('deleteTable')
    return
  }

  editor.chain().focus().deleteTable().run()
}

async function copyCurrentTable() {
  try {
    const payload = getCurrentTableClipboardPayload()
    if (!payload) return
    await writeTableClipboard(payload)
  } catch (error) {
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.table-copy',
      errorCategory: getClipboardErrorCategory(error),
    })
    notificationStore.notify('表をコピーできませんでした', {
      kind: 'error',
      source: 'editor',
      code: 'TABLE_COPY_FAILED',
    })
  }
}

function getCurrentTableClipboardPayload() {
  if (editMode.value === 'markdown') {
    const tableRange = findMarkdownTableRange()
    if (!tableRange) return null
    const markdown = localMarkdown.value.slice(tableRange.start, tableRange.end)
    return createTableClipboardPayload(markdown, parseMarkdownToRichHtml(markdown))
  }

  const table = findRichTableNode(editor.state.selection)
  if (!table) return null

  const markdown = createTiptapTableClipboardPayload(table).markdown
  return createTableClipboardPayload(markdown, serializeRichTableToHtml(table))
}

function findRichTableNode(selection: Selection): JSONContent | null {
  const { $from } = selection

  for (let depth = $from.depth; depth > 0; depth -= 1) {
    const node = $from.node(depth)
    if (node.type.name === 'table') return node.toJSON()
  }

  return null
}

function serializeRichTableToHtml(table: JSONContent) {
  const node = editor.schema.nodeFromJSON(table)
  const serializer = ProseMirrorDOMSerializer.fromSchema(editor.schema)
  const element = serializer.serializeNode(node, { document })
  return element instanceof HTMLElement ? element.outerHTML : ''
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

function handleMarkdownInput() {
  updateMarkdownSelection()
  updateAutoTitleFromMarkdown(localMarkdown.value)
  scheduleAutoSave(localMarkdown.value)
}

function handleMarkdownClick(event: MouseEvent) {
  updateMarkdownSelection()
  if (!event.ctrlKey && !event.metaKey) return

  const textarea = markdownTextarea.value
  if (!textarea) return

  const noteID = findNoteLinkTargetAt(localMarkdown.value, textarea.selectionStart)
  if (!noteID) return

  event.preventDefault()
  void noteStore.selectNote(noteID)
}

function updateAutoTitleFromMarkdown(markdown: string) {
  if (!noteStore.activeNote) return
  if (noteStore.autoTitleNoteId !== noteStore.activeNote.id) return

  const title = extractTitleFromFirstMarkdownLine(markdown)
  if (!title) return
  if (localTitle.value === title) return

  localTitle.value = title
}

function extractTitleFromFirstMarkdownLine(markdown: string) {
  const firstLine = markdown.split(/\r?\n/, 1)[0] ?? ''
  const headingMatch = firstLine.match(/^#{1,6}\s+(.*)$/)
  const title = settingsStore.editorFirstLineStyle === 'paragraph'
    ? firstLine.trim()
    : headingMatch?.[1]?.trim()

  if (!title) return ''

  return Array.from(title).slice(0, 200).join('')
}

function updateMarkdownSelection() {
  const textarea = markdownTextarea.value
  if (textarea) {
    lastMarkdownSelection = {
      start: textarea.selectionStart,
      end: textarea.selectionEnd,
    }
  }

  markdownSelectionVersion.value += 1
}

function toggleMarkdownInlineWrap(marker: string) {
  const selection = getMarkdownSelection()
  if (!selection) return

  const { start, end } = selection
  const content = localMarkdown.value
  const markerLength = marker.length
  const hasOuterMarkers =
    start >= markerLength &&
    content.slice(start - markerLength, start) === marker &&
    content.slice(end, end + markerLength) === marker

  if (hasOuterMarkers) {
    replaceMarkdownRange(
      start - markerLength,
      end + markerLength,
      content.slice(start, end),
      start - markerLength,
      end - markerLength,
    )
    return
  }

  const selectedText = content.slice(start, end)
  const nextText = `${marker}${selectedText}${marker}`
  const nextStart = selectedText ? start : start + markerLength
  const nextEnd = selectedText ? end + markerLength * 2 : nextStart
  replaceMarkdownRange(start, end, nextText, nextStart, nextEnd)
}

function toggleMarkdownLinePrefix(prefix: string, markerPattern: RegExp) {
  const range = getMarkdownLineRange()
  if (!range) return

  const selectedText = localMarkdown.value.slice(range.start, range.end)
  const lines = selectedText.split('\n')
  const contentLines = lines.filter((line) => line.length > 0)
  const hasMarker =
    contentLines.length > 0 && contentLines.every((line) => markerPattern.test(line))
  const nextText = lines
    .map((line) => {
      if (line.length === 0) return hasMarker ? line : prefix

      const withoutMarker = line.replace(markerPattern, '')
      return hasMarker ? withoutMarker : `${prefix}${withoutMarker}`
    })
    .join('\n')

  replaceMarkdownRange(range.start, range.end, nextText, range.start, range.start + nextText.length)
}

function toggleMarkdownCodeBlock() {
  const selection = getMarkdownSelection()
  if (!selection) return

  const { start, end } = selection
  const selectedText = localMarkdown.value.slice(start, end)
  const fencedMatch = selectedText.match(/^```\n([\s\S]*)\n```$/)

  if (fencedMatch) {
    replaceMarkdownRange(start, end, fencedMatch[1], start, start + fencedMatch[1].length)
    return
  }

  const nextText = `\`\`\`\n${selectedText}\n\`\`\``
  const cursorOffset = selectedText ? nextText.length : 4
  replaceMarkdownRange(start, end, nextText, start + cursorOffset, start + cursorOffset)
}

function insertMarkdownTable() {
  insertMarkdownBlock(
    [
      '|  |  |  |',
      '| --- | --- | --- |',
      '|  |  |  |',
      '|  |  |  |',
    ].join('\n'),
  )
}

function editMarkdownTable(action: 'addRow' | 'addColumn' | 'deleteRow' | 'deleteColumn' | 'deleteTable') {
  const tableRange = findMarkdownTableRange()
  if (!tableRange) return

  if (action === 'deleteTable') {
    replaceMarkdownRange(tableRange.start, tableRange.end, '', tableRange.start, tableRange.start)
    return
  }

  const tableText = localMarkdown.value.slice(tableRange.start, tableRange.end)
  const lines = tableText.split('\n')
  const columnIndex = findMarkdownTableColumnIndex()
  const currentLineIndex = findCurrentMarkdownTableLineIndex(tableRange.startLine)
  let nextLines = lines

  if (action === 'addRow') {
    const columnCount = parseMarkdownTableRow(lines[0]).length
    const row = stringifyMarkdownTableRow(Array.from({ length: columnCount }, () => ''))
    const insertAt = Math.max(currentLineIndex + 1, 2)
    nextLines = [...lines.slice(0, insertAt), row, ...lines.slice(insertAt)]
  }

  if (action === 'addColumn') {
    nextLines = lines.map((line, index) => {
      const cells = parseMarkdownTableRow(line)
      const nextValue = index === 1 ? '---' : ''
      const insertAt = Math.min(columnIndex + 1, cells.length)
      return stringifyMarkdownTableRow([...cells.slice(0, insertAt), nextValue, ...cells.slice(insertAt)])
    })
  }

  if (action === 'deleteRow') {
    if (lines.length <= 2 || currentLineIndex <= 1) return
    nextLines = lines.filter((_, index) => index !== currentLineIndex)
  }

  if (action === 'deleteColumn') {
    const columnCount = parseMarkdownTableRow(lines[0]).length
    if (columnCount <= 1) return
    nextLines = lines.map((line) => {
      const cells = parseMarkdownTableRow(line)
      return stringifyMarkdownTableRow(cells.filter((_, index) => index !== columnIndex))
    })
  }

  const nextText = nextLines.join('\n')
  replaceMarkdownRange(tableRange.start, tableRange.end, nextText, tableRange.start, tableRange.start)
}

function insertMarkdownBlock(block: string) {
  const selection = getMarkdownSelection()
  if (!selection) return

  const { start, end } = selection
  const content = localMarkdown.value
  const before = start > 0 && content[start - 1] !== '\n' ? '\n\n' : ''
  const after = end < content.length && content[end] !== '\n' ? '\n\n' : ''
  const nextText = `${before}${block}${after}`
  const nextStart = start + before.length
  replaceMarkdownRange(start, end, nextText, nextStart, nextStart + block.length)
}

function getMarkdownSelection() {
  const textarea = markdownTextarea.value
  if (!textarea) return lastMarkdownSelection

  lastMarkdownSelection = {
    start: textarea.selectionStart,
    end: textarea.selectionEnd,
  }

  return lastMarkdownSelection
}

function getMarkdownLineRange() {
  const selection = getMarkdownSelection()
  if (!selection) return null

  const content = localMarkdown.value
  const start = content.lastIndexOf('\n', Math.max(selection.start - 1, 0)) + 1
  const selectedEnd =
    selection.end > selection.start && content[selection.end - 1] === '\n'
      ? selection.end - 1
      : selection.end
  const lineEnd = content.indexOf('\n', selectedEnd)
  const end = lineEnd === -1 ? content.length : lineEnd

  return { start, end }
}

function replaceMarkdownRange(
  start: number,
  end: number,
  text: string,
  selectionStart = start + text.length,
  selectionEnd = selectionStart,
) {
  localMarkdown.value = `${localMarkdown.value.slice(0, start)}${text}${localMarkdown.value.slice(end)}`
  scheduleAutoSave(localMarkdown.value)
  markdownSelectionVersion.value += 1

  void nextTick(() => {
    const textarea = markdownTextarea.value
    if (!textarea) return

    textarea.focus()
    textarea.setSelectionRange(selectionStart, selectionEnd)
    markdownSelectionVersion.value += 1
  })
}

function findMarkdownTableRange() {
  const selection = getMarkdownSelection()
  if (!selection) return null

  const content = localMarkdown.value
  const lines = content.split('\n')
  let offset = 0
  let currentLineIndex = 0

  for (const [index, line] of lines.entries()) {
    const lineEnd = offset + line.length
    if (selection.start >= offset && selection.start <= lineEnd) {
      currentLineIndex = index
      break
    }
    offset = lineEnd + 1
  }

  if (!isMarkdownTableLine(lines[currentLineIndex])) return null

  let startLine = currentLineIndex
  while (startLine > 0 && isMarkdownTableLine(lines[startLine - 1])) {
    startLine -= 1
  }

  let endLine = currentLineIndex
  while (endLine < lines.length - 1 && isMarkdownTableLine(lines[endLine + 1])) {
    endLine += 1
  }

  const tableLines = lines.slice(startLine, endLine + 1)
  if (tableLines.length < 2 || !isMarkdownTableSeparator(tableLines[1])) return null

  const start = lines.slice(0, startLine).join('\n').length + (startLine > 0 ? 1 : 0)
  const end = start + tableLines.join('\n').length
  return { start, end, startLine, endLine }
}

function findCurrentMarkdownTableLineIndex(startLine: number) {
  const selection = getMarkdownSelection()
  if (!selection) return 0

  const beforeSelection = localMarkdown.value.slice(0, selection.start)
  return beforeSelection.split('\n').length - 1 - startLine
}

function findMarkdownTableColumnIndex() {
  const selection = getMarkdownSelection()
  if (!selection) return 0

  const lineStart = localMarkdown.value.lastIndexOf('\n', Math.max(selection.start - 1, 0)) + 1
  const currentLine = localMarkdown.value.slice(lineStart, selection.start)
  return Math.max(currentLine.split('|').length - 2, 0)
}

function isMarkdownTableLine(line = '') {
  return /^\s*\|.*\|\s*$/.test(line)
}

function isMarkdownTableSeparator(line = '') {
  return /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$/.test(line)
}

function parseMarkdownTableRow(line: string) {
  return line
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map((cell) => cell.trim())
}

function stringifyMarkdownTableRow(cells: string[]) {
  return `| ${cells.join(' | ')} |`
}

function scheduleAutoSave(content: string) {
  if (!noteStore.activeNote) return

  resetSaveFeedback()
  noteStore.scheduleDraft(noteStore.activeNote.id, getSavableTitle(), content)
}

function getSavableTitle() {
  const title = localTitle.value.trim()
  if (title) return localTitle.value

  return noteStore.activeNote?.title ?? '新しいノート'
}

function showSaved() {
  savedMessage.value = true
  savedMessageTimer = setTimeout(() => {
    savedMessage.value = false
    savedMessageTimer = null
  }, 2000)
}

function resetSaveFeedback() {
  if (savedMessageTimer) {
    clearTimeout(savedMessageTimer)
    savedMessageTimer = null
  }
  savedMessage.value = false
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('ja-JP', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<style scoped>
.title-field {
  position: relative;
  flex: 1;
  min-width: 0;
}

.title-field .title-input {
  width: 100%;
}

.title-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  color: var(--text-secondary);
  font-size: 18px;
  font-weight: 700;
  pointer-events: none;
}

.title-input.is-waiting-title {
  color: transparent;
  caret-color: var(--text-primary);
}

.title-input.is-waiting-title::placeholder {
  color: transparent;
}

.loading-dots {
  display: inline-flex;
  width: 0.9em;
}

.loading-dots span {
  opacity: 0;
  animation: title-dot-appear 1.4s infinite;
}

.loading-dots span:nth-child(2) {
  animation-delay: 0.2s;
}

.loading-dots span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes title-dot-appear {
  0%, 18% {
    opacity: 0;
  }
  30%, 78% {
    opacity: 1;
  }
  90%, 100% {
    opacity: 0;
  }
}

.mode-segment {
  display: flex;
  align-items: center;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 4px;
  background-color: var(--bg-input);
  margin-right: 8px;
}

.mode-segment-btn {
  display: grid;
  place-items: center;
  width: 32px;
  height: 26px;
  color: var(--text-secondary);
  transition: background-color 0.12s, color 0.12s;
}

.mode-segment-btn + .mode-segment-btn {
  border-left: 1px solid var(--border);
}

.mode-segment-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.mode-segment-btn.is-active {
  background-color: var(--text-secondary);
  color: var(--bg-editor);
}

.ai-summary-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 28px;
  padding: 0 9px;
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-secondary);
  font-size: 12px;
}

.ai-summary-button:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.ai-summary-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.ai-summary-panel {
  display: grid;
  gap: 10px;
  max-height: 260px;
  margin: 10px 20px 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-sidebar);
  font-size: 13px;
}

.ai-summary-heading,
.ai-summary-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.ai-summary-heading span,
.ai-summary-status {
  color: var(--text-secondary);
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
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--text-primary);
  font: inherit;
  line-height: 1.6;
}

.ai-summary-actions {
  justify-content: flex-start;
}

.ai-summary-actions button {
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.prose-editor :deep(.ProseMirror) {
  box-sizing: border-box;
  width: 100%;
  max-width: var(--editor-line-max-width);
  margin: 0 auto;
  font-family: var(--editor-font-family);
  font-size: var(--editor-font-size);
  line-height: var(--editor-line-height);
}

.prose-editor :deep(.ProseMirror > *) {
  margin-top: 0;
  margin-bottom: 0;
  line-height: var(--editor-line-height);
}

.prose-editor :deep(.ProseMirror > * + *) {
  margin-top: var(--editor-paragraph-spacing);
}

.prose-editor :deep(.ProseMirror li) {
  line-height: var(--editor-line-height);
}

.markdown-textarea {
  width: 100%;
  max-width: var(--editor-line-max-width);
  height: 100%;
  min-height: 400px;
  margin: 0 auto;
  border: none;
  resize: none;
  background-color: transparent;
  color: var(--text-primary);
  font-family: var(--editor-font-family);
  font-size: var(--editor-font-size);
  line-height: calc(var(--editor-line-height) * 1em + var(--editor-paragraph-spacing) * 0.25);
  padding: 24px;
  outline: none;
}
</style>
