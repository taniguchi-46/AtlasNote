<template>
  <section
    :class="['ai-agent-proposal', `is-${state}`]"
    :aria-busy="state === 'generating' || state === 'applying'"
    aria-label="Agentの本文変更提案"
  >
    <header class="ai-agent-proposal-header">
      <div>
        <strong>Agentの変更提案</strong>
        <span>{{ statusLabel }}</span>
      </div>
      <LoaderCircleIcon
        v-if="state === 'generating' || state === 'applying'"
        class="is-spinning"
        :size="16"
        aria-hidden="true"
      />
      <CheckIcon v-else-if="state === 'applied'" :size="16" aria-hidden="true" />
      <AlertCircleIcon v-else-if="state === 'conflict' || state === 'save-failure'" :size="16" aria-hidden="true" />
      <XIcon v-else-if="state === 'discarded'" :size="16" aria-hidden="true" />
    </header>

    <p class="ai-agent-proposal-message">{{ content }}</p>

    <template v-if="proposal">
      <dl class="ai-agent-proposal-meta">
        <div>
          <dt>対象</dt>
          <dd>{{ proposal.targetTitle || '無題のノート' }}</dd>
        </div>
        <div>
          <dt>revision</dt>
          <dd>{{ proposal.baseRevision }}</dd>
        </div>
        <div>
          <dt>変更箇所</dt>
          <dd>本文</dd>
        </div>
      </dl>

      <p class="ai-agent-proposal-reason"><strong>理由:</strong> {{ proposal.reason }}</p>

      <div class="ai-agent-proposal-diff">
        <header class="ai-agent-proposal-diff-summary">
          <strong>本文の差分</strong>
          <span>1件の変更</span>
        </header>
        <div
          class="ai-agent-proposal-diff-scroll"
          role="region"
          tabindex="0"
          aria-label="本文の変更差分。削除される本文と追加される本文"
        >
          <div class="ai-agent-proposal-diff-grid">
            <section class="ai-agent-proposal-diff-pane is-before" aria-label="削除される本文">
              <header class="ai-agent-proposal-diff-pane-header">
                <span class="ai-agent-proposal-diff-pane-title">
                  <span class="ai-agent-proposal-diff-pane-marker" aria-hidden="true">−</span>
                  <strong>変更前</strong>
                </span>
                <span class="ai-agent-proposal-diff-line-scope">相対行</span>
              </header>
              <ol class="ai-agent-proposal-diff-lines">
                <li
                  v-for="line in visualDiff.beforeLines"
                  :key="`before-${line.rowNumber}`"
                  :class="[
                    'ai-agent-proposal-diff-line',
                    { 'is-removed': line.changed, 'is-placeholder': line.placeholder },
                  ]"
                  :style="{ gridRow: line.rowNumber + 1 }"
                  :aria-hidden="line.placeholder"
                >
                  <span v-if="!line.placeholder" class="ai-agent-proposal-visually-hidden">
                    {{ line.changed ? '削除行' : '変更なし' }}、相対{{ line.lineNumber }}行目:
                  </span>
                  <span v-if="!line.placeholder" class="ai-agent-proposal-diff-line-number" aria-hidden="true">{{ line.lineNumber }}</span>
                  <span v-if="!line.placeholder" class="ai-agent-proposal-diff-line-marker" aria-hidden="true">{{ line.changed ? '−' : '' }}</span>
                  <code v-if="!line.placeholder">
                    <span
                      v-for="(segment, segmentIndex) in line.segments"
                      :key="segmentIndex"
                      :class="{ 'is-word-change': segment.changed }"
                    >{{ segment.text }}</span>
                    <span v-if="line.text === ''" class="is-empty" aria-hidden="true">↵</span>
                    <span v-if="line.text === ''" class="ai-agent-proposal-visually-hidden">空行</span>
                  </code>
                </li>
              </ol>
            </section>

            <section class="ai-agent-proposal-diff-pane is-after" aria-label="追加される本文">
              <header class="ai-agent-proposal-diff-pane-header">
                <span class="ai-agent-proposal-diff-pane-title">
                  <span class="ai-agent-proposal-diff-pane-marker" aria-hidden="true">+</span>
                  <strong>変更後</strong>
                </span>
                <span class="ai-agent-proposal-diff-line-scope">相対行</span>
              </header>
              <ol class="ai-agent-proposal-diff-lines">
                <li
                  v-for="line in visualDiff.afterLines"
                  :key="`after-${line.rowNumber}`"
                  :class="[
                    'ai-agent-proposal-diff-line',
                    { 'is-added': line.changed, 'is-placeholder': line.placeholder },
                  ]"
                  :style="{ gridRow: line.rowNumber + 1 }"
                  :aria-hidden="line.placeholder"
                >
                  <span v-if="!line.placeholder" class="ai-agent-proposal-visually-hidden">
                    {{ line.changed ? '追加行' : '変更なし' }}、相対{{ line.lineNumber }}行目:
                  </span>
                  <span v-if="!line.placeholder" class="ai-agent-proposal-diff-line-number" aria-hidden="true">{{ line.lineNumber }}</span>
                  <span v-if="!line.placeholder" class="ai-agent-proposal-diff-line-marker" aria-hidden="true">{{ line.changed ? '+' : '' }}</span>
                  <code v-if="!line.placeholder">
                    <span
                      v-for="(segment, segmentIndex) in line.segments"
                      :key="segmentIndex"
                      :class="{ 'is-word-change': segment.changed }"
                    >{{ segment.text }}</span>
                    <span v-if="line.text === ''" class="is-empty" aria-hidden="true">↵</span>
                    <span v-if="line.text === ''" class="ai-agent-proposal-visually-hidden">空行</span>
                  </code>
                </li>
              </ol>
            </section>
          </div>
        </div>
      </div>
    </template>

    <div v-if="canApply || canDiscard" class="ai-agent-proposal-actions">
      <button
        v-if="canApply"
        class="is-primary"
        type="button"
        :disabled="busy"
        @click="emit('apply')"
      >
        {{ state === 'save-failure' ? '保存を再試行' : '本文へ適用' }}
      </button>
      <button
        v-if="canDiscard"
        type="button"
        :disabled="busy"
        @click="emit('discard')"
      >
        提案を破棄
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { AlertCircleIcon, CheckIcon, LoaderCircleIcon, XIcon } from '@lucide/vue'
import type { AgentEditProposal } from '../api/ai'
import type { AgentProposalState } from '../stores/useAIChatStore'
import { createAgentEditVisualDiff } from '../utils/agentEditProposal'

const props = withDefaults(defineProps<{
  content: string
  proposal?: AgentEditProposal
  state: AgentProposalState
  busy?: boolean
}>(), {
  proposal: undefined,
  busy: false,
})

const emit = defineEmits<{
  apply: []
  discard: []
}>()

const statusLabel = computed(() => ({
  generating: '提案を生成中',
  'awaiting-review': '確認待ち',
  applying: '適用中',
  applied: '適用済み',
  conflict: '競合のため適用不可',
  'save-failure': '保存に失敗しました',
  discarded: '破棄済み',
}[props.state]))

const canApply = computed(() => (
  Boolean(props.proposal) && (props.state === 'awaiting-review' || props.state === 'save-failure')
))
const canDiscard = computed(() => (
  Boolean(props.proposal) && props.state !== 'applying' && props.state !== 'applied'
))
const visualDiff = computed(() => (
  props.proposal
    ? createAgentEditVisualDiff(props.proposal.before, props.proposal.after)
    : { beforeLines: [], afterLines: [] }
))
</script>

<style scoped>
.ai-agent-proposal {
  display: grid;
  gap: 10px;
  padding: 11px;
  border: 1px solid color-mix(in srgb, var(--brand-primary) 30%, var(--border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--bg-input) 88%, var(--brand-primary) 4%);
  color: var(--text-primary);
  font-size: 12px;
}

.ai-agent-proposal.is-conflict,
.ai-agent-proposal.is-save-failure {
  border-color: color-mix(in srgb, var(--color-warning, #8a5a00) 44%, var(--border));
}

.ai-agent-proposal.is-applied {
  border-color: color-mix(in srgb, var(--brand-primary) 50%, var(--border));
}

.ai-agent-proposal-header,
.ai-agent-proposal-header > div,
.ai-agent-proposal-actions {
  display: flex;
  align-items: center;
}

.ai-agent-proposal-header {
  justify-content: space-between;
  gap: 8px;
}

.ai-agent-proposal-header > div {
  min-width: 0;
  gap: 7px;
}

.ai-agent-proposal-header span {
  color: var(--text-secondary);
  font-size: 11px;
}

.ai-agent-proposal-message,
.ai-agent-proposal-reason {
  margin: 0;
  overflow-wrap: anywhere;
  line-height: 1.5;
}

.ai-agent-proposal-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px 12px;
  margin: 0;
  color: var(--text-secondary);
}

.ai-agent-proposal-meta div {
  display: inline-flex;
  gap: 4px;
}

.ai-agent-proposal-meta dt,
.ai-agent-proposal-meta dd {
  margin: 0;
}

.ai-agent-proposal-meta dt::after {
  content: ':';
}

.ai-agent-proposal-diff {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--bg-editor, var(--bg-input));
}

.ai-agent-proposal-diff-summary,
.ai-agent-proposal-diff-pane-header,
.ai-agent-proposal-diff-pane-title {
  display: flex;
  align-items: center;
}

.ai-agent-proposal-diff-summary {
  justify-content: space-between;
  gap: 8px;
  min-height: 31px;
  padding: 6px 9px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg-input) 86%, var(--brand-primary) 4%);
}

.ai-agent-proposal-diff-summary strong {
  font-size: 11px;
}

.ai-agent-proposal-diff-summary > span {
  padding: 1px 6px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg-input);
  color: var(--text-secondary);
  font-size: 10px;
}

.ai-agent-proposal-diff-scroll {
  max-height: 300px;
  overflow: auto;
  overscroll-behavior: contain;
  outline: none;
  scrollbar-gutter: stable;
}

.ai-agent-proposal-diff-scroll:focus-visible {
  box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--brand-primary) 55%, transparent);
}

.ai-agent-proposal-diff-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  min-width: 0;
}

.ai-agent-proposal-diff-pane {
  min-width: 0;
  background: var(--bg-editor, var(--bg-input));
}

.ai-agent-proposal-diff-pane.is-after {
  border-top: 1px solid var(--border);
}

.ai-agent-proposal-diff-pane-header {
  position: sticky;
  z-index: 1;
  top: 0;
  justify-content: space-between;
  gap: 8px;
  min-height: 29px;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border);
  backdrop-filter: blur(6px);
}

.ai-agent-proposal-diff-pane.is-before .ai-agent-proposal-diff-pane-header {
  background: color-mix(in srgb, var(--bg-editor) 88%, var(--color-danger) 12%);
  color: var(--text-primary);
}

.ai-agent-proposal-diff-pane.is-after .ai-agent-proposal-diff-pane-header {
  background: color-mix(in srgb, var(--bg-editor) 88%, var(--color-success) 12%);
  color: var(--text-primary);
}

.ai-agent-proposal-diff-pane.is-before .ai-agent-proposal-diff-pane-marker {
  color: var(--color-danger);
}

.ai-agent-proposal-diff-pane.is-after .ai-agent-proposal-diff-pane-marker {
  color: var(--color-success);
}

.ai-agent-proposal-diff-pane-title {
  min-width: 0;
  gap: 6px;
}

.ai-agent-proposal-diff-pane-title strong {
  font-size: 11px;
  font-weight: 600;
}

.ai-agent-proposal-diff-pane-marker {
  display: inline-grid;
  width: 16px;
  height: 16px;
  place-items: center;
  border: 1px solid currentColor;
  border-radius: 4px;
  font-family: ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
  line-height: 1;
}

.ai-agent-proposal-diff-line-scope {
  color: var(--text-secondary);
  font-size: 9px;
  font-weight: 500;
}

.ai-agent-proposal-diff-lines {
  margin: 0;
  padding: 0;
  list-style: none;
}

.ai-agent-proposal-diff-line {
  display: grid;
  grid-template-columns: 32px 18px minmax(0, 1fr);
  min-height: 25px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 58%, transparent);
}

.ai-agent-proposal-diff-line.is-placeholder {
  display: none;
}

.ai-agent-proposal-diff-line:last-child {
  border-bottom: 0;
}

.ai-agent-proposal-diff-line.is-removed {
  background: color-mix(in srgb, var(--bg-editor) 85%, var(--color-danger) 15%);
  box-shadow: inset 3px 0 0 var(--color-danger);
}

.ai-agent-proposal-diff-line.is-added {
  background: color-mix(in srgb, var(--bg-editor) 83%, var(--color-success) 17%);
  box-shadow: inset 3px 0 0 var(--color-success);
}

.ai-agent-proposal-diff-line-number,
.ai-agent-proposal-diff-line-marker {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  padding-top: 4px;
  font-family: ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
  font-size: 10px;
  line-height: 1.6;
  user-select: none;
}

.ai-agent-proposal-diff-line-number {
  padding-right: 7px;
  border-right: 1px solid color-mix(in srgb, var(--border) 74%, transparent);
  background: color-mix(in srgb, var(--bg-input) 82%, transparent);
  color: var(--text-muted);
}

.ai-agent-proposal-diff-line-marker {
  justify-content: center;
  padding-right: 0;
  color: var(--text-muted);
  font-weight: 700;
}

.ai-agent-proposal-diff-line.is-removed .ai-agent-proposal-diff-line-marker {
  color: var(--color-danger);
}

.ai-agent-proposal-diff-line.is-added .ai-agent-proposal-diff-line-marker {
  color: var(--color-success);
}

.ai-agent-proposal-diff-line.is-removed .ai-agent-proposal-diff-line-number {
  background: color-mix(in srgb, var(--bg-editor) 78%, var(--color-danger) 22%);
}

.ai-agent-proposal-diff-line.is-added .ai-agent-proposal-diff-line-number {
  background: color-mix(in srgb, var(--bg-editor) 76%, var(--color-success) 24%);
}

.ai-agent-proposal-diff-line code {
  display: block;
  min-width: 0;
  padding: 4px 7px;
  color: var(--text-primary);
  font-family: ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
  font-size: 10.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.ai-agent-proposal-diff-line .is-word-change {
  border-radius: 2px;
  box-decoration-break: clone;
  -webkit-box-decoration-break: clone;
  font-weight: 600;
}

.ai-agent-proposal-diff-line.is-removed .is-word-change {
  background: color-mix(in srgb, var(--bg-editor) 62%, var(--color-danger) 38%);
}

.ai-agent-proposal-diff-line.is-added .is-word-change {
  background: color-mix(in srgb, var(--bg-editor) 60%, var(--color-success) 40%);
}

.ai-agent-proposal-diff-line .is-empty {
  color: var(--text-muted);
  font-style: normal;
}

.ai-agent-proposal-visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.ai-agent-proposal-actions {
  flex-wrap: wrap;
  gap: 7px;
}

.ai-agent-proposal-actions button {
  min-height: 29px;
  padding: 0 9px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.ai-agent-proposal-actions button.is-primary {
  border-color: var(--brand-primary);
  background: var(--brand-primary);
  color: #fff;
}

.ai-agent-proposal-actions button:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.is-spinning {
  animation: agent-proposal-spin .8s linear infinite;
}

@keyframes agent-proposal-spin {
  to { transform: rotate(360deg); }
}

@container (min-width: 520px) {
  .ai-agent-proposal-diff-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ai-agent-proposal-diff-pane,
  .ai-agent-proposal-diff-lines {
    display: contents;
  }

  .ai-agent-proposal-diff-pane-header {
    grid-row: 1;
  }

  .ai-agent-proposal-diff-pane.is-before .ai-agent-proposal-diff-pane-header,
  .ai-agent-proposal-diff-pane.is-before .ai-agent-proposal-diff-line {
    grid-column: 1;
  }

  .ai-agent-proposal-diff-pane.is-after .ai-agent-proposal-diff-pane-header,
  .ai-agent-proposal-diff-pane.is-after .ai-agent-proposal-diff-line {
    grid-column: 2;
    border-left: 1px solid var(--border);
  }

  .ai-agent-proposal-diff-pane.is-after {
    border-top: 0;
  }

  .ai-agent-proposal-diff-line.is-placeholder {
    display: grid;
    visibility: hidden;
  }
}

@container (max-width: 420px) {
  .ai-agent-proposal {
    padding: 9px;
  }

  .ai-agent-proposal-diff-line {
    grid-template-columns: 28px 16px minmax(0, 1fr);
  }
}
</style>
