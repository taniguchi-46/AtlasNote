<template>
  <section
    ref="workspaceRoot"
    :class="['ai-workspace', `is-${placement}`]"
    aria-label="AIワークスペース"
  >
    <div class="ai-workspace-editor">
      <slot />
    </div>

    <button
      v-show="isOpen"
      class="ai-workspace-resizer"
      :class="{ 'is-resizing': activeResize !== null }"
      type="button"
      role="separator"
      :aria-label="resizeAriaLabel"
      :aria-orientation="placement === 'right' ? 'vertical' : 'horizontal'"
      :aria-valuemin="resizeBounds.min"
      :aria-valuemax="resizeBounds.max"
      :aria-valuenow="effectivePanelSize"
      @keydown="handleResizerKeydown"
      @pointerdown="startResize"
      @pointermove="handleResize"
      @pointerup="finishResize"
      @pointercancel="finishResize"
    />

    <aside
      v-show="isOpen"
      id="ai-workspace-panel"
      class="ai-workspace-panel"
      :style="workspacePanelStyle"
      aria-label="AIワークスペース"
      @keydown.esc.stop="closeWorkspace"
    >
      <header class="ai-workspace-header">
        <div class="ai-workspace-header-title">
          <SparklesIcon :size="16" aria-hidden="true" />
          <strong>AI</strong>
        </div>
        <div class="ai-workspace-header-actions">
          <button
            class="ai-workspace-icon-button"
            type="button"
            title="保存済みの履歴と成果物を開く"
            aria-label="保存済みの履歴と成果物を開く"
            :aria-pressed="activeFeature === 'records'"
            @click="showRecords"
          >
            <ArchiveIcon :size="16" aria-hidden="true" />
          </button>
          <button
            class="ai-workspace-icon-button"
            type="button"
            title="AIワークスペースを閉じる"
            aria-label="AIワークスペースを閉じる"
            @click="closeWorkspace"
          >
            <XIcon :size="16" aria-hidden="true" />
          </button>
        </div>
      </header>

      <p class="ai-workspace-context" role="status">
        現在のノート: {{ activeNoteLabel }}
      </p>

      <div class="ai-workspace-content">
        <div v-show="activeFeature === 'summary'" class="ai-workspace-feature" aria-label="AI要約">
          <AISummaryPanel ref="summaryPanel" />
        </div>
        <div v-show="activeFeature === 'librarian'" class="ai-workspace-feature" aria-label="AI司書">
          <AILibrarianPanel ref="librarianPanel" />
        </div>
        <div v-show="activeFeature === 'assistant'" class="ai-workspace-feature" aria-label="AIアシスタント">
          <AIAssistantPanel ref="assistantPanel" external-composer />
        </div>
        <div v-show="activeFeature === 'writing'" class="ai-workspace-feature" aria-label="AIライティング">
          <AIWritingPanel ref="writingPanel" external-composer />
        </div>
        <div v-show="activeFeature === 'records'" class="ai-workspace-feature" aria-label="履歴と成果物">
          <AIRecordsPanel @open-artifact="openArtifact" @open-history="openHistory" @open-summary="openSummary" />
        </div>
      </div>

      <form class="ai-workspace-composer" aria-label="AI入力" @submit.prevent="submitComposer">
        <textarea
          ref="composerTextarea"
          v-model="composerText"
          class="ai-workspace-composer-textarea"
          rows="2"
          :maxlength="composerMaximumLength"
          :placeholder="composerPlaceholder"
          :disabled="!canUseComposer || !isPromptComposerFeature"
          @keydown.ctrl.enter.prevent="submitComposer"
          @keydown.meta.enter.prevent="submitComposer"
        />
        <div class="ai-workspace-composer-toolbar">
          <DropdownMenuRoot>
            <DropdownMenuTrigger as-child>
              <button
                class="ai-workspace-composer-feature-button"
                type="button"
                :title="`${composerFeatureLabel}を選択`"
                :aria-label="`${composerFeatureLabel}を選択`"
                aria-haspopup="menu"
                :disabled="isAnyBusy"
              >
                <component :is="composerFeatureIcon" :size="15" aria-hidden="true" />
                <span>{{ composerFeatureLabel }}</span>
                <ChevronDownIcon :size="14" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuPortal>
              <DropdownMenuContent
                class="ai-workspace-action-menu"
                side="top"
                align="start"
                :side-offset="8"
              >
                <DropdownMenuLabel class="ai-workspace-action-menu-label">AI機能</DropdownMenuLabel>
                <DropdownMenuItem class="ai-workspace-action-menu-item" @select="selectComposerFeature('summary')">
                  <SparklesIcon :size="15" aria-hidden="true" />
                  要約
                </DropdownMenuItem>
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger class="ai-workspace-action-menu-item">
                    <BookOpenIcon :size="15" aria-hidden="true" />
                    AI司書
                    <ChevronRightIcon class="ai-workspace-action-menu-chevron" :size="15" aria-hidden="true" />
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent class="ai-workspace-action-menu" :side-offset="6" :align-offset="-4">
                    <DropdownMenuLabel class="ai-workspace-action-menu-label">AI司書</DropdownMenuLabel>
                    <DropdownMenuItem
                      v-for="operation in librarianOperations"
                      :key="operation.value"
                      class="ai-workspace-action-menu-item"
                      @select="selectLibrarianOperation(operation.value)"
                    >
                      <component :is="operation.icon" :size="15" aria-hidden="true" />
                      {{ operation.label }}
                    </DropdownMenuItem>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
                <DropdownMenuSeparator class="ai-workspace-action-menu-separator" />
                <DropdownMenuItem class="ai-workspace-action-menu-item" @select="selectComposerFeature('assistant')">
                  <MessageSquareIcon :size="15" aria-hidden="true" />
                  質問・壁打ち
                </DropdownMenuItem>
                <DropdownMenuItem class="ai-workspace-action-menu-item" @select="selectComposerFeature('writing')">
                  <PenLineIcon :size="15" aria-hidden="true" />
                  ライティング
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenuRoot>
          <button
            class="ai-workspace-model-button"
            type="button"
            :title="`${modelButtonLabel}の設定を開く`"
            :aria-label="`${modelButtonLabel}の設定を開く`"
            :disabled="isAnyBusy"
            @click="openAISettings"
          >
            <span>{{ modelButtonLabel }}</span>
            <ChevronDownIcon :size="15" aria-hidden="true" />
          </button>
          <span class="ai-workspace-composer-spacer" />
          <button
            class="ai-workspace-composer-icon-button ai-workspace-send-button"
            type="submit"
            :title="sendButtonLabel"
            :aria-label="sendButtonLabel"
            :disabled="!canSubmitComposer"
          >
            <SendIcon :size="17" aria-hidden="true" />
          </button>
        </div>
        <p v-if="composerStatus" class="ai-workspace-composer-status" role="status">
          {{ composerStatus }}
        </p>
      </form>
    </aside>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Component } from 'vue'
import {
  ArchiveIcon,
  BookOpenIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CopyIcon,
  FolderTreeIcon,
  LinkIcon,
  MessageSquareIcon,
  PenLineIcon,
  SparklesIcon,
  SendIcon,
  TagsIcon,
  XIcon,
} from '@lucide/vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from 'reka-ui'
import type { LibrarianOperation } from '../api/ai'
import {
  AI_WORKSPACE_BOTTOM_HEIGHT_MAX,
  AI_WORKSPACE_BOTTOM_HEIGHT_MIN,
  AI_WORKSPACE_RIGHT_WIDTH_MAX,
  AI_WORKSPACE_RIGHT_WIDTH_MIN,
  type AIWorkspacePlacement,
  useSettingsStore,
} from '../stores/useSettingsStore'
import { useAIStore } from '../stores/useAIStore'
import { useAIAssistantStore } from '../stores/useAIAssistantStore'
import { useAILibrarianStore } from '../stores/useAILibrarianStore'
import { useAIWritingStore } from '../stores/useAIWritingStore'
import { useNoteStore } from '../stores/useNoteStore'
import AISummaryPanel from './AISummaryPanel.vue'
import AILibrarianPanel from './AILibrarianPanel.vue'
import AIAssistantPanel from './AIAssistantPanel.vue'
import AIWritingPanel from './AIWritingPanel.vue'
import AIRecordsPanel from './AIRecordsPanel.vue'

type WorkspaceFeature = 'summary' | 'librarian' | 'assistant' | 'writing' | 'records'
type PromptComposerFeature = 'assistant' | 'writing'
type SummaryPanelHandle = { startSummary: () => Promise<void> }
type LibrarianPanelHandle = { startOperation: (operation: LibrarianOperation) => Promise<void> }
type AssistantPanelHandle = {
  openHistory: (id: string) => Promise<boolean>
  submitPrompt: (prompt: string) => Promise<boolean>
}
type WritingPanelHandle = {
  openArtifact: (id: string) => Promise<boolean>
  submitPrompt: (prompt: string) => Promise<boolean>
}
type ResizeBounds = { min: number; max: number }

const AI_WORKSPACE_EDITOR_WIDTH_MIN = 360
const AI_WORKSPACE_EDITOR_HEIGHT_MIN = 240
const AI_WORKSPACE_RIGHT_RESPONSIVE_RATIO = 0.6
const AI_WORKSPACE_BOTTOM_RESPONSIVE_RATIO = 0.6
const librarianOperations: ReadonlyArray<{ value: LibrarianOperation; label: string; icon: Component }> = [
  { value: 'title', label: 'タイトル候補', icon: SparklesIcon },
  { value: 'tags', label: 'タグ候補', icon: TagsIcon },
  { value: 'classification', label: '分類候補', icon: FolderTreeIcon },
  { value: 'related', label: '関連メモ', icon: LinkIcon },
  { value: 'duplicate', label: '重複候補', icon: CopyIcon },
]

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  'update:open': [open: boolean]
  closed: []
}>()

const settingsStore = useSettingsStore()
const aiStore = useAIStore()
const librarianStore = useAILibrarianStore()
const assistantStore = useAIAssistantStore()
const writingStore = useAIWritingStore()
const noteStore = useNoteStore()
const workspaceRoot = ref<HTMLElement | null>(null)
const composerTextarea = ref<HTMLTextAreaElement | null>(null)
const summaryPanel = ref<SummaryPanelHandle | null>(null)
const librarianPanel = ref<LibrarianPanelHandle | null>(null)
const assistantPanel = ref<AssistantPanelHandle | null>(null)
const writingPanel = ref<WritingPanelHandle | null>(null)
const activeFeature = ref<WorkspaceFeature>('assistant')
const selectedLibrarianOperation = ref<LibrarianOperation>('title')
const composerDrafts = ref<Record<PromptComposerFeature, string>>({ assistant: '', writing: '' })
const composerText = computed({
  get: () => {
    const feature = activeFeature.value
    return feature === 'assistant' || feature === 'writing' ? composerDrafts.value[feature] : ''
  },
  set: (value: string) => {
    const feature = activeFeature.value
    if (feature === 'assistant' || feature === 'writing') composerDrafts.value[feature] = value
  },
})
const workspaceWidth = ref(0)
const workspaceHeight = ref(0)
const activeResize = ref<AIWorkspacePlacement | null>(null)
const placement = computed(() => settingsStore.aiWorkspacePlacement)
const isOpen = computed(() => props.open)
const effectivePanelSize = computed(() => getEffectivePanelSize(placement.value))
const resizeBounds = computed<ResizeBounds>(() => getResizeBounds(placement.value))
const resizeAriaLabel = computed(() => (
  placement.value === 'right'
    ? 'AIワークスペースの幅を調整'
    : 'AIワークスペースの高さを調整'
))
const workspacePanelStyle = computed(() => {
  const size = `${effectivePanelSize.value}px`
  return placement.value === 'right'
    ? { width: size, flexBasis: size }
    : { height: size, flexBasis: size }
})
const activeNoteLabel = computed(() => noteStore.activeNote?.title?.trim() || '未選択')
const isAnyBusy = computed(() => (
  aiStore.isGenerating
  || librarianStore.isGenerating
  || assistantStore.isBusy
  || writingStore.isBusy
))
const isPromptComposerFeature = computed(() => (
  activeFeature.value === 'assistant' || activeFeature.value === 'writing'
))
const isComposerFeature = computed(() => activeFeature.value !== 'records')
const canUseComposer = computed(() => Boolean(noteStore.activeNote && !noteStore.activeNote.isTrashed) && isComposerFeature.value && !isAnyBusy.value)
const composerMaximumLength = computed(() => activeFeature.value === 'assistant' ? 8000 : 12_000)
const canSubmitComposer = computed(() => (
  canUseComposer.value
  && (
    !isPromptComposerFeature.value
    || (composerText.value.trim() !== '' && composerText.value.length <= composerMaximumLength.value)
  )
))
const selectedLibrarianLabel = computed(() => (
  librarianOperations.find((operation) => operation.value === selectedLibrarianOperation.value)?.label ?? 'AI司書'
))
const composerFeatureLabel = computed(() => {
  if (activeFeature.value === 'assistant') return '質問・壁打ち'
  if (activeFeature.value === 'writing') return 'AIライティング'
  if (activeFeature.value === 'summary') return '要約'
  if (activeFeature.value === 'librarian') return `AI司書：${selectedLibrarianLabel.value}`
  return 'AI機能'
})
const composerFeatureIcon = computed<Component>(() => {
  if (activeFeature.value === 'assistant') return MessageSquareIcon
  if (activeFeature.value === 'writing') return PenLineIcon
  if (activeFeature.value === 'librarian') return BookOpenIcon
  return SparklesIcon
})
const composerPlaceholder = computed(() => {
  if (activeFeature.value === 'assistant') return '現在のノートについて質問する'
  if (activeFeature.value === 'writing') return '作成したい文章の目的、読者、形式を入力'
  if (activeFeature.value === 'summary') return '現在のノートをMarkdown形式で要約します'
  if (activeFeature.value === 'librarian') return `${selectedLibrarianLabel.value}を生成します`
  return 'AI機能を選択'
})
const modelButtonLabel = computed(() => aiStore.configuredSetting?.modelID || 'モデルを設定')
const sendButtonLabel = computed(() => (
  activeFeature.value === 'assistant'
    ? '質問を送信'
    : activeFeature.value === 'writing'
      ? '文章を生成'
      : activeFeature.value === 'summary'
        ? '要約を開始'
        : activeFeature.value === 'librarian'
          ? 'AI司書を開始'
          : 'AI機能を選択してください'
))
const composerStatus = computed(() => {
  if (assistantStore.state === 'loading-context') return '参照を確認しています…'
  if (assistantStore.state === 'generating') return '質問を送信しました。AIの回答を待っています…'
  if (writingStore.state === 'loading-context') return '参照を確認しています…'
  if (writingStore.state === 'generating') return '作成指示を送信しました。文章を生成しています…'
  if (aiStore.summaryState === 'generating') return '要約を送信しました。生成しています…'
  if (librarianStore.isGenerating) return 'AI司書へ送信しました。候補を生成しています…'
  return ''
})

function getResizeBounds(targetPlacement: AIWorkspacePlacement): ResizeBounds {
  const isRight = targetPlacement === 'right'
  const min = isRight ? AI_WORKSPACE_RIGHT_WIDTH_MIN : AI_WORKSPACE_BOTTOM_HEIGHT_MIN
  const configuredMax = isRight ? AI_WORKSPACE_RIGHT_WIDTH_MAX : AI_WORKSPACE_BOTTOM_HEIGHT_MAX
  const availableSize = isRight ? workspaceWidth.value : workspaceHeight.value
  const editorMinimum = isRight ? AI_WORKSPACE_EDITOR_WIDTH_MIN : AI_WORKSPACE_EDITOR_HEIGHT_MIN
  const max = availableSize > 0
    ? Math.max(min, Math.min(configuredMax, availableSize - editorMinimum))
    : configuredMax
  return { min, max }
}

function getEffectivePanelSize(targetPlacement: AIWorkspacePlacement) {
  const isRight = targetPlacement === 'right'
  const preferredSize = isRight
    ? settingsStore.aiWorkspaceRightWidth
    : settingsStore.aiWorkspaceBottomHeight
  const availableSize = isRight ? workspaceWidth.value : workspaceHeight.value
  if (availableSize <= 0) return preferredSize

  const min = isRight ? AI_WORKSPACE_RIGHT_WIDTH_MIN : AI_WORKSPACE_BOTTOM_HEIGHT_MIN
  const configuredMax = isRight ? AI_WORKSPACE_RIGHT_WIDTH_MAX : AI_WORKSPACE_BOTTOM_HEIGHT_MAX
  const editorMinimum = isRight ? AI_WORKSPACE_EDITOR_WIDTH_MIN : AI_WORKSPACE_EDITOR_HEIGHT_MIN
  const responsiveRatio = isRight
    ? AI_WORKSPACE_RIGHT_RESPONSIVE_RATIO
    : AI_WORKSPACE_BOTTOM_RESPONSIVE_RATIO
  const max = Math.max(
    min,
    Math.min(
      configuredMax,
      Math.floor(availableSize * responsiveRatio),
      availableSize - editorMinimum,
    ),
  )
  return clamp(preferredSize, min, max)
}

let resizeObserver: ResizeObserver | null = null
let resizePointerTarget: HTMLElement | null = null
let resizePointerID: number | null = null
let resizeStartPosition = 0
let resizeStartSize = 0

function closeWorkspace() {
  if (!isOpen.value) return
  finishResize()
  emit('update:open', false)
  emit('closed')
}

function focusComposer() {
  composerTextarea.value?.focus()
}

async function selectFeature(feature: WorkspaceFeature) {
  if (isAnyBusy.value) return
  activeFeature.value = feature
  if (feature === 'assistant' || feature === 'writing') {
    await nextTick()
    focusComposer()
  }
}

async function selectComposerFeature(feature: Exclude<WorkspaceFeature, 'records'>) {
  if (isAnyBusy.value) return
  activeFeature.value = feature
  if (feature === 'assistant' || feature === 'writing') {
    await nextTick()
    focusComposer()
  }
}

async function selectLibrarianOperation(operation: LibrarianOperation) {
  if (isAnyBusy.value) return
  selectedLibrarianOperation.value = operation
  activeFeature.value = 'librarian'
  await nextTick()
}

function openAISettings() {
  settingsStore.openSettings('ai')
}

function showRecords() {
  activeFeature.value = 'records'
}

async function submitComposer() {
  if (!canUseComposer.value) return
  if (!aiStore.configuredSetting?.modelID) {
    openAISettings()
    return
  }

  if (activeFeature.value === 'summary') {
    void summaryPanel.value?.startSummary()
    return
  }
  if (activeFeature.value === 'librarian') {
    void librarianPanel.value?.startOperation(selectedLibrarianOperation.value)
    return
  }

  const prompt = composerText.value.trim()
  if (!prompt) return

  const submitted = activeFeature.value === 'assistant'
    ? await assistantPanel.value?.submitPrompt(prompt)
    : activeFeature.value === 'writing'
      ? await writingPanel.value?.submitPrompt(prompt)
      : false
  if (submitted) composerText.value = ''
}

async function openHistory(id: string) {
  const panel = assistantPanel.value
  if (panel && await panel.openHistory(id)) {
    activeFeature.value = 'assistant'
  }
}

async function openArtifact(id: string) {
  const panel = writingPanel.value
  if (panel && await panel.openArtifact(id)) {
    activeFeature.value = 'writing'
  }
}

async function openSummary(id: string) {
  if (await aiStore.loadSummaryHistory(id)) {
    activeFeature.value = 'summary'
  }
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

function resizePanel(targetPlacement: AIWorkspacePlacement, requestedSize: number) {
  const bounds = getResizeBounds(targetPlacement)
  const size = clamp(requestedSize, bounds.min, bounds.max)
  if (targetPlacement === 'right') {
    settingsStore.setAIWorkspaceRightWidth(size)
    return
  }
  settingsStore.setAIWorkspaceBottomHeight(size)
}

function updateWorkspaceSize() {
  if (!workspaceRoot.value) return
  workspaceWidth.value = workspaceRoot.value.clientWidth
  workspaceHeight.value = workspaceRoot.value.clientHeight
}

function startResize(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return

  const target = event.currentTarget
  if (!(target instanceof HTMLElement)) return

  const currentPlacement = placement.value
  activeResize.value = currentPlacement
  resizePointerTarget = target
  resizePointerID = event.pointerId
  resizeStartPosition = currentPlacement === 'right' ? event.clientX : event.clientY
  resizeStartSize = effectivePanelSize.value
  target.setPointerCapture(event.pointerId)
  document.body.classList.add(`is-ai-workspace-resizing-${currentPlacement}`)
}

function handleResize(event: PointerEvent) {
  const currentPlacement = activeResize.value
  if (!currentPlacement || resizePointerID !== event.pointerId) return

  const position = currentPlacement === 'right' ? event.clientX : event.clientY
  resizePanel(currentPlacement, resizeStartSize - (position - resizeStartPosition))
}

function finishResize(event?: PointerEvent) {
  if (!activeResize.value) return
  if (event && resizePointerID !== event.pointerId) return

  if (
    resizePointerTarget
    && resizePointerID !== null
    && resizePointerTarget.hasPointerCapture(resizePointerID)
  ) {
    resizePointerTarget.releasePointerCapture(resizePointerID)
  }
  activeResize.value = null
  resizePointerTarget = null
  resizePointerID = null
  document.body.classList.remove('is-ai-workspace-resizing-right', 'is-ai-workspace-resizing-bottom')
}

function handleResizerKeydown(event: KeyboardEvent) {
  const currentPlacement = placement.value
  let delta = 0
  if (currentPlacement === 'right') {
    if (event.key === 'ArrowLeft') delta = 10
    if (event.key === 'ArrowRight') delta = -10
  } else {
    if (event.key === 'ArrowUp') delta = 10
    if (event.key === 'ArrowDown') delta = -10
  }
  if (!delta) return

  event.preventDefault()
  resizePanel(currentPlacement, effectivePanelSize.value + delta)
}

watch(() => props.open, async (open) => {
  if (!open) {
    finishResize()
    return
  }
  await nextTick()
  focusComposer()
})

watch(() => settingsStore.aiWorkspacePlacement, async () => {
  finishResize()
  await nextTick()
  updateWorkspaceSize()
})

onMounted(() => {
  if (!workspaceRoot.value) return
  updateWorkspaceSize()
  resizeObserver = new ResizeObserver(updateWorkspaceSize)
  resizeObserver.observe(workspaceRoot.value)
})

onBeforeUnmount(() => {
  finishResize()
  resizeObserver?.disconnect()
})
</script>

<style scoped>
.ai-workspace {
  position: relative;
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.ai-workspace.is-right {
  flex-direction: row;
}

.ai-workspace.is-bottom {
  flex-direction: column;
}

.ai-workspace-editor {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
}

.ai-workspace-panel {
  display: flex;
  min-width: 360px;
  flex: 0 0 auto;
  min-height: 0;
  flex-direction: column;
  container-type: inline-size;
  border-left: 1px solid var(--border);
  background: var(--bg-sidebar);
}

.ai-workspace.is-bottom .ai-workspace-panel {
  width: 100%;
  min-width: 0;
  min-height: 220px;
  border-top: 1px solid var(--border);
  border-left: none;
}

.ai-workspace-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 34px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
}

.ai-workspace-header-title,
.ai-workspace-header-actions,
.ai-workspace-composer-toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ai-workspace-header-title {
  min-width: 0;
  color: var(--text-primary);
}

.ai-workspace-header-actions {
  flex-shrink: 0;
}

.ai-workspace-icon-button,
.ai-workspace-composer-icon-button {
  display: inline-grid;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
}

.ai-workspace-icon-button:hover,
.ai-workspace-icon-button:focus-visible,
.ai-workspace-icon-button[aria-pressed='true'],
.ai-workspace-composer-icon-button:hover,
.ai-workspace-composer-icon-button:focus-visible {
  border-color: var(--brand-primary);
  color: var(--brand-primary);
}

.ai-workspace-resizer {
  position: relative;
  z-index: 1;
  flex: 0 0 8px;
  padding: 0;
  border: none;
  background: transparent;
  touch-action: none;
}

.ai-workspace.is-right .ai-workspace-resizer {
  width: 8px;
  cursor: col-resize;
}

.ai-workspace.is-bottom .ai-workspace-resizer {
  height: 8px;
  cursor: row-resize;
}

.ai-workspace-resizer::after {
  content: '';
  position: absolute;
  background: transparent;
  transition: background-color 0.12s;
}

.ai-workspace.is-right .ai-workspace-resizer::after {
  top: 0;
  bottom: 0;
  left: 3px;
  width: 2px;
}

.ai-workspace.is-bottom .ai-workspace-resizer::after {
  top: 3px;
  right: 0;
  left: 0;
  height: 2px;
}

.ai-workspace-resizer:hover::after,
.ai-workspace-resizer:focus-visible::after,
.ai-workspace-resizer.is-resizing::after {
  background: var(--brand-primary);
}

.ai-workspace-resizer:focus-visible {
  outline: none;
}

:global(body.is-ai-workspace-resizing-right),
:global(body.is-ai-workspace-resizing-right *) {
  cursor: col-resize !important;
  user-select: none !important;
}

:global(body.is-ai-workspace-resizing-bottom),
:global(body.is-ai-workspace-resizing-bottom *) {
  cursor: row-resize !important;
  user-select: none !important;
}

.ai-workspace-context {
  align-self: flex-start;
  max-width: calc(100% - 20px);
  margin: 8px 10px 0;
  padding: 4px 7px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workspace-content {
  flex: 1;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.ai-workspace-feature {
  min-height: 100%;
}

.ai-workspace-composer {
  display: grid;
  flex: 0 0 auto;
  gap: 7px;
  padding: 10px;
  border-top: 1px solid var(--border);
  background: var(--bg-sidebar);
}

.ai-workspace-composer-textarea {
  box-sizing: border-box;
  width: 100%;
  min-height: 54px;
  max-height: 160px;
  padding: 8px 9px;
  resize: vertical;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font: inherit;
  line-height: 1.45;
}

.ai-workspace-composer-textarea:disabled {
  cursor: not-allowed;
  opacity: .7;
}

.ai-workspace-model-button {
  display: inline-flex;
  min-width: 0;
  max-width: min(100%, 220px);
  height: 28px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.ai-workspace-composer-feature-button {
  display: inline-flex;
  min-width: 0;
  max-width: min(100%, 190px);
  height: 28px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.ai-workspace-composer-feature-button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workspace-composer-feature-button:hover,
.ai-workspace-composer-feature-button:focus-visible {
  border-color: var(--brand-primary);
  color: var(--brand-primary);
}

.ai-workspace-model-button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-workspace-model-button:hover,
.ai-workspace-model-button:focus-visible {
  border-color: var(--brand-primary);
  color: var(--brand-primary);
}

.ai-workspace-composer-spacer {
  flex: 1;
}

.ai-workspace-send-button:not(:disabled) {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: #fff;
}

.ai-workspace-composer-icon-button:disabled,
.ai-workspace-model-button:disabled,
.ai-workspace-composer-feature-button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.ai-workspace-composer-status {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.4;
}

:global(.ai-workspace-action-menu) {
  z-index: 1100;
  min-width: 192px;
  padding: 5px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background-color: var(--bg-editor, #fff);
  box-shadow: 0 10px 24px rgba(15, 23, 42, .16);
  color: var(--text-primary);
  opacity: 1;
}

:global(.ai-workspace-action-menu-label) {
  padding: 5px 7px;
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
}

:global(.ai-workspace-action-menu-item) {
  display: flex;
  min-height: 30px;
  align-items: center;
  gap: 8px;
  padding: 0 7px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1.3;
  outline: none;
}

:global(.ai-workspace-action-menu-item[data-highlighted]) {
  background: var(--bg-hover);
  color: var(--brand-primary);
}

:global(.ai-workspace-action-menu-separator) {
  height: 1px;
  margin: 5px 2px;
  background: var(--border);
}

:global(.ai-workspace-action-menu-chevron) {
  margin-left: auto;
}

@container (max-width: 420px) {
  .ai-workspace-composer-toolbar {
    flex-wrap: wrap;
  }

  .ai-workspace-model-button {
    max-width: calc(100% - 76px);
  }

  .ai-workspace-composer-feature-button {
    max-width: calc(100% - 76px);
  }
}

</style>
