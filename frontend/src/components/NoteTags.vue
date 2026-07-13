<template>
  <section class="note-tags" aria-label="タグ">
    <span class="note-tags-label">タグ</span>

    <div v-if="activeTags.length > 0" class="tag-chip-list">
      <span v-for="tag in activeTags" :key="tag.id" class="tag-chip">
        <TagIcon :size="13" aria-hidden="true" />
        <span>{{ tag.name }}</span>
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

    <div v-if="!disabled" class="tag-picker">
      <div class="tag-picker-input-row">
        <input
          v-model="query"
          class="tag-picker-input"
          type="search"
          placeholder="タグを検索または作成"
          aria-label="タグを検索または作成"
          :disabled="tagStore.isMutating"
          @keydown.enter.prevent="addTagFromQuery"
          @keydown.esc="query = ''"
        />
        <button
          class="tag-picker-add"
          type="button"
          :disabled="!query.trim() || tagStore.isMutating"
          @click="addTagFromQuery"
        >
          <PlusIcon :size="14" />
          追加
        </button>
      </div>

      <ul v-if="query.trim() && availableTags.length > 0" class="tag-suggestions" role="listbox" aria-label="候補のタグ">
        <li v-for="tag in availableTags" :key="tag.id">
          <button
            type="button"
            :disabled="tagStore.isMutating"
            @mousedown.prevent="attachTag(tag)"
          >
            <TagIcon :size="13" aria-hidden="true" />
            {{ tag.name }}
          </button>
        </li>
      </ul>
    </div>

    <span v-else class="note-tags-readonly">ゴミ箱内のノートではタグを変更できません。</span>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { PlusIcon, TagIcon, XIcon } from '@lucide/vue'
import type { note } from '../../wailsjs/go/models'
import { TagApiError } from '../api/tags'
import { useTagStore } from '../stores/useTagStore'

const props = withDefaults(defineProps<{
  noteId: string
  disabled?: boolean
}>(), {
  disabled: false,
})

const tagStore = useTagStore()
const query = ref('')

const activeTags = computed(() => (
  tagStore.activeNoteId === props.noteId ? tagStore.activeNoteTags : []
))

const availableTags = computed(() => {
  const normalizedQuery = query.value.normalize('NFC').trim().toLocaleLowerCase()
  const attachedIDs = new Set(activeTags.value.map((tag) => tag.id))

  return tagStore.tags
    .filter((tag) => !attachedIDs.has(tag.id))
    .filter((tag) => tag.name.normalize('NFC').toLocaleLowerCase().includes(normalizedQuery))
    .slice(0, 8)
})

watch(() => props.noteId, (noteId) => {
  query.value = ''
  void tagStore.loadNoteTags(noteId)
}, { immediate: true })

function findSameNameTag(value: string) {
  const candidate = value.normalize('NFC').trim()
  return tagStore.tags.find((tag) => (
    tag.name.localeCompare(candidate, undefined, { sensitivity: 'accent' }) === 0
  ))
}

async function attachTag(tag: note.Tag) {
  try {
    await tagStore.attachTagToNote(props.noteId, tag.id)
    query.value = ''
  } catch (_) {
    // The tag store reports the operation error through the notification center.
  }
}

async function detachTag(tagID: string) {
  try {
    await tagStore.detachTagFromNote(props.noteId, tagID)
  } catch (_) {
    // The tag store reports the operation error through the notification center.
  }
}

async function addTagFromQuery() {
  const name = query.value
  if (!name.trim() || props.disabled) return

  const existing = findSameNameTag(name)
  if (existing) {
    await attachTag(existing)
    return
  }

  try {
    const created = await tagStore.createTag(name)
    await tagStore.attachTagToNote(props.noteId, created.id)
    query.value = ''
  } catch (cause) {
    if (!(cause instanceof TagApiError) || cause.code !== 'TAG_NAME_CONFLICT') return

    await tagStore.fetchTags()
    const conflictingTag = findSameNameTag(name)
    if (conflictingTag) await attachTag(conflictingTag)
  }
}
</script>

<style scoped>
.note-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
  min-height: 34px;
  padding: 5px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-editor);
}

.note-tags-label,
.note-tags-status,
.note-tags-readonly {
  color: var(--text-secondary);
  font-size: 12px;
}

.note-tags-label {
  flex: 0 0 auto;
  font-weight: 600;
}

.tag-chip-list {
  display: flex;
  flex: 0 1 auto;
  flex-wrap: wrap;
  gap: 5px;
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

.tag-chip > span {
  overflow: hidden;
  text-overflow: ellipsis;
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

.tag-chip-remove:disabled,
.tag-picker-add:disabled,
.tag-suggestions button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.tag-picker {
  position: relative;
  flex: 1 1 210px;
  max-width: 340px;
}

.tag-picker-input-row {
  display: flex;
  gap: 4px;
}

.tag-picker-input {
  width: 100%;
  min-width: 0;
  height: 25px;
  padding: 3px 7px;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
}

.tag-picker-input:focus {
  outline: 2px solid color-mix(in srgb, var(--color-primary) 35%, transparent);
  outline-offset: -1px;
}

.tag-picker-add {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 2px;
  padding: 0 7px;
  color: var(--text-primary);
  font-size: 12px;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
}

.tag-picker-add:hover:not(:disabled) {
  background: var(--border);
}

.tag-suggestions {
  position: absolute;
  z-index: 10;
  top: calc(100% + 3px);
  right: 0;
  left: 0;
  max-height: 180px;
  padding: 3px;
  margin: 0;
  overflow-y: auto;
  list-style: none;
  background: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 5px;
  box-shadow: 0 5px 14px rgba(0, 0, 0, 0.18);
}

.tag-suggestions button {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 6px;
  padding: 6px 7px;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 3px;
  cursor: pointer;
}

.tag-suggestions button:hover:not(:disabled) {
  background: var(--bg-hover);
}
</style>
