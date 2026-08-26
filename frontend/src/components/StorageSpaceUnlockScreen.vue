<template>
  <main class="storage-space-unlock-screen">
    <section class="storage-space-unlock-card" aria-labelledby="storage-space-unlock-title">
      <div class="storage-space-unlock-icon" aria-hidden="true">
        <LockKeyholeIcon :size="30" />
      </div>
      <p class="storage-space-unlock-kicker">保存空間はロックされています</p>
      <h1 id="storage-space-unlock-title">{{ spaceName }}</h1>
      <p class="storage-space-unlock-description">
        この保存空間の本文は暗号化されています。パスフレーズを入力すると、この起動中だけ内容を利用できます。
      </p>
      <ContentLockControls
        :target="{ type: 'space', id: spaceId }"
        :target-label="spaceName"
        :show-lock-now="false"
        @changed="emit('unlocked')"
      />
      <p class="storage-space-unlock-help">パスフレーズを忘れた場合、本文を復元することはできません。</p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { LockKeyholeIcon } from '@lucide/vue'
import ContentLockControls from './ContentLockControls.vue'

defineProps<{
  spaceId: string
  spaceName: string
}>()

const emit = defineEmits<{
  unlocked: []
}>()
</script>

<style scoped>
.storage-space-unlock-screen {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: 28px;
  background: var(--bg-editor);
  color: var(--text-primary);
}

.storage-space-unlock-card {
  width: min(430px, 100%);
  padding: 30px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-sidebar);
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.24);
}

.storage-space-unlock-icon {
  display: grid;
  width: 54px;
  height: 54px;
  margin-bottom: 18px;
  place-items: center;
  border-radius: 14px;
  background: color-mix(in srgb, var(--brand-primary) 14%, var(--bg-editor));
  color: var(--brand-primary);
}

.storage-space-unlock-kicker {
  margin: 0 0 5px;
  color: var(--text-secondary);
  font-size: 13px;
}

.storage-space-unlock-card h1 {
  margin: 0;
  font-size: 24px;
}

.storage-space-unlock-description,
.storage-space-unlock-help {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.storage-space-unlock-description {
  margin: 13px 0 20px;
}

.storage-space-unlock-help {
  margin: 18px 0 0;
}
</style>
