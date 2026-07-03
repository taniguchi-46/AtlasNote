<template>
  <main class="app-shell">
    <aside class="sidebar" aria-label="ノート一覧">
      <div class="brand">
        <span class="brand-mark">A</span>
        <span class="brand-name">Atlas Note</span>
      </div>

      <nav class="nav-list" aria-label="メイン">
        <button class="nav-item is-active" type="button">Notes</button>
        <button class="nav-item" type="button">Tags</button>
        <button class="nav-item" type="button">Settings</button>
      </nav>
    </aside>

    <section class="workspace" aria-label="ワークスペース">
      <header class="toolbar">
        <div>
          <p class="eyebrow">Local-first second brain</p>
          <h1>Atlas Note</h1>
        </div>
        <button class="primary-action" type="button">New note</button>
      </header>

      <section
        v-if="startupStatus && !startupStatus.ready"
        class="startup-alert"
        role="alert"
        aria-labelledby="startup-alert-title"
      >
        <p id="startup-alert-title" class="startup-alert-title">起動時の初期化に失敗しました</p>
        <p class="startup-alert-copy">
          DB、保存先ディレクトリ、Markdown Store のいずれかを準備できませんでした。保存機能は利用できません。
        </p>
        <dl class="startup-alert-details">
          <div v-if="startupStatus.dataDir">
            <dt>保存先</dt>
            <dd>{{ startupStatus.dataDir }}</dd>
          </div>
          <div v-if="startupStatus.message">
            <dt>エラー</dt>
            <dd>{{ startupStatus.message }}</dd>
          </div>
        </dl>
      </section>

      <section
        v-else-if="startupStatusError"
        class="startup-alert"
        role="alert"
        aria-labelledby="startup-status-error-title"
      >
        <p id="startup-status-error-title" class="startup-alert-title">起動状態を確認できません</p>
        <p class="startup-alert-copy">{{ startupStatusError }}</p>
      </section>

      <section class="editor-surface" aria-label="エディタ">
        <p class="empty-title">開発環境の土台を作成しました</p>
        <p class="empty-copy">
          Wails / Go / Vue 3 / TypeScript の最小構成です。ノート、SQLite、Markdown 保存、AI 連携はこの土台の上に追加します。
        </p>
      </section>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getStartupStatus, type StartupStatus } from './api/startup'

const startupStatus = ref<StartupStatus | null>(null)
const startupStatusError = ref('')

onMounted(async () => {
  try {
    startupStatus.value = await getStartupStatus()
  } catch (error) {
    startupStatusError.value =
      error instanceof Error ? error.message : 'Wails API の呼び出しに失敗しました。'
  }
})
</script>
