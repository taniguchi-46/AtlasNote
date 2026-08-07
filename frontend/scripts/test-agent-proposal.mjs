import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const utilityPath = path.join(rootDir, 'src', 'utils', 'agentEditProposal.ts')
const noteStorePath = path.join(rootDir, 'src', 'stores', 'useNoteStore.ts')
const proposalCardPath = path.join(rootDir, 'src', 'components', 'AIAgentProposalCard.vue')
const outDir = path.join(rootDir, '.tmp', 'agent-proposal-test')
const outFile = path.join(outDir, 'agentEditProposal.mjs')

const [utilitySource, noteStoreSource, proposalCardSource] = await Promise.all([
  readFile(utilityPath, 'utf8'),
  readFile(noteStorePath, 'utf8'),
  readFile(proposalCardPath, 'utf8'),
])

assert.match(noteStoreSource, /function applyAgentEditProposal/)
assert.match(noteStoreSource, /noteOperations\.enqueue\(noteId/)
assert.match(noteStoreSource, /expectedRevision: proposal\.baseRevision/)
assert.match(noteStoreSource, /NoteRevisionConflictError/)
assert.match(noteStoreSource, /applyAgentEditHunk\(latest\.content, proposal\)/)
assert.match(noteStoreSource, /proposal\.targetNoteID\.trim\(\)/)
assert.match(noteStoreSource, /return 'applied'/)
assert.match(noteStoreSource, /return 'save-failure'/)
assert.doesNotMatch(proposalCardSource, /v-html/)
assert.match(proposalCardSource, /<pre>\{\{ proposal\.before \}\}<\/pre>/)
assert.match(proposalCardSource, /<pre>\{\{ proposal\.after \}\}<\/pre>/)
assert.match(proposalCardSource, /本文へ適用/)
assert.match(proposalCardSource, /提案を破棄/)

await mkdir(outDir, { recursive: true })
const compiled = ts.transpileModule(utilitySource, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
  fileName: utilityPath,
})
await writeFile(outFile, compiled.outputText, 'utf8')

try {
  const { applyAgentEditHunk } = await import(pathToFileURL(outFile).href)

  assert.deepEqual(
    applyAgentEditHunk('前文\n変更前\n後文', { before: '変更前', after: '変更後' }),
    { status: 'ok', content: '前文\n変更後\n後文' },
  )
  assert.deepEqual(
    applyAgentEditHunk('変更前と変更前', { before: '変更前', after: '変更後' }),
    { status: 'conflict', reason: 'ambiguous' },
  )
  assert.deepEqual(
    applyAgentEditHunk('別本文', { before: '変更前', after: '変更後' }),
    { status: 'conflict', reason: 'not-found' },
  )
  assert.deepEqual(
    applyAgentEditHunk('', { before: '', after: '新規本文' }),
    { status: 'conflict', reason: 'invalid-empty-target' },
  )
  assert.deepEqual(
    applyAgentEditHunk('同じ本文', { before: '同じ本文', after: '同じ本文' }),
    { status: 'conflict', reason: 'unchanged' },
  )
  assert.deepEqual(
    applyAgentEditHunk('見出し\n本文\n末尾', { before: '本文\n', after: '<安全な置換>\n' }),
    { status: 'ok', content: '見出し\n<安全な置換>\n末尾' },
  )

  console.log('Agent proposal tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
