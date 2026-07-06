<template>
  <section class="editor-pane" aria-label="エディタ">
    <!-- Empty state -->
    <div v-if="!noteStore.activeNote" class="editor-empty">
      <div class="editor-empty-icon">
        <FileTextIcon :size="48" />
      </div>
      <p class="editor-empty-title">ノートを選択してください</p>
      <p class="editor-empty-sub">左のリストからノートを選ぶか、新規ノートを作成してください</p>
      <button
        id="btn-new-note-editor"
        class="primary-btn"
        type="button"
        @click="noteStore.newNote()"
      >
        新規ノート
      </button>
    </div>

    <!-- Editor -->
    <template v-else>
      <!-- Toolbar -->
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
          <span v-if="noteStore.isSaving" class="saving-indicator">保存中…</span>
          <span v-else-if="savedMessage" class="saved-indicator">保存済み</span>
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

      <!-- Format Bar -->
      <div class="editor-format-bar">
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
          title="打ち消し線"
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
          title="見出し 1"
          @click="editor?.chain().focus().toggleHeading({ level: 1 }).run()"
        >
          <Heading1Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('heading', { level: 2 }) }"
          type="button"
          title="見出し 2"
          @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()"
        >
          <Heading2Icon :size="15" />
        </button>
        <button
          class="format-btn"
          :class="{ 'is-active': editor?.isActive('heading', { level: 3 }) }"
          type="button"
          title="見出し 3"
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
        <button
          class="format-btn"
          type="button"
          title="テーブル挿入"
          @click="insertTable"
        >
          <TableIcon :size="15" />
        </button>
      </div>

      <!-- Editor Canvas -->
      <div class="editor-body">
        <BubbleMenu 
          :editor="editor" 
          :tippy-options="{ duration: 100 }"
          v-if="editor"
          class="table-bubble-menu"
          :should-show="(props: any) => props.editor.isActive('table')"
        >
          <button class="menu-btn" title="列を左に挿入" @click="editor.chain().focus().addColumnBefore().run()"><ColumnsIcon :size="14" class="flip-horizontal" /></button>
          <button class="menu-btn" title="列を右に挿入" @click="editor.chain().focus().addColumnAfter().run()"><ColumnsIcon :size="14" /></button>
          <button class="menu-btn" title="列を削除" @click="editor.chain().focus().deleteColumn().run()"><TrashIcon :size="14" class="text-danger" /></button>
          
          <span class="menu-divider" />
          
          <button class="menu-btn" title="行を上に挿入" @click="editor.chain().focus().addRowBefore().run()"><RowsIcon :size="14" class="flip-vertical" /></button>
          <button class="menu-btn" title="行を下に挿入" @click="editor.chain().focus().addRowAfter().run()"><RowsIcon :size="14" /></button>
          <button class="menu-btn" title="行を削除" @click="editor.chain().focus().deleteRow().run()"><TrashIcon :size="14" class="text-danger" /></button>
          
          <span class="menu-divider" />
          
          <button class="menu-btn" title="左揃え" @click="editor.chain().focus().setCellAttribute('align', 'left').run()"><AlignLeftIcon :size="14" /></button>
          <button class="menu-btn" title="中央揃え" @click="editor.chain().focus().setCellAttribute('align', 'center').run()"><AlignCenterIcon :size="14" /></button>
          <button class="menu-btn" title="右揃え" @click="editor.chain().focus().setCellAttribute('align', 'right').run()"><AlignRightIcon :size="14" /></button>

          <span class="menu-divider" />

          <button class="menu-btn danger" title="表を削除" @click="editor.chain().focus().deleteTable().run()">表を削除</button>
        </BubbleMenu>

        <EditorContent :editor="editor" class="prose-editor" />
      </div>

      <!-- Status bar -->
      <div class="editor-statusbar">
        <span>{{ charCount }} 文字</span>
        <span>更新: {{ formatDate(noteStore.activeNote.updatedAt) }}</span>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount } from 'vue'
import {
  FileTextIcon, StarIcon, PinIcon, Trash2Icon,
  BoldIcon, ItalicIcon, StrikethroughIcon, CodeIcon,
  Heading1Icon, Heading2Icon, Heading3Icon,
  ListIcon, ListOrderedIcon, CheckSquareIcon,
  QuoteIcon, TerminalIcon, TableIcon,
  MoreVerticalIcon, AlignLeftIcon, AlignCenterIcon, AlignRightIcon,
  CopyIcon, ColumnsIcon, RowsIcon, TrashIcon
} from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useNotebookStore, type NotebookNode } from '../stores/useNotebookStore'
import { Editor, EditorContent } from '@tiptap/vue-3'
import { BubbleMenu } from '@tiptap/vue-3/menus'
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

const CustomTableCell = TableCell.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const CustomTableHeader = TableHeader.extend({
  content: '(paragraph | heading | blockquote | codeBlock | bulletList | orderedList | taskList | horizontalRule)+',
})

const lowlight = createLowlight(common)
const noteStore = useNoteStore()
const notebookStore = useNotebookStore()
const maxEmbeddedImageBytes = 512 * 1024

const localTitle = ref('')
const savedMessage = ref(false)
let autoSaveTimer: ReturnType<typeof setTimeout> | null = null

// Tiptap Editor instance
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
      placeholder: 'ここにMarkdownで内容を入力してください...',
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
  editorProps: {
    handlePaste(_view, event) {
      return insertImageFiles(event.clipboardData?.files)
    },
    handleDrop(_view, event) {
      const handled = insertImageFiles(event.dataTransfer?.files)
      if (handled) {
        event.preventDefault()
      }
      return handled
    },
    handleKeyDown(view, event) {
      if (event.ctrlKey && event.key === 'Enter') {
        const editorInstance = editor
        if (editorInstance.isActive('table')) {
          editorInstance.commands.addRowAfter()
          return true
        }
      }
      return false
    }
  },
  onUpdate({ editor }) {
    const markdown = (editor.storage as any).markdown.getMarkdown()
    scheduleAutoSave(markdown)
  }
})

// Sync note changes to editor
watch(() => noteStore.activeNote, (note) => {
  if (note) {
    localTitle.value = note.title
    if (editor && !editor.isFocused) {
      const currentMarkdown = (editor.storage as any).markdown.getMarkdown()
      if (currentMarkdown !== note.content) {
        (editor.commands as any).setContent(note.content, false, {
          preserveWhitespace: 'full'
        })
      }
    }
  }
}, { immediate: true })

onBeforeUnmount(() => {
  editor.destroy()
})

const charCount = computed(() => {
  if (!editor) return 0
  return editor.getText().length
})

function handleTitleSave() {
  if (!noteStore.activeNote) return
  if (localTitle.value === noteStore.activeNote.title) return
  noteStore.saveNote(noteStore.activeNote.id, { title: localTitle.value })
    .then(() => showSaved())
}

function scheduleAutoSave(content: string) {
  if (autoSaveTimer) clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(() => {
    if (!noteStore.activeNote) return
    noteStore.saveNote(noteStore.activeNote.id, {
      title: localTitle.value,
      content,
    }).then(() => showSaved())
  }, 1000)
}

function showSaved() {
  savedMessage.value = true
  setTimeout(() => { savedMessage.value = false }, 2000)
}

function insertTable() {
  if (editor.isActive('table')) {
    return
  }
  editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()
}

function insertImageFiles(files?: FileList | null): boolean {
  if (!files || files.length === 0) return false

  const images = Array.from(files).filter(file => file.type.startsWith('image/'))
  if (images.length === 0) return false

  const embeddableImages = images.filter(file => file.size <= maxEmbeddedImageBytes)
  if (embeddableImages.length !== images.length) {
    noteStore.error = '512KBを超える画像はMVPでは貼り付けできません'
  }
  if (embeddableImages.length === 0) return true

  embeddableImages.forEach(file => {
    const reader = new FileReader()
    reader.onload = () => {
      const src = typeof reader.result === 'string' ? reader.result : ''
      if (!src) return
      editor.chain().focus().setImage({ src, alt: file.name }).run()
    }
    reader.readAsDataURL(file)
  })

  return true
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('ja-JP', {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}
</script>
