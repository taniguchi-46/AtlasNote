<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    class="ai-v3-panel"
    aria-label="AIライティング"
    aria-live="polite"
  >
    <div class="ai-v3-heading">
      <strong>AIライティング</strong>
      <span v-if="writingStore.isBusy">準備・生成中…</span>
    </div>

    <div class="ai-v3-grid">
      <label>
        <span>出力種別</span>
        <select v-model="kind" :disabled="writingStore.isBusy">
          <option v-for="item in writingKinds" :key="item.value" :value="item.value">
            {{ item.label }}
          </option>
        </select>
      </label>
      <label>
        <span>追加検索（任意）</span>
        <input
          v-model="searchQuery"
          type="search"
          maxlength="2000"
          placeholder="ノートを検索して参照資料に追加"
          :disabled="writingStore.isBusy"
        />
      </label>
    </div>

    <label class="ai-v3-checkbox">
      <input v-model="includeBacklinks" type="checkbox" :disabled="writingStore.isBusy" />
      <span>現在のノートへのバックリンクも参照する</span>
    </label>

    <form v-if="!props.externalComposer" class="ai-v3-form" @submit.prevent="confirmAndGenerate">
      <label>
        <span>目的・指示</span>
        <textarea
          v-model="instruction"
          rows="3"
          maxlength="12000"
          placeholder="作成したい文章の目的、読者、形式を入力"
          :disabled="writingStore.isBusy"
          @keydown.ctrl.enter.prevent="confirmAndGenerate"
          @keydown.meta.enter.prevent="confirmAndGenerate"
        />
      </label>
      <div class="ai-v3-actions">
        <button
          class="ai-v3-icon-button"
          type="button"
          title="参照を確認"
          aria-label="参照を確認"
          :disabled="writingStore.isBusy"
          @click="preview"
        >
          <SearchIcon :size="15" aria-hidden="true" />
        </button>
        <button
          class="ai-v3-icon-button"
          type="submit"
          title="文章を生成"
          aria-label="文章を生成"
          :disabled="!canGenerate"
        >
          <SparklesIcon :size="15" aria-hidden="true" />
        </button>
        <button
          v-if="writingStore.content"
          class="ai-v3-icon-button"
          type="button"
          title="結果をクリア"
          aria-label="結果をクリア"
          :disabled="writingStore.isBusy"
          @click="writingStore.clear()"
        >
          <EraserIcon :size="15" aria-hidden="true" />
        </button>
      </div>
    </form>
    <div v-else class="ai-v3-actions">
      <button
        class="ai-v3-icon-button"
        type="button"
        title="参照を確認"
        aria-label="参照を確認"
        :disabled="writingStore.isBusy"
        @click="preview"
      >
        <SearchIcon :size="15" aria-hidden="true" />
      </button>
      <button
        v-if="writingStore.content"
        class="ai-v3-icon-button"
        type="button"
        title="結果をクリア"
        aria-label="結果をクリア"
        :disabled="writingStore.isBusy"
        @click="writingStore.clear()"
      >
        <EraserIcon :size="15" aria-hidden="true" />
      </button>
    </div>

    <p class="ai-v3-privacy">生成結果は自動保存されません。必要なものだけ保存アイコンを押してください。保存データはこの端末のSQLiteだけに置かれ、WebDAV同期されません。</p>

    <div v-if="writingStore.contextSources.length > 0" class="ai-v3-context">
      <strong>今回の参照資料（{{ writingStore.contextSources.length }}件）</strong>
      <ul>
        <li v-for="source in writingStore.contextSources" :key="sourceKey(source)">
          <span>{{ source.title || '無題のノート' }}</span>
          <small>revision {{ source.revision }} / {{ formatBytes(source.contentByte) }}</small>
        </li>
      </ul>
    </div>

    <p v-if="writingStore.error" class="ai-v3-error" role="alert">
      {{ writingStore.error.message }}
    </p>
    <p v-if="writingStore.state === 'stale' || writingStore.state === 'orphaned'" class="ai-v3-warning" role="status">
      {{ writingStore.state === 'orphaned'
        ? '参照元ノートが削除されたため、この成果物は孤立しています。再生成してから保存してください。'
        : '参照元ノートが更新されたため、この成果物は古い参照を含みます。再生成してから保存してください。' }}
    </p>

    <label v-if="writingStore.content" class="ai-v3-result">
      <span>生成結果（編集してから保存できます）</span>
      <textarea
        ref="resultTextarea"
        v-model="writingStore.content"
        rows="1"
        @input="resizeResultTextarea"
      />
    </label>

    <div v-if="writingStore.content" class="ai-v3-save-row">
      <label>
        <span>成果物名</span>
        <input v-model="artifactTitle" type="text" maxlength="200" placeholder="例: README草案" />
      </label>
      <button
        class="ai-v3-icon-button"
        type="button"
        title="成果物を保存"
        aria-label="成果物を保存"
        :disabled="writingStore.state === 'stale' || writingStore.state === 'orphaned' || writingStore.isBusy"
        @click="saveArtifact"
      >
        <SaveIcon :size="15" aria-hidden="true" />
      </button>
      <button
        class="ai-v3-icon-button"
        type="button"
        :title="copied ? 'コピー済み' : '生成結果をコピー'"
        :aria-label="copied ? 'コピー済み' : '生成結果をコピー'"
        :disabled="!writingStore.content"
        @click="copyResult"
      >
        <CheckIcon v-if="copied" :size="15" aria-hidden="true" />
        <CopyIcon v-else :size="15" aria-hidden="true" />
      </button>
    </div>
    <p v-if="copyError" class="ai-v3-error" role="alert">{{ copyError }}</p>

  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { CheckIcon, CopyIcon, EraserIcon, SaveIcon, SearchIcon, SparklesIcon } from '@lucide/vue'
import { writeTableClipboard, createTableClipboardPayload } from '../utils/tableClipboard'
import type { AIContextSource, WritingKind } from '../api/ai'
import { useAIStore } from '../stores/useAIStore'
import { useAIWritingStore } from '../stores/useAIWritingStore'
import { useNoteStore } from '../stores/useNoteStore'

const props = withDefaults(defineProps<{ externalComposer?: boolean }>(), {
  externalComposer: false,
})
const noteStore = useNoteStore()
const aiStore = useAIStore()
const writingStore = useAIWritingStore()

const writingKinds: Array<{ value: WritingKind; label: string }> = [
  { value: 'prompt', label: 'プロンプト' },
  { value: 'prompt-improvement', label: 'プロンプト改善' },
  { value: 'readme', label: 'README草案' },
  { value: 'document', label: 'ドキュメント草案' },
  { value: 'blog', label: 'ブログ草案' },
  { value: 'requirements', label: '要件定義草案' },
]

const kind = ref<WritingKind>('document')
const instruction = ref('')
const searchQuery = ref('')
const includeBacklinks = ref(false)
const artifactTitle = ref('')
const copied = ref(false)
const copyError = ref('')
const resultTextarea = ref<HTMLTextAreaElement | null>(null)
let previousNoteID: string | null = null

const canGenerate = computed(() => Boolean(
  noteStore.activeNote
  && instruction.value.trim()
  && aiStore.configuredSetting?.modelID
  && !writingStore.isBusy,
))

function contextInput() {
  const noteID = noteStore.activeNote?.id
  return {
    noteIDs: noteID ? [noteID] : [],
    searchQuery: searchQuery.value.trim(),
    includeBacklinks: includeBacklinks.value,
  }
}

async function preview() {
  return writingStore.previewContext(contextInput())
}

async function confirmAndGenerate() {
  if (!canGenerate.value) return false
  if (!await preview()) return false

  const setting = aiStore.configuredSetting
  if (!setting) return false
  const sourceSummary = writingStore.contextSources.length > 0
    ? writingStore.contextSources.map((source) => `・${source.title || '無題のノート'} (revision ${source.revision})`).join('\n')
    : '・参照資料なし'
  if (!window.confirm(
    `次の内容をAIへ送信します。\n\nプロバイダー: ${setting.providerID}\nモデル: ${setting.modelID}\n検索範囲: 全ノート（追加検索: ${searchQuery.value.trim() || 'なし'}）\n本文送信範囲: 各ノート最大16 KiB、合計48 KiBまで\n参照資料:\n${sourceSummary}\n\n生成結果は自動保存されません。`,
  )) return false

  return writingStore.generate({
    providerID: setting.providerID,
    modelID: setting.modelID,
    kind: kind.value,
    instruction: instruction.value,
    ...contextInput(),
  })
}

async function submitPrompt(prompt: string) {
  instruction.value = prompt.trim()
  return confirmAndGenerate()
}

async function saveArtifact() {
  const label = writingKinds.find((item) => item.value === kind.value)?.label ?? 'AI成果物'
  const title = artifactTitle.value.trim() || `${label} ${new Date().toLocaleString('ja-JP')}`
  if (await writingStore.save(title)) artifactTitle.value = title
}

async function openArtifact(id: string) {
  const artifact = writingStore.artifacts.find((item) => item.id === id)
  if (!artifact || !await writingStore.loadArtifact(id)) return false
  kind.value = artifact.kind
  artifactTitle.value = artifact.title
  return true
}

async function copyResult() {
  copyError.value = ''
  try {
    await writeTableClipboard(createTableClipboardPayload(writingStore.content))
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1600)
  } catch {
    copyError.value = '生成結果をクリップボードへコピーできませんでした。'
  }
}

function resizeResultTextarea() {
  const textarea = resultTextarea.value
  if (!textarea) return
  textarea.style.height = 'auto'
  textarea.style.height = `${textarea.scrollHeight}px`
}

function sourceKey(source: AIContextSource) {
  return `${source.noteID}:${source.revision}`
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  return `${(value / 1024).toFixed(1)} KiB`
}

watch(
  () => [noteStore.activeNote?.id ?? null, noteStore.activeNote?.revision ?? null] as const,
  ([noteID, revision]) => {
    if (!noteID) {
      writingStore.clear()
      previousNoteID = null
      return
    }
    if (previousNoteID && previousNoteID !== noteID) writingStore.clear()
    if (typeof revision === 'number') writingStore.markStaleForRevision(noteID, revision)
    previousNoteID = noteID
  },
  { immediate: true },
)

watch(
  () => writingStore.content,
  () => {
    void nextTick(resizeResultTextarea)
  },
  { flush: 'post' },
)

defineExpose({ openArtifact, submitPrompt })

onBeforeUnmount(() => {
  writingStore.clear()
})
</script>

<style scoped>
.ai-v3-panel {
  display: grid;
  gap: 10px;
  margin: 10px 14px 0;
  padding: 10px;
  border: 1px solid var(--border-color, var(--border));
  border-radius: 8px;
  background: var(--panel-background, var(--bg-sidebar));
  font-size: 12px;
}

.ai-v3-heading,
.ai-v3-actions,
.ai-v3-save-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-v3-heading span,
.ai-v3-privacy,
.ai-v3-context small {
  color: var(--text-secondary);
}

.ai-v3-grid {
  display: grid;
  grid-template-columns: minmax(130px, 0.35fr) minmax(220px, 1fr);
  gap: 8px;
}

.ai-v3-grid label,
.ai-v3-form label,
.ai-v3-result,
.ai-v3-save-row label {
  display: grid;
  gap: 4px;
}

.ai-v3-grid input,
.ai-v3-grid select,
.ai-v3-form textarea,
.ai-v3-result textarea,
.ai-v3-save-row input {
  box-sizing: border-box;
  width: 100%;
  border: 1px solid var(--border-color, var(--border));
  border-radius: 5px;
  padding: 6px 8px;
  background: var(--bg-input);
  color: var(--text-primary);
  font: inherit;
}

.ai-v3-form textarea,
.ai-v3-result textarea {
  resize: vertical;
}

.ai-v3-checkbox {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ai-v3-actions {
  flex-wrap: wrap;
}

.ai-v3-icon-button {
  display: inline-grid;
  box-sizing: border-box;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
  border: 1px solid var(--border-color, var(--border));
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.ai-v3-actions button[type='submit'],
.ai-v3-save-row button:first-of-type {
  background: var(--text-secondary);
  color: var(--bg-editor);
}

.ai-v3-icon-button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.ai-v3-privacy,
.ai-v3-error,
.ai-v3-warning {
  margin: 0;
}

.ai-v3-context {
  display: grid;
  gap: 5px;
}

.ai-v3-context ul {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-v3-context li {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.ai-v3-error {
  color: var(--color-danger, #b42318);
}

.ai-v3-warning {
  color: var(--color-warning, #8a5a00);
}

.ai-v3-result textarea {
  min-height: 120px;
  overflow: hidden;
  resize: none;
  line-height: 1.5;
}

.ai-v3-save-row {
  align-items: end;
}

.ai-v3-save-row label {
  flex: 1;
}

@container (max-width: 420px) {
  .ai-v3-grid {
    grid-template-columns: 1fr;
  }

  .ai-v3-save-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
