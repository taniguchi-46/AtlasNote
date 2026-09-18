<template>
  <div class="mermaid-insert-popover">
    <button
      class="format-btn"
      type="button"
      :disabled="disabled"
      title="Mermaid図を挿入"
      aria-label="Mermaid図を挿入"
      aria-haspopup="dialog"
      :aria-expanded="open"
      @click="toggle"
      @keydown.esc="close"
    >
      <WorkflowIcon :size="15" />
    </button>

    <div v-if="open" class="mermaid-insert-popover-panel" role="dialog" aria-label="Mermaid図の種類を選択">
      <div class="mermaid-insert-popover-heading">
        <strong>図の種類を選択</strong>
        <button type="button" class="mermaid-insert-popover-close" aria-label="閉じる" @click="close">×</button>
      </div>
      <div class="mermaid-insert-popover-list">
        <button
          v-for="diagram in MERMAID_DIAGRAM_CATALOG"
          :key="diagram.type"
          type="button"
          class="mermaid-insert-popover-item"
          @click="selectDiagram(diagram.type)"
        >
          <span class="mermaid-insert-popover-item-title">{{ diagram.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { WorkflowIcon } from '@lucide/vue'
import { MERMAID_DIAGRAM_CATALOG, type MermaidDiagramType } from '../utils/mermaidVisualEditor'

defineProps<{
  disabled?: boolean
}>()

const emit = defineEmits<{
  select: [type: MermaidDiagramType]
}>()

const open = ref(false)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function selectDiagram(type: MermaidDiagramType) {
  emit('select', type)
  close()
}
</script>

<style scoped>
.mermaid-insert-popover {
  position: relative;
}

.mermaid-insert-popover-panel {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 1200;
  display: flex;
  width: min(420px, calc(100vw - 28px));
  max-height: min(620px, calc(100vh - 100px));
  flex-direction: column;
  padding: 12px;
  overflow: hidden;
  border: 1px solid var(--border-strong, var(--border));
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
}

.mermaid-insert-popover-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--text-primary);
  font-size: 13px;
}

.mermaid-insert-popover-close {
  width: 26px;
  height: 26px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 18px;
}

.mermaid-insert-popover-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  overflow: auto;
}

.mermaid-insert-popover-item {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: stretch;
  gap: 3px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
}

.mermaid-insert-popover-item:hover,
.mermaid-insert-popover-item:focus-visible {
  border-color: var(--brand-primary);
  outline: none;
}

.mermaid-insert-popover-item-title {
  font-size: 12px;
  font-weight: 600;
}

@media (max-width: 560px) {
  .mermaid-insert-popover-list {
    grid-template-columns: 1fr;
  }
}
</style>
