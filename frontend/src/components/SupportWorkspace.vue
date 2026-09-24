<template>
  <button
    v-if="store.isOpen && store.isMinimized"
    class="support-restore"
    :style="{ top: `${restoreTop}px` }"
    type="button"
    title="サポートを再開"
    aria-label="サポートを再開"
    @click="store.restore"
  ><PanelRightCloseIcon :size="17" aria-hidden="true" /></button>
  <section
    v-show="store.isOpen && !store.isMinimized"
    ref="panel"
    class="support-workspace"
    :class="{ 'is-floating': store.isFloating }"
    :style="panelStyle"
    aria-label="整理とAIのサポート"
  >
    <header class="support-header" @pointerdown="startMove">
      <img :src="atlasNoteLogo" alt="" aria-hidden="true">
      <nav aria-label="サポート機能" @pointerdown.stop>
        <button type="button" :class="{ active: store.activeTab === 'organize' }" :aria-current="store.activeTab === 'organize' ? 'page' : undefined" @click="store.open('organize')">整理</button>
        <button type="button" :class="{ active: store.activeTab === 'ai' }" :aria-current="store.activeTab === 'ai' ? 'page' : undefined" @click="store.open('ai')">AI</button>
      </nav>
      <div class="support-actions" @pointerdown.stop>
        <button type="button" :title="store.isFloating ? 'ドック表示' : 'フローティング表示'" :aria-label="store.isFloating ? 'ドック表示' : 'フローティング表示'" :aria-pressed="store.isFloating" @click="store.toggleFloating"><AppWindowIcon :size="16" aria-hidden="true" /></button>
        <button type="button" title="サポートを最小化" aria-label="サポートを最小化" @click="store.minimize"><XIcon :size="16" aria-hidden="true" /></button>
      </div>
    </header>
    <div class="support-content">
      <OrganizationCenter v-show="store.activeTab === 'organize'" />
      <AIWorkspace v-show="store.activeTab === 'ai' && settingsStore.aiEnabled" />
      <div v-if="store.activeTab === 'ai' && !settingsStore.aiEnabled" class="support-ai-disabled">
        <p>AI機能は現在オフです。整理機能は引き続き利用できます。</p>
        <button type="button" @click="settingsStore.openSettings('ai')">AI設定を開く</button>
      </div>
    </div>
    <button v-if="store.isFloating" class="support-floating-resize" type="button" aria-label="サポートのサイズを変更" @pointerdown.stop.prevent="startFloatingResize" />
    <button v-else class="support-dock-resize" type="button" role="separator" :aria-label="effectivePlacement === 'right' ? 'サポートの幅を調整' : 'サポートの高さを調整'" :aria-orientation="effectivePlacement === 'right' ? 'vertical' : 'horizontal'" :aria-valuemin="effectivePlacement === 'right' ? AI_WORKSPACE_RIGHT_WIDTH_MIN : AI_WORKSPACE_BOTTOM_HEIGHT_MIN" :aria-valuemax="effectivePlacement === 'right' ? AI_WORKSPACE_RIGHT_WIDTH_MAX : AI_WORKSPACE_BOTTOM_HEIGHT_MAX" :aria-valuenow="effectiveSize" @keydown="handleResizeKeydown" @pointerdown="startDockResize" />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { AppWindowIcon, PanelRightCloseIcon, XIcon } from '@lucide/vue'
import atlasNoteLogo from '../assets/AtlasNote_logo.png'
import {
  AI_WORKSPACE_BOTTOM_HEIGHT_MAX, AI_WORKSPACE_BOTTOM_HEIGHT_MIN,
  AI_WORKSPACE_RIGHT_WIDTH_MAX, AI_WORKSPACE_RIGHT_WIDTH_MIN,
  useSettingsStore,
} from '../stores/useSettingsStore'
import { useSupportWorkspaceStore } from '../stores/useSupportWorkspaceStore'
import { useNoteStore } from '../stores/useNoteStore'
import AIWorkspace from './AIWorkspace.vue'
import OrganizationCenter from './OrganizationCenter.vue'

const store = useSupportWorkspaceStore()
const settingsStore = useSettingsStore()
const noteStore = useNoteStore()
const panel = ref<HTMLElement | null>(null)
const restoreTop = ref(112)
const workspaceWidth = ref(0)
const workspaceHeight = ref(0)
const placement = computed(() => settingsStore.aiWorkspacePlacement)
const effectivePlacement = computed(() => placement.value === 'right' && workspaceWidth.value < 660 ? 'bottom' : placement.value)
const effectiveSize = computed(() => {
  const right = effectivePlacement.value === 'right'
  const preferred = right ? settingsStore.aiWorkspaceRightWidth : settingsStore.aiWorkspaceBottomHeight
  const available = right ? workspaceWidth.value : workspaceHeight.value
  const minimum = right ? AI_WORKSPACE_RIGHT_WIDTH_MIN : AI_WORKSPACE_BOTTOM_HEIGHT_MIN
  const maximum = right ? AI_WORKSPACE_RIGHT_WIDTH_MAX : AI_WORKSPACE_BOTTOM_HEIGHT_MAX
  if (!available) return preferred
  return clamp(preferred, minimum, Math.max(minimum, Math.min(maximum, Math.floor(available * .6), available - (right ? 360 : 240))))
})
const panelStyle = computed(() => store.isFloating
  ? {
    left: `${clamp(store.position.left, 8, Math.max(8, window.innerWidth - Math.min(store.position.width, window.innerWidth - 16) - 8))}px`,
    top: `${clamp(store.position.top, 8, Math.max(8, window.innerHeight - Math.min(store.position.height, window.innerHeight - 16) - 8))}px`,
    width: `${Math.min(store.position.width, window.innerWidth - 16)}px`,
    height: `${Math.min(store.position.height, window.innerHeight - 16)}px`,
  }
  : effectivePlacement.value === 'right' ? { width: `${effectiveSize.value}px`, flexBasis: `${effectiveSize.value}px` } : { height: `${effectiveSize.value}px`, flexBasis: `${effectiveSize.value}px` })

function clamp(value: number, min: number, max: number) { return Math.min(Math.max(value, min), Math.max(min, max)) }
function measure() {
  const parent = panel.value?.parentElement
  if (!parent) return
  workspaceWidth.value = parent.clientWidth
  workspaceHeight.value = parent.clientHeight
  const editorBody = parent.querySelector<HTMLElement>('.editor-body')
  if (editorBody !== observedEditorBody) {
    if (observedEditorBody) observer?.unobserve(observedEditorBody)
    observedEditorBody = editorBody
    if (editorBody) observer?.observe(editorBody)
  }
  const bodyTop = editorBody
    ? editorBody.getBoundingClientRect().top - parent.getBoundingClientRect().top + 8
    : 8
  restoreTop.value = clamp(bodyTop, 8, Math.max(8, parent.clientHeight - 38))
  if (store.isFloating) store.setPosition({
    left: clamp(store.position.left, 8, Math.max(8, window.innerWidth - Math.min(store.position.width, window.innerWidth - 16) - 8)),
    top: clamp(store.position.top, 8, Math.max(8, window.innerHeight - Math.min(store.position.height, window.innerHeight - 16) - 8)),
  })
}
let observer: ResizeObserver | null = null
let observedEditorBody: HTMLElement | null = null
let gesture: { kind: 'move' | 'floating-resize' | 'dock-resize'; x: number; y: number; left: number; top: number; width: number; height: number; size: number } | null = null
function begin(kind: NonNullable<typeof gesture>['kind'], event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return
  gesture = { kind, x: event.clientX, y: event.clientY, ...store.position, size: effectiveSize.value }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', end, { once: true })
  window.addEventListener('pointercancel', end, { once: true })
}
function startMove(event: PointerEvent) {
  if (!store.isFloating || (event.target as HTMLElement).closest('button, nav')) return
  begin('move', event)
}
function startFloatingResize(event: PointerEvent) { begin('floating-resize', event) }
function startDockResize(event: PointerEvent) { begin('dock-resize', event) }
function move(event: PointerEvent) {
  if (!gesture) return
  const dx = event.clientX - gesture.x
  const dy = event.clientY - gesture.y
  if (gesture.kind === 'move') {
    store.setPosition({ left: clamp(gesture.left + dx, 8, Math.max(8, window.innerWidth - Math.min(store.position.width, window.innerWidth - 16) - 8)), top: clamp(gesture.top + dy, 8, Math.max(8, window.innerHeight - Math.min(store.position.height, window.innerHeight - 16) - 8)) })
  } else if (gesture.kind === 'floating-resize') {
    store.setPosition({ width: clamp(gesture.width + dx, 300, Math.max(300, window.innerWidth - gesture.left - 8)), height: clamp(gesture.height + dy, 240, Math.max(240, window.innerHeight - gesture.top - 8)) })
  } else {
    resizeDock(gesture.size - (effectivePlacement.value === 'right' ? dx : dy))
  }
}
function resizeDock(size: number) {
  if (effectivePlacement.value === 'right') settingsStore.setAIWorkspaceRightWidth(size)
  else settingsStore.setAIWorkspaceBottomHeight(size)
}
function handleResizeKeydown(event: KeyboardEvent) {
  const delta = effectivePlacement.value === 'right' ? (event.key === 'ArrowLeft' ? 10 : event.key === 'ArrowRight' ? -10 : 0) : (event.key === 'ArrowUp' ? 10 : event.key === 'ArrowDown' ? -10 : 0)
  if (!delta) return
  event.preventDefault()
  resizeDock(effectiveSize.value + delta)
}
function end() {
  gesture = null
  window.removeEventListener('pointermove', move)
  window.removeEventListener('pointerup', end)
  window.removeEventListener('pointercancel', end)
}
watch(() => store.isFloating, end)
watch(() => noteStore.activeNote?.id, async () => { await nextTick(); measure() }, { flush: 'post' })
onMounted(() => { observer = new ResizeObserver(measure); if (panel.value?.parentElement) observer.observe(panel.value.parentElement); measure(); window.addEventListener('resize', measure) })
onBeforeUnmount(() => { end(); observer?.disconnect(); window.removeEventListener('resize', measure) })
</script>

<style scoped>
.support-workspace{position:relative;display:flex;min-width:0;min-height:0;flex:0 0 auto;flex-direction:column;border-left:1px solid var(--border);background:var(--bg-sidebar);color:var(--text-primary)}
.support-workspace.is-floating{position:fixed;z-index:1300;min-width:min(300px,calc(100vw - 16px));min-height:min(240px,calc(100vh - 16px));max-width:calc(100vw - 16px);max-height:calc(100vh - 16px);border:1px solid var(--border);border-radius:9px;box-shadow:0 18px 48px rgba(0,0,0,.34)}
.support-header{display:flex;min-height:42px;align-items:center;gap:5px;padding:5px 7px;border-bottom:1px solid var(--border);background:var(--bg-sidebar)}
.is-floating .support-header{cursor:move;user-select:none}
.support-header img{width:23px;height:23px;object-fit:contain;flex:none}
.support-header nav,.support-actions{display:flex;align-items:center;gap:2px;min-width:0}
.support-header nav{flex:1}
.support-header button{min-height:29px;padding:3px 7px;border:0;border-radius:5px;background:transparent;color:var(--text-secondary);cursor:pointer;white-space:nowrap}
.support-header nav button.active{background:var(--bg-active);color:var(--brand-primary);font-weight:600}
.support-actions button{display:grid;width:29px;padding:0;place-items:center}
.support-header button:focus-visible,.support-ai-disabled button:focus-visible{outline:2px solid var(--brand-primary)}
.support-content{display:flex;min-height:0;flex:1;flex-direction:column;overflow:hidden}
.support-ai-disabled{padding:16px;font-size:12px}.support-ai-disabled button{padding:7px;border:1px solid var(--border);border-radius:5px;background:var(--bg-input);color:var(--text-primary);cursor:pointer}
.support-restore{position:absolute;right:8px;z-index:30;display:grid;width:30px;height:30px;padding:0;place-items:center;border:1px solid var(--border);border-radius:6px;background:var(--bg-sidebar);color:var(--text-primary);cursor:pointer}
.support-floating-resize{position:absolute;right:0;bottom:0;width:18px;height:18px;border:0;background:linear-gradient(135deg,transparent 48%,var(--text-secondary) 52%,transparent 57%);cursor:nwse-resize;touch-action:none}
.support-dock-resize{position:absolute;top:0;bottom:0;left:-4px;width:8px;border:0;background:transparent;cursor:col-resize;touch-action:none}
.editor-workspace.is-bottom .support-dock-resize{top:-4px;right:0;bottom:auto;left:0;width:100%;height:8px;cursor:row-resize}
@media(max-width:700px){.support-workspace:not(.is-floating){width:100%!important;min-height:180px;height:min(40vh,340px)!important;flex-basis:min(40vh,340px)!important;border-top:1px solid var(--border);border-left:0}.support-workspace:not(.is-floating) .support-dock-resize{top:-4px;right:0;bottom:auto;left:0;width:100%;height:8px;cursor:row-resize}}
</style>
