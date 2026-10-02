<template>
  <section class="legacy-records" data-settings-anchor="ai" tabindex="-1" aria-label="外部AIと旧AIデータ">
    <h3>外部AI・旧AIデータ</h3>
    <p>AtlasNote内蔵AIの生成機能は廃止しました。AIは統合ターミナルからCLIとMCPを通して利用します。MCPの公開範囲は明示的に指定してください。ターミナルはsandboxではありません。</p>
    <p>旧AIの履歴と成果物は元の保存空間に保持され、ここから閲覧できます。</p>
    <button type="button" :disabled="loading" @click="refresh">{{ loading ? '読み込み中…' : '一覧を更新' }}</button>
    <p v-if="error" role="alert">{{ error }}</p>

    <section aria-label="旧AI履歴">
      <h4>履歴 ({{ histories.length }})</h4>
      <p v-if="!loading && histories.length === 0">保存済み履歴はありません。</p>
      <ul><li v-for="item in histories" :key="item.id">
        <button type="button" @click="openHistory(item.id)">{{ item.title || '無題の履歴' }}</button>
        <small>{{ item.kind }} · {{ item.status }} · {{ formatDate(item.createdAt) }}</small>
      </li></ul>
    </section>

    <section aria-label="旧AI成果物">
      <h4>成果物 ({{ writingArtifacts.length }})</h4>
      <p v-if="!loading && writingArtifacts.length === 0">保存済み成果物はありません。</p>
      <ul><li v-for="item in writingArtifacts" :key="item.id">
        <button type="button" @click="openArtifact(item.id)">{{ item.title || '無題の成果物' }}</button>
        <small>{{ item.kind }} · {{ item.status }} · {{ formatDate(item.createdAt) }}</small>
      </li></ul>
      <button v-if="writingHasNext" type="button" :disabled="loading || loadingMoreWriting" @click="loadMore('writing')">{{ loadingMoreWriting ? '読み込み中…' : '成果物をさらに表示' }}</button>
    </section>

    <section aria-label="保存済み要約">
      <h4>保存済み要約 ({{ summaries.length }})</h4>
      <p v-if="!loading && summaries.length === 0">保存済み要約はありません。</p>
      <ul><li v-for="item in summaries" :key="item.id">
        <button type="button" @click="openArtifact(item.id)">{{ item.title || '無題の要約' }}</button>
        <small>{{ item.status }} · {{ formatDate(item.createdAt) }}</small>
      </li></ul>
      <button v-if="summaryHasNext" type="button" :disabled="loading || loadingMoreSummary" @click="loadMore('summary')">{{ loadingMoreSummary ? '読み込み中…' : '要約をさらに表示' }}</button>
    </section>

    <article v-if="selectedHistory" class="legacy-detail" aria-label="履歴の内容">
      <h4>{{ selectedHistory.title }}</h4>
      <p>{{ selectedHistory.kind }} · {{ selectedHistory.status }} · {{ formatDate(selectedHistory.createdAt) }}</p>
      <div v-for="(message, index) in selectedHistory.messages ?? []" :key="index">
        <strong>{{ message.role }}</strong><pre>{{ message.content }}</pre>
      </div>
    </article>
    <article v-if="selectedArtifact" class="legacy-detail" aria-label="成果物の内容">
      <h4>{{ selectedArtifact.title }}</h4>
      <p>{{ selectedArtifact.kind }} · {{ selectedArtifact.status }} · {{ formatDate(selectedArtifact.createdAt) }}</p>
      <pre>{{ selectedArtifact.content }}</pre>
    </article>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import type { AIArtifact, AIHistory } from '../api/ai'
import { getLegacyArtifact, getLegacyHistory, listLegacyArtifactsPage, listLegacyHistories, type LegacyArtifactKind } from '../api/legacyAIRecords'
import { useContentLockStore } from '../stores/useContentLockStore'
import { useSettingsStore } from '../stores/useSettingsStore'

const settingsStore = useSettingsStore()
const contentLockStore = useContentLockStore()
const histories = ref<AIHistory[]>([])
const writingArtifacts = ref<AIArtifact[]>([])
const summaries = ref<AIArtifact[]>([])
const writingOffset = ref(0)
const summaryOffset = ref(0)
const writingHasNext = ref(false)
const summaryHasNext = ref(false)
const loadingMoreWriting = ref(false)
const loadingMoreSummary = ref(false)
const selectedHistory = ref<AIHistory | null>(null)
const selectedArtifact = ref<AIArtifact | null>(null)
const loading = ref(false)
const error = ref('')
let requestVersion = 0
let detailVersion = 0

function clearRecords() {
  requestVersion += 1
  detailVersion += 1
  histories.value = []
  writingArtifacts.value = []
  summaries.value = []
  writingOffset.value = 0
  summaryOffset.value = 0
  writingHasNext.value = false
  summaryHasNext.value = false
  loadingMoreWriting.value = false
  loadingMoreSummary.value = false
  selectedHistory.value = null
  selectedArtifact.value = null
  loading.value = false
  error.value = ''
}
watch(() => contentLockStore.locks, clearRecords, { deep: true })
watch(() => settingsStore.isSettingsOpen, open => { if (open) void refresh(); else clearRecords() })

async function refresh() {
  const version = ++requestVersion
  detailVersion += 1
  loading.value = true
  error.value = ''
  histories.value = []
  writingArtifacts.value = []
  summaries.value = []
  writingOffset.value = 0
  summaryOffset.value = 0
  writingHasNext.value = false
  summaryHasNext.value = false
  loadingMoreWriting.value = false
  loadingMoreSummary.value = false
  selectedHistory.value = null
  selectedArtifact.value = null
  try {
    const [loadedHistories, writingPage, summaryPage] = await Promise.all([
      listLegacyHistories(), listLegacyArtifactsPage('writing', 0), listLegacyArtifactsPage('summary', 0),
    ])
    if (version !== requestVersion) return
    histories.value = loadedHistories
    writingArtifacts.value = writingPage.items
    summaries.value = summaryPage.items
    writingOffset.value = writingPage.items.length
    summaryOffset.value = summaryPage.items.length
    writingHasNext.value = writingPage.hasNext
    summaryHasNext.value = summaryPage.hasNext
  } catch {
    if (version === requestVersion) error.value = '旧AIデータの一覧を読み込めませんでした。再試行してください。'
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function loadMore(kind: LegacyArtifactKind) {
  if (loading.value) return
  const isSummary = kind === 'summary'
  const isLoading = isSummary ? loadingMoreSummary : loadingMoreWriting
  const hasNext = isSummary ? summaryHasNext : writingHasNext
  const offset = isSummary ? summaryOffset : writingOffset
  const items = isSummary ? summaries : writingArtifacts
  if (isLoading.value || !hasNext.value) return
  const version = requestVersion
  isLoading.value = true
  error.value = ''
  try {
    const page = await listLegacyArtifactsPage(kind, offset.value)
    if (version !== requestVersion) return
    const seen = new Set(items.value.map(item => item.id))
    items.value = [...items.value, ...page.items.filter(item => !seen.has(item.id))]
    offset.value += page.items.length
    hasNext.value = page.hasNext
  } catch {
    if (version === requestVersion) error.value = '旧AIデータの追加分を読み込めませんでした。再試行してください。'
  } finally {
    if (version === requestVersion) isLoading.value = false
  }
}

async function openHistory(id: string) {
  const version = ++detailVersion
  const listVersion = requestVersion
  error.value = ''
  selectedArtifact.value = null
  try {
    const item = await getLegacyHistory(id)
    if (version === detailVersion && listVersion === requestVersion) selectedHistory.value = item
  } catch {
    if (version === detailVersion && listVersion === requestVersion) error.value = '履歴を読み込めませんでした。'
  }
}

async function openArtifact(id: string) {
  const version = ++detailVersion
  const listVersion = requestVersion
  error.value = ''
  selectedHistory.value = null
  try {
    const item = await getLegacyArtifact(id)
    if (version === detailVersion && listVersion === requestVersion) selectedArtifact.value = item
  } catch {
    if (version === detailVersion && listVersion === requestVersion) error.value = '成果物を読み込めませんでした。'
  }
}

function formatDate(value: string) { return new Date(value).toLocaleString('ja-JP') }
onMounted(() => { void refresh() })
</script>

<style scoped>
.legacy-records { display: grid; gap: 12px; font-size: 13px; }
.legacy-records h3, .legacy-records h4, .legacy-records p { margin: 0; }
.legacy-records section { display: grid; gap: 6px; }
.legacy-records ul { margin: 0; padding-left: 18px; }
.legacy-records li { margin: 5px 0; }
.legacy-records button { padding: 5px 8px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-input); color: var(--text-primary); cursor: pointer; }
.legacy-records button:disabled { opacity: 1; color: var(--text-secondary); cursor: wait; }
.legacy-records small { margin-left: 8px; color: var(--text-secondary); }
.legacy-detail { padding: 10px; border: 1px solid var(--border); border-radius: 5px; }
.legacy-detail pre { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
