import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import { createPinia, setActivePinia } from 'pinia'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'stores', 'useTagStore.ts')
const outDir = path.join(rootDir, '.tmp', 'tag-store-test')
const outFile = path.join(outDir, 'useTagStore.mjs')

await mkdir(outDir, { recursive: true })

const source = (await readFile(sourcePath, 'utf8'))
  .replace("from '../api/tags'", "from './mock-tags.mjs'")
  .replace("from './useNotificationStore'", "from './mock-notifications.mjs'")
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')
await writeFile(path.join(outDir, 'mock-notifications.mjs'), `
export function useNotificationStore() {
  return {
    notify() {},
    dismissBySource() {},
  }
}
`, 'utf8')
await writeFile(path.join(outDir, 'mock-tags.mjs'), `
export class TagApiError extends Error {
  constructor(error) {
    super(error.message)
    this.code = error.code
    this.field = error.field
    this.retryable = error.retryable
  }
}

export const mock = {
  listTags: async () => [],
  listNoteTags: async () => [],
  createTag: async () => ({ id: 'created', name: 'Created' }),
  updateTag: async () => ({ id: 'updated', name: 'Updated' }),
  deleteTag: async () => {},
  setNoteTags: async () => [],
}

export function configure(next) {
  Object.assign(mock, next)
}

export function listTags(...args) { return mock.listTags(...args) }
export function listNoteTags(...args) { return mock.listNoteTags(...args) }
export function createTag(...args) { return mock.createTag(...args) }
export function updateTag(...args) { return mock.updateTag(...args) }
export function deleteTag(...args) { return mock.deleteTag(...args) }
export function setNoteTags(...args) { return mock.setNoteTags(...args) }
`, 'utf8')

try {
  setActivePinia(createPinia())
  const { configure } = await import(pathToFileURL(path.join(outDir, 'mock-tags.mjs')).href)
  const { useTagStore } = await import(pathToFileURL(outFile).href)
  const store = useTagStore()
  const alpha = { id: 'alpha', name: 'Alpha' }
  const beta = { id: 'beta', name: 'Beta' }
  const zulu = { id: 'zulu', name: 'Zulu' }

  configure({
    listTags: async () => [zulu],
    createTag: async () => beta,
  })
  await store.fetchTags()
  await store.createTag('Beta')
  assert.deepEqual(store.tags.map((tag) => tag.id), ['beta', 'zulu'])

  const setRequests = []
  configure({
    listNoteTags: async (noteId) => noteId === 'note-1' ? [alpha] : [],
    setNoteTags: async (noteId, input) => {
      setRequests.push({ noteId, tagIds: input.tagIds })
      return input.tagIds.map((tagId) => ({ alpha, beta })[tagId])
    },
  })
  await store.loadNoteTags('note-1')
  await store.attachTagToNote('note-1', beta.id)
  assert.deepEqual(setRequests, [{ noteId: 'note-1', tagIds: ['alpha', 'beta'] }])
  assert.deepEqual(store.activeNoteTags.map((tag) => tag.id), ['alpha', 'beta'])

  await store.attachTagToNote('note-1', beta.id)
  assert.equal(setRequests.length, 1, 'an attached tag must not be sent twice')

  let resolveFirstLoad
  configure({
    listNoteTags: (noteId) => {
      if (noteId === 'first-note') {
        return new Promise((resolve) => {
          resolveFirstLoad = resolve
        })
      }
      return Promise.resolve([beta])
    },
  })
  const firstLoad = store.loadNoteTags('first-note')
  await store.loadNoteTags('second-note')
  resolveFirstLoad([alpha])
  await firstLoad
  assert.equal(store.activeNoteId, 'second-note')
  assert.deepEqual(store.activeNoteTags.map((tag) => tag.id), ['beta'])

  console.log('tag store tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
