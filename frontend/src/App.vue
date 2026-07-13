<template>
  <div class="app-root" :data-theme="appStore.theme">
    <AppTopBar 
      :is-always-on-top="isAlwaysOnTop"
      @sync="handleSync"
      @search="handleSearch"
      @new-note="handleNewNote"
      @toggle-always-on-top="handleToggleAlwaysOnTop"
      @open-settings="handleOpenSettings"
    />

    <!-- Startup error banner -->
    <div
      v-if="startupStatus && !startupStatus.ready"
      class="startup-banner"
      role="alert"
    >
      <span>⚠ 起動エラー: {{ startupStatus.message }}</span>
      <span class="startup-datadir">{{ startupStatus.dataDir }}</span>
    </div>

    <section
      v-else-if="startupStatus?.degraded"
      class="recovery-banner"
      aria-labelledby="recovery-title"
    >
      <div class="recovery-header">
        <div>
          <strong id="recovery-title">一部のノート本文が見つかりません</strong>
          <p>正常なノートは引き続き利用できます。ファイルを元の場所へ戻してから再検査してください。</p>
        </div>
        <button type="button" class="recovery-reinspect" :disabled="isRecoveryBusy" @click="handleReinspectRecovery">
          {{ isRecoveryBusy ? '確認中…' : '再検査' }}
        </button>
      </div>
      <p v-if="recoveryError" class="recovery-error" role="alert">{{ recoveryError }}</p>
      <ul class="recovery-list">
        <li v-for="missing in startupStatus.missingNotes" :key="missing.id" class="recovery-item">
          <div class="recovery-note">
            <span class="recovery-title">{{ missing.title }}</span>
            <code>{{ missing.filePath }}</code>
          </div>
          <button type="button" class="recovery-delete" :disabled="isRecoveryBusy" @click="handleDeleteMissingNote(missing.id, missing.title)">
            DB情報を削除
          </button>
        </li>
      </ul>
    </section>

    <!-- 3-pane shell -->
    <div
      ref="appShellRef"
      class="app-shell"
      :style="{
        gridTemplateColumns: `${settingsStore.sidebarWidth}px ${settingsStore.noteListWidth}px minmax(0, 1fr)`,
      }"
    >
      <AppSidebar />
      <NoteList />
      <NoteEditor />

      <button
        class="pane-resizer"
        :class="{ 'is-resizing': activeResize === 'sidebar' }"
        :style="{ left: `${settingsStore.sidebarWidth}px` }"
        type="button"
        role="separator"
        aria-label="サイドバーの幅を調整"
        aria-orientation="vertical"
        :aria-valuemin="SIDEBAR_WIDTH_MIN"
        :aria-valuemax="SIDEBAR_WIDTH_MAX"
        :aria-valuenow="settingsStore.sidebarWidth"
        @keydown="handleResizerKeydown('sidebar', $event)"
        @pointerdown="startResize('sidebar', $event)"
        @pointermove="handleResize"
        @pointerup="finishResize"
        @pointercancel="finishResize"
      />
      <button
        class="pane-resizer"
        :class="{ 'is-resizing': activeResize === 'noteList' }"
        :style="{ left: `${settingsStore.sidebarWidth + settingsStore.noteListWidth}px` }"
        type="button"
        role="separator"
        aria-label="ノート一覧の幅を調整"
        aria-orientation="vertical"
        :aria-valuemin="NOTE_LIST_WIDTH_MIN"
        :aria-valuemax="NOTE_LIST_WIDTH_MAX"
        :aria-valuenow="settingsStore.noteListWidth"
        @keydown="handleResizerKeydown('noteList', $event)"
        @pointerdown="startResize('noteList', $event)"
        @pointermove="handleResize"
        @pointerup="finishResize"
        @pointercancel="finishResize"
      />
    </div>

    <!-- Modals -->
    <SettingsModal />
    <NotificationCenter />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, watchEffect } from 'vue'
import AppTopBar from './components/AppTopBar.vue'
import AppSidebar from './components/AppSidebar.vue'
import NoteList from './components/NoteList.vue'
import NoteEditor from './components/NoteEditor.vue'
import SettingsModal from './components/SettingsModal.vue'
import NotificationCenter from './components/NotificationCenter.vue'
import {
  deleteMissingNote,
  getStartupStatus,
  reinspectRecovery,
  type StartupStatus,
} from './api/startup'
import { ToggleAlwaysOnTop } from '../wailsjs/go/main/App'
import { CancelClose, CompleteClose } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'
import { useNotebookStore } from './stores/useNotebookStore'
import { useSearchStore, type SearchFilters } from './stores/useSearchStore'
import { useTagStore } from './stores/useTagStore'
import { logOperationFailure } from './utils/operationLogger'
import {
  EDITOR_WIDTH_MIN,
  NOTE_LIST_WIDTH_MAX,
  NOTE_LIST_WIDTH_MIN,
  SIDEBAR_WIDTH_MAX,
  SIDEBAR_WIDTH_MIN,
  useSettingsStore,
} from './stores/useSettingsStore'

type ResizablePane = 'sidebar' | 'noteList'

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const searchStore = useSearchStore()
const tagStore = useTagStore()
const settingsStore = useSettingsStore()
const startupStatus = ref<StartupStatus | null>(null)
const isRecoveryBusy = ref(false)
const recoveryError = ref('')
const isAlwaysOnTop = ref(localStorage.getItem('atlas-always-on-top') === 'true')
const appShellRef = ref<HTMLElement | null>(null)
const activeResize = ref<ResizablePane | null>(null)
let resizeObserver: ResizeObserver | null = null
let cancelBeforeCloseListener: (() => void) | null = null
let isHandlingBeforeClose = false
let resizeStartX = 0
let resizeStartWidth = 0

// Apply font family globally
watchEffect(() => {
  document.documentElement.style.setProperty('--font-family-base', settingsStore.fontFamily)
})

const searchFilters = computed<SearchFilters>(() => ({
  notebookId: notebookStore.activeNotebookId,
  includeTrashed: appStore.sidebarSection === 'trash' && !notebookStore.activeNotebookId,
}))

// TopBar actions
function handleSync() {
}

function handleSearch(query: string) {
  void searchStore.search(query, searchFilters.value)
}

async function handleNewNote() {
  await noteStore.newNote()
  if (searchStore.isActive) await searchStore.refresh()
}

async function handleToggleAlwaysOnTop() {
  isAlwaysOnTop.value = !isAlwaysOnTop.value
  localStorage.setItem('atlas-always-on-top', String(isAlwaysOnTop.value))
  try {
    await ToggleAlwaysOnTop(isAlwaysOnTop.value)
  } catch {
    logOperationFailure({ stage: 'wails.toggle-always-on-top', errorCategory: 'runtime' })
  }
}

function handleOpenSettings() {
  settingsStore.openSettings()
}

watch(
  [() => appStore.sidebarSection, () => notebookStore.activeNotebookId],
  () => {
    if (searchStore.isActive) void searchStore.search(searchStore.query, searchFilters.value)
  },
)

watch(() => noteStore.saveFeedbackVersion, () => {
  if (searchStore.isActive) void searchStore.refresh()
})

function missingNoteIds(status: StartupStatus | null) {
  return status?.missingNotes.map((note) => note.id) ?? []
}

async function applyRecoveryStatus(status: StartupStatus) {
  startupStatus.value = status
  await noteStore.fetchNotes(missingNoteIds(status))
  if (searchStore.isActive) await searchStore.search(searchStore.query, searchFilters.value)
}

async function handleReinspectRecovery() {
  isRecoveryBusy.value = true
  recoveryError.value = ''
  try {
    await applyRecoveryStatus(await reinspectRecovery())
  } catch (error) {
    recoveryError.value = error instanceof Error ? error.message : '再検査に失敗しました'
  } finally {
    isRecoveryBusy.value = false
  }
}

async function handleDeleteMissingNote(id: string, title: string) {
  const confirmed = window.confirm(
    `「${title}」のDB情報を削除します。Markdownファイルが復元されている場合は削除されません。続行しますか？`,
  )
  if (!confirmed) return

  isRecoveryBusy.value = true
  recoveryError.value = ''
  try {
    await applyRecoveryStatus(await deleteMissingNote(id))
  } catch (error) {
    recoveryError.value = error instanceof Error ? error.message : '欠落ノートの削除に失敗しました'
  } finally {
    isRecoveryBusy.value = false
  }
}

async function handleBeforeClose() {
  if (isHandlingBeforeClose) return
  isHandlingBeforeClose = true

  try {
    // ユーザーがウィンドウを閉じようとした際、Wails側のデフォルト終了処理をフックしてこの関数が呼ばれる。
    // 即座にアプリを終了させず、未保存のノート（dirty notes）をバックエンドに書き込む時間を確保する。
    // フラッシュに成功した場合は CompleteClose を呼んで実際にアプリを終了させる。
    if (await noteStore.flushAllDirtyNotes()) {
      await CompleteClose()
      return
    }

    const shouldRetry = window.confirm(
      '未保存の変更を保存できませんでした。再試行しますか？\nキャンセルするとアプリに戻ります。',
    )
    if (!shouldRetry) {
      await CancelClose()
      return
    }

    if (await noteStore.flushAllDirtyNotes()) {
      await CompleteClose()
      return
    }

    const shouldDiscard = window.confirm(
      '再試行しても保存できませんでした。未保存の変更をすべて破棄して終了しますか？',
    )
    if (shouldDiscard) {
      noteStore.discardAllDrafts()
      await CompleteClose()
      return
    }

    await CancelClose()
  } catch {
    logOperationFailure({ stage: 'app-close', errorCategory: 'flush-or-close' })
    await CancelClose().catch(() => {})
  } finally {
    isHandlingBeforeClose = false
  }
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!noteStore.hasDirtyNotes) return

  // ブラウザの再読み込みや強制終了時に、未保存のデータがある場合は警告ダイアログを表示する。
  // （Wailsのネイティブウィンドウだけでなく、ブラウザ開発時のタブ閉じに対応するための防波堤）
  event.preventDefault()
  event.returnValue = ''
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), Math.max(min, max))
}

function resizePane(pane: ResizablePane, requestedWidth: number) {
  const shellWidth = appShellRef.value?.clientWidth
  if (!shellWidth) return

  if (pane === 'sidebar') {
    const maxWidth = Math.min(
      SIDEBAR_WIDTH_MAX,
      shellWidth - settingsStore.noteListWidth - EDITOR_WIDTH_MIN,
    )
    settingsStore.setSidebarWidth(clamp(requestedWidth, SIDEBAR_WIDTH_MIN, maxWidth))
    return
  }

  const maxWidth = Math.min(
    NOTE_LIST_WIDTH_MAX,
    shellWidth - settingsStore.sidebarWidth - EDITOR_WIDTH_MIN,
  )
  settingsStore.setNoteListWidth(clamp(requestedWidth, NOTE_LIST_WIDTH_MIN, maxWidth))
}

function normalizePaneWidths() {
  const shellWidth = appShellRef.value?.clientWidth
  if (!shellWidth) return

  let sidebarWidth = settingsStore.sidebarWidth
  let noteListWidth = settingsStore.noteListWidth
  let overflow = sidebarWidth + noteListWidth + EDITOR_WIDTH_MIN - shellWidth

  if (overflow > 0) {
    const noteListReduction = Math.min(overflow, noteListWidth - NOTE_LIST_WIDTH_MIN)
    noteListWidth -= noteListReduction
    overflow -= noteListReduction
  }

  if (overflow > 0) {
    sidebarWidth -= Math.min(overflow, sidebarWidth - SIDEBAR_WIDTH_MIN)
  }

  settingsStore.setSidebarWidth(sidebarWidth)
  settingsStore.setNoteListWidth(noteListWidth)
}

function startResize(pane: ResizablePane, event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return

  activeResize.value = pane
  resizeStartX = event.clientX
  resizeStartWidth = pane === 'sidebar'
    ? settingsStore.sidebarWidth
    : settingsStore.noteListWidth

  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  document.body.classList.add('is-pane-resizing')
}

function handleResize(event: PointerEvent) {
  if (!activeResize.value) return
  resizePane(activeResize.value, resizeStartWidth + event.clientX - resizeStartX)
}

function finishResize(event: PointerEvent) {
  if (!activeResize.value) return

  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) {
    target.releasePointerCapture(event.pointerId)
  }
  activeResize.value = null
  document.body.classList.remove('is-pane-resizing')
}

function handleResizerKeydown(pane: ResizablePane, event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return

  event.preventDefault()
  const currentWidth = pane === 'sidebar'
    ? settingsStore.sidebarWidth
    : settingsStore.noteListWidth
  resizePane(pane, currentWidth + (event.key === 'ArrowLeft' ? -10 : 10))
}

onMounted(async () => {
  window.addEventListener('beforeunload', handleBeforeUnload)
  try {
    cancelBeforeCloseListener = EventsOn('app:before-close', () => {
      void handleBeforeClose()
    })
  } catch (_) {
    // Wails runtime is unavailable in browser-only development mode.
  }

  resizeObserver = new ResizeObserver(normalizePaneWidths)
  if (appShellRef.value) {
    resizeObserver.observe(appShellRef.value)
    normalizePaneWidths()
  }

  try {
    startupStatus.value = await getStartupStatus()
    if (startupStatus.value.ready) {
      await noteStore.fetchNotes(missingNoteIds(startupStatus.value))
      await tagStore.fetchTags()
    }
  } catch (_) {
    // Network or Wails not available (dev browser mode)
    await Promise.all([
      noteStore.fetchNotes().catch(() => {}),
      tagStore.fetchTags().catch(() => {}),
    ])
  }

  // Apply initial always-on-top status
  try {
    await ToggleAlwaysOnTop(isAlwaysOnTop.value)
  } catch {
    logOperationFailure({ stage: 'wails.toggle-always-on-top', errorCategory: 'runtime' })
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  cancelBeforeCloseListener?.()
  resizeObserver?.disconnect()
  document.body.classList.remove('is-pane-resizing')
})
</script>
