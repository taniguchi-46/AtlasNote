import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const componentPath = (...parts) => path.join(rootDir, 'src', 'components', ...parts)

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
])

const workspaceTemplate = workspaceSource.slice(
  workspaceSource.indexOf('<template>'),
  workspaceSource.indexOf('<script setup'),
)

assert.match(settingsStoreSource, /export type AIWorkspacePlacement = 'right' \| 'bottom'/)
assert.match(settingsStoreSource, /readStringOption\('atlas-ai-workspace-placement', 'right'/)
assert.match(settingsStoreSource, /localStorage\.setItem\('atlas-ai-workspace-placement', newPlacement\)/)
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
assert.match(workspaceSource, /ask: new Set\(allowedToolValues\)/)
assert.match(workspaceSource, /agent: new Set\(allowedToolValues\)/)
assert.match(workspaceSource, /allowedToolsByMode\[chatStore\.mode\]\.has\(chatStore\.selectedTool\)/)
assert.match(workspaceSource, /\|\| !isSelectedToolAllowed\.value/)

// Ask/Agent are explicit modes and the selected value reaches the request harness.
assert.match(workspaceTemplate, /@select="chatStore\.setMode\('ask'\)"[\s\S]*?<strong>Ask<\/strong>/)
assert.match(workspaceTemplate, /@select="chatStore\.setMode\('agent'\)"[\s\S]*?<strong>Agent<\/strong>/)
assert.match(workspaceSource, /const modeLabel = computed\(\(\) => chatStore\.mode === 'agent' \? 'Agent' : 'Ask'\)/)
assert.match(assistantSource, /chatMode\?: AIChatMode/)
assert.match(assistantSource, /mode: props\.chatMode/)

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
assert.doesNotMatch(editorSource, /AIで要約/)
assert.doesNotMatch(editorSource, /<AILibrarianPanel/)
assert.doesNotMatch(editorSource, /<AIAssistantPanel/)
assert.doesNotMatch(editorSource, /<AIWritingPanel/)

// AI content and tool traces are in-memory only; localStorage remains layout-only.
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
assert.match(markdownPreviewSource, /Markdown\.configure\(RICH_MARKDOWN_OPTIONS\)/)
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

console.log('AI workspace tests passed')
