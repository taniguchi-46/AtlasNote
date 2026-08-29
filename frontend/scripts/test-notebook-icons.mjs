import assert from 'node:assert/strict'
import { mkdir, readdir, readFile, rm, stat, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'

const rootDir = process.cwd()
const sourcePath = path.join(rootDir, 'src', 'utils', 'notebookIcons.ts')
const pickerPath = path.join(rootDir, 'src', 'components', 'NotebookIconPicker.vue')
const treeItemPath = path.join(rootDir, 'src', 'components', 'NotebookTreeItem.vue')
const stylePath = path.join(rootDir, 'src', 'style.css')
const assetDir = path.join(rootDir, 'src', 'assets', 'notebook-icons')
const outDir = path.join(rootDir, '.tmp', 'notebook-icons-test')
const outFile = path.join(outDir, 'notebookIcons.mjs')
const existingAssetFiles = new Set([
  'simpleCalendar.png',
  'simpleLight.png',
  'simpleNote.png',
  'simplePen.png',
  'simpleTask.png',
])

function readPngDimensions(buffer) {
  assert.deepEqual([...buffer.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10])
  assert.equal(buffer.toString('ascii', 12, 16), 'IHDR')
  return {
    width: buffer.readUInt32BE(16),
    height: buffer.readUInt32BE(20),
  }
}

const storage = new Map()
const previousLocalStorage = globalThis.localStorage
globalThis.localStorage = {
  getItem(key) {
    return storage.get(key) ?? null
  },
  setItem(key, value) {
    storage.set(key, String(value))
  },
  removeItem(key) {
    storage.delete(key)
  },
}

await mkdir(outDir, { recursive: true })

try {
  const [source, pickerSource, treeItemSource, styleSource, assetFiles] = await Promise.all([
    readFile(sourcePath, 'utf8'),
    readFile(pickerPath, 'utf8'),
    readFile(treeItemPath, 'utf8'),
    readFile(stylePath, 'utf8'),
    readdir(assetDir),
  ])
  const assetImports = [...source.matchAll(/^import\s+(\w+)\s+from\s+'(\.\.\/assets\/notebook-icons\/[^']+\.png)'$/gm)]
  const importedAssetFiles = assetImports.map(([, , assetPath]) => path.basename(assetPath)).sort()
  const pngFiles = assetFiles.filter(file => file.endsWith('.png')).sort()
  const addedAssetFiles = pngFiles.filter(file => !existingAssetFiles.has(file))

  assert.equal(assetImports.length, 65, 'the catalog must import all 65 bundled icon files')
  assert.deepEqual(importedAssetFiles, pngFiles, 'every bundled icon must be registered exactly once')
  assert.equal(addedAssetFiles.length, 60, 'the supplied 60 icons must be added to the catalog')

  const addedAssetStats = await Promise.all(addedAssetFiles.map(async (file) => {
    const filePath = path.join(assetDir, file)
    const [buffer, metadata] = await Promise.all([readFile(filePath), stat(filePath)])
    return { file, dimensions: readPngDimensions(buffer), bytes: metadata.size }
  }))
  assert.ok(
    addedAssetStats.every(({ dimensions }) => dimensions.width === 256 && dimensions.height === 256),
    'new bundled icons must be optimized to 256×256 pixels',
  )
  assert.ok(
    addedAssetStats.reduce((total, asset) => total + asset.bytes, 0) <= 7 * 1024 * 1024,
    'the new bundled icons must stay within the 7 MiB asset budget',
  )

  const moduleSource = source.replace(
    /^import\s+(\w+)\s+from\s+'\.\.\/assets\/notebook-icons\/[^']+\.png'$/gm,
    'const $1 = "$1"',
  )
  const compiled = ts.transpileModule(moduleSource, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  await writeFile(outFile, compiled.outputText, 'utf8')
  const icons = await import(pathToFileURL(outFile).href)

  assert.deepEqual(
    icons.defaultNotebookIconGroups.map(group => [group.id, group.icons.length]),
    [['basic', 5], ['simple', 20], ['picture', 20], ['atlas', 20]],
    'default icons must remain grouped by collection',
  )
  assert.equal(icons.defaultNotebookIcons.length, 65)
  assert.equal(icons.defaultNotebookIcons[0].id, icons.DEFAULT_NOTEBOOK_ICON)
  assert.equal(new Set(icons.defaultNotebookIcons.map(icon => icon.id)).size, 65)
  assert.ok(icons.defaultNotebookIcons.every(icon => /^(default|user):[A-Za-z0-9_-]+$/.test(icon.id)))
  for (const id of ['default:note', 'default:pen', 'default:task', 'default:calendar', 'default:light']) {
    assert.ok(icons.defaultNotebookIcons.some(icon => icon.id === id), `existing icon ID must remain available: ${id}`)
  }
  assert.equal(icons.resolveNotebookIcon('default:missing').id, icons.DEFAULT_NOTEBOOK_ICON)

  const userIcon = {
    id: 'user:example-icon',
    label: 'Example icon',
    src: 'data:image/png;base64,AA==',
    source: 'user',
  }
  storage.set(icons.USER_ICON_STORAGE_KEY, JSON.stringify([userIcon]))
  assert.equal(icons.getNotebookIconGroups().at(-1).id, 'user')
  assert.equal(icons.getNotebookIconGroups().at(-1).icons[0].id, userIcon.id)
  assert.equal(icons.isKnownNotebookIcon(userIcon.id), true)
  assert.equal(icons.removeUserNotebookIcon(userIcon.id), true)
  assert.equal(icons.isKnownNotebookIcon(userIcon.id), false)

  assert.match(pickerSource, /getNotebookIconOptions/, 'picker must render one shared icon list')
  assert.match(pickerSource, /v-for="icon in icons"/, 'the shared panel must contain every icon')
  assert.match(pickerSource, /aria-expanded/, 'the disclosure button must expose its state')
  assert.match(pickerSource, /aria-controls/, 'the disclosure button must identify its panel')
  assert.match(pickerSource, /v-if="isIconPickerExpanded"/, 'collapsed picker must not render icon images')
  assert.match(pickerSource, /useId\(\)/, 'the shared picker panel ID must be unique per instance')
  assert.match(pickerSource, /loading="lazy"/, 'opened picker must lazily load icon images')
  assert.doesNotMatch(pickerSource, /selectedOption/, 'the collapsed header must not show the selected icon')
  assert.doesNotMatch(pickerSource, /notebook-icon-current/, 'the collapsed header must not render an icon preview')
  assert.match(pickerSource, /isIconPickerExpanded\.value = true/, 'adding an icon must reveal the shared panel')
  assert.doesNotMatch(pickerSource, /v-for="group in iconGroups"/, 'the picker must not create separate collapsible groups')
  const iconOptionStyle = pickerSource.match(/\.notebook-icon-option\s*\{([\s\S]*?)\n\}/)?.[1] ?? ''
  const iconImageStyle = pickerSource.match(/\.notebook-icon-option img\s*\{([\s\S]*?)\n\}/)?.[1] ?? ''
  const checkedIconStyle = pickerSource.match(/\.notebook-icon-option\[data-state='checked'\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? ''
  assert.match(iconOptionStyle, /padding:\s*0;/, 'icon buttons must not add inner whitespace')
  assert.match(iconOptionStyle, /border:\s*0;/, 'icon buttons must not render an outer border')
  assert.match(iconOptionStyle, /background:\s*transparent;/, 'icon buttons must not render an outer background')
  assert.match(iconImageStyle, /width:\s*100%;/, 'icon images must fill the current grid cell width')
  assert.match(iconImageStyle, /height:\s*100%;/, 'icon images must fill the current grid cell height')
  assert.match(checkedIconStyle, /box-shadow:\s*0 0 0 3px var\(--brand-primary\);/, 'selected icons must use a strong 3px brand outline')
  assert.doesNotMatch(styleSource, /\.notebook-icon\s*\{[^}]*opacity:/s, 'notebook list icons must not be dimmed relative to the picker')
  assert.match(treeItemSource, /<ContentLockControls[\s\S]*<NotebookIconPicker v-model="editIcon"/, 'lock controls must appear before the icon picker')

  console.log('notebook icon tests passed')
} finally {
  await rm(outDir, { recursive: true, force: true })
  if (previousLocalStorage === undefined) {
    delete globalThis.localStorage
  } else {
    globalThis.localStorage = previousLocalStorage
  }
}
