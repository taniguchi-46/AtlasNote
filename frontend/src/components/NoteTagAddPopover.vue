<template>
  <PopoverRoot v-model:open="isOpen">
    <PopoverTrigger as-child>
      <button
        class="note-tag-add-trigger"
        type="button"
        title="タグを追加"
        aria-label="タグを追加"
        :disabled="disabled || tagStore.isMutating"
      >
        <TagIcon :size="16" />
      </button>
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="note-tag-add-popover"
        side="top"
        align="start"
        :side-offset="8"
      >
        <div class="note-tag-add-title">タグを追加</div>

        <label class="note-tag-add-label" :for="selectID">タグ</label>
        <select
          :id="selectID"
          ref="selectRef"
          v-model="selectedTagID"
          class="note-tag-add-select"
          :disabled="disabled || tagStore.isMutating || isLoadingCandidates"
        >
          <option value="">タグを選択してください</option>
          <option v-for="tag in availableTags" :key="tag.id" :value="tag.id">
            {{ tag.name }}
          </option>
        </select>

        <p v-if="tagStore.error && tagStore.activeNoteId === props.noteId" class="note-tag-add-status">
          {{ tagStore.error }}
        </p>
        <p v-else-if="isLoadingCandidates" class="note-tag-add-status">タグを読み込み中…</p>
        <p v-else-if="availableTags.length === 0" class="note-tag-add-status">
          追加できるタグはありません。
        </p>

        <div class="note-tag-add-actions">
          <button
            class="note-tag-add-cancel"
            type="button"
            :disabled="tagStore.isMutating"
            @click="closePopover"
          >
            キャンセル
          </button>
          <button
            class="note-tag-add-submit"
            type="button"
            :disabled="!selectedTagID || tagStore.isMutating || isLoadingCandidates"
            @click="addTag"
          >
            {{ tagStore.isMutating ? '追加中…' : '追加' }}
          </button>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { TagIcon } from '@lucide/vue'
import {
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'
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
const isOpen = ref(false)
const selectedTagID = ref('')
const selectRef = ref<HTMLSelectElement | null>(null)
const selectID = computed(() => `note-tag-select-${props.noteId}`)

const activeTags = computed(() => (
  tagStore.activeNoteId === props.noteId ? tagStore.activeNoteTags : []
))

const isLoadingCandidates = computed(() => (
  tagStore.activeNoteId !== props.noteId || tagStore.isLoading || !tagStore.activeNoteTagsReady
))

const availableTags = computed(() => {
  const attachedIDs = new Set(activeTags.value.map((tag) => tag.id))
  return tagStore.tags.filter((tag) => !attachedIDs.has(tag.id))
})

watch(isOpen, (open) => {
  if (!open) {
    selectedTagID.value = ''
    return
  }

  void nextTick(() => selectRef.value?.focus())
})

watch(() => props.noteId, () => {
  selectedTagID.value = ''
  isOpen.value = false
})

function closePopover() {
  isOpen.value = false
}

async function addTag() {
  if (!selectedTagID.value || props.disabled || tagStore.isMutating || isLoadingCandidates.value) return

  const tagID = selectedTagID.value
  try {
    await tagStore.attachTagToNote(props.noteId, tagID)
    if (noteStore.activeTagId === tagID) {
      await noteStore.fetchNotes([], tagID)
    }
    closePopover()
  } catch (_) {
    // The tag store reports the operation error through the notification center.
  }
}
</script>

<style scoped>
.note-tag-add-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 25px;
  height: 25px;
  padding: 0;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px;
  cursor: pointer;
}

.note-tag-add-trigger:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--bg-hover);
  border-color: var(--border);
}

.note-tag-add-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.note-tag-add-popover {
  z-index: 1200;
  width: 245px;
  padding: 12px;
  background: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
}

.note-tag-add-title {
  margin-bottom: 10px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
}

.note-tag-add-label {
  display: block;
  margin-bottom: 4px;
  color: var(--text-secondary);
  font-size: 12px;
}

.note-tag-add-select {
  width: 100%;
  height: 31px;
  padding: 4px 7px;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
}

.note-tag-add-select:focus {
  outline: 2px solid color-mix(in srgb, var(--color-primary) 35%, transparent);
  outline-offset: -1px;
}

.note-tag-add-select:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.note-tag-add-status {
  margin: 7px 0 0;
  color: var(--text-secondary);
  font-size: 11px;
}

.note-tag-add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 12px;
}

.note-tag-add-actions button {
  min-width: 76px;
  height: 29px;
  padding: 0 9px;
  font: inherit;
  font-size: 12px;
  border-radius: 4px;
  cursor: pointer;
}

.note-tag-add-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.note-tag-add-cancel {
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid var(--border);
}

.note-tag-add-cancel:hover:not(:disabled) {
  background: var(--bg-hover);
}

.note-tag-add-submit {
  color: #fff;
  background: var(--brand-primary);
  border: 1px solid var(--brand-primary);
}

.note-tag-add-submit:hover:not(:disabled) {
  background: var(--brand-hover);
}
</style>
