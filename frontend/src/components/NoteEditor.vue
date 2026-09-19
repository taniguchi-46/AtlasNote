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
            :disabled="isEditorInputLocked"
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
              :disabled="isEditorInputLocked || noteStore.isLoading"
              @click="handleReloadConflict"
            >
              {{ noteStore.isLoading ? '再読込中...' : '最新版を再読込' }}
            </button>
            <button
              type="button"
              :disabled="isEditorInputLocked"
              @click="handleCopyConflict"
            >コピー保存</button>
          </div>
          <div
            v-else-if="saveFailed"
            class="save-error-indicator"
            role="status"
          >
            <span>保存失敗</span>
            <button
              type="button"
              :disabled="isEditorInputLocked"
              @click="handleRetrySave"
            >再試行</button>
            <button
              type="button"
              :disabled="isEditorInputLocked"
              @click="handleDiscardDraft"
            >破棄</button>
          </div>
          <span v-else-if="savedMessage" class="saved-indicator">保存済み</span>
          <div
            v-if="imagePasteError"
            class="attachment-paste-indicator"
            role="status"
            aria-live="polite"
          >
            <span>{{ imagePasteError }}</span>
            <button
              v-if="pendingImagePaste"
              type="button"
              :disabled="isEditorInputLocked || pendingImagePaste?.noteId !== noteStore.activeNote.id"
              @click="retryImagePaste"
            >再試行</button>
            <button type="button" @click="discardPendingImagePaste">閉じる</button>
          </div>

          <button
            v-if="settingsStore.aiEnabled"
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
            :disabled="isEditorInputLocked"
            type="button"
            title="モード切り替え"
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
                :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                :title="noteExportStore.isBusy || isAttachmentExporting ? 'エクスポート中...' : 'ノートをエクスポート'"
                aria-label="ノートをエクスポート"
                :aria-busy="noteExportStore.isBusy || isAttachmentExporting"
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
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportNote('html')"
                >
                  HTMLとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportNote('pdf')"
                >
                  PDFとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportNote('json')"
                >
                  JSONとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportNote('csv')"
                >
                  CSVとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportNote('txt')"
                >
                  TXTとしてエクスポート
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="note-export-menu-item"
                  :disabled="noteExportStore.isBusy || isAttachmentExporting || isEditorInputLocked"
                  @select="handleExportAttachments"
                >
                  添付ファイルをZIPで保存
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenuRoot>

          <button
            class="icon-btn"
            :disabled="isEditorInputLocked"
            type="button"
            :title="noteStore.activeNote.isFavorite ? 'お気に入りを外す' : 'お気に入りに追加'"
            @click="noteStore.toggleFavorite(noteStore.activeNote.id)"
          >
            <StarIcon :size="18" :class="{ filled: noteStore.activeNote.isFavorite }" />
          </button>
          <button
            class="icon-btn"
            :disabled="isEditorInputLocked"
            type="button"
            :title="noteStore.activeNote.isPinned ? 'ピン留めを外す' : 'ピン留め'"
            @click="noteStore.togglePinned(noteStore.activeNote.id)"
          >
            <PinIcon :size="18" :class="{ filled: noteStore.activeNote.isPinned }" />
          </button>
          <button
            class="icon-btn danger"
            :disabled="isEditorInputLocked"
            type="button"
            title="ゴミ箱へ移動"
            @click="handleTrashActiveNote"
          >
            <Trash2Icon :size="18" />
          </button>
        </div>
      </div>

      <div class="editor-format-bar" @mousedown.prevent>
        <fieldset
          class="editor-format-controls"
          :disabled="isEditorInputLocked"
        >
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
          :class="{ 'is-active': editMode === 'wysiwyg' && isOrdinaryCodeBlockActive() }"
          type="button"
          title="コードブロック"
          @click="toggleCodeBlock"
        >
          <TerminalIcon :size="15" />
        </button>

        <button
          class="format-btn"
          :class="{ 'is-active': editMode === 'wysiwyg' && editor?.isActive('horizontalRule') }"
          type="button"
          title="水平線"
          @click="toggleHorizontalRule"
        >
          <MinusIcon :size="15" />
        </button>

        <NoteLinkPopover
          v-if="noteStore.activeNote"
          :note-id="noteStore.activeNote.id"
          :disabled="noteStore.activeNote.isTrashed || isEditorInputLocked"
          @opened="rememberRichSelection"
          @select="insertNoteLink"
        />

        <span class="format-divider" />

        <MermaidInsertPopover
          :disabled="isEditorInputLocked || noteStore.activeNote?.isTrashed"
          @select="insertMermaidDiagram"
        />

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
        </fieldset>
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
          <EditorContent
            v-if="editMode === 'wysiwyg'"
            :editor="editor"
            class="prose-editor"
            @dragover="handleEditorDragOver"
            @drop="handleRichDrop"
          />
          <div
            v-if="mermaidCommandOpen"
            class="mermaid-command-menu"
            :style="mermaidCommandStyle"
            role="listbox"
            aria-label="Mermaid図の候補"
          >
            <button
              v-for="(diagram, index) in mermaidCommandSuggestions"
              :key="diagram.type"
              type="button"
              class="mermaid-command-item"
              :class="{ 'is-selected': index === mermaidCommandIndex }"
              role="option"
              :aria-selected="index === mermaidCommandIndex"
              @mousedown.prevent
              @click="selectMermaidCommand(diagram.type)"
            >
              <WorkflowIcon :size="15" aria-hidden="true" />
              <span>{{ diagram.label }}</span>
            </button>
          </div>
          <div v-if="editMode === 'markdown'" class="markdown-editor-shell">
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
              :disabled="isEditorInputLocked"
              placeholder="ここにMarkdownで内容を入力してください..."
              title="Ctrl / Cmd + クリックでノートリンクを開く"
              @beforeinput="handleMarkdownBeforeInput"
              @input="handleMarkdownInput"
              @keydown="handleMarkdownKeydown"
              @compositionstart="handleMarkdownCompositionStart"
              @compositionend="handleMarkdownCompositionEnd"
              @paste="handleMarkdownPaste"
              @dragover="handleEditorDragOver"
              @drop="handleMarkdownDrop"
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
            :disabled="noteStore.activeNote.isTrashed || isEditorInputLocked"
          />
          <NoteTags
            :note-id="noteStore.activeNote.id"
            :disabled="noteStore.activeNote.isTrashed || isEditorInputLocked"
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
import { textblockTypeInputRule, type JSONContent } from '@tiptap/core'
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
  MinusIcon,
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
  WorkflowIcon,
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
import { NodeSelection, Plugin, PluginKey, TextSelection, type Selection } from '@tiptap/pm/state'
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
import { createPdfBase64FromHtml, createPlainTextFromHtml } from '../utils/noteExportDocument'
import type { NoteExportFormat, NoteExportInput } from '../api/noteExport'
import {
  saveNoteAttachment,
  saveNoteAttachments,
  type NoteAttachment,
} from '../api/attachments'
import {
  encodeBase64,
  getClipboardImage,
  readClipboardImage,
  type ClipboardImagePayload,
} from '../utils/attachmentClipboard'
import { createAttachmentObjectURL } from '../utils/attachmentImage'
import { isManagedAttachmentReference } from '../utils/attachmentReference'
import {
  createImageWidthTitle,
  parseImageWidth,
  parseImageWidthFromTitle,
} from '../utils/imageResize'
import { logOperationFailure } from '../utils/operationLogger'
import {
  createTableClipboardPayload,
  createTiptapTableClipboardPayload,
  writeTableClipboard,
} from '../utils/tableClipboard'
import {
  restoreSerializedEmptyParagraphs,
  serializeTiptapJsonToMarkdown,
} from '../utils/tiptapMarkdownSerializer'
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
import {
  continueMarkdownList,
  createMarkdownLineBreakTracker,
} from '../utils/markdownListContinuation'
import { createMermaidFence } from '../utils/mermaidClipboard'
import { insertMermaidCodeBlock } from '../utils/mermaidInsertion'
import {
  getMermaidDiagramSuggestions,
  getMermaidDiagramDefinition,
  type MermaidDiagramType,
} from '../utils/mermaidVisualEditor'
import {
  flushMermaidEditorInputs,
  setMermaidEditorInputsLocked,
  type MermaidEditorInputFlusher,
  type MermaidEditorSessionStorage,
} from '../utils/mermaidEditorSession'
import { handleMermaidPaste } from '../utils/mermaidPaste'
import MermaidCodeBlockView from './MermaidCodeBlockView.vue'
import MermaidInsertPopover from './MermaidInsertPopover.vue'

const CustomTableCell = TableCell.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const CustomTableHeader = TableHeader.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const lowlight = createLowlight(common)
const agentEditorHighlightPluginKey = new PluginKey<DecorationSet>('agentEditorHighlight')

const MermaidCodeBlock = CodeBlockLowlight.extend({
  addStorage() {
    return {
      ...this.parent?.(),
      noteId: null as string | null,
      generation: 0,
      mermaidEditorFlushers: new Set<MermaidEditorInputFlusher>(),
      mermaidEditorInputLockers: new Set(),
      openMermaidEditorOnSelect: false,
    }
  },
  addNodeView() {
    return VueNodeViewRenderer(MermaidCodeBlockView)
  },
  addInputRules() {
    return [
      textblockTypeInputRule({
        find: /^(?:```|~~~)mermaid[\s\n]$/i,
        type: this.type,
        getAttributes: () => ({ language: 'mermaid' }),
      }),
      ...(this.parent?.() ?? []),
    ]
  },
})

const ManagedImage = Image.extend({
  addAttributes() {
    return {
      ...(this.parent?.() ?? {}),
      width: {
        default: null,
        parseHTML: (element) => parseImageWidthFromTitle(element.getAttribute('title')),
        renderHTML: (attributes) => {
          const title = createImageWidthTitle(parseImageWidth(attributes.width))
          return title ? { title } : {}
        },
      },
    }
  },
  addNodeView() {
    return ({ node, editor, getPos }) => {
      let currentNode = node
      let objectURL: string | null = null
      let loadGeneration = 0
      let resizeStart: { pointerId: number; startX: number; startWidth: number } | null = null
      const wrapper = document.createElement('span')
      wrapper.className = 'note-attachment-image-frame'
      wrapper.contentEditable = 'false'
      const dom = document.createElement('img')
      dom.className = 'note-attachment-image'
      dom.draggable = false
      const resizeHandle = document.createElement('button')
      resizeHandle.type = 'button'
      resizeHandle.className = 'visual-resize-handle'
      resizeHandle.setAttribute('aria-label', '画像のサイズを変更')
      resizeHandle.title = '右下をドラッグして画像サイズを変更'
      wrapper.append(dom, resizeHandle)

      const revokeObjectURL = () => {
        if (!objectURL) return
        URL.revokeObjectURL(objectURL)
        objectURL = null
      }

      const applyAttributes = (nextNode: ProseMirrorNode) => {
        const alt = typeof nextNode.attrs.alt === 'string' ? nextNode.attrs.alt : ''
        if (alt) dom.alt = alt
        else dom.removeAttribute('alt')
        const width = parseImageWidth(nextNode.attrs.width)
        if (width === null) dom.style.removeProperty('width')
        else dom.style.width = `${width}px`
        dom.removeAttribute('title')
      }

      const updateWidth = (width: number) => {
        const normalized = parseImageWidth(width)
        if (normalized === null || editor.isDestroyed) return
        let position: number | undefined
        try {
          position = getPos()
        } catch {
          return
        }
        if (position === undefined) return
        const nodeAtPosition = editor.state.doc.nodeAt(position)
        if (!nodeAtPosition || nodeAtPosition.type !== currentNode.type) return
        editor.view.dispatch(editor.state.tr.setNodeMarkup(position, undefined, {
          ...nodeAtPosition.attrs,
          width: normalized,
          title: createImageWidthTitle(normalized),
        }))
      }

      const stopResize = (event: PointerEvent) => {
        if (!resizeStart || resizeStart.pointerId !== event.pointerId) return
        resizeHandle.releasePointerCapture?.(event.pointerId)
        resizeStart = null
      }

      const handleResizeMove = (event: PointerEvent) => {
        if (!resizeStart || resizeStart.pointerId !== event.pointerId) return
        event.preventDefault()
        updateWidth(resizeStart.startWidth + event.clientX - resizeStart.startX)
      }

      const handleResizeStart = (event: PointerEvent) => {
        if (!editor.isEditable || event.button !== 0) return
        event.preventDefault()
        event.stopPropagation()
        const renderedWidth = dom.getBoundingClientRect().width
        if (!Number.isFinite(renderedWidth) || renderedWidth <= 0) return
        resizeStart = { pointerId: event.pointerId, startX: event.clientX, startWidth: renderedWidth }
        resizeHandle.setPointerCapture?.(event.pointerId)
      }

      resizeHandle.addEventListener('pointerdown', handleResizeStart)
      resizeHandle.addEventListener('pointermove', handleResizeMove)
      resizeHandle.addEventListener('pointerup', stopResize)
      resizeHandle.addEventListener('pointercancel', stopResize)

      const loadSource = (source: string) => {
        const generation = ++loadGeneration
        revokeObjectURL()
        if (!source) {
          dom.removeAttribute('src')
          return
        }
        if (!isManagedAttachmentReference(source)) {
          dom.src = source
          return
        }

        dom.removeAttribute('src')
        void createAttachmentObjectURL(source, noteStore.activeNote?.id).then((nextObjectURL) => {
          if (generation !== loadGeneration) {
            if (nextObjectURL) URL.revokeObjectURL(nextObjectURL)
            return
          }
          if (!nextObjectURL) return
          objectURL = nextObjectURL
          dom.src = nextObjectURL
        }).catch(() => {
          if (generation === loadGeneration) dom.removeAttribute('src')
        })
      }

      applyAttributes(node)
      loadSource(typeof node.attrs.src === 'string' ? node.attrs.src : '')

      return {
        dom: wrapper,
        update(nextNode: ProseMirrorNode) {
          if (nextNode.type !== currentNode.type) return false
          const sourceChanged = nextNode.attrs.src !== currentNode.attrs.src
          currentNode = nextNode
          applyAttributes(nextNode)
          if (sourceChanged) {
            loadSource(typeof nextNode.attrs.src === 'string' ? nextNode.attrs.src : '')
          }
          return true
        },
        selectNode() {
          wrapper.classList.add('ProseMirror-selectednode')
        },
        deselectNode() {
          wrapper.classList.remove('ProseMirror-selectednode')
        },
        destroy() {
          loadGeneration += 1
          revokeObjectURL()
          resizeHandle.removeEventListener('pointerdown', handleResizeStart)
          resizeHandle.removeEventListener('pointermove', handleResizeMove)
          resizeHandle.removeEventListener('pointerup', stopResize)
          resizeHandle.removeEventListener('pointercancel', stopResize)
        },
      }
    }
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
const isAttachmentExporting = ref(false)
const imagePasteError = ref('')
const isAIWorkspaceOpen = ref(true)
const aiWorkspaceToggle = ref<HTMLButtonElement | null>(null)
const saveConflicted = computed(() => noteStore.activeDraft?.status === 'conflicted')
const conflictDetail = computed(() => {
  const conflict = noteStore.activeDraft?.conflict
  if (!conflict) return '他の更新と競合したため、ローカルの下書きを保持しています'

  return `保存元 revision ${conflict.expectedRevision} / 最新 revision ${conflict.actualRevision}`
})
const saveFailed = computed(() => noteStore.activeDraft?.status === 'failed')
const isActiveNoteDeletionPreparing = computed(() => {
  const noteId = noteStore.activeNote?.id
  return noteId ? noteStore.isNoteDeletionPreparing(noteId) : false
})
const isContentLockPending = ref(false)
const isEditorInputLocked = computed(() => (
  isActiveNoteDeletionPreparing.value || isContentLockPending.value
))
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
const mermaidCommandOpen = ref(false)
const mermaidCommandQuery = ref('')
const mermaidCommandIndex = ref(0)
const mermaidCommandRange = ref<{ start: number; end: number } | { from: number; to: number } | null>(null)
const mermaidCommandSuggestions = computed(() => getMermaidDiagramSuggestions(mermaidCommandQuery.value).slice(0, 8))
const mermaidCommandStyle = computed(() => {
  editorStateVersion.value
  if (editMode.value === 'wysiwyg') {
    const editorElement = editor.view.dom
    const body = editorElement.closest('.editor-body')
    if (body) {
      const coords = editor.view.coordsAtPos(editor.state.selection.from)
      const bodyRect = body.getBoundingClientRect()
      return {
        top: `${coords.bottom - bodyRect.top + 8}px`,
        left: `${coords.left - bodyRect.left}px`,
      }
    }
    return undefined
  }
  if (!mermaidCommandRange.value || !('start' in mermaidCommandRange.value)) {
    return undefined
  }
  const lineStart = localMarkdown.value.lastIndexOf('\n', mermaidCommandRange.value.start - 1) + 1
  const lineIndex = localMarkdown.value.slice(0, lineStart).split('\n').length - 1
  const scrollTop = markdownTextarea.value?.scrollTop ?? 0
  return { top: `${24 + lineIndex * 24 - scrollTop}px` }
})
let imagePasteGeneration = 0
let lastMarkdownSelection = { start: 0, end: 0 }
let savedMessageTimer: ReturnType<typeof setTimeout> | null = null
let activeNoteId: string | null = null
type PendingImagePaste = {
  noteId: string
  payload: ClipboardImagePayload
  attachment: NoteAttachment | null
}
type RichImagePasteContext = {
  noteId: string
  doc: ProseMirrorNode
  from: number
  to: number
  generation: number
}
type MarkdownImagePasteContext = {
  noteId: string
  content: string
  start: number
  end: number
  generation: number
}
type MarkdownDropPosition = {
  clientX: number
  clientY: number
}
type DroppedImageFile = {
  file: File
  mimeHint: 'image/png' | 'image/jpeg'
}

class StaleImagePasteError extends Error {
  constructor() {
    super('stale-image-paste')
    this.name = 'StaleImagePasteError'
  }
}

const pendingImagePaste = ref<PendingImagePaste | null>(null)
const handledDropEvents = new WeakSet<DragEvent>()
let savedRichSelection: { from: number; to: number } | null = null
let markdownHighlightResizeObserver: ResizeObserver | null = null
let lastScrolledAgentHighlightKey = ''
let isMarkdownComposing = false
const markdownLineBreakTracker = createMarkdownLineBreakTracker()
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

const editor: Editor = new Editor({
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
    ManagedImage,
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
    clipboardTextSerializer(content, view) {
      const selection = view.state.selection
      const mermaidSource = findRichMermaidSource(selection)
      if (mermaidSource !== null && isSelectionInsideMermaid(selection)) {
        return createMermaidFence(mermaidSource)
      }

      if (selectionContainsMermaid(selection)) {
        return serializeTiptapJsonToMarkdown({
          type: 'doc',
          content: content.content.toJSON() as JSONContent[],
        })
      }

      const table = findRichTableNode(selection)
      if (table) return createTiptapTableClipboardPayload(table).plainText

      return ''
    },
    handlePaste(view, event): boolean {
      const clipboardImage = getClipboardImage(event)
      if (clipboardImage) {
        event.preventDefault()
        void handleRichImagePaste(clipboardImage)
        return true
      }

      return handleMermaidPaste({
        editor,
        view,
        event,
        parseMarkdown: (markdown) => editor.schema.nodeFromJSON(
          parseRichHtmlToJson(parseMarkdownToRichHtml(markdown)),
        ),
        onError: (error) => {
          logOperationFailure({
            noteId: noteStore.activeNote?.id,
            stage: 'note-editor.mermaid-paste',
            errorCategory: error instanceof Error ? error.name : 'paste-failed',
          })
        },
      })
    },
    handleDrop(_view, event): boolean {
      if (!hasDroppedFiles(event)) return false
      handleRichDrop(event)
      return true
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
      if (handleMermaidCommandKeydown(event, 'rich')) return true
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
    invalidateImagePasteOperations()
    editorStateVersion.value += 1
    updateMermaidCommandState('rich')
  },
  onUpdate({ editor }) {
    invalidateImagePasteOperations()
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
    updateMermaidCommandState('rich')
  },
})

editor.registerPlugin(createRichHistoryPlugin({ depth: 100, newGroupDelay: 500 }))
editor.registerPlugin(agentEditorHighlightPlugin)

function updateMermaidEditorContext(noteId: string | null) {
  if (editor.isDestroyed) return

  const storage = (editor.storage as {
    codeBlock?: {
      noteId?: string | null
      generation?: number
    }
  }).codeBlock
  if (!storage) return

  storage.noteId = noteId
  storage.generation = (storage.generation ?? 0) + 1
}

watch(
  () => noteStore.activeNote,
  (note) => {
    invalidateImagePasteOperations()
    if (!note) {
      updateMermaidEditorContext(null)
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
  () => noteStore.discardedDraftsVersion,
  () => {
    const note = noteStore.activeNote
    if (!note || noteStore.getDraft(note.id)) return

    invalidateImagePasteOperations()
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
  },
)

watch(
  () => activeAgentEditorHighlight.value?.id ?? null,
  () => {
    void nextTick(() => renderAgentEditorHighlight())
  },
  { flush: 'post' },
)

watch(editMode, () => {
  invalidateImagePasteOperations()
  void nextTick(() => renderAgentEditorHighlight())
})

watch(
  isEditorInputLocked,
  () => invalidateImagePasteOperations(),
  { flush: 'sync' },
)

watch(
  isActiveNoteDeletionPreparing,
  (preparing) => {
    if (!editor.isDestroyed) editor.setEditable(!preparing && !isContentLockPending.value)
    if (!preparing || !noteStore.activeNote) return

    // A list or sidebar delete can start while the editor still owns the
    // latest input event. Snapshot it synchronously before the store flushes.
    if (editMode.value === 'wysiwyg') {
      applyRichEditorToMarkdown()
    } else {
      updateMarkdownSelection()
      scheduleAutoSave(localMarkdown.value)
      markdownTextarea.value?.blur()
    }
  },
  { immediate: true, flush: 'sync' },
)

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
  invalidateImagePasteOperations()
  noteStore.clearAgentEditorHighlight(activeNoteId ?? undefined)
  void noteStore.flushPendingDraft({ mode: 'automatic' })
  markdownHighlightResizeObserver?.disconnect()
  isMarkdownComposing = false
  markdownLineBreakTracker.reset()
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
      void noteStore.flushPendingDraft({ mode: 'automatic' })
    }
    return
  }

  scheduleAutoSave(localMarkdown.value)
  void noteStore.flushPendingDraft({ mode: 'automatic' })
}

function handleTitleInput() {
  disableAutoTitleFromContent()
  scheduleAutoSave(localMarkdown.value)
}

async function handleTrashActiveNote() {
  const note = noteStore.activeNote
  if (!note || isActiveNoteDeletionPreparing.value) return

  if (editMode.value === 'wysiwyg') {
    applyRichEditorToMarkdown()
  } else {
    updateMarkdownSelection()
    scheduleAutoSave(localMarkdown.value)
    markdownTextarea.value?.blur()
  }

  await nextTick()
  await noteStore.trashNote(note.id)
}

function toggleAIWorkspace() {
  if (!settingsStore.aiEnabled) return
  isAIWorkspaceOpen.value = !isAIWorkspaceOpen.value
}

function focusAIWorkspaceToggle() {
  void nextTick(() => aiWorkspaceToggle.value?.focus())
}

async function handleExportNote(format: NoteExportFormat) {
  const selectedNote = noteStore.activeNote
  if (!selectedNote || noteExportStore.isBusy || isAttachmentExporting.value) return

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

    const saved = await noteStore.flushPendingDraft({ mode: 'required' })
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
      const outputLabel = {
        html: 'HTML',
        pdf: 'PDF',
        json: 'JSON',
        csv: 'CSV',
        txt: 'TXT',
      }[format]
      allowPlaintextProtected = window.confirm(
        `このノートは保護されています。暗号化領域の外へ、復号済みの本文を平文の${outputLabel}ファイルとして保存します。続行しますか？`,
      )
      if (!allowPlaintextProtected) return null
    }

    const input: NoteExportInput = {
      noteId: current.id,
      expectedRevision: current.revision,
      title: current.title,
      markdown: current.content,
      format,
      allowPlaintextProtected,
    }
    if (format === 'json' || format === 'csv') return input

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

    if (format === 'html') {
      input.htmlFragment = htmlFragment
      return input
    }

    if (format === 'txt') {
      input.textContent = createPlainTextFromHtml(htmlFragment)
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

function invalidateImagePasteOperations() {
  imagePasteGeneration += 1
}

function captureRichImagePasteContext(dropPosition?: number): RichImagePasteContext | null {
  const noteId = noteStore.activeNote?.id
  if (!noteId || isEditorInputLocked.value || editor.isDestroyed || editMode.value !== 'wysiwyg') {
    return null
  }

  if (typeof dropPosition === 'number') {
    try {
      const selection = TextSelection.near(editor.state.doc.resolve(dropPosition))
      if (!editor.state.selection.eq(selection)) {
        editor.view.dispatch(editor.state.tr.setSelection(selection))
      }
    } catch {
      // Fall back to the current selection when the browser reports an
      // unusable drop coordinate.
    }
  }

  invalidateImagePasteOperations()

  return {
    noteId,
    doc: editor.state.doc,
    from: editor.state.selection.from,
    to: editor.state.selection.to,
    generation: imagePasteGeneration,
  }
}

function getMarkdownDropOffset(
  textarea: HTMLTextAreaElement,
  content: string,
  position: MarkdownDropPosition,
): number | null {
  if (
    typeof document === 'undefined'
    || typeof window === 'undefined'
    || !Number.isFinite(position.clientX)
    || !Number.isFinite(position.clientY)
  ) return null

  const rect = textarea.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return null

  const style = window.getComputedStyle(textarea)
  const mirror = document.createElement('div')
  const copiedStyles = [
    'box-sizing',
    'border-top-width',
    'border-right-width',
    'border-bottom-width',
    'border-left-width',
    'font-family',
    'font-feature-settings',
    'font-kerning',
    'font-size',
    'font-stretch',
    'font-style',
    'font-variant',
    'font-variation-settings',
    'font-weight',
    'letter-spacing',
    'line-height',
    'padding-top',
    'padding-right',
    'padding-bottom',
    'padding-left',
    'tab-size',
    'text-align',
    'text-indent',
    'text-rendering',
    'text-transform',
    'word-break',
  ]
  for (const property of copiedStyles) mirror.style.setProperty(property, style.getPropertyValue(property))
  mirror.style.position = 'fixed'
  mirror.style.left = `${rect.left - textarea.scrollLeft}px`
  mirror.style.top = `${rect.top - textarea.scrollTop}px`
  mirror.style.width = `${textarea.clientWidth}px`
  mirror.style.height = 'auto'
  mirror.style.minHeight = '0'
  mirror.style.maxHeight = 'none'
  mirror.style.visibility = 'hidden'
  mirror.style.pointerEvents = 'none'
  mirror.style.whiteSpace = 'pre-wrap'
  mirror.style.overflowWrap = 'break-word'

  const textNode = document.createTextNode(content || ' ')
  mirror.appendChild(textNode)
  document.body.appendChild(mirror)

  try {
    const range = document.createRange()
    let bestOffset: number | null = null
    let bestScore = Number.POSITIVE_INFINITY
    const lineWeight = Math.max(textarea.clientWidth, 1) + 1

    for (let offset = 0; offset <= content.length; offset += 1) {
      range.setStart(textNode, offset)
      range.collapse(true)
      const caret = range.getClientRects()[0] ?? range.getBoundingClientRect()
      if (!caret || (!caret.width && !caret.height)) continue
      const lineDistance = Math.abs(caret.top + caret.height / 2 - position.clientY)
      const horizontalDistance = Math.abs(caret.left - position.clientX)
      const score = lineDistance * lineWeight + horizontalDistance
      if (score < bestScore) {
        bestScore = score
        bestOffset = offset
      }
    }

    return bestOffset
  } finally {
    mirror.remove()
  }
}

function captureMarkdownImagePasteContext(dropPosition?: MarkdownDropPosition): MarkdownImagePasteContext | null {
  const noteId = noteStore.activeNote?.id
  const textarea = markdownTextarea.value
  if (!noteId || !textarea || isEditorInputLocked.value || editMode.value !== 'markdown') {
    return null
  }

  let start = textarea.selectionStart
  let end = textarea.selectionEnd
  if (dropPosition) {
    const offset = getMarkdownDropOffset(textarea, localMarkdown.value, dropPosition)
    if (offset === null) return null
    textarea.focus()
    textarea.setSelectionRange(offset, offset)
    start = offset
    end = offset
  }

  updateMarkdownSelection()
  invalidateImagePasteOperations()
  return {
    noteId,
    content: localMarkdown.value,
    start,
    end,
    generation: imagePasteGeneration,
  }
}

function isRichImagePasteContextCurrent(context: RichImagePasteContext) {
  if (isEditorInputLocked.value || context.generation !== imagePasteGeneration) return false
  if (editor.isDestroyed || editMode.value !== 'wysiwyg') return false
  if (noteStore.activeNote?.id !== context.noteId) return false
  if (!editor.state.doc.eq(context.doc)) return false
  return editor.state.selection.from === context.from && editor.state.selection.to === context.to
}

function isMarkdownImagePasteContextCurrent(context: MarkdownImagePasteContext) {
  if (isEditorInputLocked.value || context.generation !== imagePasteGeneration) return false
  if (editMode.value !== 'markdown' || noteStore.activeNote?.id !== context.noteId) return false
  if (localMarkdown.value !== context.content) return false
  const textarea = markdownTextarea.value
  if (!textarea) return false
  return textarea.selectionStart === context.start && textarea.selectionEnd === context.end
}

function getDroppedImageFiles(event: DragEvent): DroppedImageFile[] {
  const files = Array.from(event.dataTransfer?.files ?? [])
  return files.flatMap((file) => {
    if (file.type === 'image/png' || file.type === 'image/jpeg') {
      return [{ file, mimeHint: file.type }]
    }

    const name = file.name.toLowerCase()
    if (name.endsWith('.png')) return [{ file, mimeHint: 'image/png' as const }]
    if (name.endsWith('.jpg') || name.endsWith('.jpeg')) {
      return [{ file, mimeHint: 'image/jpeg' as const }]
    }
    return []
  })
}

function hasDroppedFiles(event: DragEvent) {
  const dataTransfer = event.dataTransfer
  if (!dataTransfer) return false
  return Boolean(dataTransfer.files.length || Array.from(dataTransfer.types).includes('Files'))
}

function handleEditorDragOver(event: DragEvent) {
  if (!hasDroppedFiles(event)) return
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = getDroppedImageFiles(event).length > 0 || event.dataTransfer.types.includes('Files')
      ? 'copy'
      : 'none'
  }
}

function notifyUnsupportedDrop() {
  imagePasteError.value = '添付できるファイルはPNGまたはJPEG画像です。'
  notificationStore.notify(imagePasteError.value, {
    kind: 'warning',
    source: 'note-editor',
    code: 'NOTE_EDITOR_DROP_UNSUPPORTED',
  })
}

function notifyMarkdownDropPositionUnavailable() {
  imagePasteError.value = '画像の挿入位置を特定できなかったため、本文は変更していません。'
  notificationStore.notify(imagePasteError.value, {
    kind: 'warning',
    source: 'note-editor',
    code: 'NOTE_EDITOR_DROP_POSITION_UNAVAILABLE',
  })
}

function handleRichDrop(event: DragEvent) {
  if (!hasDroppedFiles(event)) return
  if (handledDropEvents.has(event)) return
  handledDropEvents.add(event)
  event.preventDefault()
  if (editMode.value !== 'wysiwyg') return

  const files = getDroppedImageFiles(event)
  if (files.length === 0) {
    notifyUnsupportedDrop()
    return
  }
  const dropPosition = editor.view.posAtCoords({
    left: event.clientX,
    top: event.clientY,
  })?.pos
  const context = captureRichImagePasteContext(dropPosition)
  if (context) void insertDroppedImages(files, 'wysiwyg', context)
}

function handleMarkdownDrop(event: DragEvent) {
  if (!hasDroppedFiles(event)) return
  event.preventDefault()
  if (editMode.value !== 'markdown') return

  const files = getDroppedImageFiles(event)
  if (files.length === 0) {
    notifyUnsupportedDrop()
    return
  }
  const context = captureMarkdownImagePasteContext({
    clientX: event.clientX,
    clientY: event.clientY,
  })
  if (context) {
    void insertDroppedImages(files, 'markdown', context)
  } else if (!isEditorInputLocked.value) {
    notifyMarkdownDropPositionUnavailable()
  }
}

async function insertDroppedImages(
  files: DroppedImageFile[],
  mode: 'wysiwyg' | 'markdown',
  context: RichImagePasteContext | MarkdownImagePasteContext,
) {
  const attachments: NoteAttachment[] = []
  let pending: PendingImagePaste | null = null
  let failedPending: PendingImagePaste | null = null
  let failure: unknown = null

  try {
    for (const dropped of files) {
      pending = null
      pendingImagePaste.value = null
      try {
        const payload = await readClipboardImage(dropped.file, dropped.mimeHint)
        pending = {
          noteId: context.noteId,
          payload,
          attachment: null,
        }
        pendingImagePaste.value = pending
        const attachment = await persistImageAttachment(pending)
        pending.attachment = attachment
        attachments.push(attachment)

        const isCurrent = mode === 'wysiwyg'
          ? isRichImagePasteContextCurrent(context as RichImagePasteContext)
          : isMarkdownImagePasteContextCurrent(context as MarkdownImagePasteContext)
        if (!isCurrent) throw new StaleImagePasteError()
      } catch (error) {
        if (error instanceof StaleImagePasteError) throw error
        failedPending = pending
        failure = error
      }
    }

    const isCurrent = mode === 'wysiwyg'
      ? isRichImagePasteContextCurrent(context as RichImagePasteContext)
      : isMarkdownImagePasteContextCurrent(context as MarkdownImagePasteContext)
    if (!isCurrent) throw new StaleImagePasteError()
    if (attachments.length === 0) {
      if (failure) throw failure
      throw new Error('image-node-insert-failed')
    }

    if (mode === 'wysiwyg') {
      const inserted = editor.chain().focus().insertContent(
        attachments.map((attachment) => ({
          type: 'image',
          attrs: {
            src: attachment.reference,
            alt: attachment.name,
            title: attachment.name,
          },
        })),
      ).run()
      if (!inserted) throw new Error('image-node-insert-failed')
    } else {
      const markdown = attachments.map((attachment) => {
        const alt = attachment.name.replace(/[\[\]]/g, '_')
        return `![${alt}](${attachment.reference})`
      }).join('\n\n')
      const replaced = replaceMarkdownRange(
        (context as MarkdownImagePasteContext).start,
        (context as MarkdownImagePasteContext).end,
        markdown,
        (context as MarkdownImagePasteContext).start + markdown.length,
        (context as MarkdownImagePasteContext).start + markdown.length,
        { guardEditorInput: true },
      )
      if (!replaced) throw new StaleImagePasteError()
    }

    if (failure) {
      clearImagePasteState()
      retainImagePasteFailure(failedPending, failure)
    } else {
      clearImagePasteState()
    }
  } catch (error) {
    retainImagePasteFailure(pending, error)
  }
}

async function persistImageAttachment(pending: PendingImagePaste) {
  if (pending.attachment) return pending.attachment
  if (isEditorInputLocked.value || noteStore.activeNote?.id !== pending.noteId) {
    throw new StaleImagePasteError()
  }

  const accessAllowed = await contentLockStore.requestAccess(
    { type: 'note', id: pending.noteId },
    noteStore.activeNote.title,
  )
  if (!accessAllowed) throw new Error('content-lock-access-denied')
  if (isEditorInputLocked.value || noteStore.activeNote?.id !== pending.noteId) {
    throw new StaleImagePasteError()
  }

  return saveNoteAttachment({
    noteId: pending.noteId,
    kind: 'image',
    mimeType: pending.payload.mimeType,
    name: pending.payload.name,
    data: encodeBase64(pending.payload.data),
  })
}

function imagePasteErrorMessage(error: unknown) {
  if (error instanceof StaleImagePasteError) {
    return 'ノートまたは本文が変わったため、画像を挿入しませんでした。再試行できます。'
  }
  if (error instanceof Error && error.message === 'content-lock-access-denied') {
    return '保護されたノートの画像貼り付けにはロック解除が必要です。'
  }
  return '画像を保存できませんでした。本文は変更していません。'
}

function retainImagePasteFailure(pending: PendingImagePaste | null, error: unknown) {
  if (pending) pendingImagePaste.value = pending
  imagePasteError.value = imagePasteErrorMessage(error)
  if (!(error instanceof StaleImagePasteError)) {
    const noteId = pending?.noteId ?? noteStore.activeNote?.id
    logOperationFailure({
      noteId,
      stage: 'note-editor.image-paste',
      errorCategory: 'paste-failed',
    })
  }
  notificationStore.notify(imagePasteError.value, {
    kind: error instanceof StaleImagePasteError ? 'warning' : 'error',
    source: 'note-editor',
    code: 'NOTE_EDITOR_IMAGE_PASTE_FAILED',
  })
}

function clearImagePasteState() {
  pendingImagePaste.value = null
  imagePasteError.value = ''
}

async function insertRichImagePaste(
  payload: ClipboardImagePayload,
  context: RichImagePasteContext,
  existingAttachment: NoteAttachment | null = null,
) {
  const pending: PendingImagePaste = {
    noteId: context.noteId,
    payload,
    attachment: existingAttachment,
  }
  pendingImagePaste.value = pending
  try {
    const attachment = await persistImageAttachment(pending)
    pending.attachment = attachment
    if (!isRichImagePasteContextCurrent(context)) throw new StaleImagePasteError()
    const inserted = editor.chain().focus().setImage({
      src: attachment.reference,
      alt: attachment.name,
      title: attachment.name,
    }).run()
    if (!inserted) throw new Error('image-node-insert-failed')
    clearImagePasteState()
  } catch (error) {
    retainImagePasteFailure(pending, error)
  }
}

async function handleRichImagePaste(file: File) {
  const context = captureRichImagePasteContext()
  if (!context) return
  try {
    const payload = await readClipboardImage(file)
    await insertRichImagePaste(payload, context)
  } catch (error) {
    retainImagePasteFailure(null, error)
  }
}

async function insertMarkdownImagePaste(
  payload: ClipboardImagePayload,
  context: MarkdownImagePasteContext,
  existingAttachment: NoteAttachment | null = null,
) {
  const pending: PendingImagePaste = {
    noteId: context.noteId,
    payload,
    attachment: existingAttachment,
  }
  pendingImagePaste.value = pending
  try {
    const attachment = await persistImageAttachment(pending)
    pending.attachment = attachment
    if (!isMarkdownImagePasteContextCurrent(context)) throw new StaleImagePasteError()
    const alt = attachment.name.replace(/[\[\]]/g, '_')
    const markdown = `![${alt}](${attachment.reference})`
    const replaced = replaceMarkdownRange(
      context.start,
      context.end,
      markdown,
      context.start + markdown.length,
      context.start + markdown.length,
      { guardEditorInput: true },
    )
    if (!replaced) throw new StaleImagePasteError()
    clearImagePasteState()
  } catch (error) {
    retainImagePasteFailure(pending, error)
  }
}

function handleMarkdownPaste(event: ClipboardEvent) {
  const clipboardImage = getClipboardImage(event)
  if (!clipboardImage) return

  event.preventDefault()
  const context = captureMarkdownImagePasteContext()
  if (!context) return
  void (async () => {
    try {
      const payload = await readClipboardImage(clipboardImage)
      await insertMarkdownImagePaste(payload, context)
    } catch (error) {
      retainImagePasteFailure(null, error)
    }
  })()
}

async function retryImagePaste() {
  const pending = pendingImagePaste.value
  if (!pending || noteStore.activeNote?.id !== pending.noteId || isEditorInputLocked.value) return

  if (editMode.value === 'wysiwyg') {
    const context = captureRichImagePasteContext()
    if (context) await insertRichImagePaste(pending.payload, context, pending.attachment)
    return
  }

  const context = captureMarkdownImagePasteContext()
  if (context) await insertMarkdownImagePaste(pending.payload, context, pending.attachment)
}

function discardPendingImagePaste() {
  clearImagePasteState()
}

function isAttachmentExportBusy() {
  return isAttachmentExporting.value
}

async function handleExportAttachments() {
  const selectedNote = noteStore.activeNote
  if (!selectedNote || noteExportStore.isBusy || isAttachmentExporting.value) return

  const selectedNoteId = selectedNote.id
  isAttachmentExporting.value = true
  try {
    if (editMode.value === 'wysiwyg') {
      applyRichEditorToMarkdown()
    }
    if (
      localMarkdown.value !== selectedNote.content
      || getSavableTitle() !== selectedNote.title
    ) {
      scheduleAutoSave(localMarkdown.value)
    }

    const accessAllowed = await contentLockStore.requestAccess(
      { type: 'note', id: selectedNoteId },
      selectedNote.title,
    )
    if (!accessAllowed || noteStore.activeNote?.id !== selectedNoteId) return

    const saved = await noteStore.flushPendingDraft({ mode: 'required' })
    if (!saved) {
      notificationStore.notify('未保存の変更を保存できないため、添付ファイルを保存しませんでした。', {
        kind: 'warning',
        source: 'note-export',
        code: 'NOTE_ATTACHMENTS_EXPORT_DRAFT_SAVE_FAILED',
      })
      return
    }

    const current = noteStore.activeNote
    if (!current || current.id !== selectedNoteId || noteStore.getDraft(selectedNoteId)) {
      notificationStore.notify('ノートの保存状態が変わったため、添付ファイルを保存しませんでした。', {
        kind: 'warning',
        source: 'note-export',
        code: 'NOTE_ATTACHMENTS_EXPORT_NOTE_CHANGED',
      })
      return
    }

    let allowPlaintextProtected = false
    if (current.protected) {
      allowPlaintextProtected = window.confirm(
        'このノートの添付画像は保護領域の外へ復号済みの平文ZIPとして保存されます。続行しますか？',
      )
      if (!allowPlaintextProtected) return
    }

    const result = await saveNoteAttachments(
      selectedNoteId,
      current.title,
      current.revision,
      allowPlaintextProtected,
    )
    if (result.cancelled) return
    if (result.error) {
      logOperationFailure({
        noteId: selectedNoteId,
        stage: 'note-editor.attachments-export',
        errorCategory: 'runtime',
      })
      notificationStore.notify(result.error, {
        kind: 'error',
        source: 'note-export',
        code: 'NOTE_ATTACHMENTS_EXPORT_FAILED',
      })
      return
    }

    notificationStore.notify(`${result.savedName ?? '添付ファイル'}を保存しました。`, {
      kind: 'success',
      source: 'note-export',
      code: 'NOTE_ATTACHMENTS_EXPORT_COMPLETED',
    })
  } catch {
    logOperationFailure({
      noteId: selectedNoteId,
      stage: 'note-editor.attachments-export',
      errorCategory: 'runtime',
    })
    notificationStore.notify('添付ファイルを保存できませんでした。', {
      kind: 'error',
      source: 'note-export',
      code: 'NOTE_ATTACHMENTS_EXPORT_FAILED',
    })
  } finally {
    isAttachmentExporting.value = false
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
    updateMermaidEditorContext(null)
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

let contentLockPreviousEditable: boolean | null = null

function setContentLockPending(pending: boolean): boolean {
  if (editor.isDestroyed) return false
  if (pending === isContentLockPending.value) return true

  if (pending) {
    contentLockPreviousEditable = editor.isEditable
    if (!setMermaidEditorInputsLocked(
      (editor.storage as { codeBlock?: MermaidEditorSessionStorage }).codeBlock,
      true,
    )) {
      setMermaidEditorInputsLocked(
        (editor.storage as { codeBlock?: MermaidEditorSessionStorage }).codeBlock,
        false,
      )
      contentLockPreviousEditable = null
      return false
    }

    isContentLockPending.value = true
    editor.setEditable(false)
    return true
  }

  const unlocked = setMermaidEditorInputsLocked(
    (editor.storage as { codeBlock?: MermaidEditorSessionStorage }).codeBlock,
    false,
  )
  isContentLockPending.value = false
  const previousEditable = contentLockPreviousEditable
  contentLockPreviousEditable = null
  editor.setEditable((previousEditable ?? true) && !isActiveNoteDeletionPreparing.value)
  return unlocked
}

function flushEditorInput(): boolean {
  if (editMode.value !== 'wysiwyg' || editor.isDestroyed) return true

  const storage = (editor.storage as { codeBlock?: MermaidEditorSessionStorage }).codeBlock
  if (!flushMermaidEditorInputs(storage)) {
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.flush-before-lock',
      errorCategory: 'mermaid-input-flush-failed',
    })
    return false
  }

  try {
    applyRichEditorToMarkdown()
    return true
  } catch {
    logOperationFailure({
      noteId: noteStore.activeNote?.id,
      stage: 'note-editor.flush-before-lock',
      errorCategory: 'rich-content-snapshot-failed',
    })
    return false
  }
}

async function saveCurrentNote(): Promise<boolean> {
  if (!noteStore.activeNote || isEditorInputLocked.value) return false

  if (editMode.value === 'wysiwyg') {
    if (!flushEditorInput()) return false
  } else {
    updateMarkdownSelection()
    scheduleAutoSave(localMarkdown.value)
  }

  return noteStore.flushPendingDraft({ mode: 'explicit' })
}

defineExpose({
  toggleAIWorkspace,
  toggleEditMode,
  flushEditorInput,
  saveCurrentNote,
  setContentLockPending,
  isAttachmentExportBusy,
})

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
  updateMermaidEditorContext(noteStore.activeNote?.id ?? null)
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
  restoreSerializedEmptyParagraphs(container)
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

function toggleHorizontalRule() {
  if (editMode.value === 'wysiwyg') {
    editor.chain().focus().setHorizontalRule().run()
    return
  }

  insertMarkdownBlock('---')
}

function insertMermaidDiagram(
  type: MermaidDiagramType,
  commandRange?: { start: number; end: number } | { from: number; to: number },
) {
  const starter = getMermaidDiagramDefinition(type).sample
  if (editMode.value === 'markdown') {
    if (commandRange && 'start' in commandRange) {
      replaceMarkdownRange(commandRange.start, commandRange.end, createMermaidFence(starter))
      closeMermaidCommand()
      return
    }
    insertMarkdownBlock(createMermaidFence(starter))
    return
  }

  insertMermaidCodeBlock(editor, starter, commandRange && 'from' in commandRange ? commandRange : undefined)
  closeMermaidCommand()
}

function getMermaidCommandMatch(value: string, cursor: number) {
  const beforeCursor = value.slice(0, cursor)
  const match = beforeCursor.match(/(?:^|\s)\/mermaid(?:\s+([^\s]*))?$/i)
  if (!match || match.index === undefined) return null
  const commandOffset = match[0].lastIndexOf('/mermaid')
  return {
    start: match.index + commandOffset,
    end: cursor,
    query: match[1] ?? '',
  }
}

function updateMermaidCommandState(mode: 'markdown' | 'rich') {
  if (mode === 'markdown') {
    const selection = getMarkdownSelection()
    const lineStart = localMarkdown.value.lastIndexOf('\n', Math.max(selection.start - 1, 0)) + 1
    const match = getMermaidCommandMatch(localMarkdown.value.slice(lineStart, selection.start), selection.start - lineStart)
    if (!match || selection.start !== selection.end) {
      closeMermaidCommand()
      return
    }
    mermaidCommandRange.value = { start: lineStart + match.start, end: lineStart + match.end }
    mermaidCommandQuery.value = match.query
  } else {
    if (editMode.value !== 'wysiwyg' || editor.state.selection.empty === false) {
      closeMermaidCommand()
      return
    }
    const selection = editor.state.selection
    const parent = selection.$from.parent
    const match = getMermaidCommandMatch(parent.textContent, selection.$from.parentOffset)
    if (!match) {
      closeMermaidCommand()
      return
    }
    mermaidCommandRange.value = {
      from: selection.$from.start() + match.start,
      to: selection.$from.pos,
    }
    mermaidCommandQuery.value = match.query
  }

  mermaidCommandOpen.value = mermaidCommandSuggestions.value.length > 0
  mermaidCommandIndex.value = Math.min(mermaidCommandIndex.value, Math.max(0, mermaidCommandSuggestions.value.length - 1))
}

function closeMermaidCommand() {
  mermaidCommandOpen.value = false
  mermaidCommandQuery.value = ''
  mermaidCommandRange.value = null
  mermaidCommandIndex.value = 0
}

function selectMermaidCommand(type: MermaidDiagramType) {
  const range = mermaidCommandRange.value
  if (!range) return
  insertMermaidDiagram(type, range)
}

function handleMermaidCommandKeydown(event: KeyboardEvent, mode: 'markdown' | 'rich') {
  if (!mermaidCommandOpen.value || (mode === 'markdown' && editMode.value !== 'markdown')) return false

  if (event.key === 'Escape') {
    event.preventDefault()
    closeMermaidCommand()
    return true
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const delta = event.key === 'ArrowDown' ? 1 : -1
    const length = mermaidCommandSuggestions.value.length
    mermaidCommandIndex.value = (mermaidCommandIndex.value + delta + length) % length
    return true
  }
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    const diagram = mermaidCommandSuggestions.value[mermaidCommandIndex.value]
    if (diagram) selectMermaidCommand(diagram.type)
    return true
  }
  return false
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

function findRichCodeBlockNode(selection: Selection): ProseMirrorNode | null {
  return findRichCodeBlockRange(selection)?.node ?? null
}

function findRichMermaidSource(selection: Selection): string | null {
  const codeBlock = findRichCodeBlockNode(selection)
  if (!codeBlock) return null

  const language = String(codeBlock.attrs.language ?? '').trim().toLowerCase()
  return language === 'mermaid' ? codeBlock.textContent : null
}

function findRichCodeBlockRange(selection: Selection) {
  if (selection instanceof NodeSelection) {
    return selection.node.type.name === 'codeBlock'
      ? { node: selection.node, from: selection.from, to: selection.to }
      : null
  }

  const { $from } = selection
  for (let depth = $from.depth; depth > 0; depth -= 1) {
    const node = $from.node(depth)
    if (node.type.name === 'codeBlock') {
      return { node, from: $from.before(depth), to: $from.after(depth) }
    }
  }

  return null
}

function isMermaidCodeBlock(node: ProseMirrorNode) {
  return node.type.name === 'codeBlock'
    && String(node.attrs.language ?? '').trim().toLowerCase() === 'mermaid'
}

function isSelectionInsideMermaid(selection: Selection) {
  const range = findRichCodeBlockRange(selection)
  return Boolean(range && isMermaidCodeBlock(range.node)
    && selection.from >= range.from
    && selection.to <= range.to)
}

function isOrdinaryCodeBlockActive() {
  return editor.isActive('codeBlock') && !isSelectionInsideMermaid(editor.state.selection)
}

function selectionContainsMermaid(selection: Selection) {
  if (isSelectionInsideMermaid(selection)) return true

  let contains = false
  editor.state.doc.nodesBetween(selection.from, selection.to, (node) => {
    if (isMermaidCodeBlock(node)) {
      contains = true
      return false
    }
    return !contains
  })
  return contains
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

function handleMarkdownCompositionStart() {
  markdownLineBreakTracker.reset()
  isMarkdownComposing = true
}

function handleMarkdownCompositionEnd() {
  isMarkdownComposing = false
}

function handleMarkdownBeforeInput(event: InputEvent) {
  if (event.inputType === 'historyUndo' || event.inputType === 'historyRedo') {
    if (event.cancelable) {
      event.preventDefault()
      pendingMarkdownInput = null
      applyMarkdownHistory(event.inputType === 'historyUndo' ? 'undo' : 'redo')
    }
    markdownLineBreakTracker.reset()
    return
  }

  const textarea = event.currentTarget as HTMLTextAreaElement
  const skipListContinuation = markdownLineBreakTracker.shouldSkipListContinuation(event)
  if (
    (event.inputType === 'insertLineBreak' || event.inputType === 'insertParagraph')
    && !skipListContinuation
    && !isMarkdownComposing
    && !event.isComposing
  ) {
    const before = createMarkdownSnapshot(textarea.value, textarea)
    const after = continueMarkdownList(before)
    if (after && event.cancelable) {
      event.preventDefault()
      applyMarkdownSnapshot(before, after)
      pendingMarkdownInput = null
      return
    }
  }

  pendingMarkdownInput = {
    before: createMarkdownSnapshot(textarea.value, textarea),
    ...getMarkdownInputRecordOptions(event.inputType),
  }
}

function handleMarkdownKeydown(event: KeyboardEvent) {
  if (handleMermaidCommandKeydown(event, 'markdown')) return
  markdownLineBreakTracker.handleKeydown(event)
  if (
    event.key === 'Enter'
    && !event.shiftKey
    && !event.ctrlKey
    && !event.altKey
    && !event.metaKey
    && !isMarkdownComposing
    && !event.isComposing
  ) {
    const textarea = event.currentTarget as HTMLTextAreaElement
    const before = createMarkdownSnapshot(textarea.value, textarea)
    const after = continueMarkdownList(before)
    if (after) {
      event.preventDefault()
      event.stopPropagation()
      applyMarkdownSnapshot(before, after)
      pendingMarkdownInput = null
      return
    }
  }

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

function applyMarkdownSnapshot(before: MarkdownEditSnapshot, after: MarkdownEditSnapshot) {
  invalidateImagePasteOperations()
  pendingMarkdownInput = null
  markdownEditHistory.record(before, after, { group: 'markdown-list', forceNewGroup: true })
  dismissAgentEditorHighlight()
  localMarkdown.value = after.content
  lastMarkdownSelection = {
    start: after.selectionStart,
    end: after.selectionEnd,
  }
  updateAutoTitleFromMarkdown(after.content)
  scheduleAutoSave(after.content)
  markdownSelectionVersion.value += 1

  const noteId = activeNoteId
  void nextTick(() => {
    if (activeNoteId !== noteId) return
    const textarea = markdownTextarea.value
    if (!textarea) return
    textarea.focus()
    textarea.setSelectionRange(after.selectionStart, after.selectionEnd)
    markdownSelectionVersion.value += 1
  })
}

function applyMarkdownHistory(action: 'undo' | 'redo') {
  const snapshot = action === 'undo'
    ? markdownEditHistory.undo()
    : markdownEditHistory.redo()
  if (!snapshot) return

  invalidateImagePasteOperations()
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
  markdownLineBreakTracker.reset()
  invalidateImagePasteOperations()
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
  updateMermaidCommandState('markdown')
  updateAutoTitleFromMarkdown(localMarkdown.value)
  scheduleAutoSave(localMarkdown.value)
}

function handleMarkdownClick(event: MouseEvent) {
  updateMarkdownSelection()
  updateMermaidCommandState('markdown')
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
  invalidateImagePasteOperations()
  const textarea = markdownTextarea.value
  if (textarea) {
    lastMarkdownSelection = {
      start: textarea.selectionStart,
      end: textarea.selectionEnd,
    }
  }

  markdownSelectionVersion.value += 1
  if (editMode.value === 'markdown') updateMermaidCommandState('markdown')
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
  options: { guardEditorInput?: boolean } = {},
): boolean {
  if (options.guardEditorInput && isEditorInputLocked.value) return false
  if (
    !Number.isInteger(start)
    || !Number.isInteger(end)
    || start < 0
    || end < start
    || end > localMarkdown.value.length
    || selectionStart < 0
    || selectionEnd < selectionStart
  ) return false

  invalidateImagePasteOperations()
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
  return true
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

.attachment-paste-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: min(520px, 45vw);
  color: var(--color-danger, #b42318);
  font-size: 12px;
}

.attachment-paste-indicator span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-paste-indicator button {
  flex: 0 0 auto;
  padding: 2px 6px;
  border: 1px solid currentColor;
  border-radius: 4px;
  color: inherit;
  font-size: 11px;
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

.mermaid-command-menu {
  position: absolute;
  z-index: 1100;
  top: 18px;
  left: 24px;
  display: flex;
  width: min(260px, calc(100% - 48px));
  flex-direction: column;
  padding: 6px;
  border: 1px solid var(--border-strong, var(--border));
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 12px 28px rgba(0, 0, 0, .2);
}

.mermaid-command-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 5px 8px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
  font-size: 12px;
}

.mermaid-command-item:hover,
.mermaid-command-item.is-selected {
  background: var(--bg-hover);
  color: var(--brand-primary);
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

.prose-editor :deep(.note-attachment-image) {
  height: auto;
  max-width: none;
}

.prose-editor :deep(.note-attachment-image-frame) {
  position: relative;
  display: inline-block;
  max-width: none;
  outline: 1px solid transparent;
}

.prose-editor :deep(.note-attachment-image-frame.ProseMirror-selectednode),
.prose-editor :deep(.note-attachment-image-frame:hover) {
  outline-color: var(--brand-primary);
}

.prose-editor :deep(.note-attachment-image-frame .visual-resize-handle) {
  position: absolute;
  right: -5px;
  bottom: -5px;
  width: 12px;
  height: 12px;
  padding: 0;
  border: 1px solid var(--bg-editor);
  border-radius: 2px;
  background: var(--brand-primary);
  cursor: nwse-resize;
  opacity: 0;
  touch-action: none;
}

.prose-editor :deep(.note-attachment-image-frame.ProseMirror-selectednode .visual-resize-handle),
.prose-editor :deep(.note-attachment-image-frame:hover .visual-resize-handle),
.prose-editor :deep(.note-attachment-image-frame .visual-resize-handle:focus-visible) {
  opacity: 1;
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
