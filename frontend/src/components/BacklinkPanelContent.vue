<template>
  <div class="backlink-panel-content">
    <button class="backlink-organization-trigger" type="button" @click="organizationStore.openNote(noteId)">
      関連ノートを整理
    </button>
    <p v-if="linkStore.isLoadingBacklinks && linkStore.backlinks.length === 0" class="backlink-status">
      読み込み中…
    </p>
    <p v-else-if="linkStore.backlinkError && linkStore.backlinks.length === 0" class="backlink-status">
      {{ linkStore.backlinkError }}
    </p>
    <p v-else-if="linkStore.backlinks.length === 0" class="backlink-status">
      バックリンクはありません。
    </p>
    <div v-else class="backlink-list" role="listbox" aria-label="バックリンク一覧">
      <button
        v-for="item in linkStore.backlinks"
        :key="item.id"
        class="backlink-item"
        type="button"
        role="option"
        @click="emit('open-note', item.id)"
      >
        <span class="backlink-item-title">{{ item.title }}</span>
        <span class="backlink-item-date">{{ formatDate(item.updatedAt) }}</span>
      </button>
    </div>
    <button
      v-if="linkStore.backlinkHasNext"
      class="backlink-more"
      type="button"
      :disabled="linkStore.isLoadingBacklinks"
      @click="linkStore.loadNextBacklinks()"
    >
      {{ linkStore.isLoadingBacklinks ? '読み込み中…' : 'さらに表示' }}
    </button>
    <section class="related-section" aria-label="関連候補">
      <strong class="related-heading">関連候補</strong>
      <button class="related-refresh" type="button" :disabled="linkStore.isLoadingRelated" @click="linkStore.loadRelated(noteId)">更新</button>
      <div class="related-scope">
        <label for="related-notebook-scope">範囲</label>
        <select id="related-notebook-scope" :value="linkStore.relatedNotebookId ?? ''" @change="changeNotebookScope">
          <option value="">保存空間全体</option>
          <option v-for="notebook in notebookStore.notebooks" :key="notebook.id" :value="notebook.id">{{ notebook.name }}</option>
        </select>
        <label><input type="checkbox" :checked="linkStore.relatedDescendants" :disabled="!linkStore.relatedNotebookId" @change="changeDescendants">子孫を含む</label>
      </div>
      <p v-if="linkStore.isLoadingRelated" class="backlink-status">読み込み中…</p>
      <p v-else-if="linkStore.relatedError" class="backlink-status">
        {{ linkStore.relatedError }}
        <button type="button" @click="linkStore.loadRelated(noteId)">再試行</button>
      </p>
      <p v-else-if="linkStore.relatedItems.length === 0" class="backlink-status">関連候補はありません。</p>
      <ul v-else class="related-list">
        <li v-for="item in linkStore.relatedItems" :key="item.noteId" class="related-item">
          <strong>{{ item.title }}</strong>
          <p v-if="item.snippet" class="related-snippet">{{ item.snippet }}</p>
          <small>{{ item.reasons.join('・') }}</small>
          <div class="related-actions">
            <button type="button" @click="emit('open-note', item.noteId)">ノートを開く</button>
            <button type="button" @click="addReference(item)">参照に追加</button>
          </div>
        </li>
      </ul>
      <p v-if="referenceMessage" role="status" class="backlink-status">{{ referenceMessage }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useOrganizationStore } from '../stores/useOrganizationStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useNoteLinkStore } from '../stores/useNoteLinkStore'
import type { note } from '../../wailsjs/go/models'
import { ref, watch } from 'vue'

const props = defineProps<{ noteId: string }>()
const emit = defineEmits<{ 'open-note': [noteId: string] }>()
const linkStore = useNoteLinkStore()
const organizationStore = useOrganizationStore()
const notebookStore = useNotebookStore()
const referenceMessage = ref('')
watch([() => props.noteId, () => linkStore.relatedNotebookId, () => linkStore.relatedDescendants], () => { referenceMessage.value = '' })

function changeNotebookScope(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  linkStore.setRelatedScope(value || null, value ? linkStore.relatedDescendants : false)
}

function changeDescendants(event: Event) {
  linkStore.setRelatedScope(linkStore.relatedNotebookId, (event.target as HTMLInputElement).checked)
}

async function addReference(item: note.RelatedNoteItem) {
  const noteId = props.noteId
  const notebookId = linkStore.relatedNotebookId
  const descendants = linkStore.relatedDescendants
  const added = await linkStore.addRelatedContext(noteId, item)
  if (noteId !== props.noteId || notebookId !== linkStore.relatedNotebookId || descendants !== linkStore.relatedDescendants) return
  referenceMessage.value = added
    ? 'AIの参照に追加しました。送信前に内容を確認してください。'
    : '参照に追加できませんでした。候補を再読み込みしてください。'
}

function formatDate(value: string | Date) {
  return new Date(value).toLocaleDateString('ja-JP')
}
</script>

<style scoped>
.related-section { margin-top: 12px; padding-top: 10px; border-top: 1px solid var(--border); }
.related-heading { font-size: 12px; }
.related-scope { display: flex; align-items: center; gap: 6px; margin: 8px 0; font-size: 11px; }
.related-scope select { min-width: 0; max-width: 150px; color: var(--text-primary); background: var(--bg-editor); border: 1px solid var(--border); border-radius: 4px; }
.related-refresh { float: right; padding: 2px 5px; border: 1px solid var(--border); border-radius: 4px; font-size: 11px; }
.related-list { max-height: 280px; margin: 8px 0 0; padding: 0; overflow-y: auto; list-style: none; }
.related-item { padding: 8px 0; border-top: 1px solid var(--border); font-size: 12px; }
.related-item:first-child { border-top: 0; }
.related-item strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.related-item small { display: block; color: var(--text-secondary); }
.related-snippet { margin: 4px 0; overflow: hidden; color: var(--text-secondary); text-overflow: ellipsis; white-space: nowrap; }
.related-actions { display: flex; gap: 8px; margin-top: 5px; }
.related-actions button { padding: 3px 6px; border: 1px solid var(--border); border-radius: 4px; font: inherit; }
.backlink-organization-trigger {
  width: 100%;
  margin: 0 0 8px;
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 11px;
  text-align: left;
}
.backlink-organization-trigger:hover { background: var(--bg-hover); }

.backlink-status {
  margin: 0;
  color: var(--text-secondary);
  font-size: 11px;
}

.backlink-list {
  display: flex;
  max-height: 240px;
  flex-direction: column;
  overflow-y: auto;
}

.backlink-item {
  display: flex;
  width: 100%;
  flex-direction: column;
  gap: 2px;
  padding: 7px 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
  text-align: left;
  cursor: pointer;
}

.backlink-item:hover { background: var(--bg-hover); }

.backlink-item-title {
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.backlink-item-date {
  color: var(--text-secondary);
  font-size: 10px;
}

.backlink-more {
  width: 100%;
  margin-top: 8px;
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.backlink-more:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.backlink-more:disabled {
  cursor: not-allowed;
  opacity: .55;
}
</style>
