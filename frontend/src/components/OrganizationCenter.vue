<template>
  <section class="organization-content" aria-label="整理センター">
    <nav class="organization-tabs" aria-label="整理センター">
      <button type="button" :class="{ active: store.activeView === 'tasks' }" @click="store.activeView = 'tasks'">タスク支援</button>
      <button type="button" :class="{ active: store.activeView === 'organize' }" @click="store.activeView = 'organize'">ノート整理</button>
    </nav>

    <div class="organization-body">
      <template v-if="store.activeView === 'tasks'">
        <div class="organization-overview">
          <h3>確認できる整理タスク</h3>
          <p v-if="!store.analysis">保存空間を解析すると、タイトル・タグ・分類・重複・空ノート・リンクの候補をここに集約します。</p>
          <p v-else>解析対象 {{ store.analysis.analyzedNotes }}件 · 未確認候補 {{ store.pendingCount }}件</p>
          <div v-if="store.analysis" class="organization-counts">
            <button v-for="item in kindCounts" :key="item.kind" type="button" @click="showKind(item.kind)">
              <strong>{{ item.count }}</strong><span>{{ item.label }}</span>
            </button>
          </div>
          <button class="organization-primary" type="button" @click="store.activeView = 'organize'">
            候補を確認して承認
          </button>
        </div>
        <p class="organization-policy">保護対象 {{ store.analysis?.skippedLocked ?? 0 }}件、ゴミ箱 {{ store.analysis?.skippedTrash ?? 0 }}件は本文解析から除外します。</p>
      </template>

      <template v-else>
        <div v-if="store.centerSessionKey.startsWith('note:')" class="organization-note-scope">
          <span>対象: {{ targetNoteTitle }} と直接のリンク・被リンク先</span>
          <button type="button" @click="showCollectionScope">前の整理範囲へ</button>
          <button type="button" :disabled="store.isAnalyzing || store.isApplying" @click="runAnalysis">再解析</button>
        </div>
        <form v-else class="organization-controls" @submit.prevent="runAnalysis">
          <label>
            解析範囲
            <select :value="selectedNotebookId" :disabled="store.isAnalyzing || store.isApplying" @change="selectNotebook">
              <option value="">保存空間全体</option>
              <option v-for="notebook in notebookStore.notebooks" :key="notebook.id" :value="notebook.id">
                {{ notebook.name }}
              </option>
            </select>
          </label>
          <label v-if="selectedNotebookId">
            Notebook範囲
            <select :value="notebookScope" :disabled="store.isAnalyzing || store.isApplying" @change="selectNotebookScope">
              <option value="notebook">選択Notebook直下のみ</option>
              <option value="descendants">子孫Notebookを含む</option>
            </select>
          </label>
          <button class="organization-primary" type="submit" :disabled="store.isAnalyzing || store.isApplying">
            {{ store.isAnalyzing ? '解析中…' : '読み取り解析' }}
          </button>
        </form>
        <p class="organization-policy organization-scope-description">
          Notebook直下のみ・子孫を含む範囲・保存空間全体から選べます。フローティング表示とドック表示はこのWails画面内で切り替わります。
        </p>
        <p v-if="store.error" class="organization-error" role="alert">{{ store.error }}</p>

        <section v-if="store.analysis" class="organization-review" aria-label="候補レビュー">
          <div class="organization-review-heading">
            <div>
              <h3>候補のレビュー</h3>
              <p>{{ store.analysis.analyzedNotes }}件を解析 · {{ store.candidates.length }}件の候補</p>
            </div>
            <button v-if="selectedKind" type="button" class="organization-link-button" @click="selectedKind = ''">すべて表示</button>
            <button type="button" class="organization-link-button" @click="() => store.selectApplicable(undefined, visibleCandidateIds)">表示中の適用可能を選択</button>
          </div>
          <div class="organization-summary" role="status">
            保護対象を{{ store.analysis.skippedLocked }}件スキップ · ゴミ箱を{{ store.analysis.skippedTrash }}件スキップ
          </div>

          <div v-if="visibleCandidates.length === 0" class="organization-empty">
            <strong>表示する候補はありません</strong>
            <span>{{ selectedKind ? 'この種類に該当する候補はありません。すべて表示で他の候補を確認できます。' : '現在の解析範囲で確認が必要な項目は見つかりませんでした。' }}</span>
          </div>
          <article v-for="candidate in visibleCandidates" :key="candidate.id" class="organization-candidate">
            <div class="organization-candidate-select">
              <input
                v-if="candidate.applicable"
                type="checkbox"
                :checked="store.selectedCandidateIds.includes(candidate.id)"
                :disabled="!store.canApply(candidate.id) || store.isApplying"
                :aria-label="`${candidate.noteTitle || candidate.noteId}の${kindLabel(candidate.kind)}を選択`"
                @change="() => store.toggleCandidate(candidate.id)"
              >
              <span class="organization-kind">{{ kindLabel(candidate.kind) }}</span>
              <span v-if="store.outcomes[candidate.id]" class="organization-outcome" :class="`is-${store.outcomes[candidate.id].status}`">
                {{ statusLabel(store.outcomes[candidate.id].status) }}
              </span>
            </div>
            <h4>{{ candidate.noteTitle || 'タグ候補' }}</h4>
            <p class="organization-reason">{{ candidate.reason }}</p>
            <dl class="organization-diff">
              <div><dt>変更前</dt><dd>{{ beforeText(candidate) }}</dd></div>
              <div><dt>変更案</dt><dd>{{ proposedText(candidate) }}</dd></div>
            </dl>
            <p v-if="candidate.relatedId" class="organization-related">
              関連先: {{ candidate.relatedTitle || candidate.relatedId }}
            </p>
            <p v-if="store.outcomes[candidate.id]?.message" class="organization-result-message">
              {{ store.outcomes[candidate.id].message }}
            </p>
            <button
              v-if="candidate.applicable && store.canApply(candidate.id)"
              type="button"
              class="organization-approve-one"
              :disabled="store.isApplying"
              @click="store.applyCandidate(candidate.id)"
            >{{ store.outcomes[candidate.id] ? '再試行' : 'この候補だけ承認して適用' }}</button>
            <p v-if="['conflict', 'stale'].includes(store.outcomes[candidate.id]?.status ?? '')" class="organization-reanalysis-hint">
              前提が変わっています。再解析して候補を確認してください。
            </p>
          </article>
        </section>
        <div v-else class="organization-empty organization-empty-start">
          <strong>まだ解析していません</strong>
          <span>候補の生成だけではノート、タグ、リンク、保存状態を変更しません。</span>
        </div>
      </template>
    </div>

    <footer v-if="store.activeView === 'organize' && store.analysis" class="organization-footer">
      <span>表示中 {{ visibleSelectedCount }}件を選択中<span v-if="hiddenSelectedCount"> · 他の種類 {{ hiddenSelectedCount }}件を選択中</span></span>
      <button type="button" class="organization-primary" :disabled="!visibleSelectedCount || store.isApplying" @click="() => store.applySelected(undefined, visibleCandidateIds)">
        {{ store.isApplying ? '適用中…' : '選択した候補を承認して適用' }}
      </button>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useOrganizationStore } from '../stores/useOrganizationStore'
import type { OrganizationCandidate } from '../api/organization'

const store = useOrganizationStore()
const notebookStore = useNotebookStore()
const noteStore = useNoteStore()
const selectedNotebookId = computed(() => store.centerSessionKey.startsWith('notebook:')
  ? store.centerSessionKey.split(':')[1] ?? '' : '')
const notebookScope = computed<'notebook' | 'descendants'>(() => store.centerSessionKey.endsWith(':descendants') ? 'descendants' : 'notebook')
const selectedKind = ref('')
watch(() => store.centerSessionKey, () => { selectedKind.value = '' })
const targetNoteTitle = computed(() => {
  const noteId = store.centerSessionKey.startsWith('note:') ? store.centerSessionKey.slice(5) : ''
  if (!noteId) return ''
  if (noteStore.activeNote?.id === noteId) return noteStore.activeNote.title || '無題のノート'
  return noteStore.summaries.find((note) => note.id === noteId)?.title || noteId
})
const kindLabels: Record<string, string> = {
  'notebook-assignment': 'Notebook分類',
  'notebook-move': 'Notebook移動',
  'unclassified-note': '未分類ノート',
  'tag-assignment': 'タグ付与',
  'duplicate-note': '重複ノート',
  'empty-note': '空ノート',
  'broken-link': 'リンク切れ',
  'orphan-note': '孤立ノート',
  'related-note': '関連ノート',
  'reciprocal-link': '相互リンク候補',
  title: 'タイトル候補',
  'duplicate-tag': '重複タグ',
}
const kindCounts = computed(() => [...new Set(store.candidates.map((candidate) => candidate.kind))]
  .map((kind) => ({ kind, label: kindLabels[kind] ?? kind, count: store.candidates.filter((candidate) => candidate.kind === kind).length })))
const visibleCandidates = computed(() => selectedKind.value
  ? store.candidates.filter((candidate) => candidate.kind === selectedKind.value)
  : store.candidates)
const visibleCandidateIds = computed(() => visibleCandidates.value.map((candidate) => candidate.id))
const visibleSelectedCount = computed(() => visibleCandidates.value.filter((candidate) => store.selectedCandidateIds.includes(candidate.id)).length)
const hiddenSelectedCount = computed(() => store.selectedCandidateIds.length - visibleSelectedCount.value)
function runAnalysis() {
  selectedKind.value = ''
  if (store.centerSessionKey.startsWith('note:')) return store.analyze({ scope: 'note', noteId: store.centerSessionKey.slice(5) })
  const scope = selectedNotebookId.value ? notebookScope.value : 'space'
  return store.analyze({ scope, notebookId: selectedNotebookId.value || undefined })
}

function showCollectionScope() {
  store.showScope(store.lastCollectionScopeKey)
}

function selectNotebook(event: Event) {
  const notebookId = (event.target as HTMLSelectElement).value
  store.showScope(notebookId ? `notebook:${notebookId}:${notebookScope.value}` : 'space')
}

function selectNotebookScope(event: Event) {
  const scope = (event.target as HTMLSelectElement).value
  store.showScope(`notebook:${selectedNotebookId.value}:${scope}`)
}

function showKind(kind: string) {
  selectedKind.value = kind
  store.activeView = 'organize'
}

function kindLabel(kind: string) { return kindLabels[kind] ?? kind }

function beforeText(candidate: OrganizationCandidate) {
  switch (candidate.kind) {
    case 'title': return String(candidate.before.title ?? '')
    case 'notebook-assignment': return '未分類（保存空間の最上位）'
    case 'notebook-move': return String(candidate.before.notebookName ?? candidate.before.notebookId ?? '現在のNotebook')
    case 'unclassified-note': return 'Notebook未分類'
    case 'tag-assignment': return 'このタグは未付与'
    case 'duplicate-note': return `${candidate.noteTitle || 'ノート'} · 本文 ${String(candidate.before.characterCount ?? 0)}文字`
    case 'empty-note': return '空または空白だけの本文'
    case 'broken-link': return String(candidate.before.targetId ?? candidate.relatedId ?? '')
    case 'orphan-note': return 'ノート間リンクなし'
    case 'related-note': return candidate.noteTitle || candidate.noteId || ''
    case 'reciprocal-link': return '相手からのリンクのみ存在'
    case 'duplicate-tag': return String(candidate.before.name ?? '')
    default: return '現在の状態'
  }
}

function proposedText(candidate: OrganizationCandidate) {
  switch (candidate.kind) {
    case 'title': return String(candidate.proposed?.title ?? '')
    case 'notebook-assignment': return String(candidate.proposed?.notebookName ?? 'Notebook')
    case 'notebook-move': return `「${String(candidate.proposed?.notebookName ?? 'Notebook')}」へ移動`
    case 'unclassified-note': return '検出のみ · 分類先候補はありません'
    case 'tag-assignment': return `タグ「${String(candidate.proposed?.tagName ?? '')}」を付与`
    case 'duplicate-note': return `「${candidate.relatedTitle || candidate.relatedId || '別ノート'}」を残して、このノートをゴミ箱へ移動`
    case 'empty-note': return 'ゴミ箱へ移動（完全削除なし）'
    case 'broken-link': return '検出のみ · 本文中のリンクは自動変更しません'
    case 'orphan-note': return '検出のみ · 削除しません'
    case 'related-note': return candidate.relatedTitle || candidate.relatedId || '関連候補を確認'
    case 'reciprocal-link': return `「${candidate.relatedTitle || candidate.relatedId || '相手ノート'}」へのMarkdownリンクを追記`
    case 'duplicate-tag': return '検出のみ · 統合は未対応'
    default: return '検出のみ'
  }
}

function statusLabel(status: string) {
  switch (status) {
    case 'applied': return '適用済み'
    case 'applied-with-draft-conflict': return '適用済み · 下書き競合'
    case 'conflict': return '競合'
    case 'save-failure': return '保存失敗'
    case 'stale': return '古い候補'
    case 'not-executed': return '未実行'
    default: return '適用不可'
  }
}

</script>

<style scoped>
.organization-content{display:flex;min-width:0;min-height:0;flex:1;flex-direction:column;overflow:hidden;background:var(--bg-editor);color:var(--text-primary);container-type:inline-size}
.organization-note-scope{display:flex;flex-wrap:wrap;align-items:center;gap:7px;margin-bottom:8px;font-size:11px}
.organization-note-scope button{padding:5px;border:1px solid var(--border);border-radius:5px;background:var(--bg-input);color:var(--text-primary);cursor:pointer}

.organization-tabs{display:flex;gap:4px;padding:8px 12px;border-bottom:1px solid var(--border)}.organization-tabs button{padding:6px 10px;border-radius:5px}.organization-tabs button.active{background:var(--bg-active);color:var(--brand-primary);font-weight:600}
.organization-body{min-height:0;flex:1;overflow:auto;padding:14px}.organization-overview{max-width:620px;margin:12px auto;padding:18px;border:1px solid var(--border);border-radius:8px}.organization-overview h3{margin:0 0 8px;font-size:14px}.organization-overview p{color:var(--text-secondary);font-size:12px;line-height:1.6}.organization-counts{display:grid;grid-template-columns:repeat(auto-fit,minmax(112px,1fr));gap:8px;margin:14px 0}.organization-counts button{display:flex;align-items:center;gap:8px;padding:9px;border:1px solid var(--border);border-radius:6px;background:var(--bg-input);color:var(--text-primary);text-align:left;cursor:pointer}.organization-counts strong{font-size:16px}.organization-counts span{font-size:11px}.organization-primary{min-height:32px;padding:0 12px;border:0;border-radius:5px;background:var(--brand-primary);color:#fff;font-size:11px;font-weight:600;cursor:pointer}.organization-primary:disabled{opacity:.5;cursor:not-allowed}
.organization-controls{display:flex;align-items:flex-end;justify-content:space-between;gap:12px}.organization-controls label{display:flex;min-width:0;flex:1;flex-direction:column;gap:5px;font-size:11px}.organization-controls select{height:32px;padding:0 8px;border:1px solid var(--border);border-radius:5px;background:var(--bg-input);color:var(--text-primary)}.organization-policy{line-height:1.6}.organization-error{color:var(--color-danger);font-size:12px}
.organization-scope-description{margin-top:18px}
.organization-approve-one{margin-top:8px;padding:6px 9px;border:1px solid var(--border);border-radius:5px;background:var(--bg-input);color:var(--text-primary);font-size:10px;cursor:pointer}.organization-approve-one:hover:not(:disabled){border-color:var(--brand-primary)}.organization-approve-one:disabled{opacity:.5;cursor:not-allowed}
.organization-reanalysis-hint{margin-top:5px;color:var(--color-warning);font-size:10px}
.organization-review-heading,.organization-candidate-select,.organization-footer{display:flex;align-items:center;justify-content:space-between;gap:8px}
.organization-review-heading h3,.organization-review-heading p{margin:0}.organization-review-heading p,.organization-summary{color:var(--text-secondary);font-size:11px}
.organization-candidate{margin:10px 0;padding:10px;border:1px solid var(--border);border-radius:6px;background:var(--bg-editor);font-size:11px;overflow-wrap:anywhere}
.organization-candidate h4{margin:8px 0 4px;font-size:12px}.organization-candidate p{margin:5px 0;line-height:1.5}
.organization-candidate-select{justify-content:flex-start}.organization-kind{color:var(--brand-primary);font-weight:600}.organization-outcome{margin-left:auto}
.organization-diff{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;margin:8px 0}.organization-diff>div{min-width:0;padding:7px;border:1px solid var(--border);border-radius:4px;background:var(--bg-input)}.organization-diff dt{color:var(--text-secondary)}.organization-diff dd{margin:4px 0 0;white-space:pre-wrap;overflow-wrap:anywhere}
.organization-footer{padding:8px 10px;border-top:1px solid var(--border);font-size:11px}.organization-empty{display:grid;gap:5px;padding:15px;color:var(--text-secondary);font-size:11px}.organization-empty strong{color:var(--text-primary)}

@container(max-width:400px){.organization-controls{flex-wrap:wrap}.organization-controls button{width:100%}.organization-diff{grid-template-columns:1fr}}
</style>
