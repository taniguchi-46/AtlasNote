import { defineStore } from 'pinia'
import { ref } from 'vue'

export type NotificationKind = 'info' | 'success' | 'warning' | 'error'

export type NotificationAction = {
  label: string
  run: () => void | Promise<unknown>
}

export type AppNotification = {
  id: number
  kind: NotificationKind
  message: string
  source?: string
  code?: string
  retryable?: boolean
  action?: NotificationAction
  dedupeKey: string
}

export type NotificationOptions = {
  kind?: NotificationKind
  source?: string
  code?: string
  retryable?: boolean
  action?: NotificationAction
  dedupeKey?: string
}

let nextNotificationId = 0

export const useNotificationStore = defineStore('notifications', () => {
  const notifications = ref<AppNotification[]>([])
  const actionRunningId = ref<number | null>(null)

  function notify(message: string, options: NotificationOptions = {}) {
    const dedupeKey = options.dedupeKey
      ?? [options.source, options.code, message].filter(Boolean).join(':')
    const existing = notifications.value.find((notification) => notification.dedupeKey === dedupeKey)

    if (existing) {
      existing.kind = options.kind ?? existing.kind
      existing.message = message
      existing.source = options.source
      existing.code = options.code
      existing.retryable = options.retryable
      existing.action = options.action
      return existing.id
    }

    const notification: AppNotification = {
      id: ++nextNotificationId,
      kind: options.kind ?? 'info',
      message,
      source: options.source,
      code: options.code,
      retryable: options.retryable,
      action: options.action,
      dedupeKey,
    }
    notifications.value.push(notification)
    return notification.id
  }

  function dismiss(id: number) {
    notifications.value = notifications.value.filter((notification) => notification.id !== id)
  }

  function dismissBySource(source: string) {
    notifications.value = notifications.value.filter((notification) => notification.source !== source)
  }

  function clear() {
    notifications.value = []
  }

  function isActionRunning(id: number) {
    return actionRunningId.value === id
  }

  async function runAction(id: number) {
    if (actionRunningId.value !== null) return

    const notification = notifications.value.find((item) => item.id === id)
    if (!notification?.action) return

    const action = notification.action
    actionRunningId.value = id
    dismiss(id)
    try {
      await action.run()
    } catch (_) {
      // The operation owns its error state and reports a new notification.
    } finally {
      actionRunningId.value = null
    }
  }

  return {
    notifications,
    notify,
    dismiss,
    dismissBySource,
    clear,
    isActionRunning,
    runAction,
  }
})
