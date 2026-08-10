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
      @keydown="handlePanelKeydown"
    >
      <header class="ai-workspace-header">
        <div class="ai-workspace-header-title">
          <span class="ai-workspace-logo" aria-hidden="true">
            <SparklesIcon :size="15" />
          </span>
          <strong>Atlas AI</strong>
        </div>
        <div class="ai-workspace-header-actions">
          <button
            class="ai-workspace-icon-button"
            type="button"
            title="AI設定を開く"
            aria-label="AI設定を開く"
            @click="openAISettings"
          >
            <Settings2Icon :size="16" aria-hidden="true" />
          </button>
          <button
            class="ai-workspace-icon-button"
            type="button"
            title="保存済みの履歴と成果物を開く"
            aria-label="保存済みの履歴と成果物を開く"
            :aria-pressed="recordsOpen"
            :disabled="isAnyBusy"
            @click="toggleRecords"
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

      <div
        v-show="!recordsOpen"
        ref="timelineRoot"
        class="ai-chat-timeline"
        role="log"
        aria-label="AIとの会話"
        aria-live="polite"
        aria-relevant="additions text"
        :aria-busy="isAnyBusy"
      >
        <div v-if="chatStore.timeline.length === 0 && !hasVisibleResult" class="ai-chat-empty">
          <span class="ai-chat-empty-icon" aria-hidden="true">
            <SparklesIcon :size="20" />
          </span>
          <strong>開いているノートから始めます</strong>
          <p>
            質問を入力するか、＋から参照するノートとスキル・ツールを追加してください。
          </p>
        </div>

        <template v-for="entry in chatStore.timeline" :key="entry.id">
          <AIAgentProposalCard
            v-if="entry.kind === 'agent-proposal'"
            :content="entry.content"
            :proposal="entry.proposal"
            :state="entry.proposalState ?? 'generating'"
            :busy="isAnyBusy"
            @apply="applyAgentProposal(entry.id)"
            @discard="discardAgentProposal(entry.id)"
          />
          <article v-else :class="['ai-chat-entry', `is-${entry.role}`, `is-${entry.kind}`]">
            <div class="ai-chat-entry-heading">
              <span class="ai-chat-entry-avatar" aria-hidden="true">
                <UserIcon v-if="entry.role === 'user'" :size="14" />
                <WrenchIcon v-else-if="entry.role === 'tool'" :size="14" />
                <AlertCircleIcon v-else-if="entry.kind === 'error'" :size="14" />
                <SparklesIcon v-else :size="14" />
              </span>
              <strong>{{ timelineRoleLabel(entry) }}</strong>
              <span v-if="entry.status" class="ai-chat-entry-status">
                <LoaderCircleIcon
                  v-if="entry.status === 'pending'"
                  class="is-spinning"
                  :size="13"
                  aria-hidden="true"
                />
                <CheckIcon v-else-if="entry.status === 'success'" :size="13" aria-hidden="true" />
                <AlertCircleIcon v-else :size="13" aria-hidden="true" />
                {{ timelineStatusLabel(entry.status) }}
              </span>
            </div>

            <AIMarkdownPreview
              v-if="entry.role === 'assistant'"
              class="ai-chat-markdown"
              :markdown="entry.content"
              aria-label="AIの回答"
            />
            <p v-else class="ai-chat-entry-content">{{ entry.content }}</p>

            <div v-if="entry.citations?.length" class="ai-chat-citation-block">
              <strong>Web出典</strong>
              <ul class="ai-chat-citations" aria-label="Web検索の参照元">
                <li v-for="(citation, index) in entry.citations" :key="`${entry.id}-${citation.url}-${index}`">
                  <a
                    v-if="safeCitationURL(citation.url)"
                    :href="safeCitationURL(citation.url) ?? undefined"
                    target="_blank"
                    rel="noreferrer noopener"
                  >
                    {{ citation.title?.trim() || citationHost(citation.url) }}
                  </a>
                  <span v-else>安全でないURLのため表示できません</span>
                </li>
              </ul>
            </div>

            <button
              v-if="canSaveAssistantEntry(entry)"
              class="ai-chat-inline-button"
              type="button"
              title="この会話を保存"
              aria-label="この会話を保存"
              @click="saveConversation"
            >
              <SaveIcon :size="14" aria-hidden="true" />
              保存
            </button>
          </article>

          <AISummaryPanel
            v-if="entry.id === visibleSummaryTraceID"
            :ref="setSummaryPanelRef"
            class="ai-chat-result-card"
            timeline
          />
          <AILibrarianPanel
            v-if="entry.id === visibleLibrarianTraceID"
            :ref="setLibrarianPanelRef"
            class="ai-chat-result-card"
            timeline
          />
          <AIWritingPanel
            v-if="entry.id === visibleWritingTraceID"
            :ref="setWritingPanelRef"
            class="ai-chat-result-card"
            external-composer
            timeline
            :additional-note-ids="additionalNoteIDs"
          />
        </template>

        <div
          v-if="assistantStateWarning"
          class="ai-chat-state-warning"
          role="status"
        >
          <AlertCircleIcon :size="14" aria-hidden="true" />
          <span>{{ assistantStateWarning }}</span>
        </div>

        <div
          v-if="composerStatus"
          class="ai-chat-progress"
          role="status"
          aria-live="polite"
        >
          <LoaderCircleIcon class="is-spinning" :size="15" aria-hidden="true" />
          <span>{{ composerStatus }}</span>
        </div>

        <AIAssistantPanel
          ref="assistantPanel"
          external-composer
          execution-bridge
          :additional-note-ids="additionalNoteIDs"
          :chat-mode="chatStore.mode"
          :web-search="isWebSearchSelected"
        />
      </div>

      <div v-show="recordsOpen" class="ai-workspace-records">
        <AIRecordsPanel
          @open-artifact="openArtifact"
          @open-history="openHistory"
          @open-summary="openSummary"
        />
      </div>

      <form
        v-show="!recordsOpen"
        class="ai-chat-composer"
        aria-label="AI入力"
        @submit.prevent="submitComposer"
      >
        <div class="ai-chat-contexts" aria-label="参照コンテキスト">
          <span
            v-if="chatStore.activeNoteContext"
            class="ai-chat-context-chip is-fixed"
            :title="`開いているノート: ${chatStore.activeNoteContext.label}`"
          >
            <LockKeyholeIcon :size="12" aria-hidden="true" />
            <span>{{ chatStore.activeNoteContext.label }}</span>
            <span class="sr-only">開いているノート。削除できません。</span>
          </span>

          <span
            v-for="context in chatStore.explicitContexts"
            :key="`${context.kind}:${context.id}`"
            class="ai-chat-context-chip"
          >
            <FileTextIcon v-if="context.kind === 'note'" :size="12" aria-hidden="true" />
            <FolderIcon v-else :size="12" aria-hidden="true" />
            <span>{{ context.label }}</span>
            <small v-if="context.kind === 'notebook'">
              {{ notebookContextDetail(context.id) }}
            </small>
            <button
              type="button"
              :aria-label="`${context.label}を参照から削除`"
              :disabled="isAnyBusy"
              @click="chatStore.removeContext(context.kind, context.id)"
            >
              <XIcon :size="11" aria-hidden="true" />
            </button>
          </span>

          <span v-if="chatStore.selectedTool" class="ai-chat-context-chip is-tool">
            <component :is="selectedToolIcon" :size="12" aria-hidden="true" />
            <span>{{ selectedToolLabel }}</span>
            <button
              type="button"
              :aria-label="`${selectedToolLabel}を解除`"
              :disabled="isAnyBusy"
              @click="chatStore.selectTool(null)"
            >
              <XIcon :size="11" aria-hidden="true" />
            </button>
          </span>
        </div>

        <p v-if="chatStore.contextError" class="ai-chat-composer-error" role="alert">
          {{ chatStore.contextError }}
        </p>
        <p v-if="webSearchUnavailableMessage" class="ai-chat-composer-warning" role="status">
          {{ webSearchUnavailableMessage }}
        </p>
        <p v-if="fixedScopeToolMessage" class="ai-chat-composer-warning" role="status">
          {{ fixedScopeToolMessage }}
        </p>

        <div
          v-if="contextPickerOpen"
          class="ai-chat-context-picker"
          role="dialog"
          :aria-label="contextPickerTitle"
          @keydown.esc.stop="closeContextPicker(true)"
        >
          <div class="ai-chat-context-picker-heading">
            <strong>{{ contextPickerTitle }}</strong>
            <button
              class="ai-workspace-icon-button"
              type="button"
              title="コンテキスト選択を閉じる"
              aria-label="コンテキスト選択を閉じる"
              @click="closeContextPicker(true)"
            >
              <XIcon :size="14" aria-hidden="true" />
            </button>
          </div>

          <label v-if="contextPickerOpen === 'note'" class="ai-chat-context-search">
            <SearchIcon :size="14" aria-hidden="true" />
            <span class="sr-only">追加するノートを検索</span>
            <input
              ref="contextSearchInput"
              v-model="contextQuery"
              type="search"
              maxlength="200"
              placeholder="ノートを検索"
            />
          </label>

          <p v-if="chatStore.isContextLoading" class="ai-chat-picker-status" role="status">
            ノート候補を読み込んでいます…
          </p>
          <ul v-else ref="contextOptions" class="ai-chat-context-options">
            <template v-if="contextPickerOpen === 'note'">
              <li v-for="item in filteredCatalogNotes" :key="item.id">
                <button
                  type="button"
                  :disabled="isAnyBusy"
                  @click="addNoteContext(item.id, item.title)"
                >
                  <FileTextIcon :size="14" aria-hidden="true" />
                  <span>{{ item.title || '無題のノート' }}</span>
                </button>
              </li>
              <li v-if="filteredCatalogNotes.length === 0" class="ai-chat-picker-empty">
                追加できるノートがありません。
              </li>
            </template>
            <template v-else>
              <li v-for="notebook in notebookStore.notebooks" :key="notebook.id">
                <button
                  type="button"
                  :disabled="isAnyBusy || !chatStore.isContextCatalogReady"
                  @click="addNotebookContext(notebook.id, notebook.name)"
                >
                  <FolderIcon :size="14" aria-hidden="true" />
                  <span>{{ notebook.name || '無題のノートブック' }}</span>
                </button>
              </li>
              <li v-if="notebookStore.notebooks.length === 0" class="ai-chat-picker-empty">
                追加できるノートブックがありません。
              </li>
            </template>
          </ul>
        </div>

        <div class="ai-chat-input-shell" :class="{ 'has-picker': contextPickerOpen }">
          <textarea
            ref="composerTextarea"
            v-model="chatStore.draft"
            class="ai-chat-textarea"
            rows="3"
            :maxlength="composerMaxLength"
            aria-label="AIへのメッセージ"
            :placeholder="composerPlaceholder"
            :disabled="!hasUsableNote"
            :readonly="isFixedScopeToolSelected"
            @compositionstart="isComposing = true"
            @compositionend="handleCompositionEnd"
            @keydown="handleComposerKeydown"
          />

          <div class="ai-chat-composer-toolbar">
            <DropdownMenuRoot>
              <DropdownMenuTrigger as-child>
                <button
                  ref="contextMenuTrigger"
                  class="ai-chat-toolbar-button is-icon"
                  type="button"
                  title="コンテキストまたはスキル・ツールを追加"
                  aria-label="コンテキストまたはスキル・ツールを追加"
                  aria-haspopup="menu"
                  :disabled="isAnyBusy"
                >
                  <PlusIcon :size="17" aria-hidden="true" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuPortal>
                <DropdownMenuContent
                  class="ai-chat-menu"
                  side="top"
                  align="start"
                  :side-offset="8"
                >
                  <DropdownMenuLabel class="ai-chat-menu-label">コンテキスト</DropdownMenuLabel>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="openContextPicker('note')">
                    <FileTextIcon :size="15" aria-hidden="true" />
                    ノート
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="openContextPicker('notebook')">
                    <FolderIcon :size="15" aria-hidden="true" />
                    ノートブック
                  </DropdownMenuItem>
                  <DropdownMenuSeparator class="ai-chat-menu-separator" />
                  <DropdownMenuLabel class="ai-chat-menu-label">スキル・ツール</DropdownMenuLabel>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('summary')">
                    <SparklesIcon :size="15" aria-hidden="true" />
                    要約
                  </DropdownMenuItem>
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger class="ai-chat-menu-item">
                      <FilePenLineIcon :size="15" aria-hidden="true" />
                      文章作成
                      <ChevronRightIcon class="ai-chat-menu-chevron" :size="14" aria-hidden="true" />
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent class="ai-chat-menu" :side-offset="6" :align-offset="-4">
                      <DropdownMenuLabel class="ai-chat-menu-label">文章作成</DropdownMenuLabel>
                      <DropdownMenuItem
                        v-for="writingKind in writingKinds"
                        :key="writingKind.value"
                        class="ai-chat-menu-item"
                        @select="selectWritingTool(writingKind.value)"
                      >
                        <FilePenLineIcon :size="15" aria-hidden="true" />
                        {{ writingKind.label }}
                      </DropdownMenuItem>
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('title')">
                    <PenLineIcon :size="15" aria-hidden="true" />
                    タイトル候補
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('tags')">
                    <TagsIcon :size="15" aria-hidden="true" />
                    タグ候補
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('classification')">
                    <FolderTreeIcon :size="15" aria-hidden="true" />
                    分類候補
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('related')">
                    <LinkIcon :size="15" aria-hidden="true" />
                    関連メモ
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="selectTool('duplicate')">
                    <CopyIcon :size="15" aria-hidden="true" />
                    重複候補
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    class="ai-chat-menu-item"
                    :disabled="!isWebSearchAvailable"
                    @select="selectTool('web-search')"
                  >
                    <Globe2Icon :size="15" aria-hidden="true" />
                    Web検索
                    <small v-if="!isWebSearchAvailable">OpenRouterのみ</small>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenuPortal>
            </DropdownMenuRoot>

            <DropdownMenuRoot>
              <DropdownMenuTrigger as-child>
                <button
                  class="ai-chat-toolbar-button"
                  type="button"
                  :title="`${modeLabel}モードを切り替える`"
                  :aria-label="`${modeLabel}モードを切り替える`"
                  aria-haspopup="menu"
                  :disabled="isAnyBusy"
                >
                  <BotIcon v-if="chatStore.mode === 'agent'" :size="14" aria-hidden="true" />
                  <MessageSquareIcon v-else :size="14" aria-hidden="true" />
                  <span>{{ modeLabel }}</span>
                  <ChevronDownIcon :size="13" aria-hidden="true" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuPortal>
                <DropdownMenuContent
                  class="ai-chat-menu"
                  side="top"
                  align="start"
                  :side-offset="8"
                >
                  <DropdownMenuLabel class="ai-chat-menu-label">モード</DropdownMenuLabel>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="chatStore.setMode('ask')">
                    <MessageSquareIcon :size="15" aria-hidden="true" />
                    <span>
                      <strong>Ask</strong>
                      <small>ノートを参照して回答</small>
                    </span>
                    <CheckIcon v-if="chatStore.mode === 'ask'" class="ai-chat-menu-check" :size="14" />
                  </DropdownMenuItem>
                  <DropdownMenuItem class="ai-chat-menu-item" @select="chatStore.setMode('agent')">
                    <BotIcon :size="15" aria-hidden="true" />
                    <span>
                      <strong>Agent</strong>
                      <small>選択したツールを使って作業</small>
                    </span>
                    <CheckIcon v-if="chatStore.mode === 'agent'" class="ai-chat-menu-check" :size="14" />
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenuPortal>
            </DropdownMenuRoot>

            <button
              class="ai-chat-toolbar-button"
              type="button"
              :title="`Auto（${configuredModelLabel}）の設定を開く`"
              :aria-label="`Autoモデル設定を開く。現在のモデル: ${configuredModelLabel}`"
              :disabled="isAnyBusy"
              @click="openAISettings"
            >
              <SparklesIcon :size="13" aria-hidden="true" />
              <span>Auto</span>
              <ChevronDownIcon :size="13" aria-hidden="true" />
            </button>

            <span class="ai-chat-toolbar-spacer" />

            <button
              class="ai-chat-send-button"
              type="submit"
              :title="sendButtonLabel"
              :aria-label="sendButtonLabel"
              :disabled="!canSubmitComposer"
            >
              <SendIcon :size="17" aria-hidden="true" />
            </button>
          </div>
        </div>

        <p v-if="submitBlockedMessage" class="ai-chat-submit-hint" role="status">
          {{ submitBlockedMessage }}
        </p>
      </form>
    </aside>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Component, ComponentPublicInstance } from 'vue'
import {
  AlertCircleIcon,
  ArchiveIcon,
  BotIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CopyIcon,
  FilePenLineIcon,
  FileTextIcon,
  FolderIcon,
  FolderTreeIcon,
  Globe2Icon,
  LinkIcon,
  LoaderCircleIcon,
  LockKeyholeIcon,
  MessageSquareIcon,
  PenLineIcon,
  PlusIcon,
  SaveIcon,
  SearchIcon,
  SendIcon,
  Settings2Icon,
  SparklesIcon,
  TagsIcon,
  UserIcon,
  WrenchIcon,
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
import type { AIChatMode, LibrarianOperation, WritingKind } from '../api/ai'
import {
  AI_WORKSPACE_BOTTOM_HEIGHT_MAX,
  AI_WORKSPACE_BOTTOM_HEIGHT_MIN,
  AI_WORKSPACE_RIGHT_WIDTH_MAX,
  AI_WORKSPACE_RIGHT_WIDTH_MIN,
  type AIWorkspacePlacement,
  useSettingsStore,
} from '../stores/useSettingsStore'
import {
  type AIChatTimelineEntry,
  type AIChatTool,
  useAIChatStore,
} from '../stores/useAIChatStore'
import { useAIStore } from '../stores/useAIStore'
import { useAIAssistantStore } from '../stores/useAIAssistantStore'
import { useAILibrarianStore } from '../stores/useAILibrarianStore'
import { useAIWritingStore } from '../stores/useAIWritingStore'
import { useNotebookStore } from '../stores/useNotebookStore'
import { useNoteStore } from '../stores/useNoteStore'
import AIMarkdownPreview from './AIMarkdownPreview.vue'
import AISummaryPanel from './AISummaryPanel.vue'
import AILibrarianPanel from './AILibrarianPanel.vue'
import AIAssistantPanel from './AIAssistantPanel.vue'
import AIWritingPanel from './AIWritingPanel.vue'
import AIRecordsPanel from './AIRecordsPanel.vue'
import AIAgentProposalCard from './AIAgentProposalCard.vue'

type ContextPickerKind = 'note' | 'notebook'
type SummaryPanelHandle = { startSummary: () => Promise<boolean> }
type LibrarianPanelHandle = { startOperation: (operation: LibrarianOperation) => Promise<boolean> }
type AssistantPanelHandle = {
  openHistory: (id: string) => Promise<boolean>
  submitPrompt: (prompt: string) => Promise<boolean>
}
type WritingPanelHandle = {
  openArtifact: (id: string) => Promise<boolean>
  submitPrompt: (prompt: string, kind?: WritingKind) => Promise<boolean>
}
type ResizeBounds = { min: number; max: number }

const AI_WORKSPACE_EDITOR_WIDTH_MIN = 360
const AI_WORKSPACE_EDITOR_HEIGHT_MIN = 240
const AI_WORKSPACE_RIGHT_RESPONSIVE_RATIO = 0.6
const AI_WORKSPACE_BOTTOM_RESPONSIVE_RATIO = 0.6

const toolDefinitions: ReadonlyArray<{
  value: AIChatTool
  label: string
  icon: Component
}> = [
  { value: 'summary', label: '要約', icon: SparklesIcon },
  { value: 'writing', label: '文章作成', icon: FilePenLineIcon },
  { value: 'title', label: 'タイトル候補', icon: PenLineIcon },
  { value: 'tags', label: 'タグ候補', icon: TagsIcon },
  { value: 'classification', label: '分類候補', icon: FolderTreeIcon },
  { value: 'related', label: '関連メモ', icon: LinkIcon },
  { value: 'duplicate', label: '重複候補', icon: CopyIcon },
  { value: 'web-search', label: 'Web検索', icon: Globe2Icon },
]

const librarianToolMap: Partial<Record<AIChatTool, LibrarianOperation>> = {
  title: 'title',
  tags: 'tags',
  classification: 'classification',
  related: 'related',
  duplicate: 'duplicate',
}

const writingKinds: ReadonlyArray<{ value: WritingKind; label: string }> = [
  { value: 'prompt', label: 'プロンプト' },
  { value: 'prompt-improvement', label: 'プロンプト改善' },
  { value: 'readme', label: 'README草案' },
  { value: 'document', label: 'ドキュメント草案' },
  { value: 'blog', label: 'ブログ草案' },
  { value: 'requirements', label: '要件定義草案' },
]

const allowedToolsByMode: Record<AIChatMode, ReadonlySet<AIChatTool>> = {
  ask: new Set<AIChatTool>([
    'summary',
    'writing',
    'title',
    'tags',
    'classification',
    'related',
    'duplicate',
    'web-search',
  ]),
  agent: new Set<AIChatTool>([
    'summary',
    'writing',
    'title',
    'tags',
    'classification',
    'related',
    'duplicate',
    'web-search',
  ]),
}
const fixedScopeTools: ReadonlySet<AIChatTool> = new Set([
  'summary',
  'title',
  'tags',
  'classification',
  'related',
  'duplicate',
])

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  'update:open': [open: boolean]
  closed: []
}>()

const settingsStore = useSettingsStore()
const aiStore = useAIStore()
const chatStore = useAIChatStore()
const librarianStore = useAILibrarianStore()
const assistantStore = useAIAssistantStore()
const writingStore = useAIWritingStore()
const notebookStore = useNotebookStore()
const noteStore = useNoteStore()
const workspaceRoot = ref<HTMLElement | null>(null)
const timelineRoot = ref<HTMLElement | null>(null)
const composerTextarea = ref<HTMLTextAreaElement | null>(null)
const contextSearchInput = ref<HTMLInputElement | null>(null)
const contextOptions = ref<HTMLUListElement | null>(null)
const contextMenuTrigger = ref<HTMLButtonElement | null>(null)
const summaryPanel = ref<SummaryPanelHandle | null>(null)
const librarianPanel = ref<LibrarianPanelHandle | null>(null)
const assistantPanel = ref<AssistantPanelHandle | null>(null)
const writingPanel = ref<WritingPanelHandle | null>(null)
const recordsOpen = ref(false)
const contextPickerOpen = ref<ContextPickerKind | null>(null)
const contextQuery = ref('')
const isComposing = ref(false)
const isSubmitting = ref(false)
const workspaceWidth = ref(0)
const workspaceHeight = ref(0)
const activeResize = ref<AIWorkspacePlacement | null>(null)
const activeLibrarianTraceID = ref<string | null>(null)
const visibleSummaryTraceID = ref<string | null>(null)
const visibleLibrarianTraceID = ref<string | null>(null)
const visibleWritingTraceID = ref<string | null>(null)
const selectedWritingKind = ref<WritingKind>('document')

type TemplateRefValue = Element | ComponentPublicInstance | null

function setSummaryPanelRef(value: TemplateRefValue) {
  const panel = value as unknown as SummaryPanelHandle | null
  if (panel && typeof panel.startSummary === 'function') {
    summaryPanel.value = panel
  } else if (!visibleSummaryTraceID.value) {
    summaryPanel.value = null
  }
}

function setLibrarianPanelRef(value: TemplateRefValue) {
  const panel = value as unknown as LibrarianPanelHandle | null
  if (panel && typeof panel.startOperation === 'function') {
    librarianPanel.value = panel
  } else if (!visibleLibrarianTraceID.value) {
    librarianPanel.value = null
  }
}

function setWritingPanelRef(value: TemplateRefValue) {
  const panel = value as unknown as WritingPanelHandle | null
  if (
    panel
    && typeof panel.openArtifact === 'function'
    && typeof panel.submitPrompt === 'function'
  ) {
    writingPanel.value = panel
  } else if (!visibleWritingTraceID.value) {
    writingPanel.value = null
  }
}

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
const isApplyingAgentProposal = computed(() => chatStore.timeline.some((entry) => (
  entry.kind === 'agent-proposal' && entry.proposalState === 'applying'
)))
const hasPendingAgentProposal = computed(() => chatStore.timeline.some((entry) => (
  entry.kind === 'agent-proposal'
  && (entry.proposalState === 'generating' || entry.proposalState === 'awaiting-review' || entry.proposalState === 'applying' || entry.proposalState === 'conflict' || entry.proposalState === 'save-failure')
)))
const isAnyBusy = computed(() => (
  isSubmitting.value
  || aiStore.isGenerating
  || librarianStore.isGenerating
  || assistantStore.isBusy
  || writingStore.isBusy
  || isApplyingAgentProposal.value
))
const hasUsableNote = computed(() => Boolean(
  noteStore.activeNote && !noteStore.activeNote.isTrashed,
))
const hasBlockingDraft = computed(() => (
  noteStore.activeDraft?.status === 'conflicted'
  || noteStore.activeDraft?.status === 'failed'
))
const isWebSearchSelected = computed(() => chatStore.selectedTool === 'web-search')
const isWebSearchAvailable = computed(() => (
  aiStore.configuredSetting?.providerID === 'openrouter'
))
const webSearchUnavailableMessage = computed(() => {
  if (!isWebSearchSelected.value || isWebSearchAvailable.value) return ''
  if (aiStore.configuredSetting?.providerID === 'gemini') {
    return 'GeminiのWeb検索は検索データが最大30日保持される方針とAtlas Noteの保存境界が一致しないため利用できません。OpenRouterを選択してください。'
  }
  return 'Web検索はOpenRouterを設定した場合だけ利用できます。'
})
const isSelectedToolAllowed = computed(() => (
  !chatStore.selectedTool
  || allowedToolsByMode[chatStore.mode].has(chatStore.selectedTool)
))
const isFixedScopeToolSelected = computed(() => (
  Boolean(chatStore.selectedTool && fixedScopeTools.has(chatStore.selectedTool))
))
const fixedScopeToolMessage = computed(() => (
  isFixedScopeToolSelected.value
    ? `${selectedToolLabel.value}は開いているノートだけを対象に実行します。入力文と追加コンテキストは使用しません。`
    : ''
))
const hasUnresolvedResultConflict = computed(() => {
  const tool = chatStore.selectedTool
  if (tool === 'summary') {
    return Boolean(visibleSummaryTraceID.value && aiStore.summary)
  }
  if (tool === 'writing') {
    return Boolean(visibleWritingTraceID.value && writingStore.content.trim())
  }
  if (tool && librarianToolMap[tool]) {
    return Boolean(
      visibleLibrarianTraceID.value
      && (librarianStore.result?.candidates.length ?? 0) > 0,
    )
  }
  return false
})
const hasUnreadyNotebookContext = computed(() => (
  chatStore.explicitContexts.some((context) => context.kind === 'notebook')
  && !chatStore.isContextCatalogReady
))
const canSubmitComposer = computed(() => {
  if (
    !hasUsableNote.value
    || noteStore.isLoading
    || hasBlockingDraft.value
    || isAnyBusy.value
    || !aiStore.configuredSetting?.modelID
    || Boolean(webSearchUnavailableMessage.value)
    || !isSelectedToolAllowed.value
    || hasUnresolvedResultConflict.value
    || hasUnreadyNotebookContext.value
    || (chatStore.mode === 'agent' && !chatStore.selectedTool && hasPendingAgentProposal.value)
  ) return false

  const promptRequired = (
    !chatStore.selectedTool
    || chatStore.selectedTool === 'web-search'
    || chatStore.selectedTool === 'writing'
  )
  return !promptRequired || chatStore.draft.trim() !== ''
})
const configuredModelLabel = computed(() => (
  aiStore.configuredSetting?.modelID || '未設定'
))
const modeLabel = computed(() => chatStore.mode === 'agent' ? 'Agent' : 'Ask')
const selectedToolDefinition = computed(() => (
  toolDefinitions.find((item) => item.value === chatStore.selectedTool) ?? null
))
const selectedToolLabel = computed(() => {
  if (chatStore.selectedTool === 'writing') {
    const label = writingKinds.find((item) => item.value === selectedWritingKind.value)?.label
    return `文章作成：${label ?? 'ドキュメント草案'}`
  }
  return selectedToolDefinition.value?.label ?? 'ツール'
})
const selectedToolIcon = computed<Component>(() => selectedToolDefinition.value?.icon ?? WrenchIcon)
const additionalNoteIDs = computed(() => (
  chatStore.resolvedNoteIDs.filter((noteID) => noteID !== noteStore.activeNote?.id)
))
const composerMaxLength = computed(() => (
  chatStore.selectedTool === 'writing' ? 12000 : 8000
))
const composerPlaceholder = computed(() => {
  if (!hasUsableNote.value) return 'ノートを開くとAIを利用できます'
  if (isFixedScopeToolSelected.value) {
    return `${selectedToolLabel.value}は開いているノートを対象に実行します`
  }
  if (chatStore.selectedTool === 'web-search') return 'Webで調べたいことを入力'
  if (chatStore.selectedTool === 'writing') return '作成したい文章の目的、読者、形式を入力'
  if (chatStore.selectedTool) return `${selectedToolLabel.value}への補足（任意）`
  return chatStore.mode === 'agent'
    ? 'ノートをもとに依頼する'
    : 'ノートについて質問する'
})
const sendButtonLabel = computed(() => {
  if (noteStore.isLoading) return 'ノートの読み込み完了後に送信できます'
  if (chatStore.selectedTool) return `${selectedToolLabel.value}を実行`
  return `${modeLabel.value}モードで送信`
})
const submitBlockedMessage = computed(() => {
  if (noteStore.isLoading) return 'ノートを読み込んでいます。完了後に送信できます。'
  if (!hasUsableNote.value) return 'AIを使うにはゴミ箱以外のノートを開いてください。'
  if (hasBlockingDraft.value) return 'ノートの保存競合または保存失敗を解消してから送信してください。'
  if (!aiStore.configuredSetting?.modelID) return '送信前にAI設定でモデルを選択してください。'
  if (!isSelectedToolAllowed.value) return '現在のモードではこのツールを実行できません。'
  if (hasUnreadyNotebookContext.value) {
    return 'ノート一覧を再読み込みしてから、ノートブック参照を使って送信してください。'
  }
  if (hasUnresolvedResultConflict.value) {
    return '表示中の候補を採用・破棄するか、結果カードを閉じてから同じツールを再実行してください。'
  }
  if (chatStore.mode === 'agent' && !chatStore.selectedTool && hasPendingAgentProposal.value) {
    return '現在の変更提案を適用または破棄してから、次のAgent依頼を送信してください。'
  }
  return ''
})
const assistantStateWarning = computed(() => {
  if (assistantStore.state === 'orphaned') {
    return 'この会話が参照したノートは削除されています。回答は履歴としてのみ表示しています。'
  }
  if (assistantStore.state === 'stale') {
    return '参照したノートが更新されています。この回答は更新前の内容に基づいています。'
  }
  return ''
})
const composerStatus = computed(() => {
  if (assistantStore.state === 'loading-context') return '参照ノートを確認しています…'
  if (assistantStore.state === 'generating') return isWebSearchSelected.value
    ? 'Webを検索し、回答を生成しています…'
    : 'AIの回答を生成しています…'
  if (writingStore.state === 'loading-context') return '参照ノートを確認しています…'
  if (writingStore.state === 'generating') return '文章を生成しています…'
  if (aiStore.summaryState === 'generating') return '要約を生成しています…'
  if (librarianStore.isGenerating) return '候補を生成しています…'
  if (isApplyingAgentProposal.value) return '変更提案を本文へ適用しています…'
  return ''
})
const hasVisibleResult = computed(() => (
  aiStore.summaryState !== 'idle'
  || librarianStore.state !== 'idle'
  || writingStore.state !== 'idle'
  || Boolean(writingStore.content)
  || chatStore.timeline.some((entry) => entry.kind === 'agent-proposal')
))
const contextPickerTitle = computed(() => (
  contextPickerOpen.value === 'notebook' ? 'ノートブックを追加' : 'ノートを追加'
))
const filteredCatalogNotes = computed(() => {
  const query = contextQuery.value.trim().toLocaleLowerCase()
  const selectedIDs = new Set(
    chatStore.explicitContexts
      .filter((context) => context.kind === 'note')
      .map((context) => context.id),
  )
  return chatStore.catalogNotes
    .filter((item) => (
      item.id !== noteStore.activeNote?.id
      && !selectedIDs.has(item.id)
      && (!query || item.title.toLocaleLowerCase().includes(query))
    ))
    .slice(0, 100)
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
  closeContextPicker()
  emit('update:open', false)
  emit('closed')
}

function focusComposer() {
  composerTextarea.value?.focus()
}

function openAISettings() {
  settingsStore.openSettings('ai')
}

function toggleRecords() {
  if (isAnyBusy.value) return
  recordsOpen.value = !recordsOpen.value
  if (!recordsOpen.value) void nextTick(focusComposer)
}

function timelineRoleLabel(entry: AIChatTimelineEntry) {
  if (entry.role === 'user') return 'あなた'
  if (entry.role === 'tool') return entry.tool
    ? toolDefinitions.find((item) => item.value === entry.tool)?.label ?? 'ツール'
    : 'ツール'
  if (entry.kind === 'error') return 'エラー'
  return 'Atlas AI'
}

function timelineStatusLabel(status: NonNullable<AIChatTimelineEntry['status']>) {
  if (status === 'pending') return '実行中'
  if (status === 'success') return '完了'
  return '失敗'
}

function safeCitationURL(value: string) {
  try {
    const parsed = new URL(value)
    return (
      parsed.protocol === 'https:'
      && !isUnsafeCitationHostname(parsed.hostname)
    )
      ? parsed.toString()
      : null
  } catch {
    return null
  }
}

function isUnsafeCitationHostname(value: string) {
  const hostname = value
    .trim()
    .toLocaleLowerCase()
    .replace(/^\[|\]$/g, '')
  if (
    !hostname
    || !hostname.includes('.')
    || hostname === 'localhost'
    || hostname.endsWith('.localhost')
    || hostname.endsWith('.local')
    || hostname.endsWith('.lan')
    || hostname.endsWith('.internal')
    || hostname.endsWith('.home')
    || hostname.endsWith('.home.arpa')
  ) return true
  if (/^(?:\d{1,3}\.){3}\d{1,3}$/.test(hostname)) return true
  return hostname.includes(':')
}

function citationHost(value: string) {
  try {
    return new URL(value).hostname
  } catch {
    return '参照元'
  }
}

function canSaveAssistantEntry(entry: AIChatTimelineEntry) {
  const latestEntry = chatStore.timeline[chatStore.timeline.length - 1]
  return (
    entry.role === 'assistant'
    && assistantStore.state === 'success'
    && latestEntry?.id === entry.id
  )
}

async function saveConversation() {
  const noteTitle = noteStore.activeNote?.title?.trim() || '無題のノート'
  await assistantStore.save(`${noteTitle}との会話 ${new Date().toLocaleString('ja-JP')}`)
}

async function applyAgentProposal(entryID: string) {
  const entry = chatStore.timeline.find((item) => item.id === entryID)
  const proposal = entry?.proposal
  if (
    !proposal
    || entry?.kind !== 'agent-proposal'
    || (entry.proposalState !== 'awaiting-review' && entry.proposalState !== 'save-failure')
    || isAnyBusy.value
  ) return

  if (!window.confirm(
    `次のAgent変更提案を本文へ適用します。\n\n対象: ${proposal.targetTitle || '無題のノート'}\nrevision: ${proposal.baseRevision}\n変更箇所: 本文\n\n通常のノート保存およびWebDAV同期の対象になります。適用後は元に戻す操作で取り消してください。`,
  )) return

  chatStore.setAgentProposalState(entryID, 'applying')
  try {
    const outcome = await noteStore.applyAgentEditProposal(proposal)
    if (outcome === 'applied') {
      chatStore.setAgentProposalState(entryID, 'applied', '変更提案を本文へ適用しました。')
    } else if (outcome === 'conflict') {
      chatStore.setAgentProposalState(entryID, 'conflict', '対象ノートまたは本文が更新されたため、変更提案を適用できませんでした。内容を確認して再生成してください。')
    } else {
      chatStore.setAgentProposalState(entryID, 'save-failure', '本文の保存に失敗しました。ノートを確認してから再試行してください。')
    }
  } catch {
    chatStore.setAgentProposalState(entryID, 'save-failure', '本文の保存に失敗しました。ノートを確認してから再試行してください。')
  }
  await nextTick()
  scrollTimelineToEnd()
}

function discardAgentProposal(entryID: string) {
  chatStore.discardAgentProposal(entryID)
}

async function openContextPicker(kind: ContextPickerKind) {
  contextPickerOpen.value = kind
  contextQuery.value = ''
  const catalogLoaded = await chatStore.loadContextCatalog()
  if (kind === 'notebook' && notebookStore.notebooks.length === 0) {
    await notebookStore.fetchNotebooks()
  }
  await nextTick()
  if (kind === 'note' && catalogLoaded) {
    contextSearchInput.value?.focus()
    return
  }
  if (kind === 'notebook') {
    contextOptions.value
      ?.querySelector<HTMLButtonElement>('button:not(:disabled)')
      ?.focus()
  }
}

function closeContextPicker(restoreTriggerFocus = false) {
  contextPickerOpen.value = null
  contextQuery.value = ''
  if (restoreTriggerFocus) {
    void nextTick(() => contextMenuTrigger.value?.focus())
  }
}

function addNoteContext(noteID: string, title: string) {
  chatStore.addNoteContext(noteID, title)
  void nextTick(() => contextSearchInput.value?.focus())
}

function addNotebookContext(notebookID: string, name: string) {
  if (!chatStore.addNotebookContext(notebookID, name)) return
  closeContextPicker()
  void nextTick(focusComposer)
}

function selectTool(tool: AIChatTool) {
  chatStore.selectTool(tool)
  closeContextPicker()
  void nextTick(focusComposer)
}

function selectWritingTool(kind: WritingKind) {
  selectedWritingKind.value = kind
  selectTool('writing')
}

function notebookContextDetail(notebookID: string) {
  const resolved = chatStore.notebookResolvedCounts[notebookID] ?? 0
  const omitted = chatStore.notebookOmissions[notebookID] ?? 0
  return omitted > 0 ? `${resolved}件・${omitted}件省略` : `${resolved}件`
}

function handleCompositionEnd() {
  window.setTimeout(() => {
    isComposing.value = false
  }, 0)
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing || isComposing.value) return
  event.preventDefault()
  void submitComposer()
}

function handlePanelKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (contextPickerOpen.value) {
    event.stopPropagation()
    closeContextPicker(true)
    return
  }
  if (event.target === composerTextarea.value) return
  if (event.target instanceof HTMLElement && event.target.closest('[role="menu"]')) return
  closeWorkspace()
}

function userSubmissionLabel(prompt: string, tool: AIChatTool | null) {
  if (!tool) return prompt
  const label = tool === chatStore.selectedTool
    ? selectedToolLabel.value
    : toolDefinitions.find((item) => item.value === tool)?.label ?? 'ツール'
  if (fixedScopeTools.has(tool)) return `${label}を実行`
  return prompt ? `${label}: ${prompt}` : `${label}を実行`
}

function showResultAfterTrace(tool: AIChatTool | null, traceID: string | null) {
  if (!traceID) return
  if (tool === 'summary') visibleSummaryTraceID.value = traceID
  if (tool === 'writing') visibleWritingTraceID.value = traceID
  if (tool && librarianToolMap[tool]) visibleLibrarianTraceID.value = traceID
}

function clearResultForTrace(traceID: string | null) {
  if (!traceID) return
  if (visibleSummaryTraceID.value === traceID) visibleSummaryTraceID.value = null
  if (visibleLibrarianTraceID.value === traceID) visibleLibrarianTraceID.value = null
  if (visibleWritingTraceID.value === traceID) visibleWritingTraceID.value = null
}

function clearResultAnchors() {
  visibleSummaryTraceID.value = null
  visibleLibrarianTraceID.value = null
  visibleWritingTraceID.value = null
}

async function submitComposer() {
  if (isSubmitting.value) return
  if (!canSubmitComposer.value) {
    if (!aiStore.configuredSetting?.modelID) openAISettings()
    return
  }

  isSubmitting.value = true
  try {
    await runComposerSubmission()
  } finally {
    isSubmitting.value = false
  }
}

async function runComposerSubmission() {
  closeContextPicker()
  const draftSnapshot = chatStore.draft
  const tool = chatStore.selectedTool
  const prompt = tool && fixedScopeTools.has(tool) ? '' : draftSnapshot.trim()
  const toolLabel = selectedToolLabel.value
  const userEntryID = chatStore.appendUserMessage(userSubmissionLabel(prompt, tool))
  const agentProposalEntryID = !tool && chatStore.mode === 'agent'
    ? chatStore.appendAgentProposalPlaceholder()
    : null
  const traceID = tool
    ? chatStore.appendToolTrace(tool, `${toolLabel}を準備しています。`)
    : null
  showResultAfterTrace(tool, traceID)
  if (traceID) await nextTick()

  let submitted = false
  let executionFailed = false
  if (tool === 'summary') {
    submitted = Boolean(await summaryPanel.value?.startSummary())
    executionFailed = !submitted && Boolean(aiStore.summaryError)
    if (traceID) {
      chatStore.updateTimelineEntry(traceID, {
        content: submitted ? '現在のノートを要約しました。' : aiStore.summaryError?.message ?? '要約を開始しませんでした。',
        status: submitted ? 'success' : 'error',
      })
    }
  } else if (tool === 'writing') {
    submitted = Boolean(
      await writingPanel.value?.submitPrompt(prompt, selectedWritingKind.value),
    )
    executionFailed = !submitted && Boolean(writingStore.error)
    if (traceID) {
      chatStore.updateTimelineEntry(traceID, {
        content: submitted
          ? `${toolLabel}を生成しました。内容を確認してください。`
          : writingStore.error?.message ?? '文章作成を開始しませんでした。',
        status: submitted ? 'success' : 'error',
      })
    }
    if (!submitted && writingStore.error) {
      chatStore.appendError(writingStore.error.message, tool)
    }
  } else if (tool && librarianToolMap[tool]) {
    activeLibrarianTraceID.value = traceID
    submitted = Boolean(
      await librarianPanel.value?.startOperation(librarianToolMap[tool]!),
    )
    executionFailed = !submitted && Boolean(librarianStore.error)
    if (!submitted && traceID) {
      chatStore.updateTimelineEntry(traceID, {
        content: librarianStore.error?.message ?? '候補生成を開始しませんでした。',
        status: 'error',
      })
      activeLibrarianTraceID.value = null
    }
  } else {
    submitted = Boolean(await assistantPanel.value?.submitPrompt(prompt))
    executionFailed = !submitted && Boolean(assistantStore.error)
    if (submitted) {
      const response = [...assistantStore.messages]
        .reverse()
        .find((message) => message.role === 'assistant')
      if (agentProposalEntryID) {
        chatStore.resolveAgentProposal(
          agentProposalEntryID,
          response?.content ?? '変更提案を生成できませんでした。',
          assistantStore.proposal,
        )
      } else if (response) {
        chatStore.appendAssistantMessage(response.content, assistantStore.citations)
      }
      if (traceID) {
        chatStore.updateTimelineEntry(traceID, {
          content: `OpenRouter Web Search（Exa）を${assistantStore.webSearchRequests}回実行して回答しました。`,
          status: 'success',
        })
      }
    } else if (assistantStore.error) {
      if (agentProposalEntryID) chatStore.removeTimelineEntry(agentProposalEntryID)
      chatStore.appendError(assistantStore.error.message, tool ?? undefined)
      if (traceID) {
        chatStore.updateTimelineEntry(traceID, {
          content: assistantStore.error.message,
          status: 'error',
        })
      }
    }
  }

  if (!submitted) {
    if (!executionFailed) {
      chatStore.removeTimelineEntry(userEntryID)
      if (traceID) chatStore.removeTimelineEntry(traceID)
      if (agentProposalEntryID) chatStore.removeTimelineEntry(agentProposalEntryID)
      clearResultForTrace(traceID)
    }
    return
  }

  if (!tool || !fixedScopeTools.has(tool)) {
    if (chatStore.draft === draftSnapshot) chatStore.setDraft('')
  }
  chatStore.selectTool(null)
  await nextTick()
  focusComposer()
  scrollTimelineToEnd()
}

async function openHistory(id: string) {
  if (isAnyBusy.value) return
  const panel = assistantPanel.value
  if (panel && await panel.openHistory(id)) {
    clearResultAnchors()
    chatStore.replaceTimelineFromConversation(assistantStore.messages, assistantStore.citations)
    recordsOpen.value = false
    await nextTick()
    scrollTimelineToEnd()
  }
}

async function openArtifact(id: string) {
  if (isAnyBusy.value) return
  chatStore.clearConversation({ keepContexts: true })
  clearResultAnchors()
  const traceID = chatStore.appendToolTrace(
    'writing',
    '保存済みのAI成果物を開いています。',
  )
  visibleWritingTraceID.value = traceID
  await nextTick()
  const panel = writingPanel.value
  if (panel && await panel.openArtifact(id)) {
    chatStore.updateTimelineEntry(traceID, {
      content: '保存済みのAI成果物を開きました。',
      status: 'success',
    })
    recordsOpen.value = false
    await nextTick()
    scrollTimelineToEnd()
    return
  }
  chatStore.removeTimelineEntry(traceID)
  clearResultForTrace(traceID)
}

async function openSummary(id: string) {
  if (isAnyBusy.value) return
  chatStore.clearConversation({ keepContexts: true })
  clearResultAnchors()
  const traceID = chatStore.appendToolTrace(
    'summary',
    '保存済みの要約を開いています。',
  )
  visibleSummaryTraceID.value = traceID
  await nextTick()
  if (await aiStore.loadSummaryHistory(id)) {
    chatStore.updateTimelineEntry(traceID, {
      content: '保存済みの要約を開きました。',
      status: 'success',
    })
    recordsOpen.value = false
    await nextTick()
    scrollTimelineToEnd()
    return
  }
  chatStore.removeTimelineEntry(traceID)
  clearResultForTrace(traceID)
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

function scrollTimelineToEnd() {
  const timeline = timelineRoot.value
  if (!timeline) return
  timeline.scrollTop = timeline.scrollHeight
}

watch(
  () => [noteStore.activeNote?.id ?? null, noteStore.activeNote?.title ?? ''] as const,
  ([noteID, title], previous) => {
    if (previous && previous[0] !== noteID) clearResultAnchors()
    chatStore.setActiveNoteContext(noteID ? { id: noteID, title } : null)
    recordsOpen.value = false
  },
  { immediate: true },
)

watch(
  () => [noteStore.activeNote?.id ?? null, noteStore.activeNote?.revision ?? null] as const,
  ([noteID, revision]) => {
    if (noteID && typeof revision === 'number') {
      chatStore.markAgentProposalStale(noteID, revision)
    }
  },
)

watch(
  () => librarianStore.state,
  (state) => {
    const traceID = activeLibrarianTraceID.value
    if (!traceID || state === 'generating' || state === 'partial' || state === 'canceling') return
    const successful = state === 'success' || state === 'empty'
    chatStore.updateTimelineEntry(traceID, {
      content: successful
        ? state === 'empty' ? '候補は見つかりませんでした。' : '候補を生成しました。内容を確認してください。'
        : librarianStore.error?.message ?? '候補生成を完了できませんでした。',
      status: successful ? 'success' : 'error',
    })
    activeLibrarianTraceID.value = null
    void nextTick(scrollTimelineToEnd)
  },
)

watch(
  () => chatStore.timeline.length,
  () => {
    void nextTick(scrollTimelineToEnd)
  },
)

watch(() => props.open, async (open) => {
  if (!open) {
    finishResize()
    closeContextPicker()
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
  min-width: 300px;
  min-height: 0;
  flex: 0 0 auto;
  flex-direction: column;
  border-left: 1px solid var(--border);
  background: var(--bg-sidebar);
  container-type: inline-size;
}

.ai-workspace.is-bottom .ai-workspace-panel {
  width: 100%;
  min-width: 0;
  min-height: 180px;
  border-top: 1px solid var(--border);
  border-left: none;
}

.ai-workspace-header {
  display: flex;
  min-height: 38px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 10px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg-sidebar) 94%, var(--brand-primary) 6%);
}

.ai-workspace-header-title,
.ai-workspace-header-actions,
.ai-chat-entry-heading,
.ai-chat-composer-toolbar {
  display: flex;
  align-items: center;
}

.ai-workspace-header-title {
  min-width: 0;
  gap: 7px;
  color: var(--text-primary);
  font-size: 13px;
}

.ai-workspace-logo,
.ai-chat-empty-icon {
  display: inline-grid;
  place-items: center;
  color: var(--brand-primary);
}

.ai-workspace-logo {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--brand-primary) 13%, transparent);
}

.ai-workspace-header-actions {
  flex-shrink: 0;
  gap: 4px;
}

.ai-workspace-icon-button {
  display: inline-grid;
  box-sizing: border-box;
  width: 28px;
  height: 28px;
  padding: 0;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
}

.ai-workspace-icon-button:hover,
.ai-workspace-icon-button:focus-visible,
.ai-workspace-icon-button[aria-pressed='true'] {
  border-color: var(--border);
  background: var(--bg-hover);
  color: var(--brand-primary);
}

.ai-workspace-icon-button:disabled {
  cursor: not-allowed;
  opacity: .45;
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
  transition: background-color .12s;
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

.ai-chat-timeline,
.ai-workspace-records {
  flex: 1;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.ai-chat-timeline {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 14px 22px;
  scroll-behavior: smooth;
}

.ai-chat-empty {
  display: grid;
  max-width: 290px;
  align-self: center;
  justify-items: center;
  gap: 8px;
  margin: auto 0;
  padding: 24px 14px;
  color: var(--text-secondary);
  text-align: center;
}

.ai-chat-empty strong {
  color: var(--text-primary);
  font-size: 14px;
}

.ai-chat-empty p {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
}

.ai-chat-empty-icon {
  width: 38px;
  height: 38px;
  border: 1px solid color-mix(in srgb, var(--brand-primary) 22%, var(--border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--brand-primary) 10%, transparent);
}

.ai-chat-entry {
  display: grid;
  gap: 7px;
  max-width: 92%;
  font-size: 13px;
}

.ai-chat-entry.is-user {
  align-self: flex-end;
  padding: 9px 11px;
  border: 1px solid color-mix(in srgb, var(--brand-primary) 22%, var(--border));
  border-radius: 12px 12px 3px;
  background: color-mix(in srgb, var(--brand-primary) 8%, var(--bg-input));
}

.ai-chat-entry.is-assistant {
  align-self: stretch;
  max-width: 100%;
}

.ai-chat-entry.is-tool-trace,
.ai-chat-entry.is-error {
  align-self: stretch;
  max-width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--bg-input) 86%, transparent);
}

.ai-chat-entry.is-error {
  border-color: color-mix(in srgb, var(--color-danger, #b42318) 30%, var(--border));
  color: var(--color-danger, #b42318);
}

.ai-chat-entry-heading {
  min-width: 0;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 11px;
}

.ai-chat-entry-heading strong {
  color: inherit;
}

.ai-chat-entry-avatar {
  display: inline-grid;
  width: 20px;
  height: 20px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 50%;
  background: var(--bg-hover);
  color: var(--brand-primary);
}

.ai-chat-entry-status {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  margin-left: auto;
}

.ai-chat-entry-content {
  margin: 0;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  line-height: 1.55;
}

.ai-chat-markdown {
  color: var(--text-primary);
}

.ai-chat-citation-block {
  display: grid;
  gap: 5px;
  color: var(--text-secondary);
  font-size: 11px;
}

.ai-chat-citations {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-chat-citations a,
.ai-chat-citations span {
  display: inline-block;
  max-width: 220px;
  padding: 3px 7px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-chat-inline-button {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 5px;
  padding: 4px 7px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: 11px;
}

.ai-chat-progress,
.ai-chat-state-warning {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--text-secondary);
  font-size: 12px;
}

.ai-chat-state-warning {
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--color-warning, #8a5a00) 30%, var(--border));
  border-radius: 7px;
  color: var(--color-warning, #8a5a00);
}

.is-spinning {
  animation: ai-chat-spin .8s linear infinite;
}

@keyframes ai-chat-spin {
  to { transform: rotate(360deg); }
}

.ai-chat-result-card {
  margin: 0 !important;
  border-radius: 9px !important;
}

.ai-workspace-records {
  padding-bottom: 12px;
}

.ai-chat-composer {
  position: relative;
  display: grid;
  flex: 0 0 auto;
  gap: 7px;
  padding: 8px 10px 10px;
  border-top: 1px solid var(--border);
  background: var(--bg-sidebar);
}

.ai-chat-contexts {
  display: flex;
  min-height: 22px;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
}

.ai-chat-context-chip {
  display: inline-flex;
  min-width: 0;
  max-width: 100%;
  min-height: 22px;
  align-items: center;
  gap: 4px;
  padding: 2px 5px 2px 7px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg-input);
  color: var(--text-secondary);
  font-size: 11px;
}

.ai-chat-context-chip.is-fixed {
  border-color: color-mix(in srgb, var(--brand-primary) 28%, var(--border));
  color: var(--text-primary);
}

.ai-chat-context-chip.is-tool {
  border-color: color-mix(in srgb, var(--brand-primary) 38%, var(--border));
  background: color-mix(in srgb, var(--brand-primary) 7%, var(--bg-input));
  color: var(--brand-primary);
}

.ai-chat-context-chip > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-chat-context-chip small {
  flex: 0 0 auto;
  color: var(--text-secondary);
}

.ai-chat-context-chip button {
  display: inline-grid;
  width: 17px;
  height: 17px;
  flex: 0 0 auto;
  padding: 0;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: inherit;
  cursor: pointer;
}

.ai-chat-context-chip button:hover,
.ai-chat-context-chip button:focus-visible {
  background: var(--bg-hover);
}

.ai-chat-context-chip button:disabled {
  cursor: not-allowed;
  opacity: .45;
}

.ai-chat-composer-error,
.ai-chat-composer-warning,
.ai-chat-submit-hint {
  margin: 0;
  font-size: 11px;
  line-height: 1.4;
}

.ai-chat-composer-error {
  color: var(--color-danger, #b42318);
}

.ai-chat-composer-warning {
  color: var(--color-warning, #8a5a00);
}

.ai-chat-submit-hint {
  color: var(--text-secondary);
}

.ai-chat-context-picker {
  position: absolute;
  z-index: 10;
  right: 10px;
  bottom: calc(100% - 2px);
  left: 10px;
  display: grid;
  max-height: min(320px, 55vh);
  gap: 8px;
  padding: 9px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg-editor, #fff);
  box-shadow: 0 12px 28px rgba(15, 23, 42, .18);
}

.ai-chat-context-picker-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
}

.ai-chat-context-search {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 7px;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-secondary);
}

.ai-chat-context-search input {
  width: 100%;
  min-width: 0;
  height: 30px;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  font-size: 12px;
}

.ai-chat-context-options {
  display: grid;
  min-height: 0;
  gap: 2px;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
}

.ai-chat-context-options button {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 7px;
  padding: 7px;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
}

.ai-chat-context-options button:hover,
.ai-chat-context-options button:focus-visible {
  background: var(--bg-hover);
}

.ai-chat-context-options button:disabled {
  cursor: not-allowed;
  opacity: .45;
}

.ai-chat-context-options button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-chat-picker-status,
.ai-chat-picker-empty {
  margin: 0;
  padding: 8px;
  color: var(--text-secondary);
  font-size: 11px;
}

.ai-chat-input-shell {
  display: grid;
  min-height: 94px;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: var(--bg-input);
  box-shadow: 0 1px 2px rgba(15, 23, 42, .05);
  transition: border-color .12s, box-shadow .12s;
}

.ai-chat-input-shell:focus-within {
  border-color: color-mix(in srgb, var(--brand-primary) 60%, var(--border));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--brand-primary) 12%, transparent);
}

.ai-chat-textarea {
  box-sizing: border-box;
  width: 100%;
  min-height: 58px;
  max-height: 180px;
  padding: 10px 11px 4px;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text-primary);
  font: inherit;
  line-height: 1.5;
}

.ai-chat-textarea:disabled {
  cursor: not-allowed;
  opacity: .65;
}

.ai-chat-textarea:read-only:not(:disabled) {
  cursor: default;
  color: var(--text-secondary);
}

.ai-chat-composer-toolbar {
  min-width: 0;
  gap: 4px;
  padding: 3px 5px 5px;
}

.ai-chat-toolbar-button,
.ai-chat-send-button {
  display: inline-flex;
  box-sizing: border-box;
  height: 28px;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 0 7px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: 11px;
}

.ai-chat-toolbar-button.is-icon {
  width: 28px;
  padding: 0;
}

.ai-chat-toolbar-button:hover,
.ai-chat-toolbar-button:focus-visible {
  border-color: var(--border);
  background: var(--bg-hover);
  color: var(--text-primary);
}

.ai-chat-toolbar-button:disabled {
  cursor: not-allowed;
  opacity: .5;
}

.ai-chat-toolbar-spacer {
  flex: 1;
}

.ai-chat-send-button {
  width: 30px;
  height: 30px;
  padding: 0;
  border-radius: 8px;
  background: var(--brand-primary);
  color: #fff;
}

.ai-chat-send-button:hover:not(:disabled),
.ai-chat-send-button:focus-visible:not(:disabled) {
  filter: brightness(.95);
}

.ai-chat-send-button:disabled {
  cursor: not-allowed;
  opacity: .42;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

:global(.ai-chat-menu) {
  z-index: 1100;
  min-width: 210px;
  padding: 5px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor, #fff);
  box-shadow: 0 10px 24px rgba(15, 23, 42, .18);
  color: var(--text-primary);
}

:global(.ai-chat-menu-label) {
  padding: 5px 7px;
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: .04em;
  text-transform: uppercase;
}

:global(.ai-chat-menu-item) {
  display: flex;
  min-height: 32px;
  align-items: center;
  gap: 8px;
  padding: 2px 7px;
  border-radius: 5px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.25;
  outline: none;
}

:global(.ai-chat-menu-item > span) {
  display: grid;
  flex: 1;
  gap: 1px;
}

:global(.ai-chat-menu-item small) {
  margin-left: auto;
  color: var(--text-secondary);
  font-size: 10px;
  font-weight: 400;
}

:global(.ai-chat-menu-item[data-highlighted]) {
  background: var(--bg-hover);
  color: var(--brand-primary);
}

:global(.ai-chat-menu-item[data-disabled]) {
  cursor: not-allowed;
  opacity: .45;
}

:global(.ai-chat-menu-separator) {
  height: 1px;
  margin: 5px 2px;
  background: var(--border);
}

:global(.ai-chat-menu-check) {
  margin-left: auto;
  color: var(--brand-primary);
}

:global(.ai-chat-menu-chevron) {
  margin-left: auto;
  color: var(--text-secondary);
}

@container (max-width: 420px) {
  .ai-chat-timeline {
    padding: 14px 10px 18px;
  }

  .ai-chat-entry {
    max-width: 96%;
  }

  .ai-chat-composer {
    padding-right: 8px;
    padding-left: 8px;
  }

  .ai-chat-toolbar-button span {
    display: none;
  }

  .ai-chat-toolbar-button {
    width: 28px;
    padding: 0;
  }

  .ai-chat-context-picker {
    right: 8px;
    left: 8px;
  }
}
</style>
