<template>
  <div class="notebook-icon-picker">
    <RadioGroupRoot
      v-model="selectedIcon"
      class="notebook-icon-grid"
      aria-label="ノートブックアイコン"
    >
      <div
        v-for="icon in icons"
        :key="icon.id"
        class="notebook-icon-option-wrapper"
      >
        <RadioGroupItem
          class="notebook-icon-option"
          :value="icon.id"
          :aria-label="icon.label"
          :title="icon.label"
        >
          <img :src="icon.src" :alt="icon.label" />
        </RadioGroupItem>
        <button
          v-if="allowUserIconDelete && icon.source === 'user'"
          class="notebook-icon-delete-btn"
          type="button"
          title="ユーザー追加アイコンを削除"
          @click.stop="deleteUserIcon(icon.id)"
        >
          &times;
        </button>
      </div>
    </RadioGroupRoot>

    <div class="notebook-user-icon-row">
      <input
        ref="fileInputRef"
        class="notebook-user-icon-input"
        type="file"
        :accept="USER_ICON_ACCEPT"
        @change="handleFileChange"
      />
      <button class="secondary-btn" type="button" @click="fileInputRef?.click()">
        画像を追加
      </button>
      <span v-if="error" class="notebook-icon-error">{{ error }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { RadioGroupItem, RadioGroupRoot } from 'reka-ui'
import {
  DEFAULT_NOTEBOOK_ICON,
  USER_ICON_ACCEPT,
  addUserNotebookIcon,
  getNotebookIconOptions,
  removeUserNotebookIcon,
} from '../utils/notebookIcons'

const props = withDefaults(defineProps<{
  modelValue: string
  allowUserIconDelete?: boolean
}>(), {
  allowUserIconDelete: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const version = ref(0)
const fileInputRef = ref<HTMLInputElement | null>(null)
const error = ref('')
const selectedIcon = computed({
  get: () => props.modelValue,
  set: (value: string) => emit('update:modelValue', value),
})
const icons = computed(() => {
  version.value
  return getNotebookIconOptions()
})

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  error.value = ''
  try {
    const icon = await addUserNotebookIcon(file)
    version.value += 1
    emit('update:modelValue', icon.id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'アイコン画像の追加に失敗しました'
  }
}

function deleteUserIcon(iconId: string) {
  error.value = ''
  if (!removeUserNotebookIcon(iconId)) {
    return
  }

  version.value += 1
  if (props.modelValue === iconId) {
    emit('update:modelValue', DEFAULT_NOTEBOOK_ICON)
  }
}
</script>

<style scoped>
.notebook-icon-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.notebook-icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(44px, 1fr));
  gap: 8px;
}

.notebook-icon-option-wrapper {
  position: relative;
  aspect-ratio: 1;
}

.notebook-icon-option {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  aspect-ratio: 1;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-input);
  overflow: hidden;
  transition: border-color 0.12s, box-shadow 0.12s, background 0.12s;
}

.notebook-icon-delete-btn {
  position: absolute;
  top: 3px;
  right: 3px;
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.62);
  color: #fff;
  font-size: 14px;
  line-height: 1;
  opacity: 0;
  transition: opacity 0.12s, background 0.12s;
}

.notebook-icon-option-wrapper:hover .notebook-icon-delete-btn,
.notebook-icon-delete-btn:focus-visible {
  opacity: 1;
}

.notebook-icon-delete-btn:hover {
  background: var(--color-danger);
}

.notebook-icon-option:hover {
  border-color: var(--border-strong);
  background: var(--bg-hover);
}

.notebook-icon-option[data-state='checked'] {
  border-color: var(--brand-primary);
  box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.25);
}

.notebook-icon-option img {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  object-fit: cover;
}

.notebook-user-icon-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 30px;
}

.notebook-user-icon-input {
  display: none;
}

.secondary-btn {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 13px;
}

.secondary-btn:hover {
  background: var(--bg-hover);
}

.notebook-icon-error {
  color: var(--color-danger);
  font-size: 12px;
  line-height: 1.4;
}
</style>
