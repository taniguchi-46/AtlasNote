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
        <input
          id="note-title-input"
          v-model="localTitle"
          class="title-input"
          type="text"
          placeholder="タイトル"
          @blur="handleTitleSave"
          @keydown.enter="handleTitleSave"
        />

        <div class="toolbar-actions">
          <span v-if="noteStore.isSaving" class="saving-indicator">保存中...</span>
          <span v-else-if="savedMessage" class="saved-indicator">保存済み</span>

          <button
            class="mode-toggle-btn"
            type="button"
            :title="editMode === 'wysiwyg' ? 'Markdownモードに切り替え' : 'プレビューモードに切り替え'"
            @click="toggleEditMode"
          >
            <CodeIcon v-if="editMode === 'wysiwyg'" :size="16" />
            <FileTextIcon v-else :size="16" />
            <span>{{ editMode === 'wysiwyg' ? 'Markdownへ' : 'Previewへ' }}</span>
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

      <div v-if="editMode === 'wysiwyg'" class="editor-format-bar">
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('bold') }"
          type="button"
          title="太字"
          @click="editor?.chain().focus().toggleBold().run()"
        >
          <BoldIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('italic') }"
          type="button"
          title="斜体"
          @click="editor?.chain().focus().toggleItalic().run()"
        >
          <ItalicIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('strike') }"
          type="button"
          title="取り消し線"
          @click="editor?.chain().focus().toggleStrike().run()"
        >
          <StrikethroughIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('code') }"
          type="button"
          title="インラインコード"
          @click="editor?.chain().focus().toggleCode().run()"
        >
          <CodeIcon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('heading', { level: 1 }) }"
          type="button"
          title="見出し1"
          @click="editor?.chain().focus().toggleHeading({ level: 1 }).run()"
        >
          <Heading1Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('heading', { level: 2 }) }"
          type="button"
          title="見出し2"
          @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()"
        >
          <Heading2Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('heading', { level: 3 }) }"
          type="button"
          title="見出し3"
          @click="editor?.chain().focus().toggleHeading({ level: 3 }).run()"
        >
          <Heading3Icon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('bulletList') }"
          type="button"
          title="箇条書きリスト"
          @click="editor?.chain().focus().toggleBulletList().run()"
        >
          <ListIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('orderedList') }"
          type="button"
          title="番号付きリスト"
          @click="editor?.chain().focus().toggleOrderedList().run()"
        >
          <ListOrderedIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('taskList') }"
          type="button"
          title="タスクリスト"
          @click="editor?.chain().focus().toggleTaskList().run()"
        >
          <CheckSquareIcon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('blockquote') }"
          type="button"
          title="引用"
          @click="editor?.chain().focus().toggleBlockquote().run()"
        >
          <QuoteIcon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('codeBlock') }"
          type="button"
          title="コードブロック"
          @click="editor?.chain().focus().toggleCodeBlock().run()"
        >
          <TerminalIcon :size="15" />
        </button>

        <span class="format-divider" />

        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('table') }"
          type="button"
          title="表を挿入"
          @click="insertTable"
        >
          <Table2Icon :size="15" />
        </button>

        <template v-if="isTableActive">
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

      <div class="editor-body">
        <EditorContent v-if="editMode === 'wysiwyg'" :editor="editor" class="prose-editor" />
        <textarea
          v-else
          v-model="localMarkdown"
          class="markdown-textarea"
          placeholder="ここにMarkdownで内容を入力してください..."
          @input="handleMarkdownInput"
        />
      </div>

      <div class="editor-statusbar">
        <span>{{ charCount }} 文字</span>
        <span>更新: {{ formatDate(noteStore.activeNote.updatedAt) }}</span>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  BoldIcon,
  CheckSquareIcon,
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
  StarIcon,
  StrikethroughIcon,
  Table2Icon,
  TableColumnsSplitIcon,
  TableRowsSplitIcon,
  TerminalIcon,
  Trash2Icon,
} from '@lucide/vue'
import { Editor, EditorContent } from '@tiptap/vue-3'
import { DOMParser as ProseMirrorDOMParser } from '@tiptap/pm/model'
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
import { serializeTiptapJsonToMarkdown } from '../utils/tiptapMarkdownSerializer'

const CustomTableCell = TableCell.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const CustomTableHeader = TableHeader.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const lowlight = createLowlight(common)
const noteStore = useNoteStore()

const localTitle = ref('')
const savedMessage = ref(false)
const editMode = ref<'wysiwyg' | 'markdown'>('markdown')
const localMarkdown = ref('')
const isApplyingContent = ref(false)
const isRichDirty = ref(false)
const editorStateVersion = ref(0)
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
let activeNoteId: string | null = null

const editor = new Editor({
  extensions: [
    StarterKit.configure({
      codeBlock: false,
      link: false,
    }),
    Markdown.configure({
      html: true,
      linkify: true,
    }),
    Placeholder.configure({
      emptyNodeClass: 'is-empty',
      showOnlyCurrent: true,
      placeholder: 'ここに内容を入力してください...',
    }),
    Link.configure({
      openOnClick: false,
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
      scheduleAutoSave(markdown)
    }
  },
})

watch(
  () => noteStore.activeNote,
  (note) => {
    if (!note) {
      activeNoteId = null
      return
    }

    const noteChanged = activeNoteId !== note.id
    activeNoteId = note.id
    localTitle.value = note.title

    if (noteChanged) {
      localMarkdown.value = note.content
      isRichDirty.value = false
      if (editMode.value === 'wysiwyg') {
        if (!setEditorFromMarkdown(note.content)) {
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

onBeforeUnmount(() => {
  editor.destroy()
})

const charCount = computed(() => {
  return localMarkdown.value.length
})

const isTableActive = computed(() => {
  editorStateVersion.value
  return editMode.value === 'wysiwyg' && editor.isActive('table')
})

function handleTitleSave() {
  if (!noteStore.activeNote) return
  if (localTitle.value === noteStore.activeNote.title) return

  noteStore
    .saveNote(noteStore.activeNote.id, { title: localTitle.value })
    .then(() => showSaved())
}

function toggleEditMode() {
  if (editMode.value === 'wysiwyg') {
    applyRichEditorToMarkdown()
    editMode.value = 'markdown'
    return
  }

  if (setEditorFromMarkdown(localMarkdown.value)) {
    editMode.value = 'wysiwyg'
  }
}

function setEditorFromMarkdown(markdown: string): boolean {
  isApplyingContent.value = true
  try {
    const html = parseMarkdownToRichHtml(escapeRawHtmlForRichEditor(markdown))
    const content = parseRichHtmlToJson(html)
    ;(editor.commands as any).setContent(content, {
      emitUpdate: false,
    })
    isRichDirty.value = false
    return true
  } catch (error) {
    console.error('Failed to load Markdown into rich editor', error)
    return false
  } finally {
    isApplyingContent.value = false
  }
}

function escapeRawHtmlForRichEditor(markdown: string) {
  let inFence = false

  return markdown
    .split('\n')
    .map((line) => {
      if (/^\s*(```|~~~)/.test(line)) {
        inFence = !inFence
        return line
      }
      if (inFence) return line

      return line.replace(/<\/?[A-Za-z][^>\n]*>/g, (tag) =>
        tag
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;'),
      )
    })
    .join('\n')
}

function applyRichEditorToMarkdown() {
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
  normalizeTableCells(container)
  return ProseMirrorDOMParser.fromSchema(editor.schema).parse(container).toJSON()
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

function insertTable() {
  editor
    .chain()
    .focus()
    .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
    .run()
}

function addTableRow() {
  editor.chain().focus().addRowAfter().run()
}

function addTableColumn() {
  editor.chain().focus().addColumnAfter().run()
}

function deleteTableRow() {
  editor.chain().focus().deleteRow().run()
}

function deleteTableColumn() {
  editor.chain().focus().deleteColumn().run()
}

function deleteTable() {
  editor.chain().focus().deleteTable().run()
}

function handleMarkdownInput() {
  scheduleAutoSave(localMarkdown.value)
}

function scheduleAutoSave(content: string) {
  if (autoSaveTimer) {
    clearTimeout(autoSaveTimer)
  }

  autoSaveTimer = setTimeout(() => {
    if (!noteStore.activeNote) return

    noteStore
      .saveNote(noteStore.activeNote.id, {
        title: localTitle.value,
        content,
      })
      .then(() => showSaved())
  }, 1000)
}

function showSaved() {
  savedMessage.value = true
  setTimeout(() => {
    savedMessage.value = false
  }, 2000)
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
.mode-toggle-btn {
  padding: 4px 10px;
  background-color: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  transition: background-color 0.2s, border-color 0.2s;
  margin-right: 8px;
}

.mode-toggle-btn:hover {
  background-color: var(--bg-hover);
  border-color: var(--border-strong);
}

.markdown-textarea {
  width: 100%;
  height: 100%;
  min-height: 400px;
  border: none;
  resize: none;
  background-color: transparent;
  color: var(--text-primary);
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 14px;
  line-height: 1.6;
  padding: 24px;
  outline: none;
}
</style>
