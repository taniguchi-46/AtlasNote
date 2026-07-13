<template>
  <div class="note-tags" aria-label="タグ">

    <div v-if="activeTags.length > 0" class="tag-chip-list">
      <span v-for="tag in activeTags" :key="tag.id" class="tag-chip">
        <span class="tag-chip-name">{{ tag.name }}</span>
        <button
          v-if="!disabled"
          class="tag-chip-remove"
          type="button"
          :disabled="tagStore.isMutating"
          :aria-label="`「${tag.name}」をノートから外す`"
          @click="detachTag(tag.id)"
        >
          <XIcon :size="13" />
        </button>
      </span>
    </div>

    <span v-else-if="tagStore.isLoading && tagStore.activeNoteId === noteId" class="note-tags-status">
      タグを読み込み中…
    </span>

    <span v-else-if="disabled" class="note-tags-readonly">ゴミ箱内のノートではタグを変更できません。</span>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { XIcon } from '@lucide/vue'
import { useTagStore } from '../stores/useTagStore'
import { useNoteStore } from '../stores/useNoteStore'

const props = withDefaults(defineProps<{
  noteId: string
  disabled?: boolean
}>(), {
  disabled: false,
})

const tagStore = useTagStore()
const noteStore = useNoteStore()

const activeTags = computed(() => (
  tagStore.activeNoteId === props.noteId ? tagStore.activeNoteTags : []
))

watch(() => props.noteId, (noteId) => {
  void tagStore.loadNoteTags(noteId)
}, { immediate: true })

async function detachTag(tagID: string) {
  try {
    await tagStore.detachTagFromNote(props.noteId, tagID)
    if (noteStore.activeTagId === tagID) {
      await noteStore.fetchNotes([], tagID)
    }
  } catch (_) {
    // The tag store reports the operation error through the notification center.
  }
}
</script>

<style scoped>
.note-tags {
  display: flex;
  align-items: center;
  flex: 0 1 auto;
  min-width: 0;
}

.note-tags-status,
.note-tags-readonly {
  color: var(--text-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.tag-chip-list {
  display: flex;
  flex: 0 1 auto;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  max-width: 180px;
  padding: 3px 5px 3px 7px;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 12px;
  white-space: nowrap;
  text-overflow: ellipsis;
  background: var(--bg-hover);
  border-radius: 999px;
}

.tag-chip-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag-chip-remove {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 17px;
  height: 17px;
  padding: 0;
  color: inherit;
  background: transparent;
  border: 0;
  border-radius: 50%;
  cursor: pointer;
}

.tag-chip-remove:hover:not(:disabled) {
  background: var(--border);
}

.tag-chip-remove:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
