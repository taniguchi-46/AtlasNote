<template>
  <DialogRoot
    :open="settingsStore.isSettingsOpen"
    @update:open="handleOpenChange"
  >
    <DialogPortal>
      <DialogOverlay class="settings-modal-overlay" />
      <DialogContent class="settings-modal-content">
        <VisuallyHidden>
          <DialogDescription>アプリケーション設定を変更します</DialogDescription>
        </VisuallyHidden>
      <header class="settings-header">
        <DialogTitle as="h2">設定</DialogTitle>
        <div class="settings-search">
          <label for="settings-search-input" class="sr-only">設定を検索</label>
          <input
            id="settings-search-input"
            ref="settingsSearchInput"
            v-model="settingsQuery"
            type="search"
            placeholder="設定を検索"
            autocomplete="off"
            @keydown="handleSettingsSearchKeydown"
          />
          <div
            v-if="settingsQuery.trim()"
            class="settings-search-results"
            role="listbox"
            aria-label="設定の検索結果"
          >
            <button
              v-for="item in searchResults"
              :key="item.id"
              type="button"
              class="settings-search-result"
              data-settings-search-result
              role="option"
              @click="selectSearchResult(item)"
            >
              <strong>{{ item.label }}</strong>
              <span>{{ item.category }}</span>
            </button>
            <p v-if="searchResults.length === 0" class="settings-search-empty">
              該当する設定がありません。
            </p>
          </div>
        </div>
        <DialogClose as-child>
          <button class="icon-btn close-btn" title="閉じる" type="button">
            <XIcon :size="20" />
          </button>
        </DialogClose>
      </header>

      <TabsRoot v-model="activeTab" class="settings-body">
        <TabsList class="settings-sidebar" aria-label="設定カテゴリー">
          <TabsTrigger
            v-for="tab in tabs" 
            :key="tab.id"
            :value="tab.id"
            class="settings-tab"
          >
            {{ tab.name }}
          </TabsTrigger>
        </TabsList>

        <main class="settings-panel">
          <!-- テーマ設定 -->
          <TabsContent value="theme" as-child>
            <section data-settings-anchor="theme" tabindex="-1">
            <h3>テーマ</h3>
            <div class="setting-group">
              <label>アプリケーションテーマ</label>
              <select v-model="appStore.theme">
                <option value="light">ライト</option>
                <option value="dark">ダーク</option>
              </select>
            </div>
            </section>
          </TabsContent>

          <!-- 一般設定 -->
          <TabsContent value="general" as-child>
            <section data-settings-anchor="general" tabindex="-1">
            <h3>一般</h3>
            <div class="settings-section">
              <h4>ノートブック</h4>
              <div class="setting-group" data-settings-anchor="general.notebook-icon" tabindex="-1">
                <label>既定アイコン</label>
                <NotebookIconPicker
                  v-model="settingsStore.defaultNotebookIcon"
                  allow-user-icon-delete
                />
              </div>
            </div>
            <div class="settings-section">
              <h4>AIワークスペース</h4>
              <div class="setting-group" data-settings-anchor="general.ai-placement" tabindex="-1">
                <label for="ai-workspace-placement">表示位置</label>
                <select id="ai-workspace-placement" v-model="settingsStore.aiWorkspacePlacement">
                  <option value="right">右側</option>
                  <option value="bottom">下側</option>
                </select>
                <p class="setting-help">位置を選択し、表示中の境界をドラッグして幅または高さを調整します。</p>
              </div>
              <div class="setting-group" data-settings-anchor="general.ai-agent-permission" tabindex="-1">
                <label for="ai-agent-edit-permission">Agentの本文編集権限</label>
                <select id="ai-agent-edit-permission" v-model="settingsStore.aiAgentEditPermission">
                  <option value="review-required">提案のみ（適用前に確認）</option>
                  <option value="auto-update">更新可能（生成後に自動適用）</option>
                </select>
                <p class="setting-help">
                  更新可能では、送信した通常のAgent依頼が返した本文1箇所の差分を自動保存します。変更前後はAIタイムラインで確認できます。
                </p>
              </div>
            </div>
            <div class="settings-section" data-settings-anchor="general.uninstall" tabindex="-1">
              <h4>アンインストール</h4>
              <p class="setting-help">
                アプリの削除はWindowsの「インストールされているアプリ」から行います。ノートや保存空間などのユーザーデータは削除しません。必要なデータは先にバックアップしてください。
              </p>
              <button type="button" class="secondary-button" @click="handleOpenInstalledApps">
                Windowsのインストールされているアプリを開く
              </button>
              <p v-if="uninstallMessage" class="setting-help" role="status">{{ uninstallMessage }}</p>
            </div>
            </section>
          </TabsContent>

          <!-- エディター設定 -->
          <TabsContent value="editor" as-child>
            <section data-settings-anchor="editor" tabindex="-1">
            <h3>エディター</h3>
            <div class="settings-section">
              <h4>タイポグラフィ</h4>
              <div class="setting-group" data-settings-anchor="editor.font-family" tabindex="-1">
                <label>フォント指定</label>
                <select v-model="settingsStore.fontFamily">
                  <option value="Meiryo">Meiryo</option>
                  <option value="Yu Gothic UI">Yu Gothic UI</option>
                  <option value="Noto Sans JP">Noto Sans JP</option>
                  <option value="BIZ UDPGothic">BIZ UDPGothic</option>
                </select>
              </div>
              <div class="setting-group" data-settings-anchor="editor.font-size" tabindex="-1">
                <label>フォントサイズ指定</label>
                <select v-model="settingsStore.editorFontSize">
                  <option v-for="size in fontSizeOptions" :key="size" :value="size">
                    {{ size }}
                  </option>
                </select>
              </div>
            </div>

            <div class="settings-section">
              <h4>エディタ</h4>
              <div class="setting-group" data-settings-anchor="editor.first-line" tabindex="-1">
                <label>新規ノート1行目のスタイル</label>
                <select v-model="settingsStore.editorFirstLineStyle">
                  <option value="heading1">H1</option>
                  <option value="heading2">H2</option>
                  <option value="heading3">H3</option>
                  <option value="paragraph">普通</option>
                </select>
              </div>
              <div class="setting-group" data-settings-anchor="editor.line-length" tabindex="-1">
                <div class="setting-label-row">
                  <label>行の長さ</label>
                  <span>{{ settingsStore.editorLineLength }}</span>
                </div>
                <input
                  v-model.number="settingsStore.editorLineLength"
                  type="range"
                  min="520"
                  max="1200"
                  step="20"
                />
              </div>
              <div class="setting-group" data-settings-anchor="editor.line-height" tabindex="-1">
                <div class="setting-label-row">
                  <label>行間</label>
                  <span>{{ settingsStore.editorLineHeight.toFixed(1) }}</span>
                </div>
                <input
                  v-model.number="settingsStore.editorLineHeight"
                  type="range"
                  min="1.2"
                  max="2.4"
                  step="0.1"
                />
              </div>
              <div class="setting-group" data-settings-anchor="editor.paragraph-spacing" tabindex="-1">
                <div class="setting-label-row">
                  <label>段落の間隔</label>
                  <span>{{ settingsStore.editorParagraphSpacing.toFixed(1) }}</span>
                </div>
                <input
                  v-model.number="settingsStore.editorParagraphSpacing"
                  type="range"
                  min="0"
                  max="2"
                  step="0.1"
                />
              </div>
            </div>
            </section>
          </TabsContent>

          <TabsContent value="shortcuts" as-child>
            <ShortcutSettingsPanel />
          </TabsContent>

          <TabsContent value="sync" as-child>
            <SyncSettingsPanel />
          </TabsContent>

          <TabsContent value="ai" as-child>
            <AISettingsPanel />
          </TabsContent>

          <TabsContent value="storage-locations" as-child>
            <section class="storage-location-settings-tab" data-settings-anchor="storage-locations" tabindex="-1">
              <StorageLocationSettingsPanel />
              <div class="storage-space-settings-section">
                <StorageSpaceSettingsPanel />
              </div>
            </section>
          </TabsContent>

          <TabsContent value="backups" as-child>
            <BackupSettingsPanel />
          </TabsContent>

          <TabsContent value="locks" as-child>
            <ContentLockSettingsPanel />
          </TabsContent>

          <TabsContent value="help" as-child>
            <HelpSettingsPanel />
          </TabsContent>

        </main>
      </TabsRoot>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { XIcon } from '@lucide/vue'
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  TabsContent,
  TabsList,
  TabsRoot,
  TabsTrigger,
  VisuallyHidden,
} from 'reka-ui'
import { useSettingsStore } from '../stores/useSettingsStore'
import { useAppStore } from '../stores/useAppStore'
import type { SettingsTab } from '../stores/useSettingsStore'
import { useSyncStore } from '../stores/useSyncStore'
import { useAIStore } from '../stores/useAIStore'
import { useStorageSpaceStore } from '../stores/useStorageSpaceStore'
import { useStorageLocationStore } from '../stores/useStorageLocationStore'
import { useBackupStore } from '../stores/useBackupStore'
import { openInstalledApps } from '../api/startup'
import { searchSettings, type SettingsSearchItem } from '../utils/settingsSearch'
import NotebookIconPicker from './NotebookIconPicker.vue'
import SyncSettingsPanel from './SyncSettingsPanel.vue'
import AISettingsPanel from './AISettingsPanel.vue'
import StorageSpaceSettingsPanel from './StorageSpaceSettingsPanel.vue'
import StorageLocationSettingsPanel from './StorageLocationSettingsPanel.vue'
import BackupSettingsPanel from './BackupSettingsPanel.vue'
import ContentLockSettingsPanel from './ContentLockSettingsPanel.vue'
import ShortcutSettingsPanel from './ShortcutSettingsPanel.vue'
import HelpSettingsPanel from './HelpSettingsPanel.vue'

const settingsStore = useSettingsStore()
const appStore = useAppStore()
const syncStore = useSyncStore()
const aiStore = useAIStore()
const storageSpaceStore = useStorageSpaceStore()
const storageLocationStore = useStorageLocationStore()
const backupStore = useBackupStore()

const tabs: { id: SettingsTab; name: string }[] = [
  { id: 'theme', name: 'テーマ' },
  { id: 'general', name: '一般' },
  { id: 'editor', name: 'エディター' },
  { id: 'shortcuts', name: 'ショートカット' },
]
tabs.push({ id: 'sync', name: '同期' })
tabs.push({ id: 'ai', name: 'AI' })
tabs.push({ id: 'storage-locations', name: '保存場所' })
tabs.push({ id: 'backups', name: 'バックアップ' })
tabs.push({ id: 'locks', name: 'ロック' })
tabs.push({ id: 'help', name: 'ヘルプ' })
const activeTab = ref<SettingsTab>('theme')
const fontSizeOptions = [12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 26]
const settingsQuery = ref('')
const settingsSearchInput = ref<HTMLInputElement | null>(null)
const uninstallMessage = ref('')
const searchResults = computed(() => searchSettings(settingsQuery.value))

watch(
  () => settingsStore.isSettingsOpen,
  (open) => {
    if (!open) return
    activeTab.value = settingsStore.requestedTab
    settingsQuery.value = ''
    uninstallMessage.value = ''
    syncStore.resetDraft()
    aiStore.resetDraft()
    void storageSpaceStore.initialize()
    void storageLocationStore.initialize()
    void backupStore.initialize()
  },
)

watch(
  () => settingsStore.requestedTab,
  (requestedTab) => {
    if (settingsStore.isSettingsOpen) activeTab.value = requestedTab
  },
)

function handleSettingsSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    settingsQuery.value = ''
    return
  }
  if (event.key === 'Enter') {
    const firstResult = searchResults.value[0]
    if (!firstResult) return
    event.preventDefault()
    selectSearchResult(firstResult)
    return
  }
  if (event.key === 'ArrowDown' && searchResults.value.length > 0) {
    event.preventDefault()
    void nextTick(() => {
      document.querySelector<HTMLButtonElement>('[data-settings-search-result]')?.focus()
    })
  }
}

function selectSearchResult(item: SettingsSearchItem) {
  activeTab.value = item.tab
  settingsQuery.value = ''
  void nextTick(() => {
    const element = document.querySelector<HTMLElement>(
      `[data-settings-anchor="${item.anchor}"]`,
    )
    element?.scrollIntoView({ block: 'center' })
    element?.focus({ preventScroll: true })
  })
}

async function handleOpenInstalledApps() {
  uninstallMessage.value = ''
  try {
    await openInstalledApps()
    uninstallMessage.value = 'Windowsの設定を開きました。アプリ一覧からAtlas Noteを選んでください。'
  } catch {
    uninstallMessage.value = 'Windowsの設定を開けませんでした。Windowsの設定から「インストールされているアプリ」を開いてください。'
  }
}

function handleOpenChange(open: boolean) {
  if (open) {
    settingsStore.openSettings()
    return
  }

  syncStore.discardDraft()
  aiStore.discardDraft()
  settingsQuery.value = ''
  settingsStore.closeSettings()
}
</script>

<style scoped>
.settings-modal-overlay {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.5);
  z-index: 1000;
}

.settings-modal-content {
  position: fixed;
  top: 50%;
  left: 50%;
  z-index: 1001;
  transform: translate(-50%, -50%);
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  width: min(820px, calc(100vw - 32px));
  height: min(560px, calc(100vh - 48px));
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  overflow: hidden;
}

.settings-header {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
}

.settings-search {
  position: relative;
  flex: 1;
  max-width: 360px;
  margin-left: auto;
}

.settings-search input {
  width: 100%;
  box-sizing: border-box;
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  font: inherit;
  font-size: 13px;
}

.settings-search-results {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  left: 0;
  z-index: 2;
  display: grid;
  gap: 2px;
  max-height: 300px;
  overflow-y: auto;
  padding: 5px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-editor);
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.18);
}

.settings-search-result {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  padding: 8px 9px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
  text-align: left;
  cursor: pointer;
}

.settings-search-result:hover,
.settings-search-result:focus-visible {
  background: var(--bg-active);
  outline: none;
}

.settings-search-result span {
  flex-shrink: 0;
  color: var(--text-tertiary);
  font-size: 11px;
}

.settings-search-empty {
  margin: 4px 6px;
  color: var(--text-secondary);
  font-size: 12px;
}

.settings-header h2 {
  margin: 0;
  font-size: 1.2rem;
  color: var(--text-primary);
}

.settings-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.settings-sidebar {
  width: 200px;
  background-color: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 16px 0;
}

.settings-tab {
  padding: 12px 24px;
  text-align: left;
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
}

.settings-tab:hover {
  background-color: var(--bg-hover);
}

.settings-tab[data-state='active'] {
  background-color: var(--bg-active);
  color: var(--brand-primary);
  font-weight: 500;
  border-left: 3px solid var(--brand-primary);
}

.settings-panel {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background-color: var(--bg-editor);
}

.settings-panel > :deep(section > h3),
.storage-location-settings-tab :deep(section > h3) {
  margin-top: 0;
  margin-bottom: 24px;
  font-size: 1.1rem;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border);
  padding-bottom: 8px;
}

.storage-location-settings-tab {
  display: grid;
  gap: 32px;
}

.storage-space-settings-section {
  padding-top: 4px;
}

.settings-section {
  margin-bottom: 28px;
}

.settings-section h4 {
  margin: 0 0 16px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 600;
}

.secondary-button {
  padding: 7px 11px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.secondary-button:hover {
  border-color: var(--brand-primary);
  background: var(--bg-active);
}

.setting-group {
  margin-bottom: 24px;
  color: var(--text-primary);
}

.setting-group label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.setting-help {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.setting-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 240px;
  margin-bottom: 8px;
}

.setting-label-row label {
  margin-bottom: 0;
}

.setting-label-row span {
  color: var(--text-secondary);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

select {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background-color: var(--bg-input);
  color: var(--text-primary);
  font-size: 14px;
  width: 200px;
}

input[type='range'] {
  width: 240px;
  accent-color: var(--brand-primary);
}

.close-btn {
  width: 32px;
  height: 32px;
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

@media (max-width: 620px) {
  .settings-header {
    flex-wrap: wrap;
  }

  .settings-search {
    order: 3;
    flex-basis: 100%;
    max-width: none;
  }
}

</style>
