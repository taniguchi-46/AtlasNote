<template>
  <div class="ai-markdown-preview" :aria-label="ariaLabel">
    <EditorContent v-if="editor && !parseFailed" :editor="editor" class="ai-markdown-preview-content" />
    <pre v-else class="ai-markdown-preview-fallback">{{ markdown }}</pre>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { Link } from '@tiptap/extension-link'
import { Markdown } from 'tiptap-markdown'
import { RICH_MARKDOWN_OPTIONS } from '../utils/markdownSecurity'
import { parseNoteLinkHref } from '../utils/noteLink'

const props = withDefaults(defineProps<{
  markdown: string
  ariaLabel?: string
}>(), {
  ariaLabel: 'Markdownプレビュー',
})

const editor = shallowRef<Editor | null>(null)
const parseFailed = ref(false)

function updateMarkdown(markdown: string) {
  if (!editor.value) return
  try {
    const html = (editor.value.storage as any).markdown.parser.parse(markdown)
    ;(editor.value.commands as any).setContent(html, { emitUpdate: false })
    parseFailed.value = false
  } catch {
    // AI output is still useful as escaped plain text if it contains malformed
    // Markdown. Never fall back to rendering raw HTML.
    parseFailed.value = true
  }
}

onMounted(() => {
  editor.value = new Editor({
    editable: false,
    extensions: [
      StarterKit.configure({ link: false }),
      Markdown.configure(RICH_MARKDOWN_OPTIONS),
      Link.configure({
        openOnClick: false,
        protocols: ['atlasnote'],
        isAllowedUri: (url, context) => (
          url.startsWith('atlasnote:')
            ? parseNoteLinkHref(url) !== null
            : context.defaultValidate(url)
        ),
      }),
    ],
  })
  updateMarkdown(props.markdown)
})

watch(() => props.markdown, updateMarkdown)

onBeforeUnmount(() => {
  editor.value?.destroy()
  editor.value = null
})
</script>

<style scoped>
.ai-markdown-preview {
  min-width: 0;
}

.ai-markdown-preview-content :deep(.ProseMirror) {
  outline: none;
  overflow-wrap: anywhere;
  line-height: 1.6;
}

.ai-markdown-preview-content :deep(.ProseMirror > :first-child) {
  margin-top: 0;
}

.ai-markdown-preview-content :deep(.ProseMirror > :last-child) {
  margin-bottom: 0;
}

.ai-markdown-preview-content :deep(pre) {
  overflow-x: auto;
  padding: 8px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--bg-editor) 88%, var(--text-primary));
}

.ai-markdown-preview-content :deep(code) {
  overflow-wrap: anywhere;
}

.ai-markdown-preview-content :deep(a) {
  color: var(--brand-primary);
}

.ai-markdown-preview-fallback {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: inherit;
  line-height: 1.6;
}
</style>
