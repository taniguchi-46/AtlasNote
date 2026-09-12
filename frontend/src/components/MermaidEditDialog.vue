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
        <DialogDescription class="mermaid-edit-dialog-description">
          Mermaidのソースを編集し、プレビューを確認できます。保存すると本文の図へ反映します。
        </DialogDescription>

        <div class="mermaid-edit-dialog-grid">
          <label class="mermaid-edit-dialog-label" for="mermaid-edit-source">
            ソース
            <textarea
              id="mermaid-edit-source"
              ref="sourceInput"
              :value="localSource"
              class="mermaid-edit-dialog-source"
              :readonly="isInputLocked"
              :aria-readonly="isInputLocked ? 'true' : undefined"
              spellcheck="false"
              @input="handleSourceInput"
              @compositionstart="handleCompositionStart"
              @compositionend="handleCompositionEnd"
            />
          </label>

          <div class="mermaid-edit-dialog-preview" aria-live="polite" :aria-busy="status === 'loading'">
            <p v-if="status === 'loading'" class="mermaid-edit-dialog-status" role="status">
              Mermaid図を描画しています…
            </p>
            <p v-else-if="status === 'error'" class="mermaid-edit-dialog-error" role="alert">
              {{ errorMessage }}
            </p>
            <img v-else-if="svgUrl" :src="svgUrl" alt="Mermaid図のプレビュー" class="mermaid-edit-dialog-image" />
          </div>
        </div>

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
const sourceInput = ref<HTMLTextAreaElement | null>(null)
const status = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const errorMessage = ref('')
const svgUrl = ref<string | null>(null)
const isComposing = ref(false)
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
  if (inputValue !== localSource.value) localSource.value = inputValue
  if (!isComposing.value) emitSourceUpdate(inputValue)
}

function commitSourceFromInput() {
  const inputValue = sourceInput.value?.value
  if (isInputLocked.value) {
    const input = sourceInput.value
    if (input && inputValue !== undefined && input.value !== localSource.value) {
      input.value = localSource.value
    }
    return
  }
  if (inputValue !== undefined) {
    if (inputValue !== localSource.value) localSource.value = inputValue
    emitSourceUpdate(inputValue)
    return
  }

  emitSourceUpdate()
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
  grid-template-columns: minmax(280px, 1fr) minmax(280px, 1fr);
  gap: 14px;
  min-height: 360px;
  margin-top: 18px;
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

.mermaid-edit-dialog-preview {
  display: grid;
  min-height: 320px;
  place-items: center;
  padding: 12px;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
}

.mermaid-edit-dialog-image {
  display: block;
  max-width: 100%;
  height: auto;
}

.mermaid-edit-dialog-status,
.mermaid-edit-dialog-error {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
  text-align: center;
}

.mermaid-edit-dialog-error {
  color: var(--color-danger, #c0392b);
}

.mermaid-edit-dialog-actions {
  display: flex;
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
</style>
