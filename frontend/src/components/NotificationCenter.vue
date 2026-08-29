<template>
  <aside v-if="notificationStore.notifications.length > 0" class="notification-center" aria-label="通知">
    <div
      v-for="notification in notificationStore.notifications"
      :key="notification.id"
      class="notification"
      :class="`notification-${notification.kind}`"
      :role="notification.kind === 'error' ? 'alert' : 'status'"
      aria-atomic="true"
    >
      <span class="notification-message">{{ notification.message }}</span>
      <button
        v-if="notification.action"
        type="button"
        class="notification-action"
        :disabled="notificationStore.isActionRunning(notification.id)"
        @click="notificationStore.runAction(notification.id)"
      >
        {{ notification.action.label }}
      </button>
      <button
        type="button"
        class="notification-dismiss"
        aria-label="通知を閉じる"
        @click="notificationStore.dismiss(notification.id)"
      >
        ×
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useNotificationStore } from '../stores/useNotificationStore'

const notificationStore = useNotificationStore()
</script>

<style scoped>
.notification-center {
  position: fixed;
  top: 14px;
  right: 14px;
  z-index: 1400;
  display: flex;
  width: min(360px, calc(100vw - 28px));
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}

.notification {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  background: var(--bg-editor);
  color: var(--text-primary);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
  font-size: 12px;
  line-height: 1.45;
  pointer-events: auto;
}

.notification-error {
  border-color: color-mix(in srgb, var(--color-danger) 55%, transparent);
  color: var(--color-danger);
}

.notification-warning {
  border-color: color-mix(in srgb, var(--color-warning) 55%, transparent);
  color: var(--color-warning);
}

.notification-success {
  border-color: color-mix(in srgb, var(--color-success) 55%, transparent);
  color: var(--color-success);
}

.notification-message {
  flex: 1;
  overflow-wrap: anywhere;
}

.notification-dismiss {
  flex: 0 0 auto;
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: currentColor;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.notification-action {
  flex: 0 0 auto;
  padding: 4px 8px;
  border: 1px solid currentColor;
  border-radius: 5px;
  background: transparent;
  color: currentColor;
  font-size: 11px;
  cursor: pointer;
}

.notification-action:hover:not(:disabled) {
  background: color-mix(in srgb, currentColor 10%, transparent);
}

.notification-action:disabled {
  cursor: wait;
  opacity: 0.55;
}

.notification-dismiss:hover {
  opacity: 0.7;
}
</style>
