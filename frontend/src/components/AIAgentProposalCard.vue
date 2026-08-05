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

      <div class="ai-agent-proposal-diff" aria-label="本文の変更差分">
        <div>
          <span>変更前</span>
          <pre>{{ proposal.before }}</pre>
        </div>
        <div>
          <span>変更後</span>
          <pre>{{ proposal.after }}</pre>
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
  display: grid;
  gap: 8px;
}

.ai-agent-proposal-diff > div {
  display: grid;
  gap: 4px;
}

.ai-agent-proposal-diff span {
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.ai-agent-proposal-diff pre {
  max-height: 180px;
  margin: 0;
  padding: 8px;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-editor, var(--bg-input));
  color: var(--text-primary);
  font: inherit;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
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

@container (max-width: 420px) {
  .ai-agent-proposal {
    padding: 9px;
  }
}
</style>
