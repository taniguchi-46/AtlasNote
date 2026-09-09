<template>
  <section class="shortcut-settings" data-settings-anchor="shortcuts" tabindex="-1">
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
        <div class="shortcut-bindings">
          <div v-for="slot in shortcutSlots" :key="slot" class="shortcut-slot">
            <span class="shortcut-slot-label">枠{{ slot + 1 }}</span>
            <kbd>{{ formatShortcutBinding(settingsStore.shortcutBindings[action.id][slot]) }}</kbd>
            <button
              type="button"
              class="secondary-btn shortcut-change"
              data-shortcut-capture
              :aria-pressed="capturingActionId === action.id && capturingSlot === slot"
              @click="beginCapture(action.id, slot)"
              @keydown="handleCaptureKeydown(action.id, slot, $event)"
            >
              {{ capturingActionId === action.id && capturingSlot === slot ? 'キーを入力…' : '変更' }}
            </button>
            <button
              type="button"
              class="text-btn"
              :disabled="settingsStore.shortcutBindings[action.id][slot] === null"
              @click="clearBinding(action.id, slot)"
            >
              解除
            </button>
          </div>
        </div>
        <button
          type="button"
          class="text-btn"
          :disabled="isDefaultBinding(action.id, action.defaultBinding)"
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
  type ShortcutBinding,
  type ShortcutBindingSlot,
} from '../utils/keyboardShortcuts'

const settingsStore = useSettingsStore()
const shortcutActions = SHORTCUT_ACTIONS
const shortcutSlots = [0, 1] as const
const capturingActionId = ref<ShortcutActionId | null>(null)
const capturingSlot = ref<ShortcutBindingSlot | null>(null)
const feedback = ref<{
  actionId: ShortcutActionId | null
  kind: 'success' | 'error'
  message: string
} | null>(null)

function beginCapture(actionId: ShortcutActionId, slot: ShortcutBindingSlot) {
  capturingActionId.value = actionId
  capturingSlot.value = slot
  feedback.value = {
    actionId,
    kind: 'success',
    message: '新しいキーを入力してください。',
  }
}

function handleCaptureKeydown(actionId: ShortcutActionId, slot: ShortcutBindingSlot, event: KeyboardEvent) {
  if (capturingActionId.value !== actionId || capturingSlot.value !== slot) return
  event.preventDefault()
  event.stopPropagation()

  if (
    event.code === 'Escape'
    || event.code === 'Delete'
    || event.code === 'Backspace'
  ) {
    const result = settingsStore.setShortcutBinding(actionId, slot, null)
    if (result.ok) {
      capturingActionId.value = null
      capturingSlot.value = null
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

  const result = settingsStore.setShortcutBinding(actionId, slot, binding)
  if (!result.ok) {
    feedback.value = { actionId, kind: 'error', message: result.message }
    return
  }

  capturingActionId.value = null
  capturingSlot.value = null
  feedback.value = { actionId, kind: 'success', message: 'ショートカットを変更しました。' }
}

function clearBinding(actionId: ShortcutActionId, slot: ShortcutBindingSlot) {
  const result = settingsStore.setShortcutBinding(actionId, slot, null)
  if (capturingActionId.value === actionId && capturingSlot.value === slot) {
    capturingActionId.value = null
    capturingSlot.value = null
  }
  feedback.value = result.ok
    ? { actionId, kind: 'success', message: '割り当てを解除しました。' }
    : { actionId, kind: 'error', message: result.message }
}

function isDefaultBinding(actionId: ShortcutActionId, defaultBinding: ShortcutBinding | null) {
  const slots = settingsStore.shortcutBindings[actionId]
  return shortcutBindingsEqual(slots[0], defaultBinding) && slots[1] === null
}

function resetBinding(actionId: ShortcutActionId) {
  const result = settingsStore.resetShortcutBinding(actionId)
  if (capturingActionId.value === actionId) {
    capturingActionId.value = null
    capturingSlot.value = null
  }
  feedback.value = result.ok
    ? { actionId, kind: 'success', message: '初期値に戻しました。' }
    : { actionId, kind: 'error', message: result.message }
}

function resetAllBindings() {
  settingsStore.resetAllShortcutBindings()
  capturingActionId.value = null
  capturingSlot.value = null
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
  grid-template-columns: minmax(180px, 1fr) minmax(280px, 1.5fr) auto;
  gap: 12px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}

.shortcut-description,
.shortcut-bindings {
  display: flex;
  gap: 8px;
}

.shortcut-description {
  flex-direction: column;
  gap: 3px;
}

.shortcut-bindings {
  flex-wrap: wrap;
}

.shortcut-slot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-width: 132px;
}

.shortcut-slot-label {
  flex-basis: 100%;
  color: var(--text-tertiary);
  font-size: 11px;
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
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .shortcut-description {
    grid-column: 1 / -1;
  }

  .shortcut-feedback {
    grid-column: 1 / -1;
  }

  .shortcut-bindings,
  .shortcut-row > .text-btn {
    grid-column: 1 / -1;
  }
}
</style>
