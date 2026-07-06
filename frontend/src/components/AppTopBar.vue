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
      <button class="icon-btn primary" title="新しいノート" type="button" @click="$emit('new-note')">
        <FilePlusIcon :size="18" />
      </button>
      
      <!-- 常に最前面 -->
      <button class="icon-btn" title="常に最前面" type="button" @click="$emit('toggle-always-on-top')">
        <PinIcon :size="18" />
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
  PinIcon, 
  SettingsIcon 
} from '@lucide/vue'

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
  background-color: var(--bg-primary);
  border-bottom: 1px solid var(--border-color);
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
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 13px;
  transition: width 0.2s, border-color 0.2s;
}

.search-input:focus {
  outline: none;
  border-color: var(--primary-color);
  width: 250px;
}

.icon-btn.primary {
  color: var(--primary-color);
}
</style>
