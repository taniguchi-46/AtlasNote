<template>
  <section
    v-if="noteStore.activeNote && !noteStore.activeNote.isTrashed"
    class="ai-v3-panel"
    aria-label="AIアシスタント"
    aria-live="polite"
  >
    <div class="ai-v3-heading">
      <strong>AIアシスタント</strong>
      <span v-if="assistantStore.isBusy">準備・生成中…</span>
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
        <pre>{{ message.content }}</pre>
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
        :disabled="assistantStore.state === 'stale' || assistantStore.state === 'orphaned' || assistantStore.isBusy"
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
import type { AIContextSource, AssistantKind } from '../api/ai'
import { useAIStore } from '../stores/useAIStore'
import { useAIAssistantStore } from '../stores/useAIAssistantStore'
import { useNoteStore } from '../stores/useNoteStore'

const props = withDefaults(defineProps<{ externalComposer?: boolean }>(), {
  externalComposer: false,
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
  && question.value.trim()
  && aiStore.configuredSetting?.modelID
  && !assistantStore.isBusy,
))

function contextInput() {
  const noteID = noteStore.activeNote?.id
  return {
    kind: kind.value,
    question: question.value.trim(),
    noteIDs: noteID ? [noteID] : [],
    searchQuery: searchQuery.value.trim(),
    includeBacklinks: includeBacklinks.value,
  }
}

async function preview() {
  if (!noteStore.activeNote) return false
  return assistantStore.previewContext(contextInput())
}

async function confirmAndAsk() {
  if (!canAsk.value || !noteStore.activeNote) return false
  if (!await preview()) return false

  const setting = aiStore.configuredSetting
  if (!setting) return false
  const sourceSummary = assistantStore.contextSources.length > 0
    ? assistantStore.contextSources.map((source) => `・${source.title || '無題のノート'} (revision ${source.revision})`).join('\n')
    : '・参照資料なし'
  if (!window.confirm(
    `次の内容をAIへ送信します。\n\nプロバイダー: ${setting.providerID}\nモデル: ${setting.modelID}\n検索範囲: 全ノート（追加検索: ${searchQuery.value.trim() || 'なし'}）\n本文送信範囲: 各ノート最大16 KiB、合計48 KiBまで\n参照資料:\n${sourceSummary}\n\n質問・応答は自動保存されません。`,
  )) return false

  return assistantStore.ask({
    kind: kind.value,
    question: question.value,
    noteIDs: contextInput().noteIDs,
    searchQuery: searchQuery.value,
    includeBacklinks: includeBacklinks.value,
  })
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
