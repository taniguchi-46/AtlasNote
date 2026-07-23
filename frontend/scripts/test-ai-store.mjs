import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useAIStore.ts')
const panelPath = path.join(rootDir, 'src', 'components', 'AISettingsPanel.vue')
const editorPath = path.join(rootDir, 'src', 'components', 'NoteEditor.vue')
const outDir = path.join(rootDir, '.tmp', 'ai-store-test')
const outFile = path.join(outDir, 'useAIStore.mjs')

await mkdir(outDir, { recursive: true })
const source = (await readFile(sourcePath, 'utf8'))
  .replace("from '../api/ai'", "from './mock-ai.mjs'")
  .replace("from './useNotificationStore'", "from './mock-notifications.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
})
await writeFile(outFile, compiled.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-notifications.mjs'), `
export const notices = []
export function useNotificationStore() {
  return { notify(message, options) { notices.push({ message, options }) } }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-ai.mjs'), `
export const calls = { getSettings: 0, configure: [], test: [], list: [], generate: [], deleteProvider: [], deleteAll: 0 }
let settings = [
  { providerID: 'openrouter', modelID: 'retired-model', credentialStatus: 'persistent' },
  { providerID: 'gemini', modelID: '', credentialStatus: 'not-configured' },
]
let nextTestError
let nextModelResponse
let generateDeferred
const summaryResponses = []

function clone(value) { return JSON.parse(JSON.stringify(value)) }
function deferred() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  return { promise, resolve }
}

export function rejectNextConnectionCheck(error) { nextTestError = error }
export function setNextModelResponse(response) { nextModelResponse = response }
export function deferSummary() { generateDeferred = deferred(); return generateDeferred }
export function queueSummaryResponse(response) { summaryResponses.push(response) }

export async function getAISettings() {
  calls.getSettings += 1
  return clone(settings)
}
export async function configureAIProvider(input) {
  calls.configure.push(clone(input))
  settings = settings.map((setting) => setting.providerID === input.providerID
    ? { providerID: input.providerID, modelID: input.modelID, credentialStatus: 'persistent' }
    : setting)
  return clone(settings)
}
export async function testAIConnection(input) {
  calls.test.push(clone(input))
  if (nextTestError) {
    const error = nextTestError
    nextTestError = undefined
    throw error
  }
  return { success: true }
}
export async function listAIModels(input) {
  calls.list.push(clone(input))
  if (nextModelResponse) {
    const response = nextModelResponse
    nextModelResponse = undefined
    return clone(response)
  }
  return {
    models: [
      { id: 'summarizer-model', displayName: 'Summarizer', supportsSummary: true, available: true },
      { id: 'unknown-limit-model', displayName: 'Unknown limit', supportsSummary: true, available: true },
    ],
    retrievedAt: '2026-07-22T00:00:00Z',
  }
}
export async function generateAISummary(input) {
  calls.generate.push(clone(input))
  if (generateDeferred) {
    const waiting = generateDeferred
    generateDeferred = undefined
    await waiting.promise
  }
  return summaryResponses.length > 0 ? clone(summaryResponses.shift()) : { text: 'summary-result-marker' }
}
export async function deleteAIProviderCredential(providerID) {
  calls.deleteProvider.push(providerID)
  settings = settings.map((setting) => setting.providerID === providerID
    ? { providerID, modelID: '', credentialStatus: 'not-configured' }
    : setting)
  return clone(settings)
}
export async function deleteAllAICredentials() {
  calls.deleteAll += 1
  settings = settings.map((setting) => ({ ...setting, modelID: '', credentialStatus: 'not-configured' }))
  return clone(settings)
}
`, 'utf8')

try {
  const [panelSource, editorSource] = await Promise.all([
    readFile(panelPath, 'utf8'),
    readFile(editorPath, 'utf8'),
  ])
  assert.match(panelSource, /type="password"/, 'the API Key field must remain a password input')
  assert.match(panelSource, /window\.confirm/, 'credential deletion must require confirmation')
  assert.match(panelSource, /<details/, 'model metadata must be collapsible')
  assert.match(panelSource, /モデル詳細を表示/, 'the collapsed model metadata must remain discoverable')
  assert.doesNotMatch(panelSource, /<details[^>]*\bopen\b/, 'model metadata must start collapsed')
  assert.match(editorSource, /AIで要約/, 'the editor must expose an AI summary action')
  assert.match(editorSource, /flushPendingDraft/, 'summary must flush a draft before it is sent')
  assert.match(editorSource, /window\.confirm/, 'summary must require explicit confirmation')
  assert.match(editorSource, /事実を補わずに簡潔に要約/, 'confirmation must name the fixed summary instruction')
  assert.doesNotMatch(editorSource, /\$\{snapshot\.content\}/, 'confirmation must not echo the note body into another UI surface')
  assert.match(editorSource, /discardSummaryForActiveNote/, 'note changes must discard an in-memory summary')
  assert.match(editorSource, /isAISummaryStale/, 'a result must be marked stale when its revision is old')
  assert.match(editorSource, /handleCopyAISummary/, 'the result panel must only offer explicit copy handling')
  assert.doesNotMatch(source, /localStorage/, 'AI state must not be persisted in localStorage')

  setActivePinia(createPinia())
  const mockAI = await import(pathToFileURL(path.join(outDir, 'mock-ai.mjs')).href)
  const { notices } = await import(pathToFileURL(path.join(outDir, 'mock-notifications.mjs')).href)
  const { useAIStore } = await import(pathToFileURL(outFile).href)
  const store = useAIStore()

  await store.initialize()
  assert.equal(store.draft.apiKey, '', 'stored API keys must never be placed in the draft')
  assert.equal(store.draft.modelID, 'retired-model')

  store.draft.apiKey = 'test-key-marker'
  mockAI.rejectNextConnectionCheck(new Error('AI_AUTH_FAILED raw-provider-detail-marker'))
  assert.equal(await store.checkConnection(), false, 'a failed check must remain non-persistent')
  assert.equal(mockAI.calls.configure.length, 0, 'connection checking must not save settings')
  assert.equal(store.draft.apiKey, 'test-key-marker', 'a failed check must keep the draft for correction')
  assert.equal(store.connectionError.code, 'AI_AUTH_FAILED')
  assert.doesNotMatch(store.connectionError.message, /raw-provider-detail-marker/)

  assert.equal(await store.checkConnection(), true)
  assert.equal(mockAI.calls.test.length, 2)
  mockAI.setNextModelResponse({ models: [], error: { code: 'raw-provider-detail-marker' } })
  assert.equal(await store.refreshModels(), false)
  assert.equal(store.modelsError.code, 'AI_PROVIDER_UNAVAILABLE')
  assert.doesNotMatch(store.modelsError.message, /raw-provider-detail-marker/)
  assert.equal(await store.refreshModels(), true)
  assert.equal(store.draft.modelID, 'retired-model', 'an unavailable saved model must not be auto-selected')
  assert.equal(store.selectedModelAvailable, false, 'the user must explicitly reselect a listed model')

  store.draft.modelID = 'summarizer-model'
  assert.equal(store.canApply, true)
  assert.equal(await store.applyConfiguration(), true)
  assert.equal(mockAI.calls.configure.length, 1)
  assert.equal(mockAI.calls.configure[0].apiKey, 'test-key-marker')
  assert.equal(store.draft.apiKey, '', 'Apply must clear the in-memory API Key field')
  assert.equal(store.isSummaryReady, true, 'a saved, checked, listed model may be used for a summary')

  const sourceNote = { noteID: 'note-1', content: 'source-body-marker', baseRevision: 4 }
  assert.equal(store.beginSummary(sourceNote), true)
  assert.equal(store.summaryState, 'confirming')
  assert.equal(mockAI.calls.generate.length, 0, 'opening confirmation must not call the provider')
  store.cancelSummaryConfirmation()
  assert.equal(mockAI.calls.generate.length, 0, 'cancelling confirmation must not call the provider')

  assert.equal(store.beginSummary(sourceNote), true)
  assert.equal(await store.confirmSummary({
    noteID: 'note-1', content: 'source-body-marker', revision: 5, hasPendingDraft: false,
  }), false, 'a changed revision must invalidate the pending snapshot')
  assert.equal(store.summaryError.code, 'AI_DRAFT_NOT_SAVED')
  assert.equal(mockAI.calls.generate.length, 0, 'an invalidated snapshot must not call the provider')

  assert.equal(store.beginSummary(sourceNote), true)
  store.draft.modelID = 'unknown-limit-model'
  assert.equal(await store.confirmSummary({
    noteID: 'note-1', content: 'source-body-marker', revision: 4, hasPendingDraft: false,
  }), false, 'a provider/model change must invalidate the pending confirmation')
  assert.equal(mockAI.calls.generate.length, 0)
  store.draft.modelID = 'summarizer-model'

  const deferredSummary = mockAI.deferSummary()
  assert.equal(store.beginSummary(sourceNote), true)
  const pendingSummary = store.confirmSummary({
    noteID: 'note-1', content: 'source-body-marker', revision: 4, hasPendingDraft: false,
  })
  await Promise.resolve()
  assert.equal(store.isGenerating, true)
  assert.equal(store.beginSummary({ noteID: 'note-2', content: 'other-body-marker', baseRevision: 1 }), false, 'only one app-wide summary may execute')
  store.discardSummaryForActiveNote('note-2')
  deferredSummary.resolve()
  await pendingSummary
  assert.equal(store.summary, null, 'completion for an unseen note must be discarded')
  assert.equal(store.summaryState, 'idle')
  assert.match(notices.at(-1).message, /破棄/, 'an unseen result uses a generic notification')
  assert.doesNotMatch(JSON.stringify(notices), /source-body-marker|summary-result-marker/)

  mockAI.queueSummaryResponse({ error: { code: 'AI_RATE_LIMITED', retryAfterSeconds: 30 } })
  assert.equal(store.beginSummary(sourceNote), true)
  assert.equal(await store.confirmSummary({
    noteID: 'note-1', content: 'source-body-marker', revision: 4, hasPendingDraft: false,
  }), false)
  assert.equal(store.summaryError.code, 'AI_RATE_LIMITED')
  assert.equal(store.summaryError.retryAfterSeconds, 30)
  assert.match(store.summaryError.message, /30/)
  assert.doesNotMatch(store.summaryError.message, /source-body-marker/)

  assert.equal(store.beginSummary(sourceNote), true, 'retry must begin a new confirmation')
  assert.equal(mockAI.calls.generate.length, 2, 'retry must not call the provider before confirmation')
  store.cancelSummaryConfirmation()

  mockAI.queueSummaryResponse({ text: 'summary-result-marker' })
  assert.equal(store.beginSummary(sourceNote), true)
  assert.equal(await store.confirmSummary({
    noteID: 'note-1', content: 'source-body-marker', revision: 4, hasPendingDraft: false,
  }), true)
  assert.equal(store.summary.text, 'summary-result-marker')
  assert.equal(store.summary.baseRevision, 4, 'the result retains the immutable source revision')

  assert.equal(store.beginSummary({ noteID: 'note-1', content: 'x'.repeat(12 * 1024 + 1), baseRevision: 5 }), false)
  assert.equal(store.summaryError.code, 'AI_INPUT_TOO_LARGE')
  assert.equal(mockAI.calls.generate.length, 3, 'oversized input must not call the provider')
  store.discardSummary()
  assert.equal(store.summary, null)

  assert.equal(await store.deleteProvider('openrouter'), true)
  assert.deepEqual(mockAI.calls.deleteProvider, ['openrouter'])
  assert.equal(await store.deleteAllProviders(), true)
  assert.equal(mockAI.calls.deleteAll, 1)
  console.log('AI store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
