<template>
  <section class="terminal-panel" data-terminal aria-label="統合ターミナル">
    <div class="terminal-toolbar">
      <span>{{ status }}</span>
      <div>
        <button type="button" :disabled="starting || !!sessionId" @click="start">{{ exited ? '再起動' : '開始' }}</button>
        <button type="button" :disabled="!sessionId" @click="stop">停止</button>
      </div>
    </div>
    <p class="terminal-scope">ユーザー権限のローカル shell です。実行した CLI は OS 上のファイルにアクセスできます。MCP の公開範囲は、このターミナルを制限しません。</p>
    <div ref="terminalHost" class="terminal-screen" aria-label="ターミナル画面" @pointerdown="focusTerminal" />
    <p v-if="error" class="terminal-error" role="alert">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { terminalAPI } from '../api/terminal'

type DataEvent = { sessionId: string; data: string; sequence: number }
type ExitEvent = { sessionId: string; exitCode: number; stopped?: boolean }
const props = defineProps<{ visible: boolean }>()
const terminalHost = ref<HTMLElement | null>(null)
const sessionId = ref('')
const status = ref('未起動')
const error = ref('')
const starting = ref(false)
const exited = ref(false)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let observer: ResizeObserver | null = null
let cancelData: (() => void) | null = null
let cancelExit: (() => void) | null = null
let cancelInput: { dispose(): void } | null = null
let inputQueue = Promise.resolve()
let pendingEvents: DataEvent[] = []
let pendingExit: ExitEvent | null = null
let resizeFrame = 0
let lastCols = 0
let lastRows = 0
let disposed = false

function focusTerminal() { terminal?.focus() }

function renderData(event: DataEvent) {
  if (event.sessionId !== sessionId.value || !terminal) return
  const bytes = Uint8Array.from(atob(event.data), char => char.charCodeAt(0))
  terminal.write(bytes, () => { void terminalAPI.acknowledge(event.sessionId, event.sequence).catch(() => {}) })
}

function handleData(event: DataEvent) {
  if (starting.value && !sessionId.value) { pendingEvents.push(event); return }
  renderData(event)
}

function handleExit(event: ExitEvent) {
  if (starting.value && !sessionId.value) { pendingExit = event; return }
  if (event.sessionId !== sessionId.value) return
  sessionId.value = ''
  exited.value = true
  status.value = event.stopped ? '停止しました' : `終了コード: ${event.exitCode}`
}

function fit() {
  if (!props.visible || !terminalHost.value || !fitAddon || !terminal) return
  if (terminalHost.value.clientWidth < 20 || terminalHost.value.clientHeight < 20) return
  fitAddon.fit()
  const cols = terminal.cols
  const rows = terminal.rows
  if (cols < 1 || rows < 1 || cols > 500 || rows > 500) return
  if (cols === lastCols && rows === lastRows) return
  lastCols = cols
  lastRows = rows
  if (sessionId.value) void terminalAPI.resize(sessionId.value, cols, rows).catch(() => { error.value = 'ターミナルのサイズ変更に失敗しました。' })
}

function scheduleFit() {
  cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(fit)
}

async function start() {
  if (starting.value || sessionId.value || !terminal) return
  starting.value = true
  error.value = ''
  status.value = '起動中'
  pendingEvents = []
  pendingExit = null
  await nextTick()
  fit()
  try {
    const state = await terminalAPI.start(Math.max(1, terminal.cols), Math.max(1, terminal.rows))
    if (disposed) { await terminalAPI.stop(state.sessionId); return }
    sessionId.value = state.sessionId
    exited.value = false
    status.value = '実行中'
    for (const event of pendingEvents) renderData(event)
    if (pendingExit) handleExit(pendingExit)
    terminal.focus()
  } catch {
    status.value = '起動失敗'
    error.value = 'shell を起動できませんでした。既定の shell を確認してください。'
  } finally {
    starting.value = false
    pendingEvents = []
    pendingExit = null
  }
}

async function stop() {
  const id = sessionId.value
  if (!id) return
  status.value = '停止中'
  try { await terminalAPI.stop(id) }
  catch { error.value = 'ターミナルの停止に失敗しました。'; status.value = '停止失敗' }
}

onMounted(() => {
  terminal = new Terminal({ scrollback: 2000, convertEol: false, cursorBlink: true })
  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.open(terminalHost.value!)
  cancelData = EventsOn('terminal:data', handleData)
  cancelExit = EventsOn('terminal:exit', handleExit)
  cancelInput = terminal.onData(data => {
    const id = sessionId.value
    if (!id) return
    // Preserve keystroke order; xterm handles IME composition itself.
    inputQueue = inputQueue.then(() => terminalAPI.write(id, data)).catch(() => { error.value = 'ターミナルへの入力に失敗しました。' })
  })
  observer = new ResizeObserver(scheduleFit)
  observer.observe(terminalHost.value!)
  if (props.visible) { scheduleFit(); void start() }
})

watch(() => props.visible, async visible => {
  if (!visible) return
  await nextTick()
  scheduleFit()
  if (!sessionId.value && !starting.value && !exited.value) void start()
})

onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(resizeFrame)
  observer?.disconnect()
  cancelData?.()
  cancelExit?.()
  cancelInput?.dispose()
  if (sessionId.value) void terminalAPI.stop(sessionId.value).catch(() => {})
  terminal?.dispose()
})
</script>

<style scoped>
.terminal-panel{display:flex;flex:1;flex-direction:column;min-height:0;background:#0d1117;color:#d7dee7}
.terminal-toolbar{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:6px 9px;font-size:12px}
.terminal-toolbar div{display:flex;gap:6px}
.terminal-toolbar button{padding:3px 9px;border:1px solid #455365;border-radius:4px;background:#202b38;color:#eef3f8;cursor:pointer}
.terminal-toolbar button:disabled{opacity:.5;cursor:default}
.terminal-toolbar button:focus-visible{outline:2px solid var(--brand-primary)}
.terminal-scope{margin:0;padding:0 9px 6px;font-size:11px;line-height:1.4;color:#aab8c7}
.terminal-screen{flex:1;min-height:0;padding:4px 7px;overflow:hidden}
.terminal-error{margin:0;padding:5px 9px;color:#ffb4ad;font-size:12px}
</style>
