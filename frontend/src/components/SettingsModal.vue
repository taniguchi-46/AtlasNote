<template>
  <div v-if="settingsStore.isSettingsOpen" class="settings-modal-overlay" @click.self="settingsStore.closeSettings">
    <div class="settings-modal-content">
      <header class="settings-header">
        <h2>設定</h2>
        <button class="icon-btn close-btn" title="閉じる" @click="settingsStore.closeSettings">
          <XIcon :size="20" />
        </button>
      </header>

      <div class="settings-body">
        <aside class="settings-sidebar">
          <button 
            v-for="tab in tabs" 
            :key="tab.id"
            class="settings-tab"
            :class="{ active: activeTab === tab.id }"
            @click="activeTab = tab.id"
          >
            {{ tab.name }}
          </button>
        </aside>

        <main class="settings-panel">
          <!-- テーマ設定 -->
          <section v-if="activeTab === 'theme'">
            <h3>テーマ</h3>
            <div class="setting-group">
              <label>アプリケーションテーマ</label>
              <select v-model="appStore.theme">
                <option value="light">ライト</option>
                <option value="dark">ダーク</option>
              </select>
            </div>
          </section>

          <!-- 一般設定 -->
          <section v-if="activeTab === 'general'">
            <h3>一般</h3>
            <div class="setting-group">
              <label>グローバルショートカット</label>
              <p class="setting-desc">（現在開発中...）</p>
            </div>
            <div class="settings-section">
              <h4>ノートブック</h4>
              <div class="setting-group">
                <label>既定アイコン</label>
                <NotebookIconPicker
                  v-model="settingsStore.defaultNotebookIcon"
                  allow-user-icon-delete
                />
              </div>
            </div>
          </section>

          <!-- エディター設定 -->
          <section v-if="activeTab === 'editor'">
            <h3>エディター</h3>
            <div class="settings-section">
              <h4>タイポグラフィ</h4>
              <div class="setting-group">
                <label>フォント指定</label>
                <select v-model="settingsStore.fontFamily">
                  <option value="Meiryo">Meiryo</option>
                  <option value="Yu Gothic UI">Yu Gothic UI</option>
                  <option value="Noto Sans JP">Noto Sans JP</option>
                  <option value="BIZ UDPGothic">BIZ UDPGothic</option>
                </select>
              </div>
              <div class="setting-group">
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
              <div class="setting-group">
                <label>新規ノート1行目のスタイル</label>
                <select v-model="settingsStore.editorFirstLineStyle">
                  <option value="heading1">H1</option>
                  <option value="heading2">H2</option>
                  <option value="heading3">H3</option>
                  <option value="paragraph">普通</option>
                </select>
              </div>
              <div class="setting-group">
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
              <div class="setting-group">
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
              <div class="setting-group">
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

          <!-- バックアップ設定 -->
          <section v-if="activeTab === 'backup'">
            <h3>バックアップ</h3>
            <div class="setting-group">
              <label>自動バックアップ</label>
              <input type="checkbox" checked disabled /> 有効（デフォルト）
            </div>
            <div class="setting-group">
              <label>バックアップの復元</label>
              <button class="primary-btn" disabled>復元する</button>
            </div>
          </section>
        </main>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { XIcon } from '@lucide/vue'
import { useSettingsStore } from '../stores/useSettingsStore'
import { useAppStore } from '../stores/useAppStore'
import NotebookIconPicker from './NotebookIconPicker.vue'

const settingsStore = useSettingsStore()
const appStore = useAppStore()

const tabs = [
  { id: 'theme', name: 'テーマ' },
  { id: 'general', name: '一般' },
  { id: 'editor', name: 'エディター' },
  { id: 'backup', name: 'バックアップ' },
]
const activeTab = ref('theme')
const fontSizeOptions = [12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 26]
</script>

<style scoped>
.settings-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.settings-modal-content {
  background-color: var(--bg-editor);
  border: 1px solid var(--border);
  border-radius: 8px;
  width: 700px;
  height: 500px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  overflow: hidden;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
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

.settings-tab.active {
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

.settings-panel h3 {
  margin-top: 0;
  margin-bottom: 24px;
  font-size: 1.1rem;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border);
  padding-bottom: 8px;
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

.setting-desc {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
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

.primary-btn {
  padding: 6px 16px;
  background-color: var(--brand-primary);
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
}

.primary-btn:disabled {
  background-color: var(--bg-hover);
  color: var(--text-secondary);
  cursor: not-allowed;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}
</style>
