import { ref } from 'vue'

export type AIActivityFlags = {
  isSubmitting?: boolean
  isSummaryBusy?: boolean
  isLibrarianBusy?: boolean
  isAssistantBusy?: boolean
  isWritingBusy?: boolean
  isApplyingAgentProposal?: boolean
  isSettingsBusy?: boolean
}

// The workspace owns the submission lifecycle, while the settings panel is a
// sibling component. Keep this small reactive bridge outside the stores so the
// setting can observe preparation and draft-flush time without introducing a
// store dependency cycle.
export const isAIComposerSubmitting = ref(false)
let activeComposerSubmissions = 0

export function beginAIComposerSubmission() {
  activeComposerSubmissions += 1
  isAIComposerSubmitting.value = true
}

export function endAIComposerSubmission() {
  activeComposerSubmissions = Math.max(0, activeComposerSubmissions - 1)
  isAIComposerSubmitting.value = activeComposerSubmissions > 0
}

export async function withAIComposerSubmission<T>(task: () => Promise<T>) {
  beginAIComposerSubmission()
  try {
    return await task()
  } finally {
    endAIComposerSubmission()
  }
}

export function isAIActivityBusy(flags: AIActivityFlags) {
  return Boolean(
    flags.isSubmitting
    || flags.isSummaryBusy
    || flags.isLibrarianBusy
    || flags.isAssistantBusy
    || flags.isWritingBusy
    || flags.isApplyingAgentProposal
    || flags.isSettingsBusy,
  )
}
