<template>
  <PopoverRoot v-model:open="isOpen">
    <PopoverTrigger as-child>
      <button
        class="format-btn note-link-trigger"
        type="button"
        title="ノートリンクを挿入"
        aria-label="ノートリンクを挿入"
        :disabled="disabled"
      >
        <LinkIcon :size="15" />
      </button>
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="note-link-popover"
        side="bottom"
        align="start"
        :side-offset="8"
      >
        <div class="note-link-title">リンク先のノート</div>
        <input
          ref="searchInput"
          v-model="searchText"
          class="note-link-search"
          type="search"
          placeholder="タイトルで検索"
          aria-label="リンク先ノートを検索"
          @input="searchTargets"
        />

        <p v-if="linkStore.isSearchingTargets" class="note-link-status">検索中…</p>
        <p v-else-if="linkStore.targetError" class="note-link-status">{{ linkStore.targetError }}</p>
        <p v-else-if="!searchText.trim()" class="note-link-status">
          タイトルを入力してください。
        </p>
        <p v-else-if="linkStore.targetItems.length === 0" class="note-link-status">
          リンク先のノートが見つかりません。
        </p>
        <div v-else class="note-link-results" role="listbox" aria-label="リンク先ノート">
          <button
            v-for="item in linkStore.targetItems"
            :key="item.note.id"
            class="note-link-result"
            type="button"
            role="option"
            @click="selectTarget(item.note)"
          >
            <span class="note-link-result-title">{{ item.note.title }}</span>
            <span class="note-link-result-date">{{ formatDate(item.note.updatedAt) }}</span>
          </button>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import type { note } from '../../wailsjs/go/models'
import { LinkIcon } from '@lucide/vue'
import {
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'
import { useNoteLinkStore } from '../stores/useNoteLinkStore'

const props = defineProps<{
  noteId: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  select: [target: { id: string; title: string }]
  opened: []
}>()

const linkStore = useNoteLinkStore()
const isOpen = ref(false)
const searchText = ref('')
const searchInput = ref<HTMLInputElement | null>(null)

watch(isOpen, (open) => {
  if (open) {
    searchText.value = ''
    linkStore.clearTargetSearch()
    emit('opened')
    void nextTick(() => searchInput.value?.focus())
    return
  }
  linkStore.clearTargetSearch()
})

watch(() => props.noteId, () => {
  isOpen.value = false
  searchText.value = ''
  linkStore.clearTargetSearch()
})

function searchTargets() {
  void linkStore.searchTargets(searchText.value)
}

function selectTarget(target: note.Summary) {
  emit('select', { id: target.id, title: target.title })
  isOpen.value = false
}

function formatDate(value: string | Date) {
  return new Date(value).toLocaleDateString('ja-JP')
}
</script>

<style scoped>
.note-link-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px;
  cursor: pointer;
}

.note-link-trigger:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--bg-hover);
  border-color: var(--border);
}

.note-link-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.note-link-popover {
  z-index: 1200;
  width: 280px;
  padding: 12px;
  background: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
}

.note-link-title {
  margin-bottom: 9px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
}

.note-link-search {
  width: 100%;
  height: 31px;
  padding: 4px 8px;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
}

.note-link-search:focus {
  outline: 2px solid color-mix(in srgb, var(--color-primary) 35%, transparent);
  outline-offset: -1px;
}

.note-link-status {
  margin: 9px 0 0;
  color: var(--text-secondary);
  font-size: 11px;
}

.note-link-results {
  display: flex;
  flex-direction: column;
  max-height: 240px;
  margin-top: 8px;
  overflow-y: auto;
}

.note-link-result {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 7px 8px;
  color: var(--text-primary);
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 4px;
  cursor: pointer;
}

.note-link-result:hover {
  background: var(--bg-hover);
}

.note-link-result-title {
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.note-link-result-date {
  color: var(--text-secondary);
  font-size: 10px;
}
</style>
