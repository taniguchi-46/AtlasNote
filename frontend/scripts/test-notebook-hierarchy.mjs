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
const tagManagerPath = path.join(rootDir, 'src', 'components', 'TagManager.vue')
const stylePath = path.join(rootDir, 'src', 'style.css')
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

  const [storeSource, treeItemSource, sidebarSource, tagManagerSource, styleSource] = await Promise.all([
    readFile(storePath, 'utf8'),
    readFile(treeItemPath, 'utf8'),
    readFile(sidebarPath, 'utf8'),
    readFile(tagManagerPath, 'utf8'),
    readFile(stylePath, 'utf8'),
  ])
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
  assert.match(treeItemSource, /const isChildrenExpanded = ref\(true\)/, 'subnotebooks must keep their own expanded state')
  assert.match(treeItemSource, /class="notebook-children-toggle"/, 'parents must expose a subnotebook disclosure control')
  assert.match(treeItemSource, /:aria-expanded="isChildrenExpanded"/, 'the subnotebook disclosure control must expose its state')
  assert.match(treeItemSource, /v-show="isChildrenExpanded"/, 'collapsing a notebook must preserve nested item state')
  assert.match(treeItemSource, /<NotebookIconPicker v-model="editIcon"/, 'icon selection must live in the notebook edit form')
  assert.doesNotMatch(treeItemSource, /isIconPickerOpen/, 'clicking the displayed notebook icon must not open a picker')
  assert.match(treeItemSource, /updateNotebookDetails\(props\.node\.id, input\)/, 'name and icon edits must share one notebook update')
  assert.match(sidebarSource, /@drop="handleRootDrop"/)
  assert.match(sidebarSource, /ルートへ移動/)
  assert.match(sidebarSource, /isNotebooksExpanded/, 'the notebook section must track its expanded state')
  assert.match(sidebarSource, /aria-expanded/, 'the notebook section toggle must expose its expanded state')
  assert.match(sidebarSource, /aria-controls="sidebar-notebooks-tree"/, 'the notebook section toggle must identify its controlled tree')
  assert.match(sidebarSource, /v-show="isNotebooksExpanded"/, 'collapsing the notebook section must preserve the tree component state')
  assert.match(styleSource, /\.sidebar\s*\{[^}]*min-height:\s*0;/, 'the sidebar must establish a shrink-safe scroll boundary')
  assert.match(styleSource, /\.new-note-btn\s*\{[^}]*flex-shrink:\s*0;[^}]*min-height:\s*36px;/, 'the notebook creation button must not shrink')
  assert.match(styleSource, /\.sidebar-nav\s*\{[^}]*flex-shrink:\s*0;/, 'navigation must not shrink before the sidebar scrolls')
  assert.match(styleSource, /\.sidebar-notebooks-section\s*\{[^}]*flex-shrink:\s*0;/, 'the notebook section must not shrink before the sidebar scrolls')
  assert.match(styleSource, /\.theme-toggle\s*\{[^}]*flex-shrink:\s*0;[^}]*min-height:\s*32px;/, 'the theme button must not shrink')
  assert.match(tagManagerSource, /\.tag-manager\s*\{[^}]*flex:\s*0\s+0\s+auto;/, 'the tag section must not shrink before its own list scrolls')
  assert.match(tagManagerSource, /const isTagsExpanded = ref\(true\)/, 'the tag section must track its expanded state')
  assert.match(tagManagerSource, /aria-controls="sidebar-tags-list"/, 'the tag toggle must identify its controlled list')
  assert.match(tagManagerSource, /v-show="isTagsExpanded"/, 'collapsing tags must preserve the tag list state')

  console.log('notebook hierarchy tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
}
