<template>
  <PopoverRoot v-model:open="isOpen">
    <PopoverTrigger as-child>
      <button
        class="backlink-trigger"
        type="button"
        title="このノートへのリンク"
        aria-label="このノートへのバックリンク"
      >
        <Link2Icon :size="15" />
        <span>バックリンク</span>
        <span v-if="linkStore.backlinkTotal > 0" class="backlink-count">
          {{ linkStore.backlinkTotal }}
        </span>
      </button>
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="backlink-popover"
        side="top"
        align="start"
        :side-offset="8"
      >
        <div class="backlink-title">このノートへのリンク</div>
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
            @click="openNote(item.id)"
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
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Link2Icon } from '@lucide/vue'
import {
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'
import { useNoteLinkStore } from '../stores/useNoteLinkStore'
import { useNoteStore } from '../stores/useNoteStore'

const props = defineProps<{
  noteId: string
}>()

const linkStore = useNoteLinkStore()
const noteStore = useNoteStore()
const isOpen = ref(false)

watch(() => props.noteId, (noteId) => {
  void linkStore.loadBacklinks(noteId)
}, { immediate: true })

async function openNote(noteId: string) {
  await noteStore.selectNote(noteId)
  isOpen.value = false
}

function formatDate(value: string | Date) {
  return new Date(value).toLocaleDateString('ja-JP')
}
</script>

<style scoped>
.backlink-trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 25px;
  padding: 0 6px;
  color: var(--text-secondary);
  font: inherit;
  font-size: 11px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px;
  cursor: pointer;
}

.backlink-trigger:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
  border-color: var(--border);
}

.backlink-count {
  min-width: 16px;
  padding: 1px 4px;
  color: var(--text-primary);
  text-align: center;
  background: var(--bg-hover);
  border-radius: 999px;
}

.backlink-popover {
  z-index: 1200;
  width: 260px;
  padding: 12px;
  background: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
}

.backlink-title {
  margin-bottom: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
}

.backlink-status {
  margin: 0;
  color: var(--text-secondary);
  font-size: 11px;
}

.backlink-list {
  display: flex;
  flex-direction: column;
  max-height: 240px;
  overflow-y: auto;
}

.backlink-item {
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

.backlink-item:hover {
  background: var(--bg-hover);
}

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
  color: var(--text-secondary);
  font: inherit;
  font-size: 11px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
}

.backlink-more:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.backlink-more:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
