<template>
  <section class="shortcut-settings">
    <div class="shortcut-settings-heading">
      <div>
        <h3>ショートカット</h3>
        <p>
          アプリ内で使うキーを変更できます。元に戻す／やり直すはノート本文の編集中だけ動作します。
        </p>
      </div>
      <button type="button" class="secondary-btn" @click="resetAllBindings">
        すべて初期値に戻す
      </button>
    </div>

    <p class="shortcut-capture-help">
      「変更」を選んで新しいキーを押してください。Esc、Delete、Backspaceで割り当てを解除します。
    </p>
    <p
      v-if="feedback?.actionId === null"
      class="shortcut-feedback shortcut-global-feedback"
      role="status"
    >
      {{ feedback.message }}
    </p>

    <ul class="shortcut-list">
      <li v-for="action in shortcutActions" :key="action.id" class="shortcut-row">
        <div class="shortcut-description">
          <span class="shortcut-label">{{ action.label }}</span>
          <span class="shortcut-scope">{{ action.scope === 'editor' ? 'ノート本文' : 'アプリ全体' }}</span>
        </div>
        <kbd>{{ formatShortcutBinding(settingsStore.shortcutBindings[action.id]) }}</kbd>
        <button
          type="button"
          class="secondary-btn shortcut-change"
          data-shortcut-capture
          :aria-pressed="capturingActionId === action.id"
          @click="beginCapture(action.id)"
          @keydown="handleCaptureKeydown(action.id, $event)"
        >
          {{ capturingActionId === action.id ? 'キーを入力…' : '変更' }}
        </button>
        <button
          type="button"
          class="text-btn"
          :disabled="shortcutBindingsEqual(settingsStore.shortcutBindings[action.id], action.defaultBinding)"
          @click="resetBinding(action.id)"
        >
          初期値
        </button>
        <p
          v-if="feedback?.actionId === action.id"
          class="shortcut-feedback"
          :class="{ 'is-error': feedback.kind === 'error' }"
          role="status"
        >
          {{ feedback.message }}
        </p>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useSettingsStore } from '../stores/useSettingsStore'
import {
  SHORTCUT_ACTIONS,
  bindingFromKeyboardEvent,
  formatShortcutBinding,
  shortcutBindingsEqual,
  type ShortcutActionId,
} from '../utils/keyboardShortcuts'

const settingsStore = useSettingsStore()
const shortcutActions = SHORTCUT_ACTIONS
const capturingActionId = ref<ShortcutActionId | null>(null)
const feedback = ref<{
  actionId: ShortcutActionId | null
  kind: 'success' | 'error'
  message: string
} | null>(null)

function beginCapture(actionId: ShortcutActionId) {
  capturingActionId.value = actionId
  feedback.value = {
    actionId,
    kind: 'success',
    message: '新しいキーを入力してください。',
  }
}

function handleCaptureKeydown(actionId: ShortcutActionId, event: KeyboardEvent) {
  if (capturingActionId.value !== actionId) return
  event.preventDefault()
  event.stopPropagation()

  if (
    event.code === 'Escape'
    || event.code === 'Delete'
    || event.code === 'Backspace'
  ) {
    const result = settingsStore.setShortcutBinding(actionId, null)
    if (result.ok) {
      capturingActionId.value = null
      feedback.value = { actionId, kind: 'success', message: '割り当てを解除しました。' }
    }
    return
  }

  const binding = bindingFromKeyboardEvent(event)
  if (!binding) {
    feedback.value = {
      actionId,
      kind: 'error',
      message: 'Ctrl、Alt、Metaのいずれか、またはF1〜F12を含めてください。',
    }
    return
  }

  const result = settingsStore.setShortcutBinding(actionId, binding)
  if (!result.ok) {
    feedback.value = { actionId, kind: 'error', message: result.message }
    return
  }

  capturingActionId.value = null
  feedback.value = { actionId, kind: 'success', message: 'ショートカットを変更しました。' }
}

function resetBinding(actionId: ShortcutActionId) {
  const result = settingsStore.resetShortcutBinding(actionId)
  feedback.value = result.ok
    ? { actionId, kind: 'success', message: '初期値に戻しました。' }
    : { actionId, kind: 'error', message: result.message }
}

function resetAllBindings() {
  settingsStore.resetAllShortcutBindings()
  capturingActionId.value = null
  feedback.value = {
    actionId: null,
    kind: 'success',
    message: 'すべてのショートカットを初期値に戻しました。',
  }
}
</script>

<style scoped>
.shortcut-settings-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.shortcut-settings-heading h3 {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 1.1rem;
}

.shortcut-settings-heading p,
.shortcut-capture-help {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.shortcut-capture-help {
  margin-top: 16px;
}

.shortcut-list {
  margin: 16px 0 0;
  padding: 0;
  list-style: none;
}

.shortcut-row {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) 120px auto auto;
  gap: 12px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}

.shortcut-description {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.shortcut-label {
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 500;
}

.shortcut-scope {
  color: var(--text-tertiary);
  font-size: 11px;
}

kbd {
  justify-self: start;
  min-width: 76px;
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-family: inherit;
  font-size: 12px;
  text-align: center;
  white-space: nowrap;
}

.secondary-btn,
.text-btn {
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font-size: 12px;
}

.secondary-btn {
  min-height: 30px;
  padding: 5px 10px;
}

.secondary-btn:hover,
.secondary-btn[aria-pressed='true'] {
  border-color: var(--brand-primary);
  background: var(--bg-active);
}

.text-btn {
  padding: 4px 6px;
  border-color: transparent;
  background: transparent;
  color: var(--text-secondary);
}

.text-btn:hover:not(:disabled) {
  color: var(--brand-primary);
}

.text-btn:disabled {
  cursor: default;
  opacity: 0.45;
}

.shortcut-feedback {
  grid-column: 2 / -1;
  margin: -4px 0 0;
  color: var(--brand-primary);
  font-size: 12px;
}

.shortcut-feedback.is-error {
  color: var(--danger, #d14343);
}

.shortcut-global-feedback {
  margin-top: 8px;
}

@media (max-width: 720px) {
  .shortcut-settings-heading {
    flex-direction: column;
  }

  .shortcut-row {
    grid-template-columns: minmax(0, 1fr) auto auto;
  }

  .shortcut-description {
    grid-column: 1 / -1;
  }

  .shortcut-feedback {
    grid-column: 1 / -1;
  }
}
</style>
