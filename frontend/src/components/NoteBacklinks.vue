<template>
  <div class="backlink-root">
    <PopoverRoot v-model:open="isOpen">
      <PopoverTrigger as-child>
        <button class="backlink-trigger" type="button" title="このノートへのリンク" aria-label="このノートへのバックリンク">
          <Link2Icon :size="15" />
          <span>バックリンク</span>
          <span v-if="linkStore.backlinkTotal > 0" class="backlink-count">{{ linkStore.backlinkTotal }}</span>
        </button>
      </PopoverTrigger>
      <PopoverPortal>
        <PopoverContent class="backlink-popover" side="top" align="start" :side-offset="8">
          <header class="backlink-header">
            <strong class="backlink-title">このノートへのリンク</strong>
            <div class="backlink-window-actions">
              <button
                type="button"
                :aria-label="isFloating ? 'バックリンクを通常表示に戻す' : 'バックリンクをフローティング表示'"
                :title="isFloating ? '通常表示へ戻す' : 'フローティング表示'"
                :aria-pressed="isFloating"
                @click="toggleFloating"
              >
                <AppWindowIcon :size="15" aria-hidden="true" />
              </button>
              <button type="button" aria-label="バックリンクを最小化" title="最小化" @click="minimizePanel">
                <MinusIcon :size="15" aria-hidden="true" />
              </button>
              <button type="button" aria-label="バックリンクを閉じる" title="閉じる" @click="isOpen = false">
                <XIcon :size="15" aria-hidden="true" />
              </button>
            </div>
          </header>
          <BacklinkPanelContent :note-id="props.noteId" @open-note="openNote" />
        </PopoverContent>
      </PopoverPortal>
    </PopoverRoot>

    <Teleport to="body">
      <button v-if="isMinimized" class="backlink-restore" type="button" @click="restorePanel">バックリンクを再開</button>
      <section
        v-if="isFloating && !isMinimized"
        class="backlink-floating-panel"
        :style="floatingStyle"
        aria-label="バックリンク"
      >
        <header class="backlink-header backlink-floating-header" @pointerdown="startMove">
          <strong class="backlink-title">このノートへのリンク</strong>
          <div class="backlink-window-actions" @pointerdown.stop>
            <button type="button" aria-label="バックリンクを通常配置へ戻す" title="通常配置へ戻す" :aria-pressed="isFloating" @click="toggleFloating">
              <AppWindowIcon :size="15" aria-hidden="true" />
            </button>
            <button type="button" aria-label="バックリンクを最小化" title="最小化" @click="minimizePanel">
              <MinusIcon :size="15" aria-hidden="true" />
            </button>
            <button type="button" aria-label="バックリンクを閉じる" title="閉じる" @click="closePanel">
              <XIcon :size="15" aria-hidden="true" />
            </button>
          </div>
        </header>
        <BacklinkPanelContent :note-id="props.noteId" @open-note="openNote" />
        <button class="backlink-floating-resizer" type="button" aria-label="バックリンクのサイズを変更" @pointerdown.stop.prevent="startResize" />
      </section>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { AppWindowIcon, Link2Icon, MinusIcon, XIcon } from '@lucide/vue'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { useNoteLinkStore } from '../stores/useNoteLinkStore'
import { useNoteStore } from '../stores/useNoteStore'
import BacklinkPanelContent from './BacklinkPanelContent.vue'

const props = defineProps<{ noteId: string }>()
const linkStore = useNoteLinkStore()
const noteStore = useNoteStore()
const isOpen = ref(false)
const isFloating = ref(false)
const isMinimized = ref(false)
const floating = ref({ left: 160, top: 120, width: 340, height: 420 })
const floatingStyle = computed(() => ({
  left: `${floating.value.left}px`, top: `${floating.value.top}px`,
  width: `${floating.value.width}px`, height: `${floating.value.height}px`,
}))

watch(() => props.noteId, (noteId) => { void linkStore.loadBacklinks(noteId) }, { immediate: true })

async function openNote(noteId: string) {
  await noteStore.selectNote(noteId)
  if (!isFloating.value) isOpen.value = false
}
function toggleFloating() {
  const returningToPopover = isFloating.value
  isFloating.value = !isFloating.value
  isOpen.value = returningToPopover
  isMinimized.value = false
}
function minimizePanel() { isOpen.value = false; isMinimized.value = true }
function restorePanel() { isMinimized.value = false; isOpen.value = true }
function closePanel() { isFloating.value = false; isMinimized.value = false; isOpen.value = false }

type Gesture = { kind: 'move' | 'resize'; x: number; y: number; left: number; top: number; width: number; height: number }
let gesture: Gesture | null = null
function startMove(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0 || (event.target as HTMLElement).closest('button')) return
  beginGesture('move', event)
}
function startResize(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return
  beginGesture('resize', event)
}
function beginGesture(kind: Gesture['kind'], event: PointerEvent) {
  gesture = { kind, x: event.clientX, y: event.clientY, ...floating.value }
  window.addEventListener('pointermove', updateGesture)
  window.addEventListener('pointerup', finishGesture, { once: true })
  window.addEventListener('pointercancel', finishGesture, { once: true })
}
function updateGesture(event: PointerEvent) {
  if (!gesture) return
  const dx = event.clientX - gesture.x
  const dy = event.clientY - gesture.y
  if (gesture.kind === 'move') {
    floating.value = {
      ...floating.value,
      left: Math.max(8, Math.min(window.innerWidth - 120, gesture.left + dx)),
      top: Math.max(8, Math.min(window.innerHeight - 48, gesture.top + dy)),
    }
    return
  }
  const left = Math.max(8, Math.min(window.innerWidth - 320, gesture.left))
  const top = Math.max(8, Math.min(window.innerHeight - 240, gesture.top))
  floating.value = {
    left,
    top,
    width: Math.max(320, Math.min(900, window.innerWidth - left - 8, gesture.width + dx)),
    height: Math.max(240, Math.min(800, window.innerHeight - top - 8, gesture.height + dy)),
  }
}
function finishGesture() {
  gesture = null
  window.removeEventListener('pointermove', updateGesture)
  window.removeEventListener('pointerup', finishGesture)
  window.removeEventListener('pointercancel', finishGesture)
}
onBeforeUnmount(finishGesture)
</script>

<style scoped>
.backlink-root { display: inline-flex; }

.backlink-trigger {
  display: inline-flex;
  min-height: 25px;
  align-items: center;
  gap: 4px;
  padding: 0 6px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}

.backlink-trigger:hover { color: var(--text-primary); background: var(--bg-hover); border-color: var(--border); }
.backlink-count { min-width: 16px; padding: 1px 4px; border-radius: 999px; background: var(--bg-hover); color: var(--text-primary); text-align: center; }

.backlink-popover {
  z-index: 1200;
  width: 290px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 12px 28px rgba(0, 0, 0, .28);
}

.backlink-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
.backlink-title { color: var(--text-primary); font-size: 13px; }
.backlink-window-actions { display: flex; flex-shrink: 0; gap: 3px; }

.backlink-window-actions button {
  display: inline-grid;
  width: 28px;
  height: 28px;
  place-items: center;
  padding: 3px 5px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 10px;
  cursor: pointer;
}

.backlink-window-actions button:hover { color: var(--text-primary); background: var(--bg-hover); }
.backlink-window-actions button:focus-visible { outline: 2px solid var(--brand-primary); outline-offset: 1px; }

.backlink-floating-panel {
  position: fixed;
  z-index: 1350;
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-editor);
  box-shadow: 0 16px 40px rgba(0, 0, 0, .32);
}

.backlink-floating-header { flex: 0 0 auto; cursor: move; user-select: none; }
.backlink-floating-panel :deep(.backlink-panel-content) { min-height: 0; overflow: auto; }

.backlink-floating-resizer {
  position: absolute;
  right: 1px;
  bottom: 1px;
  width: 18px;
  height: 18px;
  border: 0;
  background: linear-gradient(135deg, transparent 50%, var(--text-secondary) 52%, transparent 60%, transparent 68%, var(--text-secondary) 70%, transparent 78%);
  cursor: nwse-resize;
  touch-action: none;
}

.backlink-restore {
  position: fixed;
  right: 8px;
  bottom: 8px;
  z-index: 1340;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-editor);
  color: var(--text-primary);
  cursor: pointer;
}
</style>
