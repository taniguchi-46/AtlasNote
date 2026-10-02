<template>
  <section class="terminal-panel" :style="terminalStyle" data-terminal aria-label="統合ターミナル">
    <Teleport :to="toolbarTarget || 'body'" :disabled="!toolbarTarget">
      <div v-show="visible" class="terminal-toolbar" role="group" aria-label="ターミナル操作">
        <span class="terminal-shell" title="OSの既定shell">既定のshell</span>
        <button type="button" :title="startLabel" :aria-label="startLabel" :disabled="starting || stopping || !!sessionId" @click="start"><PlusIcon :size="15" aria-hidden="true" /></button>
        <button type="button" title="ターミナルを終了" aria-label="ターミナルを終了" :disabled="!sessionId || stopping" @click="stop"><Trash2Icon :size="15" aria-hidden="true" /></button>
        <DropdownMenuRoot>
          <DropdownMenuTrigger as-child><button type="button" title="その他の操作" aria-label="その他の操作"><EllipsisIcon :size="15" aria-hidden="true" /></button></DropdownMenuTrigger>
          <DropdownMenuPortal><DropdownMenuContent class="terminal-menu" :side-offset="5">
            <DropdownMenuItem class="terminal-menu-item" @select="settingsStore.openSettings('terminal')">ターミナル設定</DropdownMenuItem>
            <DropdownMenuItem class="terminal-menu-item" @select="showScope = !showScope">{{ showScope ? 'shellの説明を隠す' : 'shellの説明を表示' }}</DropdownMenuItem>
          </DropdownMenuContent></DropdownMenuPortal>
        </DropdownMenuRoot>
      </div>
    </Teleport>
    <div class="terminal-session-bar" aria-label="ターミナルセッション">
      <button v-if="sessionId || exited" type="button" class="terminal-session-tab" title="ターミナルへフォーカス" aria-label="shell 1へフォーカス" @click="focusTerminal"><SquareTerminalIcon :size="14" aria-hidden="true" />shell 1</button>
      <span class="terminal-status" role="status">{{ status }}</span>
    </div>
    <p v-if="showScope" class="terminal-scope">ユーザー権限のローカル shell です。実行した CLI は OS 上のファイルにアクセスできます。MCP の公開範囲は、このターミナルを制限しません。</p>
    <div ref="terminalHost" class="terminal-screen" aria-label="ターミナル画面" @pointerdown="focusTerminal" />
    <p v-if="error" class="terminal-error" role="alert">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { PlusIcon, Trash2Icon, EllipsisIcon, SquareTerminalIcon } from '@lucide/vue'
import { DropdownMenuRoot, DropdownMenuTrigger, DropdownMenuPortal, DropdownMenuContent, DropdownMenuItem } from 'reka-ui'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { terminalAPI } from '../api/terminal'
import { useSettingsStore } from '../stores/useSettingsStore'
import { TERMINAL_THEMES, terminalFontFamily } from '../utils/terminalAppearance'

type DataEvent = { sessionId: string; data: string; sequence: number }
type ExitEvent = { sessionId: string; exitCode: number; stopped?: boolean }
const props = defineProps<{ visible: boolean; toolbarTarget?: HTMLElement | null }>()
const settingsStore = useSettingsStore()
const appearance = computed(() => TERMINAL_THEMES[settingsStore.terminalTheme].theme)
const terminalStyle = computed(() => ({ '--terminal-background': appearance.value.background, '--terminal-foreground': appearance.value.foreground }))
const terminalHost = ref<HTMLElement | null>(null)
const sessionId = ref('')
const status = ref('未起動')
const error = ref('')
const showScope = ref(false)
const starting = ref(false)
const exited = ref(false)
const stopping = ref(false)
const startLabel = computed(() => exited.value ? 'ターミナルを新規作成・再起動' : '新しいターミナル')
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
  if (starting.value || stopping.value || sessionId.value || !terminal) return
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
  if (!id || stopping.value) return
  stopping.value = true
  status.value = '停止中'
  try { await terminalAPI.stop(id) }
  catch { error.value = 'ターミナルの停止に失敗しました。'; status.value = '停止失敗' }
  finally { stopping.value = false }
}

onMounted(() => {
  terminal = new Terminal({ scrollback: settingsStore.terminalScrollback, convertEol: false, cursorBlink: settingsStore.terminalCursorBlink, cursorStyle: settingsStore.terminalCursorStyle, fontFamily: terminalFontFamily(settingsStore.terminalFont), fontSize: settingsStore.terminalFontSize, theme: { ...appearance.value } })
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

watch(() => [settingsStore.terminalFontSize, settingsStore.terminalFont] as const, async ([fontSize, font]) => {
  if (!terminal) return
  terminal.options.fontSize = fontSize
  terminal.options.fontFamily = terminalFontFamily(font)
  lastCols = 0
  lastRows = 0
  await nextTick()
  scheduleFit()
})
watch(() => settingsStore.terminalCursorStyle, value => { if (terminal) terminal.options.cursorStyle = value })
watch(() => settingsStore.terminalCursorBlink, value => { if (terminal) terminal.options.cursorBlink = value })
watch(() => settingsStore.terminalScrollback, value => { if (terminal) terminal.options.scrollback = value })
watch(appearance, theme => { if (terminal) terminal.options.theme = { ...theme } })

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
.terminal-panel{display:flex;flex:1;flex-direction:column;min-height:0;background:var(--terminal-background);color:var(--terminal-foreground)}
.terminal-toolbar{display:flex;align-items:center;gap:2px;font-size:12px;color:var(--text-primary)}
.terminal-shell{padding:2px 7px;border:1px solid var(--border);border-radius:3px;white-space:nowrap;color:var(--text-secondary)}
.terminal-toolbar button{display:grid;place-items:center;width:26px;height:26px;padding:0;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;cursor:pointer}
.terminal-toolbar button:hover:not(:disabled){background:var(--bg-hover)}
.terminal-toolbar button:disabled{color:var(--text-secondary);cursor:default}
.terminal-toolbar button:focus-visible{outline:2px solid var(--brand-primary)}
.terminal-session-bar{display:flex;align-items:center;gap:12px;min-height:31px;border-bottom:1px solid var(--border);background:var(--bg-sidebar);color:var(--text-primary)}
.terminal-session-tab{display:flex;align-items:center;gap:6px;align-self:stretch;padding:4px 12px;border-bottom:2px solid var(--brand-primary);background:var(--bg-active);font-size:12px}
.terminal-session-tab:focus-visible{outline:2px solid var(--brand-primary);outline-offset:-2px}
.terminal-status{margin-left:auto;padding:0 10px;font-size:11px;color:var(--text-secondary)}
.terminal-scope{margin:0;padding:0 9px 6px;font-size:11px;line-height:1.4;color:inherit}
.terminal-screen{flex:1;min-height:0;padding:4px 7px;overflow:hidden}
.terminal-error{margin:0;padding:5px 9px;color:inherit;font-size:12px;font-weight:600}
</style>

<style>
.terminal-menu{z-index:1500;max-width:min(340px,calc(100vw - 20px));padding:4px;border:1px solid var(--border-strong);border-radius:4px;background:var(--bg-sidebar);color:var(--text-primary);box-shadow:0 8px 24px #0004}
.terminal-menu-item{padding:6px 12px;font-size:12px;cursor:pointer;outline:none}
.terminal-menu-item[data-highlighted]{background:var(--bg-active)}
</style>
