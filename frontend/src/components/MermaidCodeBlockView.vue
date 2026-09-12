<template>
  <NodeViewWrapper as="pre" class="code-block-node-view">
    <NodeViewContent
      as="code"
      :class="{ 'mermaid-code-block-source': isMermaid }"
      :contenteditable="isMermaid ? 'false' : undefined"
      :tabindex="isMermaid ? -1 : undefined"
      :aria-hidden="isMermaid ? 'true' : undefined"
    />
    <div
      v-if="isMermaid"
      class="mermaid-code-block-preview"
      contenteditable="false"
      :aria-busy="status === 'loading'"
    >
      <div class="mermaid-code-block-toolbar">
        <span>Mermaid図</span>
        <button
          type="button"
          class="mermaid-code-block-edit"
          :disabled="!canEdit"
          @click.stop="openEditor"
        >
          編集
        </button>
      </div>
      <p v-if="status === 'loading'" class="mermaid-code-block-status" role="status">
        Mermaid図を描画しています…
      </p>
      <p v-else-if="status === 'error'" class="mermaid-code-block-error" role="alert">
        {{ errorMessage }}
      </p>
      <img v-else-if="svgUrl" :src="svgUrl" :alt="altText" class="mermaid-code-block-image" />
    </div>
    <MermaidEditDialog
      ref="editDialogRef"
      v-if="isMermaid"
      :open="editDialogOpen"
      :source="source"
      :theme="appStore.theme"
      :input-locked="isInputLocked"
      @update:open="handleDialogOpen"
      @update:source="applySource"
      @save="finishEditing"
    />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { NodeViewContent, NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { Fragment, type Node as ProseMirrorNode } from '@tiptap/pm/model'
import { useAppStore } from '../stores/useAppStore'
import { useNoteStore } from '../stores/useNoteStore'
import { renderMermaidDiagram } from '../utils/mermaidRenderer'
import {
  type MermaidEditorInputFlusher,
  type MermaidEditorInputLocker,
  type MermaidEditorSessionStorage,
} from '../utils/mermaidEditorSession'
import MermaidEditDialog from './MermaidEditDialog.vue'

const props = defineProps(nodeViewProps)
const appStore = useAppStore()
const noteStore = useNoteStore()

type MermaidEditorContext = {
  noteId: string | null
  generation: number
}

type MermaidEditSession = MermaidEditorContext & {
  node: ProseMirrorNode
}

type MermaidEditDialogExpose = {
  flushInput: () => void
  setInputLocked: (locked: boolean) => void
}

const isMermaid = computed(() => {
  const language = String(props.node.attrs.language ?? '').trim().toLowerCase()
  return language === 'mermaid'
})
const source = computed(() => props.node.textContent)
const editDialogOpen = ref(false)
const editDialogRef = ref<MermaidEditDialogExpose | null>(null)
const isInputLocked = ref(false)
let editSession: MermaidEditSession | null = null

function getEditorStorage() {
  return (props.extension as { storage?: MermaidEditorSessionStorage } | undefined)?.storage
}

function getEditorContext(): MermaidEditorContext {
  const storage = getEditorStorage()
  return {
    noteId: typeof storage?.noteId === 'string' ? storage.noteId : null,
    generation: typeof storage?.generation === 'number' ? storage.generation : 0,
  }
}

const canEdit = computed(() => {
  const context = getEditorContext()
  const editorIsEditable = props.editor?.isEditable === true
  return isMermaid.value
    && !props.editor?.isDestroyed
    && context.noteId !== null
    && !noteStore.isNoteDeletionPreparing(context.noteId)
    && (editorIsEditable || (editDialogOpen.value && isInputLocked.value))
})

function flushEditorInput(): boolean {
  if (!editDialogOpen.value) return true

  const dialog = editDialogRef.value
  if (!dialog) return false
  dialog.flushInput()
  return true
}

const editorInputFlusher: MermaidEditorInputFlusher = flushEditorInput
getEditorStorage()?.mermaidEditorFlushers?.add(editorInputFlusher)

const editorInputLocker: MermaidEditorInputLocker = (locked) => {
  isInputLocked.value = locked
  editDialogRef.value?.setInputLocked(locked)
}
getEditorStorage()?.mermaidEditorInputLockers?.add(editorInputLocker)

const status = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const errorMessage = ref('')
const svgUrl = ref<string | null>(null)
const altText = ref('Mermaid図')

let renderTimer: ReturnType<typeof setTimeout> | null = null
let renderGeneration = 0
let currentObjectUrl: string | null = null

function revokeObjectUrl() {
  if (!currentObjectUrl) return
  if (typeof URL !== 'undefined' && typeof URL.revokeObjectURL === 'function') {
    URL.revokeObjectURL(currentObjectUrl)
  }
  currentObjectUrl = null
}

function clearPreview() {
  revokeObjectUrl()
  svgUrl.value = null
  altText.value = 'Mermaid図'
}

function scheduleRender() {
  renderGeneration += 1
  const generation = renderGeneration

  if (renderTimer) {
    clearTimeout(renderTimer)
    renderTimer = null
  }

  if (!isMermaid.value) {
    status.value = 'idle'
    errorMessage.value = ''
    clearPreview()
    return
  }

  status.value = 'loading'
  errorMessage.value = ''
  renderTimer = setTimeout(() => {
    renderTimer = null
    void render(generation)
  }, 250)
}

async function render(generation: number) {
  let result: Awaited<ReturnType<typeof renderMermaidDiagram>>
  try {
    result = await renderMermaidDiagram(source.value, {
      theme: appStore.theme,
    })
  } catch {
    if (generation !== renderGeneration) return
    clearPreview()
    status.value = 'error'
    errorMessage.value = 'Mermaidを読み込めないため表示できません。'
    return
  }
  if (generation !== renderGeneration) return

  if (!result.ok) {
    clearPreview()
    status.value = 'error'
    errorMessage.value = result.message
    return
  }

  if (typeof URL === 'undefined' || typeof URL.createObjectURL !== 'function') {
    clearPreview()
    status.value = 'error'
    errorMessage.value = '図の出力を安全に表示できませんでした。'
    return
  }

  let nextObjectUrl: string
  try {
    nextObjectUrl = URL.createObjectURL(new Blob([result.svg], { type: 'image/svg+xml' }))
  } catch {
    clearPreview()
    status.value = 'error'
    errorMessage.value = '図の出力を安全に表示できませんでした。'
    return
  }
  if (generation !== renderGeneration) {
    if (typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(nextObjectUrl)
    return
  }

  revokeObjectUrl()
  currentObjectUrl = nextObjectUrl
  svgUrl.value = nextObjectUrl
  altText.value = result.altText
  status.value = 'ready'
}

function openEditor() {
  if (!canEdit.value || isInputLocked.value) return
  const target = getCurrentMermaidTarget()
  if (!target) return

  if (!target.node.eq(props.node)) return

  editSession = {
    ...getEditorContext(),
    node: target.node,
  }
  editDialogOpen.value = true
}

function applySource(nextSource: string) {
  const session = editSession
  if (!session) return closeEditor()

  const context = getEditorContext()
  if (context.noteId !== session.noteId || context.generation !== session.generation) {
    return closeEditor()
  }

  // A deletion preparation makes the editor read-only after the dialog has
  // already accepted an input event. Permit that final controlled update so
  // an IME composition is captured before the store flushes the note.
  const target = getCurrentMermaidTarget(false)
  if (!target || !target.node.eq(session.node)) return closeEditor()

  if (target.node.textContent !== nextSource) {
    const content = nextSource.length ? target.editor.schema.text(nextSource) : Fragment.empty
    try {
      const transaction = target.editor.state.tr.replaceWith(
        target.position + 1,
        target.position + target.node.nodeSize - 1,
        content,
      )
      target.editor.view.dispatch(transaction)
    } catch {
      return closeEditor()
    }
  }

  const updatedNode = target.editor.state.doc.nodeAt(target.position)
  if (!updatedNode || updatedNode.type.name !== 'codeBlock') return closeEditor()
  session.node = updatedNode
}

function finishEditing() {
  const session = editSession
  if (!session) return closeEditor()

  const context = getEditorContext()
  const target = getCurrentMermaidTarget()
  if (
    context.noteId !== session.noteId
    || context.generation !== session.generation
    || !target
    || !target.node.eq(session.node)
  ) return closeEditor()

  closeEditor()
}

function getCurrentMermaidTarget(requireEditable = true) {
  const editor = props.editor
  if (!editor || editor.isDestroyed || (requireEditable && editor.isEditable !== true)) return null

  let position: number | undefined
  try {
    position = props.getPos()
  } catch {
    return null
  }
  if (typeof position !== 'number') return null

  const node = editor.state.doc.nodeAt(position)
  if (
    !node
    || node.type.name !== 'codeBlock'
    || String(node.attrs.language ?? '').trim().toLowerCase() !== 'mermaid'
  ) return null

  return { editor, position, node }
}

function closeEditor() {
  editSession = null
  editDialogOpen.value = false
}

function handleDialogOpen(open: boolean) {
  editDialogOpen.value = open
  if (!open) editSession = null
}

watch(
  () => [
    props.node.attrs.language,
    props.node.textContent,
    appStore.theme,
  ],
  scheduleRender,
  { immediate: true },
)

watch(canEdit, (editable) => {
  if (editable) return
  editDialogOpen.value = false
  // Let the controlled dialog emit its final DOM value before the session is
  // discarded. This matters when read-only mode begins during IME input.
  void nextTick(() => {
    if (!editDialogOpen.value) editSession = null
  })
})

onBeforeUnmount(() => {
  getEditorStorage()?.mermaidEditorFlushers?.delete(editorInputFlusher)
  getEditorStorage()?.mermaidEditorInputLockers?.delete(editorInputLocker)
  editSession = null
  renderGeneration += 1
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = null
  clearPreview()
})
</script>
