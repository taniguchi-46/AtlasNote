<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    :class="['ai-v3-panel', { 'is-execution-bridge': props.executionBridge }]"
    aria-label="AIアシスタント"
    aria-live="polite"
    :aria-busy="assistantStore.isBusy"
  >
    <div class="ai-v3-heading">
      <strong>AIアシスタント</strong>
      <span v-if="assistantStore.state === 'loading-context'">参照を確認中…</span>
      <span v-else-if="assistantStore.state === 'generating'">送信済み・応答待ち…</span>
    </div>

    <div class="ai-v3-grid">
      <label>
        <span>モード</span>
        <select v-model="kind" :disabled="assistantStore.isBusy">
          <option value="qa">質問応答</option>
          <option value="brainstorm">ブレインストーミング</option>
        </select>
      </label>
      <label>
        <span>追加検索（任意）</span>
        <input
          v-model="searchQuery"
          type="search"
          maxlength="2000"
          placeholder="ノートを検索して参照資料に追加"
          :disabled="assistantStore.isBusy"
        />
      </label>
    </div>

    <label class="ai-v3-checkbox">
      <input v-model="includeBacklinks" type="checkbox" :disabled="assistantStore.isBusy" />
      <span>現在のノートへのバックリンクも参照する</span>
    </label>

    <form v-if="!props.externalComposer" class="ai-v3-form" @submit.prevent="confirmAndAsk">
      <label>
        <span>質問・相談</span>
        <textarea
          v-model="question"
          rows="3"
          maxlength="8000"
          placeholder="現在のノートについて質問する"
          :disabled="assistantStore.isBusy"
          @keydown.ctrl.enter.prevent="confirmAndAsk"
          @keydown.meta.enter.prevent="confirmAndAsk"
        />
      </label>
      <div class="ai-v3-actions">
        <button
          class="ai-v3-icon-button"
          type="button"
          title="参照を確認"
          aria-label="参照を確認"
          :disabled="assistantStore.isBusy"
          @click="preview"
        >
          <SearchIcon :size="15" aria-hidden="true" />
        </button>
        <button
          class="ai-v3-icon-button"
          type="submit"
          title="質問を送信"
          aria-label="質問を送信"
          :disabled="!canAsk"
        >
          <SendIcon :size="15" aria-hidden="true" />
        </button>
        <button
          v-if="assistantStore.messages.length > 0"
          class="ai-v3-icon-button"
          type="button"
          title="会話をクリア"
          aria-label="会話をクリア"
          :disabled="assistantStore.isBusy"
          @click="assistantStore.clearConversation()"
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
        :disabled="assistantStore.isBusy"
        @click="preview"
      >
        <SearchIcon :size="15" aria-hidden="true" />
      </button>
      <button
        v-if="assistantStore.messages.length > 0"
        class="ai-v3-icon-button"
        type="button"
        title="会話をクリア"
        aria-label="会話をクリア"
        :disabled="assistantStore.isBusy"
        @click="assistantStore.clearConversation()"
      >
        <EraserIcon :size="15" aria-hidden="true" />
      </button>
    </div>

    <p class="ai-v3-privacy">質問・応答は自動保存されません。必要なものだけ保存アイコンを押してください。保存データはこの端末のSQLiteだけに置かれ、WebDAV同期されません。</p>

    <div v-if="assistantStore.contextSources.length > 0" class="ai-v3-context">
      <strong>今回の参照資料（{{ assistantStore.contextSources.length }}件）</strong>
      <ul>
        <li v-for="source in assistantStore.contextSources" :key="sourceKey(source)">
          <span>{{ source.title || '無題のノート' }}</span>
          <small>revision {{ source.revision }} / {{ formatBytes(source.contentByte) }}</small>
        </li>
      </ul>
    </div>

    <p v-if="assistantStore.error" class="ai-v3-error" role="alert">
      {{ assistantStore.error.message }}
    </p>
    <p v-if="assistantStore.state === 'stale' || assistantStore.state === 'orphaned'" class="ai-v3-warning" role="status">
        {{ assistantStore.state === 'orphaned'
          ? '参照元ノートが削除されたため、この会話は孤立しています。再生成してから保存してください。'
          : '参照元ノートが更新されたため、この会話は古い参照を含みます。再生成してから保存してください。' }}
    </p>

    <ul v-if="assistantStore.messages.length > 0" class="ai-v3-messages" aria-label="AI会話">
      <li
        v-for="(message, index) in assistantStore.messages"
        :key="`${index}-${message.role}`"
        :class="['ai-v3-message', `is-${message.role}`]"
      >
        <strong>{{ message.role === 'user' ? 'あなた' : 'AI' }}</strong>
        <AIMarkdownPreview
          v-if="message.role === 'assistant'"
          :markdown="message.content"
          aria-label="AIアシスタントの回答"
        />
        <pre v-else>{{ message.content }}</pre>
      </li>
    </ul>

    <div v-if="assistantStore.messages.length >= 2" class="ai-v3-save-row">
      <label>
        <span>履歴名</span>
        <input v-model="historyTitle" type="text" maxlength="200" placeholder="例: 設計メモの質問" />
      </label>
      <button
        class="ai-v3-icon-button"
        type="button"
        title="履歴を保存"
        aria-label="履歴を保存"
        :disabled="assistantStore.state !== 'success' || assistantStore.isBusy"
        @click="saveHistory"
      >
        <SaveIcon :size="15" aria-hidden="true" />
      </button>
    </div>

  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EraserIcon, SaveIcon, SearchIcon, SendIcon } from '@lucide/vue'
import type { AIChatMode, AIContextSource, AssistantKind } from '../api/ai'
import { useAIStore } from '../stores/useAIStore'
import { useAIAssistantStore } from '../stores/useAIAssistantStore'
import { useNoteStore } from '../stores/useNoteStore'
import AIMarkdownPreview from './AIMarkdownPreview.vue'

const props = withDefaults(defineProps<{
  externalComposer?: boolean
  executionBridge?: boolean
  additionalNoteIDs?: string[]
  chatMode?: AIChatMode
  webSearch?: boolean
}>(), {
  externalComposer: false,
  executionBridge: false,
  additionalNoteIDs: () => [],
  chatMode: 'ask',
  webSearch: false,
})
const noteStore = useNoteStore()
const aiStore = useAIStore()
const assistantStore = useAIAssistantStore()

const kind = ref<AssistantKind>('qa')
const question = ref('')
const searchQuery = ref('')
const includeBacklinks = ref(false)
const historyTitle = ref('')
let previousNoteID: string | null = null

const canAsk = computed(() => Boolean(
  noteStore.activeNote
  && !noteStore.activeNote.isTrashed
  && question.value.trim()
  && aiStore.configuredSetting?.modelID
  && !assistantStore.isBusy,
))

function contextInput() {
  const noteID = noteStore.activeNote?.id
  const noteIDs = [
    ...(noteID ? [noteID] : []),
    ...props.additionalNoteIDs,
  ]
    .filter((id, index, all) => Boolean(id) && all.indexOf(id) === index)
    .slice(0, 10)
  return {
    kind: kind.value,
    mode: props.chatMode,
    question: question.value.trim(),
    noteIDs,
    searchQuery: searchQuery.value.trim(),
    includeBacklinks: includeBacklinks.value,
    webSearch: props.webSearch,
  }
}

async function preview() {
  if (!await ensureCurrentNotePersisted()) return false
  return assistantStore.previewContext(contextInput())
}

async function ensureCurrentNotePersisted() {
  const selectedNote = noteStore.activeNote
  if (!selectedNote || selectedNote.isTrashed) {
    assistantStore.setPreconditionError('AI_NOTE_UNAVAILABLE')
    return false
  }
  if (noteStore.activeDraft?.status === 'conflicted' || noteStore.activeDraft?.status === 'failed') {
    assistantStore.setPreconditionError('AI_DRAFT_NOT_SAVED')
    return false
  }
  const noteID = selectedNote.id
  try {
    if (!await noteStore.flushPendingDraft()) {
      assistantStore.setPreconditionError('AI_DRAFT_NOT_SAVED')
      return false
    }
  } catch {
    assistantStore.setPreconditionError('AI_DRAFT_NOT_SAVED')
    return false
  }
  const current = noteStore.activeNote
  if (!current || current.id !== noteID || current.isTrashed || noteStore.activeDraft) {
    assistantStore.setPreconditionError(current?.isTrashed ? 'AI_NOTE_UNAVAILABLE' : 'AI_DRAFT_NOT_SAVED')
    return false
  }
  return true
}

async function confirmAndAsk() {
  if (!canAsk.value || !noteStore.activeNote) return false
  if (!await preview()) return false

  const setting = aiStore.configuredSetting
  if (!setting) return false
  if (props.webSearch && setting.providerID !== 'openrouter') {
    return false
  }
  const sourceSummary = assistantStore.contextSources.length > 0
    ? assistantStore.contextSources.map((source) => `・${source.title || '無題のノート'} (revision ${source.revision})`).join('\n')
    : '・参照資料なし'
  const localSearchSummary = searchQuery.value.trim()
    ? `全ノートから「${searchQuery.value.trim()}」を検索`
    : 'なし'
  const webSearchSummary = props.webSearch
    ? '\nWeb検索: 有効（OpenRouter Web Search / Exaを必須化します。各検索・合計とも最大3件で、実行回数が1回でない応答は表示しません。追加料金が発生します。質問・参照内容から生成された検索クエリはOpenRouterとExaへ外部送信されます。）'
    : '\nWeb検索: 無効'
  if (!window.confirm(
    `次の内容をAIへ送信します。\n\nプロバイダー: ${setting.providerID}\nモデル: ${setting.modelID}\nモード: ${props.chatMode === 'agent' ? 'Agent' : 'Ask'}\nローカル追加検索: ${localSearchSummary}${webSearchSummary}\n本文送信範囲: 各ノート最大16 KiB、合計48 KiBまで\n参照資料:\n${sourceSummary}\n\n質問・応答は自動保存されません。`,
  )) return false

  const asked = await assistantStore.ask({
    kind: kind.value,
    mode: props.chatMode,
    question: question.value,
    noteIDs: contextInput().noteIDs,
    searchQuery: searchQuery.value,
    includeBacklinks: includeBacklinks.value,
    webSearch: props.webSearch,
  })
  if (asked) question.value = ''
  return asked
}

async function submitPrompt(prompt: string) {
  question.value = prompt.trim()
  return confirmAndAsk()
}

async function saveHistory() {
  const title = historyTitle.value.trim() || `${kind.value === 'qa' ? '質問応答' : 'ブレインストーミング'} ${new Date().toLocaleString('ja-JP')}`
  if (await assistantStore.save(title)) historyTitle.value = title
}

async function openHistory(id: string) {
  const history = assistantStore.histories.find((item) => item.id === id)
  if (!history || !await assistantStore.loadHistory(id)) return false
  kind.value = history.kind
  historyTitle.value = history.title
  return true
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
      assistantStore.clearConversation()
      previousNoteID = null
      return
    }
    if (previousNoteID && previousNoteID !== noteID) assistantStore.clearConversation()
    if (typeof revision === 'number') assistantStore.markStaleForRevision(noteID, revision)
    previousNoteID = noteID
  },
  { immediate: true },
)

defineExpose({ openHistory, submitPrompt })

onBeforeUnmount(() => {
  assistantStore.clearConversation()
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

.ai-v3-panel.is-execution-bridge {
  display: none;
}

.ai-v3-heading,
.ai-v3-actions,
.ai-v3-save-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-v3-heading {
  justify-content: space-between;
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
.ai-v3-save-row label {
  display: grid;
  gap: 4px;
}

.ai-v3-grid input,
.ai-v3-grid select,
.ai-v3-form textarea,
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

.ai-v3-form textarea {
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
.ai-v3-save-row button {
  background: var(--text-secondary);
  color: var(--bg-editor);
}

.ai-v3-icon-button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.ai-v3-privacy {
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

.ai-v3-error,
.ai-v3-warning {
  margin: 0;
}

.ai-v3-error {
  color: var(--color-danger, #b42318);
}

.ai-v3-warning {
  color: var(--color-warning, #8a5a00);
}

.ai-v3-messages {
  display: grid;
  gap: 7px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-v3-message {
  display: grid;
  gap: 3px;
  padding: 7px;
  border-radius: 5px;
  background: var(--bg-input);
}

.ai-v3-message.is-assistant {
  border-left: 3px solid var(--text-secondary);
}

.ai-v3-message pre {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: inherit;
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
