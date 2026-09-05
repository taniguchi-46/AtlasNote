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

          <button
            ref="aiWorkspaceToggle"
            class="icon-btn ai-workspace-toggle"
            :class="{ 'is-active': isAIWorkspaceOpen }"
            type="button"
            :title="aiWorkspaceToggleLabel"
            :aria-label="aiWorkspaceToggleLabel"
            aria-controls="ai-workspace-panel"
            :aria-pressed="isAIWorkspaceOpen"
            @click="toggleAIWorkspace"
          >
            <component :is="aiWorkspaceToggleIcon" :size="17" />
          </button>

          <button
            class="mode-segment"
            type="button"
            :title="editMode === 'markdown' ? 'リッチテキストモードに切り替え' : 'Markdownモードに切り替え'"
            :aria-label="editMode === 'markdown' ? 'リッチテキストモードに切り替え' : 'Markdownモードに切り替え'"
            @click="toggleEditMode"
          >
            <span
              class="mode-segment-option"
              :class="{ 'is-active': editMode === 'wysiwyg' }"
              aria-hidden="true"
            >
              <SquarePenIcon :size="17" />
            </span>
            <span
              class="mode-segment-option"
              :class="{ 'is-active': editMode === 'markdown' }"
              aria-hidden="true"
            >
              <SquareMIcon :size="17" />
            </span>
          </button>

          <DropdownMenuRoot>
            <DropdownMenuTrigger as-child>
              <button
                class="icon-btn"
                type="button"
                :disabled="noteExportStore.isBusy"
                :title="noteExportStore.isBusy ? 'エクスポート中...' : 'ノートをエクスポート'"
                aria-label="ノートをエクスポート"
                :aria-busy="noteExportStore.isBusy"
              >
                <DownloadIcon :size="18" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuPortal>
              <DropdownMenuContent
                class="note-export-menu"
                side="bottom"
                align="end"
                :side-offset="6"
              >
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy"
                  @select="handleExportNote('html')"
                >
                  HTMLとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy"
                  @select="handleExportNote('pdf')"
                >
                  PDFとしてエクスポート
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenuRoot>

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

      <AIWorkspace
        v-model:open="isAIWorkspaceOpen"
        @closed="focusAIWorkspaceToggle"
      >
        <div class="editor-body">
          <div
            v-if="activeAgentEditorHighlight"
            class="agent-editor-highlight-status"
            role="status"
            aria-live="polite"
          >
            <SparklesIcon :size="14" aria-hidden="true" />
            <span>Agent更新箇所</span>
            <button
              type="button"
              title="ハイライトを閉じる"
              aria-label="Agent更新箇所のハイライトを閉じる"
              @click="dismissAgentEditorHighlight"
            >
              <XIcon :size="13" aria-hidden="true" />
            </button>
          </div>
          <EditorContent v-if="editMode === 'wysiwyg'" :editor="editor" class="prose-editor" />
          <div v-else class="markdown-editor-shell">
            <div
              v-if="markdownAgentHighlight"
              ref="markdownHighlightLayer"
              class="markdown-highlight-layer"
              aria-hidden="true"
            >
              <span>{{ markdownAgentHighlight.prefix }}</span><mark
                ref="markdownHighlightMark"
                class="agent-editor-highlight-mark"
                :class="{ 'is-deletion': markdownAgentHighlight.isDeletion }"
              >{{ markdownAgentHighlight.highlighted }}</mark><span>{{ markdownAgentHighlight.suffix }}</span><span>&#8203;</span>
            </div>
            <textarea
              ref="markdownTextarea"
              v-model="localMarkdown"
              class="markdown-textarea"
              placeholder="ここにMarkdownで内容を入力してください..."
              title="Ctrl / Cmd + クリックでノートリンクを開く"
              @beforeinput="handleMarkdownBeforeInput"
              @input="handleMarkdownInput"
              @keydown="handleMarkdownKeydown"
              @scroll="syncMarkdownHighlightLayer"
              @click="handleMarkdownClick"
              @keyup="updateMarkdownSelection"
              @select="updateMarkdownSelection"
            />
          </div>
        </div>
      </AIWorkspace>

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
  DownloadIcon,
  FileTextIcon,
  Heading1Icon,
  Heading2Icon,
  Heading3Icon,
  ItalicIcon,
  ListIcon,
  ListOrderedIcon,
  PanelBottomCloseIcon,
  PanelBottomOpenIcon,
  PanelRightCloseIcon,
  PanelRightOpenIcon,
  PinIcon,
  QuoteIcon,
  Rows3Icon,
  SquareMIcon,
  SquarePenIcon,
  SparklesIcon,
  StarIcon,
  StrikethroughIcon,
  Table2Icon,
  TableColumnsSplitIcon,
  TableRowsSplitIcon,
  TerminalIcon,
  Trash2Icon,
  XIcon,
} from '@lucide/vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import { Editor, EditorContent, VueNodeViewRenderer } from '@tiptap/vue-3'
import {
  DOMParser as ProseMirrorDOMParser,
  DOMSerializer as ProseMirrorDOMSerializer,
  type Node as ProseMirrorNode,
} from '@tiptap/pm/model'
import { Plugin, PluginKey, type Selection } from '@tiptap/pm/state'
import {
  history as createRichHistoryPlugin,
  redo as redoRichHistory,
  undo as undoRichHistory,
} from '@tiptap/pm/history'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
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
import { useNoteStore } from '../stores/useNoteStore'
import { useNoteExportStore } from '../stores/useNoteExportStore'
import { useContentLockStore } from '../stores/useContentLockStore'
import { useNotificationStore } from '../stores/useNotificationStore'
import { useSettingsStore } from '../stores/useSettingsStore'
import AIWorkspace from './AIWorkspace.vue'
import NoteTags from './NoteTags.vue'
import NoteTagAddPopover from './NoteTagAddPopover.vue'
import NoteLinkPopover from './NoteLinkPopover.vue'
import NoteBacklinks from './NoteBacklinks.vue'
import { RICH_MARKDOWN_OPTIONS } from '../utils/markdownSecurity'
import { createPdfBase64FromHtml } from '../utils/noteExportDocument'
import type { NoteExportFormat, NoteExportInput } from '../api/noteExport'
import { logOperationFailure } from '../utils/operationLogger'
import {
  createTableClipboardPayload,
  createTiptapTableClipboardPayload,
  writeTableClipboard,
} from '../utils/tableClipboard'
import { serializeTiptapJsonToMarkdown } from '../utils/tiptapMarkdownSerializer'
import {
  createAgentEditorTextHighlight,
  findChangedTopLevelBlockRange,
  type AgentEditorBlockRange,
} from '../utils/agentEditorHighlight'
import {
  createNoteLinkHref,
  createNoteLinkMarkdown,
  findNoteLinkTargetAt,
  parseNoteLinkHref,
} from '../utils/noteLink'
import {
  findMatchingShortcutAction,
  isNativeHistoryShortcut,
} from '../utils/keyboardShortcuts'
import {
  createMarkdownEditHistory,
  type MarkdownEditSnapshot,
} from '../utils/markdownEditHistory'
import MermaidCodeBlockView from './MermaidCodeBlockView.vue'

const CustomTableCell = TableCell.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const CustomTableHeader = TableHeader.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const lowlight = createLowlight(common)
const agentEditorHighlightPluginKey = new PluginKey<DecorationSet>('agentEditorHighlight')

const MermaidCodeBlock = CodeBlockLowlight.extend({
  addNodeView() {
    return VueNodeViewRenderer(MermaidCodeBlockView)
  },
})

type AgentEditorHighlightPluginMeta = {
  range: AgentEditorBlockRange | null
  isDeletion: boolean
}

const noteStore = useNoteStore()
const noteExportStore = useNoteExportStore()
const contentLockStore = useContentLockStore()
const notificationStore = useNotificationStore()
const settingsStore = useSettingsStore()

const localTitle = ref('')
const savedMessage = ref(false)
const isAIWorkspaceOpen = ref(true)
const aiWorkspaceToggle = ref<HTMLButtonElement | null>(null)
const saveConflicted = computed(() => noteStore.activeDraft?.status === 'conflicted')
const conflictDetail = computed(() => {
  const conflict = noteStore.activeDraft?.conflict
  if (!conflict) return '他の更新と競合したため、ローカルの下書きを保持しています'

  return `保存元 revision ${conflict.expectedRevision} / 最新 revision ${conflict.actualRevision}`
})
const saveFailed = computed(() => noteStore.activeDraft?.status === 'failed')
const aiWorkspaceToggleLabel = computed(() => {
  const placement = settingsStore.aiWorkspacePlacement === 'right' ? '右側' : '下側'
  return `AIワークスペースを${isAIWorkspaceOpen.value ? '閉じる' : '開く'}（${placement}）`
})
const aiWorkspaceToggleIcon = computed(() => {
  if (settingsStore.aiWorkspacePlacement === 'right') {
    return isAIWorkspaceOpen.value ? PanelRightCloseIcon : PanelRightOpenIcon
  }
  return isAIWorkspaceOpen.value ? PanelBottomCloseIcon : PanelBottomOpenIcon
})
const editMode = ref<'wysiwyg' | 'markdown'>('markdown')
const localMarkdown = ref('')
const markdownTextarea = ref<HTMLTextAreaElement | null>(null)
const markdownHighlightLayer = ref<HTMLElement | null>(null)
const markdownHighlightMark = ref<HTMLElement | null>(null)
const isApplyingContent = ref(false)
const isRichDirty = ref(false)
const editorStateVersion = ref(0)
const markdownSelectionVersion = ref(0)
let lastMarkdownSelection = { start: 0, end: 0 }
let savedMessageTimer: ReturnType<typeof setTimeout> | null = null
let activeNoteId: string | null = null
let savedRichSelection: { from: number; to: number } | null = null
let markdownHighlightResizeObserver: ResizeObserver | null = null
let lastScrolledAgentHighlightKey = ''
let pendingMarkdownInput: {
  before: MarkdownEditSnapshot
  forceNewGroup: boolean
  group: string
} | null = null
const markdownEditHistory = createMarkdownEditHistory({
  content: '',
  selectionStart: 0,
  selectionEnd: 0,
})

const activeAgentEditorHighlight = computed(() => {
  const highlight = noteStore.agentEditorHighlight
  const note = noteStore.activeNote
  if (!highlight || !note) return null
  if (noteStore.activeDraft) return null
  if (highlight.noteId !== note.id || highlight.revision !== note.revision) return null
  if (localMarkdown.value !== note.content) return null
  return highlight
})

const markdownAgentHighlight = computed(() => {
  const highlight = activeAgentEditorHighlight.value
  if (!highlight) return null
  return createAgentEditorTextHighlight(localMarkdown.value, highlight)
})

const agentEditorHighlightPlugin = new Plugin<DecorationSet>({
  key: agentEditorHighlightPluginKey,
  state: {
    init: () => DecorationSet.empty,
    apply(transaction, decorations) {
      const meta = transaction.getMeta(agentEditorHighlightPluginKey) as
        | AgentEditorHighlightPluginMeta
        | undefined
      if (meta) {
        return createRichAgentDecorationSet(transaction.doc, meta.range, meta.isDeletion)
      }
      return decorations.map(transaction.mapping, transaction.doc)
    },
  },
  props: {
    decorations(state) {
      return agentEditorHighlightPluginKey.getState(state) ?? null
    },
  },
})

const editor = new Editor({
  extensions: [
    StarterKit.configure({
      codeBlock: false,
      link: false,
      undoRedo: false,
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
    MermaidCodeBlock.configure({
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
    handleKeyDown(view, event) {
      const actionId = findMatchingShortcutAction(
        event,
        settingsStore.shortcutBindings,
        'editor',
      )
      if (actionId === 'editor.undo' || actionId === 'editor.redo') {
        event.preventDefault()
        if (!event.repeat) {
          const command = actionId === 'editor.undo' ? undoRichHistory : redoRichHistory
          command(view.state, view.dispatch)
        }
        return true
      }
      if (isNativeHistoryShortcut(event)) {
        event.preventDefault()
        return true
      }
      return false
    },
  },
  onSelectionUpdate() {
    editorStateVersion.value += 1
  },
  onUpdate({ editor }) {
    editorStateVersion.value += 1

    if (editMode.value !== 'wysiwyg') return
    if (isApplyingContent.value) return

    dismissAgentEditorHighlight()
    const markdown = serializeTiptapJsonToMarkdown(editor.getJSON())
    isRichDirty.value = true

    if (localMarkdown.value !== markdown) {
      localMarkdown.value = markdown
      updateAutoTitleFromMarkdown(markdown)
      scheduleAutoSave(markdown)
    }
  },
})

editor.registerPlugin(createRichHistoryPlugin({ depth: 100, newGroupDelay: 500 }))
editor.registerPlugin(agentEditorHighlightPlugin)

watch(
  () => noteStore.activeNote,
  (note) => {
    if (!note) {
      noteStore.clearAgentEditorHighlight()
      activeNoteId = null
      savedRichSelection = null
      localTitle.value = ''
      localMarkdown.value = ''
      isRichDirty.value = false
      resetMarkdownEditHistory('')
      resetRichEditorToEmpty()
      return
    }

    const noteChanged = activeNoteId !== note.id
    if (noteChanged) {
      noteStore.clearAgentEditorHighlight()
    } else if (
      noteStore.agentEditorHighlight?.noteId === note.id
      && noteStore.agentEditorHighlight.revision !== note.revision
    ) {
      noteStore.clearAgentEditorHighlight(note.id)
    }
    activeNoteId = note.id
    const draft = noteStore.getDraft(note.id)
    const editableContent = draft?.content ?? note.content
    localTitle.value =
      draft?.title ?? (noteStore.autoTitleNoteId === note.id && extractTitleFromFirstMarkdownLine(editableContent) === ''
        ? ''
        : note.title)

    if (noteChanged) {
      savedRichSelection = null
      resetSaveFeedback()
      localMarkdown.value = editableContent
      resetMarkdownEditHistory(editableContent)
      isRichDirty.value = false
      if (editMode.value === 'wysiwyg') {
        if (!setEditorFromMarkdown(editableContent)) {
          editMode.value = 'markdown'
        }
      } else {
        resetRichEditorToEmpty()
      }
      return
    }

    if (draft || localMarkdown.value === note.content) return

    savedRichSelection = null
    localMarkdown.value = note.content
    resetMarkdownEditHistory(note.content)
    isRichDirty.value = false
    if (editMode.value === 'wysiwyg') {
      if (!setEditorFromMarkdown(note.content)) editMode.value = 'markdown'
    } else {
      resetRichEditorToEmpty()
    }
  },
  { immediate: true },
)

watch(
  () => activeAgentEditorHighlight.value?.id ?? null,
  () => {
    void nextTick(() => renderAgentEditorHighlight())
  },
  { flush: 'post' },
)

watch(editMode, () => {
  void nextTick(() => renderAgentEditorHighlight())
})

watch(
  markdownTextarea,
  (textarea) => {
    markdownHighlightResizeObserver?.disconnect()
    markdownHighlightResizeObserver = null
    if (!textarea) return

    if (typeof ResizeObserver !== 'undefined') {
      markdownHighlightResizeObserver = new ResizeObserver(syncMarkdownHighlightLayer)
      markdownHighlightResizeObserver.observe(textarea)
    }
    void nextTick(syncMarkdownHighlightLayer)
  },
  { flush: 'post' },
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
  noteStore.clearAgentEditorHighlight(activeNoteId ?? undefined)
  void noteStore.flushPendingDraft()
  markdownHighlightResizeObserver?.disconnect()
  pendingMarkdownInput = null
  markdownEditHistory.reset({ content: '', selectionStart: 0, selectionEnd: 0 })
  localMarkdown.value = ''
  localTitle.value = ''
  if (savedMessageTimer) {
    clearTimeout(savedMessageTimer)
  }
  editor.destroy()
})

const charCount = computed(() => {
  return localMarkdown.value.length
})

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

function toggleAIWorkspace() {
  isAIWorkspaceOpen.value = !isAIWorkspaceOpen.value
}

function focusAIWorkspaceToggle() {
  void nextTick(() => aiWorkspaceToggle.value?.focus())
}

async function handleExportNote(format: NoteExportFormat) {
  const selectedNote = noteStore.activeNote
  if (!selectedNote || noteExportStore.isBusy) return

  const selectedNoteId = selectedNote.id
  const result = await noteExportStore.runPrepared(async () => {
    if (noteStore.activeNote?.id !== selectedNoteId) return null

    if (editMode.value === 'wysiwyg') {
      applyRichEditorToMarkdown()
    }
    if (
      localMarkdown.value !== noteStore.activeNote.content
      || getSavableTitle() !== noteStore.activeNote.title
    ) {
      scheduleAutoSave(localMarkdown.value)
    }

    const accessAllowed = await contentLockStore.requestAccess(
      { type: 'note', id: selectedNoteId },
      noteStore.activeNote.title,
    )
    if (!accessAllowed || noteStore.activeNote?.id !== selectedNoteId) return null

    const saved = await noteStore.flushPendingDraft()
    if (!saved) {
      notificationStore.notify('未保存の変更を保存できないため、エクスポートしませんでした。', {
        kind: 'warning',
        source: 'note-export',
        code: 'NOTE_EXPORT_DRAFT_SAVE_FAILED',
      })
      return null
    }

    const current = noteStore.activeNote
    if (!current || current.id !== selectedNoteId || noteStore.getDraft(selectedNoteId)) {
      notificationStore.notify('ノートの保存状態が変わったため、エクスポートしませんでした。', {
        kind: 'warning',
        source: 'note-export',
        code: 'NOTE_EXPORT_NOTE_CHANGED',
      })
      return null
    }

    let allowPlaintextProtected = false
    if (current.protected) {
      const outputLabel = format === 'html' ? 'HTML' : 'PDF'
      allowPlaintextProtected = window.confirm(
        `このノートは保護されています。暗号化領域の外へ、復号済みの本文を平文の${outputLabel}ファイルとして保存します。続行しますか？`,
      )
      if (!allowPlaintextProtected) return null
    }

    let htmlFragment: string
    try {
      htmlFragment = parseMarkdownToRichHtml(current.content)
    } catch {
      logOperationFailure({
        noteId: selectedNoteId,
        stage: 'note-export.markdown-to-html',
        errorCategory: 'parse-failed',
      })
      notificationStore.notify('ノート本文をエクスポート用に変換できませんでした。', {
        kind: 'error',
        source: 'note-export',
        code: 'NOTE_EXPORT_RENDER_FAILED',
      })
      return null
    }

    const input: NoteExportInput = {
      noteId: current.id,
      expectedRevision: current.revision,
      title: current.title,
      markdown: current.content,
      format,
      allowPlaintextProtected,
    }
    if (format === 'html') {
      input.htmlFragment = htmlFragment
      return input
    }

    try {
      input.pdfBase64 = await createPdfBase64FromHtml(htmlFragment, current.title)
      return input
    } catch {
      logOperationFailure({
        noteId: selectedNoteId,
        stage: 'note-export.pdf-render',
        errorCategory: 'render-failed',
      })
      notificationStore.notify('PDFを生成できませんでした。', {
        kind: 'error',
        source: 'note-export',
        code: 'NOTE_EXPORT_RENDER_FAILED',
      })
      return null
    }
  })

  if (!result || result.cancelled) return
  if (result.error) {
    notificationStore.notify(result.error.message, {
      kind: result.error.retryable ? 'warning' : 'error',
      source: 'note-export',
      code: result.error.code,
      retryable: result.error.retryable,
    })
    return
  }

  notificationStore.notify(`${result.exportedName ?? 'ノート'}をエクスポートしました。`, {
    kind: 'success',
    source: 'note-export',
    code: 'NOTE_EXPORT_COMPLETED',
  })
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
  resetMarkdownEditHistory(latestNote.content)
  isRichDirty.value = false
  if (editMode.value === 'wysiwyg') {
    if (!setEditorFromMarkdown(latestNote.content)) editMode.value = 'markdown'
  } else {
    resetRichEditorToEmpty()
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
  resetMarkdownEditHistory(note.content)
  isRichDirty.value = false
  if (editMode.value === 'wysiwyg') {
    if (!setEditorFromMarkdown(note.content)) editMode.value = 'markdown'
  } else {
    resetRichEditorToEmpty()
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
    resetMarkdownEditHistory(localMarkdown.value)
    resetRichEditorToEmpty()
    return
  }

  if (
    localMarkdown.value !== noteStore.activeNote?.content
    || getSavableTitle() !== noteStore.activeNote?.title
  ) {
    scheduleAutoSave(localMarkdown.value)
  }
  if (setEditorFromMarkdown(localMarkdown.value)) {
    resetMarkdownEditHistory(localMarkdown.value)
    editMode.value = mode
  }
}

function toggleEditMode() {
  setEditMode(editMode.value === 'markdown' ? 'wysiwyg' : 'markdown')
}

defineExpose({ toggleAIWorkspace, toggleEditMode })

function replaceRichEditorContent(content: JSONContent) {
  editor.unregisterPlugin('history')
  try {
    ;(editor.commands as any).setContent(content, {
      emitUpdate: false,
    })
  } finally {
    if (!editor.isDestroyed) {
      editor.registerPlugin(createRichHistoryPlugin({ depth: 100, newGroupDelay: 500 }))
    }
  }
}

function resetRichEditorToEmpty() {
  const wasApplyingContent = isApplyingContent.value
  isApplyingContent.value = true
  try {
    replaceRichEditorContent({ type: 'doc', content: [{ type: 'paragraph' }] })
    isRichDirty.value = false
  } finally {
    isApplyingContent.value = wasApplyingContent
  }
}

function setEditorFromMarkdown(markdown: string): boolean {
  isApplyingContent.value = true
  try {
    const html = parseMarkdownToRichHtml(markdown)
    const content = parseRichHtmlToJson(html)
    replaceRichEditorContent(content)
    isRichDirty.value = false
    return true
  } catch {
    resetRichEditorToEmpty()
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

function createRichAgentDecorationSet(
  document: ProseMirrorNode,
  range: AgentEditorBlockRange | null,
  isDeletion: boolean,
): DecorationSet {
  if (!range) return DecorationSet.empty

  const decorations: Decoration[] = []
  document.forEach((node, offset, index) => {
    if (index < range.startIndex || index >= range.endIndex) return
    const usesDeletionStyle = isDeletion || range.usesDeletionAnchor
    decorations.push(Decoration.node(
      offset,
      offset + node.nodeSize,
      {
        class: usesDeletionStyle
          ? 'agent-editor-highlight-block is-deletion'
          : 'agent-editor-highlight-block',
      },
    ))
  })
  return DecorationSet.create(document, decorations)
}

function setRichAgentEditorHighlight(
  range: AgentEditorBlockRange | null,
  isDeletion = false,
) {
  if (editor.isDestroyed) return
  editor.view.dispatch(editor.state.tr.setMeta(agentEditorHighlightPluginKey, {
    range,
    isDeletion,
  } satisfies AgentEditorHighlightPluginMeta))
}

function renderAgentEditorHighlight() {
  const highlight = activeAgentEditorHighlight.value
  if (!highlight) {
    setRichAgentEditorHighlight(null)
    return
  }

  if (editMode.value === 'markdown') {
    setRichAgentEditorHighlight(null)
    syncMarkdownHighlightLayer()
    scrollToAgentEditorHighlight()
    return
  }

  try {
    const beforeDocument = parseRichHtmlToJson(parseMarkdownToRichHtml(highlight.beforeMarkdown))
    const afterDocument = editor.getJSON()
    const blockRange = findChangedTopLevelBlockRange(beforeDocument, afterDocument)
      ?? createFallbackRichAgentBlockRange(afterDocument, localMarkdown.value, highlight.start)
    setRichAgentEditorHighlight(blockRange, highlight.changeKind === 'delete')
    void nextTick(scrollToAgentEditorHighlight)
  } catch {
    setRichAgentEditorHighlight(null)
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.agent-highlight',
      errorCategory: 'parse-failed',
    })
  }
}

function createFallbackRichAgentBlockRange(
  document: JSONContent,
  markdown: string,
  markdownOffset: number,
): AgentEditorBlockRange | null {
  const blockCount = document.content?.length ?? 0
  if (blockCount === 0) return null

  const normalizedOffset = Math.min(Math.max(markdownOffset, 0), markdown.length)
  const linesBefore = markdown.slice(0, normalizedOffset).split(/\r\n|\n|\r/).length - 1
  const totalLines = Math.max(markdown.split(/\r\n|\n|\r/).length, 1)
  const index = Math.min(Math.floor((linesBefore / totalLines) * blockCount), blockCount - 1)
  return {
    startIndex: index,
    endIndex: index + 1,
    usesDeletionAnchor: false,
  }
}

function dismissAgentEditorHighlight() {
  noteStore.clearAgentEditorHighlight(noteStore.activeNote?.id)
  setRichAgentEditorHighlight(null)
}

function syncMarkdownHighlightLayer() {
  const textarea = markdownTextarea.value
  const layer = markdownHighlightLayer.value
  if (!textarea || !layer) return

  layer.style.width = `${textarea.clientWidth}px`
  layer.style.height = `${textarea.clientHeight}px`
  layer.scrollTop = textarea.scrollTop
  layer.scrollLeft = textarea.scrollLeft
}

function scrollToAgentEditorHighlight() {
  const highlight = activeAgentEditorHighlight.value
  if (!highlight) return

  const scrollKey = `${highlight.id}:${editMode.value}`
  if (lastScrolledAgentHighlightKey === scrollKey) return

  if (editMode.value === 'markdown') {
    const textarea = markdownTextarea.value
    const mark = markdownHighlightMark.value
    if (!textarea || !mark) return

    textarea.scrollTop = Math.max(0, mark.offsetTop - textarea.clientHeight * 0.35)
    syncMarkdownHighlightLayer()
  } else {
    const target = editor.view.dom.querySelector<HTMLElement>('.agent-editor-highlight-block')
    const scrollContainer = editor.view.dom.parentElement
    if (!target || !scrollContainer) return

    const targetRect = target.getBoundingClientRect()
    const containerRect = scrollContainer.getBoundingClientRect()
    scrollContainer.scrollTop += targetRect.top - containerRect.top - scrollContainer.clientHeight * 0.35
  }

  lastScrolledAgentHighlightKey = scrollKey
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

function createMarkdownSnapshot(
  content = localMarkdown.value,
  textarea = markdownTextarea.value,
): MarkdownEditSnapshot {
  return {
    content,
    selectionStart: textarea?.selectionStart ?? 0,
    selectionEnd: textarea?.selectionEnd ?? 0,
  }
}

function resetMarkdownEditHistory(content: string) {
  pendingMarkdownInput = null
  lastMarkdownSelection = { start: 0, end: 0 }
  markdownEditHistory.reset({ content, selectionStart: 0, selectionEnd: 0 })
}

function getMarkdownInputRecordOptions(inputType: string) {
  if (inputType === 'insertText' || inputType === 'insertCompositionText') {
    return { group: 'insert-text', forceNewGroup: false }
  }
  if (inputType === 'deleteContentBackward' || inputType === 'deleteContentForward') {
    return { group: inputType, forceNewGroup: false }
  }
  return { group: inputType || 'input', forceNewGroup: true }
}

function handleMarkdownBeforeInput(event: InputEvent) {
  if (event.inputType === 'historyUndo' || event.inputType === 'historyRedo') {
    if (event.cancelable) {
      event.preventDefault()
      pendingMarkdownInput = null
      applyMarkdownHistory(event.inputType === 'historyUndo' ? 'undo' : 'redo')
    }
    return
  }

  const textarea = event.currentTarget as HTMLTextAreaElement
  pendingMarkdownInput = {
    before: createMarkdownSnapshot(textarea.value, textarea),
    ...getMarkdownInputRecordOptions(event.inputType),
  }
}

function handleMarkdownKeydown(event: KeyboardEvent) {
  const actionId = findMatchingShortcutAction(
    event,
    settingsStore.shortcutBindings,
    'editor',
  )
  if (actionId === 'editor.undo' || actionId === 'editor.redo') {
    event.preventDefault()
    event.stopPropagation()
    if (!event.repeat) applyMarkdownHistory(actionId === 'editor.undo' ? 'undo' : 'redo')
    return
  }

  if (isNativeHistoryShortcut(event)) {
    event.preventDefault()
    event.stopPropagation()
  }
}

function applyMarkdownHistory(action: 'undo' | 'redo') {
  const snapshot = action === 'undo'
    ? markdownEditHistory.undo()
    : markdownEditHistory.redo()
  if (!snapshot) return

  pendingMarkdownInput = null
  dismissAgentEditorHighlight()
  localMarkdown.value = snapshot.content
  lastMarkdownSelection = {
    start: snapshot.selectionStart,
    end: snapshot.selectionEnd,
  }
  updateAutoTitleFromMarkdown(snapshot.content)
  scheduleAutoSave(snapshot.content)
  markdownSelectionVersion.value += 1

  const noteId = activeNoteId
  void nextTick(() => {
    if (activeNoteId !== noteId) return
    const textarea = markdownTextarea.value
    if (!textarea) return
    textarea.focus()
    textarea.setSelectionRange(snapshot.selectionStart, snapshot.selectionEnd)
    markdownSelectionVersion.value += 1
  })
}

function handleMarkdownInput(event: Event) {
  const textarea = event.currentTarget as HTMLTextAreaElement
  const after = createMarkdownSnapshot(textarea.value, textarea)
  const pending = pendingMarkdownInput
  pendingMarkdownInput = null
  markdownEditHistory.record(
    pending?.before ?? markdownEditHistory.current(),
    after,
    pending
      ? { group: pending.group, forceNewGroup: pending.forceNewGroup }
      : { group: 'input', forceNewGroup: true },
  )
  localMarkdown.value = textarea.value
  dismissAgentEditorHighlight()
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
  const before = createMarkdownSnapshot()
  dismissAgentEditorHighlight()
  const nextContent = `${localMarkdown.value.slice(0, start)}${text}${localMarkdown.value.slice(end)}`
  localMarkdown.value = nextContent
  markdownEditHistory.record(
    before,
    {
      content: nextContent,
      selectionStart,
      selectionEnd,
    },
    { group: 'command', forceNewGroup: true },
  )
  lastMarkdownSelection = { start: selectionStart, end: selectionEnd }
  updateAutoTitleFromMarkdown(nextContent)
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
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 4px;
  background-color: var(--bg-input);
  margin-right: 8px;
  cursor: pointer;
}

.mode-segment:focus-visible {
  outline: 2px solid var(--brand-primary);
  outline-offset: 2px;
}

.mode-segment-option {
  display: grid;
  place-items: center;
  width: 32px;
  height: 26px;
  color: var(--text-secondary);
  transition: background-color 0.12s, color 0.12s;
}

.mode-segment-option + .mode-segment-option {
  border-left: 1px solid var(--border);
}

.mode-segment:hover .mode-segment-option:not(.is-active) {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.mode-segment-option.is-active {
  background-color: var(--text-secondary);
  color: var(--bg-editor);
}

.ai-workspace-toggle.is-active {
  background: var(--bg-active);
  color: var(--brand-primary);
}

:global(.note-export-menu) {
  z-index: 1100;
  min-width: 210px;
  padding: 5px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor, #fff);
  box-shadow: 0 10px 24px rgba(15, 23, 42, .18);
  color: var(--text-primary);
}

:global(.note-export-menu-item) {
  display: flex;
  min-height: 32px;
  align-items: center;
  padding: 2px 8px;
  border-radius: 5px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.25;
  outline: none;
}

:global(.note-export-menu-item[data-highlighted]) {
  background: var(--bg-hover);
  color: var(--brand-primary);
}

:global(.note-export-menu-item[data-disabled]) {
  cursor: not-allowed;
  opacity: .45;
}

.editor-body {
  position: relative;
}

.agent-editor-highlight-status {
  position: absolute;
  top: 12px;
  right: 16px;
  z-index: 5;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 4px 6px 4px 9px;
  border: 1px solid color-mix(in srgb, var(--color-success) 35%, var(--border));
  border-radius: 7px;
  background: color-mix(in srgb, var(--bg-editor) 88%, var(--color-success) 12%);
  box-shadow: 0 4px 14px color-mix(in srgb, var(--text-primary) 10%, transparent);
  color: color-mix(in srgb, var(--color-success) 70%, var(--text-primary) 30%);
  font-size: 12px;
  font-weight: 600;
}

.agent-editor-highlight-status button {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  color: currentColor;
}

.agent-editor-highlight-status button:hover {
  background: color-mix(in srgb, var(--color-success) 12%, transparent);
}

.agent-editor-highlight-status button:focus-visible {
  outline: 2px solid var(--color-success);
  outline-offset: 1px;
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

.prose-editor :deep(.agent-editor-highlight-block) {
  position: relative;
  border-radius: 3px;
  background: color-mix(in srgb, var(--bg-editor) 95%, var(--color-success) 5%);
}

.prose-editor :deep(.agent-editor-highlight-block)::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: -10px;
  width: 2px;
  border-radius: 1px;
  background: color-mix(in srgb, var(--color-success) 78%, transparent);
}

.prose-editor :deep(.agent-editor-highlight-block.is-deletion) {
  background: color-mix(in srgb, var(--bg-editor) 97%, var(--color-success) 3%);
  outline: 1px dashed color-mix(in srgb, var(--color-success) 30%, transparent);
  outline-offset: -1px;
}

.markdown-editor-shell {
  position: relative;
  flex: 1;
  width: 100%;
  max-width: var(--editor-line-max-width);
  min-height: 400px;
  margin: 0 auto;
  overflow: hidden;
}

.markdown-highlight-layer,
.markdown-textarea {
  box-sizing: border-box;
  position: absolute;
  inset: 0;
  width: 100%;
  max-width: none;
  height: 100%;
  min-height: 400px;
  margin: 0;
  border: none;
  font-family: var(--editor-font-family);
  font-size: var(--editor-font-size);
  line-height: calc(var(--editor-line-height) * 1em + var(--editor-paragraph-spacing) * 0.25);
  white-space: pre-wrap;
  overflow-wrap: break-word;
  word-break: break-word;
  tab-size: 4;
  padding: 24px;
}

.markdown-highlight-layer {
  z-index: 0;
  overflow: hidden;
  color: transparent;
  pointer-events: none;
}

.agent-editor-highlight-mark {
  position: relative;
  border-radius: 3px;
  background: color-mix(in srgb, var(--bg-editor) 94%, var(--color-success) 6%);
  color: transparent;
  box-decoration-break: clone;
  -webkit-box-decoration-break: clone;
}

.agent-editor-highlight-mark:not(.is-deletion)::before {
  content: '';
  position: absolute;
  top: -0.1em;
  bottom: -0.1em;
  left: -10px;
  width: 2px;
  border-radius: 1px;
  background: color-mix(in srgb, var(--color-success) 78%, transparent);
}

.agent-editor-highlight-mark.is-deletion {
  background: transparent;
}

.agent-editor-highlight-mark.is-deletion::before {
  content: '';
  position: absolute;
  top: -0.1em;
  left: -10px;
  width: 2px;
  height: 1.2em;
  border-radius: 1px;
  background: color-mix(in srgb, var(--color-success) 78%, transparent);
}

.markdown-textarea {
  z-index: 1;
  resize: none;
  background-color: transparent;
  color: var(--text-primary);
  caret-color: var(--text-primary);
  outline: none;
}
</style>
