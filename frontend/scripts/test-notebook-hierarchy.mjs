import assert from 'node:assert/strict'
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'notebookHierarchy.ts')
const storePath = path.join(rootDir, 'src', 'stores', 'useNotebookStore.ts')
const treeItemPath = path.join(rootDir, 'src', 'components', 'NotebookTreeItem.vue')
const sidebarPath = path.join(rootDir, 'src', 'components', 'AppSidebar.vue')
const outDir = path.join(rootDir, '.tmp', 'notebook-hierarchy-test')
const outFile = path.join(outDir, 'notebookHierarchy.mjs')

await mkdir(outDir, { recursive: true })

const source = await readFile(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})

await writeFile(outFile, compiled.outputText, 'utf8')

const { wouldCreateNotebookCycle } = await import(pathToFileURL(outFile).href)

try {
  const notebooks = [
    { id: 'parent' },
    { id: 'child', parentId: 'parent' },
    { id: 'grandchild', parentId: 'child' },
    { id: 'other-root' },
  ]

  assert.equal(wouldCreateNotebookCycle(notebooks, 'parent', 'parent'), true)
  assert.equal(wouldCreateNotebookCycle(notebooks, 'parent', 'child'), true)
  assert.equal(wouldCreateNotebookCycle(notebooks, 'parent', 'grandchild'), true)
  assert.equal(wouldCreateNotebookCycle(notebooks, 'parent', 'other-root'), false)
  assert.equal(wouldCreateNotebookCycle(notebooks, 'parent', null), false)

  const storeSource = await readFile(storePath, 'utf8')
  const treeItemSource = await readFile(treeItemPath, 'utf8')
  const sidebarSource = await readFile(sidebarPath, 'utf8')
  const validationIndex = storeSource.indexOf('wouldCreateNotebookCycle(notebooks.value, id, parentId)')
  const updateIndex = storeSource.indexOf('const updated = await updateNotebook(', validationIndex)
  assert.notEqual(validationIndex, -1)
  assert.ok(updateIndex > validationIndex, 'cycle validation must run before the update API call')
  assert.match(storeSource, /useNotificationStore/)
  assert.match(storeSource, /NOTEBOOK_LIST_FAILED/)
  assert.match(storeSource, /NOTEBOOK_MOVE_INVALID/)
  assert.match(storeSource, /run: \(\) => fetchNotebooks/)
  assert.match(storeSource, /draggedNotebookId/)
  assert.match(treeItemSource, /:draggable="!isEditing"/)
  assert.match(treeItemSource, /@dragstart="handleDragStart"/)
  assert.match(treeItemSource, /@drop\.stop\.prevent="handleDrop"/)
  assert.match(treeItemSource, /wouldCreateNotebookCycle\(notebookStore\.notebooks, draggedId, props\.node\.id\)/)
  assert.match(sidebarSource, /@drop="handleRootDrop"/)
  assert.match(sidebarSource, /ルートへ移動/)

  console.log('notebook hierarchy tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
