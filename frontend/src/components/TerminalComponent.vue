<template>
  <section class="terminal-panel" :style="terminalStyle" data-terminal aria-label="統合ターミナル">
    <Teleport :to="toolbarTarget || 'body'" :disabled="!toolbarTarget">
      <div v-show="visible" class="terminal-toolbar" role="group" aria-label="ターミナル操作">
        <span class="terminal-shell" title="OSの既定shell">既定のshell</span>
        <button type="button" title="新しいターミナル" aria-label="新しいターミナル" :disabled="starting" @click="start"><PlusIcon :size="15" aria-hidden="true" /></button>
        <button type="button" title="ターミナルを終了・閉じる" aria-label="ターミナルを終了・閉じる" :disabled="!activeSession || activeSession.stopping" @click="stop"><Trash2Icon :size="15" aria-hidden="true" /></button>
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
      <div class="terminal-session-tabs" role="tablist" aria-label="shell一覧">
        <button v-for="session in sessions" :key="session.id" type="button" class="terminal-session-tab" :class="{ active: session.id === activeId }" role="tab" :aria-selected="session.id === activeId" :title="sessionStatus(session)" :aria-label="`${session.name}へ切替${session.exited ? `、終了コード: ${session.exitCode}` : ''}`" @click="selectSession(session)"><SquareTerminalIcon :size="14" aria-hidden="true" />{{ session.name }}<span v-if="session.exited" class="terminal-exit-label">終了: {{ session.exitCode }}</span></button>
      </div>
      <span class="terminal-status" role="status">{{ status }}</span>
    </div>
    <p v-if="showScope" class="terminal-scope">ユーザー権限のローカル shell です。実行した CLI は OS 上のファイルにアクセスできます。MCP の公開範囲は、このターミナルを制限しません。</p>
    <p v-if="!sessions.length && !starting" class="terminal-empty">ターミナルはありません。＋で新しいshellを作成できます。</p>
    <div ref="terminalHost" class="terminal-screen" aria-label="ターミナル画面" @pointerdown="focusTerminal" />
    <p v-if="error" class="terminal-error" role="alert">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowReactive, watch } from 'vue'
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
type TerminalSession = {
  id: string
  name: string
  terminal: Terminal
  fitAddon: FitAddon
  host: HTMLElement
  exited: boolean
  exitCode: number | null
  stopped: boolean
  stopping: boolean
  error: string
  input: { dispose(): void } | null
  inputQueue: Promise<void>
  lastCols: number
  lastRows: number
}
const props = defineProps<{ visible: boolean; toolbarTarget?: HTMLElement | null }>()
const settingsStore = useSettingsStore()
const appearance = computed(() => TERMINAL_THEMES[settingsStore.terminalTheme].theme)
const terminalStyle = computed(() => ({ '--terminal-background': appearance.value.background, '--terminal-foreground': appearance.value.foreground }))
const terminalHost = ref<HTMLElement | null>(null)
// Shallow state leaves xterm, addons and DOM nodes outside Vue's proxies.
const sessions = shallowReactive<TerminalSession[]>([])
const activeId = ref('')
const activeSession = computed(() => sessions.find(session => session.id === activeId.value))
const startError = ref('')
const error = computed(() => activeSession.value?.error || startError.value)
const status = computed(() => starting.value ? '起動中' : activeSession.value ? sessionStatus(activeSession.value) : '未起動')
const showScope = ref(false)
const starting = ref(false)
let observer: ResizeObserver | null = null
let cancelData: (() => void) | null = null
let cancelExit: (() => void) | null = null
// Wails can emit before StartTerminal resolves. Only replay the returned ID;
// known sessions keep receiving output during another session's startup.
let pendingEvents: Array<DataEvent | ExitEvent> = []
const closedIds = new Set<string>()
let nextNumber = 1
let initialized = false
let resizeFrame = 0
let disposed = false

function sessionStatus(session: TerminalSession) {
  if (session.stopping) return '停止中'
  if (session.exited) return session.stopped ? `停止しました (終了コード: ${session.exitCode})` : `終了コード: ${session.exitCode}`
  return '実行中'
}
function focusTerminal() { if (props.visible) activeSession.value?.terminal.focus() }

function renderData(session: TerminalSession, event: DataEvent) {
  if (session.exited || disposed) return
  const bytes = Uint8Array.from(atob(event.data), char => char.charCodeAt(0))
  session.terminal.write(bytes, () => {
    if (!disposed && !closedIds.has(event.sessionId)) void terminalAPI.acknowledge(event.sessionId, event.sequence).catch(() => {})
  })
}
function handleData(event: DataEvent) {
  const session = sessions.find(item => item.id === event.sessionId)
  if (session) renderData(session, event)
  else if (starting.value && !disposed && !closedIds.has(event.sessionId)) pendingEvents.push(event)
}
function handleExit(event: ExitEvent) {
  const session = sessions.find(item => item.id === event.sessionId)
  if (!session) {
    if (starting.value && !disposed && !closedIds.has(event.sessionId)) pendingEvents.push(event)
    return
  }
  if (session.exited) return
  session.exited = true
  session.exitCode = event.exitCode
  session.stopped = !!event.stopped
  session.input?.dispose()
  session.input = null
}

function fitSession(session: TerminalSession) {
  if (disposed || !props.visible || session.id !== activeId.value) return
  if (session.host.clientWidth < 20 || session.host.clientHeight < 20) return
  // Opening under display:none prevents xterm from measuring its font cells.
  // Output can enter its buffer before open; initialize the renderer on reveal.
  if (!session.terminal.element) session.terminal.open(session.host)
  session.fitAddon.fit()
  const { cols, rows } = session.terminal
  if (cols < 1 || rows < 1 || cols > 500 || rows > 500) return
  if (cols === session.lastCols && rows === session.lastRows) return
  session.lastCols = cols
  session.lastRows = rows
  if (session.id && !session.exited && !session.stopping) {
    void terminalAPI.resize(session.id, cols, rows).catch(() => {
      if (!disposed && !session.exited && !closedIds.has(session.id)) {
        session.lastCols = 0
        session.lastRows = 0
        session.error = 'ターミナルのサイズ変更に失敗しました。'
      }
    })
  }
}
function scheduleFit() {
  if (disposed) return
  cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(() => { if (activeSession.value) fitSession(activeSession.value) })
}
async function selectSession(session: TerminalSession) {
  activeId.value = session.id
  for (const item of sessions) item.host.style.display = item === session ? '' : 'none'
  await nextTick()
  if (disposed || activeSession.value !== session) return
  fitSession(session)
  focusTerminal()
}
function disposeSession(session: TerminalSession) {
  session.input?.dispose()
  session.input = null
  session.terminal.dispose()
  session.host.remove()
}

async function start() {
  if (starting.value || disposed || !terminalHost.value) return
  initialized = true
  starting.value = true
  startError.value = ''
  pendingEvents = []
  const terminal = new Terminal({ scrollback: settingsStore.terminalScrollback, convertEol: false, cursorBlink: settingsStore.terminalCursorBlink, cursorStyle: settingsStore.terminalCursorStyle, fontFamily: terminalFontFamily(settingsStore.terminalFont), fontSize: settingsStore.terminalFontSize, theme: { ...appearance.value } })
  const fitAddon = new FitAddon()
  const host = document.createElement('div')
  host.className = 'terminal-session-screen'
  host.style.height = '100%'
  host.style.display = 'none'
  terminalHost.value.append(host)
  terminal.loadAddon(fitAddon)
  const session = shallowReactive<TerminalSession>({ id: '', name: `shell ${nextNumber++}`, terminal, fitAddon, host, exited: false, exitCode: null, stopped: false, stopping: false, error: '', input: null, inputQueue: Promise.resolve(), lastCols: 0, lastRows: 0 })
  try {
    // Start with xterm's default dimensions; fit only after this session is visible.
    const state = await terminalAPI.start(terminal.cols, terminal.rows)
    if (disposed) { await terminalAPI.stop(state.sessionId); disposeSession(session); return }
    session.id = state.sessionId
    // Settings may change while the backend is creating the PTY.
    terminal.options.fontSize = settingsStore.terminalFontSize
    terminal.options.fontFamily = terminalFontFamily(settingsStore.terminalFont)
    terminal.options.cursorStyle = settingsStore.terminalCursorStyle
    terminal.options.cursorBlink = settingsStore.terminalCursorBlink
    terminal.options.scrollback = settingsStore.terminalScrollback
    terminal.options.theme = { ...appearance.value }
    session.input = terminal.onData(data => {
      if (session.exited || session.stopping || closedIds.has(session.id) || disposed) return
      // Each session owns its queue; a blocked Write cannot delay other sessions.
      session.inputQueue = session.inputQueue.then(async () => {
        if (!session.exited && !session.stopping && !closedIds.has(session.id) && !disposed) await terminalAPI.write(session.id, data)
      }).catch(() => {
        if (!session.exited && !session.stopping && !closedIds.has(session.id) && !disposed) session.error = 'ターミナルへの入力に失敗しました。'
      })
    })
    sessions.push(session)
    for (const event of pendingEvents) {
      if (event.sessionId !== session.id) continue
      if ('data' in event) renderData(session, event)
      else handleExit(event)
    }
    await selectSession(session)
  } catch {
    disposeSession(session)
    if (!disposed) startError.value = 'shell を起動できませんでした。既定の shell を確認してください。'
  } finally {
    starting.value = false
    pendingEvents = []
  }
}
async function stop() {
  const session = activeSession.value
  if (!session || session.stopping) return
  session.stopping = true
  session.error = ''
  try {
    if (!session.exited) await terminalAPI.stop(session.id)
    if (disposed) return
    const index = sessions.indexOf(session)
    if (index < 0) return
    closedIds.add(session.id)
    sessions.splice(index, 1)
    disposeSession(session)
    // Do not steal focus if the user selected/created another tab while stopping.
    if (activeId.value === session.id) {
      const neighbor = sessions[Math.min(index, sessions.length - 1)]
      if (neighbor) await selectSession(neighbor)
      else activeId.value = ''
    }
  } catch {
    if (!disposed) session.error = 'ターミナルの停止に失敗しました。'
  } finally { session.stopping = false }
}

onMounted(() => {
  cancelData = EventsOn('terminal:data', handleData)
  cancelExit = EventsOn('terminal:exit', handleExit)
  observer = new ResizeObserver(scheduleFit)
  observer.observe(terminalHost.value!)
  if (props.visible) void start()
})
watch(() => [settingsStore.terminalFontSize, settingsStore.terminalFont] as const, async ([fontSize, font]) => {
  for (const session of sessions) {
    session.terminal.options.fontSize = fontSize
    session.terminal.options.fontFamily = terminalFontFamily(font)
    session.lastCols = 0
    session.lastRows = 0
  }
  await nextTick()
  scheduleFit()
})
watch(() => settingsStore.terminalCursorStyle, value => { for (const session of sessions) session.terminal.options.cursorStyle = value })
watch(() => settingsStore.terminalCursorBlink, value => { for (const session of sessions) session.terminal.options.cursorBlink = value })
watch(() => settingsStore.terminalScrollback, value => { for (const session of sessions) session.terminal.options.scrollback = value })
watch(appearance, theme => { for (const session of sessions) session.terminal.options.theme = { ...theme } })
watch(() => props.visible, async visible => {
  if (!visible) return
  await nextTick()
  if (disposed) return
  scheduleFit()
  if (!initialized) void start()
})
onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(resizeFrame)
  observer?.disconnect()
  cancelData?.()
  cancelExit?.()
  for (const session of sessions) {
    if (!session.exited) void terminalAPI.stop(session.id).catch(() => {})
    disposeSession(session)
  }
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
.terminal-session-tabs{display:flex;min-width:0;overflow-x:auto;align-self:stretch}
.terminal-session-tab{display:flex;align-items:center;gap:6px;flex-shrink:0;padding:4px 12px;border-bottom:2px solid transparent;background:transparent;font-size:12px;cursor:pointer}
.terminal-session-tab.active{border-bottom-color:var(--brand-primary);background:var(--bg-active)}
.terminal-exit-label{font-size:10px;color:var(--text-secondary)}
.terminal-session-tab:focus-visible{outline:2px solid var(--brand-primary);outline-offset:-2px}
.terminal-status{margin-left:auto;padding:0 10px;font-size:11px;color:var(--text-secondary)}
.terminal-scope{margin:0;padding:0 9px 6px;font-size:11px;line-height:1.4;color:inherit}
.terminal-screen{flex:1;min-height:0;padding:4px 7px;overflow:hidden}
.terminal-empty{margin:0;padding:12px;font-size:12px;color:var(--text-secondary)}
.terminal-error{margin:0;padding:5px 9px;color:inherit;font-size:12px;font-weight:600}
</style>

<style>
.terminal-menu{z-index:1500;max-width:min(340px,calc(100vw - 20px));padding:4px;border:1px solid var(--border-strong);border-radius:4px;background:var(--bg-sidebar);color:var(--text-primary);box-shadow:0 8px 24px #0004}
.terminal-menu-item{padding:6px 12px;font-size:12px;cursor:pointer;outline:none}
.terminal-menu-item[data-highlighted]{background:var(--bg-active)}
</style>
