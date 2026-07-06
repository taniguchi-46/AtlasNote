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
              <label>表示フォント</label>
              <select v-model="settingsStore.fontFamily">
                <option value="Meiryo">Meiryo</option>
                <option value="Yu Gothic UI">Yu Gothic UI</option>
                <option value="Noto Sans JP">Noto Sans JP</option>
                <option value="BIZ UDPGothic">BIZ UDPGothic</option>
              </select>
            </div>
            <div class="setting-group">
              <label>グローバルショートカット</label>
              <p class="setting-desc">（モック：将来的に実装予定）</p>
            </div>
          </section>

          <!-- エディター設定 -->
          <section v-if="activeTab === 'editor'">
            <h3>エディター</h3>
            <div class="setting-group">
              <label>タイポグラフィ</label>
              <p class="setting-desc">（モック：行間や文字サイズなどを設定）</p>
            </div>
            <div class="setting-group">
              <label>ファイル設定</label>
              <p class="setting-desc">（モック：添付ファイルの保存先など）</p>
            </div>
          </section>

          <!-- バックアップ設定 -->
          <section v-if="activeTab === 'backup'">
            <h3>バックアップ</h3>
            <div class="setting-group">
              <label>自動バックアップ</label>
              <input type="checkbox" checked disabled /> 有効（モック）
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

const settingsStore = useSettingsStore()
const appStore = useAppStore()

const tabs = [
  { id: 'theme', name: 'テーマ' },
  { id: 'general', name: '一般' },
  { id: 'editor', name: 'エディター' },
  { id: 'backup', name: 'バックアップ' },
]
const activeTab = ref('theme')
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
  background-color: var(--bg-primary);
  border-radius: 8px;
  width: 700px;
  height: 500px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
}

.settings-header h2 {
  margin: 0;
  font-size: 1.2rem;
}

.settings-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.settings-sidebar {
  width: 200px;
  background-color: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
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
  background-color: var(--item-hover);
}

.settings-tab.active {
  background-color: var(--item-active);
  color: var(--primary-color);
  font-weight: 500;
  border-left: 3px solid var(--primary-color);
}

.settings-panel {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
}

.settings-panel h3 {
  margin-top: 0;
  margin-bottom: 24px;
  font-size: 1.1rem;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 8px;
}

.setting-group {
  margin-bottom: 24px;
}

.setting-group label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.setting-desc {
  font-size: 13px;
  color: var(--text-tertiary);
  margin: 0;
}

select {
  padding: 6px 12px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background-color: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  width: 200px;
}
</style>
