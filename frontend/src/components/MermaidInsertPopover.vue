<template>
  <div class="mermaid-insert-popover">
    <div class="mermaid-insert-split-button">
    <button
      class="format-btn mermaid-insert-main"
      type="button"
      :disabled="disabled"
      title="Mermaid図を挿入"
      aria-label="Mermaid図を挿入"
      @click="insertDefault"
    >
      <WorkflowIcon :size="15" />
      <span>{{ selectedDiagram.label }}</span>
    </button>
    <button
      class="format-btn mermaid-insert-toggle"
      type="button"
      :disabled="disabled"
      title="Mermaid図の種類を選択"
      aria-label="Mermaid図の種類を選択"
      aria-haspopup="dialog"
      :aria-expanded="open"
      @click="toggle"
      @keydown.esc="close"
    >
      <ChevronDownIcon :size="14" />
    </button>
    </div>

    <div v-if="open" class="mermaid-insert-popover-panel" role="dialog" aria-label="Mermaid図の種類を選択">
      <div class="mermaid-insert-popover-heading">
        <strong>{{ showAll ? 'すべての図' : '図の種類を選択' }}</strong>
        <button type="button" class="mermaid-insert-popover-close" aria-label="閉じる" @click="close">×</button>
      </div>
      <div class="mermaid-insert-popover-list">
        <button
          v-for="diagram in visibleDiagrams"
          :key="diagram.type"
          type="button"
          class="mermaid-insert-popover-item"
          @click="selectDiagram(diagram.type)"
        >
          <WorkflowIcon :size="15" />
          <span class="mermaid-insert-popover-item-title">{{ diagram.label }}</span>
          <span v-if="diagram.type === selectedType" class="mermaid-insert-popover-check">✓</span>
        </button>
      </div>
      <div v-if="!showAll" class="mermaid-insert-popover-footer">
        <button type="button" class="mermaid-insert-popover-all" @click="showAll = true">すべての図を見る…</button>
      </div>
      <div v-else class="mermaid-insert-popover-footer">
        <button type="button" class="mermaid-insert-popover-all" @click="showAll = false">主要な図に戻る</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDownIcon, WorkflowIcon } from '@lucide/vue'
import {
  getMermaidDiagramDefinition,
  MERMAID_DIAGRAM_CATALOG,
  MERMAID_QUICK_DIAGRAM_TYPES,
  type MermaidDiagramType,
} from '../utils/mermaidVisualEditor'

defineProps<{
  disabled?: boolean
}>()

const emit = defineEmits<{
  select: [type: MermaidDiagramType]
}>()

const open = ref(false)
const showAll = ref(false)
const storageKey = 'atlas-mermaid-last-diagram'
const storedType = localStorage.getItem(storageKey) as MermaidDiagramType | null
const selectedType = ref(MERMAID_DIAGRAM_CATALOG.some((diagram) => diagram.type === storedType) ? storedType! : 'flowchart')
const selectedDiagram = computed(() => getMermaidDiagramDefinition(selectedType.value))
const visibleDiagrams = computed(() => showAll.value
  ? MERMAID_DIAGRAM_CATALOG
  : MERMAID_QUICK_DIAGRAM_TYPES.map((type) => getMermaidDiagramDefinition(type)))

function toggle() {
  open.value = !open.value
  if (open.value) showAll.value = false
}

function close() {
  open.value = false
}

function selectDiagram(type: MermaidDiagramType) {
  selectedType.value = type
  localStorage.setItem(storageKey, type)
  emit('select', type)
  close()
}

function insertDefault() {
  selectDiagram(selectedType.value)
}
</script>

<style scoped>
.mermaid-insert-popover {
  position: relative;
  display: inline-flex;
}

.mermaid-insert-split-button {
  display: inline-flex;
}

.mermaid-insert-main {
  display: inline-flex;
  width: auto;
  min-width: 0;
  flex-shrink: 0;
  align-items: center;
  gap: 5px;
  padding-inline: 7px;
  white-space: nowrap;
  border-radius: 5px 0 0 5px;
}

.mermaid-insert-toggle {
  padding-inline: 7px;
  border-left: 2px solid var(--border-strong, var(--border));
  border-radius: 0 5px 5px 0;
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
  flex-direction: row;
  align-items: center;
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

.mermaid-insert-popover-check {
  margin-left: auto;
  color: var(--brand-primary);
}

.mermaid-insert-popover-footer {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
  text-align: center;
}

.mermaid-insert-popover-all {
  border: 0;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 12px;
}

.mermaid-insert-popover-all:hover,
.mermaid-insert-popover-all:focus-visible {
  color: var(--brand-primary);
}

@media (max-width: 560px) {
  .mermaid-insert-popover-list {
    grid-template-columns: 1fr;
  }
}
</style>
