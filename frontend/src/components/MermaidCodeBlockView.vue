<template>
  <NodeViewWrapper as="pre" class="code-block-node-view"><NodeViewContent as="code" /><div
      v-if="isMermaid"
      class="mermaid-code-block-preview"
      contenteditable="false"
      :aria-busy="status === 'loading'"
    >
      <p v-if="status === 'loading'" class="mermaid-code-block-status" role="status">
        Mermaid図を描画しています…
      </p>
      <p v-else-if="status === 'error'" class="mermaid-code-block-error" role="alert">
        {{ errorMessage }}
      </p>
      <img v-else-if="svgUrl" :src="svgUrl" :alt="altText" class="mermaid-code-block-image" />
    </div></NodeViewWrapper>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NodeViewContent, NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { useAppStore } from '../stores/useAppStore'
import { renderMermaidDiagram } from '../utils/mermaidRenderer'

const props = defineProps(nodeViewProps)
const appStore = useAppStore()

const isMermaid = computed(() => {
  const language = String(props.node.attrs.language ?? '').trim().toLowerCase()
  return language === 'mermaid'
})

const status = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
const errorMessage = ref('')
const svgUrl = ref<string | null>(null)
const altText = ref('Mermaid図')

let renderTimer: ReturnType<typeof setTimeout> | null = null
let renderGeneration = 0
let currentObjectUrl: string | null = null

function revokeObjectUrl() {
  if (!currentObjectUrl) return
  if (typeof URL !== 'undefined' && typeof URL.revokeObjectURL === 'function') {
    URL.revokeObjectURL(currentObjectUrl)
  }
  currentObjectUrl = null
}

function clearPreview() {
  revokeObjectUrl()
  svgUrl.value = null
  altText.value = 'Mermaid図'
}

function scheduleRender() {
  renderGeneration += 1
  const generation = renderGeneration

  if (renderTimer) {
    clearTimeout(renderTimer)
    renderTimer = null
  }

  if (!isMermaid.value) {
    status.value = 'idle'
    errorMessage.value = ''
    clearPreview()
    return
  }

  status.value = 'loading'
  errorMessage.value = ''
  renderTimer = setTimeout(() => {
    renderTimer = null
    void render(generation)
  }, 250)
}

async function render(generation: number) {
  let result: Awaited<ReturnType<typeof renderMermaidDiagram>>
  try {
    result = await renderMermaidDiagram(props.node.textContent, {
      theme: appStore.theme,
    })
  } catch {
    if (generation !== renderGeneration) return
    clearPreview()
    status.value = 'error'
    errorMessage.value = 'Mermaidを読み込めないため表示できません。'
    return
  }
  if (generation !== renderGeneration) return

  if (!result.ok) {
    clearPreview()
    status.value = 'error'
    errorMessage.value = result.message
    return
  }

  if (typeof URL === 'undefined' || typeof URL.createObjectURL !== 'function') {
    clearPreview()
    status.value = 'error'
    errorMessage.value = '図の出力を安全に表示できませんでした。'
    return
  }

  let nextObjectUrl: string
  try {
    nextObjectUrl = URL.createObjectURL(new Blob([result.svg], { type: 'image/svg+xml' }))
  } catch {
    clearPreview()
    status.value = 'error'
    errorMessage.value = '図の出力を安全に表示できませんでした。'
    return
  }
  if (generation !== renderGeneration) {
    if (typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(nextObjectUrl)
    return
  }

  revokeObjectUrl()
  currentObjectUrl = nextObjectUrl
  svgUrl.value = nextObjectUrl
  altText.value = result.altText
  status.value = 'ready'
}

watch(
  () => [
    props.node.attrs.language,
    props.node.textContent,
    appStore.theme,
  ],
  scheduleRender,
  { immediate: true },
)

onBeforeUnmount(() => {
  renderGeneration += 1
  if (renderTimer) clearTimeout(renderTimer)
  renderTimer = null
  clearPreview()
})
</script>
