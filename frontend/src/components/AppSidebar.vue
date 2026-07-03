<template>
  <aside class="sidebar" aria-label="サイドバー">
    <!-- Brand -->
    <div class="sidebar-brand">
      <div class="brand-icon">
        <span>A</span>
      </div>
      <span class="brand-name">Atlas Note</span>
    </div>

    <!-- New Note Button -->
    <button
      id="btn-new-note"
      class="new-note-btn"
      type="button"
      :disabled="noteStore.isSaving"
      @click="noteStore.newNote()"
    >
      <PlusIcon :size="16" />
      <span>新規ノート</span>
    </button>

    <!-- Nav Sections -->
    <nav class="sidebar-nav" aria-label="メインナビゲーション">
      <button
        v-for="item in navItems"
        :key="item.section"
        :id="`nav-${item.section}`"
        class="nav-item"
        :class="{ 'is-active': appStore.sidebarSection === item.section }"
        type="button"
        @click="appStore.setSidebarSection(item.section)"
      >
        <component :is="item.icon" :size="16" class="nav-icon" />
        <span>{{ item.label }}</span>
        <span v-if="item.count > 0" class="nav-badge">{{ item.count }}</span>
      </button>
    </nav>

    <!-- Spacer -->
    <div class="sidebar-spacer" />

    <!-- Theme Toggle -->
    <button
      id="btn-theme-toggle"
      class="theme-toggle"
      type="button"
      :title="appStore.theme === 'dark' ? 'ライトテーマに切り替え' : 'ダークテーマに切り替え'"
      @click="appStore.toggleTheme()"
    >
      <SunIcon v-if="appStore.theme === 'dark'" :size="16" />
      <MoonIcon v-else :size="16" />
    </button>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { PlusIcon, FileTextIcon, StarIcon, PinIcon, Trash2Icon, SunIcon, MoonIcon } from '@lucide/vue'
import { useNoteStore } from '../stores/useNoteStore'
import { useAppStore } from '../stores/useAppStore'
import type { SidebarSection } from '../stores/useAppStore'

const noteStore = useNoteStore()
const appStore = useAppStore()

const navItems = computed<Array<{
  section: SidebarSection
  label: string
  icon: unknown
  count: number
}>>(() => [
  {
    section: 'notes',
    label: 'すべてのノート',
    icon: FileTextIcon,
    count: noteStore.activeNotes.length,
  },
  {
    section: 'favorites',
    label: 'お気に入り',
    icon: StarIcon,
    count: noteStore.favoriteNotes.length,
  },
  {
    section: 'pinned',
    label: 'ピン留め',
    icon: PinIcon,
    count: noteStore.pinnedNotes.length,
  },
  {
    section: 'trash',
    label: 'ゴミ箱',
    icon: Trash2Icon,
    count: noteStore.trashedNotes.length,
  },
])
</script>
