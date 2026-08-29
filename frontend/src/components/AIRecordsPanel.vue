<template>
  <section class="ai-records-panel" aria-label="AI履歴・成果物" aria-live="polite">
    <div class="ai-records-heading">
      <strong>履歴・成果物</strong>
    </div>
    <p class="ai-records-description">
      保存した会話・成果物と、生成成功時に自動保存する要約を表示します。AI司書の一時結果は保存されません。
    </p>

    <p v-if="assistantStore.error" class="ai-records-error" role="alert">
      {{ assistantStore.error.message }}
    </p>
    <p v-if="writingStore.error" class="ai-records-error" role="alert">
      {{ writingStore.error.message }}
    </p>
    <p v-if="aiStore.summaryHistoryError" class="ai-records-error" role="alert">
      {{ aiStore.summaryHistoryError.message }}
    </p>

    <section v-if="aiStore.summaryHistory.length > 0" class="ai-records-section" aria-label="保存済み要約">
      <div class="ai-records-section-heading">
        <strong>要約履歴</strong>
      </div>
      <ul>
        <li v-for="summary in aiStore.summaryHistory" :key="summary.id">
          <button class="ai-records-item-button" type="button" @click="emit('open-summary', summary.id)">
            {{ summary.title || '無題の要約' }}
          </button>
          <span :class="`is-${summary.status}`">{{ statusLabel(summary.status) }}</span>
          <button
            class="ai-records-icon-button is-danger"
            type="button"
            title="要約履歴を削除"
            aria-label="要約履歴を削除"
            @click="removeSummary(summary.id)"
          >
            <Trash2Icon :size="15" aria-hidden="true" />
          </button>
        </li>
      </ul>
    </section>

    <section v-if="assistantStore.histories.length > 0" class="ai-records-section" aria-label="保存済み履歴">
      <div class="ai-records-section-heading">
        <strong>保存済み履歴</strong>
        <button
          class="ai-records-icon-button is-danger"
          type="button"
          title="保存済み履歴をすべて削除"
          aria-label="保存済み履歴をすべて削除"
          @click="removeAllHistories"
        >
          <Trash2Icon :size="15" aria-hidden="true" />
        </button>
      </div>
      <ul>
        <li v-for="history in assistantStore.histories" :key="history.id">
          <button class="ai-records-item-button" type="button" @click="emit('open-history', history.id)">
            {{ history.title || '無題の履歴' }}
          </button>
          <span :class="`is-${history.status}`">{{ statusLabel(history.status) }}</span>
          <button
            class="ai-records-icon-button is-danger"
            type="button"
            title="履歴を削除"
            aria-label="履歴を削除"
            @click="removeHistory(history.id)"
          >
            <Trash2Icon :size="15" aria-hidden="true" />
          </button>
        </li>
      </ul>
    </section>

    <section v-if="writingStore.artifacts.length > 0" class="ai-records-section" aria-label="保存済み成果物">
      <div class="ai-records-section-heading">
        <strong>保存済み成果物</strong>
        <button
          class="ai-records-icon-button is-danger"
          type="button"
          title="保存済み成果物をすべて削除"
          aria-label="保存済み成果物をすべて削除"
          @click="removeAllArtifacts"
        >
          <Trash2Icon :size="15" aria-hidden="true" />
        </button>
      </div>
      <ul>
        <li v-for="artifact in writingStore.artifacts" :key="artifact.id">
          <button class="ai-records-item-button" type="button" @click="emit('open-artifact', artifact.id)">
            {{ artifact.title || '無題の成果物' }}
          </button>
          <span :class="`is-${artifact.status}`">{{ statusLabel(artifact.status) }}</span>
          <button
            class="ai-records-icon-button is-danger"
            type="button"
            title="成果物を削除"
            aria-label="成果物を削除"
            @click="removeArtifact(artifact.id)"
          >
            <Trash2Icon :size="15" aria-hidden="true" />
          </button>
        </li>
      </ul>
    </section>

    <p v-if="assistantStore.histories.length === 0 && writingStore.artifacts.length === 0 && aiStore.summaryHistory.length === 0" class="ai-records-empty">
      保存済みの履歴・成果物はありません。
    </p>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { Trash2Icon } from '@lucide/vue'
import type { AIRecordStatus } from '../api/ai'
import { useAIStore } from '../stores/useAIStore'
import { useAIAssistantStore } from '../stores/useAIAssistantStore'
import { useAIWritingStore } from '../stores/useAIWritingStore'

const emit = defineEmits<{
  (event: 'open-history', id: string): void
  (event: 'open-artifact', id: string): void
  (event: 'open-summary', id: string): void
}>()

const aiStore = useAIStore()
const assistantStore = useAIAssistantStore()
const writingStore = useAIWritingStore()

onMounted(() => {
  void assistantStore.refreshHistories()
  void writingStore.refreshArtifacts()
  void aiStore.refreshSummaryHistory()
})

async function removeSummary(id: string) {
  const summary = aiStore.summaryHistory.find((item) => item.id === id)
  if (!summary || !window.confirm(`要約「${summary.title || '無題の要約'}」を削除しますか？`)) return
  await aiStore.removeSummaryHistory(id)
}

async function removeHistory(id: string) {
  const history = assistantStore.histories.find((item) => item.id === id)
  if (!history || !window.confirm(`履歴「${history.title || '無題の履歴'}」を削除しますか？`)) return
  await assistantStore.removeHistory(id)
}

async function removeAllHistories() {
  if (!window.confirm('保存済みのAI履歴をすべて削除しますか？')) return
  await assistantStore.removeAllHistories()
}

async function removeArtifact(id: string) {
  const artifact = writingStore.artifacts.find((item) => item.id === id)
  if (!artifact || !window.confirm(`成果物「${artifact.title || '無題の成果物'}」を削除しますか？`)) return
  await writingStore.removeArtifact(id)
}

async function removeAllArtifacts() {
  if (!window.confirm('保存済みのAI成果物をすべて削除しますか？')) return
  await writingStore.removeAllArtifacts()
}

function statusLabel(status: AIRecordStatus) {
  return status === 'saved' ? '保存済み' : status === 'stale' ? '古い参照' : '元ノートなし'
}
</script>

<style scoped>
.ai-records-panel {
  display: grid;
  gap: 12px;
  padding: 14px;
}

.ai-records-heading,
.ai-records-section-heading,
.ai-records-section li {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-records-heading,
.ai-records-section-heading {
  justify-content: space-between;
}

.ai-records-description,
.ai-records-empty {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.ai-records-error {
  margin: 0;
  color: var(--color-danger, #b42318);
}

.ai-records-section {
  display: grid;
  gap: 6px;
}

.ai-records-section ul {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-records-item-button,
.ai-records-icon-button {
  border: 1px solid var(--border-color, var(--border));
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.ai-records-item-button {
  padding: 5px 9px;
}

.ai-records-icon-button {
  display: inline-grid;
  box-sizing: border-box;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
}

.ai-records-icon-button.is-danger {
  color: var(--color-danger, #b42318);
}

.ai-records-section li {
  justify-content: space-between;
}

.ai-records-item-button {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-records-section span {
  color: var(--text-secondary);
  font-size: 12px;
}

.ai-records-section span.is-stale {
  color: var(--color-warning, #8a5a00);
}

.ai-records-section span.is-orphaned {
  color: var(--color-danger, #b42318);
}
</style>
