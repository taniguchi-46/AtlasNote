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
  </div>
</template>

<script setup lang="ts">
import { useOrganizationStore } from '../stores/useOrganizationStore'
import { useNoteLinkStore } from '../stores/useNoteLinkStore'

const props = defineProps<{ noteId: string }>()
const emit = defineEmits<{ 'open-note': [noteId: string] }>()
const linkStore = useNoteLinkStore()
const organizationStore = useOrganizationStore()

function formatDate(value: string | Date) {
  return new Date(value).toLocaleDateString('ja-JP')
}
</script>

<style scoped>
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
