<template>
  <div class="app-root" :data-theme="appStore.theme">
    <StorageSpaceUnlockScreen
      v-if="startupStatus?.locked"
      :space-id="startupStatus.activeStorageSpace?.id ?? ''"
      :space-name="startupStatus.activeStorageSpace?.name ?? '保存空間'"
      @unlocked="handleStorageSpaceUnlocked"
    />
    <StorageLocationSetupScreen
      v-else-if="startupStatus?.phase === 'storage-recovery'"
      mode="recovery"
      :initial-status="startupStatus?.storageLocations"
      :initial-error="startupStatus?.storageLocationError"
      @completed="handleStorageLocationCompleted"
    />
    <StorageLocationSetupScreen
      v-else-if="startupStatus?.setupRequired || startupStatus?.phase === 'setup-required'"
      :initial-status="startupStatus?.storageLocations"
      @completed="handleStorageLocationCompleted"
    />
    <main
      v-else-if="startupStatus && !startupStatus.ready"
      class="startup-error-screen"
      aria-labelledby="startup-error-title"
    >
      <section class="startup-error-card">
        <p class="startup-error-eyebrow">Atlas Note</p>
        <h1 id="startup-error-title">Atlas Noteを起動できませんでした</h1>
        <p>{{ startupStatus.message || '起動に必要なデータを読み込めませんでした。' }}</p>
        <code v-if="startupStatus.dataDir">{{ startupStatus.dataDir }}</code>
        <p class="startup-error-note">データは変更されていません。アプリを再起動して再試行してください。</p>
      </section>
    </main>
    <template v-else>
    <AppTopBar
      ref="appTopBarRef"
      :is-always-on-top="isAlwaysOnTop"
      @sync="handleSync"
      @search="handleSearch"
      @new-note="handleNewNote"
      @import-notes="handleOpenNoteImport"
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
      <NoteEditor ref="noteEditorRef" />

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
    <NoteImportModal
      :open="isNoteImportOpen"
      @close="isNoteImportOpen = false"
      @completed="handleNoteImportCompleted"
    />
    <SettingsModal />
    <ContentUnlockDialog />
    <NotificationCenter />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, watchEffect } from 'vue'
import AppTopBar from './components/AppTopBar.vue'
import AppSidebar from './components/AppSidebar.vue'
import NoteList from './components/NoteList.vue'
import NoteEditor from './components/NoteEditor.vue'
import NoteImportModal from './components/NoteImportModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import NotificationCenter from './components/NotificationCenter.vue'
import StorageSpaceUnlockScreen from './components/StorageSpaceUnlockScreen.vue'
import StorageLocationSetupScreen from './components/StorageLocationSetupScreen.vue'
import ContentUnlockDialog from './components/ContentUnlockDialog.vue'
import {
  deleteMissingNote,
  getStartupStatus,
  reinspectRecovery,
  type StartupStatus,
} from './api/startup'
import { ToggleAlwaysOnTop } from '../wailsjs/go/main/App'
import { CancelClose, CompleteClose, RestartApp } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { useNoteStore } from './stores/useNoteStore'
import { useAppStore } from './stores/useAppStore'
import { useNotebookStore } from './stores/useNotebookStore'
import { useSearchStore, type SearchFilters } from './stores/useSearchStore'
import { useTagStore } from './stores/useTagStore'
import { useSyncStore } from './stores/useSyncStore'
import { useAIStore } from './stores/useAIStore'
import { useAIAssistantStore } from './stores/useAIAssistantStore'
import { useAILibrarianStore } from './stores/useAILibrarianStore'
import { useAIWritingStore } from './stores/useAIWritingStore'
import { useStorageSpaceStore } from './stores/useStorageSpaceStore'
import { useStorageLocationStore } from './stores/useStorageLocationStore'
import { useBackupStore } from './stores/useBackupStore'
import { useContentLockStore } from './stores/useContentLockStore'
import { useNoteImportStore } from './stores/useNoteImportStore'
import { useNoteExportStore } from './stores/useNoteExportStore'
import { useNotificationStore } from './stores/useNotificationStore'
import type { NoteImportResult } from './api/noteImport'
import { logOperationFailure } from './utils/operationLogger'
import { createContentLockAutoLock } from './utils/contentLockAutoLock'
import { prepareBackupOperation } from './services/backupLifecycle'
import { prepareStorageSpaceSwitch } from './services/storageSpaceSwitch'
import {
  findMatchingShortcutAction,
  type ShortcutActionId,
} from './utils/keyboardShortcuts'
import {
  EDITOR_WIDTH_MIN,
  NOTE_LIST_WIDTH_MAX,
  NOTE_LIST_WIDTH_MIN,
  SIDEBAR_WIDTH_MAX,
  SIDEBAR_WIDTH_MIN,
  useSettingsStore,
} from './stores/useSettingsStore'

type ResizablePane = 'sidebar' | 'noteList'
type AppTopBarExpose = { focusSearch: () => void }
type NoteEditorExpose = {
  toggleAIWorkspace: () => void
  toggleEditMode: () => void
}

const noteStore = useNoteStore()
const appStore = useAppStore()
const notebookStore = useNotebookStore()
const searchStore = useSearchStore()
const tagStore = useTagStore()
const syncStore = useSyncStore()
const aiStore = useAIStore()
const aiAssistantStore = useAIAssistantStore()
const aiLibrarianStore = useAILibrarianStore()
const aiWritingStore = useAIWritingStore()
const storageSpaceStore = useStorageSpaceStore()
const storageLocationStore = useStorageLocationStore()
const backupStore = useBackupStore()
const contentLockStore = useContentLockStore()
const noteImportStore = useNoteImportStore()
const noteExportStore = useNoteExportStore()
const notificationStore = useNotificationStore()
const settingsStore = useSettingsStore()

contentLockStore.setBeforeLock(() => noteStore.flushAllDirtyNotes())
syncStore.setBeforeSync(() => noteStore.flushAllDirtyNotes())
storageSpaceStore.setSwitchLifecycle(
  () => prepareStorageSpaceSwitch({
     isBackupBusy: () => backupStore.isBusy || backupStore.status?.pendingRestore === true,
    isSyncBusy: () => syncStore.isBusy,
    isAIBusy: () => (
      aiStore.isSettingsBusy
      || aiStore.isGenerating
      || aiAssistantStore.isBusy
      || aiLibrarianStore.isGenerating
      || aiWritingStore.isBusy
    ),
    isImportBusy: () => noteImportStore.isBusy,
    isExportBusy: () => noteExportStore.isBusy,
    suspendSync: () => syncStore.suspend(),
    resumeSync: () => syncStore.resume(),
    flushAllDirtyNotes: () => noteStore.flushAllDirtyNotes(),
    notify: (message, code) => notificationStore.notify(message, {
      kind: 'warning', source: 'storage-space', code,
    }),
  }),
  async () => {
    await RestartApp()
  },
)
storageLocationStore.setLifecycle(
  () => prepareStorageSpaceSwitch({
     isBackupBusy: () => backupStore.isBusy || backupStore.status?.pendingRestore === true,
    isSyncBusy: () => syncStore.isBusy,
    isAIBusy: () => (
      aiStore.isSettingsBusy
      || aiStore.isGenerating
      || aiAssistantStore.isBusy
      || aiLibrarianStore.isGenerating
      || aiWritingStore.isBusy
    ),
    isImportBusy: () => noteImportStore.isBusy,
    isExportBusy: () => noteExportStore.isBusy,
    suspendSync: () => syncStore.suspend(),
    resumeSync: () => syncStore.resume(),
    flushAllDirtyNotes: () => noteStore.flushAllDirtyNotes(),
    notify: (message, code) => notificationStore.notify(message, {
      kind: 'warning', source: 'storage-location', code,
    }),
  }),
  async () => {
    await RestartApp()
  },
)
backupStore.setLifecycle(
  () => prepareBackupOperation({
    isStorageSpaceBusy: () => storageSpaceStore.isBusy,
    isSyncBusy: () => syncStore.isBusy,
    isAIBusy: () => (
      aiStore.isSettingsBusy
      || aiStore.isGenerating
      || aiAssistantStore.isBusy
      || aiLibrarianStore.isGenerating
      || aiWritingStore.isBusy
    ),
    isImportBusy: () => noteImportStore.isBusy,
    isExportBusy: () => noteExportStore.isBusy,
    isContentLockBusy: () => contentLockStore.isBusy,
    suspendSync: () => syncStore.suspend(),
    resumeSync: () => syncStore.resume(),
    flushAllDirtyNotes: () => noteStore.flushAllDirtyNotes(),
    notify: (message, code) => notificationStore.notify(message, {
      kind: 'warning', source: 'backup', code,
    }),
  }),
  async () => {
    await RestartApp()
  },
)
const startupStatus = ref<StartupStatus | null>(null)
const isNoteImportOpen = ref(false)
const isRecoveryBusy = ref(false)
const recoveryError = ref('')
const isAlwaysOnTop = ref(localStorage.getItem('atlas-always-on-top') === 'true')
const appShellRef = ref<HTMLElement | null>(null)
const appTopBarRef = ref<AppTopBarExpose | null>(null)
const noteEditorRef = ref<NoteEditorExpose | null>(null)
const activeResize = ref<ResizablePane | null>(null)
let resizeObserver: ResizeObserver | null = null
let cancelBeforeCloseListener: (() => void) | null = null
let isHandlingBeforeClose = false
let resizeStartX = 0
let resizeStartWidth = 0

const contentLockAutoLock = createContentLockAutoLock(async (targets) => {
  const result = await contentLockStore.lockTargetsNow(targets)
  if (!result.error) return true
  notificationStore.notify(result.error.message, {
    kind: 'warning',
    source: 'content-lock',
    code: `CONTENT_LOCK_AUTO_LOCK_${result.error.code}`,
    dedupeKey: `content-lock:auto-lock:${result.error.code}`,
  })
  return false
})

function updateContentLockAutoLock() {
  contentLockAutoLock.update({
    minutes: settingsStore.contentLockAutoLockMinutes,
    locks: contentLockStore.locks,
    unlockedAt: contentLockStore.unlockedAt,
  })
}

function checkContentLockAutoLock() {
  void contentLockAutoLock.check()
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') checkContentLockAutoLock()
}

// Apply font family globally
watchEffect(() => {
  document.documentElement.style.setProperty('--font-family-base', settingsStore.fontFamily)
})

const searchFilters = computed<SearchFilters>(() => ({
  notebookId: notebookStore.activeNotebookId,
  includeTrashed: appStore.sidebarSection === 'trash' && !notebookStore.activeNotebookId,
}))

// TopBar actions
async function handleSync() {
  try {
    const result = await syncStore.refresh()
    if (!result.connection || result.status === 'disabled') {
      settingsStore.openSettings('sync')
      return
    }
    await syncStore.runSync({ forceRetry: true })
  } catch (_) {
    notificationStore.notify('同期を開始できませんでした', {
      kind: 'error',
      source: 'sync',
      code: 'SYNC_START_FAILED',
      retryable: true,
    })
  }
}

function handleSearch(query: string) {
  noteStore.clearTagFilter()
  void searchStore.search(query, searchFilters.value)
  if (!query.trim()) {
    void noteStore.fetchNotes([], null, appStore.sidebarSection === 'recent')
  }
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

function executeGlobalShortcut(actionId: ShortcutActionId) {
  switch (actionId) {
    case 'note.new':
      void handleNewNote()
      return true
    case 'search.focus':
      if (!appTopBarRef.value) return false
      appTopBarRef.value.focusSearch()
      return true
    case 'settings.open':
      handleOpenSettings()
      return true
    case 'sync.run':
      void handleSync()
      return true
    case 'note.import':
      handleOpenNoteImport()
      return true
    case 'window.toggleAlwaysOnTop':
      void handleToggleAlwaysOnTop()
      return true
    case 'theme.toggle':
      appStore.toggleTheme()
      return true
    case 'editor.toggleMode':
      if (!noteStore.activeNote || !noteEditorRef.value) return false
      noteEditorRef.value.toggleEditMode()
      return true
    case 'ai.toggleWorkspace':
      if (!noteStore.activeNote || !noteEditorRef.value) return false
      noteEditorRef.value.toggleAIWorkspace()
      return true
    case 'editor.undo':
    case 'editor.redo':
      return false
  }
}

function handleGlobalShortcut(event: KeyboardEvent) {
  if (event.defaultPrevented || event.repeat || event.isComposing) return
  if (event.getModifierState('AltGraph')) return
  if (startupStatus.value?.locked || (startupStatus.value && !startupStatus.value.ready)) return
  if (settingsStore.isSettingsOpen || isNoteImportOpen.value || contentLockStore.accessRequest) return

  const target = event.target
  if (
    target instanceof Element
    && target.closest('[data-shortcut-capture], [role="dialog"], [role="menu"], [role="listbox"]')
  ) return

  const actionId = findMatchingShortcutAction(event, settingsStore.shortcutBindings, 'app')
  if (!actionId || !executeGlobalShortcut(actionId)) return

  event.preventDefault()
  event.stopPropagation()
}

watch(
  [() => appStore.sidebarSection, () => notebookStore.activeNotebookId],
  () => {
    if (searchStore.isActive) void searchStore.search(searchStore.query, searchFilters.value)
  },
)

watch(() => noteStore.saveFeedbackVersion, () => {
  if (searchStore.isActive) void searchStore.refresh()
  syncStore.scheduleAutoSync()
})

watch(
  [
    () => settingsStore.contentLockAutoLockMinutes,
    () => contentLockStore.locks,
    () => contentLockStore.unlockedAt,
    () => contentLockStore.unlockVersion,
  ],
  updateContentLockAutoLock,
  { deep: true, immediate: true },
)

watch(() => contentLockStore.lastChangedTarget, async (target) => {
  if (!target) return
  try {
    await Promise.all([
      notebookStore.fetchNotebooks(),
      noteStore.fetchNotes([], noteStore.activeTagId, appStore.sidebarSection === 'recent'),
    ])
    await noteStore.refreshActiveNoteLockStatus()
    if (searchStore.isActive) await searchStore.refresh()
  } finally {
    contentLockStore.clearLastChangedTarget()
  }
})

async function handleLockedTargets(targets: { type: 'space' | 'notebook' | 'note'; id: string }[]) {
  if (targets.length === 0) return
  try {
    if (targets.some((target) => target.type === 'space')) {
      contentLockStore.cancelAccessRequest()
      noteStore.clearActiveNote()
      startupStatus.value = await getStartupStatus()
      if (!startupStatus.value.ready) return
    } else {
      await noteStore.refreshActiveNoteLockStatus()
    }
    await noteStore.fetchNotes([], noteStore.activeTagId, appStore.sidebarSection === 'recent')
    if (searchStore.isActive) await searchStore.refresh()
  } catch {
    notificationStore.notify('ロック後の表示を更新できませんでした。', {
      kind: 'warning', source: 'content-lock', code: 'CONTENT_LOCK_AFTER_LOCK_REFRESH_FAILED',
    })
  }
}

function handleOpenNoteImport() {
  isNoteImportOpen.value = true
  void notebookStore.fetchNotebooks()
}

async function handleNoteImportCompleted(result: NoteImportResult) {
  if (result.imported.length === 0) return

  try {
    await Promise.all([
      notebookStore.fetchNotebooks(),
      noteStore.fetchNotes([], noteStore.activeTagId, appStore.sidebarSection === 'recent'),
    ])
    if (searchStore.isActive) await searchStore.refresh()
  } catch {
    notificationStore.notify('インポート後の一覧を更新できませんでした。', {
      kind: 'warning', source: 'note-import', code: 'NOTE_IMPORT_REFRESH_FAILED',
    })
  } finally {
    syncStore.scheduleAutoSync()
  }

  if (!result.error && result.failures.length === 0) {
    isNoteImportOpen.value = false
    notificationStore.notify(`${result.imported.length}件のノートを取り込みました。`, {
      kind: 'success', source: 'note-import', code: 'NOTE_IMPORT_COMPLETED',
    })
    return
  }

  notificationStore.notify(`${result.imported.length}件のノートを取り込みました。一部のファイルは確認してください。`, {
    kind: 'warning', source: 'note-import', code: 'NOTE_IMPORT_PARTIAL',
  })
}

watch(() => contentLockStore.lastLockedTarget, async (target) => {
  if (!target) return
  try {
    await handleLockedTargets([target])
  } finally {
    contentLockStore.clearLastLockedTarget()
  }
})

watch(() => contentLockStore.lastLockedTargets, async (targets) => {
  if (!targets) return
  try {
    await handleLockedTargets(targets)
  } finally {
    contentLockStore.clearLastLockedTargets()
  }
})

function missingNoteIds(status: StartupStatus | null) {
  return status?.missingNotes.map((note) => note.id) ?? []
}

async function applyRecoveryStatus(status: StartupStatus) {
  startupStatus.value = status
  await noteStore.fetchNotes(missingNoteIds(status))
  if (searchStore.isActive) await searchStore.search(searchStore.query, searchFilters.value)
}

async function initializeReadyWorkspace(status: StartupStatus) {
  await noteStore.fetchNotes(missingNoteIds(status))
  await tagStore.fetchTags()
  await contentLockStore.refresh()
  if (status.syncRecoveryBackup) {
    notificationStore.notify('同期先からの復旧が完了し、以前のローカルデータをバックアップしました', {
      kind: 'success', source: 'sync', code: 'SYNC_RECOVERY_REDOWNLOAD_COMPLETED',
    })
  }
  await syncStore.initialize().catch(() => {})
  await aiStore.initialize().catch(() => {})
  await storageSpaceStore.initialize()
  if (status.backupRestoreSafetyBackupId) {
    notificationStore.notify('復元前の現在データを安全用バックアップとして保存しました', {
      kind: 'success', source: 'backup', code: 'BACKUP_RESTORE_SAFETY_CREATED',
    })
  }
  await backupStore.refreshStatus()
  backupStore.startScheduler()
  void backupStore.runAutomaticIfDue()
}

async function handleStorageSpaceUnlocked() {
  try {
    const status = await getStartupStatus()
    startupStatus.value = status
    if (status.ready) await initializeReadyWorkspace(status)
  } catch {
    notificationStore.notify('保存空間のロック状態を更新できませんでした', {
      kind: 'error', source: 'content-lock', code: 'CONTENT_LOCK_STARTUP_REFRESH_FAILED',
    })
  }
}

async function handleStorageLocationCompleted() {
  try {
    const status = await getStartupStatus()
    startupStatus.value = status
    if (status.ready) await initializeReadyWorkspace(status)
  } catch {
    notificationStore.notify('保存場所の設定後に起動状態を確認できませんでした', {
      kind: 'error', source: 'storage-location', code: 'STORAGE_LOCATION_STARTUP_REFRESH_FAILED',
    })
  }
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
  window.addEventListener('focus', checkContentLockAutoLock)
  window.addEventListener('keydown', handleGlobalShortcut, true)
  document.addEventListener('visibilitychange', handleVisibilityChange)
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
      await initializeReadyWorkspace(startupStatus.value)
    } else if (startupStatus.value.locked) {
      await storageSpaceStore.initialize()
    }
  } catch (_) {
    // Network or Wails not available (dev browser mode)
    await Promise.all([
      noteStore.fetchNotes().catch(() => {}),
      tagStore.fetchTags().catch(() => {}),
    ])
  }

  if (
    startupStatus.value?.ready !== true
    && !startupStatus.value?.setupRequired
    && startupStatus.value?.phase !== 'storage-recovery'
    && startupStatus.value?.phase !== 'error'
  ) await storageSpaceStore.initialize()

  // Apply initial always-on-top status
  try {
    await ToggleAlwaysOnTop(isAlwaysOnTop.value)
  } catch {
    logOperationFailure({ stage: 'wails.toggle-always-on-top', errorCategory: 'runtime' })
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  window.removeEventListener('focus', checkContentLockAutoLock)
  window.removeEventListener('keydown', handleGlobalShortcut, true)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  cancelBeforeCloseListener?.()
  resizeObserver?.disconnect()
  syncStore.dispose()
  backupStore.dispose()
  storageSpaceStore.clearSwitchLifecycle()
  storageLocationStore.clearLifecycle()
  aiStore.discardSummary()
  contentLockAutoLock.dispose()
  contentLockStore.cancelAccessRequest()
  document.body.classList.remove('is-pane-resizing')
})
</script>
