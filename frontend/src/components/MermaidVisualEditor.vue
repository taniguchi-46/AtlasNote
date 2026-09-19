<template>
  <section
    ref="editorRoot"
    class="mermaid-visual-editor"
    aria-label="Mermaidキャンバス編集"
    tabindex="0"
    @keydown="handleKeydown"
  >
    <div class="mermaid-visual-editor-toolbar">
      <div class="mermaid-visual-editor-tools">
        <button
          type="button"
          :class="{ 'is-active': mode === 'select' }"
          title="選択"
          aria-label="選択"
          @click="activateSelectMode"
        >
          ↖<span>選択</span>
        </button>
        <button
          type="button"
          :class="{ 'is-active': mode === 'connect' }"
          :disabled="editorDisabled"
          title="接続を作成"
          aria-label="接続を作成"
          @click="toggleConnectMode"
        >
          ↗<span>接続</span>
        </button>
        <div class="mermaid-visual-editor-arrange-menu">
          <button
            type="button"
            :disabled="editorDisabled"
            :class="{ 'is-active': arrangeMenuOpen }"
            title="配置メニュー"
            aria-label="配置メニュー"
            aria-haspopup="menu"
            :aria-expanded="arrangeMenuOpen"
            @click.stop="arrangeMenuOpen = !arrangeMenuOpen"
          >
            ⌗<span>配置</span>
          </button>
          <div v-if="arrangeMenuOpen" class="mermaid-visual-editor-arrange-popover" role="menu">
            <button type="button" role="menuitem" :disabled="editorDisabled" @click="autoArrange(); arrangeMenuOpen = false">自動整列</button>
            <button type="button" role="menuitem" :disabled="editorDisabled || selectedIds.size < 2" @click="alignSelected('horizontal'); arrangeMenuOpen = false">横に整列</button>
            <button type="button" role="menuitem" :disabled="editorDisabled || selectedIds.size < 2" @click="alignSelected('vertical'); arrangeMenuOpen = false">縦に整列</button>
          </div>
        </div>
        <button
          type="button"
          :class="{ 'is-active': propertiesOpen }"
          title="プロパティを表示"
          aria-label="プロパティを表示"
          @click="propertiesOpen = !propertiesOpen"
        >
          ⚙<span>プロパティ</span>
        </button>
        <span class="mermaid-visual-editor-divider" />
        <button
          v-for="kind in paletteKinds"
          :key="`tool-${kind.kind}`"
          type="button"
          :disabled="editorDisabled"
          :title="kind.label"
          :aria-label="kind.label"
          @click="addElement(kind.kind)"
        >
          <span class="mermaid-visual-editor-tool-icon">{{ toolIcon(kind.sourceKind) }}</span>
          <span>{{ shortLabel(kind.label) }}</span>
        </button>
        <span class="mermaid-visual-editor-divider" />
        <button
          type="button"
          :disabled="editorDisabled || !selectedElement"
          title="複製"
          aria-label="複製"
          @click="duplicateElement"
        >
          ⧉
        </button>
        <button
          type="button"
          :disabled="editorDisabled || !canUndo"
          title="元に戻す"
          aria-label="元に戻す"
          @click="undo"
        >
          ↶
        </button>
        <button
          type="button"
          :disabled="editorDisabled || !canRedo"
          title="やり直す"
          aria-label="やり直す"
          @click="redo"
        >
          ↷
        </button>
      </div>
      <div class="mermaid-visual-editor-zoom" aria-label="ズーム">
        <button type="button" title="縮小" aria-label="縮小" @click="setZoom(zoom - 0.1)">−</button>
        <span>{{ Math.round(zoom * 100) }}%</span>
        <button type="button" title="拡大" aria-label="拡大" @click="setZoom(zoom + 0.1)">＋</button>
      </div>
    </div>

    <p v-if="validationMessage" class="mermaid-visual-editor-warning" role="alert">
      {{ validationMessage }}
    </p>

    <div v-if="!documentModel" class="mermaid-visual-editor-fallback" role="status">
      図種を判別できないため、下のMermaidソースを使って編集してください。
    </div>

    <div v-else class="mermaid-visual-editor-layout">
      <aside class="mermaid-visual-editor-palette" aria-label="要素">
        <strong>要素</strong>
        <button
          v-for="kind in paletteKinds"
          :key="`palette-${kind.kind}`"
          type="button"
          :disabled="editorDisabled"
          class="mermaid-visual-editor-palette-item"
          @click="addElement(kind.kind)"
        >
          <span class="mermaid-visual-editor-palette-icon">{{ toolIcon(kind.sourceKind) }}</span>
          <span>
            <b>{{ kind.label }}</b>
            <small>{{ paletteHint(kind.sourceKind) }}</small>
          </span>
        </button>
        <p v-if="documentModel.unknownCount" class="mermaid-visual-editor-hint">
          未解釈の行 {{ documentModel.unknownCount }} 行は原文を保持しています。必要な場合は下のソースを編集してください。
        </p>
      </aside>

      <div
        ref="canvas"
        class="mermaid-visual-editor-canvas"
        @pointerdown="handleCanvasPointerDown"
      >
        <div class="mermaid-visual-editor-stage-shell" :style="stageShellStyle">
          <div class="mermaid-visual-editor-stage" :style="stageStyle">
            <div class="mermaid-visual-editor-grid-pattern" />
            <svg
              class="mermaid-visual-editor-edges"
              :viewBox="`0 0 ${CANVAS_WIDTH} ${CANVAS_HEIGHT}`"
              role="img"
              aria-label="Mermaidの接続線"
            >
              <g
                v-for="lifeline in sequenceLifelines"
                :key="lifeline.id"
                class="mermaid-visual-editor-lifeline"
              >
                <line :x1="lifeline.x" :y1="lifeline.y1" :x2="lifeline.x" :y2="lifeline.y2" />
              </g>
              <g
                v-for="edge in visibleEdges"
                :key="edge.id"
                class="mermaid-visual-editor-edge"
                :class="{ 'is-selected': selectedIds.has(edge.id) }"
                @click.stop="selectElementById(edge.id)"
                @dblclick.stop="selectElementById(edge.id)"
                @pointerdown.stop="startMessageDrag($event, edge.id)"
              >
                <line
                  :x1="edge.x1"
                  :y1="edge.y1"
                  :x2="edge.x2"
                  :y2="edge.y2"
                  :class="{ 'is-dashed': edge.dashed }"
                />
                <polygon v-if="edge.hasArrow" :points="arrowPoints(edge)" />
                <text v-if="edge.label" :x="(edge.x1 + edge.x2) / 2" :y="(edge.y1 + edge.y2) / 2 - 8">
                  {{ edge.label }}
                </text>
              </g>
              <line
                v-if="connectionDrag"
                class="mermaid-visual-editor-connection-preview"
                :x1="connectionDrag.x1"
                :y1="connectionDrag.y1"
                :x2="connectionDrag.x2"
                :y2="connectionDrag.y2"
              />
            </svg>

            <div
              v-for="block in sequenceBlocks"
              :key="block.id"
              class="mermaid-visual-editor-sequence-block"
              :style="sequenceBlockStyle(block)"
              :aria-label="`${block.label}ブロック`"
            >
              <span>{{ block.label }}</span>
            </div>

            <button
              v-for="item in drawableElements"
              :key="item.id"
              type="button"
              class="mermaid-visual-editor-node"
              :class="elementClasses(item)"
              :style="elementStyle(item)"
              :title="mode === 'connect' || connectionDrag ? '接続先として選択' : item.editable ? 'クリックで選択、ダブルクリックで編集' : '原文を保持している行'"
              @click.stop="handleNodeClick($event, item)"
              @dblclick.stop="beginInlineEdit(item)"
              @pointerdown.stop="startDrag($event, item)"
              @pointerup.stop="finishConnectionDrag(item)"
            >
              <span
                v-for="side in connectionPointSides"
                v-show="canConnectItem(item)"
                :key="side"
                class="mermaid-visual-editor-connection-point"
                :class="`is-${side}`"
                role="button"
                tabindex="-1"
                :aria-label="`${side}から接続`"
                @pointerdown.stop.prevent="startConnectionDrag($event, item, side)"
              />
              <input
                v-if="editingId === item.id"
                class="mermaid-visual-editor-inline-input"
                :value="displayLabel(item)"
                @click.stop
                @keydown.enter.prevent="finishInlineEdit(item, $event)"
                @keydown.esc.prevent="cancelInlineEdit"
                @compositionstart="inlineComposing = true"
                @compositionend="finishInlineComposition(item, $event)"
                @blur="finishInlineEdit(item, $event)"
              />
              <template v-else>
                <span class="mermaid-visual-editor-node-kind">{{ getElementLabel(item.kind) }}</span>
                <span>{{ displayLabel(item) }}</span>
              </template>
            </button>

            <p v-if="!drawableElements.length" class="mermaid-visual-editor-empty">
              左の要素から追加できます
            </p>
          </div>
        </div>
      </div>

      <aside v-if="propertiesOpen" class="mermaid-visual-editor-properties" aria-label="プロパティ">
        <template v-if="selectedElement">
          <strong>プロパティ</strong>
          <p class="mermaid-visual-editor-selection-summary">
            {{ selectedIds.size > 1 ? `${selectedIds.size}個を選択中` : getElementLabel(selectedElement.kind) }}
          </p>
          <label class="mermaid-visual-editor-field">
            <span>種類</span>
            <input :value="getElementLabel(selectedElement.kind)" disabled />
          </label>
          <template v-for="field in selectedElementDefinition.fields" :key="field.key">
            <label class="mermaid-visual-editor-field">
              <span>{{ field.label }}</span>
              <select
                v-if="field.type === 'select'"
                :value="selectedElement.fields[field.key] ?? field.options?.[0] ?? ''"
                :disabled="editorDisabled || !selectedElement.editable"
                @change="updateField(field.key, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="option in field.options ?? []" :key="option" :value="option">{{ option }}</option>
              </select>
              <textarea
                v-else-if="field.type === 'textarea'"
                :value="selectedElement.fields[field.key] ?? ''"
                :disabled="editorDisabled || !selectedElement.editable"
                rows="3"
                @input="updateField(field.key, ($event.target as HTMLTextAreaElement).value)"
              />
              <input
                v-else
                :value="selectedElement.fields[field.key] ?? ''"
                :disabled="editorDisabled || !selectedElement.editable"
                @input="updateField(field.key, ($event.target as HTMLInputElement).value)"
              />
            </label>
          </template>

          <div v-if="supportsStyles && selectedElement.kind === 'node'" class="mermaid-visual-editor-style-fields">
            <strong>スタイル</strong>
            <label>
              <span>塗りつぶし</span>
              <input
                type="color"
                :value="styleFor(selectedElement).fill"
                :disabled="editorDisabled"
                @input="updateStyle('fill', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label>
              <span>枠線</span>
              <input
                type="color"
                :value="styleFor(selectedElement).stroke"
                :disabled="editorDisabled"
                @input="updateStyle('stroke', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label>
              <span>テキスト色</span>
              <input
                type="color"
                :value="styleFor(selectedElement).text"
                :disabled="editorDisabled"
                @input="updateStyle('text', ($event.target as HTMLInputElement).value)"
              />
            </label>
          </div>

          <div class="mermaid-visual-editor-property-actions">
            <button type="button" :disabled="editorDisabled || !selectedElement.editable" @click="duplicateElement">⧉ 複製</button>
            <button type="button" class="danger" :disabled="editorDisabled || !selectedElement.editable" @click="removeElement">⌫ 削除</button>
          </div>
        </template>
        <p v-else class="mermaid-visual-editor-empty">要素を選択するとプロパティを表示します</p>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  createMermaidElement,
  generateMermaidSource,
  getMermaidElementDefinition,
  getMermaidElementLabel,
  getMermaidElementKindDefinitions,
  isMermaidEditorValueSafe,
  parseMermaidVisualSource,
  sanitizeMermaidStyleValue,
  type MermaidDiagramType,
  type MermaidVisualDocument,
  type MermaidVisualElement,
} from '../utils/mermaidVisualEditor'

type Position = { x: number; y: number }
type StyleValue = { fill: string; stroke: string; text: string }
type EditorSnapshot = {
  document: MermaidVisualDocument | null
  positions: Record<string, Position>
  styles: Record<string, StyleValue>
  source: string
}
type PaletteItem = { kind: string; sourceKind: string; label: string }
type RenderedEdge = {
  id: string
  x1: number
  y1: number
  x2: number
  y2: number
  label: string
  arrow: string
  dashed: boolean
  hasArrow: boolean
}
type SequenceBlockFrame = {
  id: string
  label: string
  x: number
  y: number
  width: number
  height: number
}
type DragState = {
  pointerId: number
  startX: number
  startY: number
  origins: Record<string, Position>
  before: EditorSnapshot
  changed: boolean
  pendingSelectionId: string | null
}
type MessageDragState = {
  pointerId: number
  id: string
  startY: number
  scopeIds: string[]
  initialIndex: number
  before: EditorSnapshot
  changed: boolean
}
type ClipboardItem = {
  kind: string
  fields: Record<string, string>
  raw: string
  editable: boolean
  sourceId?: string
  derived?: boolean
  blockId?: string
  blockRole?: 'start' | 'member' | 'end'
}

const props = defineProps<{ source: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:source': [source: string] }>()

const CANVAS_WIDTH = 760
const CANVAS_HEIGHT = 520
const NODE_WIDTH = 168
const NODE_HEIGHT = 62
const connectionPointSides = ['top', 'right', 'bottom', 'left'] as const
const edgeKinds = new Set(['edge', 'message', 'relation', 'transition', 'flow'])
const referenceFields = ['from', 'to', 'over', 'owner', 'target'] as const
const defaultStyle: StyleValue = { fill: '#eef2ff', stroke: '#7c8ff5', text: '#172554' }

const editorRoot = ref<HTMLElement | null>(null)
const canvas = ref<HTMLElement | null>(null)
const documentModel = ref<MermaidVisualDocument | null>(null)
const selectedId = ref<string | null>(null)
const selectedIds = ref<Set<string>>(new Set())
const editingId = ref<string | null>(null)
const inlineComposing = ref(false)
const mode = ref<'select' | 'connect'>('select')
const connectionSourceId = ref<string | null>(null)
const connectionDrag = ref<{ sourceId: string; x1: number; y1: number; x2: number; y2: number; pointerId: number } | null>(null)
const clipboard = ref<ClipboardItem[]>([])
const zoom = ref(1)
const validationMessage = ref('')
const positions = ref<Record<string, Position>>({})
const stylesByTarget = ref<Record<string, StyleValue>>({})
const undoStack = ref<EditorSnapshot[]>([])
const redoStack = ref<EditorSnapshot[]>([])
const propertiesOpen = ref(true)
const arrangeMenuOpen = ref(false)
const localInputLocked = ref(false)
let lastEmittedSource = ''
let sourceLoaded = false
let drag: DragState | null = null
let messageDrag: MessageDragState | null = null
let suppressNodeClick = false

const editorDisabled = computed(() => props.disabled === true || localInputLocked.value)
const diagramType = computed<MermaidDiagramType | null>(() => documentModel.value?.type ?? null)
const elementKindDefinitions = computed(() => diagramType.value ? getMermaidElementKindDefinitions(diagramType.value) : [])
const paletteKinds = computed<PaletteItem[]>(() => {
  const type = diagramType.value
  if (!type) return []
  const definitions = elementKindDefinitions.value
  if (type === 'sequence') {
    return [
      { kind: 'participant', sourceKind: 'participant', label: '参加者' },
      { kind: 'message', sourceKind: 'message', label: 'メッセージ' },
      { kind: 'note', sourceKind: 'note', label: 'Note' },
      { kind: 'control-alt', sourceKind: 'control', label: '分岐' },
      { kind: 'control-loop', sourceKind: 'control', label: 'ループ' },
    ].filter((item) => definitions.some((definition) => definition.kind === item.sourceKind))
  }
  const preferred = ['node', 'edge', 'group', 'relation', 'transition', 'state', 'entity', 'class', 'item', 'section', 'annotation']
  return preferred
    .map((kind) => definitions.find((definition) => definition.kind === kind))
    .filter((definition): definition is NonNullable<typeof definition> => Boolean(definition))
    .slice(0, 6)
    .map((definition) => ({ kind: definition.kind, sourceKind: definition.kind, label: definition.label }))
})
const selectedElement = computed(() => documentModel.value?.elements.find((item) => item.id === selectedId.value) ?? null)
const selectedElementDefinition = computed(() => diagramType.value && selectedElement.value
  ? getMermaidElementDefinition(diagramType.value, selectedElement.value.kind)
  : { kind: 'raw', label: 'その他の行', fields: [{ key: 'content', label: '内容', type: 'textarea' as const }] })
const drawableElements = computed(() => documentModel.value?.elements.filter((item) => !edgeKinds.has(item.kind)
  && !['raw', 'direction', 'style'].includes(item.kind)
  && !(item.kind === 'control' && item.blockRole === 'end')
  && !(item.kind === 'group' && item.fields.groupEnd === 'true')) ?? [])
const visibleEdges = computed<RenderedEdge[]>(() => {
  const model = documentModel.value
  if (!model) return []
  const edges = model.elements.filter((item) => edgeKinds.has(item.kind))
  const sequenceMessages = edges.filter((item) => item.kind === 'message')
  return edges.map((item) => {
    if (model.type === 'sequence' && item.kind === 'message') {
      const source = findPosition(item.fields.from ?? '')
      const target = findPosition(item.fields.to ?? '')
      const index = Math.max(0, sequenceMessages.findIndex((candidate) => candidate.id === item.id))
      const y = 122 + index * 58
      return renderedEdge(item, source.x + NODE_WIDTH / 2, y, target.x + NODE_WIDTH / 2, y, item.fields.message ?? '')
    }
    const source = findPosition(item.fields.from ?? item.fields.owner ?? '')
    const target = findPosition(item.fields.to ?? item.fields.target ?? '')
    return renderedEdge(item, source.x + NODE_WIDTH / 2, source.y + NODE_HEIGHT / 2, target.x + NODE_WIDTH / 2, target.y + NODE_HEIGHT / 2, item.fields.label ?? item.fields.message ?? '')
  })
})
const sequenceBlocks = computed<SequenceBlockFrame[]>(() => {
  const model = documentModel.value
  if (!model || model.type !== 'sequence') return []
  const messages = model.elements.filter((item) => item.kind === 'message')
  return model.elements
    .filter((item) => item.kind === 'control' && item.blockRole === 'start')
    .map((start) => {
      const startIndex = model.elements.findIndex((item) => item.id === start.id)
      const endIndex = model.elements.findIndex((item, index) => index > startIndex && item.kind === 'control' && item.blockRole === 'end' && item.blockId === start.blockId)
      const lastIndex = endIndex >= 0 ? endIndex : model.elements.length - 1
      const members = model.elements.slice(startIndex, lastIndex + 1)
      const startPosition = positions.value[start.id] ?? { x: 32, y: 112 }
      const messageBottoms = members
        .filter((item) => item.kind === 'message')
        .map((item) => 122 + Math.max(0, messages.findIndex((candidate) => candidate.id === item.id)) * 58 + 24)
      const cardBottoms = members
        .filter((item) => !(item.kind === 'control' && item.blockRole === 'end') && !isEdgeKind(item.kind))
        .map((item) => {
          const position = positions.value[item.id] ?? startPosition
          return position.y + (item.kind === 'control' ? 62 : NODE_HEIGHT) + 16
        })
      const top = Math.max(18, startPosition.y - 12)
      const bottom = Math.min(CANVAS_HEIGHT - 12, Math.max(top + 96, ...messageBottoms, ...cardBottoms))
      return {
        id: start.blockId ?? start.id,
        label: `${start.fields.command ?? 'block'}${start.fields.label ? ` ${start.fields.label}` : ''}`,
        x: 18,
        y: top,
        width: CANVAS_WIDTH - 36,
        height: bottom - top,
      }
    })
})
const sequenceLifelines = computed(() => {
  if (documentModel.value?.type !== 'sequence') return []
  return documentModel.value.elements
    .filter((item) => item.kind === 'participant')
    .map((item) => {
      const p = positions.value[item.id] ?? { x: 40, y: 28 }
      return { id: item.id, x: p.x + NODE_WIDTH / 2, y1: p.y + NODE_HEIGHT + 8, y2: CANVAS_HEIGHT - 18 }
    })
})
const canUndo = computed(() => undoStack.value.length > 0)
const canRedo = computed(() => redoStack.value.length > 0)
const supportsStyles = computed(() => documentModel.value?.type === 'flowchart')
const stageStyle = computed(() => ({
  width: `${CANVAS_WIDTH}px`,
  height: `${CANVAS_HEIGHT}px`,
  transform: `scale(${zoom.value})`,
  transformOrigin: 'top left',
}))
const stageShellStyle = computed(() => ({
  width: `${CANVAS_WIDTH * zoom.value}px`,
  height: `${CANVAS_HEIGHT * zoom.value}px`,
}))

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function isEdgeKind(kind: string) {
  return edgeKinds.has(kind)
}

function identityOf(item: MermaidVisualElement) {
  return item.fields.id || item.fields.name || item.fields.target || item.fields.label || item.fields.content || item.id
}

function positionKey(item: MermaidVisualElement) {
  return `${item.kind}:${identityOf(item)}`
}

function styleTarget(item: MermaidVisualElement) {
  return item.fields.id || item.fields.name || item.fields.target || ''
}

function getElementLabel(kind: string) {
  return diagramType.value ? getMermaidElementLabel(diagramType.value, kind) : '要素'
}

function findElementByReference(reference: string, model = documentModel.value) {
  if (!model || !reference) return undefined
  return model.elements.find((item) => !isEdgeKind(item.kind) && identityOf(item) === reference)
}

function findPosition(reference: string) {
  const item = findElementByReference(reference)
  if (item) return positions.value[item.id] ?? { x: 40, y: 40 }
  return { x: 60, y: 80 }
}

function positionForItem(item: MermaidVisualElement, index: number): Position {
  if (documentModel.value?.type === 'sequence' && item.kind === 'participant') {
    return { x: 42 + index * 210, y: 28 }
  }
  const columns = Math.max(1, Math.floor((CANVAS_WIDTH - 44) / 190))
  return { x: 24 + (index % columns) * 190, y: 28 + Math.floor(index / columns) * 94 }
}

function layoutPositions(parsed: MermaidVisualDocument, previous: MermaidVisualDocument | null, previousPositions: Record<string, Position>) {
  const oldPositions = new Map<string, Position>()
  if (previous) {
    for (const item of previous.elements) {
      const value = previousPositions[item.id]
      if (value) oldPositions.set(positionKey(item), value)
    }
  }
  const next: Record<string, Position> = {}
  const items = parsed.elements.filter((item) => !isEdgeKind(item.kind) && !['raw', 'direction', 'style'].includes(item.kind))
  items.forEach((item, index) => {
    next[item.id] = oldPositions.get(positionKey(item)) ?? positionForItem(item, index)
  })
  return next
}

function extractStyles(parsed: MermaidVisualDocument, previous: Record<string, StyleValue>) {
  const next: Record<string, StyleValue> = {}
  for (const item of parsed.elements) {
    if (item.kind !== 'style' || !item.fields.target) continue
    const previousStyle = previous[item.fields.target] ?? defaultStyle
    next[item.fields.target] = {
      fill: sanitizeMermaidStyleValue(item.fields.fill, previousStyle.fill),
      stroke: sanitizeMermaidStyleValue(item.fields.stroke, previousStyle.stroke),
      text: sanitizeMermaidStyleValue(item.fields.text, previousStyle.text),
    }
  }
  return next
}

function loadSource(source: string, resetHistory: boolean, preserveLayout: boolean) {
  const previous = documentModel.value
  const parsed = parseMermaidVisualSource(source)
  lastEmittedSource = source
  mode.value = 'select'
  connectionSourceId.value = null
  clearConnectionDrag()
  editingId.value = null
  validationMessage.value = parsed ? '' : 'Mermaidソースを解釈できないため、キャンバス編集を利用できません。ソースは保持されています。'
  documentModel.value = parsed
  if (!parsed) {
    selectedId.value = null
    selectedIds.value = new Set()
    positions.value = {}
    stylesByTarget.value = {}
    if (resetHistory) {
      undoStack.value = []
      redoStack.value = []
    }
    sourceLoaded = true
    return
  }
  positions.value = layoutPositions(parsed, preserveLayout ? previous : null, positions.value)
  stylesByTarget.value = extractStyles(parsed, preserveLayout ? stylesByTarget.value : {})
  const previousSelection = selectedId.value && parsed.elements.some((item) => item.id === selectedId.value)
    ? selectedId.value
    : parsed.elements.find((item) => item.editable && !isEdgeKind(item.kind) && !['raw', 'direction', 'style'].includes(item.kind))?.id ?? null
  setSelection(previousSelection ? [previousSelection] : [])
  if (resetHistory) {
    undoStack.value = []
    redoStack.value = []
  }
  sourceLoaded = true
}

function captureSnapshot(source = lastEmittedSource): EditorSnapshot {
  return {
    document: documentModel.value ? clone(documentModel.value) : null,
    positions: clone(positions.value),
    styles: clone(stylesByTarget.value),
    source: source || (documentModel.value ? generateMermaidSource(documentModel.value) : ''),
  }
}

function snapshotsEqual(a: EditorSnapshot, b: EditorSnapshot) {
  return JSON.stringify(a) === JSON.stringify(b)
}

function restoreSnapshot(snapshot: EditorSnapshot, emitSource: boolean) {
  documentModel.value = snapshot.document ? clone(snapshot.document) : null
  positions.value = clone(snapshot.positions)
  stylesByTarget.value = clone(snapshot.styles)
  lastEmittedSource = snapshot.source
  validationMessage.value = ''
  if (!documentModel.value) {
    selectedId.value = null
    selectedIds.value = new Set()
  } else {
    const primary = selectedId.value && documentModel.value.elements.some((item) => item.id === selectedId.value)
      ? selectedId.value
      : documentModel.value.elements.find((item) => item.editable && !isEdgeKind(item.kind) && !['raw', 'direction', 'style'].includes(item.kind))?.id ?? null
    setSelection(primary ? [primary] : [])
  }
  if (emitSource) emit('update:source', snapshot.source)
}

function applyMutation(mutator: (model: MermaidVisualDocument) => void, force = false) {
  if (!documentModel.value || (editorDisabled.value && !force)) return false
  const before = captureSnapshot()
  if (!before) return false
  mutator(documentModel.value)
  return commitDocumentMutation(before)
}

function commitDocumentMutation(before: EditorSnapshot) {
  if (!documentModel.value) return false
  const source = generateMermaidSource(documentModel.value)
  if (!isMermaidEditorValueSafe(source)) {
    restoreSnapshot(before, false)
    validationMessage.value = '安全に扱えない記法が含まれるため、変更を適用していません。'
    return false
  }
  const after = captureSnapshot(source)
  if (snapshotsEqual(before, after)) return false
  undoStack.value.push(before)
  redoStack.value = []
  lastEmittedSource = source
  validationMessage.value = ''
  if (source !== before.source) emit('update:source', source)
  return true
}

function recordPositionMutation(before: EditorSnapshot) {
  const after = captureSnapshot()
  if (!after || snapshotsEqual(before, after)) return
  undoStack.value.push(before)
  redoStack.value = []
}

function setSelection(ids: string[], primary = ids[ids.length - 1] ?? null) {
  const available = new Set(documentModel.value?.elements.map((item) => item.id) ?? [])
  const next = ids.filter((id) => available.has(id))
  selectedIds.value = new Set(next)
  selectedId.value = primary && next.includes(primary) ? primary : next[next.length - 1] ?? null
  if (next.length) propertiesOpen.value = true
}

function selectElement(item: MermaidVisualElement) {
  setSelection([item.id])
}

function selectElementById(id: string) {
  if (!documentModel.value?.elements.some((item) => item.id === id)) return
  setSelection([id])
}

function clearSelection() {
  setSelection([])
  editingId.value = null
  mode.value = 'select'
  connectionSourceId.value = null
  clearConnectionDrag()
}

function handleCanvasPointerDown(event: PointerEvent) {
  const target = event.target as HTMLElement | null
  if (!target || target === event.currentTarget || target.classList.contains('mermaid-visual-editor-grid-pattern') || target.classList.contains('mermaid-visual-editor-stage')) {
    clearSelection()
  }
}

function handleNodeClick(event: MouseEvent, item: MermaidVisualElement) {
  if (mode.value === 'connect') {
    createConnection(item)
    return
  }
  if (suppressNodeClick) {
    suppressNodeClick = false
    return
  }
  if (event.ctrlKey || event.metaKey) {
    const next = new Set(selectedIds.value)
    if (next.has(item.id)) next.delete(item.id)
    else next.add(item.id)
    setSelection([...next], item.id)
    return
  }
  selectElement(item)
}

function activateSelectMode() {
  mode.value = 'select'
  connectionSourceId.value = null
  clearConnectionDrag()
}

function toggleConnectMode() {
  if (editorDisabled.value) return
  mode.value = mode.value === 'connect' ? 'select' : 'connect'
  connectionSourceId.value = null
  clearConnectionDrag()
}

function connectionKind(type: MermaidDiagramType | null) {
  if (type === 'sequence') return 'message'
  if (type === 'state') return 'transition'
  if (type === 'er' || type === 'class') return 'relation'
  if (type === 'flowchart' || type === 'block' || type === 'architecture') return 'edge'
  return null
}

function canConnectItem(item: MermaidVisualElement) {
  const type = diagramType.value
  if (!item.editable || !type) return false
  if (type === 'sequence') return item.kind === 'participant'
  if (type === 'state') return item.kind === 'state'
  if (type === 'er') return item.kind === 'entity'
  if (type === 'class') return item.kind === 'class'
  if (type === 'flowchart' || type === 'block' || type === 'architecture') return item.kind === 'node'
  return false
}

function createConnection(target: MermaidVisualElement, sourceId = connectionSourceId.value) {
  const model = documentModel.value
  const source = sourceId ? model?.elements.find((item) => item.id === sourceId) : null
  const kind = connectionKind(model?.type ?? null)
  if (!model || !kind || !canConnectItem(target)) return false
  if (!source) {
    connectionSourceId.value = target.id
    selectElement(target)
    return false
  }
  if (source.id === target.id || !canConnectItem(source)) return false
  let createdId: string | null = null
  const changed = applyMutation((current) => {
    const connection = createMermaidElement(current.type, kind, current.elements.length)
    connection.id = uniqueInternalId(`${source.id}-connection`, current.elements)
    const from = identityOf(source)
    const to = identityOf(target)
    connection.fields.from = from
    connection.fields.to = to
    // A new connection references the existing elements by identifier. Do not
    // copy their labels/shapes into the edge, otherwise Mermaid treats the
    // endpoint as a new rectangular node and changes the visual shape.
    if (kind === 'edge') {
      delete connection.fields.fromLabel
      delete connection.fields.fromShape
      delete connection.fields.toLabel
      delete connection.fields.toShape
    }
    if (kind === 'relation' && current.type === 'er') {
      connection.fields.cardinalityFrom = connection.fields.cardinalityFrom || '||'
      connection.fields.cardinalityTo = connection.fields.cardinalityTo || 'o{'
    }
    if (kind === 'relation' && current.type === 'class') connection.fields.arrow = '-->'
    connection.dirty = true
    current.elements.push(connection)
    createdId = connection.id
  })
  if (changed && createdId) setSelection([createdId])
  mode.value = 'select'
  connectionSourceId.value = null
  return changed
}

function startConnectionDrag(event: PointerEvent, item: MermaidVisualElement, side: typeof connectionPointSides[number]) {
  if (editorDisabled.value || event.button !== 0 || !canConnectItem(item)) return
  const point = connectionPoint(item, side)
  connectionSourceId.value = item.id
  selectElement(item)
  connectionDrag.value = { sourceId: item.id, x1: point.x, y1: point.y, x2: point.x, y2: point.y, pointerId: event.pointerId }
  window.addEventListener('pointermove', updateConnectionDrag)
  window.addEventListener('pointerup', handleConnectionPointerUp)
  window.addEventListener('pointercancel', handleConnectionPointerCancel, true)
}

function updateConnectionDrag(event: PointerEvent) {
  const active = connectionDrag.value
  if (!active || active.pointerId !== event.pointerId) return
  const viewport = canvas.value
  const rect = viewport?.getBoundingClientRect()
  if (!viewport || !rect) return
  const x = (event.clientX - rect.left + viewport.scrollLeft) / zoom.value
  const y = (event.clientY - rect.top + viewport.scrollTop) / zoom.value
  connectionDrag.value = { ...active, x2: Math.max(0, Math.min(CANVAS_WIDTH, x)), y2: Math.max(0, Math.min(CANVAS_HEIGHT, y)) }
}

function finishConnectionDrag(target: MermaidVisualElement) {
  const active = connectionDrag.value
  if (!active) return
  if (target.id !== active.sourceId && !editorDisabled.value) createConnection(target, active.sourceId)
  clearConnectionDrag()
}

function handleConnectionPointerUp(event: PointerEvent) {
  if (!connectionDrag.value || connectionDrag.value.pointerId !== event.pointerId) return
  clearConnectionDrag()
}

function handleConnectionPointerCancel(event: PointerEvent) {
  if (!connectionDrag.value || connectionDrag.value.pointerId !== event.pointerId) return
  clearConnectionDrag()
}

function cancelConnectionDrag() {
  clearConnectionDrag()
}

function clearConnectionDrag() {
  connectionDrag.value = null
  connectionSourceId.value = null
  window.removeEventListener('pointermove', updateConnectionDrag)
  window.removeEventListener('pointerup', handleConnectionPointerUp)
  window.removeEventListener('pointercancel', handleConnectionPointerCancel, true)
}

function connectionPoint(item: MermaidVisualElement, side: typeof connectionPointSides[number]) {
  const p = positions.value[item.id] ?? { x: 40, y: 40 }
  if (side === 'top') return { x: p.x + NODE_WIDTH / 2, y: p.y }
  if (side === 'right') return { x: p.x + NODE_WIDTH, y: p.y + NODE_HEIGHT / 2 }
  if (side === 'bottom') return { x: p.x + NODE_WIDTH / 2, y: p.y + NODE_HEIGHT }
  return { x: p.x, y: p.y + NODE_HEIGHT / 2 }
}

function sequenceInsertionIndex(model: MermaidVisualDocument, kind: string) {
  const isSequence = model.type === 'sequence' && ['message', 'note', 'control'].includes(kind)
  const isFlowGroup = (model.type === 'flowchart' || model.type === 'block' || model.type === 'architecture') && kind !== 'direction'
  if (!isSequence && !isFlowGroup) return model.elements.length
  const selected = model.elements.find((item) => item.id === selectedId.value)
  const blockId = selected?.blockId
  if (!blockId) return model.elements.length
  const endIndex = model.elements.findIndex((item, index) => index > 0
    && (isSequence ? item.kind === 'control' : item.kind === 'group')
    && item.blockRole === 'end'
    && item.blockId === blockId)
  return endIndex >= 0 ? endIndex : model.elements.length
}

function setConnectionDefaults(model: MermaidVisualDocument, item: MermaidVisualElement) {
  const references = model.type === 'sequence'
    ? model.elements.filter((candidate) => candidate.kind === 'participant').map((candidate) => identityOf(candidate))
    : model.type === 'state'
      ? model.elements.filter((candidate) => candidate.kind === 'state').map((candidate) => identityOf(candidate))
      : model.type === 'class'
        ? model.elements.filter((candidate) => candidate.kind === 'class').map((candidate) => identityOf(candidate))
        : model.type === 'er'
          ? model.elements.filter((candidate) => candidate.kind === 'entity').map((candidate) => identityOf(candidate))
          : model.elements.filter((candidate) => candidate.kind === 'node').map((candidate) => identityOf(candidate))
  if (!references.length) return
  if (item.kind === 'note') {
    item.fields.over = references[0]
    return
  }
  item.fields.from = references[0]
  item.fields.to = references[1] ?? references[0]
  if (item.kind === 'edge') {
    delete item.fields.fromLabel
    delete item.fields.fromShape
    delete item.fields.toLabel
    delete item.fields.toShape
  }
}

function addElement(kind: string) {
  if (!documentModel.value || editorDisabled.value) return
  const sourceKind = kind === 'control-alt' || kind === 'control-loop' ? 'control' : kind
  const addedIds: string[] = []
  const changed = applyMutation((model) => {
    const item = createMermaidElement(model.type, sourceKind, model.elements.length)
    item.id = uniqueInternalId(`mermaid-element-new-${sourceKind}`, model.elements)
    item.dirty = true
    if (kind === 'control-alt') item.fields.command = 'alt'
    if (kind === 'control-loop') item.fields.command = 'loop'
    if (sourceKind === 'group') item.fields.autoClose = ''
    if (sourceKind === 'node' && model.type === 'flowchart') item.fields.id = uniqueIdentifier(item.fields.id || 'N', collectIdentifiers(model))
    if (sourceKind === 'participant') item.fields.name = uniqueIdentifier(item.fields.name || 'P', collectIdentifiers(model))
    if (sourceKind === 'class') item.fields.name = uniqueIdentifier(item.fields.name || 'Class', collectIdentifiers(model))
    if (sourceKind === 'entity') item.fields.name = uniqueIdentifier(item.fields.name || 'ENTITY', collectIdentifiers(model))
    if (sourceKind === 'relation' && model.type === 'class') item.fields.arrow = '-->'
    if (sourceKind === 'message' || sourceKind === 'note' || sourceKind === 'edge' || sourceKind === 'relation' || sourceKind === 'transition') {
      setConnectionDefaults(model, item)
    }
    const insertionIndex = sequenceInsertionIndex(model, sourceKind)
    const selected = model.elements.find((candidate) => candidate.id === selectedId.value)
    const enclosingBlockId = (model.type === 'sequence' || model.type === 'flowchart' || model.type === 'block' || model.type === 'architecture') && selected?.blockId
      ? selected.blockId
      : undefined
    if (sourceKind === 'message' || sourceKind === 'note' || (sourceKind !== 'control' && sourceKind !== 'group' && enclosingBlockId)) {
      item.blockId = enclosingBlockId
      item.blockRole = enclosingBlockId ? 'member' : undefined
    }
    if (sourceKind === 'group') {
      item.blockId = item.id
      item.blockRole = 'start'
    }
    if (sourceKind === 'control' && (item.fields.command === 'alt' || item.fields.command === 'loop')) {
      item.blockId = item.id
      item.blockRole = 'start'
    }
    model.elements.splice(insertionIndex, 0, item)
    positions.value[item.id] = positionForItem(item, insertionIndex)
    addedIds.push(item.id)
    if (sourceKind === 'control' && (item.fields.command === 'alt' || item.fields.command === 'loop')) {
      const end = createMermaidElement(model.type, 'control', model.elements.length)
      end.id = uniqueInternalId(`${item.id}-end`, model.elements)
      end.fields.command = 'end'
      end.fields.label = ''
      end.blockId = item.id
      end.blockRole = 'end'
      end.dirty = true
      model.elements.splice(insertionIndex + 1, 0, end)
    }
    if (sourceKind === 'group') {
      const end = createMermaidElement(model.type, 'group', model.elements.length)
      end.id = uniqueInternalId(`${item.id}-end`, model.elements)
      end.fields = { groupEnd: 'true', title: '', content: '' }
      end.blockId = item.id
      end.blockRole = 'end'
      end.dirty = true
      model.elements.splice(insertionIndex + 1, 0, end)
    }
  })
  if (changed) setSelection(addedIds)
}

function autoArrange() {
  if (!documentModel.value || editorDisabled.value) return
  const before = captureSnapshot()
  if (!before) return
  const items = drawableElements.value
  const next = { ...positions.value }
  if (documentModel.value.type === 'sequence') {
    items.filter((item) => item.kind === 'participant').forEach((item, index) => {
      next[item.id] = { x: 42 + index * 210, y: 28 }
    })
    items.filter((item) => item.kind !== 'participant').forEach((item, index) => {
      next[item.id] = { x: 48 + (index % 3) * 220, y: 112 + Math.floor(index / 3) * 82 }
    })
  } else {
    items.forEach((item, index) => { next[item.id] = positionForItem(item, index) })
  }
  positions.value = next
  recordPositionMutation(before)
}

function alignSelected(axis: 'horizontal' | 'vertical') {
  if (!documentModel.value || editorDisabled.value || selectedIds.value.size < 2) return
  const items = [...selectedIds.value]
    .map((id) => positions.value[id])
    .filter((position): position is Position => Boolean(position))
  if (items.length < 2) return
  const before = captureSnapshot()
  const anchor = items[0]
  for (const id of selectedIds.value) {
    const position = positions.value[id]
    if (!position) continue
    positions.value[id] = axis === 'horizontal'
      ? { ...position, y: anchor.y }
      : { ...position, x: anchor.x }
  }
  recordPositionMutation(before)
}

function sequenceBlockStyle(block: SequenceBlockFrame) {
  return {
    left: `${block.x}px`,
    top: `${block.y}px`,
    width: `${block.width}px`,
    height: `${block.height}px`,
  }
}

function sequenceMessageScope(model: MermaidVisualDocument, target: MermaidVisualElement) {
  const stack: Array<{ id: string; branch: number }> = []
  const blockStarts = new Set(['alt', 'opt', 'loop', 'par', 'critical', 'break'])
  for (const item of model.elements) {
    if (item.id === target.id) {
      return stack.map((frame) => `${frame.id}:${frame.branch}`).join('/')
    }
    if (item.kind !== 'control') continue
    const command = (item.fields.command ?? '').trim().toLowerCase()
    if (blockStarts.has(command)) {
      stack.push({ id: item.blockId ?? item.id, branch: 0 })
    } else if (command === 'else' || command === 'and') {
      const current = stack[stack.length - 1]
      if (current) current.branch += 1
    } else if (command === 'end') {
      stack.pop()
    }
  }
  return ''
}

function sameMessageScope(model: MermaidVisualDocument, a: MermaidVisualElement, b: MermaidVisualElement) {
  return sequenceMessageScope(model, a) === sequenceMessageScope(model, b)
}

function startMessageDrag(event: PointerEvent, id: string) {
  const model = documentModel.value
  if (editorDisabled.value || event.button !== 0 || model?.type !== 'sequence') return
  const item = model.elements.find((candidate) => candidate.id === id && candidate.kind === 'message')
  if (!item) return
  endMessageDrag(true)
  const scopedMessages = model.elements.filter((candidate) => candidate.kind === 'message' && sameMessageScope(model, candidate, item))
  const initialIndex = scopedMessages.findIndex((candidate) => candidate.id === item.id)
  if (initialIndex < 0) return
  const before = captureSnapshot()
  messageDrag = {
    pointerId: event.pointerId,
    id,
    startY: event.clientY,
    scopeIds: scopedMessages.map((candidate) => candidate.id),
    initialIndex,
    before,
    changed: false,
  }
  selectElementById(id)
  window.addEventListener('pointermove', onMessageDrag)
  window.addEventListener('pointerup', handleMessageDragPointerUp, true)
  window.addEventListener('pointercancel', handleMessageDragPointerCancel, true)
}

function onMessageDrag(event: PointerEvent) {
  const active = messageDrag
  const model = documentModel.value
  if (!active || active.pointerId !== event.pointerId || !model || model.type !== 'sequence') return
  if (editorDisabled.value || (event.buttons & 1) === 0) {
    endMessageDrag(true)
    return
  }
  const currentScopeMessages = model.elements.filter((item) => item.kind === 'message' && active.scopeIds.includes(item.id))
  if (currentScopeMessages.length !== active.scopeIds.length) return
  const offset = Math.round((event.clientY - active.startY) / (58 * zoom.value))
  const targetScopeIndex = Math.max(0, Math.min(active.scopeIds.length - 1, active.initialIndex + offset))
  const desiredIds = [...active.scopeIds]
  const [movingId] = desiredIds.splice(active.initialIndex, 1)
  desiredIds.splice(targetScopeIndex, 0, movingId)
  const currentIds = currentScopeMessages.map((item) => item.id)
  if (currentIds.every((item, index) => item === desiredIds[index])) return
  const elementsById = new Map(currentScopeMessages.map((item) => [item.id, item]))
  const scopeIds = new Set(active.scopeIds)
  let nextScopeIndex = 0
  model.elements = model.elements.map((item) => {
    if (!scopeIds.has(item.id)) return item
    const next = elementsById.get(desiredIds[nextScopeIndex])
    nextScopeIndex += 1
    return next ?? item
  })
  active.changed = true
}

function endMessageDrag(commitHistory = true) {
  if (!messageDrag) return
  const current = messageDrag
  messageDrag = null
  window.removeEventListener('pointermove', onMessageDrag)
  window.removeEventListener('pointerup', handleMessageDragPointerUp, true)
  window.removeEventListener('pointercancel', handleMessageDragPointerCancel, true)
  if (commitHistory && current.changed) commitDocumentMutation(current.before)
}

function handleMessageDragPointerUp() {
  endMessageDrag(true)
}

function handleMessageDragPointerCancel() {
  endMessageDrag(true)
}

function isIdentifierField(key: string) {
  return ['id', 'name', 'from', 'to', 'over', 'owner', 'target'].includes(key)
}

function validateFieldValue(key: string, value: string) {
  if (!isIdentifierField(key)) return true
  const normalized = value.trim()
  if (!normalized || /[\r\n\u0000]/.test(normalized)) return false
  if (key === 'id' || key === 'name') return /^[A-Za-z_][\w-]*$/.test(normalized)
  return normalized !== ':' && normalized.length <= 200
}

function syncReferences(model: MermaidVisualDocument, item: MermaidVisualElement, oldIdentity: string, newIdentity: string) {
  if (!oldIdentity || oldIdentity === newIdentity) return
  for (const candidate of model.elements) {
    if (candidate.id === item.id) continue
    for (const key of referenceFields) {
      const currentValue = candidate.fields[key]
      const nextValue = key === 'over' && currentValue
        ? noteReferenceList(currentValue).map((reference) => reference === oldIdentity ? newIdentity : reference).join(',')
        : currentValue === oldIdentity ? newIdentity : currentValue
      if (nextValue !== undefined && nextValue !== currentValue) {
        candidate.fields[key] = nextValue
        candidate.dirty = true
      }
    }
  }
}

function renameStyleTarget(oldTarget: string, newTarget: string) {
  if (!oldTarget || oldTarget === newTarget) return
  const style = stylesByTarget.value[oldTarget]
  if (!style) return
  const next = { ...stylesByTarget.value, [newTarget]: style }
  delete next[oldTarget]
  stylesByTarget.value = next
}

function syncFlowEndpointFields(model: MermaidVisualDocument, item: MermaidVisualElement) {
  if (model.type !== 'flowchart' || item.kind !== 'node') return
  const id = item.fields.id ?? ''
  for (const candidate of model.elements) {
    if (candidate.kind !== 'edge') continue
    if (candidate.fields.from === id && ('fromLabel' in candidate.fields || 'fromShape' in candidate.fields)) {
      candidate.fields.fromLabel = displayLabel(item)
      candidate.fields.fromShape = item.fields.shape ?? 'rect'
      candidate.dirty = true
    }
    if (candidate.fields.to === id && ('toLabel' in candidate.fields || 'toShape' in candidate.fields)) {
      candidate.fields.toLabel = displayLabel(item)
      candidate.fields.toShape = item.fields.shape ?? 'rect'
      candidate.dirty = true
    }
  }
}

function updateField(key: string, value: string, force = false) {
  const item = selectedElement.value
  if (!item || !item.editable || (!force && editorDisabled.value)) return
  const nextValue = value.replace(/\u0000/g, '')
  const normalizedValue = ['id', 'name'].includes(key) ? nextValue.trim() : nextValue
  if (!validateFieldValue(key, normalizedValue)) {
    validationMessage.value = '識別子には改行や未対応の文字を使用できません。'
    return
  }
  if (['id', 'name'].includes(key) && ['node', 'participant', 'class', 'entity', 'state'].includes(item.kind)) {
    const duplicate = documentModel.value?.elements.some((candidate) => candidate.id !== item.id
      && candidate.kind === item.kind
      && candidate.fields[key] === normalizedValue)
    if (duplicate) {
      validationMessage.value = '同じ種類の要素で同じ識別子は使用できません。'
      return
    }
  }
  applyMutation((model) => {
    const current = model.elements.find((candidate) => candidate.id === item.id)
    if (!current) return
    const oldIdentity = identityOf(current)
    current.fields[key] = normalizedValue
    current.dirty = true
    const newIdentity = identityOf(current)
    syncReferences(model, current, oldIdentity, newIdentity)
    if (current.kind === 'node' || current.kind === 'class' || current.kind === 'entity' || current.kind === 'state') {
      renameStyleTarget(oldIdentity, newIdentity)
    }
    syncFlowEndpointFields(model, current)
  }, force)
}

function updateStyle(key: keyof StyleValue, value: string) {
  const item = selectedElement.value
  if (!item || !supportsStyles.value || item.kind !== 'node' || editorDisabled.value || !/^#[0-9a-f]{6}$/i.test(value)) return
  const target = styleTarget(item)
  if (!target) return
  applyMutation((model) => {
    let style = model.elements.find((candidate) => candidate.kind === 'style' && candidate.fields.target === target)
    if (!style) {
      style = createMermaidElement(model.type, 'style', model.elements.length)
      style.id = uniqueInternalId(`style-${target}`, model.elements)
      style.fields.target = target
      style.fields.fill = styleFor(item).fill
      style.fields.stroke = styleFor(item).stroke
      style.fields.text = styleFor(item).text
      model.elements.push(style)
    }
    style.fields[key] = value
    style.dirty = true
    stylesByTarget.value = { ...stylesByTarget.value, [target]: { ...styleFor(item), [key]: value } }
  })
}

function expandBlockSelection(model: MermaidVisualDocument, ids: Set<string>) {
  const expanded = new Set(ids)
  const isSequenceBlock = model.type === 'sequence'
  const isFlowBlock = model.type === 'flowchart' || model.type === 'block' || model.type === 'architecture'
  if (!isSequenceBlock && !isFlowBlock) return expanded
  for (const item of model.elements) {
    const isBoundary = isSequenceBlock
      ? item.kind === 'control' && ['start', 'end'].includes(item.blockRole ?? '')
      : item.kind === 'group' && ['start', 'end'].includes(item.blockRole ?? '')
    if (!ids.has(item.id) || !isBoundary || !item.blockId) continue
    const startIndex = model.elements.findIndex((candidate) => candidate.id === item.blockId)
    if (startIndex < 0) continue
    const endIndex = model.elements.findIndex((candidate, index) => index >= startIndex
      && (isSequenceBlock ? candidate.kind === 'control' : candidate.kind === 'group')
      && candidate.blockRole === 'end'
      && candidate.blockId === item.blockId)
    const lastIndex = endIndex >= 0 ? endIndex : model.elements.length - 1
    for (const member of model.elements.slice(startIndex, lastIndex + 1)) expanded.add(member.id)
  }
  return expanded
}

function noteReferenceList(value: string) {
  return value.split(',').map((reference) => reference.trim()).filter(Boolean)
}

function promoteOrphanedDerivedFlowNodes(model: MermaidVisualDocument) {
  if (model.type !== 'flowchart' && model.type !== 'block' && model.type !== 'architecture') return
  const referenced = new Set<string>()
  for (const item of model.elements) {
    if (!isEdgeKind(item.kind)) continue
    for (const key of ['from', 'to']) {
      const reference = item.fields[key]
      if (reference) referenced.add(reference)
    }
  }
  for (const item of model.elements) {
    if (item.kind !== 'node' || !item.derived || referenced.has(item.fields.id ?? '')) continue
    item.derived = false
    item.raw = ''
    item.dirty = true
  }
}

function removeElement() {
  const model = documentModel.value
  const ids = model ? expandBlockSelection(model, new Set(selectedIds.value)) : new Set<string>()
  if (!model || !ids.size || editorDisabled.value) return
  const explicitlySelectedItems = model.elements.filter((item) => selectedIds.value.has(item.id))
  if (explicitlySelectedItems.some((item) => !item.editable)) return
  const items = model.elements.filter((item) => ids.has(item.id))
  if (!items.length) return
  const removedReferences = new Set(items.map((item) => identityOf(item)))
  const changed = applyMutation((current) => {
    current.elements = current.elements.filter((candidate) => {
      if (ids.has(candidate.id)) return false
      if (candidate.kind === 'style' && removedReferences.has(candidate.fields.target ?? '')) return false
      if (isEdgeKind(candidate.kind)) return !referenceFields.some((key) => removedReferences.has(candidate.fields[key] ?? ''))
      if (candidate.kind === 'note' && candidate.fields.over) {
        const references = noteReferenceList(candidate.fields.over)
        const remaining = references.filter((reference) => !removedReferences.has(reference))
        if (remaining.length !== references.length) {
          if (!remaining.length) return false
          candidate.fields.over = remaining.join(',')
          candidate.dirty = true
        }
      }
      return true
    })
    promoteOrphanedDerivedFlowNodes(current)
    for (const target of removedReferences) delete stylesByTarget.value[target]
  })
  if (changed) setSelection([])
}

function collectIdentifiers(model: MermaidVisualDocument) {
  const used = new Set<string>()
  for (const item of model.elements) {
    for (const key of ['id', 'name', 'target']) {
      const value = item.fields[key]
      if (value) used.add(value)
    }
  }
  return used
}

function uniqueIdentifier(base: string, used: Set<string>) {
  const safeBase = /^[A-Za-z_][\w-]*$/.test(base) ? base : 'N'
  let candidate = safeBase
  let suffix = 2
  while (used.has(candidate)) candidate = `${safeBase}${suffix++}`
  used.add(candidate)
  return candidate
}

function uniqueInternalId(base: string, elements: MermaidVisualElement[]) {
  const used = new Set(elements.map((item) => item.id))
  let candidate = base
  let suffix = 2
  while (used.has(candidate)) candidate = `${base}-${suffix++}`
  return candidate
}

function isBlockBoundary(model: MermaidVisualDocument, item: MermaidVisualElement) {
  if (model.type === 'sequence') return item.kind === 'control' && ['start', 'end'].includes(item.blockRole ?? '')
  if (model.type === 'flowchart' || model.type === 'block' || model.type === 'architecture') {
    return item.kind === 'group' && ['start', 'end'].includes(item.blockRole ?? '')
  }
  return false
}

function selectedClipboardItems(model: MermaidVisualDocument) {
  const selected = new Set(selectedIds.value)
  const sourceIds = expandBlockSelection(model, selected)
  const includesBlockBoundary = model.elements.some((item) => selected.has(item.id) && isBlockBoundary(model, item))
  return model.elements
    .filter((item) => sourceIds.has(item.id) && (item.editable || (includesBlockBoundary && Boolean(item.blockId))))
    .map((item): ClipboardItem => ({
      kind: item.kind,
      fields: clone(item.fields),
      raw: item.raw,
      editable: item.editable,
      sourceId: item.id,
      derived: item.derived,
      blockId: item.blockId,
      blockRole: item.blockRole,
    }))
}

function copyElements(items: ClipboardItem[], model: MermaidVisualDocument) {
  const usedIdentifiers = collectIdentifiers(model)
  const mapping = new Map<string, string>()
  const copies: MermaidVisualElement[] = []
  for (const item of items) {
    const copy = createMermaidElement(model.type, item.kind, model.elements.length + copies.length)
    copy.id = uniqueInternalId(`${item.kind}-copy`, [...model.elements, ...copies])
    copy.fields = { ...item.fields }
    copy.raw = item.editable ? '' : item.raw
    copy.editable = item.editable
    copy.derived = item.editable ? false : item.derived
    copy.dirty = item.editable
    copy.blockRole = item.blockRole
    copy.blockId = item.blockId
    const sourceIdentity = item.fields.id || item.fields.name
    if (item.fields.id) {
      const next = uniqueIdentifier(item.fields.id, usedIdentifiers)
      mapping.set(item.fields.id, next)
      copy.fields.id = next
    } else if (item.fields.name) {
      const next = uniqueIdentifier(item.fields.name, usedIdentifiers)
      mapping.set(item.fields.name, next)
      copy.fields.name = next
    }
    if (sourceIdentity && !mapping.has(sourceIdentity)) mapping.set(sourceIdentity, sourceIdentity)
    copies.push(copy)
  }
  const blockMapping = new Map<string, string>()
  items.forEach((item, index) => {
    if (item.blockRole === 'start' && item.blockId) blockMapping.set(item.blockId, copies[index].id)
  })
  for (const copy of copies) {
    for (const key of referenceFields) {
      const value = copy.fields[key]
      if (!value) continue
      if (key === 'over') {
        copy.fields[key] = noteReferenceList(value).map((reference) => mapping.get(reference) ?? reference).join(',')
      } else if (mapping.has(value)) {
        copy.fields[key] = mapping.get(value) ?? value
      }
    }
    if (copy.blockId) copy.blockId = blockMapping.get(copy.blockId)
  }
  return copies
}

function duplicateElement() {
  const model = documentModel.value
  if (!model || editorDisabled.value) return
  const items = selectedClipboardItems(model)
  if (!items.length) return
  const copies = copyElements(items, model)
  const copyIds = copies.filter((item) => item.editable).map((item) => item.id)
  const origins = items.map((item) => positions.value[item.sourceId ?? ''] ?? { x: 70, y: 70 })
  const changed = applyMutation((current) => {
    current.elements.push(...copies)
    copies.forEach((copy, index) => {
      positions.value[copy.id] = { x: origins[index].x + 24, y: origins[index].y + 24 }
    })
  })
  if (changed) setSelection(copyIds)
}

function copySelected() {
  const model = documentModel.value
  if (!model) return
  clipboard.value = selectedClipboardItems(model)
}

function pasteClipboard() {
  const model = documentModel.value
  if (!model || !clipboard.value.length || editorDisabled.value) return
  const copies = copyElements(clipboard.value, model)
  const changed = applyMutation((current) => {
    current.elements.push(...copies)
    copies.forEach((copy, index) => { positions.value[copy.id] = { x: 70 + index * 24, y: 70 + index * 24 } })
  })
  if (changed) setSelection(copies.filter((item) => item.editable).map((item) => item.id))
}

function startDrag(event: PointerEvent, item: MermaidVisualElement) {
  if (editorDisabled.value || !item.editable || event.button !== 0 || mode.value === 'connect' || connectionDrag.value || editingId.value) return
  endDrag(true)
  suppressNodeClick = false
  const isMultiSelectGesture = event.ctrlKey || event.metaKey
  let pendingSelectionId: string | null = null
  if (isMultiSelectGesture && !selectedIds.value.has(item.id)) {
    // Wait until the pointer actually moves before adding the item. A plain
    // Ctrl/Cmd-click must be toggled exactly once by the click handler.
    pendingSelectionId = item.id
  } else if (!isMultiSelectGesture && !selectedIds.value.has(item.id)) {
    selectElement(item)
  }
  const origins: Record<string, Position> = {}
  for (const id of selectedIds.value) {
    if (positions.value[id]) origins[id] = { ...positions.value[id] }
  }
  if (pendingSelectionId && positions.value[pendingSelectionId]) {
    origins[pendingSelectionId] = { ...positions.value[pendingSelectionId] }
  }
  const before = captureSnapshot()
  if (!before || !Object.keys(origins).length) return
  drag = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, origins, before, changed: false, pendingSelectionId }
  window.addEventListener('pointermove', onDrag)
  window.addEventListener('pointerup', handleDragPointerUp, true)
  window.addEventListener('pointercancel', handleDragPointerCancel, true)
}

function onDrag(event: PointerEvent) {
  if (!drag || drag.pointerId !== event.pointerId) return
  if (editorDisabled.value || (event.buttons & 1) === 0) {
    endDrag(true)
    return
  }
  if (drag.pendingSelectionId) {
    const pendingSelectionId = drag.pendingSelectionId
    drag.pendingSelectionId = null
    setSelection([...selectedIds.value, pendingSelectionId], pendingSelectionId)
  }
  const dx = (event.clientX - drag.startX) / zoom.value
  const dy = (event.clientY - drag.startY) / zoom.value
  for (const [id, origin] of Object.entries(drag.origins)) {
    positions.value[id] = {
      x: Math.max(8, Math.min(CANVAS_WIDTH - NODE_WIDTH - 8, origin.x + dx)),
      y: Math.max(8, Math.min(CANVAS_HEIGHT - NODE_HEIGHT - 8, origin.y + dy)),
    }
  }
  drag.changed = true
}

function endDrag(commitHistory = true) {
  if (!drag) return
  const current = drag
  drag = null
  window.removeEventListener('pointermove', onDrag)
  window.removeEventListener('pointerup', handleDragPointerUp, true)
  window.removeEventListener('pointercancel', handleDragPointerCancel, true)
  if (commitHistory && current.changed) {
    suppressNodeClick = true
    recordPositionMutation(current.before)
  }
}

function handleDragPointerUp() {
  endDrag(true)
}

function handleDragPointerCancel() {
  endDrag(true)
}

function cancelPointerInteraction() {
  endDrag(true)
  endMessageDrag(true)
  cancelConnectionDrag()
}

function beginInlineEdit(item: MermaidVisualElement) {
  if (!item.editable || editorDisabled.value) return
  selectElement(item)
  editingId.value = item.id
  inlineComposing.value = false
  void nextTick(() => {
    const input = editorRoot.value?.querySelector('.mermaid-visual-editor-inline-input') as HTMLInputElement | null
    input?.focus()
    input?.select()
  })
}

function inlineField(item: MermaidVisualElement) {
  if (item.kind === 'participant') return 'alias'
  if (item.fields.label !== undefined) return 'label'
  if (item.fields.message !== undefined) return 'message'
  if (item.fields.name !== undefined) return 'name'
  return 'content'
}

function commitInlineValue(item: MermaidVisualElement, value: string, force = false) {
  if (editingId.value !== item.id) return
  updateField(inlineField(item), value, force)
  editingId.value = null
  inlineComposing.value = false
}

function finishInlineEdit(item: MermaidVisualElement, event: Event) {
  if (inlineComposing.value || editingId.value !== item.id) return
  const value = (event.target as HTMLInputElement | null)?.value
  if (value !== undefined) commitInlineValue(item, value)
}

function finishInlineComposition(item: MermaidVisualElement, event: Event) {
  inlineComposing.value = false
  const value = (event.target as HTMLInputElement | null)?.value
  if (value !== undefined) commitInlineValue(item, value)
}

function cancelInlineEdit() {
  editingId.value = null
  inlineComposing.value = false
}

function isTextEditingTarget(target: EventTarget | null) {
  const element = target instanceof HTMLElement ? target : null
  if (!element) return false
  if (element.isContentEditable || element.closest('[contenteditable="true"]')) return true
  return element.tagName === 'INPUT' || element.tagName === 'TEXTAREA' || element.tagName === 'SELECT'
}

function handleKeydown(event: KeyboardEvent) {
  if (editorDisabled.value || event.isComposing || event.keyCode === 229 || isTextEditingTarget(event.target)) return
  if (editingId.value) return
  const modifier = event.ctrlKey || event.metaKey
  if ((event.key === 'Delete' || event.key === 'Backspace') && selectedIds.value.size) {
    event.preventDefault()
    removeElement()
    return
  }
  if (!modifier) return
  const key = event.key.toLowerCase()
  if (key === 'c') { event.preventDefault(); copySelected() }
  else if (key === 'v') { event.preventDefault(); pasteClipboard() }
  else if (key === 'd') { event.preventDefault(); duplicateElement() }
  else if (key === 'z') { event.preventDefault(); undo() }
  else if (key === 'y') { event.preventDefault(); redo() }
}

function undo() {
  if (editorDisabled.value) return
  const target = undoStack.value.pop()
  if (!target) return
  const current = captureSnapshot()
  if (current) redoStack.value.push(current)
  restoreSnapshot(target, true)
}

function redo() {
  if (editorDisabled.value) return
  const target = redoStack.value.pop()
  if (!target) return
  const current = captureSnapshot()
  if (current) undoStack.value.push(current)
  restoreSnapshot(target, true)
}

function setZoom(value: number) {
  zoom.value = Math.min(2, Math.max(0.5, Math.round(value * 10) / 10))
}

function styleFor(item: MermaidVisualElement): StyleValue {
  const target = styleTarget(item)
  return stylesByTarget.value[target] ?? defaultStyle
}

function elementStyle(item: MermaidVisualElement) {
  const p = positions.value[item.id] ?? { x: 40, y: 40 }
  const style = item.kind === 'participant' || item.kind === 'node' || item.kind === 'state' || item.kind === 'class' || item.kind === 'entity'
    ? styleFor(item)
    : defaultStyle
  const width = item.kind === 'control' ? 440 : item.kind === 'note' ? 190 : NODE_WIDTH
  return {
    left: `${p.x}px`,
    top: `${p.y}px`,
    width: `${width}px`,
    backgroundColor: style.fill,
    borderColor: style.stroke,
    color: style.text,
  }
}

function elementClasses(item: MermaidVisualElement) {
  return {
    [`is-${item.kind}`]: true,
    'is-selected': selectedIds.value.has(item.id),
    'is-multi-selected': selectedIds.value.size > 1 && selectedIds.value.has(item.id),
    'is-connect-target': mode.value === 'connect' || Boolean(connectionDrag.value),
    'is-raw': !item.editable,
  }
}

function displayLabel(item: MermaidVisualElement) {
  if (item.kind === 'participant') return item.fields.alias || item.fields.name || '参加者'
  return item.fields.label || item.fields.message || item.fields.name || item.fields.content || item.fields.id || item.fields.raw || '要素'
}

function shortLabel(label: string) {
  return label.length > 5 ? label.slice(0, 5) : label
}

function toolIcon(kind: string) {
  return ({ participant: '♙', node: '□', message: '→', edge: '↗', note: '▱', control: '◇', relation: '↔', transition: '→', state: '◇', class: '▤', entity: '▭', item: '•', section: '▤' } as Record<string, string>)[kind] ?? '＋'
}

function paletteHint(kind: string) {
  return kind === 'message' || kind === 'edge' || kind === 'relation' || kind === 'transition' ? '接続を追加' : kind === 'control' ? '枠を追加' : 'キャンバスに追加'
}

function renderedEdge(item: MermaidVisualElement, x1: number, y1: number, x2: number, y2: number, label: string): RenderedEdge {
  const arrow = item.fields.arrow ?? '-->'
  return { id: item.id, x1, y1, x2, y2, label, arrow, dashed: arrow.includes('--'), hasArrow: !['---', '==='].includes(arrow) }
}

function arrowPoints(edge: RenderedEdge) {
  const angle = Math.atan2(edge.y2 - edge.y1, edge.x2 - edge.x1)
  const size = 8
  const a = `${edge.x2},${edge.y2}`
  const b = `${edge.x2 - size * Math.cos(angle - Math.PI / 6)},${edge.y2 - size * Math.sin(angle - Math.PI / 6)}`
  const c = `${edge.x2 - size * Math.cos(angle + Math.PI / 6)},${edge.y2 - size * Math.sin(angle + Math.PI / 6)}`
  return `${a} ${b} ${c}`
}

function flushInput() {
  if (!editingId.value || !documentModel.value) return
  const item = documentModel.value.elements.find((candidate) => candidate.id === editingId.value)
  const input = editorRoot.value?.querySelector('.mermaid-visual-editor-inline-input') as HTMLInputElement | null
  if (item && input) commitInlineValue(item, input.value, true)
}

function setInputLocked(locked: boolean) {
  localInputLocked.value = locked
}

defineExpose({ flushInput, setInputLocked })

onMounted(() => {
  window.addEventListener('blur', cancelPointerInteraction)
})

watch(() => props.source, (source) => {
  if (source === lastEmittedSource) return
  if (sourceLoaded) {
    undoStack.value.push(captureSnapshot())
    redoStack.value = []
  }
  loadSource(source, false, true)
}, { immediate: true })

watch(() => props.disabled, (disabled) => {
  if (disabled) cancelPointerInteraction()
}, { flush: 'sync' })

onBeforeUnmount(() => {
  endDrag(false)
  endMessageDrag(false)
  cancelConnectionDrag()
  window.removeEventListener('blur', cancelPointerInteraction)
})
</script>

<style scoped>
.mermaid-visual-editor { display: flex; min-height: 520px; flex-direction: column; gap: 10px; outline: none; }
.mermaid-visual-editor-toolbar { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 8px; padding: 0 8px; overflow-x: auto; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-input); }
.mermaid-visual-editor-tools, .mermaid-visual-editor-zoom { display: flex; align-items: center; gap: 4px; }
.mermaid-visual-editor-toolbar button, .mermaid-visual-editor-zoom button { min-width: 32px; height: 34px; border: 0; border-radius: 6px; background: transparent; color: var(--text-primary); cursor: pointer; font: inherit; font-size: 12px; white-space: nowrap; }
.mermaid-visual-editor-toolbar button span { display: inline-block; margin-left: 3px; }
.mermaid-visual-editor-toolbar button:hover, .mermaid-visual-editor-toolbar button.is-active, .mermaid-visual-editor-zoom button:hover { background: color-mix(in srgb, var(--brand-primary) 12%, transparent); color: var(--brand-primary); }
.mermaid-visual-editor-toolbar button:disabled { opacity: .45; cursor: not-allowed; }
.mermaid-visual-editor-arrange-menu { position: relative; }
.mermaid-visual-editor-arrange-popover { position: absolute; z-index: 5; top: calc(100% + 4px); left: 0; display: grid; min-width: 130px; padding: 4px; border: 1px solid var(--border); border-radius: 7px; background: var(--bg-input); box-shadow: 0 6px 16px rgba(20, 40, 80, .16); }
.mermaid-visual-editor-arrange-popover button { width: 100%; height: 30px; padding: 0 8px; text-align: left; }
.mermaid-visual-editor-tool-icon { display: inline-block; font-size: 18px; }
.mermaid-visual-editor-divider { width: 1px; height: 26px; margin: 0 4px; background: var(--border); }
.mermaid-visual-editor-zoom { margin-left: auto; }
.mermaid-visual-editor-zoom span { min-width: 46px; color: var(--text-secondary); text-align: center; font-size: 12px; }
.mermaid-visual-editor-warning { margin: 0; padding: 7px 10px; border: 1px solid color-mix(in srgb, var(--color-danger, #c0392b) 35%, var(--border)); border-radius: 6px; color: var(--color-danger, #c0392b); font-size: 11px; line-height: 1.5; }
.mermaid-visual-editor-layout { display: grid; min-height: 470px; grid-template-columns: 190px minmax(360px, 1fr) 220px; gap: 10px; }
.mermaid-visual-editor-palette, .mermaid-visual-editor-properties { min-width: 0; padding: 12px; overflow: auto; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-input); }
.mermaid-visual-editor-palette > strong, .mermaid-visual-editor-properties > strong { display: block; margin-bottom: 8px; font-size: 14px; }
.mermaid-visual-editor-palette-item { display: flex; width: 100%; align-items: center; gap: 9px; margin-bottom: 8px; padding: 9px; border: 1px solid var(--border); border-radius: 7px; background: var(--bg-editor); color: var(--text-primary); cursor: pointer; text-align: left; }
.mermaid-visual-editor-palette-item:hover { border-color: var(--brand-primary); }
.mermaid-visual-editor-palette-item:disabled { opacity: .5; cursor: not-allowed; }
.mermaid-visual-editor-palette-icon { display: grid; width: 30px; height: 30px; flex: 0 0 30px; place-items: center; border-radius: 5px; background: color-mix(in srgb, var(--brand-primary) 12%, transparent); color: var(--brand-primary); font-size: 19px; }
.mermaid-visual-editor-palette-item b, .mermaid-visual-editor-palette-item small { display: block; }
.mermaid-visual-editor-palette-item small { margin-top: 2px; color: var(--text-secondary); font-size: 10px; }
.mermaid-visual-editor-hint, .mermaid-visual-editor-selection-summary { color: var(--text-secondary); font-size: 11px; line-height: 1.5; }
.mermaid-visual-editor-selection-summary { margin: -4px 0 12px; }
.mermaid-visual-editor-canvas { min-width: 0; min-height: 470px; overflow: auto; border: 1px solid var(--border); border-radius: 8px; background: var(--bg-editor); }
.mermaid-visual-editor-stage-shell { position: relative; flex: 0 0 auto; }
.mermaid-visual-editor-stage { position: relative; }
.mermaid-visual-editor-grid-pattern { position: absolute; inset: 0; opacity: .55; background-image: radial-gradient(color-mix(in srgb, var(--text-secondary) 23%, transparent) 1px, transparent 1px); background-size: 16px 16px; }
.mermaid-visual-editor-edges { position: absolute; z-index: 0; inset: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.mermaid-visual-editor-edge { pointer-events: all; cursor: pointer; }
.mermaid-visual-editor-edge { touch-action: none; }
.mermaid-visual-editor-edge line { stroke: var(--text-secondary); stroke-width: 1.6; }
.mermaid-visual-editor-edge line.is-dashed { stroke-dasharray: 5 4; }
.mermaid-visual-editor-edge.is-selected line { stroke: var(--brand-primary); stroke-width: 2.7; }
.mermaid-visual-editor-edge polygon { fill: var(--text-secondary); }
.mermaid-visual-editor-edge.is-selected polygon { fill: var(--brand-primary); }
.mermaid-visual-editor-edge text { fill: var(--text-secondary); font-size: 12px; text-anchor: middle; }
.mermaid-visual-editor-lifeline { pointer-events: none; }
.mermaid-visual-editor-lifeline line { stroke: color-mix(in srgb, var(--brand-primary) 55%, var(--border)); stroke-width: 1.2; stroke-dasharray: 4 4; }
.mermaid-visual-editor-sequence-block { position: absolute; z-index: 0; box-sizing: border-box; padding: 7px 10px; overflow: hidden; border: 1px dashed color-mix(in srgb, var(--brand-primary) 62%, var(--border)); border-radius: 8px; background: color-mix(in srgb, var(--brand-primary) 4%, transparent); color: var(--text-secondary); font-size: 11px; pointer-events: none; }
.mermaid-visual-editor-node { position: absolute; z-index: 1; display: flex; min-height: 62px; box-sizing: border-box; align-items: center; justify-content: center; flex-direction: column; gap: 4px; padding: 8px; border: 1px solid color-mix(in srgb, var(--brand-primary) 42%, var(--border)); border-radius: 6px; background: color-mix(in srgb, var(--brand-primary) 9%, var(--bg-input)); color: var(--text-primary); cursor: grab; font: inherit; font-size: 13px; box-shadow: 0 1px 2px rgba(20, 40, 80, .06); }
.mermaid-visual-editor-node:active { cursor: grabbing; }
.mermaid-visual-editor-node.is-selected { border-width: 2px; box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-primary) 15%, transparent); }
.mermaid-visual-editor-node.is-multi-selected { background: color-mix(in srgb, var(--brand-primary) 18%, var(--bg-input)); }
.mermaid-visual-editor-node.is-connect-target { cursor: crosshair; }
.mermaid-visual-editor-node.is-raw { opacity: .6; cursor: default; }
.mermaid-visual-editor-node.is-note { align-items: flex-start; padding: 10px 12px; border-color: #d59b23; background: color-mix(in srgb, #f7c948 18%, var(--bg-input)); text-align: left; }
.mermaid-visual-editor-node.is-control { justify-content: flex-start; align-items: flex-start; border-style: dashed; background: color-mix(in srgb, var(--brand-primary) 7%, var(--bg-input)); text-align: left; }
.mermaid-visual-editor-node-kind { color: var(--text-secondary); font-size: 10px; }
.mermaid-visual-editor-inline-input { width: 100%; box-sizing: border-box; border: 1px solid var(--brand-primary); border-radius: 4px; background: var(--bg-editor); color: var(--text-primary); font: inherit; text-align: center; }
.mermaid-visual-editor-field { display: flex; margin-bottom: 11px; flex-direction: column; gap: 4px; color: var(--text-secondary); font-size: 11px; }
.mermaid-visual-editor-field input, .mermaid-visual-editor-field textarea, .mermaid-visual-editor-field select { box-sizing: border-box; width: 100%; padding: 7px; border: 1px solid var(--border); border-radius: 5px; background: var(--bg-editor); color: var(--text-primary); font: inherit; }
.mermaid-visual-editor-field textarea { resize: vertical; }
.mermaid-visual-editor-style-fields { margin: 16px 0 4px; padding-top: 12px; border-top: 1px solid var(--border); }
.mermaid-visual-editor-style-fields > strong { display: block; margin-bottom: 8px; font-size: 12px; }
.mermaid-visual-editor-style-fields label { display: flex; align-items: center; justify-content: space-between; margin: 7px 0; color: var(--text-secondary); font-size: 11px; }
.mermaid-visual-editor-style-fields input[type='color'] { width: 34px; height: 24px; padding: 2px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg-editor); cursor: pointer; }
.mermaid-visual-editor-property-actions { display: flex; gap: 6px; margin-top: 16px; }
.mermaid-visual-editor-property-actions button { flex: 1; min-height: 32px; border: 1px solid var(--border); border-radius: 5px; background: var(--bg-editor); color: var(--text-primary); cursor: pointer; font: inherit; font-size: 11px; }
.mermaid-visual-editor-property-actions .danger { border-color: var(--color-danger, #c0392b); color: var(--color-danger, #c0392b); }
.mermaid-visual-editor-empty, .mermaid-visual-editor-fallback { margin: auto; color: var(--text-secondary); font-size: 12px; text-align: center; }
.mermaid-visual-editor-connection-preview { stroke: var(--brand-primary); stroke-width: 2; stroke-dasharray: 5 4; pointer-events: none; }
.mermaid-visual-editor-connection-point { position: absolute; z-index: 3; width: 9px; height: 9px; border: 2px solid var(--brand-primary); border-radius: 50%; background: var(--bg-editor); opacity: 0; transition: opacity .12s; touch-action: none; }
.mermaid-visual-editor-node:hover .mermaid-visual-editor-connection-point, .mermaid-visual-editor-node.is-selected .mermaid-visual-editor-connection-point, .mermaid-visual-editor-node.is-connect-target .mermaid-visual-editor-connection-point { opacity: 1; }
.mermaid-visual-editor-connection-point.is-top { top: -6px; left: calc(50% - 6px); cursor: ns-resize; }
.mermaid-visual-editor-connection-point.is-right { top: calc(50% - 6px); right: -6px; cursor: ew-resize; }
.mermaid-visual-editor-connection-point.is-bottom { bottom: -6px; left: calc(50% - 6px); cursor: ns-resize; }
.mermaid-visual-editor-connection-point.is-left { top: calc(50% - 6px); left: -6px; cursor: ew-resize; }
@media (max-width: 1100px) { .mermaid-visual-editor-layout { grid-template-columns: 165px minmax(300px, 1fr); } .mermaid-visual-editor-properties { grid-column: 1 / -1; max-height: 360px; } }
@media (max-width: 720px) { .mermaid-visual-editor-layout { grid-template-columns: 1fr; } .mermaid-visual-editor-palette { display: flex; gap: 6px; overflow-x: auto; } .mermaid-visual-editor-palette > strong, .mermaid-visual-editor-hint { display: none; } .mermaid-visual-editor-palette-item { min-width: 130px; margin: 0; } .mermaid-visual-editor-canvas { min-height: 400px; } }
</style>
