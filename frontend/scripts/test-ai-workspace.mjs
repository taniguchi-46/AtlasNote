import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import ts from 'typescript'

const rootDir = process.cwd()
const componentPath = (...parts) => path.join(rootDir, 'src', 'components', ...parts)
const agentProposalPermissionPath = path.join(rootDir, 'src', 'utils', 'agentProposalPermission.ts')
const workspaceTimelinePath = path.join(rootDir, 'src', 'utils', 'aiWorkspaceTimeline.ts')

const [
  workspaceSource,
  editorSource,
  settingsStoreSource,
  chatStoreSource,
  settingsModalSource,
  summarySource,
  librarianSource,
  assistantSource,
  writingSource,
  recordsSource,
  markdownPreviewSource,
  agentProposalCardSource,
  agentProposalPermissionSource,
  workspaceTimelineSource,
] = await Promise.all([
  readFile(componentPath('AIWorkspace.vue'), 'utf8'),
  readFile(componentPath('NoteEditor.vue'), 'utf8'),
  readFile(path.join(rootDir, 'src', 'stores', 'useSettingsStore.ts'), 'utf8'),
  readFile(path.join(rootDir, 'src', 'stores', 'useAIChatStore.ts'), 'utf8'),
  readFile(componentPath('SettingsModal.vue'), 'utf8'),
  readFile(componentPath('AISummaryPanel.vue'), 'utf8'),
  readFile(componentPath('AILibrarianPanel.vue'), 'utf8'),
  readFile(componentPath('AIAssistantPanel.vue'), 'utf8'),
  readFile(componentPath('AIWritingPanel.vue'), 'utf8'),
  readFile(componentPath('AIRecordsPanel.vue'), 'utf8'),
  readFile(componentPath('AIMarkdownPreview.vue'), 'utf8'),
  readFile(componentPath('AIAgentProposalCard.vue'), 'utf8'),
  readFile(agentProposalPermissionPath, 'utf8'),
  readFile(workspaceTimelinePath, 'utf8'),
])

const workspaceTemplate = workspaceSource.slice(
  workspaceSource.indexOf('<template>'),
  workspaceSource.indexOf('<script setup'),
)

assert.match(settingsStoreSource, /export type AIWorkspacePlacement = 'right' \| 'bottom'/)
assert.match(settingsStoreSource, /readStringOption\('atlas-ai-workspace-placement', 'right'/)
assert.match(settingsStoreSource, /localStorage\.setItem\('atlas-ai-workspace-placement', newPlacement\)/)
assert.match(settingsStoreSource, /export type AIAgentEditPermission = 'review-required' \| 'auto-update'/)
assert.match(settingsStoreSource, /'atlas-ai-agent-edit-permission',\s*'review-required'/)
assert.match(settingsStoreSource, /localStorage\.setItem\('atlas-ai-agent-edit-permission', newPermission\)/)
assert.match(settingsStoreSource, /aiAgentEditPermission,/)
assert.match(settingsStoreSource, /AI_WORKSPACE_RIGHT_WIDTH_MIN = 300/)
assert.match(settingsStoreSource, /AI_WORKSPACE_RIGHT_WIDTH_MAX = 960/)
assert.match(settingsStoreSource, /readClampedNumberInRange/)
assert.match(settingsStoreSource, /rawValue === null \|\| rawValue\.trim\(\) === ''/)
assert.match(settingsStoreSource, /'atlas-ai-workspace-right-width',\s*480,/)
assert.match(settingsStoreSource, /AI_WORKSPACE_BOTTOM_HEIGHT_MIN = 180/)
assert.match(settingsStoreSource, /AI_WORKSPACE_BOTTOM_HEIGHT_MAX = 760/)
assert.match(settingsStoreSource, /atlas-ai-workspace-right-width/)
assert.match(settingsStoreSource, /atlas-ai-workspace-bottom-height/)
assert.match(settingsStoreSource, /setAIWorkspaceRightWidth/)
assert.match(settingsStoreSource, /setAIWorkspaceBottomHeight/)
assert.match(settingsModalSource, /AIワークスペース/)
assert.match(settingsModalSource, /v-model="settingsStore\.aiWorkspacePlacement"/)
assert.match(settingsModalSource, /value="right">右側/)
assert.match(settingsModalSource, /value="bottom">下側/)
assert.match(settingsModalSource, /境界をドラッグして幅または高さを調整/)
assert.match(settingsModalSource, /Agentの本文編集権限/)
assert.match(settingsModalSource, /v-model="settingsStore\.aiAgentEditPermission"/)
assert.match(settingsModalSource, /value="review-required">提案のみ/)
assert.match(settingsModalSource, /value="auto-update">更新可能/)

// Right/bottom placement and pointer/keyboard resizing remain available.
assert.match(workspaceSource, /ResizeObserver/)
assert.match(workspaceSource, /defineProps<\{ open: boolean \}>/)
assert.match(workspaceSource, /role="separator"/)
assert.match(workspaceSource, /@pointerdown="startResize"/)
assert.match(workspaceSource, /@pointermove="handleResize"/)
assert.match(workspaceSource, /@pointerup="finishResize"/)
assert.match(workspaceSource, /setPointerCapture/)
assert.match(workspaceSource, /releasePointerCapture/)
assert.match(workspaceSource, /clientX/)
assert.match(workspaceSource, /clientY/)
assert.match(workspaceSource, /ArrowUp/)
assert.match(workspaceSource, /ArrowDown/)
assert.match(workspaceSource, /emit\('update:open', false\)/)
assert.match(workspaceSource, /v-show="isOpen"\s+class="ai-workspace-resizer"/)
assert.match(workspaceSource, /v-show="isOpen"\s+id="ai-workspace-panel"/)
assert.match(workspaceSource, /is-ai-workspace-resizing-right/)
assert.match(workspaceSource, /is-ai-workspace-resizing-bottom/)
assert.match(workspaceSource, /AI_WORKSPACE_RIGHT_RESPONSIVE_RATIO = 0\.6/)
assert.match(workspaceSource, /AI_WORKSPACE_BOTTOM_RESPONSIVE_RATIO = 0\.6/)
assert.match(workspaceSource, /const effectivePanelSize = computed\(\(\) => getEffectivePanelSize\(placement\.value\)\)/)
assert.match(workspaceSource, /const size = `\$\{effectivePanelSize\.value\}px`/)
assert.match(workspaceSource, /resizeStartSize = effectivePanelSize\.value/)
assert.doesNotMatch(workspaceSource, /normalizeWorkspacePanelSize/)
assert.match(workspaceSource, /container-type: inline-size/)
assert.match(workspaceSource, /\.ai-workspace-panel\s*\{[^}]*min-width: 300px/s)
assert.match(workspaceSource, /\.ai-workspace\.is-bottom \.ai-workspace-panel\s*\{[^}]*min-height: 180px/s)
assert.doesNotMatch(workspaceSource, /MIN_WIDTH_FOR_RIGHT_WORKSPACE/)
assert.doesNotMatch(workspaceSource, /effectivePlacement/)
assert.doesNotMatch(workspaceSource, /ai-workspace-edge-tab/)

// The main surface is one ordered chat timeline instead of feature switching.
assert.equal(
  (workspaceTemplate.match(/class="ai-chat-timeline"/g) ?? []).length,
  1,
  'AIWorkspace must render exactly one primary chat timeline',
)
assert.match(workspaceTemplate, /class="ai-chat-timeline"\s+role="log"/)
assert.match(workspaceTemplate, /v-for="entry in chatStore\.timeline"/)
assert.match(workspaceTemplate, /`is-\$\{entry\.role\}`/)
assert.match(workspaceTemplate, /`is-\$\{entry\.kind\}`/)
assert.match(workspaceTemplate, /entry\.status === 'pending'/)
assert.match(workspaceTemplate, /timelineStatusLabel\(entry\.status\)/)
assert.match(workspaceTemplate, /v-if="entry\.role === 'assistant'"/)
assert.match(workspaceTemplate, /:markdown="entry\.content"/)
const timelineLoopStart = workspaceTemplate.indexOf('<template v-for="entry in chatStore.timeline"')
const timelineLoopEnd = workspaceTemplate.indexOf('</template>', timelineLoopStart)
assert.ok(timelineLoopStart >= 0 && timelineLoopEnd > timelineLoopStart, 'timeline entries require one keyed render loop')
const timelineLoopSource = workspaceTemplate.slice(timelineLoopStart, timelineLoopEnd)
assert.match(timelineLoopSource, /<article[\s\S]*?<\/article>[\s\S]*?<AISummaryPanel/)
assert.match(timelineLoopSource, /<AIAgentProposalCard\s+v-if="entry\.kind === 'agent-proposal'"/)
assert.match(timelineLoopSource, /@apply="applyAgentProposal\(entry\.id\)"/)
assert.match(timelineLoopSource, /@discard="discardAgentProposal\(entry\.id\)"/)
assert.match(timelineLoopSource, /<AISummaryPanel\s+v-if="entry\.id === visibleSummaryTraceID"/)
assert.match(timelineLoopSource, /<AILibrarianPanel\s+v-if="entry\.id === visibleLibrarianTraceID"/)
assert.match(timelineLoopSource, /<AIWritingPanel\s+v-if="entry\.id === visibleWritingTraceID"/)
assert.match(timelineLoopSource, /:ref="setSummaryPanelRef"/)
assert.match(timelineLoopSource, /:ref="setLibrarianPanelRef"/)
assert.match(timelineLoopSource, /:ref="setWritingPanelRef"/)
assert.doesNotMatch(timelineLoopSource, /ref="(?:summaryPanel|librarianPanel|writingPanel)"/)
assert.ok(
  timelineLoopSource.indexOf('<article')
    < timelineLoopSource.indexOf('<AISummaryPanel')
    && timelineLoopSource.indexOf('<AISummaryPanel')
      < timelineLoopSource.indexOf('<AILibrarianPanel')
    && timelineLoopSource.indexOf('<AILibrarianPanel')
      < timelineLoopSource.indexOf('<AIWritingPanel'),
  'candidate cards must be anchored immediately after their matching tool-trace entry',
)
assert.doesNotMatch(workspaceSource, /type WorkspaceFeature/)
assert.doesNotMatch(workspaceSource, /activeFeature/)
assert.doesNotMatch(workspaceSource, /ai-workspace-composer-feature-button/)
assert.doesNotMatch(workspaceSource, /role="tablist"/)
assert.doesNotMatch(workspaceSource, /handleTabKeydown/)

// Existing AI capabilities render inline in the timeline and keep their adoption controls.
assert.match(workspaceTemplate, /<AISummaryPanel[\s\S]*?class="ai-chat-result-card"[\s\S]*?timeline/)
assert.match(workspaceTemplate, /<AILibrarianPanel[\s\S]*?class="ai-chat-result-card"[\s\S]*?timeline/)
assert.match(workspaceTemplate, /<AIWritingPanel[\s\S]*?class="ai-chat-result-card"[\s\S]*?external-composer[\s\S]*?timeline/)
assert.match(workspaceTemplate, /<AIAssistantPanel[\s\S]*?external-composer[\s\S]*?execution-bridge/)
assert.match(workspaceTemplate, /:additional-note-ids="additionalNoteIDs"/)
assert.match(workspaceTemplate, /:chat-mode="chatStore\.mode"/)
assert.match(workspaceTemplate, /:web-search="isWebSearchSelected"/)
assert.match(workspaceSource, /const additionalNoteIDs = computed\(\(\) => \(\s*chatStore\.resolvedNoteIDs\.filter/s)
assert.match(workspaceSource, /function showResultAfterTrace\(tool: AIChatTool \| null, traceID: string \| null\)/)
assert.match(workspaceSource, /if \(tool === 'summary'\) visibleSummaryTraceID\.value = traceID/)
assert.match(workspaceSource, /if \(tool === 'writing'\) visibleWritingTraceID\.value = traceID/)
assert.match(workspaceSource, /if \(tool && librarianToolMap\[tool\]\) visibleLibrarianTraceID\.value = traceID/)
assert.match(workspaceSource, /showResultAfterTrace\(tool, traceID\)/)
assert.match(workspaceSource, /const hasUnresolvedResultConflict = computed/)
assert.match(workspaceSource, /visibleSummaryTraceID\.value && aiStore\.summary/)
assert.match(workspaceSource, /visibleWritingTraceID\.value && writingStore\.content\.trim\(\)/)
assert.match(workspaceSource, /visibleLibrarianTraceID\.value[\s\S]*?librarianStore\.result\?\.candidates\.length/)
assert.match(workspaceSource, /\|\| hasUnresolvedResultConflict\.value/)
assert.match(workspaceSource, /表示中の候補を採用・破棄/)
assert.match(workspaceSource, /type TemplateRefValue = Element \| ComponentPublicInstance \| null/)
assert.match(workspaceSource, /function setSummaryPanelRef\(value: TemplateRefValue\)[\s\S]*?typeof panel\.startSummary === 'function'/)
assert.match(workspaceSource, /function setLibrarianPanelRef\(value: TemplateRefValue\)[\s\S]*?typeof panel\.startOperation === 'function'/)
assert.match(workspaceSource, /function setWritingPanelRef\(value: TemplateRefValue\)[\s\S]*?typeof panel\.openArtifact === 'function'[\s\S]*?typeof panel\.submitPrompt === 'function'/)
assert.match(assistantSource, /is-execution-bridge/)
assert.match(assistantSource, /\.ai-v3-panel\.is-execution-bridge\s*\{\s*display: none;/)

// The open note is a fixed context; only explicit note/notebook contexts are removable.
const fixedContextStart = workspaceTemplate.indexOf('class="ai-chat-context-chip is-fixed"')
const explicitContextStart = workspaceTemplate.indexOf('v-for="context in chatStore.explicitContexts"')
assert.ok(fixedContextStart >= 0, 'fixed active-note context chip is required')
assert.ok(explicitContextStart > fixedContextStart, 'explicit contexts must follow the fixed context')
const fixedContextSource = workspaceTemplate.slice(fixedContextStart, explicitContextStart)
assert.match(fixedContextSource, /LockKeyholeIcon/)
assert.match(fixedContextSource, /開いているノート。削除できません。/)
assert.doesNotMatch(fixedContextSource, /<button/)
assert.match(workspaceTemplate, /chatStore\.removeContext\(context\.kind, context\.id\)/)
assert.match(workspaceTemplate, /role="dialog"\s+:aria-label="contextPickerTitle"/)
assert.match(workspaceTemplate, /ref="contextOptions"/)
assert.match(workspaceTemplate, /@keydown\.esc\.stop="closeContextPicker\(true\)"/)
assert.match(workspaceSource, /querySelector<HTMLButtonElement>\('button:not\(:disabled\)'\)/)
assert.match(workspaceSource, /contextMenuTrigger\.value\?\.focus\(\)/)
const explicitContextSource = workspaceTemplate.slice(
  explicitContextStart,
  workspaceTemplate.indexOf('v-if="chatStore.selectedTool"', explicitContextStart),
)
const selectedToolContextSource = workspaceTemplate.slice(
  workspaceTemplate.indexOf('v-if="chatStore.selectedTool"', explicitContextStart),
  workspaceTemplate.indexOf('</div>', workspaceTemplate.indexOf('v-if="chatStore.selectedTool"', explicitContextStart)) + 6,
)
assert.match(
  explicitContextSource,
  /:disabled="isAnyBusy"[\s\S]*?chatStore\.removeContext/,
  'explicit contexts must not change while an AI run is active',
)
assert.match(
  selectedToolContextSource,
  /:disabled="isAnyBusy"[\s\S]*?chatStore\.selectTool\(null\)/,
  'the selected tool must not change while an AI run is active',
)
assert.match(workspaceSource, /chatStore\.setActiveNoteContext\(noteID \? \{ id: noteID, title \} : null\)/)
assert.match(workspaceSource, /noteStore\.isLoading/)
assert.match(workspaceSource, /!hasUsableNote\.value/)
assert.match(workspaceSource, /const hasBlockingDraft = computed\(\(\) => \(\s*noteStore\.activeDraft\?\.status === 'conflicted'\s*\|\| noteStore\.activeDraft\?\.status === 'failed'/s)
assert.match(workspaceSource, /\|\| hasBlockingDraft\.value/)

// The + menu exposes every context and tool requested by the chat contract.
assert.match(workspaceTemplate, /PlusIcon/)
assert.match(workspaceTemplate, /@select="openContextPicker\('note'\)"[\s\S]*?ノート/)
assert.match(workspaceTemplate, /@select="openContextPicker\('notebook'\)"[\s\S]*?ノートブック/)
for (const [tool, label] of [
  ['summary', '要約'],
  ['title', 'タイトル候補'],
  ['tags', 'タグ候補'],
  ['classification', '分類候補'],
  ['related', '関連メモ'],
  ['duplicate', '重複候補'],
  ['web-search', 'Web検索'],
]) {
  assert.match(
    workspaceTemplate,
    new RegExp(`@select="selectTool\\('${tool}'\\)"[\\s\\S]{0,180}${label}`),
    `${label} must be available from the + menu`,
  )
}
assert.match(workspaceTemplate, /<DropdownMenuSub>[\s\S]*?<DropdownMenuSubTrigger class="ai-chat-menu-item">[\s\S]*?文章作成/)
assert.match(workspaceTemplate, /v-for="writingKind in writingKinds"/)
assert.match(workspaceTemplate, /@select="selectWritingTool\(writingKind\.value\)"/)
for (const [kind, label] of [
  ['prompt', 'プロンプト'],
  ['prompt-improvement', 'プロンプト改善'],
  ['readme', 'README草案'],
  ['document', 'ドキュメント草案'],
  ['blog', 'ブログ草案'],
  ['requirements', '要件定義草案'],
]) {
  assert.match(
    workspaceSource,
    new RegExp(`\\{ value: '${kind}', label: '${label}' \\}`),
    `${label} must remain available from the writing submenu`,
  )
}
assert.match(workspaceSource, /\{ value: 'writing', label: '文章作成', icon: FilePenLineIcon \}/)
assert.match(workspaceSource, /const selectedWritingKind = ref<WritingKind>\('document'\)/)
assert.match(workspaceSource, /function selectWritingTool\(kind: WritingKind\)\s*\{\s*selectedWritingKind\.value = kind\s*selectTool\('writing'\)\s*\}/)
assert.match(workspaceSource, /chatStore\.selectedTool === 'writing'/)
assert.match(workspaceTemplate, /:disabled="!isWebSearchAvailable"[\s\S]*?@select="selectTool\('web-search'\)"/)
assert.match(workspaceSource, /configuredSetting\?\.providerID === 'openrouter'/)
assert.match(workspaceSource, /const allowedToolsByMode: Record<AIChatMode, ReadonlySet<AIChatTool>>/)
assert.doesNotMatch(workspaceSource, /const allowedToolValues = toolDefinitions\.map/)
for (const mode of ['ask', 'agent']) {
  assert.match(
    workspaceSource,
    new RegExp(`${mode}: new Set<AIChatTool>\\(\\[\\s*'summary',\\s*'writing',\\s*'title',\\s*'tags',\\s*'classification',\\s*'related',\\s*'duplicate',\\s*'web-search',\\s*\\]\\)`),
    `${mode} must use an explicit tool allowlist`,
  )
}
assert.match(workspaceSource, /allowedToolsByMode\[chatStore\.mode\]\.has\(chatStore\.selectedTool\)/)
assert.match(workspaceSource, /\|\| !isSelectedToolAllowed\.value/)
assert.match(workspaceSource, /const agentCapabilityUnavailableMessage = computed/)
assert.match(workspaceSource, /aiStore\.isAgentCapabilityUnavailable/)
assert.match(workspaceSource, /Agentの本文差分提案に対応していません/)

// Ask/Agent are explicit modes and the selected value reaches the request harness.
assert.match(workspaceTemplate, /@select="chatStore\.setMode\('ask'\)"[\s\S]*?<strong>Ask<\/strong>/)
assert.match(workspaceTemplate, /@select="chatStore\.setMode\('agent'\)"[\s\S]*?<strong>Agent<\/strong>/)
assert.match(workspaceSource, /const modeLabel = computed\(\(\) => chatStore\.mode === 'agent' \? 'Agent' : 'Ask'\)/)
assert.match(assistantSource, /chatMode\?: AIChatMode/)
assert.match(assistantSource, /mode: props\.chatMode/)
assert.match(assistantSource, /agentTarget/)
assert.match(assistantSource, /Agent変更提案/)
assert.match(assistantSource, /agentEditPermission: AIAgentEditPermission = 'review-required'/)
assert.match(assistantSource, /検証後に自動適用/)
assert.match(workspaceSource, /appendAgentProposalPlaceholder/)
assert.match(workspaceSource, /resolveAgentProposal/)
assert.match(workspaceSource, /applyAgentEditProposal/)
assert.match(workspaceSource, /async function persistAgentProposal\(entryID: string, automatic = false\)/)
assert.match(workspaceSource, /submitPrompt\(prompt, agentEditPermission\)/)
assert.match(
  workspaceSource,
  /const agentEditPermission = settingsStore\.aiAgentEditPermission[\s\S]*?runAgentProposalPermissionFlow\(\{[\s\S]*?permission: agentEditPermission,[\s\S]*?resolveProposal: \(entryID, content, proposal\) => \{\s*chatStore\.resolveAgentProposal\(entryID, content, proposal\)\s*\},/,
)
assert.match(workspaceSource, /autoApply: async \(entryID\) => persistAgentProposal\(entryID, true\)/)
assert.match(agentProposalPermissionSource, /permission === 'auto-update'/)
await testAgentProposalPermissionFlow(agentProposalPermissionSource)
assert.match(workspaceSource, /Agentが本文を更新しました。変更前後の差分を確認できます。/)
assert.match(
  workspaceSource,
  /recordAssistantTimelineFailure\(\{[\s\S]*?errorMessage: assistantStore\.error\.message,[\s\S]*?agentProposalEntryID,[\s\S]*?removeTimelineEntry:/,
)
assert.match(
  workspaceSource,
  /completeLibrarianTimelineTrace\(\{[\s\S]*?state,[\s\S]*?errorMessage: librarianStore\.error\?\.message,[\s\S]*?updateTimelineEntry:/,
)
assert.match(workspaceTimelineSource, /state === 'generating' \|\| state === 'partial' \|\| state === 'canceling'/)
assert.match(workspaceTimelineSource, /agentProposalEntryID\) removeTimelineEntry\(agentProposalEntryID\)/)
const persistAgentProposalStart = workspaceSource.indexOf('async function persistAgentProposal')
const persistAgentProposalEnd = workspaceSource.indexOf('\nfunction discardAgentProposal', persistAgentProposalStart)
assert.ok(
  persistAgentProposalStart >= 0 && persistAgentProposalEnd > persistAgentProposalStart,
  'the Agent proposal persistence boundary must remain inspectable',
)
const persistAgentProposalSource = workspaceSource.slice(
  persistAgentProposalStart,
  persistAgentProposalEnd,
)
assert.match(
  persistAgentProposalSource,
  /else \{[\s\S]*?setAgentProposalState\(\s*entryID,\s*'save-failure'/,
  'a failed save outcome must move the proposal card to save-failure',
)
assert.match(
  persistAgentProposalSource,
  /catch \{[\s\S]*?setAgentProposalState\(\s*entryID,\s*'save-failure'/,
  'a thrown save must move the proposal card to save-failure',
)
assert.match(
  workspaceSource,
  /outcome === 'applied-with-draft-conflict'[\s\S]*?setAgentProposalState\(\s*entryID,\s*'applied',[\s\S]*?適用中に入力されたローカル下書きを競合として保持しています。/,
)
assert.match(workspaceSource, /markAgentProposalStale/)
assert.match(workspaceSource, /window\.confirm\([\s\S]*?Agent変更提案/)
assert.match(workspaceSource, /const hasPendingAgentProposal = computed/)
assert.match(workspaceSource, /現在の変更提案を適用または破棄/)
assert.doesNotMatch(agentProposalCardSource, /v-html/)
assert.doesNotMatch(agentProposalCardSource, /<pre>/)
assert.match(agentProposalCardSource, /createAgentEditVisualDiff/)
assert.match(agentProposalCardSource, /本文の差分/)
assert.match(agentProposalCardSource, /role="region"/)
assert.match(agentProposalCardSource, /tabindex="0"/)
assert.match(agentProposalCardSource, /visualDiff\.beforeLines/)
assert.match(agentProposalCardSource, /visualDiff\.afterLines/)
assert.match(agentProposalCardSource, /is-removed/)
assert.match(agentProposalCardSource, /is-added/)
assert.match(agentProposalCardSource, /@container \(min-width: 520px\)/)

// The send button is the last control in the toolbar at the bottom-right inside the input shell.
const inputShellStart = workspaceTemplate.indexOf('<div class="ai-chat-input-shell"')
const inputShellEnd = workspaceTemplate.indexOf('<p v-if="submitBlockedMessage"', inputShellStart)
assert.ok(inputShellStart >= 0 && inputShellEnd > inputShellStart, 'chat input shell is required')
const inputShellSource = workspaceTemplate.slice(inputShellStart, inputShellEnd)
const textareaIndex = inputShellSource.indexOf('class="ai-chat-textarea"')
const toolbarIndex = inputShellSource.indexOf('class="ai-chat-composer-toolbar"')
const spacerIndex = inputShellSource.indexOf('class="ai-chat-toolbar-spacer"')
const sendIndex = inputShellSource.indexOf('class="ai-chat-send-button"')
assert.ok(
  textareaIndex >= 0
    && toolbarIndex > textareaIndex
    && spacerIndex > toolbarIndex
    && sendIndex > spacerIndex,
  'send button must be positioned at the bottom-right inside the input shell',
)
assert.match(inputShellSource, /type="submit"/)
assert.match(inputShellSource, /:disabled="!canSubmitComposer"/)
assert.match(
  inputShellSource,
  /v-if="assistantStore\.isBusy"[\s\S]*?type="button"[\s\S]*?title="AI処理を停止"[\s\S]*?aria-label="AI処理を停止"/,
  'Assistant context loading and generation must replace send with an accessible stop button',
)
assert.match(inputShellSource, /:disabled="assistantStore\.state === 'canceling'"/)
assert.match(inputShellSource, /@click="stopAssistant"[\s\S]*?<SquareIcon/)
assert.match(inputShellSource, /v-else[\s\S]*?type="submit"/)
assert.match(inputShellSource, /:maxlength="composerMaxLength"/)
assert.match(inputShellSource, /:readonly="isFixedScopeToolSelected"/)
assert.match(workspaceSource, /chatStore\.selectedTool === 'writing' \? 12000 : 8000/)
assert.match(workspaceSource, /const fixedScopeTools: ReadonlySet<AIChatTool> = new Set/)
for (const tool of ['summary', 'title', 'tags', 'classification', 'related', 'duplicate']) {
  assert.match(
    workspaceSource,
    new RegExp(`'${tool}'`),
    `${tool} must remain in the fixed-scope tool contract`,
  )
}
assert.match(workspaceSource, /入力文と追加コンテキストは使用しません/)
assert.match(workspaceSource, /if \(fixedScopeTools\.has\(tool\)\) return `\$\{label\}を実行`/)
assert.match(workspaceSource, /const prompt = tool && fixedScopeTools\.has\(tool\) \? '' : draftSnapshot\.trim\(\)/)
assert.match(workspaceSource, /const isSubmitting = ref\(false\)/)
assert.match(workspaceSource, /if \(isSubmitting\.value\) return/)
assert.match(workspaceSource, /isSubmitting\.value = true[\s\S]*?try \{[\s\S]*?await runComposerSubmission\(\)[\s\S]*?finally \{[\s\S]*?isSubmitting\.value = false/)
assert.match(workspaceSource, /if \(assistantStore\.state === 'canceling'\) return 'AI処理を停止しています…'/)
assert.match(workspaceSource, /assistantStore\.state === 'generating' && assistantStore\.error[\s\S]*?AI処理は継続しています。/)
assert.match(workspaceSource, /async function stopAssistant\(\) \{\s*if \(assistantStore\.state === 'canceling'\) return\s*await assistantStore\.cancel\(\)/)
assert.match(workspaceSource, /const draftSnapshot = chatStore\.draft/)
assert.match(workspaceSource, /if \(chatStore\.draft === draftSnapshot\) chatStore\.setDraft\(''\)/)
assert.match(workspaceTemplate, /title="保存済みの履歴と成果物を開く"[\s\S]*?:disabled="isAnyBusy"/)
assert.match(workspaceSource, /async function openHistory\(id: string\) \{\s*if \(isAnyBusy\.value\) return/)
assert.match(workspaceSource, /\.ai-chat-input-shell\s*\{[^}]*display: grid;[^}]*min-height: 94px;/s)
assert.match(workspaceSource, /\.ai-chat-composer-toolbar\s*\{[^}]*padding: 3px 5px 5px;/s)
assert.match(workspaceSource, /\.ai-chat-toolbar-spacer\s*\{\s*flex: 1;/)
assert.match(workspaceSource, /\.ai-chat-send-button\s*\{[^}]*width: 30px;[^}]*height: 30px;/s)
assert.match(workspaceSource, /@container \(max-width: 420px\)/)
assert.match(workspaceSource, /@container \(max-width: 420px\)[\s\S]*?\.ai-chat-toolbar-button span\s*\{\s*display: none;/)

// Web search is capability-gated and every external execution requires confirmation.
assert.match(assistantSource, /webSearch\?: boolean/)
assert.match(assistantSource, /props\.webSearch && setting\.providerID !== 'openrouter'/)
assert.match(assistantSource, /Web検索: 有効/)
assert.match(assistantSource, /外部送信/)
assert.match(assistantSource, /window\.confirm/)
assert.match(assistantSource, /webSearch: props\.webSearch/)
assert.match(assistantSource, /additionalNoteIDs\?: string\[\]/)
assert.match(assistantSource, /\.\.\.props\.additionalNoteIDs/)
assert.match(assistantSource, /\.slice\(0, 10\)/)
assert.match(assistantSource, /const asked = await assistantStore\.ask/)
assert.match(assistantSource, /if \(asked\) question\.value = ''/)
const confirmIndex = assistantSource.indexOf('if (!window.confirm(')
const askIndex = assistantSource.indexOf('const asked = await assistantStore.ask')
assert.ok(
  confirmIndex >= 0
    && askIndex > confirmIndex
    && assistantSource.slice(confirmIndex, askIndex).includes(')) return false'),
  'canceling the external-send confirmation must return before the provider request',
)
assert.match(workspaceTemplate, /safeCitationURL\(citation\.url\)/)
assert.match(workspaceTemplate, /rel="noreferrer noopener"/)
assert.match(workspaceSource, /parsed\.protocol === 'https:'/)
assert.match(workspaceSource, /!isUnsafeCitationHostname\(parsed\.hostname\)/)
assert.match(workspaceSource, /hostname\.endsWith\('\.localhost'\)/)
assert.match(workspaceSource, /hostname\.endsWith\('\.local'\)/)
assert.match(workspaceSource, /\^\(\?:\\d\{1,3\}\\\.\)\{3\}\\d\{1,3\}\$/)
assert.match(workspaceSource, /OpenRouter Web Search（Exa）を\$\{assistantStore\.webSearchRequests\}回実行/)
assert.match(workspaceTemplate, /v-if="assistantStateWarning"/)
assert.match(workspaceSource, /assistantStore\.state === 'orphaned'/)
assert.match(workspaceSource, /assistantStore\.state === 'stale'/)

assert.match(workspaceSource, /settingsStore\.openSettings\('ai'\)/)
assert.match(workspaceTemplate, /title="保存済みの履歴と成果物を開く"/)
assert.match(workspaceTemplate, /title="AIワークスペースを閉じる"/)
assert.match(workspaceTemplate, /<XIcon :size="16" aria-hidden="true" \/>/)

assert.match(editorSource, /v-model:open="isAIWorkspaceOpen"/)
assert.match(editorSource, /class="icon-btn ai-workspace-toggle"/)
assert.match(editorSource, /PanelRightOpenIcon/)
assert.match(editorSource, /PanelBottomOpenIcon/)
assert.ok(
  editorSource.indexOf('class="icon-btn ai-workspace-toggle"') < editorSource.indexOf('class="mode-segment"'),
  'AI workspace toggle must be placed to the left of the editor mode switcher',
)
assert.match(editorSource, /class="mode-segment"[\s\S]*?@click="toggleEditMode"/, 'the full mode control must be one native switch button')
assert.match(
  editorSource,
  /function toggleEditMode\(\)[\s\S]*?editMode\.value === 'markdown' \? 'wysiwyg' : 'markdown'/,
  'the mode control must always switch to the opposite editor mode',
)
assert.doesNotMatch(editorSource, /class="mode-segment-btn"/, 'individual mode halves must not be independently clickable')
assert.doesNotMatch(editorSource, /AIで要約/)
assert.doesNotMatch(editorSource, /<AILibrarianPanel/)
assert.doesNotMatch(editorSource, /<AIAssistantPanel/)
assert.doesNotMatch(editorSource, /<AIWritingPanel/)

// A successful same-note content save must refresh both editor modes without
// replacing an unresolved local draft or scheduling a duplicate save.
const activeNoteWatchAnchor = editorSource.indexOf('() => noteStore.activeNote')
const activeNoteWatchStart = editorSource.lastIndexOf('watch(', activeNoteWatchAnchor)
const saveFeedbackWatchAnchor = editorSource.indexOf(
  '() => noteStore.saveFeedbackVersion',
  activeNoteWatchAnchor + 1,
)
const activeNoteWatchEnd = editorSource.lastIndexOf('watch(', saveFeedbackWatchAnchor)
assert.ok(
  activeNoteWatchStart >= 0 && activeNoteWatchEnd > activeNoteWatchStart,
  'NoteEditor must watch the active note',
)
const activeNoteWatchSource = editorSource.slice(activeNoteWatchStart, activeNoteWatchEnd)
assert.match(activeNoteWatchSource, /if \(draft \|\| localMarkdown\.value === note\.content\) return/)
assert.match(activeNoteWatchSource, /localMarkdown\.value = note\.content/)
assert.match(activeNoteWatchSource, /isRichDirty\.value = false/)
assert.match(activeNoteWatchSource, /editMode\.value === 'wysiwyg'[\s\S]*?setEditorFromMarkdown\(note\.content\)/)
assert.doesNotMatch(activeNoteWatchSource, /editMode\.value === 'markdown'[\s\S]*?return/)
assert.doesNotMatch(activeNoteWatchSource, /scheduleAutoSave/)

// Agent-applied content is highlighted in the central editor without adding
// persistent markup to the Markdown source.
assert.match(editorSource, /Agent更新箇所/)
assert.match(editorSource, /aria-label="Agent更新箇所のハイライトを閉じる"/)
assert.match(editorSource, /createAgentEditorTextHighlight/)
assert.match(editorSource, /findChangedTopLevelBlockRange/)
assert.match(editorSource, /new Plugin<DecorationSet>/)
assert.match(editorSource, /Decoration\.node/)
assert.match(editorSource, /setMeta\(agentEditorHighlightPluginKey/)
assert.match(editorSource, /clearAgentEditorHighlight/)
assert.match(editorSource, /markdownHighlightLayer/)
assert.match(editorSource, /scrollTop = textarea\.scrollTop/)
assert.match(editorSource, /scrollLeft = textarea\.scrollLeft/)
assert.match(editorSource, /pointer-events: none/)
assert.match(editorSource, /agent-editor-highlight-block/)
assert.match(editorSource, /agent-editor-highlight-mark/)
assert.match(
  editorSource,
  /background: color-mix\(in srgb, var\(--bg-editor\) 95%, var\(--color-success\) 5%\)/,
)
assert.match(
  editorSource,
  /background: color-mix\(in srgb, var\(--bg-editor\) 94%, var\(--color-success\) 6%\)/,
)
assert.match(
  editorSource,
  /\.prose-editor :deep\(\.agent-editor-highlight-block\)::before\s*\{[^}]*position: absolute;[^}]*left: -10px;[^}]*width: 2px;/s,
)
assert.match(
  editorSource,
  /\.agent-editor-highlight-mark:not\(\.is-deletion\)::before\s*\{[^}]*position: absolute;[^}]*left: -10px;[^}]*width: 2px;/s,
)
assert.match(
  editorSource,
  /\.agent-editor-highlight-mark\.is-deletion::before\s*\{[^}]*position: absolute;[^}]*left: -10px;[^}]*width: 2px;/s,
)
assert.match(
  editorSource,
  /function setEditMode[\s\S]*?localMarkdown\.value !== noteStore\.activeNote\?\.content[\s\S]*?scheduleAutoSave/,
)
assert.doesNotMatch(editorSource, /v-html/)

// AI content and tool traces are in-memory only; only non-secret UI preferences persist locally.
for (const [name, source] of [
  ['workspace', workspaceSource],
  ['chat store', chatStoreSource],
  ['summary', summarySource],
  ['librarian', librarianSource],
  ['assistant', assistantSource],
  ['writing', writingSource],
  ['records', recordsSource],
]) {
  assert.doesNotMatch(source, /localStorage/, `${name} must not persist AI content in localStorage`)
}
assert.match(chatStoreSource, /const timeline = ref<AIChatTimelineEntry\[\]>\(\[\]\)/)
assert.match(chatStoreSource, /kind: 'tool-trace'/)
assert.match(chatStoreSource, /const MAX_CONTEXT_NOTE_IDS = 10/)

// Existing adoption, revision, history, and markdown safety contracts remain reachable.
assert.match(librarianSource, /discardForNote/)
assert.match(librarianSource, /markStaleForRevision/)
assert.match(summarySource, /defineExpose\(\{ startSummary: handleAISummary \}\)/)
assert.match(librarianSource, /defineExpose\(\{ startOperation \}\)/)
assert.match(assistantSource, /externalComposer\?: boolean/)
assert.match(assistantSource, /v-if="!props\.externalComposer"/)
assert.match(assistantSource, /submitPrompt/)
assert.match(assistantSource, /defineExpose\(\{ openHistory, submitPrompt \}\)/)
assert.match(writingSource, /externalComposer\?: boolean/)
assert.match(writingSource, /v-if="!props\.externalComposer/)
assert.match(writingSource, /async function submitPrompt\(prompt: string, requestedKind\?: WritingKind\)/)
assert.match(writingSource, /if \(requestedKind\) kind\.value = requestedKind/)
assert.match(writingSource, /defineExpose\(\{ openArtifact, submitPrompt \}\)/)
assert.match(workspaceSource, /submitPrompt: \(prompt: string, kind\?: WritingKind\) => Promise<boolean>/)
assert.match(workspaceSource, /tool === 'writing'/)
assert.match(workspaceSource, /writingPanel\.value\?\.submitPrompt\(prompt, selectedWritingKind\.value\)/)
assert.match(workspaceSource, /writingStore\.error\?\.message/)
assert.match(assistantSource, /@container \(max-width: 420px\)/)
assert.match(writingSource, /@container \(max-width: 420px\)/)
assert.doesNotMatch(assistantSource, /@media \(max-width: 720px\)/)
assert.doesNotMatch(writingSource, /@media \(max-width: 720px\)/)
assert.doesNotMatch(assistantSource, /ai-v3-records/)
assert.doesNotMatch(writingSource, /ai-v3-records/)
assert.match(summarySource, /aria-label="要約を生成"/)
assert.match(summarySource, /AIMarkdownPreview/)
assert.match(markdownPreviewSource, /Markdown\.configure\(AI_MARKDOWN_OPTIONS\)/)
assert.match(markdownPreviewSource, /const sanitizedHtml = sanitizeAIHtml\(html\)/)
assert.match(markdownPreviewSource, /setContent\(sanitizedHtml/)
assert.doesNotMatch(markdownPreviewSource, /setContent\(html/)
assert.doesNotMatch(markdownPreviewSource, /v-html/)
assert.match(markdownPreviewSource, /openOnClick: false/)
assert.match(markdownPreviewSource, /parseNoteLinkHref/)
assert.match(markdownPreviewSource, /ai-markdown-preview-fallback/)
assert.match(librarianSource, /<component :is="item\.icon" :size="15" aria-hidden="true" \/>/)
assert.match(assistantSource, /aria-label="質問を送信"/)
assert.match(assistantSource, /送信済み・応答待ち/)
assert.match(assistantSource, /AIMarkdownPreview/)
assert.match(writingSource, /aria-label="文章を生成"/)
assert.match(writingSource, /送信済み・文章を作成中/)
assert.match(writingSource, /新規ノートにする/)
assert.match(writingSource, /末尾に追記/)
assert.match(writingSource, /本文を置換/)
assert.match(recordsSource, /Trash2Icon/)
assert.doesNotMatch(summarySource, /overflow: auto/)
assert.doesNotMatch(librarianSource, /max-height: 120px/)
assert.doesNotMatch(librarianSource, /\.ai-librarian-partial\s*\{[^}]*overflow:/)
assert.doesNotMatch(assistantSource, /max-height: 300px/)
assert.doesNotMatch(assistantSource, /\.ai-v3-messages\s*\{[^}]*overflow:/)
assert.match(writingSource, /ref="resultTextarea"/)
assert.match(writingSource, /function resizeResultTextarea/)
assert.match(writingSource, /void nextTick\(resizeResultTextarea\)/)
assert.match(recordsSource, /refreshHistories/)
assert.match(recordsSource, /refreshArtifacts/)
assert.match(recordsSource, /removeAllHistories/)
assert.match(recordsSource, /removeAllArtifacts/)

testNativeKeyboardActionContracts()

console.log('AI workspace tests passed')

function testNativeKeyboardActionContracts() {
  // Enter/Space activation and focusability come from the native button element.
  // Keep each action on a button with an accessible name, a click handler, and
  // the relevant busy-state guard instead of introducing custom key handlers.
  assertNativeActionButton({
    source: workspaceSource,
    marker: 'title="AI処理を停止"',
    name: 'Assistant cancel',
    accessibleName: /aria-label="AI処理を停止"/,
    handler: /@click="stopAssistant"/,
    disabled: /:disabled="assistantStore\.state === 'canceling'"/,
  })
  assertNativeActionButton({
    source: librarianSource,
    marker: 'title="生成をキャンセル"',
    name: 'librarian cancel',
    accessibleName: /aria-label="生成をキャンセル"/,
    handler: /@click="librarianStore\.cancel\(\)"/,
  })
  assert.match(
    librarianSource,
    /v-if="librarianStore\.state !== 'canceling'"[\s\S]*?title="生成をキャンセル"/,
    'librarian cancel must not remain actionable while cancellation is pending',
  )

  assertNativeActionButton({
    source: librarianSource,
    marker: 'title="候補を採用"',
    name: 'librarian adoption',
    accessibleName: /aria-label="候補を採用"/,
    handler: /@click="adopt\(candidate\)"/,
    disabled: /:disabled="noteStore\.isSaving"/,
  })
  assertNativeActionButton({
    source: agentProposalCardSource,
    marker: "state === 'save-failure' ? '保存を再試行' : '本文へ適用'",
    name: 'Agent proposal adoption and retry',
    accessibleName: /保存を再試行'[\s\S]*?'本文へ適用/,
    handler: /@click="emit\('apply'\)"/,
    disabled: /:disabled="busy"/,
  })

  assertNativeActionButton({
    source: librarianSource,
    marker: 'title="候補を破棄"',
    name: 'librarian candidate discard',
    accessibleName: /aria-label="候補を破棄"/,
    handler: /@click="librarianStore\.removeCandidate\(candidate\)"/,
  })
  assertNativeActionButton({
    source: agentProposalCardSource,
    marker: '提案を破棄',
    name: 'Agent proposal discard',
    accessibleName: /提案を破棄/,
    handler: /@click="emit\('discard'\)"/,
    disabled: /:disabled="busy"/,
  })

  assertNativeActionButton({
    source: librarianSource,
    marker: 'title="現在の操作を再試行"',
    name: 'librarian retry',
    accessibleName: /aria-label="現在の操作を再試行"/,
    handler: /@click="retryCurrentOperation"/,
  })
}

function assertNativeActionButton({
  source,
  marker,
  name,
  accessibleName,
  handler,
  disabled = null,
}) {
  const markerIndex = source.indexOf(marker)
  assert.ok(markerIndex >= 0, `${name} marker must exist`)
  const buttonStart = source.lastIndexOf('<button', markerIndex)
  const buttonEnd = source.indexOf('</button>', markerIndex)
  assert.ok(buttonStart >= 0 && buttonEnd > markerIndex, `${name} must be rendered by a native button`)
  const buttonSource = source.slice(buttonStart, buttonEnd + '</button>'.length)
  assert.match(buttonSource, /<button\b/)
  assert.match(buttonSource, /type="button"/, `${name} must not submit an enclosing form`)
  assert.match(buttonSource, accessibleName, `${name} must have an accessible name`)
  assert.match(buttonSource, handler, `${name} must delegate native activation to its handler`)
  if (disabled) assert.match(buttonSource, disabled, `${name} must expose its busy-state guard`)
  assert.doesNotMatch(buttonSource, /tabindex="-1"/, `${name} must remain keyboard-focusable`)
  assert.doesNotMatch(buttonSource, /@key(?:down|up)/, `${name} must retain native Enter/Space semantics`)
}

async function testAgentProposalPermissionFlow(source) {
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: agentProposalPermissionPath,
  })
  const moduleURL = `data:text/javascript;base64,${Buffer.from(compiled.outputText, 'utf8').toString('base64')}`
  const { runAgentProposalPermissionFlow } = await import(moduleURL)
  const proposal = Object.freeze({
    targetNoteID: 'note-1',
    targetTitle: '対象ノート',
    baseRevision: 4,
    reason: '重複を減らすため',
    before: '変更前',
    after: '変更後',
    affectedFields: ['content'],
  })

  function createHarness({
    resultProposal = proposal,
    submitted = true,
    submitGate = null,
    autoApplyState = 'applied',
  } = {}) {
    const calls = { submit: [], resolve: [], autoApply: [] }
    const events = []
    const entry = {
      id: 'proposal-1',
      kind: 'agent-proposal',
      content: '生成中',
      proposal: null,
      proposalState: 'generating',
    }

    return {
      calls,
      events,
      entry,
      input(permission) {
        return {
          prompt: '本文を整理して',
          permission,
          entryID: entry.id,
          async submitPrompt(prompt, submittedPermission) {
            calls.submit.push({ prompt, permission: submittedPermission })
            if (submitGate) await submitGate
            return submitted
          },
          readResult: () => ({
            content: resultProposal ? '変更提案を生成しました。' : '変更提案を生成できませんでした。',
            proposal: resultProposal,
          }),
          resolveProposal(entryID, content, resolvedProposal) {
            calls.resolve.push(entryID)
            events.push('resolve')
            entry.content = content
            entry.proposal = resolvedProposal
            entry.kind = resolvedProposal ? 'agent-proposal' : 'message'
            entry.proposalState = resolvedProposal ? 'awaiting-review' : undefined
          },
          async autoApply(entryID) {
            calls.autoApply.push(entryID)
            events.push('auto-apply')
            entry.proposalState = autoApplyState
            entry.content = autoApplyState === 'save-failure'
              ? 'Agentによる本文の更新に失敗しました。'
              : 'Agentが本文を更新しました。'
          },
        }
      },
    }
  }

  const reviewRequired = createHarness()
  assert.equal(
    await runAgentProposalPermissionFlow(reviewRequired.input('review-required')),
    true,
  )
  assert.deepEqual(reviewRequired.calls.submit, [{
    prompt: '本文を整理して',
    permission: 'review-required',
  }])
  assert.equal(reviewRequired.calls.resolve.length, 1)
  assert.equal(reviewRequired.calls.autoApply.length, 0, 'review-required must not save automatically')
  assert.equal(reviewRequired.entry.proposalState, 'awaiting-review')

  const autoUpdate = createHarness()
  assert.equal(await runAgentProposalPermissionFlow(autoUpdate.input('auto-update')), true)
  assert.deepEqual(autoUpdate.events, ['resolve', 'auto-apply'])
  assert.equal(autoUpdate.calls.autoApply.length, 1, 'auto-update must save exactly once')
  assert.equal(autoUpdate.entry.proposalState, 'applied')
  assert.equal(autoUpdate.entry.proposal, proposal, 'the applied card must retain its proposal diff')

  let releaseSubmission
  const submissionGate = new Promise((resolve) => { releaseSubmission = resolve })
  let selectedPermission = 'review-required'
  const fixedPermission = createHarness({ submitGate: submissionGate })
  const pendingSubmission = runAgentProposalPermissionFlow(fixedPermission.input(selectedPermission))
  assert.equal(fixedPermission.calls.submit.length, 1, 'the deferred Agent request must have started')
  selectedPermission = 'auto-update'
  releaseSubmission()
  assert.equal(await pendingSubmission, true)
  assert.equal(selectedPermission, 'auto-update')
  assert.equal(
    fixedPermission.calls.autoApply.length,
    0,
    'a setting change while waiting must not replace the request-start permission',
  )

  const noProposal = createHarness({ resultProposal: null })
  assert.equal(await runAgentProposalPermissionFlow(noProposal.input('auto-update')), true)
  assert.equal(noProposal.calls.autoApply.length, 0, 'an empty Agent proposal must never save')
  assert.equal(noProposal.entry.kind, 'message')

  const rejectedSubmission = createHarness({ submitted: false })
  assert.equal(await runAgentProposalPermissionFlow(rejectedSubmission.input('auto-update')), false)
  assert.equal(rejectedSubmission.calls.resolve.length, 0)
  assert.equal(rejectedSubmission.calls.autoApply.length, 0, 'a rejected or failed submission must never save')

  const failedAutoUpdate = createHarness({ autoApplyState: 'save-failure' })
  assert.equal(await runAgentProposalPermissionFlow(failedAutoUpdate.input('auto-update')), true)
  assert.equal(failedAutoUpdate.calls.autoApply.length, 1)
  assert.equal(failedAutoUpdate.entry.proposalState, 'save-failure')
  assert.equal(
    failedAutoUpdate.entry.proposal,
    proposal,
    'an auto-update failure must retain the proposal card for review',
  )
}
