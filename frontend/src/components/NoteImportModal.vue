<template>
  <DialogRoot :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay class="note-import-overlay" />
      <DialogContent as-child @open-auto-focus="handleOpenAutoFocus">
        <form class="note-import-modal" @submit.prevent="submit">
          <header class="note-import-header">
            <div>
              <DialogTitle as="h2">ノートをインポート</DialogTitle>
              <DialogDescription>Markdown、テキスト、HTMLファイルを新しいノートとして取り込みます。</DialogDescription>
            </div>
            <button
              class="icon-btn"
              type="button"
              title="閉じる"
              :disabled="isBusy"
              @click="close"
            >
              <XIcon :size="18" />
            </button>
          </header>

          <div class="note-import-body">
            <fieldset class="note-import-fieldset" :disabled="isBusy">
              <legend>保存先</legend>
              <label class="note-import-option">
                <input v-model="destinationMode" type="radio" value="root">
                最上位階層
              </label>
              <label class="note-import-option">
                <input v-model="destinationMode" type="radio" value="notebook">
                既存のノートブック
              </label>
              <select
                v-if="destinationMode === 'notebook'"
                v-model="selectedNotebookId"
                class="note-import-input"
                aria-label="保存先ノートブック"
              >
                <option value="" disabled>ノートブックを選択</option>
                <option v-for="notebook in notebookStore.notebooks" :key="notebook.id" :value="notebook.id">
                  {{ notebook.name }}
                </option>
              </select>
              <label class="note-import-option">
                <input v-model="destinationMode" type="radio" value="new-notebook">
                新しいトップレベルノートブック
              </label>
              <input
                v-if="destinationMode === 'new-notebook'"
                ref="newNotebookNameInput"
                v-model="newNotebookName"
                class="note-import-input"
                type="text"
                maxlength="200"
                placeholder="ノートブック名"
              >
            </fieldset>

            <fieldset class="note-import-fieldset" :disabled="isBusy">
              <legend>タイトル</legend>
              <label class="note-import-option" for="note-import-title-mode">タイトルに使用する情報</label>
              <select
                id="note-import-title-mode"
                v-model="titleMode"
                class="note-import-input"
              >
                <option value="auto">自動</option>
                <option value="filename">ファイル名</option>
                <option value="heading">先頭見出し</option>
                <option value="metadata">メタデータ</option>
              </select>
              <p class="note-import-help">選択した情報がない場合は、ファイル名を使用します。</p>
            </fieldset>

            <p class="note-import-help">
              「ファイルを選択してインポート」を押すと、対応するファイルを複数選択できます。
            </p>
            <p v-if="localError || importStore.error" class="note-import-error" role="alert">
              {{ localError || importStore.error?.message }}
            </p>
            <p v-if="importStore.lastResult?.cancelled" class="note-import-info" role="status">
              ファイルの選択をキャンセルしました。
            </p>

            <section v-if="importStore.lastResult && !importStore.lastResult.cancelled" class="note-import-result" aria-live="polite">
              <p v-if="importStore.lastResult.imported.length > 0" class="note-import-success">
                {{ importStore.lastResult.imported.length }}件のノートを取り込みました。
              </p>
              <p v-if="importStore.lastResult.error && importStore.lastResult.imported.length > 0" class="note-import-warning">
                保存エラーのため、残りのファイルは取り込んでいません。成功したノートは保持されています。
              </p>
              <ul v-if="importStore.lastResult.failures.length > 0" class="note-import-failures">
                <li v-for="failure in importStore.lastResult.failures" :key="`${failure.sourceName}:${failure.code}`">
                  <strong>{{ failure.sourceName }}</strong>: {{ failure.message }}
                </li>
              </ul>
            </section>
          </div>

          <footer class="note-import-footer">
            <button class="secondary-btn" type="button" :disabled="isBusy" @click="close">キャンセル</button>
            <button class="primary-btn" type="submit" :disabled="isBusy">
              {{ isBusy ? 'インポート中…' : 'ファイルを選択してインポート' }}
            </button>
          </footer>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { XIcon } from '@lucide/vue'
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'
import type { NoteImportInput, NoteImportResult, NoteImportTitleMode } from '../api/noteImport'
import { useContentLockStore } from '../stores/useContentLockStore'
import { useNoteImportStore } from '../stores/useNoteImportStore'
import { useNotebookStore } from '../stores/useNotebookStore'

type DestinationMode = 'root' | 'notebook' | 'new-notebook'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  completed: [result: NoteImportResult]
}>()

const notebookStore = useNotebookStore()
const contentLockStore = useContentLockStore()
const importStore = useNoteImportStore()
const destinationMode = ref<DestinationMode>('root')
const titleMode = ref<NoteImportTitleMode>('auto')
const selectedNotebookId = ref('')
const newNotebookName = ref('')
const newNotebookNameInput = ref<HTMLInputElement | null>(null)
const localError = ref('')
const isPreparing = ref(false)
const isBusy = computed(() => isPreparing.value || importStore.isBusy)

watch(() => props.open, (open) => {
  if (!open) return
  destinationMode.value = 'root'
  titleMode.value = 'auto'
  selectedNotebookId.value = ''
  newNotebookName.value = ''
  localError.value = ''
  importStore.reset()
})

function handleOpenChange(open: boolean) {
  if (!open) close()
}

function handleOpenAutoFocus(event: Event) {
  event.preventDefault()
}

async function submit() {
  if (isBusy.value) return
  localError.value = ''

  const input = prepareInput()
  if (!input) return

  isPreparing.value = true
  try {
    if (destinationMode.value === 'notebook') {
      const notebook = notebookStore.notebooks.find((candidate) => candidate.id === selectedNotebookId.value)
      const accessAllowed = await contentLockStore.requestAccess(
        { type: 'notebook', id: selectedNotebookId.value },
        notebook?.name ?? 'ノートブック',
      )
      if (!accessAllowed) return
    }

    const result = await importStore.run(input)
    if (!result || result.cancelled) return
    if (result.imported.length > 0) emit('completed', result)
  } finally {
    isPreparing.value = false
  }
}

function prepareInput(): NoteImportInput | null {
  const input: NoteImportInput = { titleMode: titleMode.value }
  if (destinationMode.value === 'root') return input
  if (destinationMode.value === 'notebook') {
    if (selectedNotebookId.value) return { ...input, notebookId: selectedNotebookId.value }
    localError.value = '保存先のノートブックを選択してください。'
    return null
  }

  const name = newNotebookName.value.trim()
  if (!name) {
    localError.value = '新しいノートブック名を入力してください。'
    nextTick(() => newNotebookNameInput.value?.focus())
    return null
  }
  return { ...input, newNotebookName: name }
}

function close() {
  if (isBusy.value) return
  emit('close')
}
</script>

<style scoped>
.note-import-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgba(0, 0, 0, 0.5);
}

.note-import-modal {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1101;
  display: flex;
  width: min(520px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  flex-direction: column;
  transform: translate(-50%, -50%);
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.35);
}

.note-import-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}

.note-import-header h2,
.note-import-header p {
  margin: 0;
}

.note-import-header h2 {
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.note-import-header p {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
}

.note-import-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  overflow-y: auto;
}

.note-import-fieldset {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  border: 0;
}

.note-import-fieldset legend {
  margin-bottom: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.note-import-option {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-primary);
  font-size: 13px;
}

.note-import-input {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
}

.note-import-input:focus {
  border-color: var(--brand-primary);
}

.note-import-help,
.note-import-info,
.note-import-success,
.note-import-warning,
.note-import-error {
  margin: 0;
  font-size: 12px;
}

.note-import-help,
.note-import-info {
  color: var(--text-secondary);
}

.note-import-error {
  color: var(--color-danger);
}

.note-import-success {
  color: var(--color-success, #2f9e63);
}

.note-import-warning {
  margin-top: 6px;
  color: var(--color-warning, #c98300);
}

.note-import-result {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 4px;
  border-top: 1px solid var(--border);
}

.note-import-failures {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 160px;
  margin: 0;
  padding-left: 20px;
  overflow-y: auto;
  color: var(--text-secondary);
  font-size: 12px;
}

.note-import-failures strong {
  color: var(--text-primary);
}

.note-import-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px;
  border-top: 1px solid var(--border);
}

.primary-btn,
.secondary-btn {
  min-width: 104px;
  height: 34px;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
}

.primary-btn {
  background: var(--brand-primary);
  color: #fff;
}

.primary-btn:hover:not(:disabled) {
  background: var(--brand-hover);
}

.secondary-btn {
  border-color: var(--border);
  background: var(--bg-input);
  color: var(--text-primary);
}

.secondary-btn:hover:not(:disabled) {
  background: var(--bg-hover);
}

.primary-btn:disabled,
.secondary-btn:disabled,
.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
</style>
