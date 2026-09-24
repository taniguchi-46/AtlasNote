<template>
  <section class="organization-mini" :aria-label="source === 'ai' ? 'AIワークスペースの整理候補' : '関連ノートの整理候補'">
    <header class="organization-mini-header">
      <div>
        <strong>このノートの整理候補</strong>
        <span v-if="session?.analysis?.noteId === noteId">{{ candidates.length }}件</span>
      </div>
      <button class="organization-mini-close" type="button" title="候補を保持して閉じる" aria-label="整理候補パネルを閉じる" @click="store.closeMini">
        <XIcon :size="15" aria-hidden="true" />
      </button>
    </header>

    <p v-if="store.isSessionAnalyzing(sessionKey)" class="organization-mini-status" role="status">関連ノートを読み取り解析中…</p>
    <p v-else-if="!session?.analysis" class="organization-mini-status">
      対象ノートの候補を読み込めませんでした。整理センターから再解析してください。
    </p>
    <p v-if="session?.error" class="organization-mini-error" role="alert">{{ session.error }}</p>

    <div v-if="session?.analysis?.noteId === noteId" class="organization-mini-list">
      <article v-for="candidate in candidates" :key="candidate.id" class="organization-mini-candidate">
        <label v-if="candidate.applicable" class="organization-mini-selection">
          <input
            type="checkbox"
            :checked="session.selectedCandidateIds.includes(candidate.id)"
            :disabled="!store.canApply(candidate.id, sessionKey) || store.isApplying"
            @change="store.toggleCandidate(candidate.id, sessionKey)"
          >
          <strong>{{ kindLabel(candidate.kind) }}</strong>
        </label>
        <strong v-else>{{ kindLabel(candidate.kind) }}</strong>
        <dl class="organization-mini-diff">
          <div><dt>対象</dt><dd>{{ candidate.noteTitle || candidate.noteId }}</dd></div>
          <div v-if="candidate.relatedId"><dt>{{ relationLabel(candidate.kind) }}</dt><dd>{{ candidate.relatedTitle || candidate.relatedId }}</dd></div>
          <div><dt>変更前</dt><dd>{{ beforeText(candidate) }}</dd></div>
          <div><dt>変更案</dt><dd>{{ proposedText(candidate) }}</dd></div>
        </dl>
        <p class="organization-mini-reason">{{ candidate.reason }}</p>
        <p v-if="session.outcomes[candidate.id]" class="organization-mini-outcome">
          {{ statusLabel(session.outcomes[candidate.id].status) }}<template v-if="session.outcomes[candidate.id].message"> · {{ session.outcomes[candidate.id].message }}</template>
        </p>
        <button
          v-if="candidate.applicable && store.canApply(candidate.id, sessionKey)"
          type="button"
          :disabled="store.isApplying"
          @click="store.applyCandidate(candidate.id, sessionKey)"
        >{{ session.outcomes[candidate.id] ? '再試行' : '承認して適用' }}</button>
        <small v-if="['conflict', 'stale'].includes(session.outcomes[candidate.id]?.status ?? '')">
          前提が変わりました。タスク支援から再解析してください。
        </small>
      </article>
      <p v-if="candidates.length === 0 && !store.isSessionAnalyzing(sessionKey)" class="organization-mini-status">確認が必要な候補はありません。</p>
    </div>

    <footer v-if="session?.analysis?.noteId === noteId && session.selectedCandidateIds.length" class="organization-mini-footer">
      <span>{{ session.selectedCandidateIds.length }}件選択</span>
      <button type="button" :disabled="store.isApplying" @click="store.applySelected(sessionKey)">
        {{ store.isApplying ? '適用中…' : '選択を承認して適用' }}
      </button>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { XIcon } from '@lucide/vue'
import type { OrganizationCandidate } from '../api/organization'
import { useOrganizationStore, type OrganizationMiniSource } from '../stores/useOrganizationStore'

const props = defineProps<{ noteId: string; source: OrganizationMiniSource }>()
const store = useOrganizationStore()
const sessionKey = computed(() => store.noteSessionKey(props.noteId))
const session = computed(() => store.getSessionForNote(props.noteId))
const candidates = computed(() => session.value?.analysis?.candidates ?? [])

function kindLabel(kind: string) {
  const labels: Record<string, string> = {
    title: 'タイトル',
    'tag-assignment': 'タグ付与',
    'notebook-assignment': 'Notebook分類',
    'notebook-move': 'Notebook移動',
    'unclassified-note': '未分類ノート',
    'duplicate-note': '重複ノート',
    'empty-note': '空ノート',
    'broken-link': 'リンク切れ',
    'orphan-note': '孤立ノート',
    'related-note': '関連候補',
    'reciprocal-link': '相互リンク',
    'duplicate-tag': '重複タグ',
  }
  return labels[kind] ?? kind
}

function relationLabel(kind: string) {
  return kind === 'duplicate-note' ? '残すノート' : '関連先'
}

function beforeText(candidate: OrganizationCandidate) {
  switch (candidate.kind) {
    case 'title': return String(candidate.before.title ?? '')
    case 'notebook-assignment': return '未分類'
    case 'notebook-move': return String(candidate.before.notebookName ?? candidate.before.notebookId ?? '現在のNotebook')
    case 'unclassified-note': return 'Notebook未分類'
    case 'tag-assignment': return 'このタグは未付与'
    case 'duplicate-note': return `本文 ${String(candidate.before.characterCount ?? 0)}文字`
    case 'empty-note': return '空または空白だけの本文'
    case 'reciprocal-link': return '逆方向リンクなし'
    default: return '現在の状態'
  }
}

function proposedText(candidate: OrganizationCandidate) {
  switch (candidate.kind) {
    case 'title': return String(candidate.proposed?.title ?? '')
    case 'notebook-assignment':
    case 'notebook-move': return String(candidate.proposed?.notebookName ?? 'Notebook')
    case 'unclassified-note': return '検出のみ · 分類先候補はありません'
    case 'tag-assignment': return `タグ「${String(candidate.proposed?.tagName ?? '')}」を付与`
    case 'duplicate-note': return 'このノートをゴミ箱へ移動'
    case 'empty-note': return 'このノートをゴミ箱へ移動'
    case 'reciprocal-link': return `「${candidate.relatedTitle || candidate.relatedId || '関連ノート'}」へのMarkdownリンクを追記`
    default: return '検出のみ'
  }
}

function statusLabel(status: string) {
  if (status === 'applied') return '適用済み'
  if (status === 'applied-with-draft-conflict') return '適用済み・下書き競合'
  if (status === 'conflict' || status === 'stale') return '再解析が必要'
  if (status === 'save-failure' || status === 'not-executed') return '再試行できます'
  return '適用不可'
}
</script>

<style scoped>
.organization-mini{display:flex;max-height:320px;min-height:0;flex-direction:column;overflow:hidden;border:1px solid var(--border);border-radius:7px;background:var(--bg-editor);color:var(--text-primary)}
.organization-mini-header,.organization-mini-footer{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:7px 9px;border-bottom:1px solid var(--border);font-size:11px}
.organization-mini-header>div{display:flex;align-items:center;gap:7px;min-width:0}.organization-mini-header span,.organization-mini-status,.organization-mini-footer span{color:var(--text-secondary);font-size:10px}
.organization-mini-header button,.organization-mini-candidate button,.organization-mini-footer button{padding:4px 7px;border:1px solid var(--border);border-radius:4px;background:var(--bg-input);color:var(--text-primary);font-size:10px;cursor:pointer}.organization-mini-header .organization-mini-close{display:inline-grid;width:28px;height:28px;padding:0;place-items:center;flex:0 0 auto}.organization-mini-close:focus-visible{outline:2px solid var(--brand-primary);outline-offset:1px}
.organization-mini-list{min-height:0;overflow:auto;padding:6px}.organization-mini-candidate{display:grid;gap:5px;padding:8px 5px;border-bottom:1px solid var(--border)}.organization-mini-selection{display:flex;align-items:center;gap:6px;font-size:10px}.organization-mini-diff{display:grid;gap:4px;margin:0;font-size:10px}.organization-mini-diff>div{display:grid;grid-template-columns:54px minmax(0,1fr);gap:6px}.organization-mini-diff dt{color:var(--text-secondary)}.organization-mini-diff dd{min-width:0;margin:0;overflow-wrap:anywhere}.organization-mini-reason{margin:0;color:var(--text-secondary);font-size:10px;line-height:1.4}.organization-mini-candidate small,.organization-mini-error,.organization-mini-outcome{color:var(--color-warning);font-size:10px}.organization-mini-footer{border-top:1px solid var(--border);border-bottom:0}.organization-mini-footer button:disabled,.organization-mini-candidate button:disabled{opacity:.55;cursor:not-allowed}
</style>
