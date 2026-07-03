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

      <section class="editor-surface" aria-label="エディタ">
        <p class="empty-title">開発環境の土台を作成しました</p>
        <p class="empty-copy">
          Wails / Go / Vue 3 / TypeScript の最小構成です。ノート、SQLite、Markdown 保存、AI 連携はこの土台の上に追加します。
        </p>
      </section>
    </section>
  </main>
</template>

