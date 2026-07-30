import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import path from 'node:path'

const rootDir = process.cwd()
const componentPath = (...parts) => path.join(rootDir, 'src', 'components', ...parts)

const [
  workspaceSource,
  editorSource,
  settingsStoreSource,
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
  readFile(componentPath('SettingsModal.vue'), 'utf8'),
  readFile(componentPath('AISummaryPanel.vue'), 'utf8'),
  readFile(componentPath('AILibrarianPanel.vue'), 'utf8'),
  readFile(componentPath('AIAssistantPanel.vue'), 'utf8'),
  readFile(componentPath('AIWritingPanel.vue'), 'utf8'),
  readFile(componentPath('AIRecordsPanel.vue'), 'utf8'),
  readFile(componentPath('AIMarkdownPreview.vue'), 'utf8'),
])

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
assert.doesNotMatch(workspaceSource, /MIN_WIDTH_FOR_RIGHT_WORKSPACE/)
assert.doesNotMatch(workspaceSource, /effectivePlacement/)
assert.doesNotMatch(workspaceSource, /ai-workspace-edge-tab/)
assert.match(workspaceSource, /type WorkspaceFeature = 'summary' \| 'librarian' \| 'assistant' \| 'writing' \| 'records'/)
for (const feature of ['summary', 'librarian', 'assistant', 'writing', 'records']) {
  assert.match(workspaceSource, new RegExp(`v-show="activeFeature === '${feature}'"`), `${feature} must remain mounted while inactive`)
}
assert.match(workspaceSource, /const activeFeature = ref<WorkspaceFeature>\('assistant'\)/)
assert.match(workspaceSource, /<AISummaryPanel ref="summaryPanel"\s*\/>/)
assert.match(workspaceSource, /<AILibrarianPanel ref="librarianPanel"\s*\/>/)
assert.match(workspaceSource, /<AIAssistantPanel ref="assistantPanel" external-composer\s*\/>/)
assert.match(workspaceSource, /<AIWritingPanel ref="writingPanel" external-composer\s*\/>/)
assert.match(workspaceSource, /<AIRecordsPanel @open-artifact="openArtifact" @open-history="openHistory" @open-summary="openSummary"\s*\/>/)
assert.match(workspaceSource, /DropdownMenuRoot/)
assert.match(workspaceSource, /class="ai-workspace-composer-feature-button"/)
assert.match(workspaceSource, /composerFeatureLabel/)
assert.match(workspaceSource, /composerFeatureIcon/)
assert.match(workspaceSource, /DropdownMenuSub/)
assert.match(workspaceSource, /AI司書/)
assert.match(workspaceSource, /:global\(\.ai-workspace-action-menu\)\s*\{/)
assert.match(workspaceSource, /background-color: var\(--bg-editor, #fff\)/)
assert.match(workspaceSource, /opacity: 1/)
assert.match(workspaceSource, /selectComposerFeature\('summary'\)/)
assert.match(workspaceSource, /selectLibrarianOperation\(operation\.value\)/)
assert.match(workspaceSource, /void summaryPanel\.value\?\.startSummary\(\)/)
assert.match(workspaceSource, /void librarianPanel\.value\?\.startOperation\(selectedLibrarianOperation\.value\)/)
assert.match(workspaceSource, /const composerStatus = computed/)
assert.match(workspaceSource, /v-model="composerText"/)
assert.match(workspaceSource, /:maxlength="composerMaximumLength"/)
assert.match(workspaceSource, /const composerMaximumLength = computed\(\(\) => activeFeature\.value === 'assistant' \? 8000 : 12_000\)/)
assert.match(workspaceSource, /@submit\.prevent="submitComposer"/)
assert.match(workspaceSource, /const modelButtonLabel = computed/)
assert.match(workspaceSource, /settingsStore\.openSettings\('ai'\)/)
assert.match(workspaceSource, /title="保存済みの履歴と成果物を開く"/)
assert.match(workspaceSource, /title="AIワークスペースを閉じる"/)
assert.match(workspaceSource, /<XIcon :size="16" aria-hidden="true" \/>/)
assert.match(workspaceSource, /activeFeature\.value = 'assistant'/)
assert.match(workspaceSource, /activeFeature\.value = 'writing'/)
assert.doesNotMatch(workspaceSource, /role="tablist"/)
assert.doesNotMatch(workspaceSource, /handleTabKeydown/)
assert.doesNotMatch(workspaceSource, /organizeMode/)
assert.doesNotMatch(workspaceSource, /composeMode/)
assert.doesNotMatch(workspaceSource, /overflow-x: auto/)

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

for (const [name, source] of [
  ['workspace', workspaceSource],
  ['summary', summarySource],
  ['librarian', librarianSource],
  ['assistant', assistantSource],
  ['writing', writingSource],
  ['records', recordsSource],
]) {
  assert.doesNotMatch(source, /localStorage/, `${name} must not persist AI content in localStorage`)
}

assert.match(librarianSource, /discardForNote/)
assert.match(librarianSource, /markStaleForRevision/)
assert.match(summarySource, /defineExpose\(\{ startSummary: handleAISummary \}\)/)
assert.match(librarianSource, /defineExpose\(\{ startOperation \}\)/)
assert.match(assistantSource, /externalComposer\?: boolean/)
assert.match(assistantSource, /v-if="!props\.externalComposer"/)
assert.match(assistantSource, /submitPrompt/)
assert.match(assistantSource, /defineExpose\(\{ openHistory, submitPrompt \}\)/)
assert.match(writingSource, /externalComposer\?: boolean/)
assert.match(writingSource, /v-if="!props\.externalComposer"/)
assert.match(writingSource, /submitPrompt/)
assert.match(writingSource, /defineExpose\(\{ openArtifact, submitPrompt \}\)/)
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
