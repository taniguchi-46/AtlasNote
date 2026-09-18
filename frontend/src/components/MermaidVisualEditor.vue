<template>
  <section class="mermaid-visual-editor" aria-label="Mermaidかんたん編集">
    <div class="mermaid-visual-editor-heading">
      <div>
        <strong>{{ diagramLabel }}</strong>
        <p>{{ diagramDescription }}</p>
      </div>
      <span v-if="documentModel" class="mermaid-visual-editor-count">
        {{ documentModel.elements.length }} 要素
      </span>
    </div>

    <p v-if="!documentModel" class="mermaid-visual-editor-warning" role="status">
      図種を判別できないため、ソース編集を使ってください。
    </p>

    <template v-else>
      <p v-if="documentModel.unknownCount > 0" class="mermaid-visual-editor-warning" role="status">
        解釈できない行 {{ documentModel.unknownCount }} 行は原文を保持しています。変更が必要な場合は「ソース」へ切り替えてください。
      </p>
      <p v-if="validationMessage" class="mermaid-visual-editor-warning" role="alert">
        {{ validationMessage }}
      </p>

      <div class="mermaid-visual-editor-toolbar">
        <label class="mermaid-visual-editor-add">
          <span>要素を追加</span>
          <select v-model="selectedAddKind" :disabled="disabled">
            <option v-for="kind in elementKindDefinitions" :key="kind.kind" :value="kind.kind">
              {{ kind.label }}
            </option>
          </select>
        </label>
        <button type="button" :disabled="disabled" @click="addElement">追加</button>
      </div>

      <div class="mermaid-visual-editor-grid">
        <div class="mermaid-visual-editor-list" role="listbox" aria-label="図の要素">
          <button
            v-for="(element, index) in documentModel.elements"
            :key="element.id"
            type="button"
            class="mermaid-visual-editor-list-item"
            :class="{ 'is-selected': index === selectedIndex, 'is-raw': !element.editable }"
            :aria-selected="index === selectedIndex"
            role="option"
            @click="selectedIndex = index"
          >
            <span class="mermaid-visual-editor-list-number">{{ index + 1 }}</span>
            <span class="mermaid-visual-editor-list-text">
              <strong>{{ getElementLabel(element.kind) }}</strong>
              <span>{{ previewElement(element) }}</span>
            </span>
            <span v-if="!element.editable" class="mermaid-visual-editor-raw-badge">原文</span>
          </button>
          <p v-if="documentModel.elements.length === 0" class="mermaid-visual-editor-empty">
            要素がありません。「追加」から作成できます。
          </p>
        </div>

        <div v-if="selectedElement" class="mermaid-visual-editor-form">
          <div class="mermaid-visual-editor-form-heading">
            <strong>{{ getElementLabel(selectedElement.kind) }}</strong>
            <span>{{ selectedIndex + 1 }} / {{ documentModel.elements.length }}</span>
          </div>

          <p v-if="!selectedElement.editable" class="mermaid-visual-editor-readonly">
            この行は解釈せず原文を保持しています。編集する場合は「ソース」を使ってください。
          </p>

          <label v-for="field in selectedElementDefinition.fields" :key="field.key" class="mermaid-visual-editor-field">
            <span>{{ field.label }}</span>
            <select
              v-if="field.type === 'select'"
              :value="selectedElement.fields[field.key] ?? field.options?.[0] ?? ''"
              :disabled="disabled || !selectedElement.editable"
              @change="updateField(field.key, ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="option in field.options ?? []" :key="option" :value="option">
                {{ option }}
              </option>
            </select>
            <textarea
              v-else-if="field.type === 'textarea'"
              :value="selectedElement.fields[field.key] ?? ''"
              :disabled="disabled || !selectedElement.editable"
              rows="3"
              @input="updateField(field.key, ($event.target as HTMLTextAreaElement).value)"
            />
            <input
              v-else
              :value="selectedElement.fields[field.key] ?? ''"
              :disabled="disabled || !selectedElement.editable"
              type="text"
              @input="updateField(field.key, ($event.target as HTMLInputElement).value)"
            />
          </label>

          <p v-if="connectionNotice" class="mermaid-visual-editor-notice" role="status">
            {{ connectionNotice }}
          </p>

          <div class="mermaid-visual-editor-form-actions">
            <button type="button" :disabled="disabled || selectedIndex <= 0" @click="moveElement(-1)">↑ 上へ</button>
            <button type="button" :disabled="disabled || selectedIndex >= documentModel.elements.length - 1" @click="moveElement(1)">↓ 下へ</button>
            <button type="button" class="danger" :disabled="disabled || !selectedElement.editable" @click="removeElement">削除</button>
          </div>
        </div>
        <p v-else class="mermaid-visual-editor-empty mermaid-visual-editor-form-empty">
          左の一覧から要素を選択してください。
        </p>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  createMermaidElement,
  flowchartConnectionCount,
  generateMermaidSource,
  getMermaidDiagramDefinition,
  getMermaidElementDefinition,
  getMermaidElementLabel,
  getMermaidElementKindDefinitions,
  isMermaidEditorValueSafe,
  parseMermaidVisualSource,
  type MermaidDiagramType,
  type MermaidVisualDocument,
  type MermaidVisualElement,
} from '../utils/mermaidVisualEditor'

const props = defineProps<{
  source: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:source': [source: string]
}>()

const documentModel = ref<MermaidVisualDocument | null>(null)
const selectedIndex = ref(0)
const selectedAddKind = ref('')
const validationMessage = ref('')
const connectionNotice = ref('')
let lastEmittedSource = ''

const diagramType = computed<MermaidDiagramType | null>(() => documentModel.value?.type ?? null)
const diagramDefinition = computed(() => diagramType.value ? getMermaidDiagramDefinition(diagramType.value) : null)
const diagramLabel = computed(() => diagramDefinition.value?.label ?? 'Mermaid図')
const diagramDescription = computed(() => diagramDefinition.value?.description ?? '')
const elementKindDefinitions = computed(() => diagramType.value ? getMermaidElementKindDefinitions(diagramType.value) : [])
const selectedElement = computed(() => documentModel.value?.elements[selectedIndex.value] ?? null)
const selectedElementDefinition = computed(() => diagramType.value && selectedElement.value
  ? getMermaidElementDefinition(diagramType.value, selectedElement.value.kind)
  : { kind: 'raw', label: 'その他の行', fields: [{ key: 'content', label: '内容', type: 'textarea' as const }] })

function loadSource(source: string) {
  const parsed = parseMermaidVisualSource(source)
  documentModel.value = parsed
  selectedIndex.value = parsed && parsed.elements.length > 0 ? Math.min(selectedIndex.value, parsed.elements.length - 1) : 0
  selectedAddKind.value = parsed?.type ? (getMermaidElementKindDefinitions(parsed.type)[0]?.kind ?? '') : ''
  validationMessage.value = ''
  connectionNotice.value = ''
  lastEmittedSource = source
}

function commit() {
  const model = documentModel.value
  if (!model) return
  const nextSource = generateMermaidSource(model)
  if (!isMermaidEditorValueSafe(nextSource)) {
    validationMessage.value = '安全に扱えない記法が含まれるため、ソースを更新していません。'
    return
  }
  validationMessage.value = ''
  if (nextSource === lastEmittedSource) return
  lastEmittedSource = nextSource
  emit('update:source', nextSource)
}

function updateField(key: string, value: string) {
  const item = selectedElement.value
  if (!item || !item.editable || props.disabled) return
  item.fields[key] = value.replace(/\u0000/g, '')
  item.dirty = true
  connectionNotice.value = ''
  commit()
}

function addElement() {
  const model = documentModel.value
  if (!model || props.disabled || !selectedAddKind.value) return
  const item = createMermaidElement(model.type, selectedAddKind.value, model.elements.length)
  model.elements.push(item)
  selectedIndex.value = model.elements.length - 1
  connectionNotice.value = ''
  commit()
}

function removeElement() {
  const model = documentModel.value
  const item = selectedElement.value
  if (!model || !item || props.disabled || !item.editable) return

  let removedConnections = 0
  if (model.type === 'flowchart' && item.kind === 'node') {
    const nodeId = item.fields.id ?? ''
    removedConnections = flowchartConnectionCount(model, nodeId)
    model.elements = model.elements.filter((candidate) => candidate !== item && !(candidate.kind === 'edge' && (candidate.fields.from === nodeId || candidate.fields.to === nodeId)))
  } else {
    model.elements.splice(selectedIndex.value, 1)
  }

  selectedIndex.value = Math.max(0, Math.min(selectedIndex.value, model.elements.length - 1))
  connectionNotice.value = removedConnections > 0
    ? `この図形に接続していた ${removedConnections} 件の接続も削除しました。Undoで一括復元できます。`
    : ''
  commit()
}

function moveElement(delta: number) {
  const model = documentModel.value
  const index = selectedIndex.value
  const nextIndex = index + delta
  if (!model || props.disabled || index < 0 || nextIndex < 0 || nextIndex >= model.elements.length) return
  const [item] = model.elements.splice(index, 1)
  model.elements.splice(nextIndex, 0, item)
  selectedIndex.value = nextIndex
  commit()
}

function getElementLabel(kind: string) {
  return diagramType.value ? getMermaidElementLabel(diagramType.value, kind) : '要素'
}

function previewElement(item: MermaidVisualElement) {
  const fields = item.fields
  return fields.label || fields.name || fields.content || fields.message || fields.id || fields.from || fields.raw || '空の要素'
}

watch(
  () => props.source,
  (source) => {
    if (!documentModel.value || (source !== lastEmittedSource && generateMermaidSource(documentModel.value) !== source)) {
      loadSource(source)
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.mermaid-visual-editor {
  display: flex;
  min-height: 320px;
  flex-direction: column;
  gap: 10px;
}

.mermaid-visual-editor-heading,
.mermaid-visual-editor-toolbar,
.mermaid-visual-editor-form-heading,
.mermaid-visual-editor-form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.mermaid-visual-editor-heading p {
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
}

.mermaid-visual-editor-count,
.mermaid-visual-editor-form-heading span {
  color: var(--text-secondary);
  font-size: 12px;
}

.mermaid-visual-editor-warning,
.mermaid-visual-editor-notice,
.mermaid-visual-editor-readonly {
  margin: 0;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--brand-primary) 30%, var(--border));
  border-radius: 6px;
  background: color-mix(in srgb, var(--brand-primary) 8%, var(--bg-input));
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.mermaid-visual-editor-toolbar {
  justify-content: flex-start;
}

.mermaid-visual-editor-add {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 12px;
}

.mermaid-visual-editor-toolbar select,
.mermaid-visual-editor-toolbar button,
.mermaid-visual-editor-form-actions button {
  min-height: 30px;
  padding: 0 9px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.mermaid-visual-editor-toolbar button {
  border-color: var(--brand-primary);
}

.mermaid-visual-editor-grid {
  display: grid;
  min-height: 240px;
  grid-template-columns: minmax(190px, 0.9fr) minmax(220px, 1.1fr);
  gap: 10px;
}

.mermaid-visual-editor-list,
.mermaid-visual-editor-form {
  min-height: 0;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
}

.mermaid-visual-editor-list {
  padding: 4px;
}

.mermaid-visual-editor-list-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 7px;
  padding: 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
}

.mermaid-visual-editor-list-item:hover,
.mermaid-visual-editor-list-item.is-selected {
  background: color-mix(in srgb, var(--brand-primary) 12%, transparent);
}

.mermaid-visual-editor-list-item.is-raw {
  color: var(--text-secondary);
}

.mermaid-visual-editor-list-number {
  min-width: 20px;
  color: var(--text-secondary);
  font-size: 11px;
  text-align: right;
}

.mermaid-visual-editor-list-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
  font-size: 12px;
}

.mermaid-visual-editor-list-text span {
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mermaid-visual-editor-raw-badge {
  margin-left: auto;
  color: var(--text-secondary);
  font-size: 10px;
}

.mermaid-visual-editor-form {
  padding: 10px;
}

.mermaid-visual-editor-field {
  display: flex;
  margin-top: 10px;
  flex-direction: column;
  gap: 4px;
  color: var(--text-secondary);
  font-size: 12px;
}

.mermaid-visual-editor-field input,
.mermaid-visual-editor-field textarea,
.mermaid-visual-editor-field select {
  box-sizing: border-box;
  width: 100%;
  padding: 7px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-editor);
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
}

.mermaid-visual-editor-field textarea {
  resize: vertical;
}

.mermaid-visual-editor-form-actions {
  justify-content: flex-end;
  margin-top: 14px;
}

.mermaid-visual-editor-form-actions .danger {
  border-color: color-mix(in srgb, var(--color-danger, #c0392b) 55%, var(--border));
  color: var(--color-danger, #c0392b);
}

.mermaid-visual-editor-empty {
  margin: 0;
  padding: 18px 10px;
  color: var(--text-secondary);
  font-size: 12px;
  text-align: center;
}

.mermaid-visual-editor-form-empty {
  border: 1px solid var(--border);
  border-radius: 6px;
}

@media (max-width: 720px) {
  .mermaid-visual-editor-grid {
    grid-template-columns: 1fr;
  }

  .mermaid-visual-editor-list {
    max-height: 180px;
  }
}
</style>
