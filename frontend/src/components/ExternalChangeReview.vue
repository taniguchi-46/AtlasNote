<template>
  <section class="external-review" aria-label="外部変更の確認">
    <header class="external-review-heading">
      <div><strong>変更確認</strong><p>外部CLI・MCPからの変更要求は、ここで承認するまで保存されません。</p></div>
      <button type="button" :disabled="loading" @click="refresh()">更新</button>
    </header>
    <p v-if="error" role="alert" class="external-review-error">{{ error }}</p>
    <p v-if="!reviews.length && !loading" class="external-review-empty">確認する変更はありません。</p>
    <div class="external-review-list">
      <article v-for="review in reviews" :key="review.operationId" class="external-review-card">
        <header><strong>{{ kindLabel(review.kind) }}</strong><span>{{ stateLabel(review.state) }}</span></header>
        <p>対象: {{ review.items.map((item) => item.noteTitle || item.noteId || '新しいノート').join('、') }}</p>
        <p>影響件数: {{ review.impactCount }}　期限: {{ formatTime(review.expiresAt) }}</p>
        <div v-for="(item, index) in review.items" :key="item.candidateId || index" class="external-review-item">
          <p>項目の状態: {{ stateLabel(item.status || review.state) }}</p>
          <p v-if="item.reason"><strong>理由:</strong> {{ item.reason }}</p>
          <div class="external-review-diff">
            <section><strong>変更前</strong><pre>{{ formatValue(item.before) }}</pre></section>
            <section><strong>変更後</strong><pre>{{ formatValue(item.after) }}</pre></section>
          </div>
        </div>
        <p v-if="review.message" role="status">{{ review.message }}</p>
        <div v-if="review.state === 'pending_approval'" class="external-review-actions">
          <button type="button" :disabled="busyId !== ''" @click="reject(review)">却下</button>
          <button type="button" class="external-review-approve" :disabled="busyId !== ''" @click="approve(review)">{{ busyId === review.operationId ? '確認中…' : '承認して適用' }}</button>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { approveExternalChange, listExternalChangeReviews, rejectExternalChange, type ChangeReview } from '../api/externalChanges'
import { useNoteStore } from '../stores/useNoteStore'
import { useTagStore } from '../stores/useTagStore'

const noteStore = useNoteStore()
const tagStore = useTagStore()
const reviews = ref<ChangeReview[]>([])
const loading = ref(false)
const busyId = ref('')
const error = ref('')
let timer: ReturnType<typeof setInterval> | undefined

async function refresh(preserveError = false) {
  if (loading.value || busyId.value) return
  loading.value = true
  try {
    reviews.value = await listExternalChangeReviews() ?? []
    if (!preserveError) error.value = ''
  } catch {
    error.value = '変更要求を取得できませんでした。'
  } finally {
    loading.value = false
  }
}

async function approve(review: ChangeReview) {
  if (busyId.value || review.state !== 'pending_approval') return
  busyId.value = review.operationId
  error.value = ''
  try {
    const noteIds = review.kind === 'notes.request_create' ? [] : review.targetIds
    const candidateNoteIds = Object.fromEntries(review.items.filter((item) => item.candidateId && item.noteId).map((item) => [item.candidateId!, item.noteId!]))
    const result = await noteStore.runExternalChangeOperation(noteIds, () => approveExternalChange(review.operationId), candidateNoteIds)
    if (!result) {
      error.value = '未保存の下書きを保存できませんでした。下書きと変更要求は保持されています。'
    } else if (result.state === 'applied' || result.items?.some((item) => item.status === 'applied')) {
      if (review.kind === 'notes.request_create') await noteStore.fetchNotes()
      if (review.kind === 'notes.request_tags' || review.kind === 'organize.request_apply') {
        for (const noteId of noteIds) await tagStore.refreshNoteTagsIfActive(noteId)
      }
    }
  } catch {
    error.value = '変更を適用できませんでした。要求を再確認してください。'
  } finally {
    busyId.value = ''
    await refresh(true)
  }
}

async function reject(review: ChangeReview) {
  if (busyId.value || review.state !== 'pending_approval') return
  busyId.value = review.operationId
  try { await rejectExternalChange(review.operationId) }
  catch { error.value = '変更要求を却下できませんでした。' }
  finally { busyId.value = ''; await refresh(true) }
}

function kindLabel(kind: string): string {
  return ({
    'organize.request_apply': '整理候補の適用', 'notes.propose_edit': '本文編集案',
    'notes.request_create': 'ノート作成', 'notes.request_update': 'ノート更新',
    'notes.request_move': 'Notebook移動', 'notes.request_tags': 'タグ変更',
    'notes.request_trash': 'ゴミ箱へ移動',
  } as Record<string, string>)[kind] ?? kind
}
function stateLabel(state: string): string {
  return ({ pending_approval: '承認待ち', applying: '適用中', applied: '適用済み', conflict: '競合', rejected: '却下', expired: '期限切れ', 'save-failure': '保存失敗', 'not-executed': '未適用', stale: '期限切れ', 'not-applicable': '適用対象外' } as Record<string, string>)[state] ?? state
}
function formatValue(value: unknown): string {
  if (value === undefined || value === null) return 'なし'
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}
function formatTime(value: string): string { return new Date(value).toLocaleString('ja-JP') }

onMounted(() => { void refresh(); timer = setInterval(() => { void refresh(true) }, 3000) })
onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.external-review{display:flex;flex:1;min-height:0;flex-direction:column;padding:12px;gap:10px;overflow:auto;font-size:12px}
.external-review-heading,.external-review-card>header,.external-review-actions{display:flex;align-items:center;justify-content:space-between;gap:8px}
.external-review-heading p{margin:4px 0;color:var(--text-secondary)}
.external-review-heading button,.external-review-actions button{padding:6px 10px;border:1px solid var(--border);border-radius:5px;background:var(--bg-input);color:var(--text-primary);cursor:pointer}
.external-review-heading button:focus-visible,.external-review-actions button:focus-visible{outline:2px solid var(--brand-primary)}
.external-review-list{display:flex;flex-direction:column;gap:10px}
.external-review-card{padding:10px;border:1px solid var(--border);border-radius:7px;background:var(--bg-sidebar)}
.external-review-card p{margin:7px 0}
.external-review-item{margin-top:8px;border-top:1px solid var(--border)}
.external-review-diff{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px}
.external-review-diff section{min-width:0;padding:7px;background:var(--bg-input);border-radius:5px}
.external-review-diff pre{max-height:220px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}
.external-review-approve{background:var(--brand-primary)!important;color:white!important}
.external-review-error{color:var(--color-danger,#c33)}
@media(max-width:700px){.external-review-diff{grid-template-columns:1fr}}
</style>
