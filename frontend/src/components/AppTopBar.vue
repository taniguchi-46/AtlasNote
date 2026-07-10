<template>
  <header class="app-topbar">
    <div class="topbar-left">
      <!-- メモ同期 -->
      <button class="icon-btn" title="メモ同期" type="button" @click="$emit('sync')">
        <RefreshCwIcon :size="18" />
      </button>
      
      <!-- メモ検索 -->
      <div class="search-container">
        <SearchIcon :size="16" class="search-icon" />
        <input 
          type="text" 
          class="search-input" 
          placeholder="メモを検索..." 
          @input="$emit('search', ($event.target as HTMLInputElement).value)" 
        />
      </div>
    </div>

    <div class="topbar-right">
      <!-- 新しいノート -->
      <button class="icon-btn" title="新しいノート" type="button" @click="$emit('new-note')">
        <FilePlusIcon :size="18" />
      </button>
      
      <!-- 常に最前面 -->
      <button 
        class="icon-btn" 
        :class="{ active: isAlwaysOnTop }" 
        title="常に最前面" 
        type="button" 
        @click="$emit('toggle-always-on-top')"
      >
        <AppWindowIcon :size="18" />
      </button>

      <!-- 設定 -->
      <button class="icon-btn" title="設定" type="button" @click="$emit('open-settings')">
        <SettingsIcon :size="18" />
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { 
  RefreshCwIcon, 
  SearchIcon, 
  FilePlusIcon, 
  AppWindowIcon, 
  SettingsIcon 
} from '@lucide/vue'

defineProps<{
  isAlwaysOnTop: boolean
}>()

defineEmits<{
  (e: 'sync'): void
  (e: 'search', query: string): void
  (e: 'new-note'): void
  (e: 'toggle-always-on-top'): void
  (e: 'open-settings'): void
}>()
</script>

<style scoped>
.app-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 16px;
  background-color: var(--bg-app);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  -webkit-app-region: drag; /* For window dragging if Wails frameless is used */
}

.topbar-left, .topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  -webkit-app-region: no-drag;
}

.search-container {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 8px;
  color: var(--text-tertiary);
}

.search-input {
  width: 200px;
  height: 28px;
  padding: 0 8px 0 32px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background-color: var(--bg-input);
  color: var(--text-primary);
  font-size: 13px;
  transition: width 0.2s, border-color 0.2s;
}

.search-input:focus {
  outline: none;
  border-color: var(--brand-primary);
  width: 250px;
}

.icon-btn {
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
  transition: background-color 0.2s, color 0.2s;
}

.icon-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.icon-btn.active {
  color: var(--brand-primary);
  background-color: var(--bg-active);
}
</style>
