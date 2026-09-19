<template>
  <DialogRoot :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="mermaid-edit-dialog-overlay" />
      <DialogContent
        class="mermaid-edit-dialog-content"
        @escape-key-down="handleEscape"
        @interact-outside="handleInteractOutside"
      >
        <DialogTitle as="h2">Mermaid図を編集</DialogTitle>
        <DialogDescription class="mermaid-edit-dialog-description">キャンバス上で図を選択・移動・編集できます。</DialogDescription>

        <div class="mermaid-edit-dialog-actions">
          <button type="button" @click="close">閉じる</button>
          <button
            type="button"
            class="mermaid-edit-dialog-save"
            :disabled="isComposing || isInputLocked"
            @click="save"
          >
            保存
          </button>
        </div>

        <div class="mermaid-edit-dialog-grid">
          <MermaidVisualEditor
            ref="visualEditorRef"
            :source="localSource"
            :disabled="isInputLocked"
            class="mermaid-edit-dialog-visual"
            @update:source="handleVisualSourceUpdate"
          />
        </div>
        <div class="mermaid-edit-dialog-render-status" :class="`is-${status}`" aria-live="polite" :aria-busy="status === 'loading'">
          <span v-if="status === 'loading'">Mermaid図を確認しています…</span>
          <span v-else-if="status === 'error'" role="alert">{{ errorMessage }}</span>
          <span v-else-if="status === 'ready'">Mermaidの構文を確認しました。</span>
        </div>
        <details v-if="status === 'ready' && svgUrl" class="mermaid-edit-dialog-render-preview">
          <summary>Mermaidレンダラーの出力を確認</summary>
          <div class="mermaid-edit-dialog-render-preview-viewport">
            <img :src="svgUrl" alt="Mermaid図のレンダラー出力" />
          </div>
        </details>
        <section class="mermaid-edit-dialog-source-panel">
          <button type="button" class="mermaid-edit-dialog-source-toggle" :aria-expanded="sourceOpen" @click="sourceOpen = !sourceOpen"><span>&lt;/&gt;</span> Mermaidソース <span class="source-chevron">{{ sourceOpen ? '⌃' : '⌄' }}</span></button>
          <label v-if="sourceOpen" class="mermaid-edit-dialog-label" for="mermaid-edit-source">
            <span class="visually-hidden">Mermaidソース</span>
            <textarea id="mermaid-edit-source" ref="sourceInput" :value="localSource" class="mermaid-edit-dialog-source" :readonly="isInputLocked" :aria-readonly="isInputLocked ? 'true' : undefined" spellcheck="false" @input="handleSourceInput" @compositionstart="handleCompositionStart" @compositionend="handleCompositionEnd" />
          </label>
        </section>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import { renderMermaidDiagram, type MermaidTheme } from '../utils/mermaidRenderer'
import MermaidVisualEditor from './MermaidVisualEditor.vue'

type MermaidVisualEditorExpose = {
  flushInput: () => void
  setInputLocked: (locked: boolean) => void
}

const props = defineProps<{
  open: boolean
  source: string
  theme: MermaidTheme
  inputLocked?: boolean
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  'update:source': [source: string]
  save: []
}>()

const localSource = ref(props.source)
const sourceOpen = ref(false)
const visualEditorRef = ref<MermaidVisualEditorExpose | null>(null)
const sourceInput = ref<HTMLTextAreaElement | null>(null)
const status = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const errorMessage = ref('')
const svgUrl = ref<string | null>(null)
const isComposing = ref(false)
const sourceInputDirty = ref(false)
const localInputLocked = ref(false)
const isInputLocked = computed(() => props.inputLocked === true || localInputLocked.value)
let lastEmittedSource = props.source

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
}

function schedulePreview() {
  renderGeneration += 1
  const generation = renderGeneration
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = null

  if (!props.open) {
    status.value = 'idle'
    errorMessage.value = ''
    clearPreview()
    return
  }

  status.value = 'loading'
  errorMessage.value = ''
  renderTimer = setTimeout(() => {
    renderTimer = null
    void renderPreview(generation)
  }, 250)
}

function setLocalSource(source: string) {
  lastEmittedSource = source
  localSource.value = source
  sourceInputDirty.value = false
}

function emitSourceUpdate(source = localSource.value) {
  if (source === lastEmittedSource) return

  lastEmittedSource = source
  emit('update:source', source)
}

function handleSourceInput(event: Event) {
  const inputValue = (event.currentTarget as HTMLTextAreaElement | null)?.value
  if (inputValue === undefined) return
  if (isInputLocked.value) {
    const input = event.currentTarget as HTMLTextAreaElement | null
    if (input && input.value !== localSource.value) input.value = localSource.value
    return
  }

  // Read the value from the input event itself. This also captures browsers'
  // composition input that v-model intentionally defers until compositionend.
  sourceInputDirty.value = true
  if (inputValue !== localSource.value) localSource.value = inputValue
  if (!isComposing.value) emitSourceUpdate(inputValue)
}

function handleVisualSourceUpdate(source: string) {
  if (isInputLocked.value) return
  if (source !== localSource.value) localSource.value = source
  sourceInputDirty.value = false
  emitSourceUpdate(source)
}

function commitSourceFromInput() {
  const input = sourceInput.value
  const sourceBeforeVisualFlush = localSource.value
  visualEditorRef.value?.flushInput()
  const visualSourceChanged = localSource.value !== sourceBeforeVisualFlush
  if (isInputLocked.value) {
    if (input && input.value !== localSource.value) {
      input.value = localSource.value
    }
    return
  }

  // The visual editor flushes its inline IME value synchronously. When the
  // source panel is open, Vue may not have patched the textarea DOM yet, so
  // reading it unconditionally can overwrite a newer visual value with a
  // stale snapshot. The textarea wins only when it actually received input
  // and the visual flush did not produce a newer source. A focused textarea
  // alone is not enough while Vue is still patching its previous value.
  if (input && !visualSourceChanged && sourceInputDirty.value) {
    const inputValue = input.value
    if (inputValue !== localSource.value) localSource.value = inputValue
    emitSourceUpdate(inputValue)
  } else {
    emitSourceUpdate()
  }
  sourceInputDirty.value = false
}

async function renderPreview(generation: number) {
  let result: Awaited<ReturnType<typeof renderMermaidDiagram>>
  try {
    result = await renderMermaidDiagram(localSource.value, { theme: props.theme })
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
  status.value = 'ready'
}

watch(localSource, schedulePreview)

watch(
  () => props.open,
  async (open) => {
    if (!open) commitSourceFromInput()
    if (open) {
      setLocalSource(props.source)
      sourceOpen.value = false
      await nextTick()
      if (props.open) sourceInput.value?.focus()
    }
    schedulePreview()
  },
  { immediate: true },
)

watch(() => props.source, (source) => {
  if (source === localSource.value) {
    lastEmittedSource = source
    return
  }

  // The source is controlled by the Tiptap node. A source change while the
  // dialog is open is therefore an external document change, not a theme
  // change, and should be mirrored instead of being silently discarded.
  setLocalSource(source)
})

watch(() => props.theme, schedulePreview)

function handleCompositionStart() {
  isComposing.value = true
}

function handleCompositionEnd() {
  isComposing.value = false
  void nextTick(commitSourceFromInput)
}

function close() {
  // Commit the latest DOM value even when the IME composition has just ended.
  // Closing the dialog must not turn its source editor into a discard dialog.
  commitSourceFromInput()
  emit('update:open', false)
}

function save() {
  if (isComposing.value) return
  commitSourceFromInput()
  emit('save')
  emit('update:open', false)
}

function handleOpenChange(open: boolean) {
  if (!open) close()
}

function handleEscape(event: Event) {
  event.preventDefault()
  close()
}

function handleInteractOutside(event: Event) {
  event.preventDefault()
  close()
}

function flushInput() {
  // Locking can interrupt an active composition before compositionend fires.
  // The DOM value is the only complete snapshot available at that point.
  commitSourceFromInput()
}

function setInputLocked(locked: boolean) {
  localInputLocked.value = locked
  visualEditorRef.value?.setInputLocked(locked)
}

defineExpose({ flushInput, setInputLocked })

onBeforeUnmount(() => {
  renderGeneration += 1
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = null
  clearPreview()
})
</script>

<style scoped>
:global(.mermaid-edit-dialog-overlay) {
  position: fixed;
  inset: 0;
  z-index: 1600;
  background: rgba(0, 0, 0, 0.58);
}

:global(.mermaid-edit-dialog-content) {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1601;
  display: flex;
  width: min(920px, calc(100vw - 32px));
  max-height: min(760px, calc(100vh - 32px));
  flex-direction: column;
  transform: translate(-50%, -50%);
  padding: 22px;
  overflow: auto;
  border: 1px solid var(--border-strong, var(--border));
  border-radius: 10px;
  background: var(--bg-editor);
  color: var(--text-primary);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.42);
  outline: none;
}

:global(.mermaid-edit-dialog-content h2) {
  margin: 0;
  font-size: 18px;
}

.mermaid-edit-dialog-description {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.mermaid-edit-dialog-grid {
  display: grid;
  order: 1;
  grid-template-columns: minmax(280px, 1fr) minmax(280px, 1fr);
  gap: 14px;
  min-height: 360px;
  margin-top: 18px;
}

.mermaid-edit-dialog-visual {
  min-width: 0;
}

.mermaid-edit-dialog-render-status {
  min-height: 18px;
  margin-top: 8px;
  color: var(--text-secondary);
  font-size: 11px;
  line-height: 1.5;
}

.mermaid-edit-dialog-render-status.is-error {
  color: var(--color-danger, #c0392b);
}

.mermaid-edit-dialog-render-preview {
  margin-top: 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 11px;
}

.mermaid-edit-dialog-render-preview summary {
  padding: 7px 9px;
  cursor: pointer;
}

.mermaid-edit-dialog-render-preview-viewport {
  max-height: 180px;
  padding: 8px;
  overflow: auto;
  border-top: 1px solid var(--border);
  background: var(--bg-input);
}

.mermaid-edit-dialog-render-preview-viewport img {
  display: block;
  width: max-content;
  min-width: 160px;
  max-width: 100%;
  height: auto;
}

.mermaid-edit-dialog-label {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 7px;
  color: var(--text-secondary);
  font-size: 13px;
}

.mermaid-edit-dialog-source {
  box-sizing: border-box;
  flex: 1;
  width: 100%;
  min-height: 320px;
  resize: vertical;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-family: var(--editor-font-family, monospace);
  font-size: 13px;
  line-height: 1.5;
  outline: none;
}

.mermaid-edit-dialog-source:focus-visible {
  border-color: var(--brand-primary);
  outline: 2px solid color-mix(in srgb, var(--brand-primary) 28%, transparent);
}

.mermaid-edit-dialog-actions {
  display: flex;
  order: 2;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}

.mermaid-edit-dialog-actions button {
  min-height: 34px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.mermaid-edit-dialog-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.mermaid-edit-dialog-actions .mermaid-edit-dialog-save {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: #fff;
}

@media (max-width: 720px) {
  .mermaid-edit-dialog-grid {
    grid-template-columns: 1fr;
  }
}

.mermaid-edit-dialog-content {
  width: min(1180px, calc(100vw - 32px));
  max-height: min(900px, calc(100vh - 24px));
  padding: 20px 28px 18px;
  overflow-x: hidden;
  overflow-y: auto;
}

.mermaid-edit-dialog-grid {
  display: block;
  min-height: 0;
  margin-top: 14px;
  order: 1;
}

.mermaid-edit-dialog-visual { min-height: 0; }
.mermaid-edit-dialog-source-panel { order: 2; margin-top: 12px; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-input); }
.mermaid-edit-dialog-actions { order: 3; }
.mermaid-edit-dialog-source-toggle { display:flex; width:100%; align-items:center; gap:8px; min-height:42px; padding:0 12px; border:0; background:transparent; color:var(--text-primary); cursor:pointer; font:inherit; font-size:13px; text-align:left; }
.mermaid-edit-dialog-source-toggle span:first-child { color:var(--brand-primary); font-family:var(--editor-font-family, monospace); font-weight:700; }
.mermaid-edit-dialog-source-toggle .source-chevron { margin-left:auto; color:var(--text-secondary); font-size:16px; }
.mermaid-edit-dialog-source-panel .mermaid-edit-dialog-label { display:block; padding:0 10px 10px; }
.mermaid-edit-dialog-source-panel .mermaid-edit-dialog-source { min-height:150px; }
.visually-hidden { position:absolute; width:1px; height:1px; padding:0; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }

@media (max-width: 720px) {
  .mermaid-edit-dialog-content { width:calc(100vw - 16px); padding:14px; }
}
</style>
